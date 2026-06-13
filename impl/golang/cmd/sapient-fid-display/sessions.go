package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// SessionEntry mirrors cmd/sapient-buyer's sessionInfo (field names only — the
// buyer's read-only GET /sessions contract). nodeId may be empty until the
// session's first message (the buyer captures it from the first decoded
// SapientMessage), so matching falls back to peerID.
type SessionEntry struct {
	ID           uint64    `json:"id"`
	PeerID       string    `json:"peerID"`
	RemoteAddr   string    `json:"remoteAddr"`
	NodeID       string    `json:"nodeId"`
	ConnectedAt  time.Time `json:"connectedAt"`
	LastSeen     time.Time `json:"lastSeen"`
	MessageCount uint64    `json:"messageCount"`
}

// SessionsSnapshot is the poller's latest view. Err carries the last poll
// failure verbatim (surfaced in the UI as "buyer /sessions unreachable") — a
// failing poll must degrade honestly, never silently.
type SessionsSnapshot struct {
	PolledAt time.Time
	Entries  []SessionEntry
	Err      string
}

// ratePoint is one cumulative-counter sample for message-rate computation.
type ratePoint struct {
	At         time.Time
	CountByKey map[string]uint64 // keyed by session peerID (stable pre-nodeId)
}

// rateWindow is the sliding window message rates are computed over.
const rateWindow = 60 * time.Second

// SessionsPoller polls the buyer's /sessions endpoint and keeps a short ring
// of cumulative counts for rate computation. All methods are safe for
// concurrent use; a nil poller (sessions-url unset) is handled by callers.
type SessionsPoller struct {
	url    string
	client *http.Client

	mu   sync.RWMutex
	cur  SessionsSnapshot
	ring []ratePoint
}

func NewSessionsPoller(url string) *SessionsPoller {
	return &SessionsPoller{
		url:    url,
		client: &http.Client{Timeout: 3 * time.Second},
	}
}

// Run polls every interval until ctx is done. The first poll is immediate.
func (p *SessionsPoller) Run(ctx context.Context, interval time.Duration, logger *log.Logger) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	p.pollOnce(ctx, logger)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce(ctx, logger)
		}
	}
}

func (p *SessionsPoller) pollOnce(ctx context.Context, logger *log.Logger) {
	now := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		p.setErr(now, err.Error())
		return
	}
	resp, err := p.client.Do(req)
	if err != nil {
		p.setErr(now, err.Error())
		logger.Printf("sessions poll: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		p.setErr(now, fmt.Sprintf("GET %s: HTTP %d", p.url, resp.StatusCode))
		return
	}
	var body struct {
		Sessions []SessionEntry `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		p.setErr(now, "decode /sessions: "+err.Error())
		return
	}

	counts := make(map[string]uint64, len(body.Sessions))
	for _, s := range body.Sessions {
		counts[s.PeerID] = s.MessageCount
	}

	p.mu.Lock()
	p.cur = SessionsSnapshot{PolledAt: now, Entries: body.Sessions}
	p.ring = append(p.ring, ratePoint{At: now, CountByKey: counts})
	// Keep a little more than the rate window so the oldest in-window sample
	// always has a predecessor.
	cutoff := now.Add(-rateWindow - 30*time.Second)
	for len(p.ring) > 0 && p.ring[0].At.Before(cutoff) {
		p.ring = p.ring[1:]
	}
	p.mu.Unlock()
}

func (p *SessionsPoller) setErr(now time.Time, msg string) {
	p.mu.Lock()
	p.cur = SessionsSnapshot{PolledAt: now, Entries: p.cur.Entries, Err: msg}
	p.mu.Unlock()
}

// Current returns the latest snapshot (copy).
func (p *SessionsPoller) Current() SessionsSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := p.cur
	out.Entries = append([]SessionEntry(nil), p.cur.Entries...)
	return out
}

// RateFor returns the messages/second for a session peerID over the rate
// window, or nil when unknown (fewer than two samples).
func (p *SessionsPoller) RateFor(peerID string, now time.Time) *float64 {
	p.mu.RLock()
	ring := append([]ratePoint(nil), p.ring...)
	p.mu.RUnlock()
	return messageRate(ring, peerID, rateWindow, now)
}

// messageRate computes msgs/sec for key from cumulative samples over window.
// Pure (unit-tested). Returns nil when fewer than two samples carry the key —
// "unknown" is honest, 0.0 is a claim. A negative delta (counter reset after a
// session reconnect) restarts measurement from that sample.
func messageRate(samples []ratePoint, key string, window time.Duration, now time.Time) *float64 {
	cutoff := now.Add(-window)
	var pts []struct {
		at    time.Time
		count uint64
	}
	for _, s := range samples {
		if s.At.Before(cutoff) {
			continue
		}
		if c, ok := s.CountByKey[key]; ok {
			pts = append(pts, struct {
				at    time.Time
				count uint64
			}{s.At, c})
		}
	}
	if len(pts) < 2 {
		return nil
	}
	// Walk forward; on a counter reset, restart the measurement base.
	base := 0
	for i := 1; i < len(pts); i++ {
		if pts[i].count < pts[i-1].count {
			base = i
		}
	}
	first, last := pts[base], pts[len(pts)-1]
	elapsed := last.at.Sub(first.at).Seconds()
	if elapsed <= 0 {
		return nil
	}
	rate := float64(last.count-first.count) / elapsed
	return &rate
}
