package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
)

func TestSessionRegistry_Lifecycle(t *testing.T) {
	t.Parallel()
	reg := newSessionRegistry()

	id1 := reg.openSession("peer-A", "/ip4/127.0.0.1/udp/1/quic-v1")
	id2 := reg.openSession("peer-B", "/ip4/127.0.0.1/udp/2/quic-v1")
	require.NotEqual(t, id1, id2)

	t0 := time.Now()
	reg.observe(id1, "node-A", t0)
	reg.observe(id1, "node-A-changed", t0.Add(time.Second)) // first non-empty node_id wins
	reg.observe(id2, "", t0)                                // empty node_id never overwrites

	snap := reg.snapshot()
	require.Len(t, snap, 2)
	byPeer := map[string]sessionInfo{}
	for _, s := range snap {
		byPeer[s.PeerID] = s
	}
	a := byPeer["peer-A"]
	assert.Equal(t, "node-A", a.NodeID, "node_id captured from the first message and pinned")
	assert.Equal(t, uint64(2), a.MessageCount)
	assert.Equal(t, t0.Add(time.Second).Unix(), a.LastSeen.Unix())
	b := byPeer["peer-B"]
	assert.Empty(t, b.NodeID)
	assert.Equal(t, uint64(1), b.MessageCount)

	reg.closeSession(id1)
	snap = reg.snapshot()
	require.Len(t, snap, 1, "closed sessions leave the open set")
	assert.Equal(t, "peer-B", snap[0].PeerID)

	opened, closed, msgs := reg.totals()
	assert.Equal(t, uint64(2), opened)
	assert.Equal(t, uint64(1), closed)
	assert.Equal(t, uint64(3), msgs)
}

// pushSession opens one stream via dial and pushes msgs, re-stamped with
// nodeID (the seller re-stamp contract), then half-closes.
func pushSession(t *testing.T, ctx context.Context, dial func(context.Context) (libp2pnetwork.Stream, error), nodeID string, msgs []*sapientpb.SapientMessage) {
	t.Helper()
	stream, err := dial(ctx)
	require.NoError(t, err)
	w := delivery.NewFrameWriter(stream)
	for _, msg := range msgs {
		m := proto.Clone(msg).(*sapientpb.SapientMessage)
		m.NodeId = proto.String(nodeID)
		b, merr := proto.Marshal(m)
		require.NoError(t, merr)
		require.NoError(t, w.WriteFrame(b))
	}
	require.NoError(t, stream.CloseWrite())
}

// TestMultiSellerConcurrentForwarding: two sellers with distinct node_ids push
// concurrently through one buyer stream handler; every frame reaches the edge,
// no frame is cross-attributed, and the session registry tracks both sessions
// with per-session node_id and message counts. This is the multi-source core.
func TestMultiSellerConcurrentForwarding(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	buyerKey, err := resolveKey("")
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	capture := &capturePublisher{}
	reg := newSessionRegistry()
	buyerHost.SetStreamHandler(protocol.ID(sapient.ProtocolDetection), func(s libp2pnetwork.Stream) {
		forwardStream(s, capture, log.New(io.Discard, "", 0), reg)
	})

	samples := loadSample(t)
	const (
		nodeRID  = "11111111-1111-1111-1111-111111111111"
		nodeADSB = "22222222-2222-2222-2222-222222222222"
	)

	mkSeller := func() (interface{ Close() error }, func(context.Context) (libp2pnetwork.Stream, error)) {
		key, kerr := resolveKey("")
		require.NoError(t, kerr)
		h, herr := delivery.NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
		require.NoError(t, herr)
		require.NoError(t, h.Connect(ctx, peer.AddrInfo{ID: buyerHost.ID(), Addrs: buyerHost.Addrs()}))
		return h, func(c context.Context) (libp2pnetwork.Stream, error) {
			return h.NewStream(c, buyerHost.ID(), protocol.ID(sapient.ProtocolDetection))
		}
	}
	sellerA, dialA := mkSeller()
	defer sellerA.Close()
	sellerB, dialB := mkSeller()
	defer sellerB.Close()

	doneA := make(chan struct{})
	doneB := make(chan struct{})
	go func() { defer close(doneA); pushSession(t, ctx, dialA, nodeRID, samples) }()
	go func() { defer close(doneB); pushSession(t, ctx, dialB, nodeADSB, samples) }()
	<-doneA
	<-doneB

	want := 2 * len(samples)
	require.Eventually(t, func() bool { return capture.count() == want }, 10*time.Second, 20*time.Millisecond,
		"edge must receive every frame from both sellers (got %d, want %d)", capture.count(), want)

	byNode := map[string]int{}
	capture.mu.Lock()
	for _, m := range capture.msgs {
		byNode[m.GetNodeId()]++
	}
	capture.mu.Unlock()
	assert.Equal(t, len(samples), byNode[nodeRID], "every RID-session frame attributed to its node_id")
	assert.Equal(t, len(samples), byNode[nodeADSB], "every ADSB-session frame attributed to its node_id")

	// Registry observed both sessions with the right node_ids and counts.
	require.Eventually(t, func() bool {
		opened, closed, _ := reg.totals()
		return opened == 2 && closed == 2
	}, 10*time.Second, 20*time.Millisecond, "both sessions opened and closed in the registry")
	opened, closed, msgs := reg.totals()
	assert.Equal(t, uint64(2), opened)
	assert.Equal(t, uint64(2), closed)
	assert.Equal(t, uint64(want), msgs)
}

// TestMultiSellerWithSlowDownstreamEdgeClient is the end-to-end head-of-line
// check at the buyer level: a real FeedServer edge with one never-reading
// client must not stall two pushing sellers — the reading client still sees
// frames from BOTH node_ids.
func TestMultiSellerWithSlowDownstreamEdgeClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	feed, err := sapient.ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer feed.Close()

	// Never-reading edge client (registered, then silent).
	slow, err := net.Dial("tcp", feed.Addr())
	require.NoError(t, err)
	defer slow.Close()
	// Reading edge client.
	good, errc := sapient.ReadBridgeFeed(ctx, feed.Addr())
	_ = errc
	require.Eventually(t, func() bool { return feed.ClientCount() == 2 }, 5*time.Second, 10*time.Millisecond)

	buyerKey, err := resolveKey("")
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	reg := newSessionRegistry()
	buyerHost.SetStreamHandler(protocol.ID(sapient.ProtocolDetection), func(s libp2pnetwork.Stream) {
		forwardStream(s, feed, log.New(io.Discard, "", 0), reg)
	})

	samples := loadSample(t)
	const (
		nodeRID  = "11111111-1111-1111-1111-111111111111"
		nodeADSB = "22222222-2222-2222-2222-222222222222"
	)
	for _, nodeID := range []string{nodeRID, nodeADSB} {
		key, kerr := resolveKey("")
		require.NoError(t, kerr)
		h, herr := delivery.NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
		require.NoError(t, herr)
		defer h.Close()
		require.NoError(t, h.Connect(ctx, peer.AddrInfo{ID: buyerHost.ID(), Addrs: buyerHost.Addrs()}))
		dial := func(c context.Context) (libp2pnetwork.Stream, error) {
			return h.NewStream(c, buyerHost.ID(), protocol.ID(sapient.ProtocolDetection))
		}
		pushSession(t, ctx, dial, nodeID, samples)
	}

	seen := map[string]int{}
	require.Eventually(t, func() bool {
		select {
		case m, ok := <-good:
			if ok && m != nil {
				seen[m.GetNodeId()]++
			}
		case <-time.After(50 * time.Millisecond):
		}
		return seen[nodeRID] >= 1 && seen[nodeADSB] >= 1
	}, 20*time.Second, time.Millisecond,
		"reading edge client must receive frames from both sellers despite the dead client (seen=%v)", seen)
}

// TestSessionRegistry_ReconnectCyclesNoGrowth: repeated seller stream churn
// must not leak registry entries — the open set returns to zero. (The buyer
// adds no per-session goroutines: libp2p owns handler goroutines and the
// FeedServer's per-client writers are goleak-verified in internal/dapp/sapient.)
func TestSessionRegistry_ReconnectCyclesNoGrowth(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	buyerKey, err := resolveKey("")
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	capture := &capturePublisher{}
	reg := newSessionRegistry()
	buyerHost.SetStreamHandler(protocol.ID(sapient.ProtocolDetection), func(s libp2pnetwork.Stream) {
		forwardStream(s, capture, log.New(io.Discard, "", 0), reg)
	})

	key, err := resolveKey("")
	require.NoError(t, err)
	sellerHost, err := delivery.NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()
	require.NoError(t, sellerHost.Connect(ctx, peer.AddrInfo{ID: buyerHost.ID(), Addrs: buyerHost.Addrs()}))

	sample := loadSample(t)[0]
	const cycles = 20
	for range cycles {
		stream, serr := sellerHost.NewStream(ctx, buyerHost.ID(), protocol.ID(sapient.ProtocolDetection))
		require.NoError(t, serr)
		m := proto.Clone(sample).(*sapientpb.SapientMessage)
		m.NodeId = proto.String("33333333-3333-3333-3333-333333333333")
		b, merr := proto.Marshal(m)
		require.NoError(t, merr)
		w := delivery.NewFrameWriter(stream)
		require.NoError(t, w.WriteFrame(b))
		require.NoError(t, stream.CloseWrite())
	}

	require.Eventually(t, func() bool {
		opened, closed, _ := reg.totals()
		return opened == cycles && closed == cycles && len(reg.snapshot()) == 0
	}, 15*time.Second, 20*time.Millisecond, "every churned session must close; open set returns to zero")
}

// TestSessionsHTTPEndpoint: the read-only observability endpoint serves the
// session list as JSON on GET and rejects other methods.
func TestSessionsHTTPEndpoint(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	reg := newSessionRegistry()
	id := reg.openSession("peer-X", "/ip4/10.0.0.1/udp/4242/quic-v1")
	reg.observe(id, "44444444-4444-4444-4444-444444444444", time.Now())

	srv, addr, err := startSessionsHTTP(ctx, "127.0.0.1:0", reg, nil, log.New(io.Discard, "", 0))
	require.NoError(t, err)
	defer srv.Close()

	resp, err := http.Get("http://" + addr + "/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body struct {
		Count    int           `json:"count"`
		Sessions []sessionInfo `json:"sessions"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, 1, body.Count)
	require.Len(t, body.Sessions, 1)
	assert.Equal(t, "peer-X", body.Sessions[0].PeerID)
	assert.Equal(t, "44444444-4444-4444-4444-444444444444", body.Sessions[0].NodeID)
	assert.Equal(t, uint64(1), body.Sessions[0].MessageCount)

	post, err := http.Post("http://"+addr+"/sessions", "application/json", nil)
	require.NoError(t, err)
	defer post.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, post.StatusCode)

	notFound, err := http.Get("http://" + addr + "/nope")
	require.NoError(t, err)
	defer notFound.Body.Close()
	assert.Equal(t, http.StatusNotFound, notFound.StatusCode)
}
