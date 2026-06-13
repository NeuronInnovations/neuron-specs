package tasking

import (
	"context"
	"io"
	"log"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/auditlane"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// Forwarder is one session's data sink — the seller wraps the 009 FrameWriter so
// the Manager fans DetectionReports out to live consumer streams.
type Forwarder interface {
	Send(*sapientpb.SapientMessage) error
}

// Options configures a Manager. A nil Lane disables the control plane entirely
// (the seller then behaves exactly as the data-only build).
type Options struct {
	ASMNodeID      string         // this ASM's node_id (envelope NodeId on its output)
	Lane           auditlane.Lane // nil => tasking/status disabled
	FeedSource     string         // live|replay|synthetic|placeholder (017 FR-R-E02)
	StatusInterval time.Duration  // 0 => no StatusReport ticker
	Mode           string         // ASM mode string (default "operational")
	IDFunc         func() string  // ULID source (default DefaultIDFunc)
	Now            func() time.Time
	Logger         *log.Logger
	// StatusInputs, if non-nil, is sampled on every StatusReport tick for the
	// runtime telemetry the seller owns (upstream bridge state + degraded). Nil
	// => bridge reported connected, never degraded. The forwarded-frame count is
	// owned by the Manager and is NOT taken from here.
	StatusInputs func() StatusSnapshot
	// Registration, if non-nil, parameterizes EmitRegistration for non-RID
	// sellers (the JetVision ADS-B seller). nil keeps the original RID
	// Registration byte-identical.
	Registration *RegistrationIdentity
}

// RegistrationIdentity is the modality-specific identity EmitRegistration
// publishes (015 FR-S21 pointer model).
type RegistrationIdentity struct {
	Name        string   // e.g. "Neuron SAPIENT ADS-B seller"
	ShortName   string   // e.g. "neuron-adsb"
	Model       string   // e.g. "sapient-jv-seller"
	NodeSubType []string // capability namespaces, e.g. ["neuron.adsb/1"]
}

type session struct {
	out  Forwarder
	open bool
}

// Manager owns per-session DetectionReport gating, Task handling, and StatusReport
// emission for one ASM. Safe for concurrent use.
type Manager struct {
	opts Options

	mu              sync.Mutex
	sessions        map[string]*session // keyed by consumer node_id (the session id)
	activeTaskID    string
	statusCount     uint64
	framesForwarded uint64 // DetectionReports fanned to ≥1 open session
	regAckSet       bool   // a RegistrationAck has been observed on stdIn
	regAcked        bool   // its acceptance flag (valid only when regAckSet)
	regAckReason    []string

	wg sync.WaitGroup
}

// NewManager builds a Manager, applying defaults. An empty FeedSource defaults to
// "live" per 017 FR-R-E02 (absent ⇒ live).
func NewManager(opts Options) *Manager {
	if opts.Mode == "" {
		opts.Mode = "operational"
	}
	if opts.FeedSource == "" {
		opts.FeedSource = "live"
	}
	if opts.IDFunc == nil {
		opts.IDFunc = DefaultIDFunc
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = log.New(io.Discard, "", 0)
	}
	return &Manager{opts: opts, sessions: make(map[string]*session)}
}

// RegisterSession adds (or replaces) a consumer session, initially OPEN.
func (m *Manager) RegisterSession(consumerNodeID string, out Forwarder) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[consumerNodeID] = &session{out: out, open: true}
}

// RemoveSession drops a consumer session.
func (m *Manager) RemoveSession(consumerNodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, consumerNodeID)
}

// SessionOpen reports a session's gate state (exists, open) — for tests/observability.
func (m *Manager) SessionOpen(consumerNodeID string) (exists, open bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[consumerNodeID]
	if !ok {
		return false, false
	}
	return true, s.open
}

// Forward fans msg out to every OPEN session. In-flight/earlier sends are never
// recalled. Returns the first send error (the seller treats it as fatal for its
// single session); a nil Forwarder session is skipped.
func (m *Manager) Forward(msg *sapientpb.SapientMessage) error {
	m.mu.Lock()
	targets := make([]Forwarder, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.open && s.out != nil {
			targets = append(targets, s.out)
		}
	}
	if len(targets) > 0 {
		m.framesForwarded++ // count frames actually fanned to a live consumer
	}
	m.mu.Unlock()

	var firstErr error
	for _, out := range targets {
		if err := out.Send(msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Start launches the lane subscriber (Task handling) and, if StatusInterval > 0,
// the StatusReport ticker. No-op when Lane is nil. Stop by cancelling ctx and
// calling Wait.
func (m *Manager) Start(ctx context.Context) error {
	if m.opts.Lane == nil {
		return nil
	}
	sub, err := m.opts.Lane.Subscribe(ctx, auditlane.Channel{ASMNodeID: m.opts.ASMNodeID, Role: auditlane.RoleStdIn})
	if err != nil {
		return Wrap(ErrLane, "Start", err)
	}
	m.wg.Go(func() { m.consume(ctx, sub) })
	if m.opts.StatusInterval > 0 {
		m.wg.Go(func() { m.statusLoop(ctx) })
	}
	return nil
}

// Wait blocks until the Manager's goroutines have stopped (after ctx cancel).
func (m *Manager) Wait() { m.wg.Wait() }

func (m *Manager) consume(ctx context.Context, sub <-chan *sapientpb.SapientMessage) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub:
			if !ok {
				return
			}
			if ack := msg.GetRegistrationAck(); ack != nil {
				m.recordRegistrationAck(ack) // informational; non-gating (FR-S96 ack is DEFERRED)
				continue
			}
			task := msg.GetTask()
			if task == nil {
				continue // not a Task (e.g. an AlertAck) — out of scope
			}
			if dest := msg.GetDestinationId(); dest != "" && dest != m.opts.ASMNodeID {
				continue // addressed to a different ASM
			}
			from := msg.GetNodeId()
			ack := m.applyControl(task, from)
			if err := m.publish(ctx, auditlane.RoleStdOut, m.wrapAck(ack, from)); err != nil {
				m.opts.Logger.Printf("publish TaskAck: %v", err)
			}
		}
	}
}

// applyControl mutates session gate state for a Task and returns the TaskAck.
//
// control (field 6) and command (field 8) are SEPARATE Task fields (not a oneof),
// so a per-session STOP/START may legitimately co-carry a command. FR-S29 requires
// the mandatory per-session STOP/START to be honoured and NOT rejected as
// UnsupportedTask — so control is evaluated FIRST; the node-global command refusal
// applies only to a pure command Task (control unset).
func (m *Manager) applyControl(task *sapientpb.Task, from string) *sapientpb.TaskAck {
	taskID := task.GetTaskId()
	// 017 says the rid service accepts region/class per-session filters, but this
	// prototype implements only STOP/START. A co-present region/class filter is
	// disclosed (FR-S26a: never silently ignored), never silently applied.
	hasFilter := len(task.GetRegion()) > 0
	switch task.GetControl() {
	case sapientpb.Task_CONTROL_STOP:
		m.setGate(from, false)
		m.setActiveTask(taskID)
		return controlAck(taskID, hasFilter)
	case sapientpb.Task_CONTROL_START:
		m.setGate(from, true)
		m.setActiveTask(taskID)
		return controlAck(taskID, hasFilter)
	case sapientpb.Task_CONTROL_PAUSE:
		return rejectAck(taskID, "UnsupportedControl: CONTROL_PAUSE not implemented by this prototype")
	}
	// control is UNSPECIFIED — a pure command/filter Task.
	if name := commandName(task.GetCommand()); name != "" {
		return rejectAck(taskID, "UnsupportedTask: node-global command "+name+" not supported by this passive sensor (FR-S30/S60)")
	}
	if hasFilter {
		return rejectAck(taskID, "UnsupportedTask: region/class filters not implemented by this prototype (FR-S26a)")
	}
	return rejectAck(taskID, "UnsupportedTask: no actionable control or command")
}

func (m *Manager) setGate(consumerNodeID string, open bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[consumerNodeID]; ok {
		s.open = open
		return
	}
	m.opts.Logger.Printf("control for unknown session %q (accepted, no active stream)", consumerNodeID)
}

func (m *Manager) setActiveTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeTaskID = taskID
}

func (m *Manager) statusLoop(ctx context.Context) {
	ticker := time.NewTicker(m.opts.StatusInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.EmitStatusReport(ctx); err != nil {
				m.opts.Logger.Printf("emit StatusReport: %v", err)
			}
		}
	}
}

// EmitStatusReport builds and publishes one StatusReport on the ASM's stdOut. Safe
// to call directly (the ticker calls it). No-op when Lane is nil.
func (m *Manager) EmitStatusReport(ctx context.Context) error {
	if m.opts.Lane == nil {
		return nil
	}
	msg := &sapientpb.SapientMessage{
		NodeId:    proto.String(m.opts.ASMNodeID),
		Timestamp: timestamppb.New(m.opts.Now()),
		Content:   &sapientpb.SapientMessage_StatusReport{StatusReport: m.buildStatusReport()},
	}
	return m.publish(ctx, auditlane.RoleStdOut, msg)
}

func (m *Manager) buildStatusReport() *sapientpb.StatusReport {
	m.mu.Lock()
	active := m.activeTaskID
	first := m.statusCount == 0
	m.statusCount++
	frames := m.framesForwarded
	m.mu.Unlock()

	// Runtime telemetry the seller owns (bridge health + degraded). Default:
	// bridge connected, not degraded (a control plane with no inputs hook).
	snap := StatusSnapshot{BridgeConnected: true}
	if m.opts.StatusInputs != nil {
		snap = m.opts.StatusInputs()
	}

	info := sapientpb.StatusReport_INFO_UNCHANGED
	if first {
		info = sapientpb.StatusReport_INFO_NEW
	}
	system := sapientpb.StatusReport_SYSTEM_OK
	if snap.Degraded {
		system = sapientpb.StatusReport_SYSTEM_WARNING // FR-S31 health, distinct from 005 liveness
	}
	sr := &sapientpb.StatusReport{
		ReportId: proto.String(m.opts.IDFunc()),
		System:   system.Enum(),
		Info:     info.Enum(),
		Mode:     proto.String(m.opts.Mode),
		Status: []*sapientpb.StatusReport_Status{
			neuronStatus(FeedSourceStatusValue(m.opts.FeedSource)),
			neuronStatus(FramesForwardedStatusValue(frames)),
			neuronStatus(BridgeStatusValue(snap.BridgeConnected)),
		},
	}
	if active != "" {
		sr.ActiveTaskId = proto.String(active)
	}
	return sr
}

// neuronStatus builds an informational OTHER status[] entry carrying one
// "neuron.*" status_value (feedSource / framesForwarded / bridge).
func neuronStatus(value string) *sapientpb.StatusReport_Status {
	return &sapientpb.StatusReport_Status{
		StatusType:  sapientpb.StatusReport_STATUS_TYPE_OTHER.Enum(),
		StatusLevel: sapientpb.StatusReport_STATUS_LEVEL_INFORMATION_STATUS.Enum(),
		StatusValue: proto.String(value),
	}
}

// ridExtensionID mirrors sapient.ExtensionID (the neuron.rid/1 capability
// namespace) without importing the sapient card builder into the control plane.
const ridExtensionID = "neuron.rid/1"

// EmitRegistration publishes the ASM's Registration on stdOut (015 FR-S21 /
// FR-S92 — Registration rides the auditable lane, ASM stdOut). Pointer model:
// node_id + ICD version + the neuron.rid/1 extension namespace + sensor config;
// the full capability set lives in the agent card the Registration points at.
// Call once after Start and ONLY when the lane is topic-backed — the file-mode
// audit stream stays byte-identical. No-op when Lane is nil.
func (m *Manager) EmitRegistration(ctx context.Context) error {
	if m.opts.Lane == nil {
		return nil
	}
	msg := &sapientpb.SapientMessage{
		NodeId:    proto.String(m.opts.ASMNodeID),
		Timestamp: timestamppb.New(m.opts.Now()),
		Content:   &sapientpb.SapientMessage_Registration{Registration: buildRegistration(m.opts.Registration)},
	}
	return m.publish(ctx, auditlane.RoleStdOut, msg)
}

// buildRegistration assembles the minimal pointer-model Registration. A nil
// identity keeps the original RID values byte-identical.
func buildRegistration(ri *RegistrationIdentity) *sapientpb.Registration {
	if ri == nil {
		ri = &RegistrationIdentity{
			Name:        "Neuron SAPIENT RID seller",
			ShortName:   "neuron-rid",
			Model:       "sapient-rid-seller",
			NodeSubType: []string{ridExtensionID},
		}
	}
	return &sapientpb.Registration{
		IcdVersion: proto.String("BSI Flex 335 v2.0"),
		Name:       proto.String(ri.Name),
		ShortName:  proto.String(ri.ShortName),
		NodeDefinition: []*sapientpb.Registration_NodeDefinition{{
			NodeSubType: ri.NodeSubType,
		}},
		ConfigData: []*sapientpb.Registration_ConfigurationData{{
			Manufacturer: "Neuron",
			Model:        ri.Model,
		}},
	}
}

// recordRegistrationAck stores the last RegistrationAck observed on stdIn and
// logs accept/reject. It does NOT gate DetectionReport flow (the buyer-side ack
// is DEFERRED; FR-S96).
func (m *Manager) recordRegistrationAck(ack *sapientpb.RegistrationAck) {
	accepted := ack.GetAcceptance()
	reason := ack.GetAckResponseReason()
	m.mu.Lock()
	m.regAckSet = true
	m.regAcked = accepted
	m.regAckReason = reason
	m.mu.Unlock()
	if accepted {
		m.opts.Logger.Printf("RegistrationAck: accepted")
	} else {
		m.opts.Logger.Printf("RegistrationAck: REJECTED reason=%v", reason)
	}
}

// RegistrationAck reports the last RegistrationAck observed on stdIn: ok is false
// until one arrives; accepted is its acceptance flag. Informational only (the
// buyer/HLDMM-side ack is DEFERRED — FR-S96).
func (m *Manager) RegistrationAck() (accepted, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.regAcked, m.regAckSet
}

func (m *Manager) wrapAck(ack *sapientpb.TaskAck, dest string) *sapientpb.SapientMessage {
	msg := &sapientpb.SapientMessage{
		NodeId:    proto.String(m.opts.ASMNodeID),
		Timestamp: timestamppb.New(m.opts.Now()),
		Content:   &sapientpb.SapientMessage_TaskAck{TaskAck: ack},
	}
	if dest != "" {
		msg.DestinationId = proto.String(dest)
	}
	return msg
}

func (m *Manager) publish(ctx context.Context, role auditlane.Role, msg *sapientpb.SapientMessage) error {
	return m.opts.Lane.Publish(ctx, auditlane.Channel{ASMNodeID: m.opts.ASMNodeID, Role: role}, msg)
}

func acceptAck(taskID string) *sapientpb.TaskAck {
	a := &sapientpb.TaskAck{TaskStatus: sapientpb.TaskAck_TASK_STATUS_ACCEPTED.Enum()}
	if taskID != "" {
		a.TaskId = proto.String(taskID)
	}
	return a
}

// controlAck builds the ACCEPTED ack for a STOP/START. The mandatory per-session
// control is always accepted (FR-S29); when the same Task also carries a region/
// class filter this prototype does not apply, that is disclosed in reason[] rather
// than silently ignored (FR-S26a).
func controlAck(taskID string, ignoredFilter bool) *sapientpb.TaskAck {
	a := acceptAck(taskID)
	if ignoredFilter {
		a.Reason = []string{"control accepted; co-present region/class filter ignored — not implemented by this prototype (FR-S26a)"}
	}
	return a
}

func rejectAck(taskID, reason string) *sapientpb.TaskAck {
	a := &sapientpb.TaskAck{
		TaskStatus: sapientpb.TaskAck_TASK_STATUS_REJECTED.Enum(),
		Reason:     []string{reason},
	}
	if taskID != "" {
		a.TaskId = proto.String(taskID)
	}
	return a
}
