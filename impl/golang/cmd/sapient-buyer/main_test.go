package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
)

// capturePublisher is an in-memory publisher capturing forwarded SapientMessages.
type capturePublisher struct {
	mu   sync.Mutex
	msgs []*sapientpb.SapientMessage
}

func (c *capturePublisher) Publish(m *sapientpb.SapientMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, m)
	return nil
}
func (c *capturePublisher) count() int { c.mu.Lock(); defer c.mu.Unlock(); return len(c.msgs) }
func (c *capturePublisher) first() *sapientpb.SapientMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.msgs[0]
}

func loadSample(t *testing.T) []*sapientpb.SapientMessage {
	t.Helper()
	f, err := os.Open("testdata/bridge-sample.ndjson")
	require.NoError(t, err)
	defer f.Close()
	var msgs []*sapientpb.SapientMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		m := &sapientpb.SapientMessage{}
		require.NoError(t, protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(sc.Bytes(), m))
		msgs = append(msgs, m)
	}
	require.NoError(t, sc.Err())
	require.NotEmpty(t, msgs)
	return msgs
}

// TestSapientBuyer_ReverseConnect_ForwardsOpaque is the FR-S90 proof: a seller
// DIALS the in-process Buyer Proxy and pushes the captured-from-the-real-bridge
// SapientMessages; the proxy forwards each one UNCHANGED onto the SAPIENT edge —
// a SapientMessage, never a rid-projected TaggedFrame — preserving identity and
// the seller-restamped node_id. The proxy never parses object_info.
func TestSapientBuyer_ReverseConnect_ForwardsOpaque(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	// Buyer Proxy = reachable receiver: listens + forwards to a capturing edge.
	buyerKey, err := resolveKey("")
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	capture := &capturePublisher{}
	buyerHost.SetStreamHandler(protocol.ID(sapient.ProtocolDetection), func(s libp2pnetwork.Stream) {
		forwardStream(s, capture, log.New(io.Discard, "", 0), newSessionRegistry())
	})

	// Seller = pusher: DIALS the buyer and pushes the captured sample (re-stamped).
	sellerKey, err := resolveKey("")
	require.NoError(t, err)
	sellerHost, err := delivery.NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	require.NoError(t, sellerHost.Connect(ctx, peer.AddrInfo{ID: buyerHost.ID(), Addrs: buyerHost.Addrs()}))
	stream, err := sellerHost.NewStream(ctx, buyerHost.ID(), protocol.ID(sapient.ProtocolDetection))
	require.NoError(t, err)

	const nodeID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" // seller's Neuron identity (not the placeholder)
	samples := loadSample(t)
	wantReportID := samples[0].GetDetectionReport().GetReportId()
	w := delivery.NewFrameWriter(stream)
	for _, msg := range samples {
		msg.NodeId = proto.String(nodeID)
		b, err := proto.Marshal(msg)
		require.NoError(t, err)
		require.NoError(t, w.WriteFrame(b))
	}
	require.NoError(t, stream.CloseWrite())

	require.Eventually(t, func() bool { return capture.count() >= 1 }, 8*time.Second, 20*time.Millisecond,
		"proxy must forward at least one SapientMessage")

	got := capture.first()
	require.NotNil(t, got.GetDetectionReport(), "DetectionReport forwarded intact (opaque)")
	require.Equal(t, wantReportID, got.GetDetectionReport().GetReportId())
	require.Equal(t, nodeID, got.GetNodeId(), "seller-restamped node_id preserved across the proxy")
}

// TestKeyHexOrEnv: the systemd EnvironmentFile path — NEURON_KEY_HEX supplies
// the key when --key-hex is absent so the secret never appears in argv (a
// permanent buyer needs a stable key: a restart with a fresh PeerID would
// invalidate every seller's --buyer dial multiaddr); an explicit flag always
// wins; both empty keeps the ephemeral default.
func TestKeyHexOrEnv(t *testing.T) {
	t.Setenv("NEURON_KEY_HEX", "aa11")
	require.Equal(t, "ff22", keyHexOrEnv("ff22"), "explicit flag wins over env")
	require.Equal(t, "aa11", keyHexOrEnv(""), "env fallback when flag empty")
	t.Setenv("NEURON_KEY_HEX", "")
	require.Empty(t, keyHexOrEnv(""), "both empty -> ephemeral path")
}
