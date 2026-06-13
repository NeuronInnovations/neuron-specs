package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxStateBytes caps a proxied /state.json body (defensive against a misbehaving
// or compromised upstream). The live display's snapshot is far smaller.
const maxStateBytes = 8 << 20 // 8 MiB

// tracksHandler proxies the live sapient-fid-display /state.json read-only. If
// the upstream is unreachable it returns a graceful degraded payload (HTTP 200)
// so the map view shows an "offline" banner rather than erroring — the registry
// view is unaffected either way.
func (s *server) tracksHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), fidProbeTimeout)
	defer cancel()
	if body, ok := s.proxyState(ctx); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tracks":   []any{},
		"count":    0,
		"degraded": true,
		"reason":   "fid-display unreachable",
	})
}

// proxyState GETs the upstream /state.json and returns its body verbatim. Any
// error, non-2xx status, or empty fidURL reports not-ok (caller degrades).
func (s *server) proxyState(ctx context.Context) ([]byte, bool) {
	if s.cfg.fidURL == "" {
		return nil, false
	}
	url := strings.TrimRight(s.cfg.fidURL, "/") + "/state.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxStateBytes))
	if err != nil {
		return nil, false
	}
	return body, true
}

// eventsHandler proxies the upstream /events SSE stream read-only. The upstream
// request is tied to the client's request context, so a browser disconnect
// cancels the upstream read and the handler returns — no goroutine is leaked.
// When the upstream is unreachable it emits a single "degraded" SSE event so the
// browser EventSource can fall back to polling /tracks.json.
func (s *server) eventsHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if s.streamUpstreamEvents(w, r, flusher) {
		return
	}
	fmt.Fprint(w, "event: degraded\ndata: {\"reason\":\"fid-display unreachable\"}\n\n")
	flusher.Flush()
}

// streamUpstreamEvents copies the upstream SSE stream to w, flushing each chunk.
// Returns true once a 2xx upstream stream was established (even if it then closed
// cleanly); false if the upstream could not be reached so the caller can degrade.
func (s *server) streamUpstreamEvents(w http.ResponseWriter, r *http.Request, flusher http.Flusher) bool {
	if s.cfg.fidURL == "" {
		return false
	}
	url := strings.TrimRight(s.cfg.fidURL, "/") + "/events"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	flusher.Flush() // commit 200 + SSE headers before streaming
	_, _ = io.Copy(&flushWriter{w: w, f: flusher}, resp.Body)
	return true
}

// flushWriter flushes the underlying ResponseWriter after every write so SSE
// frames reach the browser immediately rather than being buffered.
type flushWriter struct {
	w io.Writer
	f http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return n, err
}
