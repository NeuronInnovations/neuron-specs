package adsb

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// HeartbeatLoopOptions configures StartHeartbeatLoop.
type HeartbeatLoopOptions struct {
	Key       *keylib.NeuronPrivateKey
	StdOutRef topic.TopicRef
	Adapter   topic.TopicAdapter
	Descriptor ServiceDescriptor
	Interval  time.Duration
	Logger    *log.Logger

	// SellerEVM is REQUIRED — mirrors Spec 017 FR-R21 shape for the ADS-B
	// BaseStation seller. Spec 016 does not yet have an FR-A21 equivalent;
	// the implementation slice ships the FR-R21-shape operational sub-object
	// regardless, per docs/normalized-track-contract.md §9.
	SellerEVM string

	// SellerPeerID is REQUIRED for the same reason as SellerEVM.
	SellerPeerID string

	// ServiceName identifies the DApp commerce service. Defaults to
	// CommerceServiceName ("adsb") when empty.
	ServiceName string

	// TopicBackend / EscrowBackend / AgentURISha256 are operational-disclosure
	// metadata. Empty → omitted from the wire.
	TopicBackend   string
	EscrowBackend  string
	AgentURISha256 string

	// DegradedFunc, when non-nil, is invoked per tick. A true return sets
	// Capabilities.Operational.Degraded=true for that heartbeat.
	DegradedFunc func() bool
}

// HeartbeatLoop is the handle returned by StartHeartbeatLoop.
type HeartbeatLoop struct {
	Done     <-chan struct{}
	sequence *uint64
}

// Sequence returns the last published sequence number.
func (l *HeartbeatLoop) Sequence() uint64 { return atomic.LoadUint64(l.sequence) }

// StartHeartbeatLoop spawns a goroutine that publishes heartbeats every
// Interval, advertising commerceMode / feedSource / profile capability
// disclosures plus the FR-R21-shape operational sub-object.
func StartHeartbeatLoop(ctx context.Context, opts HeartbeatLoopOptions) (*HeartbeatLoop, error) {
	if opts.Key == nil {
		return nil, errors.New("adsb.StartHeartbeatLoop: Key required")
	}
	if opts.Adapter == nil {
		return nil, errors.New("adsb.StartHeartbeatLoop: Adapter required")
	}
	if opts.SellerEVM == "" {
		return nil, errors.New("adsb.StartHeartbeatLoop: SellerEVM required (FR-R21-shape operational disclosure)")
	}
	if opts.SellerPeerID == "" {
		return nil, errors.New("adsb.StartHeartbeatLoop: SellerPeerID required (FR-R21-shape operational disclosure)")
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(discardWriter{}, "", 0)
	}

	serviceName := opts.ServiceName
	if serviceName == "" {
		serviceName = CommerceServiceName
	}

	done := make(chan struct{})
	seq := new(uint64)
	loop := &HeartbeatLoop{Done: done, sequence: seq}

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		publishOnce(ctx, opts, interval, serviceName, seq, logger)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				publishOnce(ctx, opts, interval, serviceName, seq, logger)
			}
		}
	}()

	return loop, nil
}

// publishOnce builds the Capabilities and publishes a single heartbeat.
//
// Phase 5 B2 fix: the nextHeartbeatDeadline must account for HCS
// propagation lag, otherwise observers consistently log
// `[DeadlineInPast] nextHeartbeatDeadline N is not after consensusTimestamp M`
// (Phase 3A/3B limitations.md). Use interval + GracePeriod (= seller's
// stated next-publish-time + observer-grace-window) instead of the floor
// MinDeadlineDelta=10s which is below the typical Hedera HCS round-trip.
func publishOnce(_ context.Context, opts HeartbeatLoopOptions, interval time.Duration, serviceName string, seq *uint64, logger *log.Logger) {
	now := uint64(time.Now().Unix())
	deadlineDelta := uint64(interval/time.Second) + health.GracePeriod
	if deadlineDelta < health.MinDeadlineDelta {
		deadlineDelta = health.MinDeadlineDelta
	}

	degraded := false
	if opts.DegradedFunc != nil {
		degraded = opts.DegradedFunc()
	}
	caps := &health.Capabilities{
		Protocols:    []health.ProtocolID{health.ProtocolID(ProtocolBaseStation)},
		CommerceMode: opts.Descriptor.CommerceMode,
		FeedSource:   opts.Descriptor.FeedSource,
		Profile:      opts.Descriptor.ProfileID,
		Operational: &health.OperationalCapabilities{
			ServiceName:    serviceName,
			SellerEVM:      opts.SellerEVM,
			SellerPeerID:   opts.SellerPeerID,
			TopicBackend:   opts.TopicBackend,
			EscrowBackend:  opts.EscrowBackend,
			AgentURISha256: opts.AgentURISha256,
			Degraded:       degraded,
		},
	}

	payload := health.BuildHeartbeatPayload(
		now+deadlineDelta,
		health.RoleSeller,
		health.WithCapabilities(caps),
	)
	next := atomic.AddUint64(seq, 1)
	if _, err := health.PublishHeartbeat(payload, opts.Key, opts.StdOutRef, opts.Adapter, now, next); err != nil {
		logger.Printf("[heartbeat] publish error seq=%d: %v", next, err)
		atomic.AddUint64(seq, ^uint64(0))
		return
	}
	logger.Printf("[heartbeat] published seq=%d to stdOut topic=%s capabilities={commerceMode:%s feedSource:%s profile:%s} operational={serviceName:%s sellerEVM:%s topicBackend:%s escrowBackend:%s degraded:%v}",
		next, opts.StdOutRef.Locator(),
		caps.CommerceMode, caps.FeedSource, caps.Profile,
		serviceName, opts.SellerEVM, opts.TopicBackend, opts.EscrowBackend, degraded)
}

// discardWriter is the no-op io.Writer used when callers don't supply a logger.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
