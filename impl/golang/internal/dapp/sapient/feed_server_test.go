package sapient

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// loadFirstSample reads the first SapientMessage from the captured-from-the-real-
// bridge NDJSON sample shared across the SAPIENT tests.
func loadFirstSample(t *testing.T) *sapientpb.SapientMessage {
	t.Helper()
	f, err := os.Open("testdata/bridge-sample.ndjson")
	require.NoError(t, err)
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), MaxEdgeFrameSize)
	require.True(t, sc.Scan(), "sample has at least one line")
	msg := &sapientpb.SapientMessage{}
	require.NoError(t, protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(sc.Bytes(), msg))
	return msg
}

// recvUntilPublished publishes sample on a ticker until the client channel yields
// a message — absorbing the accept/registration race (a Publish before the
// just-dialed client is registered is dropped, by live-telemetry design).
func recvUntilPublished(t *testing.T, srv *FeedServer, sample *sapientpb.SapientMessage, out <-chan *sapientpb.SapientMessage) *sapientpb.SapientMessage {
	t.Helper()
	var got *sapientpb.SapientMessage
	require.Eventually(t, func() bool {
		require.NoError(t, srv.Publish(sample))
		select {
		case m := <-out:
			got = m
			return true
		case <-time.After(40 * time.Millisecond):
			return false
		}
	}, 5*time.Second, 10*time.Millisecond, "client must receive a published message")
	require.NotNil(t, got)
	return got
}

// TestFeedServer_RoundTrip proves the SAPIENT edge round-trips a DetectionReport
// intact: ServeFeed → ReadBridgeFeed dials in → Publish → the client receives the
// same message (id, object_id, node_id, position all preserved). This is the
// proxy→consumer hop the M1 split introduces.
func TestFeedServer_RoundTrip(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv, err := ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	out, errc := ReadBridgeFeed(ctx, srv.Addr())

	sample := loadFirstSample(t)
	got := recvUntilPublished(t, srv, sample, out)

	gdr := got.GetDetectionReport()
	require.NotNil(t, gdr, "DetectionReport survives the edge")
	require.Equal(t, sample.GetDetectionReport().GetReportId(), gdr.GetReportId())
	require.Equal(t, sample.GetDetectionReport().GetObjectId(), gdr.GetObjectId())
	require.Equal(t, sample.GetNodeId(), got.GetNodeId())
	require.InDelta(t, sample.GetDetectionReport().GetLocation().GetY(), gdr.GetLocation().GetY(), 1e-9)

	cancel()
	select {
	case e := <-errc:
		require.NoError(t, e, "clean cancel is not an error")
	case <-time.After(time.Second):
	}
}

// TestFeedServer_FanOut proves two simultaneously-connected clients each receive
// every published message (multi-consumer fan-out).
func TestFeedServer_FanOut(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv, err := ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	outA, _ := ReadBridgeFeed(ctx, srv.Addr())
	outB, _ := ReadBridgeFeed(ctx, srv.Addr())
	sample := loadFirstSample(t)

	var gotA, gotB bool
	require.Eventually(t, func() bool {
		require.NoError(t, srv.Publish(sample))
		select {
		case <-outA:
			gotA = true
		case <-outB:
			gotB = true
		case <-time.After(40 * time.Millisecond):
		}
		return gotA && gotB
	}, 5*time.Second, 10*time.Millisecond, "both clients must receive the feed")
}

// TestFeedServer_Close proves Close ends a connected client's stream (its
// ReadBridgeFeed channel closes), and that Close is idempotent.
func TestFeedServer_Close(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv, err := ServeFeed("127.0.0.1:0")
	require.NoError(t, err)

	out, _ := ReadBridgeFeed(ctx, srv.Addr())
	// Make sure the client is connected before closing.
	recvUntilPublished(t, srv, loadFirstSample(t), out)

	require.NoError(t, srv.Close())
	require.NoError(t, srv.Close(), "Close is idempotent")

	// The client's feed channel drains and closes once the server drops the conn.
	require.Eventually(t, func() bool {
		select {
		case _, ok := <-out:
			return !ok // closed
		case <-time.After(40 * time.Millisecond):
			return false
		}
	}, 3*time.Second, 10*time.Millisecond, "client feed closes after server Close")
}

// TestFeedServer_WireIsLittleEndian inspects the raw on-wire bytes (not via
// ReadBridgeFeed) and proves the FR-S91 framing: a 4-byte LITTLE-endian length
// prefix equal to the protobuf payload length, followed by exactly that payload.
func TestFeedServer_WireIsLittleEndian(t *testing.T) {
	srv, err := ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	defer conn.Close()

	sample := loadFirstSample(t)
	wantPayload, err := proto.Marshal(sample)
	require.NoError(t, err)

	// Publish until the raw client is registered and a full prefix arrives.
	var prefix [4]byte
	require.Eventually(t, func() bool {
		require.NoError(t, srv.Publish(sample))
		_ = conn.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
		_, rerr := io.ReadFull(conn, prefix[:])
		return rerr == nil
	}, 5*time.Second, 10*time.Millisecond, "raw client must receive a frame prefix")

	var wantPrefix [4]byte
	binary.LittleEndian.PutUint32(wantPrefix[:], uint32(len(wantPayload)))
	require.Equal(t, wantPrefix[:], prefix[:], "on-wire prefix is little-endian")

	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	body := make([]byte, binary.LittleEndian.Uint32(prefix[:]))
	_, err = io.ReadFull(conn, body)
	require.NoError(t, err)
	require.Equal(t, wantPayload, body, "framed body is the exact protobuf payload")
}
