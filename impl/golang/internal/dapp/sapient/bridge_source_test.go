package sapient

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// TestReadBridgeFeed_ParsesLEFramedSample serves the captured-from-the-real-bridge
// sample as 4-byte little-endian length-prefixed protobuf — the FR-S91 SAPIENT
// edge framing the corrected bridge (--sapient-format protobuf) emits — and
// asserts ReadBridgeFeed decodes SapientMessages from it. This doubles as the
// bridge↔seller wire-compatibility proof: the test frames exactly as the bridge
// will, and the production reader parses it.
func TestReadBridgeFeed_ParsesLEFramedSample(t *testing.T) {
	// The on-disk fixture stays human-readable protojson (NDJSON); we re-frame each
	// message as LE protobuf on the wire, the way --sapient-format protobuf does.
	f, err := os.Open("testdata/bridge-sample.ndjson")
	require.NoError(t, err)
	defer f.Close()

	var framed bytes.Buffer
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), MaxEdgeFrameSize)
	var nLines int
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		msg := &sapientpb.SapientMessage{}
		require.NoError(t, protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(sc.Bytes(), msg))
		payload, err := proto.Marshal(msg)
		require.NoError(t, err)
		require.NoError(t, writeLEFrame(&framed, payload))
		nLines++
	}
	require.NoError(t, sc.Err())
	require.GreaterOrEqual(t, nLines, 1)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write(framed.Bytes()) // FIN after data → clean EOF for the reader
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, errc := ReadBridgeFeed(ctx, ln.Addr().String())

	var msgs []*sapientpb.SapientMessage
	for m := range out {
		msgs = append(msgs, m)
	}
	select {
	case e := <-errc:
		require.NoError(t, e, "clean EOF must not surface as an error")
	default:
	}

	require.GreaterOrEqual(t, len(msgs), 1)
	dr := msgs[0].GetDetectionReport()
	require.NotNil(t, dr)
	require.Regexp(t, `^[0-9A-HJKMNP-TV-Z]{26}$`, dr.GetObjectId(), "object_id is a ULID")
	require.Equal(t, "1581F8B1234567890ABC", dr.GetId(), "native id = serial")
}
