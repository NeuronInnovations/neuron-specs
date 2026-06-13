package edgeapp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleFrame(seller string, ns int64) AggregatedFrame {
	return AggregatedFrame{
		SellerEVM:    seller,
		SellerName:   seller[:6],
		SellerPeerID: "12D3KooWFakePeer",
		Frame: feeds.FeedFrame{
			Raw:                  []byte{0x8D, 0x4C, 0xA8, 0x53, 0x58, 0xC3, 0x82, 0xD6, 0x90, 0xC8, 0xAC, 0x28, 0x63, 0xA7},
			SecondsSinceMidnight: 12345,
			Nanoseconds:          uint64(ns),
		},
		Meta: feeds.ModeSMeta{DF: 17, ICAO: "4ca853", Long: true},
		ReceivedAt: time.Unix(0, ns).UTC(),
	}
}

func TestNewOutputSink_Spec(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		spec    string
		wantErr bool
	}{
		{"", false},
		{"stdout", false},
		{"file:" + filepath.Join(dir, "a.jsonl"), false},
		{"file+:" + filepath.Join(dir, "b.jsonl"), false},
		{"tcp:localhost:1", false}, // dial is lazy; no error on construction
		{"file:", true},
		{"unknown:foo", true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.spec, func(t *testing.T) {
			s, err := NewOutputSink(c.spec)
			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, s)
			require.NoError(t, s.Close())
		})
	}
}

func TestFileJSONLSink_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frames.jsonl")
	sink, err := NewFileJSONLSink(path, false)
	require.NoError(t, err)

	want := []AggregatedFrame{
		sampleFrame("0xAAAAAAAAAA", 100),
		sampleFrame("0xBBBBBBBBBB", 200),
		sampleFrame("0xCCCCCCCCCC", 300),
	}
	for _, f := range want {
		require.NoError(t, sink.Emit(context.Background(), f))
	}
	require.NoError(t, sink.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var got []AggregatedFrame
	for scanner.Scan() {
		var af AggregatedFrame
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &af))
		got = append(got, af)
	}
	require.NoError(t, scanner.Err())
	require.Len(t, got, len(want))
	for i := range want {
		assert.Equal(t, want[i].SellerEVM, got[i].SellerEVM)
		assert.Equal(t, want[i].Meta.ICAO, got[i].Meta.ICAO)
		assert.Equal(t, want[i].Frame.Raw, got[i].Frame.Raw)
	}
}

func TestFileJSONLSink_AppendModePreservesOldData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appended.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(`{"prior":"line"}`+"\n"), 0o644))

	sink, err := NewFileJSONLSink(path, true)
	require.NoError(t, err)
	require.NoError(t, sink.Emit(context.Background(), sampleFrame("0xDDDDDDDDDD", 1)))
	require.NoError(t, sink.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"prior":"line"`, "append must preserve prior content")
	assert.Contains(t, string(data), `"sellerEVM":"0xDDDDDDDDDD"`)
}

func TestStdoutJSONLSink_NotPanic(t *testing.T) {
	s := NewStdoutJSONLSink()
	// Send to /dev/null by redirecting stdout temporarily. Easier: just assert no error.
	require.NoError(t, s.Emit(context.Background(), sampleFrame("0xEEEEEEEEEE", 0)))
	require.NoError(t, s.Close())
}

func TestTCPSink_RoundTrip(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	type rcvLine struct {
		bytes []byte
		err   error
	}
	rcv := make(chan rcvLine, 4)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			rcv <- rcvLine{err: err}
			return
		}
		defer conn.Close()
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			rcv <- rcvLine{bytes: append([]byte(nil), scanner.Bytes()...)}
		}
	}()

	sink := NewTCPSink(ln.Addr().String())
	for i := 0; i < 3; i++ {
		require.NoError(t, sink.Emit(context.Background(), sampleFrame("0xFFFFFFFFFF", int64(i))))
	}
	// Receive 3 lines.
	for i := 0; i < 3; i++ {
		select {
		case line := <-rcv:
			require.NoError(t, line.err)
			var af AggregatedFrame
			require.NoError(t, json.Unmarshal(line.bytes, &af))
			assert.Equal(t, "0xFFFFFFFFFF", af.SellerEVM)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for line %d", i)
		}
	}
	require.NoError(t, sink.Close())
}

func TestTCPSink_AfterCloseReturnsError(t *testing.T) {
	sink := NewTCPSink("127.0.0.1:1")
	require.NoError(t, sink.Close())
	err := sink.Emit(context.Background(), sampleFrame("0x0000000000", 0))
	assert.Error(t, err)
}

