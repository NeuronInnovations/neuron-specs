package remoteid

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
	// Key signs every heartbeat. Required.
	Key *keylib.NeuronPrivateKey

	// StdOutRef is the seller's stdOut topic ref where heartbeats are
	// published. Required.
	StdOutRef topic.TopicRef

	// Adapter is the underlying TopicAdapter (memory in tests, HCS in
	// production). Required.
	Adapter topic.TopicAdapter

	// Descriptor carries the FR-P58 / FR-R15 / FR-F-02 capability values
	// that the heartbeat advertises. Required.
	Descriptor ServiceDescriptor

	// Interval between heartbeats. Defaults to 5 seconds. Tests pass
	// 50ms; production deployments SHOULD use >= 10s to match the spec
	// 005 MinDeadlineDelta. The loop bypasses the rate-limited
	// HeartbeatPublisher wrapper so fast ticks remain legal.
	Interval time.Duration

	// Logger receives one `[heartbeat] published seq=N …` line per tick.
	// Optional; if nil, a no-op logger is used.
	Logger *log.Logger

	// SellerEVM is the seller's EVM address (hex, 0x-prefixed). Required
	// by Spec 017 FR-R21 (Stage 3C operational disclosure). StartHeartbeatLoop
	// rejects empty values.
	SellerEVM string

	// SellerPeerID is the seller's libp2p PeerID. Required by Spec 017
	// FR-R21. StartHeartbeatLoop rejects empty values.
	SellerPeerID string

	// ServiceName identifies the DApp commerce service. Defaults to
	// CommerceServiceName ("remote-id") when empty. Surfaced via
	// Capabilities.Operational.ServiceName on every heartbeat (FR-R21).
	ServiceName string

	// TopicBackend identifies the topic transport ("memory" | "hcs"),
	// surfaced via Capabilities.Operational.TopicBackend (FR-R21). Empty
	// → omitted from the wire.
	TopicBackend string

	// EscrowBackend identifies the escrow backend ("memory" | "evm"),
	// surfaced via Capabilities.Operational.EscrowBackend (FR-R21). Empty
	// → omitted from the wire.
	EscrowBackend string

	// AgentURISha256 is the hex SHA-256 of the seller's registered
	// AgentURI JSON. Surfaced via Capabilities.Operational.AgentURISha256
	// (FR-R21). Empty → omitted from the wire.
	AgentURISha256 string

	// DegradedFunc, if non-nil, is invoked at the start of every publish
	// tick. A true return populates Capabilities.Operational.Degraded=true
	// for that heartbeat. Nil → degraded is never advertised. Stage 3C
	// production seller wires this to `func() bool { return false }`;
	// Stage 3D wires it to real feed / stream health signals.
	DegradedFunc func() bool
}

// HeartbeatLoop is the handle returned by StartHeartbeatLoop. Done closes
// when the loop has stopped (after ctx cancellation or fatal publish
// error). Sequence returns the last sequence number published.
type HeartbeatLoop struct {
	Done     <-chan struct{}
	sequence *uint64
}

// Sequence returns the last published sequence number.
func (l *HeartbeatLoop) Sequence() uint64 { return atomic.LoadUint64(l.sequence) }

// StartHeartbeatLoop spawns a goroutine that publishes heartbeats every
// Interval, carrying the descriptor's commerceMode / feedSource / profile
// capability disclosures (008 FR-P58, 017 FR-R15, 013 FR-F-02) plus the
// Stage 3C operational disclosure block (Spec 017 FR-R21). The goroutine
// exits when ctx is cancelled.
//
// FR-R21 requires SellerEVM and SellerPeerID to be non-empty; both anchor
// the seller identity surfaced in the heartbeat's Capabilities.Operational.
func StartHeartbeatLoop(ctx context.Context, opts HeartbeatLoopOptions) (*HeartbeatLoop, error) {
	if opts.Key == nil {
		return nil, errors.New("remoteid.StartHeartbeatLoop: Key required")
	}
	if opts.Adapter == nil {
		return nil, errors.New("remoteid.StartHeartbeatLoop: Adapter required")
	}
	if opts.SellerEVM == "" {
		return nil, errors.New("remoteid.StartHeartbeatLoop: SellerEVM required (Spec 017 FR-R21 — operational disclosure)")
	}
	if opts.SellerPeerID == "" {
		return nil, errors.New("remoteid.StartHeartbeatLoop: SellerPeerID required (Spec 017 FR-R21 — operational disclosure)")
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
		// Publish first heartbeat immediately so subscribers see one
		// without waiting a full interval.
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

// publishOnce builds the Capabilities (including a fresh Operational
// block — DegradedFunc is re-evaluated per tick) and publishes a single
// heartbeat. Capabilities are constructed per-call so a flapping
// DegradedFunc produces alternating wire values.
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
		Protocols:    []health.ProtocolID{health.ProtocolID(ProtocolRaw)},
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

	// Phase 5 B2: deadline = now + (interval + GracePeriod) accounts for
	// HCS propagation lag; eliminates spurious [DeadlineInPast] warnings.
	payload := health.BuildHeartbeatPayload(
		now+deadlineDelta,
		health.RoleSeller,
		health.WithCapabilities(caps),
	)
	next := atomic.AddUint64(seq, 1)
	if _, err := health.PublishHeartbeat(payload, opts.Key, opts.StdOutRef, opts.Adapter, now, next); err != nil {
		logger.Printf("[heartbeat] publish error seq=%d: %v", next, err)
		atomic.AddUint64(seq, ^uint64(0)) // decrement: AddUint64(x, ^0) == subtract 1
		return
	}
	logger.Printf("[heartbeat] published seq=%d to stdOut topic=%s capabilities={commerceMode:%s feedSource:%s profile:%s} operational={serviceName:%s sellerEVM:%s topicBackend:%s escrowBackend:%s degraded:%v}",
		next, opts.StdOutRef.Locator(),
		caps.CommerceMode, caps.FeedSource, caps.Profile,
		serviceName, opts.SellerEVM, opts.TopicBackend, opts.EscrowBackend, degraded)
}

// discardWriter is the no-op io.Writer used when callers don't supply
// a logger.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
