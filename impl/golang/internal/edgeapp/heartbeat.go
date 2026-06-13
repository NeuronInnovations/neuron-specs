package edgeapp

import (
	"context"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

type heartbeatConfig struct {
	Role       health.NodeRole
	Bus        topic.TopicAdapter
	Key        *keylib.NeuronPrivateKey
	StdOut     topic.TopicRef
	Period     time.Duration
	Reachable  bool
	Location   *health.Location
	ProtocolID string
	Logger     Logger
}

// runHeartbeat publishes a spec-005 envelope to stdOut every Period until ctx
// is cancelled. The first heartbeat is sent immediately so observers see
// liveness without waiting a full period.
//
// On graceful shutdown, runHeartbeat publishes a final ShutdownSentinel
// payload (deadline=0) to signal liveness termination per FR-H10.
func runHeartbeat(ctx context.Context, cfg heartbeatConfig) {
	if cfg.Logger == nil {
		cfg.Logger = nopLogger{}
	}

	publisher := health.NewHeartbeatPublisher(cfg.Key, cfg.StdOut, cfg.Bus)

	emit := func(deadline uint64) {
		now := uint64(time.Now().Unix())
		caps := &health.Capabilities{
			NATReachability: cfg.Reachable,
			NATType:         "",
			Protocols:       []health.ProtocolID{health.ProtocolID(cfg.ProtocolID)},
		}
		payload := health.BuildHeartbeatPayload(deadline, cfg.Role,
			health.WithCapabilities(caps),
			health.WithLocation(cfg.Location),
		)
		if _, err := publisher.Publish(payload, now); err != nil {
			cfg.Logger.Printf("[heartbeat] publish failed (role=%s): %v", cfg.Role, err)
			return
		}
		cfg.Logger.Printf("[heartbeat] %s deadline=+%ds reachable=%v",
			cfg.Role, deadline-now, cfg.Reachable)
	}

	// First heartbeat immediately.
	periodSec := uint64(cfg.Period.Seconds())
	if periodSec < health.MinDeadlineDelta {
		periodSec = health.MinDeadlineDelta
	}
	now := uint64(time.Now().Unix())
	emit(now + periodSec)

	ticker := time.NewTicker(cfg.Period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown sentinel. Use the lower-level PublishHeartbeat
			// to bypass HeartbeatPublisher's rate limiter — the sentinel is
			// exempt from delta checks (V-PUB-03) but the rate limiter is not
			// aware of that exemption, so we go around it.
			payload := health.BuildHeartbeatPayload(health.ShutdownSentinel, cfg.Role)
			if _, err := health.PublishHeartbeat(payload, cfg.Key, cfg.StdOut, cfg.Bus,
				uint64(time.Now().Unix()), shutdownSequence(time.Now())); err != nil {
				cfg.Logger.Printf("[heartbeat] shutdown sentinel publish failed: %v", err)
			} else {
				cfg.Logger.Printf("[heartbeat] %s shutdown sentinel published", cfg.Role)
			}
			return
		case <-ticker.C:
			n := uint64(time.Now().Unix())
			emit(n + periodSec)
		}
	}
}

// shutdownSequence picks a sequence number for the shutdown sentinel that is
// guaranteed not to collide with regular heartbeat sequences from the same
// process — the regular publisher uses ints starting at 1; we use the
// current Unix-nano so it's always larger.
func shutdownSequence(now time.Time) uint64 { return uint64(now.UnixNano()) }
