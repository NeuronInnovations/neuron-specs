//go:build livefeed

// Live-feed harness — Phase B test against a real BEAST source.
//
// Excluded from the default build (build tag `livefeed`). Depends on a TCP
// source at NEURON_LIVEFEED_TCP (default 127.0.0.1:10003) — typically a
// JetVision Air!Squitter / rcd / dump1090 reachable directly or via SSH
// local-port-forward.
//
// The harness wires (in one process):
//
//	feeds.RunBeastTCP(NEURON_LIVEFEED_TCP)  →  RunSeller (mock bus)
//	                                                 ↓ libp2p QUIC, loopback
//	                                          RunBuyer (mock bus, JSONL → t.Logf)
//
// Real bytes from the device traverse the entire spec-built pipeline (BEAST
// decode → wire envelope → libp2p stream → wire decode → AggregatedFrame).
//
// Run:
//
//	cd impl/golang
//	# open an SSH tunnel to the device first, e.g.:
//	#   ssh -f -N -i <key> -L 127.0.0.1:10003:127.0.0.1:10003 <user>@<device-host>
//	go test -tags livefeed -count=1 -v ./internal/edgeapp/... \
//	    -run TestLiveFeed_FromJVTunnel -timeout 60s
//
//	# Reconnect variant:
//	go test -tags livefeed -count=1 -v ./internal/edgeapp/... \
//	    -run TestLiveFeed_RestartReconnect -timeout 90s
//
// Tunables (all env, all optional):
//
//	NEURON_LIVEFEED_TCP            host:port for feeds.RunBeastTCP (default 127.0.0.1:10003)
//	NEURON_LIVEFEED_DURATION       run length (default 20s)
//	NEURON_LIVEFEED_MIN_FRAMES     pass threshold (default 50)
//	NEURON_LIVEFEED_SAMPLE_EVERY   log every Nth frame as JSONL (default 25)
//	NEURON_LIVEFEED_RESTART_AFTER  for the restart variant (default 6s)

package edgeapp

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveFeedFixture is the shared setup that both live-feed tests build from.
type liveFeedFixture struct {
	cfg     liveFeedConfig
	bus     *MemoryBus
	logger  Logger
	seller  SellerConfig
	buyer   BuyerConfig
	total   *atomic.Uint64 // total frames received
	icaoCnt *atomic.Uint64 // frames whose Meta.ICAO was non-empty
	icaos   *atomicSet     // distinct ICAOs seen
	status  chan livefeedStatus
}

type livefeedStatus struct {
	state SellerState
	when  time.Time
	err   string
}

type liveFeedConfig struct {
	tcpAddr      string
	duration     time.Duration
	minFrames    int
	sampleEvery  int
	restartAfter time.Duration
}

func loadLiveFeedConfig(t *testing.T) liveFeedConfig {
	t.Helper()
	c := liveFeedConfig{
		tcpAddr:      lfEnv("NEURON_LIVEFEED_TCP", "127.0.0.1:10003"),
		duration:     lfDur(t, "NEURON_LIVEFEED_DURATION", "20s"),
		minFrames:    lfInt(t, "NEURON_LIVEFEED_MIN_FRAMES", 50),
		sampleEvery:  lfInt(t, "NEURON_LIVEFEED_SAMPLE_EVERY", 25),
		restartAfter: lfDur(t, "NEURON_LIVEFEED_RESTART_AFTER", "6s"),
	}
	t.Logf("live-feed cfg: tcp=%s duration=%s minFrames=%d sampleEvery=%d restartAfter=%s",
		c.tcpAddr, c.duration, c.minFrames, c.sampleEvery, c.restartAfter)
	return c
}

func lfEnv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func lfDur(t *testing.T, k, def string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(lfEnv(k, def))
	require.NoErrorf(t, err, "parse %s", k)
	return d
}

func lfInt(t *testing.T, k string, def int) int {
	t.Helper()
	if v := os.Getenv(k); v != "" {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		require.NoErrorf(t, err, "parse %s", k)
		return n
	}
	return def
}

// preflightTCP skips the test if the TCP source is unreachable.
func preflightTCP(t *testing.T, addr string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Skipf("live-feed source %s unreachable (%v); open the SSH tunnel first", addr, err)
		return
	}
	_ = conn.Close()
}

func newLiveFeedFixture(t *testing.T) *liveFeedFixture {
	t.Helper()
	c := loadLiveFeedConfig(t)
	preflightTCP(t, c.tcpAddr)

	bus := NewMemoryBus()
	logger := tlogger{t: t}

	sellerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	mkTopic := func(memo string) topic.TopicRef {
		ref, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport(), Memo: memo})
		require.NoError(t, err)
		return ref
	}
	sIn, sOut, sErr := mkTopic("seller-stdIn"), mkTopic("seller-stdOut"), mkTopic("seller-stdErr")
	bIn, bOut, bErr := mkTopic("buyer-stdIn"), mkTopic("buyer-stdOut"), mkTopic("buyer-stdErr")

	sellerPub, err := sellerKey.PublicKey().ToBlockchainKey()
	require.NoError(t, err)

	total := &atomic.Uint64{}
	icaoCnt := &atomic.Uint64{}
	icaos := &atomicSet{}
	status := make(chan livefeedStatus, 64)

	sampleEvery := uint64(c.sampleEvery)
	if sampleEvery == 0 {
		sampleEvery = 1
	}

	onAgg := func(af AggregatedFrame) {
		n := total.Add(1)
		if af.Meta.ICAO != "" {
			icaoCnt.Add(1)
			icaos.add(af.Meta.ICAO)
		}
		if n%sampleEvery == 0 {
			if data, err := json.Marshal(af); err == nil {
				t.Logf("frame#%d: %s", n, string(data))
			}
		}
	}
	onStatus := func(s SellerStatus) {
		select {
		case status <- livefeedStatus{state: s.State, when: time.Now(), err: s.LastError}:
		default:
		}
	}

	// Static seller side — uses the TCP source pointed at the tunnel.
	sellerCfg := SellerConfig{
		Bus:              bus,
		PrivateKey:       &sellerKey,
		StdIn:            sIn,
		StdOut:           sOut,
		StdErr:           sErr,
		LibP2PListenAddr: "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:         DefaultProtocol,
		HeartbeatPeriod:  10 * time.Second,
		FeedSource: func(ctx context.Context, out chan<- feeds.FeedFrame) error {
			return feeds.RunBeastTCP(ctx, c.tcpAddr, out)
		},
		Logger: logger,
	}
	buyerCfg := BuyerConfig{
		Bus:              bus,
		PrivateKey:       &buyerKey,
		StdIn:            bIn,
		StdOut:           bOut,
		StdErr:           bErr,
		Sellers:          []SellerEntry{{StdIn: sIn, PubKey: sellerPub, DisplayName: "jv"}},
		LibP2PListenAddr: "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:         DefaultProtocol,
		RequestID:        "phaseB-jv-001",
		HeartbeatPeriod:  10 * time.Second,
		// Tight reconnect timing for tests (production defaults are 10s/60s).
		ReconnectBackoff:  500 * time.Millisecond,
		SellerDialTimeout: 5 * time.Second,
		OnAggregatedFrame: onAgg,
		OnSellerStatus:    onStatus,
		Logger:            logger,
	}

	return &liveFeedFixture{
		cfg: c, bus: bus, logger: logger,
		seller: sellerCfg, buyer: buyerCfg,
		total: total, icaoCnt: icaoCnt, icaos: icaos, status: status,
	}
}

// TestLiveFeed_FromJVTunnel is the primary Phase B harness — single seller,
// single buyer, real BEAST source.
func TestLiveFeed_FromJVTunnel(t *testing.T) {
	f := newLiveFeedFixture(t)

	// `t.Context()` (Go 1.24+) ties the run to the test lifetime.
	ctx, cancel := context.WithTimeout(t.Context(), f.cfg.duration+5*time.Second)
	defer cancel()

	sellerDone := make(chan error, 1)
	buyerDone := make(chan error, 1)
	go func() { sellerDone <- RunSeller(ctx, f.seller) }()
	time.Sleep(200 * time.Millisecond) // small stagger so seller's stdIn sub is up
	go func() { buyerDone <- RunBuyer(ctx, f.buyer) }()

	// Run for the whole duration (don't break early — keep flowing so the
	// JSONL sample and heartbeat publishes get exercised).
	<-time.After(f.cfg.duration)

	t.Logf("post-run: total=%d icaoFrames=%d uniqueICAOs=%d",
		f.total.Load(), f.icaoCnt.Load(), f.icaos.size())
	t.Logf("sample ICAOs (redacted): %v", redactedICAOSample(f.icaos, 10))

	cancel()
	awaitDone(t, "seller", sellerDone)
	awaitDone(t, "buyer", buyerDone)

	assert.GreaterOrEqualf(t, int(f.total.Load()), f.cfg.minFrames,
		"too few frames in %s — sky may be quiet, retry or extend NEURON_LIVEFEED_DURATION",
		f.cfg.duration)
	assert.Greaterf(t, int(f.icaoCnt.Load()), 0,
		"no DF 11/17/18 frames decoded; a real BEAST stream should produce at least one")
}

// TestLiveFeed_RestartReconnect proves the buyer reconnects to a seller that
// drops and comes back, using real BEAST data.
func TestLiveFeed_RestartReconnect(t *testing.T) {
	f := newLiveFeedFixture(t)

	parentCtx, parentCancel := context.WithTimeout(t.Context(), f.cfg.duration+30*time.Second)
	defer parentCancel()

	// Buyer runs against the parent ctx for the whole test.
	buyerDone := make(chan error, 1)
	go func() { buyerDone <- RunBuyer(parentCtx, f.buyer) }()

	// Phase 1 seller — its own context so we can cancel independently.
	s1Ctx, s1Cancel := context.WithCancel(parentCtx)
	s1Done := make(chan error, 1)
	go func() { s1Done <- RunSeller(s1Ctx, f.seller) }()

	// Wait for restart point.
	select {
	case <-time.After(f.cfg.restartAfter):
	case <-parentCtx.Done():
		t.Fatal("parent ctx cancelled before restartAfter elapsed")
	}
	beforeRestart := f.total.Load()
	t.Logf("phase 1 done at total=%d icaoFrames=%d ; cancelling seller",
		beforeRestart, f.icaoCnt.Load())

	s1Cancel()
	awaitDone(t, "seller-phase1", s1Done)

	// Brief pause so the buyer notices disconnect.
	time.Sleep(2 * time.Second)
	disconnectedAt := f.total.Load()
	t.Logf("after seller drop: total=%d (delta during disconnect=%d)",
		disconnectedAt, disconnectedAt-beforeRestart)

	// Phase 2 — restart with same key + topics.
	s2Ctx, s2Cancel := context.WithCancel(parentCtx)
	s2Done := make(chan error, 1)
	go func() { s2Done <- RunSeller(s2Ctx, f.seller) }()

	// Wait either for the test duration to elapse or for ≥ minFrames/4 new
	// frames after disconnect (whichever first).
	want := uint64(f.cfg.minFrames / 4)
	if want == 0 {
		want = 1
	}
	resumeDeadline := time.After(20 * time.Second)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	resumed := false
resumeLoop:
	for {
		select {
		case <-resumeDeadline:
			break resumeLoop
		case <-tick.C:
			if f.total.Load()-disconnectedAt >= want {
				resumed = true
				break resumeLoop
			}
		}
	}
	finalDelta := f.total.Load() - disconnectedAt

	t.Logf("post-restart: total=%d delta=%d resumed=%v uniqueICAOs=%d",
		f.total.Load(), finalDelta, resumed, f.icaos.size())

	s2Cancel()
	awaitDone(t, "seller-phase2", s2Done)
	parentCancel()
	awaitDone(t, "buyer", buyerDone)

	// Drain status events for the report (non-blocking).
	close(f.status)
	var byState [4]int
	for ev := range f.status {
		switch ev.state {
		case SellerStateConnecting:
			byState[0]++
		case SellerStateConnected:
			byState[1]++
		case SellerStateDisconnected:
			byState[2]++
		case SellerStateError:
			byState[3]++
		}
	}
	t.Logf("status counts: connecting=%d connected=%d disconnected=%d error=%d",
		byState[0], byState[1], byState[2], byState[3])

	assert.True(t, resumed,
		"frames did not resume within 20s of seller restart; reconnect path is broken")
	assert.GreaterOrEqual(t, byState[1], 2,
		"expected at least two 'connected' events (initial + post-restart); got %d", byState[1])
}

// awaitDone fails the test if the given goroutine doesn't return within 5s.
func awaitDone(t *testing.T, label string, ch <-chan error) {
	t.Helper()
	select {
	case err := <-ch:
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			t.Errorf("%s returned unexpected error: %v", label, err)
		}
	case <-time.After(5 * time.Second):
		t.Errorf("%s did not return within 5s of cancel", label)
	}
}

// atomicSet is a tiny lock-free set of strings.
type atomicSet struct {
	m sync.Map
	n atomic.Uint64
}

func (s *atomicSet) add(k string) {
	if _, loaded := s.m.LoadOrStore(k, struct{}{}); !loaded {
		s.n.Add(1)
	}
}

func (s *atomicSet) size() uint64 { return s.n.Load() }

func (s *atomicSet) sample(maxN int) []string {
	out := make([]string, 0, maxN)
	s.m.Range(func(k, _ any) bool {
		out = append(out, k.(string))
		return len(out) < maxN
	})
	return out
}

// redactedICAOSample returns up to maxN ICAOs with the last 2 hex chars
// redacted ("4ca8**"). Keeps logs from publishing specific aircraft IDs.
func redactedICAOSample(s *atomicSet, maxN int) []string {
	raw := s.sample(maxN)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if len(r) >= 6 {
			out = append(out, r[:4]+"**")
		} else {
			out = append(out, r)
		}
	}
	return out
}
