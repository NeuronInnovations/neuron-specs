package tasking

import (
	"bytes"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/auditlane"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

var ulidRe = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// capture is a counting Forwarder.
type capture struct {
	mu sync.Mutex
	n  int
}

func (c *capture) Send(*sapientpb.SapientMessage) error {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return nil
}
func (c *capture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func detection() *sapientpb.SapientMessage {
	return &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			ObjectId: proto.String("01J9ZSN0W5K3QH7Y8B4F2C6XME"),
		}},
	}
}

func TestNewULID_CanonicalFormat(t *testing.T) {
	// Deterministic: fixed time + fixed entropy => stable, valid ULID.
	id, err := newULID(time.UnixMilli(1_700_000_000_000), bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}))
	require.NoError(t, err)
	require.Regexp(t, ulidRe, id)
	id2, _ := newULID(time.UnixMilli(1_700_000_000_000), bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}))
	require.Equal(t, id, id2, "same inputs => same ULID")
	require.Regexp(t, ulidRe, DefaultIDFunc())
}

func TestApplyControl_StopStartIdempotent(t *testing.T) {
	m := NewManager(Options{ASMNodeID: "asm-1"})
	m.RegisterSession("A", &capture{})

	ack := m.applyControl(&sapientpb.Task{TaskId: proto.String("t1"), Control: sapientpb.Task_CONTROL_STOP.Enum()}, "A")
	require.Equal(t, sapientpb.TaskAck_TASK_STATUS_ACCEPTED, ack.GetTaskStatus())
	_, open := m.SessionOpen("A")
	require.False(t, open, "STOP closes the gate")

	// Idempotent STOP — still accepted, still closed.
	ack = m.applyControl(&sapientpb.Task{TaskId: proto.String("t2"), Control: sapientpb.Task_CONTROL_STOP.Enum()}, "A")
	require.Equal(t, sapientpb.TaskAck_TASK_STATUS_ACCEPTED, ack.GetTaskStatus())
	_, open = m.SessionOpen("A")
	require.False(t, open)

	ack = m.applyControl(&sapientpb.Task{TaskId: proto.String("t3"), Control: sapientpb.Task_CONTROL_START.Enum()}, "A")
	require.Equal(t, sapientpb.TaskAck_TASK_STATUS_ACCEPTED, ack.GetTaskStatus())
	_, open = m.SessionOpen("A")
	require.True(t, open, "START reopens the gate")
}

func TestApplyControl_RejectsNodeGlobalAndPause(t *testing.T) {
	m := NewManager(Options{ASMNodeID: "asm-1"})
	cases := map[string]*sapientpb.Task_Command{
		"mode_change": {Command: &sapientpb.Task_Command_ModeChange{ModeChange: "night"}},
		"look_at":     {Command: &sapientpb.Task_Command_LookAt{LookAt: &sapientpb.LocationOrRangeBearing{}}},
		"move_to":     {Command: &sapientpb.Task_Command_MoveTo{MoveTo: &sapientpb.LocationList{}}},
		"patrol":      {Command: &sapientpb.Task_Command_Patrol{Patrol: &sapientpb.LocationList{}}},
		"follow":      {Command: &sapientpb.Task_Command_Follow{Follow: &sapientpb.FollowObject{}}},
	}
	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			ack := m.applyControl(&sapientpb.Task{TaskId: proto.String("x"), Command: cmd}, "A")
			require.Equal(t, sapientpb.TaskAck_TASK_STATUS_REJECTED, ack.GetTaskStatus())
			require.NotEmpty(t, ack.GetReason())
			require.Contains(t, ack.GetReason()[0], name)
		})
	}
	// PAUSE is explicitly not implemented — rejected, not silently dropped.
	ack := m.applyControl(&sapientpb.Task{TaskId: proto.String("p"), Control: sapientpb.Task_CONTROL_PAUSE.Enum()}, "A")
	require.Equal(t, sapientpb.TaskAck_TASK_STATUS_REJECTED, ack.GetTaskStatus())
}

func TestApplyControl_StopWithCommand_StillAccepted(t *testing.T) {
	// FR-S29: control and command are separate Task fields; a per-session STOP that
	// co-carries a node-global command MUST NOT be rejected as UnsupportedTask.
	m := NewManager(Options{ASMNodeID: "asm-1"})
	m.RegisterSession("A", &capture{})
	task := &sapientpb.Task{
		TaskId:  proto.String("t1"),
		Control: sapientpb.Task_CONTROL_STOP.Enum(),
		Command: &sapientpb.Task_Command{Command: &sapientpb.Task_Command_MoveTo{MoveTo: &sapientpb.LocationList{}}},
	}
	ack := m.applyControl(task, "A")
	require.Equal(t, sapientpb.TaskAck_TASK_STATUS_ACCEPTED, ack.GetTaskStatus(), "STOP+command must still be accepted")
	_, open := m.SessionOpen("A")
	require.False(t, open, "the STOP still gated the session")
}

func TestApplyControl_RegionFilter(t *testing.T) {
	m := NewManager(Options{ASMNodeID: "asm-1"})
	m.RegisterSession("A", &capture{})

	// Region-only task (no control) → REJECTED with a precise reason (017 says rid
	// supports region/class filters; this prototype does not — FR-S26a).
	ack := m.applyControl(&sapientpb.Task{TaskId: proto.String("r1"), Region: []*sapientpb.Task_Region{{}}}, "A")
	require.Equal(t, sapientpb.TaskAck_TASK_STATUS_REJECTED, ack.GetTaskStatus())
	require.NotEmpty(t, ack.GetReason())
	require.Contains(t, ack.GetReason()[0], "region/class")

	// STOP + region → mandatory STOP ACCEPTED (FR-S29); ignored filter DISCLOSED, not silent (FR-S26a).
	ack = m.applyControl(&sapientpb.Task{TaskId: proto.String("r2"), Control: sapientpb.Task_CONTROL_STOP.Enum(), Region: []*sapientpb.Task_Region{{}}}, "A")
	require.Equal(t, sapientpb.TaskAck_TASK_STATUS_ACCEPTED, ack.GetTaskStatus())
	require.NotEmpty(t, ack.GetReason(), "co-present filter must be disclosed, not silently ignored")
	_, open := m.SessionOpen("A")
	require.False(t, open, "the STOP still gated the session")
}

func TestConsume_IgnoresTaskForOtherASM(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()
	m := NewManager(Options{ASMNodeID: "asm-1", Lane: lane})
	require.NoError(t, m.Start(t.Context()))
	m.RegisterSession("A", &capture{})
	m.RegisterSession("B", &capture{})

	// A STOP addressed to a DIFFERENT ASM (DestinationId mismatch) must be ignored.
	wrongDest := &sapientpb.SapientMessage{
		NodeId:        proto.String("A"),
		DestinationId: proto.String("asm-OTHER"),
		Content:       &sapientpb.SapientMessage_Task{Task: &sapientpb.Task{TaskId: proto.String("x1"), Control: sapientpb.Task_CONTROL_STOP.Enum()}},
	}
	require.NoError(t, lane.Publish(t.Context(), auditlane.Channel{ASMNodeID: "asm-1", Role: auditlane.RoleStdIn}, wrongDest))

	// A correct STOP for B is processed in-order AFTER the wrong-dest one; once B is
	// gated, the earlier mismatch has been seen-and-skipped — so A must still be open.
	_, err := IssueControl(t.Context(), lane, "asm-1", "B", sapientpb.Task_CONTROL_STOP, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool { _, open := m.SessionOpen("B"); return !open }, 3*time.Second, 10*time.Millisecond)

	_, openA := m.SessionOpen("A")
	require.True(t, openA, "a Task addressed to asm-OTHER must NOT gate session A")
}

func TestValidFeedSource(t *testing.T) {
	for _, s := range []string{"live", "replay", "synthetic", "placeholder"} {
		require.True(t, ValidFeedSource(s), "%q should be valid", s)
	}
	for _, s := range []string{"", "sythetic", "LIVE", "bogus"} {
		require.False(t, ValidFeedSource(s), "%q should be rejected", s)
	}
}

func TestForward_OnlyOpenSessions_InFlightNotRecalled(t *testing.T) {
	m := NewManager(Options{ASMNodeID: "asm-1"})
	a, b := &capture{}, &capture{}
	m.RegisterSession("A", a)
	m.RegisterSession("B", b)

	require.NoError(t, m.Forward(detection())) // both receive
	require.Equal(t, 1, a.count())
	require.Equal(t, 1, b.count())

	m.applyControl(&sapientpb.Task{Control: sapientpb.Task_CONTROL_STOP.Enum()}, "A")
	require.NoError(t, m.Forward(detection())) // only B
	require.Equal(t, 1, a.count(), "A already-delivered frame is not recalled and no new frame is sent")
	require.Equal(t, 2, b.count())
}

// TestTwoBuyerIsolation_OverLane is the end-to-end isolation proof: A's STOP gates
// only A's stream while B keeps flowing; A's START resumes A.
func TestTwoBuyerIsolation_OverLane(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()
	m := NewManager(Options{ASMNodeID: "asm-1", Lane: lane})
	require.NoError(t, m.Start(t.Context()))

	a, b := &capture{}, &capture{}
	m.RegisterSession("A", a)
	m.RegisterSession("B", b)

	require.NoError(t, m.Forward(detection()))
	require.Equal(t, 1, a.count())
	require.Equal(t, 1, b.count())

	// A issues STOP.
	_, err := IssueControl(t.Context(), lane, "asm-1", "A", sapientpb.Task_CONTROL_STOP, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool { _, open := m.SessionOpen("A"); return !open }, 3*time.Second, 10*time.Millisecond)

	require.NoError(t, m.Forward(detection()))
	require.Equal(t, 1, a.count(), "A stopped")
	require.Equal(t, 2, b.count(), "B unaffected")

	// A resumes.
	_, err = IssueControl(t.Context(), lane, "asm-1", "A", sapientpb.Task_CONTROL_START, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool { _, open := m.SessionOpen("A"); return open }, 3*time.Second, 10*time.Millisecond)

	require.NoError(t, m.Forward(detection()))
	require.Equal(t, 2, a.count(), "A resumed")
	require.Equal(t, 3, b.count())
}

func TestTaskAck_EmittedOnStdOut(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()
	m := NewManager(Options{ASMNodeID: "asm-1", Lane: lane})
	require.NoError(t, m.Start(t.Context()))
	m.RegisterSession("A", &capture{})

	acks, err := lane.Subscribe(t.Context(), auditlane.Channel{ASMNodeID: "asm-1", Role: auditlane.RoleStdOut})
	require.NoError(t, err)

	_, err = IssueControl(t.Context(), lane, "asm-1", "A", sapientpb.Task_CONTROL_STOP, nil)
	require.NoError(t, err)

	select {
	case msg := <-acks:
		require.Equal(t, sapientpb.TaskAck_TASK_STATUS_ACCEPTED, msg.GetTaskAck().GetTaskStatus())
		require.Equal(t, "A", msg.GetDestinationId())
	case <-time.After(3 * time.Second):
		t.Fatal("no TaskAck on stdOut")
	}
}

func TestStatusReport_FeedSourceAndChannel(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()
	m := NewManager(Options{ASMNodeID: "asm-1", Lane: lane, FeedSource: "synthetic"})

	// Status emission must not reach the data sessions.
	sess := &capture{}
	m.RegisterSession("A", sess)

	out, err := lane.Subscribe(t.Context(), auditlane.Channel{ASMNodeID: "asm-1", Role: auditlane.RoleStdOut})
	require.NoError(t, err)
	require.NoError(t, m.EmitStatusReport(t.Context()))

	select {
	case msg := <-out:
		sr := msg.GetStatusReport()
		require.NotNil(t, sr)
		require.Equal(t, sapientpb.StatusReport_SYSTEM_OK, sr.GetSystem())
		require.Equal(t, sapientpb.StatusReport_INFO_NEW, sr.GetInfo(), "first report is NEW")
		require.Regexp(t, ulidRe, sr.GetReportId())
		require.Len(t, sr.GetStatus(), 3, "feedSource + framesForwarded + bridge")
		// Conformance: feedSource rides status[0] (017 CL-R9, namespaced prefix).
		require.Equal(t, "neuron.feedSource=synthetic", sr.GetStatus()[0].GetStatusValue())
		fs, ok := ParseFeedSource(sr.GetStatus()[0].GetStatusValue())
		require.True(t, ok)
		require.Equal(t, "synthetic", fs)
		// Enriched telemetry, distinct from the 005 heartbeat (015 FR-S31).
		frames, ok := ParseFramesForwarded(sr.GetStatus()[1].GetStatusValue())
		require.True(t, ok)
		require.Equal(t, uint64(0), frames)
		connected, ok := ParseBridgeConnected(sr.GetStatus()[2].GetStatusValue())
		require.True(t, ok)
		require.True(t, connected, "default bridge state is connected with no StatusInputs hook")
	case <-time.After(3 * time.Second):
		t.Fatal("no StatusReport on stdOut")
	}
	require.Equal(t, 0, sess.count(), "StatusReport must not touch the p2p DetectionReport stream")

	// Second report flips INFO to UNCHANGED.
	require.NoError(t, m.EmitStatusReport(t.Context()))
	select {
	case msg := <-out:
		require.Equal(t, sapientpb.StatusReport_INFO_UNCHANGED, msg.GetStatusReport().GetInfo())
	case <-time.After(3 * time.Second):
		t.Fatal("no second StatusReport")
	}
}

func TestStatusReport_EnrichedTelemetry(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()
	m := NewManager(Options{
		ASMNodeID: "asm-1", Lane: lane, FeedSource: "live",
		StatusInputs: func() StatusSnapshot { return StatusSnapshot{BridgeConnected: false, Degraded: true} },
	})
	m.RegisterSession("A", &capture{})
	require.NoError(t, m.Forward(detection())) // framesForwarded → 2
	require.NoError(t, m.Forward(detection()))

	out, err := lane.Subscribe(t.Context(), auditlane.Channel{ASMNodeID: "asm-1", Role: auditlane.RoleStdOut})
	require.NoError(t, err)
	require.NoError(t, m.EmitStatusReport(t.Context()))

	select {
	case msg := <-out:
		sr := msg.GetStatusReport()
		require.NotNil(t, sr)
		require.Equal(t, sapientpb.StatusReport_SYSTEM_WARNING, sr.GetSystem(), "degraded → SYSTEM_WARNING")
		var sawFrames, sawBridge bool
		for _, s := range sr.GetStatus() {
			if n, ok := ParseFramesForwarded(s.GetStatusValue()); ok {
				require.Equal(t, uint64(2), n)
				sawFrames = true
			}
			if c, ok := ParseBridgeConnected(s.GetStatusValue()); ok {
				require.False(t, c, "bridge reported disconnected")
				sawBridge = true
			}
		}
		require.True(t, sawFrames, "framesForwarded entry present")
		require.True(t, sawBridge, "bridge entry present")
	case <-time.After(3 * time.Second):
		t.Fatal("no StatusReport on stdOut")
	}
}

func TestEmitRegistration_OnStdOut(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()
	m := NewManager(Options{ASMNodeID: "asm-rid-1", Lane: lane})
	out, err := lane.Subscribe(t.Context(), auditlane.Channel{ASMNodeID: "asm-rid-1", Role: auditlane.RoleStdOut})
	require.NoError(t, err)
	require.NoError(t, m.EmitRegistration(t.Context()))

	select {
	case msg := <-out:
		require.Equal(t, "asm-rid-1", msg.GetNodeId())
		reg := msg.GetRegistration()
		require.NotNil(t, reg)
		require.Equal(t, "BSI Flex 335 v2.0", reg.GetIcdVersion())
		require.Equal(t, "neuron-rid", reg.GetShortName())
		require.NotEmpty(t, reg.GetNodeDefinition())
		require.Contains(t, reg.GetNodeDefinition()[0].GetNodeSubType(), "neuron.rid/1",
			"the extension namespace is declared (pointer model)")
	case <-time.After(3 * time.Second):
		t.Fatal("no Registration on stdOut")
	}
}

func TestRegistrationAck_RecognizedNonGating(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()
	m := NewManager(Options{ASMNodeID: "asm-1", Lane: lane})
	require.NoError(t, m.Start(t.Context()))
	m.RegisterSession("A", &capture{})

	_, ok := m.RegistrationAck()
	require.False(t, ok, "no ack observed yet")

	ackMsg := &sapientpb.SapientMessage{
		NodeId:  proto.String("hldmm"),
		Content: &sapientpb.SapientMessage_RegistrationAck{RegistrationAck: &sapientpb.RegistrationAck{Acceptance: proto.Bool(true)}},
	}
	require.NoError(t, lane.Publish(t.Context(), auditlane.Channel{ASMNodeID: "asm-1", Role: auditlane.RoleStdIn}, ackMsg))

	require.Eventually(t, func() bool {
		accepted, ok := m.RegistrationAck()
		return ok && accepted
	}, 3*time.Second, 10*time.Millisecond)

	// Non-gating: the data gate is untouched by the ack — forwarding still works.
	require.NoError(t, m.Forward(detection()))
}

func TestStatusReport_TickerAtInterval(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()
	m := NewManager(Options{ASMNodeID: "asm-1", Lane: lane, FeedSource: "synthetic", StatusInterval: 25 * time.Millisecond})
	out, err := lane.Subscribe(t.Context(), auditlane.Channel{ASMNodeID: "asm-1", Role: auditlane.RoleStdOut})
	require.NoError(t, err)
	require.NoError(t, m.Start(t.Context()))

	got := 0
	deadline := time.After(2 * time.Second)
	for got < 2 {
		select {
		case <-out:
			got++
		case <-deadline:
			t.Fatalf("only %d StatusReports at the configured interval", got)
		}
	}
}
