package adsb

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// AdsbLivenessState is the DApp-level liveness surface returned to an ADS-B
// BaseStation buyer. Mirrors internal/dapp/remoteid.RemoteIdLivenessState
// with the {Unknown / Healthy / Stale / Offline / Degraded} vocabulary.
type AdsbLivenessState string

const (
	LivenessUnknown  AdsbLivenessState = "Unknown"
	LivenessHealthy  AdsbLivenessState = "Healthy"
	LivenessStale    AdsbLivenessState = "Stale"
	LivenessOffline  AdsbLivenessState = "Offline"
	LivenessDegraded AdsbLivenessState = "Degraded"
)

// AdsbLivenessEvent is what the monitor emits on a state transition.
type AdsbLivenessEvent struct {
	SellerEVM    string
	State        AdsbLivenessState
	Spec005      health.LivenessState
	Degraded     bool
	LastDeadline uint64
	At           time.Time
}

// DefaultLivenessPollInterval governs how often the watchdog ticks when
// AdsbLivenessMonitor.PollInterval is unset.
const DefaultLivenessPollInterval = 5 * time.Second

// AdsbLivenessMonitor watches a single ADS-B BaseStation seller's heartbeat
// stream and emits an AdsbLivenessEvent on every DApp-level state
// transition. Mirrors internal/dapp/remoteid.RemoteIdLivenessMonitor.
type AdsbLivenessMonitor struct {
	SellerEVM              string
	Deliveries             <-chan topic.MessageDelivery
	PollInterval           time.Duration
	Now                    func() time.Time
	Logger                 *log.Logger
	Events                 chan<- AdsbLivenessEvent
	ExpectedAgentURISha256 string
}

func (m *AdsbLivenessMonitor) validate() error {
	if m == nil {
		return errors.New("adsb.AdsbLivenessMonitor: nil monitor")
	}
	if m.SellerEVM == "" {
		return errors.New("adsb.AdsbLivenessMonitor: SellerEVM required")
	}
	if m.Deliveries == nil {
		return errors.New("adsb.AdsbLivenessMonitor: Deliveries channel required")
	}
	if m.Events == nil {
		return errors.New("adsb.AdsbLivenessMonitor: Events channel required")
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

type adsbMonitorState struct {
	mu           sync.Mutex
	observer     *health.HeartbeatObserver
	lastEmitted  AdsbLivenessState
	lastDegraded bool
}

// Run blocks until ctx is cancelled or Deliveries is closed, processing
// each delivery and emitting AdsbLivenessEvents on transitions.
func (m *AdsbLivenessMonitor) Run(ctx context.Context) error {
	if err := m.validate(); err != nil {
		return err
	}
	defer close(m.Events)

	state := &adsbMonitorState{
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

func (m *AdsbLivenessMonitor) processDelivery(ctx context.Context, s *adsbMonitorState, d topic.MessageDelivery) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload, err := s.observer.ProcessDelivery(d)
	if err != nil {
		m.Logger.Printf("[liveness:adsb] sellerEVM=%s dropped invalid heartbeat: %v",
			m.SellerEVM, err)
		return
	}

	if m.ExpectedAgentURISha256 != "" && payload.Capabilities != nil && payload.Capabilities.Operational != nil {
		adv := payload.Capabilities.Operational.AgentURISha256
		if adv != "" && adv != m.ExpectedAgentURISha256 {
			m.Logger.Printf("[liveness:adsb] evidence-grade mismatch: registered=%s advertised=%s",
				m.ExpectedAgentURISha256, adv)
		}
	}

	s.lastDegraded = false
	if payload.Capabilities != nil && payload.Capabilities.Operational != nil {
		s.lastDegraded = payload.Capabilities.Operational.Degraded
	}

	m.evaluateLocked(ctx, s)
}

func (m *AdsbLivenessMonitor) tickEvaluate(ctx context.Context, s *adsbMonitorState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.evaluateLocked(ctx, s)
}

func (m *AdsbLivenessMonitor) evaluateLocked(ctx context.Context, s *adsbMonitorState) {
	nowSec := uint64(m.Now().Unix())
	spec005 := s.observer.GetLivenessState(m.SellerEVM, nowSec)
	next := mapAdsbState(spec005, s.lastDegraded)
	if next == LivenessUnknown || next == s.lastEmitted {
		return
	}
	m.emit(ctx, s, next, spec005)
}

func (m *AdsbLivenessMonitor) emit(ctx context.Context, s *adsbMonitorState, state AdsbLivenessState, spec005 health.LivenessState) {
	var deadline uint64
	if rec := s.observer.GetRecord(m.SellerEVM); rec != nil {
		deadline = rec.LastDeadline()
	}
	ev := AdsbLivenessEvent{
		SellerEVM:    m.SellerEVM,
		State:        state,
		Spec005:      spec005,
		Degraded:     s.lastDegraded,
		LastDeadline: deadline,
		At:           m.Now().UTC(),
	}
	select {
	case m.Events <- ev:
		m.Logger.Printf("[liveness:adsb] sellerEVM=%s state=%s spec005=%s degraded=%v deadline=%d",
			m.SellerEVM, state, spec005, s.lastDegraded, deadline)
	case <-ctx.Done():
	}
	s.lastEmitted = state
}

// mapAdsbState projects Spec 005's five-state liveness onto the DApp-level
// surface. Mirrors internal/dapp/remoteid.mapState.
func mapAdsbState(spec005 health.LivenessState, degraded bool) AdsbLivenessState {
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
	default:
		return LivenessUnknown
	}
}
