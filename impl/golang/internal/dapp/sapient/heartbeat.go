package sapient

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

// Heartbeat tuning + disclosure constants for the SAPIENT RID seller.
const (
	// DefaultHeartbeatInterval is the spec-005 publish cadence when the caller
	// does not override it. 15s sits comfortably above MinDeadlineDelta (10s)
	// and below typical observer suspect windows.
	DefaultHeartbeatInterval = 15 * time.Second

	// ProfileSAPIENTRID is the connectivity-profile id (013 FR-F-02) advertised
	// in the heartbeat's Capabilities.Profile — the SAPIENT RID seller posture.
	ProfileSAPIENTRID = "sapient-rid"

	// ProfileSAPIENTADSB is the JetVision ADS-B seller posture
	// (cmd/sapient-jv-seller; sibling of ProfileSAPIENTRID).
	ProfileSAPIENTADSB = "sapient-adsb"
)

// HeartbeatOptions configures StartHeartbeatLoop. It mirrors the remoteid loop
// but advertises the SAPIENT detection protocol and the ASM node_id.
type HeartbeatOptions struct {
	// Key signs every heartbeat. Required.
	Key *keylib.NeuronPrivateKey

	// StdOutRef is the seller's stdOut topic ref — the single liveness lane the
	// registry advertises, SHARED with the SAPIENT control lane (the audit
	// TopicLane demuxes by payload). Required.
	StdOutRef topic.TopicRef

	// Adapter is the underlying TopicAdapter (memory in tests, HCS in
	// production). Required.
	Adapter topic.TopicAdapter

	// Interval between heartbeats. Defaults to DefaultHeartbeatInterval. The
	// loop bypasses the rate-limited HeartbeatPublisher wrapper, so faster test
	// ticks stay legal.
	Interval time.Duration

	// Logger receives one line per published heartbeat. Optional.
	Logger *log.Logger

	// SellerEVM / SellerPeerID anchor the publisher identity in the operational
	// disclosure block (parity with Spec 017 FR-R21). Both required.
	SellerEVM    string
	SellerPeerID string

	// ASMNodeID is the SAPIENT ASM node_id (NodeIDFromIdentity) — the UUID a
	// Task addresses and the identity bound in the agent card. Required;
	// surfaced via Capabilities.Operational.ASMNodeID (015 FR-S94).
	ASMNodeID string

	// FeedSource advertises data-plane provenance (017 FR-R-E02): live | replay
	// | synthetic | placeholder. Empty → omitted (observer default "live").
	FeedSource string

	// CommerceMode advertises the 008 posture (off | full). Empty → omitted.
	CommerceMode string

	// TopicBackend identifies the audit-lane/heartbeat transport ("memory" |
	// "hcs"), surfaced via Capabilities.Operational.TopicBackend. Empty →
	// omitted.
	TopicBackend string

	// AgentURISha256 is the hex SHA-256 of the seller's registered AgentURI
	// JSON, surfaced for defence-in-depth cross-check. Empty → omitted.
	AgentURISha256 string

	// ServiceName overrides the advertised commerce service. Defaults to
	// CommerceServiceName ("rid").
	ServiceName string

	// Profile overrides the connectivity-profile id advertised in
	// Capabilities.Profile. Defaults to ProfileSAPIENTRID (live-path
	// behavior); the JV seller passes ProfileSAPIENTADSB.
	Profile string

	// DegradedFunc, if non-nil, is invoked at the start of every tick; a true
	// return sets Capabilities.Operational.Degraded for that heartbeat (e.g.
	// the bridge feed has gone silent). Nil → never degraded.
	DegradedFunc func() bool
}

// HeartbeatLoop is the handle returned by StartHeartbeatLoop. Done closes when
// the loop stops (after ctx cancellation). Sequence returns the last sequence
// number published.
type HeartbeatLoop struct {
	Done     <-chan struct{}
	sequence *uint64
}

// Sequence returns the last published sequence number.
func (l *HeartbeatLoop) Sequence() uint64 { return atomic.LoadUint64(l.sequence) }

// StartHeartbeatLoop spawns a goroutine publishing spec-005 heartbeats every
// Interval on the seller's stdOut topic, carrying the SAPIENT detection protocol,
// feedSource/commerceMode/profile disclosures, and the operational block
// (sellerEVM, sellerPeerID, asmNodeId, topicBackend, agentURISha256, degraded).
// The first beat is immediate; on ctx cancellation a graceful OFFLINE sentinel
// (deadline 0, FR-H10/H12) is published best-effort before the goroutine exits.
func StartHeartbeatLoop(ctx context.Context, opts HeartbeatOptions) (*HeartbeatLoop, error) {
	if opts.Key == nil {
		return nil, errors.New("sapient.StartHeartbeatLoop: Key required")
	}
	if opts.Adapter == nil {
		return nil, errors.New("sapient.StartHeartbeatLoop: Adapter required")
	}
	if opts.SellerEVM == "" {
		return nil, errors.New("sapient.StartHeartbeatLoop: SellerEVM required")
	}
	if opts.SellerPeerID == "" {
		return nil, errors.New("sapient.StartHeartbeatLoop: SellerPeerID required")
	}
	if opts.ASMNodeID == "" {
		return nil, errors.New("sapient.StartHeartbeatLoop: ASMNodeID required (015 FR-S94)")
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
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
		publishHeartbeatOnce(opts, interval, serviceName, seq, logger)
		for {
			select {
			case <-ctx.Done():
				publishShutdownSentinel(opts, serviceName, seq, logger)
				return
			case <-ticker.C:
				publishHeartbeatOnce(opts, interval, serviceName, seq, logger)
			}
		}
	}()
	return loop, nil
}

// sapientCapabilities builds a fresh Capabilities block (DegradedFunc re-evaluated
// per call so a flapping bridge produces alternating wire values).
func sapientCapabilities(opts HeartbeatOptions, serviceName string) (*health.Capabilities, bool) {
	degraded := false
	if opts.DegradedFunc != nil {
		degraded = opts.DegradedFunc()
	}
	profile := opts.Profile
	if profile == "" {
		profile = ProfileSAPIENTRID
	}
	return &health.Capabilities{
		Protocols:    []health.ProtocolID{health.ProtocolID(ProtocolDetection)},
		CommerceMode: opts.CommerceMode,
		FeedSource:   opts.FeedSource,
		Profile:      profile,
		Operational: &health.OperationalCapabilities{
			ServiceName:    serviceName,
			SellerEVM:      opts.SellerEVM,
			SellerPeerID:   opts.SellerPeerID,
			ASMNodeID:      opts.ASMNodeID,
			TopicBackend:   opts.TopicBackend,
			AgentURISha256: opts.AgentURISha256,
			Degraded:       degraded,
		},
	}, degraded
}

// publishHeartbeatOnce builds and publishes one live heartbeat. Deadline =
// now + interval + GracePeriod, accounting for HCS propagation lag (the remoteid
// Phase-5 B2 fix) so observers don't log spurious DeadlineInPast warnings.
func publishHeartbeatOnce(opts HeartbeatOptions, interval time.Duration, serviceName string, seq *uint64, logger *log.Logger) {
	now := uint64(time.Now().Unix())
	deadlineDelta := max(uint64(interval/time.Second)+health.GracePeriod, health.MinDeadlineDelta)
	caps, degraded := sapientCapabilities(opts, serviceName)
	payload := health.BuildHeartbeatPayload(now+deadlineDelta, health.RoleSeller, health.WithCapabilities(caps))
	next := atomic.AddUint64(seq, 1)
	if _, err := health.PublishHeartbeat(payload, opts.Key, opts.StdOutRef, opts.Adapter, now, next); err != nil {
		logger.Printf("[heartbeat] publish error seq=%d: %v", next, err)
		atomic.AddUint64(seq, ^uint64(0)) // decrement on failure
		return
	}
	logger.Printf("[heartbeat] published seq=%d stdOut=%s service=%s protocol=%s feedSource=%s topicBackend=%s degraded=%v",
		next, opts.StdOutRef.Locator(), serviceName, ProtocolDetection, opts.FeedSource, opts.TopicBackend, degraded)
}

// publishShutdownSentinel publishes a deadline-0 heartbeat (graceful OFFLINE,
// FR-H10/H12) so observers distinguish a clean stop from a crash. Best-effort.
func publishShutdownSentinel(opts HeartbeatOptions, serviceName string, seq *uint64, logger *log.Logger) {
	caps, _ := sapientCapabilities(opts, serviceName)
	payload := health.BuildHeartbeatPayload(health.ShutdownSentinel, health.RoleSeller, health.WithCapabilities(caps))
	next := atomic.AddUint64(seq, 1)
	if _, err := health.PublishHeartbeat(payload, opts.Key, opts.StdOutRef, opts.Adapter, uint64(time.Now().Unix()), next); err != nil {
		logger.Printf("[heartbeat] shutdown sentinel publish error: %v", err)
		return
	}
	logger.Printf("[heartbeat] shutdown sentinel published (OFFLINE) seq=%d", next)
}

// discardWriter is the no-op io.Writer used when callers don't supply a logger.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
