package edgeapp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// OutputSink consumes AggregatedFrames produced by RunBuyer and forwards
// them to the airport-display path (stdout, JSONL file, or a TCP socket).
//
// Implementations must be safe to call from multiple goroutines (the buyer
// invokes Emit from one goroutine per seller). Implementations should NOT
// block indefinitely on slow downstream consumers — a non-trivial backlog
// is a bug, not a silent feature; either drop with logging or surface back
// via the returned error.
type OutputSink interface {
	// Emit writes the frame downstream. ctx may be honored for cancellation
	// of in-flight network writes, but should not be relied on for tight
	// real-time bounds.
	Emit(ctx context.Context, frame AggregatedFrame) error

	// Close flushes any buffered data and releases the underlying handle.
	// Safe to call multiple times.
	Close() error
}

// NewOutputSink parses spec and returns the corresponding sink:
//
//	"stdout"               → StdoutJSONLSink
//	"file:/path/to/x.jsonl" → FileJSONLSink (truncates on open)
//	"file+:/path/to/x.jsonl" → FileJSONLSink (appends if file exists)
//	"tcp:host:port"        → TCPSink (reconnecting; line-delimited JSON)
//	""                      → StdoutJSONLSink (default)
//
// Unknown specs return an error.
func NewOutputSink(spec string) (OutputSink, error) {
	if spec == "" || spec == "stdout" {
		return NewStdoutJSONLSink(), nil
	}
	switch {
	case strings.HasPrefix(spec, "file:"):
		return NewFileJSONLSink(strings.TrimPrefix(spec, "file:"), false)
	case strings.HasPrefix(spec, "file+:"):
		return NewFileJSONLSink(strings.TrimPrefix(spec, "file+:"), true)
	case strings.HasPrefix(spec, "tcp:"):
		return NewTCPSink(strings.TrimPrefix(spec, "tcp:")), nil
	default:
		return nil, fmt.Errorf("edgeapp: unknown output sink %q (want stdout|file:PATH|file+:PATH|tcp:HOST:PORT)", spec)
	}
}

// jsonlWriter is the shared JSONL-encoding kernel used by every sink.
// It serializes one frame per line (line terminator '\n') with no spaces.
func jsonlWriter(w io.Writer, frame AggregatedFrame) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("marshal aggregated frame: %w", err)
	}
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// ─── stdout ──────────────────────────────────────────────────────────────

// StdoutJSONLSink writes one JSON line per frame to os.Stdout.
//
// Stdout writes are line-buffered by default in most operating environments,
// so this sink does not add its own buffer; this keeps the airport-display
// path observable at line granularity from `tail -f` or piped readers.
type StdoutJSONLSink struct {
	mu sync.Mutex
}

// NewStdoutJSONLSink returns a sink that writes to os.Stdout.
func NewStdoutJSONLSink() *StdoutJSONLSink { return &StdoutJSONLSink{} }

// Emit writes one JSON line to stdout.
func (s *StdoutJSONLSink) Emit(_ context.Context, f AggregatedFrame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return jsonlWriter(os.Stdout, f)
}

// Close is a no-op for stdout — we don't own the file descriptor.
func (s *StdoutJSONLSink) Close() error { return nil }

// ─── file ────────────────────────────────────────────────────────────────

// FileJSONLSink writes one JSON line per frame to a file. It uses a small
// bufio.Writer; the buffer is flushed on every Emit so a `tail -f` reader
// observes lines at sub-second latency.
type FileJSONLSink struct {
	mu  sync.Mutex
	f   *os.File
	w   *bufio.Writer
	dst string
}

// NewFileJSONLSink opens path for writing. When append is true and the file
// exists, new frames are appended; otherwise the file is truncated on open.
func NewFileJSONLSink(path string, append bool) (*FileJSONLSink, error) {
	if path == "" {
		return nil, errors.New("edgeapp.NewFileJSONLSink: empty path")
	}
	flags := os.O_CREATE | os.O_WRONLY
	if append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil, fmt.Errorf("edgeapp.NewFileJSONLSink: open %s: %w", path, err)
	}
	return &FileJSONLSink{f: f, w: bufio.NewWriterSize(f, 64*1024), dst: path}, nil
}

// Emit writes one line and flushes.
func (s *FileJSONLSink) Emit(_ context.Context, f AggregatedFrame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := jsonlWriter(s.w, f); err != nil {
		return err
	}
	return s.w.Flush()
}

// Close flushes the buffer and closes the file descriptor.
func (s *FileJSONLSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	if s.w != nil {
		_ = s.w.Flush()
	}
	err := s.f.Close()
	s.f = nil
	s.w = nil
	return err
}

// ─── tcp ─────────────────────────────────────────────────────────────────

// TCPSink writes one JSON line per frame to a TCP socket. It connects on
// first Emit and reconnects on write failure with a 1 s backoff. Frames
// emitted while disconnected are dropped (logged via the connection-error
// surface, not buffered) — the upstream airport display is expected to
// observe a "best-effort live" stream.
type TCPSink struct {
	mu     sync.Mutex
	addr   string
	conn   net.Conn
	closed bool
}

// NewTCPSink returns a sink that writes to addr. The dial is lazy.
func NewTCPSink(addr string) *TCPSink {
	return &TCPSink{addr: addr}
}

// Emit ensures a connection is open, writes one line, and on any I/O error
// closes the conn so the next Emit reconnects.
func (s *TCPSink) Emit(ctx context.Context, f AggregatedFrame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("edgeapp.TCPSink: closed")
	}
	if s.conn == nil {
		d := net.Dialer{Timeout: 5 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", s.addr)
		if err != nil {
			return fmt.Errorf("edgeapp.TCPSink: dial %s: %w", s.addr, err)
		}
		s.conn = conn
	}
	if err := jsonlWriter(s.conn, f); err != nil {
		_ = s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}

// Close shuts down the TCP connection. Subsequent Emit calls return an error.
func (s *TCPSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}
