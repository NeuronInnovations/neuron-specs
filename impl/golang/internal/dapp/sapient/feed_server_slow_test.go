package sapient

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// bigSample returns a SapientMessage padded to roughly 64 KiB so a handful of
// frames saturate kernel socket buffers (forcing the slow-client path).
func bigSample(t *testing.T) *sapientpb.SapientMessage {
	t.Helper()
	msg := proto.Clone(loadFirstSample(t)).(*sapientpb.SapientMessage)
	dr := msg.GetDetectionReport()
	require.NotNil(t, dr)
	pad := strings.Repeat("x", 64*1024)
	key := "test.pad"
	dr.ObjectInfo = append(dr.ObjectInfo, &sapientpb.DetectionReport_TrackObjectInfo{
		Type: &key, Value: &pad,
	})
	return msg
}

// TestFeedServer_SlowClientDoesNotBlockOthers is the head-of-line-blocking
// regression test for the multi-source buyer: one downstream client that never
// reads must not stall Publish for the others. Frames are pumped in lockstep
// with the fast client's receipt, so the only way the test times out is a
// Publish blocked on the slow client.
func TestFeedServer_SlowClientDoesNotBlockOthers(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv, err := ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	sample := bigSample(t)

	// Fast client: the decoded SAPIENT feed.
	fast, _ := ReadBridgeFeed(ctx, srv.Addr())
	recvUntilPublished(t, srv, sample, fast)

	// Slow client: raw conn that reads exactly one frame (proving it is
	// registered) and then never reads again.
	slow, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	defer slow.Close()
	require.Eventually(t, func() bool {
		require.NoError(t, srv.Publish(sample))
		_ = slow.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
		var prefix [4]byte
		if _, rerr := io.ReadFull(slow, prefix[:]); rerr != nil {
			return false
		}
		body := make([]byte, binary.LittleEndian.Uint32(prefix[:]))
		_, rerr := io.ReadFull(slow, body)
		return rerr == nil
	}, 5*time.Second, 10*time.Millisecond, "slow client must be registered before the pump")
	// Drain anything the fast client already received from the setup publishes.
	for {
		select {
		case <-fast:
			continue
		case <-time.After(100 * time.Millisecond):
		}
		break
	}

	// Pump in lockstep: every published frame must reach the fast client even
	// though the slow client has stopped reading (64 KiB frames overflow its
	// kernel buffer and then its server-side backlog).
	const frames = 600
	for i := range frames {
		done := make(chan error, 1)
		go func() { done <- srv.Publish(sample) }()
		select {
		case perr := <-done:
			require.NoError(t, perr)
		case <-time.After(5 * time.Second):
			t.Fatalf("Publish blocked at frame %d — slow client stalls the fan-out", i)
		}
		select {
		case <-fast:
		case <-time.After(5 * time.Second):
			t.Fatalf("fast client starved at frame %d — slow client stalls the fan-out", i)
		}
	}
}

// TestFeedServer_SlowClientEventuallyDropped proves overflow semantics: a
// never-reading client is disconnected (live telemetry drops, never buffers
// unboundedly), leaving only the healthy client connected.
func TestFeedServer_SlowClientEventuallyDropped(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv, err := ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	sample := bigSample(t)

	fast, _ := ReadBridgeFeed(ctx, srv.Addr())
	recvUntilPublished(t, srv, sample, fast)

	slow, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	defer slow.Close()
	// One frame proves registration; then the client goes silent.
	require.Eventually(t, func() bool {
		require.NoError(t, srv.Publish(sample))
		_ = slow.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
		var prefix [4]byte
		if _, rerr := io.ReadFull(slow, prefix[:]); rerr != nil {
			return false
		}
		body := make([]byte, binary.LittleEndian.Uint32(prefix[:]))
		_, rerr := io.ReadFull(slow, body)
		return rerr == nil
	}, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, 2, srv.ClientCount(), "both clients registered")

	// Keep publishing (draining the fast side) until the slow client is shed.
	require.Eventually(t, func() bool {
		require.NoError(t, srv.Publish(sample))
		select {
		case <-fast:
		default:
		}
		return srv.ClientCount() == 1
	}, 30*time.Second, time.Millisecond, "never-reading client must be dropped")
}

// TestFeedServer_NoGoroutineLeakOnClientChurn churns raw clients through the
// server and verifies every per-client goroutine is reclaimed after Close.
func TestFeedServer_NoGoroutineLeakOnClientChurn(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	srv, err := ServeFeed("127.0.0.1:0")
	require.NoError(t, err)

	sample := loadFirstSample(t)
	for range 50 {
		conn, derr := net.Dial("tcp", srv.Addr())
		require.NoError(t, derr)
		// Receive one frame to prove the round-trip, then churn.
		require.Eventually(t, func() bool {
			require.NoError(t, srv.Publish(sample))
			_ = conn.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
			var prefix [4]byte
			if _, rerr := io.ReadFull(conn, prefix[:]); rerr != nil {
				return false
			}
			body := make([]byte, binary.LittleEndian.Uint32(prefix[:]))
			_, rerr := io.ReadFull(conn, body)
			return rerr == nil
		}, 5*time.Second, 5*time.Millisecond)
		require.NoError(t, conn.Close())
	}

	require.NoError(t, srv.Close())
}
