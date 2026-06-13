package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// sessionInfo is the observable state of one seller session (one inbound
// /sapient/detection/2.0.0 stream). Everything here is transport- or
// SAPIENT-envelope-derived — the proxy stays vendor/modality-blind (FR-S90):
// NodeID is the top-level SapientMessage node_id (already decoded for
// forwarding), never anything parsed out of object_info.
type sessionInfo struct {
	ID           uint64    `json:"id"`
	PeerID       string    `json:"peerID"`
	RemoteAddr   string    `json:"remoteAddr"`
	NodeID       string    `json:"nodeId,omitempty"`
	ConnectedAt  time.Time `json:"connectedAt"`
	LastSeen     time.Time `json:"lastSeen"`
	MessageCount uint64    `json:"messageCount"`
}

// sessionRegistry tracks the open seller sessions plus cumulative totals.
// There is deliberately no "current seller" — every session is independent
// state keyed by its own id, so N concurrent sellers coexist and a reconnect
// is simply close-then-open.
type sessionRegistry struct {
	mu          sync.Mutex
	seq         uint64
	open        map[uint64]*sessionInfo
	totalOpened uint64
	totalClosed uint64
	totalMsgs   uint64
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{open: make(map[uint64]*sessionInfo)}
}

// openSession registers a new session and returns its id.
func (r *sessionRegistry) openSession(peerID, remoteAddr string) uint64 {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	r.totalOpened++
	r.open[r.seq] = &sessionInfo{
		ID:          r.seq,
		PeerID:      peerID,
		RemoteAddr:  remoteAddr,
		ConnectedAt: now,
		LastSeen:    now,
	}
	return r.seq
}

// observe records one forwarded message: bumps the count, refreshes last-seen,
// and pins the session's node_id from the first message that carries one.
func (r *sessionRegistry) observe(id uint64, nodeID string, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.totalMsgs++
	s, ok := r.open[id]
	if !ok {
		return
	}
	s.MessageCount++
	s.LastSeen = at
	if s.NodeID == "" && nodeID != "" {
		s.NodeID = nodeID
	}
}

// closeSession removes the session from the open set.
func (r *sessionRegistry) closeSession(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.open[id]; ok {
		delete(r.open, id)
		r.totalClosed++
	}
}

// snapshot returns a copy of the open sessions, sorted by id.
func (r *sessionRegistry) snapshot() []sessionInfo {
	r.mu.Lock()
	out := make([]sessionInfo, 0, len(r.open))
	for _, s := range r.open {
		out = append(out, *s)
	}
	r.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// totals reports the cumulative opened/closed/message counters.
func (r *sessionRegistry) totals() (opened, closed, msgs uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totalOpened, r.totalClosed, r.totalMsgs
}

// edgeCounter is the optional downstream-edge observability hook
// (*sapient.FeedServer implements it); nil is fine in tests.
type edgeCounter interface {
	ClientCount() int
}

// startSessionsHTTP serves the read-only session observability endpoint on
// addr (loopback by convention): GET /sessions → {"count":N,"sessions":[…]}.
// It returns the server and the bound address (addr may carry port 0).
func startSessionsHTTP(ctx context.Context, addr string, reg *sessionRegistry, edge edgeCounter, logger *log.Logger) (*http.Server, string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", fmt.Errorf("sessions-http listen %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sessions", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sessions := reg.snapshot()
		opened, closed, msgs := reg.totals()
		body := map[string]any{
			"count":       len(sessions),
			"sessions":    sessions,
			"totalOpened": opened,
			"totalClosed": closed,
			"totalMsgs":   msgs,
		}
		if edge != nil {
			body["edgeClients"] = edge.ClientCount()
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(body)
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	logger.Printf("sessions endpoint listening http://%s/sessions", ln.Addr())
	return srv, ln.Addr().String(), nil
}

// sessionSummaryLoop logs a one-line fleet summary every interval — the
// always-on observability for a headless buyer (systemd journal).
func sessionSummaryLoop(ctx context.Context, reg *sessionRegistry, edge edgeCounter, interval time.Duration, logger *log.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sessions := reg.snapshot()
			opened, closed, msgs := reg.totals()
			var line strings.Builder
			fmt.Fprintf(&line, "sessions=%d totalOpened=%d totalClosed=%d totalMsgs=%d", len(sessions), opened, closed, msgs)
			if edge != nil {
				fmt.Fprintf(&line, " edgeClients=%d", edge.ClientCount())
			}
			for _, s := range sessions {
				fmt.Fprintf(&line, " [peer=%s node_id=%s msgs=%d lastSeen=%s]",
					s.PeerID, s.NodeID, s.MessageCount, s.LastSeen.Format(time.RFC3339))
			}
			logger.Print(line.String())
		}
	}
}
