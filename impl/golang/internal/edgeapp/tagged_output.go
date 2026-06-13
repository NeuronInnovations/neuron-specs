package edgeapp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// TaggedSink is the output interface for DApp-tagged frames per the Phase-2
// reference MVP plan (specifically, the dual-stream display contract documented
// in docs/fid-display-contract.md).
//
// Unlike OutputSink (which is hardcoded to AggregatedFrame for the legacy
// ADS-B path), TaggedSink accepts any JSON-marshallable value. Callers
// emit pre-built envelopes like:
//
//	{"source":"remote-id", "type":"drone", "sellerPeerID":"...", "frame":{...}}
//	{"source":"adsb",      "type":"aircraft", "sellerPeerID":"...", "frame":{...}}
//
// The sink serializes to JSONL (one record per line, '\n' terminator) per
// the contract the React/FID display already expects.
//
// Implementations MUST be safe for concurrent Emit; the Phase-5 fused buyer
// will call Emit from multiple per-DApp goroutines into one sink.
type TaggedSink interface {
	Emit(ctx context.Context, value any) error
	Close() error
}

// NewTaggedJSONLSink parses spec and returns the corresponding sink:
//
//	"stdout"               → stdout
//	"file:/path/to/x.jsonl" → file (truncate on open)
//	"file+:/path/to/x.jsonl" → file (append)
//	"tcp:host:port"        → TCP socket (reconnecting)
//	""                      → stdout (default)
//
// Unknown specs return an error.
//
// Sinks mirror the legacy OutputSink semantics for sink configuration but
// emit a generic any-typed envelope.
func NewTaggedJSONLSink(spec string) (TaggedSink, error) {
	if spec == "" || spec == "stdout" {
		return newTaggedStdoutSink(), nil
	}
	switch {
	case strings.HasPrefix(spec, "file:"):
		return newTaggedFileSink(strings.TrimPrefix(spec, "file:"), false)
	case strings.HasPrefix(spec, "file+:"):
		return newTaggedFileSink(strings.TrimPrefix(spec, "file+:"), true)
	case strings.HasPrefix(spec, "tcp:"):
		return newTaggedTCPSink(strings.TrimPrefix(spec, "tcp:")), nil
	default:
		return nil, fmt.Errorf("edgeapp: unknown tagged sink %q (want stdout|file:PATH|file+:PATH|tcp:HOST:PORT)", spec)
	}
}

// taggedJSONLWriter is the shared encoder kernel.
func taggedJSONLWriter(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal tagged frame: %w", err)
	}
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write tagged frame: %w", err)
	}
	return nil
}

// ─── stdout ──────────────────────────────────────────────────────────────

type taggedStdoutSink struct {
	mu sync.Mutex
}

func newTaggedStdoutSink() *taggedStdoutSink { return &taggedStdoutSink{} }

func (s *taggedStdoutSink) Emit(_ context.Context, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return taggedJSONLWriter(os.Stdout, v)
}

func (s *taggedStdoutSink) Close() error { return nil }

// ─── file ────────────────────────────────────────────────────────────────

type taggedFileSink struct {
	mu sync.Mutex
	f  *os.File
	w  *bufio.Writer
}

func newTaggedFileSink(path string, append bool) (*taggedFileSink, error) {
	flag := os.O_WRONLY | os.O_CREATE
	if append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open tagged sink file %s: %w", path, err)
	}
	return &taggedFileSink{f: f, w: bufio.NewWriter(f)}, nil
}

func (s *taggedFileSink) Emit(_ context.Context, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := taggedJSONLWriter(s.w, v); err != nil {
		return err
	}
	return s.w.Flush()
}

func (s *taggedFileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.w != nil {
		_ = s.w.Flush()
	}
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}

// ─── tcp ─────────────────────────────────────────────────────────────────
//
// Reconnecting TCP sink mirroring the legacy TCPSink shape. The dial is
// lazy; first Emit triggers connection; subsequent emit failures reset
// the connection so the next Emit retries.

type taggedTCPSink struct {
	addr string
	mu   sync.Mutex
	conn net.Conn
}

func newTaggedTCPSink(addr string) *taggedTCPSink {
	return &taggedTCPSink{addr: addr}
}

func (s *taggedTCPSink) Emit(ctx context.Context, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		d := net.Dialer{Timeout: 5 * time.Second}
		c, err := d.DialContext(ctx, "tcp", s.addr)
		if err != nil {
			return fmt.Errorf("dial tagged tcp sink %s: %w", s.addr, err)
		}
		s.conn = c
	}
	if err := taggedJSONLWriter(s.conn, v); err != nil {
		_ = s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}

func (s *taggedTCPSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	return err
}
