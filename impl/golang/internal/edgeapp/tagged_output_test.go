package edgeapp

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sampleTagged struct {
	Source       string `json:"source"`
	Type         string `json:"type"`
	SellerPeerID string `json:"sellerPeerID"`
	Detail       string `json:"detail,omitempty"`
}

func TestNewTaggedJSONLSink_ParsesAllSchemes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, spec := range []string{
		"",
		"stdout",
		"file:" + filepath.Join(dir, "a.jsonl"),
		"file+:" + filepath.Join(dir, "b.jsonl"),
		// tcp deferred to its own test (needs a listener)
	} {
		s, err := NewTaggedJSONLSink(spec)
		require.NoError(t, err, "spec %q", spec)
		require.NotNil(t, s, "spec %q", spec)
		_ = s.Close()
	}
}

func TestNewTaggedJSONLSink_RejectsUnknownSpec(t *testing.T) {
	t.Parallel()
	_, err := NewTaggedJSONLSink("invalid-prefix:/path")
	assert.Error(t, err)
}

func TestTaggedFileSink_EmitWritesJSONL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "drones.jsonl")

	sink, err := NewTaggedJSONLSink("file:" + path)
	require.NoError(t, err)

	rec := sampleTagged{
		Source:       "remote-id",
		Type:         "drone",
		SellerPeerID: "12D3KooW",
		Detail:       "test",
	}
	require.NoError(t, sink.Emit(context.Background(), rec))
	require.NoError(t, sink.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// File should contain exactly one line ending in \n.
	line := string(data)
	assert.True(t, len(line) > 0 && line[len(line)-1] == '\n', "missing newline terminator: %q", line)

	var back sampleTagged
	require.NoError(t, json.Unmarshal([]byte(line[:len(line)-1]), &back))
	assert.Equal(t, "remote-id", back.Source)
	assert.Equal(t, "drone", back.Type)
}

func TestTaggedFileSink_AppendMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "drones.jsonl")

	// First sink: truncate-mode write one line.
	s1, err := NewTaggedJSONLSink("file:" + path)
	require.NoError(t, err)
	require.NoError(t, s1.Emit(context.Background(), sampleTagged{Source: "remote-id", Type: "first"}))
	require.NoError(t, s1.Close())

	// Second sink: append-mode write a second line.
	s2, err := NewTaggedJSONLSink("file+:" + path)
	require.NoError(t, err)
	require.NoError(t, s2.Emit(context.Background(), sampleTagged{Source: "remote-id", Type: "second"}))
	require.NoError(t, s2.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 2)

	var first, second sampleTagged
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &second))
	assert.Equal(t, "first", first.Type)
	assert.Equal(t, "second", second.Type)
}

// TestTaggedTCPSink_EmitReachesListener verifies the TCP sink writes
// JSONL onto a connection that survives a single Emit.
func TestTaggedTCPSink_EmitReachesListener(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	type rcv struct {
		data []byte
		err  error
	}
	rcvCh := make(chan rcv, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			rcvCh <- rcv{err: err}
			return
		}
		defer conn.Close()
		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		rcvCh <- rcv{data: buf[:n], err: err}
	}()

	sink := newTaggedTCPSink(ln.Addr().String())
	require.NoError(t, sink.Emit(context.Background(), sampleTagged{Source: "remote-id", Type: "drone"}))
	defer sink.Close()

	select {
	case r := <-rcvCh:
		require.NoError(t, r.err)
		assert.Contains(t, string(r.data), `"source":"remote-id"`)
		assert.Contains(t, string(r.data), `"type":"drone"`)
	case <-time.After(time.Second):
		t.Fatal("listener did not receive bytes within 1s")
	}
}

