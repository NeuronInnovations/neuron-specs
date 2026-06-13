package remoteid

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// RemoteIdLivenessState is the DApp-level liveness surface returned to a
// Remote ID buyer. It wraps Spec 005's five-state machine
// (Unknown/Alive/Suspect/Dead/Offline) plus the seller-advertised
// `operational.degraded` boolean (Spec 017 FR-R21). Stage 3C never
// modifies the Spec 005 state machine; it only re-projects it.
type RemoteIdLivenessState string

const (
	// LivenessUnknown — no heartbeat observed yet. Not emitted to the
	// Events channel (matches edgeapp LivenessTracker precedent).
	LivenessUnknown RemoteIdLivenessState = "Unknown"
	// LivenessHealthy — Spec 005 Alive AND operational.degraded != true.
	LivenessHealthy RemoteIdLivenessState = "Healthy"
	// LivenessStale — Spec 005 Suspect (deadline+grace expired, before
	// SuspectToDead window).
	LivenessStale RemoteIdLivenessState = "Stale"
	// LivenessOffline — Spec 005 Dead OR Offline (shutdown sentinel).
	LivenessOffline RemoteIdLivenessState = "Offline"
	// LivenessDegraded — Spec 005 Alive AND operational.degraded == true.
	LivenessDegraded RemoteIdLivenessState = "Degraded"
)

// RemoteIdLivenessEvent is what the monitor emits on a state transition.
// The caller receives the event on the Events channel and decides what to
// log / persist / surface. The monitor itself only emits on transitions
// (no repetition while a state is steady).
type RemoteIdLivenessEvent struct {
	// SellerEVM is the address of the seller whose state changed.
	SellerEVM string

	// State is the new DApp-level state.
	State RemoteIdLivenessState

	// Spec005 is the underlying Spec 005 liveness state that produced
	// State. Useful so reviewers can see whether a "Stale" / "Offline"
	// classification came from the time-based machine or the shutdown
	// sentinel path.
	Spec005 health.LivenessState

	// Degraded reflects the most recent operational.degraded value seen.
	// True iff the most recent valid heartbeat carried
	// `capabilities.operational.degraded = true`.
	Degraded bool

	// LastDeadline is the most recently observed NextHeartbeatDeadline
	// (Unix seconds). 0 if no valid heartbeat has been observed.
	LastDeadline uint64

	// At is when the monitor detected the transition (wall clock UTC).
	At time.Time
}

// DefaultLivenessPollInterval governs how often the watchdog ticks
// when RemoteIdLivenessMonitor.PollInterval is unset. Matches the Spec
// 005 cadence used by edgeapp's LivenessTracker.
const DefaultLivenessPollInterval = 5 * time.Second

// RemoteIdLivenessMonitor watches a single Remote ID seller's heartbeat
// stream and emits a RemoteIdLivenessEvent on every transition between
// the DApp-level liveness states. It mirrors the public shape of
// `internal/edgeapp/liveness.go::LivenessTracker` so reviewers can map
// the two implementations to each other directly; the difference is the
// DApp-state surface and the optional FR-R21 AgentURISha256 cross-check.
//
// Concurrency: Run blocks until ctx is cancelled OR Deliveries is closed.
// Events is closed exactly once on Run's return.
type RemoteIdLivenessMonitor struct {
	// SellerEVM is the address of the monitored seller (hex, 0x-prefixed).
	// Required. Used to scope observer state; deliveries from other
	// senders are validated but ignored for state-transition purposes.
	SellerEVM string

	// Deliveries is the channel of TopicMessage deliveries the caller
	// produces — typically by Subscribe'ing to the seller's stdOut.
	// Each delivery's ConsensusTimestamp must be in **seconds** matching
	// the heartbeat's NextHeartbeatDeadline scale. Required.
	Deliveries <-chan topic.MessageDelivery

	// PollInterval is how often the watchdog re-evaluates liveness state
	// against Now() so degradation transitions (Healthy→Stale etc.) fire
	// even when no new heartbeat arrives. Default: DefaultLivenessPollInterval.
	PollInterval time.Duration

	// Now returns the monitor's current wall-clock time. Tests inject a
	// controllable clock; production callers leave it nil to default to
	// time.Now.
	Now func() time.Time

	// Logger receives one info-level line per state transition + per
	// validation failure + per FR-R21 AgentURISha256 mismatch. Optional;
	// nil → no logs.
	Logger *log.Logger

	// Events is the channel the monitor writes RemoteIdLivenessEvents to.
	// Required (suggested capacity: 4). Closed by Run.
	Events chan<- RemoteIdLivenessEvent

	// ExpectedAgentURISha256, when non-empty, enables the FR-R21
	// defence-in-depth cross-check. Every valid delivery whose
	// operational.agentURISha256 disagrees with this value produces a
	// single `evidence-grade mismatch` log line. NO state transition is
	// emitted; the check is observation-only. Confirmed 2026-05-13 per
	// Stage 3C design decision.
	ExpectedAgentURISha256 string
}

func (m *RemoteIdLivenessMonitor) validate() error {
	if m == nil {
		return errors.New("remoteid.RemoteIdLivenessMonitor: nil monitor")
	}
	if m.SellerEVM == "" {
		return errors.New("remoteid.RemoteIdLivenessMonitor: SellerEVM required")
	}
	if m.Deliveries == nil {
		return errors.New("remoteid.RemoteIdLivenessMonitor: Deliveries channel required")
	}
	if m.Events == nil {
		return errors.New("remoteid.RemoteIdLivenessMonitor: Events channel required")
	}
	if m.PollInterval <= 0 {
		m.PollInterval = DefaultLivenessPollInterval
	}
	if m.Now == nil {
		m.Now = time.Now
	}
	if m.Logger == nil {
		m.Logger = log.New(discardWriter{}, "", 0)
	}
	return nil
}

// monitorState is the mutable per-Run state, guarded by mu. Splitting it
// off the public RemoteIdLivenessMonitor keeps the public type a value
// configuration object (zero-allocation, safe to compose into structs)
// without leaking ticker / observer / mutex internals.
type monitorState struct {
	mu           sync.Mutex
	observer     *health.HeartbeatObserver
	lastEmitted  RemoteIdLivenessState
	lastDegraded bool
}

// Run blocks until ctx is cancelled or Deliveries is closed, processing
// each delivery through the Spec 005 V-OBS-01..06 pipeline and emitting
// RemoteIdLivenessEvents on transitions between {Healthy, Stale, Offline,
// Degraded}. Run closes Events exactly once on return. Returns nil on
// graceful termination, error on misconfiguration.
func (m *RemoteIdLivenessMonitor) Run(ctx context.Context) error {
	if err := m.validate(); err != nil {
		return err
	}
	defer close(m.Events)

	state := &monitorState{
		observer:    health.NewHeartbeatObserver(),
		lastEmitted: LivenessUnknown,
	}

	tick := time.NewTicker(m.PollInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-m.Deliveries:
			if !ok {
				return nil
			}
			m.processDelivery(ctx, state, d)
		case <-tick.C:
			m.tickEvaluate(ctx, state)
		}
	}
}

func (m *RemoteIdLivenessMonitor) processDelivery(ctx context.Context, s *monitorState, d topic.MessageDelivery) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload, err := s.observer.ProcessDelivery(d)
	if err != nil {
		m.Logger.Printf("[liveness:remote-id] sellerEVM=%s dropped invalid heartbeat: %v",
			m.SellerEVM, err)
		return
	}

	// FR-R21 defence-in-depth: cross-check operational.agentURISha256
	// against the value the buyer captured at registry-lookup time.
	// Warn-only; no state transition.
	if m.ExpectedAgentURISha256 != "" && payload.Capabilities != nil && payload.Capabilities.Operational != nil {
		adv := payload.Capabilities.Operational.AgentURISha256
		if adv != "" && adv != m.ExpectedAgentURISha256 {
			m.Logger.Printf("[liveness:remote-id] evidence-grade mismatch: registered=%s advertised=%s",
				m.ExpectedAgentURISha256, adv)
		}
	}

	// Refresh lastDegraded from this heartbeat's Operational block.
	s.lastDegraded = false
	if payload.Capabilities != nil && payload.Capabilities.Operational != nil {
		s.lastDegraded = payload.Capabilities.Operational.Degraded
	}

	// Re-evaluate immediately on each delivery so recovery transitions
	// land without waiting for a tick.
	m.evaluateLocked(ctx, s)
}

func (m *RemoteIdLivenessMonitor) tickEvaluate(ctx context.Context, s *monitorState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.evaluateLocked(ctx, s)
}

// evaluateLocked assumes s.mu is held. Emits at most one event per call.
func (m *RemoteIdLivenessMonitor) evaluateLocked(ctx context.Context, s *monitorState) {
	nowSec := uint64(m.Now().Unix())
	spec005 := s.observer.GetLivenessState(m.SellerEVM, nowSec)
	next := mapState(spec005, s.lastDegraded)
	if next == LivenessUnknown || next == s.lastEmitted {
		return
	}
	m.emit(ctx, s, next, spec005)
}

// emit assumes s.mu is held.
func (m *RemoteIdLivenessMonitor) emit(ctx context.Context, s *monitorState, state RemoteIdLivenessState, spec005 health.LivenessState) {
	var deadline uint64
	if rec := s.observer.GetRecord(m.SellerEVM); rec != nil {
		deadline = rec.LastDeadline()
	}
	ev := RemoteIdLivenessEvent{
		SellerEVM:    m.SellerEVM,
		State:        state,
		Spec005:      spec005,
		Degraded:     s.lastDegraded,
		LastDeadline: deadline,
		At:           m.Now().UTC(),
	}
	select {
	case m.Events <- ev:
		m.Logger.Printf("[liveness:remote-id] sellerEVM=%s state=%s spec005=%s degraded=%v deadline=%d",
			m.SellerEVM, state, spec005, s.lastDegraded, deadline)
	case <-ctx.Done():
	}
	s.lastEmitted = state
}

// mapState projects Spec 005's five-state liveness onto the DApp-level
// surface defined by Spec 017 FR-R21. Stage 3C never alters Spec 005;
// it only re-labels.
func mapState(spec005 health.LivenessState, degraded bool) RemoteIdLivenessState {
	switch spec005 {
	case health.StateAlive:
		if degraded {
			return LivenessDegraded
		}
		return LivenessHealthy
	case health.StateSuspect:
		return LivenessStale
	case health.StateDead, health.StateOffline:
		return LivenessOffline
	default: // StateUnknown
		return LivenessUnknown
	}
}
