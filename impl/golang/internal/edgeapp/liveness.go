package edgeapp

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// LivenessEvent is what the buyer-side liveness watchdog emits when a
// monitored seller's heartbeat deadline elapses without refresh, or when
// the seller publishes a graceful-shutdown sentinel.
type LivenessEvent struct {
	// SellerEVM is the address of the seller whose state changed.
	SellerEVM string

	// State is the new liveness state per spec 005's five-state machine
	// (UNKNOWN / ALIVE / SUSPECT / DEAD / OFFLINE). Values are taken
	// verbatim from health.LivenessState.
	State health.LivenessState

	// LastDeadline is the most recently observed nextHeartbeatDeadline
	// from this seller (Unix seconds), or 0 if no valid heartbeat has
	// been observed yet.
	LastDeadline uint64

	// At is when the watchdog detected the transition (wall clock UTC).
	At time.Time
}

// LivenessTracker watches a single seller's heartbeat stream and emits
// a LivenessEvent every time the seller's evaluated state crosses from
// ALIVE into SUSPECT, DEAD, or OFFLINE — and back into ALIVE on recovery.
//
// The tracker is opt-in at the buyer level (BuyerConfig.EnforceDeadlines).
// Without the flag, the buyer behaves exactly as in Phase C.2 — observes
// nothing on the seller's stdOut and relies on libp2p stream death to
// detect seller loss.
//
// Internals: each tracker wraps a health.HeartbeatObserver (which already
// implements V-OBS-01..06 validation + the LivenessRecord state machine)
// and exposes a thin watchdog ticker on top. The caller owns the
// MessageDelivery channel — subscription, multi-topic fan-in, and
// backend-specific consensusTimestamp normalization (e.g. HCS-nanos →
// seconds) all live above this type.
//
// Concurrency: Run blocks until ctx is cancelled OR the input channel
// is closed. The Events channel is closed exactly once on Run's return.
type LivenessTracker struct {
	// SellerEVM is the address of the monitored seller. Used to scope
	// observer state to a specific publisher; deliveries whose recovered
	// signer doesn't match are still validated but ignored for
	// state-transition purposes. Required.
	SellerEVM string

	// Deliveries is the channel of TopicMessage deliveries the caller
	// produces — typically by Subscribe'ing to the seller's stdOut.
	// Required. Closing it is one of two ways to stop Run; the other
	// is ctx cancellation.
	//
	// Each delivery's ConsensusTimestamp must be in **seconds** matching
	// the heartbeat's NextHeartbeatDeadline scale. Callers stamping in
	// other units (e.g. HCS nanoseconds) must normalize before forwarding.
	Deliveries <-chan topic.MessageDelivery

	// PollInterval is how often the watchdog re-evaluates liveness state
	// against Now() so degradation transitions (ALIVE→SUSPECT etc.) fire
	// even when no new heartbeat arrives. Defaults to
	// DefaultLivenessPollInterval. The watchdog also re-evaluates on
	// every delivery, so recovery transitions (SUSPECT→ALIVE) are
	// observed promptly without waiting for a tick.
	PollInterval time.Duration

	// Now, when non-nil, replaces time.Now for the watchdog's "current
	// time" computation. Returns wall-clock time; only Unix() seconds
	// are consulted. Tests inject a controllable clock; production
	// callers leave it nil.
	Now func() time.Time

	// Logger receives diagnostic info-level events (one per state
	// transition + one per validation failure). Optional.
	Logger Logger

	// Events is the channel the tracker writes LivenessEvents to. Must
	// be set by the caller (suggested capacity: 4). Closed by Run.
	Events chan<- LivenessEvent
}

// DefaultLivenessPollInterval governs how often the watchdog ticks
// when LivenessTracker.PollInterval is unset.
const DefaultLivenessPollInterval = 5 * time.Second

// validate normalizes defaults and rejects fatal misconfiguration.
func (t *LivenessTracker) validate() error {
	if t == nil {
		return errors.New("liveness: nil tracker")
	}
	if t.SellerEVM == "" {
		return errors.New("liveness: SellerEVM required")
	}
	if t.Deliveries == nil {
		return errors.New("liveness: Deliveries channel required")
	}
	if t.Events == nil {
		return errors.New("liveness: Events channel required")
	}
	if t.PollInterval <= 0 {
		t.PollInterval = DefaultLivenessPollInterval
	}
	if t.Now == nil {
		t.Now = time.Now
	}
	if t.Logger == nil {
		t.Logger = nopLogger{}
	}
	return nil
}

// Run blocks until ctx is cancelled or Deliveries is closed, processing
// each delivery through the spec-005 V-OBS-01..06 pipeline and emitting
// LivenessEvents on transitions out of (or back into) ALIVE. Run closes
// the Events channel exactly once on return.
//
// Returns nil for both clean termination paths.
func (t *LivenessTracker) Run(ctx context.Context) error {
	if err := t.validate(); err != nil {
		return err
	}
	defer close(t.Events)

	observer := health.NewHeartbeatObserver()
	var lastEmitted health.LivenessState // throttle: emit only on transition

	tick := time.NewTicker(t.PollInterval)
	defer tick.Stop()

	emit := func(state health.LivenessState) {
		// Re-fetch the record so LastDeadline reflects the latest update.
		var deadline uint64
		if rec := observer.GetRecord(t.SellerEVM); rec != nil {
			deadline = rec.LastDeadline()
		}
		ev := LivenessEvent{
			SellerEVM:    t.SellerEVM,
			State:        state,
			LastDeadline: deadline,
			At:           t.Now().UTC(),
		}
		select {
		case t.Events <- ev:
			t.Logger.Printf("[liveness] %s -> %s (deadline=%d)", t.SellerEVM, state, deadline)
		case <-ctx.Done():
		}
		lastEmitted = state
	}

	// Lock guards observer + lastEmitted across the subscribe and tick
	// goroutines. The wrapped HeartbeatObserver is itself safe for
	// concurrent use, but we want a coherent (record, evaluatedState)
	// pair when emitting.
	var mu sync.Mutex

	processDelivery := func(d topic.MessageDelivery) {
		mu.Lock()
		defer mu.Unlock()
		_, err := observer.ProcessDelivery(d)
		if err != nil {
			t.Logger.Printf("[liveness] %s dropped invalid heartbeat: %v",
				t.SellerEVM, err)
			return
		}
		// Re-evaluate immediately on each delivery so a recovery from
		// SUSPECT -> ALIVE is observed promptly and we don't keep firing
		// events while ALIVE.
		nowSec := uint64(t.Now().Unix())
		state := observer.GetLivenessState(t.SellerEVM, nowSec)
		if state != lastEmitted && state != health.StateUnknown && shouldEmit(state, lastEmitted) {
			// We bypass the "transition into bad state" guard for OFFLINE,
			// which always emits. SUSPECT/DEAD also emit; ALIVE emits when
			// transitioning from a degraded state (recovery).
			emit(state)
		}
	}

	tickEvaluate := func() {
		mu.Lock()
		defer mu.Unlock()
		nowSec := uint64(t.Now().Unix())
		state := observer.GetLivenessState(t.SellerEVM, nowSec)
		if state != lastEmitted && state != health.StateUnknown && shouldEmit(state, lastEmitted) {
			emit(state)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-t.Deliveries:
			if !ok {
				// Caller closed the input; treat as graceful shutdown.
				return nil
			}
			processDelivery(d)
		case <-tick.C:
			tickEvaluate()
		}
	}
}

// shouldEmit returns true when the new state is one we want the buyer to
// react to. We always want to surface: SUSPECT, DEAD, OFFLINE (degradation).
// We also surface: ALIVE iff the previous emitted state was a degraded
// one (recovery). UNKNOWN is never emitted (callers know nothing has been
// observed; it's the resting state before the first heartbeat).
func shouldEmit(next, last health.LivenessState) bool {
	switch next {
	case health.StateSuspect, health.StateDead, health.StateOffline:
		return true
	case health.StateAlive:
		return last == health.StateSuspect || last == health.StateDead
	default:
		return false
	}
}

// startSellerLivenessWatch is the buyer-side wiring used by runSellerWorker
// when BuyerConfig.EnforceDeadlines is set. It subscribes to rs.entry.StdOut,
// normalizes each delivery's ConsensusTimestamp from the bus's native scale
// (Unix nanoseconds, both for HCS and MemoryBus) to seconds (which is what
// the spec-005 deadline arithmetic expects), pumps the deliveries into a
// LivenessTracker, and on the first non-ALIVE event invokes onStale to
// terminate the active stream.
//
// The returned channel signals when the watcher goroutine has fully exited
// — caller waits on it after stream teardown so the next iteration starts
// with a clean subscriber list.
func startSellerLivenessWatch(
	ctx context.Context,
	bus topic.TopicAdapter,
	rs *resolvedSeller,
	onStale func(),
	logger Logger,
) chan error {
	done := make(chan error, 1)

	go func() {
		defer func() { done <- nil }()

		raw, err := bus.Subscribe(ctx, rs.entry.StdOut, topic.SubscribeOpts{})
		if err != nil {
			logger.Printf("[liveness:%s] subscribe failed: %v", rs.displayName, err)
			return
		}

		// Normalize nano-stamped deliveries to seconds.
		normalized := make(chan topic.MessageDelivery, 8)
		go func() {
			defer close(normalized)
			for d := range raw {
				d.ConsensusTimestamp = consensusToSeconds(d.ConsensusTimestamp)
				select {
				case <-ctx.Done():
					return
				case normalized <- d:
				}
			}
		}()

		events := make(chan LivenessEvent, 4)
		tr := &LivenessTracker{
			SellerEVM:  rs.evm,
			Deliveries: normalized,
			Events:     events,
			Logger:     logger,
		}

		trackerDone := make(chan error, 1)
		go func() { trackerDone <- tr.Run(ctx) }()

		// Forward the first stale event to the per-stream cancel and keep
		// draining (the tracker will close events on ctx-done).
		stale := false
		for ev := range events {
			if !stale && ev.State != health.StateAlive {
				stale = true
				logger.Printf("[liveness:%s] stale (%s); closing stream",
					rs.displayName, ev.State)
				onStale()
			}
		}
		<-trackerDone
	}()

	return done
}

// consensusToSeconds normalizes a backend-stamped consensus timestamp to
// Unix seconds. Both HCSAdapter and MemoryBus stamp nanoseconds; values
// that look like nanoseconds (>= 1e15, i.e. roughly post-2001 in ns) are
// scaled, others passed through. The threshold is well below the
// smallest plausible nanosecond timestamp from any chain we deal with
// (MemoryBus uses time.Now().UnixNano()), and well above the largest
// plausible Unix-seconds value, so the heuristic is safe for any near-
// future time.
func consensusToSeconds(ts uint64) uint64 {
	const nanoThreshold = uint64(1_000_000_000_000_000) // 1e15
	if ts >= nanoThreshold {
		return ts / 1_000_000_000
	}
	return ts
}
