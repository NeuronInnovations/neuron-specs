package main

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
)

// Source status vocabulary. Rules:
//   - live:    a buyer session for this source exists and its lastSeen is
//     within the stale threshold (rate 0.0/s stays LIVE — honest);
//   - stale:   a session exists but nothing was seen within the threshold
//     ("connected, no recent messages"), or — without a sessions feed — the
//     newest track is older than the threshold;
//   - offline: the sessions feed is healthy and shows NO session for this
//     source (the DroneScout truth since Jun 10);
//   - unknown: no sessions feed configured and no tracks to judge by.
//
// A failing sessions poll NEVER fakes offline: status degrades to the
// track-recency rule and the poll error is surfaced in the payload.
const (
	statusLive    = "live"
	statusStale   = "stale"
	statusOffline = "offline"
	statusUnknown = "unknown"
)

// HeartbeatStatus is the honest spec-005 posture for a source. The deployed
// sellers run the file audit-lane (no heartbeat topic), so the display reports
// "not-published" — it must never imply a heartbeat it cannot observe.
type HeartbeatStatus struct {
	Status string `json:"status"` // "not-published" (no heartbeat topic observed by this UI)
	Note   string `json:"note,omitempty"`
}

// CommerceStatus is the commerce posture parsed from the card: binding "none"
// means advertisement-only (no payment flow engaged).
type CommerceStatus struct {
	Binding string `json:"binding,omitempty"`
	Mode    string `json:"mode"` // "advertisement-only" | "advertised: <binding>"
}

// TrackCounts aggregates this source's tracks currently held by the display.
type TrackCounts struct {
	Total     int `json:"total"`
	Aircraft  int `json:"aircraft"`
	Drones    int `json:"drones"`
	Operators int `json:"operators"`
	Stale     int `json:"stale"`
}

// SourceView is one SOURCES/SELLERS card — the additive per-source block in
// /state.json. Identity comes from the seller's evidence file, liveness from
// the buyer's /sessions, payload counts from the display's own track state.
type SourceView struct {
	Name      string `json:"name"`
	NodeID    string `json:"nodeId,omitempty"`
	Service   string `json:"service,omitempty"`
	AgentID   string `json:"agentId,omitempty"`
	SellerEVM string `json:"sellerEVM,omitempty"`
	PeerID    string `json:"peerID,omitempty"`
	Simulated bool   `json:"simulated"`

	RegistryAddress string `json:"registryAddress,omitempty"`
	TransactionHash string `json:"transactionHash,omitempty"`
	FeedSource      string `json:"feedSource,omitempty"`

	Modality     string   `json:"modality,omitempty"` // "rid" | "adsb" | raw
	ExtensionID  string   `json:"extensionId,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	SensorModels []string `json:"sensorModels,omitempty"`

	Status               string     `json:"status"`
	SessionConnected     *bool      `json:"sessionConnected,omitempty"` // nil = unknown (no sessions feed)
	AwaitingFirstMessage bool       `json:"awaitingFirstMessage,omitempty"`
	SessionRemoteAddr    string     `json:"sessionRemoteAddr,omitempty"`
	LastSeen             *time.Time `json:"lastSeen,omitempty"` // max(session lastSeen, newest track)
	MessageCount         uint64     `json:"messageCount"`
	MessageRate          *float64   `json:"messageRate,omitempty"` // msgs/s over 60s; nil = unknown

	Heartbeat   HeartbeatStatus `json:"heartbeat"`
	Commerce    CommerceStatus  `json:"commerce"`
	TrackCounts TrackCounts     `json:"trackCounts"`

	Unregistered bool `json:"unregistered,omitempty"` // tracks observed, no evidence file
}

// sourceSeed is one evidence file plus its parsed card meta.
type sourceSeed struct {
	ev   sapient.AgentEvidence
	meta sapient.CardMeta
}

// seedLoader lazily loads evidence seeds. Sellers write their evidence AFTER
// registering — typically after the display starts — so every still-missing
// path is retried on each call until it loads (the consumer's evidenceLoader
// pattern); successes are cached. Safe for concurrent use.
type seedLoader struct {
	paths  []string
	dir    string
	logger *log.Logger

	mu      sync.Mutex
	loaded  map[string]sourceSeed // by path (explicit + dir-discovered)
	ordered []string              // load order for stable output
}

func newSeedLoader(paths []string, dir string, logger *log.Logger) *seedLoader {
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" {
			clean = append(clean, p)
		}
	}
	return &seedLoader{paths: clean, dir: dir, logger: logger, loaded: map[string]sourceSeed{}}
}

// seeds returns the currently-loaded seeds, attempting to load any missing
// paths first.
func (l *seedLoader) seeds() []sourceSeed {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tryLoad := func(p string) {
		if _, ok := l.loaded[p]; ok {
			return
		}
		ev, err := sapient.ReadEvidence(p)
		if err != nil {
			return // not written yet (or unreadable) — retry next call
		}
		l.loaded[p] = sourceSeed{ev: ev, meta: sapient.ParseCardMeta(ev.AgentURI)}
		l.ordered = append(l.ordered, p)
		l.logger.Printf("source evidence loaded: %s (service=%s nodeId=%s)", p, ev.Service, ev.NodeID)
	}
	for _, p := range l.paths {
		tryLoad(p)
	}
	if l.dir != "" {
		if matches, err := filepath.Glob(filepath.Join(l.dir, "*.json")); err == nil {
			for _, m := range matches {
				tryLoad(m)
			}
		}
	}
	out := make([]sourceSeed, 0, len(l.ordered))
	for _, p := range l.ordered {
		out = append(out, l.loaded[p])
	}
	return out
}

// sourceName derives the card title: sensor-family word + modality label
// ("JetVision Air!Squitter" + adsb → "JetVision ADS-B"; "DroneScout DS240" +
// rid → "DroneScout RID"); falls back to the service name, then a short node id.
func sourceName(ev sapient.AgentEvidence, meta sapient.CardMeta) string {
	family := ""
	if len(meta.SensorModels) > 0 {
		family, _, _ = strings.Cut(meta.SensorModels[0], " ")
	}
	modality := ""
	switch meta.Modality {
	case "adsb":
		modality = "ADS-B"
	case "rid":
		modality = "RID"
	default:
		modality = meta.Modality
	}
	if family != "" && modality != "" {
		return family + " " + modality
	}
	if ev.Service != "" {
		return ev.Service
	}
	if ev.NodeID != "" {
		return "source " + shortID(ev.NodeID)
	}
	return "unknown source"
}

func shortID(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:8] + "…"
}

// trackAgg is the per-nodeId aggregation of the display's current tracks.
type trackAgg struct {
	counts   TrackCounts
	newest   time.Time
	anyTrack bool
}

func aggregateTracks(tracks []TrackSnapshot, now time.Time, staleAfter time.Duration) map[string]trackAgg {
	agg := map[string]trackAgg{}
	for _, t := range tracks {
		a := agg[t.NodeID]
		a.anyTrack = true
		a.counts.Total++
		if t.Kind == "adsb" {
			a.counts.Aircraft++
		} else {
			a.counts.Drones++
		}
		if t.RID != nil && t.RID.OperatorLat != nil && t.RID.OperatorLon != nil {
			a.counts.Operators++
		}
		if now.Sub(t.LastSeen) > staleAfter {
			a.counts.Stale++
		}
		if t.LastSeen.After(a.newest) {
			a.newest = t.LastSeen
		}
		agg[t.NodeID] = a
	}
	return agg
}

// BuildSources is the pure merge of evidence seeds + the latest sessions
// snapshot + per-track aggregation. sess == nil means no sessions feed is
// configured (sessionConnected omitted); sess.Err != "" means the feed is
// configured but failing (degrade to track recency — never fake offline).
func BuildSources(seeds []sourceSeed, sess *SessionsSnapshot, rates map[string]*float64, tracks []TrackSnapshot, now time.Time, staleAfter time.Duration) []SourceView {
	agg := aggregateTracks(tracks, now, staleAfter)
	sessionsHealthy := sess != nil && sess.Err == ""

	bySessionNode := map[string]SessionEntry{}
	bySessionPeer := map[string]SessionEntry{}
	if sessionsHealthy {
		for _, e := range sess.Entries {
			if e.NodeID != "" {
				bySessionNode[e.NodeID] = e
			}
			bySessionPeer[e.PeerID] = e
		}
	}

	var out []SourceView
	seenNodes := map[string]bool{}
	for _, seed := range seeds {
		ev, meta := seed.ev, seed.meta
		sv := SourceView{
			Name:            sourceName(ev, meta),
			NodeID:          ev.NodeID,
			Service:         ev.Service,
			AgentID:         ev.AgentID,
			SellerEVM:       ev.SellerEVM,
			PeerID:          ev.PeerID,
			Simulated:       ev.Simulated,
			RegistryAddress: ev.RegistryAddress,
			TransactionHash: ev.TransactionHash,
			FeedSource:      ev.FeedSource,
			Modality:        meta.Modality,
			ExtensionID:     meta.ExtensionID,
			Capabilities:    meta.Capabilities,
			SensorModels:    meta.SensorModels,
			Heartbeat: HeartbeatStatus{
				Status: "not-published",
				Note:   "file audit-lane; no heartbeat topic observed by this display",
			},
			Commerce:    commerceStatus(meta),
			TrackCounts: agg[ev.NodeID].counts,
		}
		seenNodes[ev.NodeID] = true

		var session *SessionEntry
		if sessionsHealthy {
			if e, ok := bySessionNode[ev.NodeID]; ok {
				session = &e
			} else if e, ok := bySessionPeer[ev.PeerID]; ok {
				session = &e
			}
		}
		applyLiveness(&sv, session, sessionsHealthy, rates, agg[ev.NodeID], now, staleAfter)
		out = append(out, sv)
	}

	// Honest synthetic card for any track source with no evidence file.
	for nodeID, a := range agg {
		if seenNodes[nodeID] || !a.anyTrack {
			continue
		}
		sv := SourceView{
			Name:         fmt.Sprintf("unregistered source %s", shortID(nodeID)),
			NodeID:       nodeID,
			Unregistered: true,
			Heartbeat:    HeartbeatStatus{Status: "not-published"},
			Commerce:     CommerceStatus{Mode: "unknown"},
			TrackCounts:  a.counts,
		}
		var session *SessionEntry
		if sessionsHealthy {
			if e, ok := bySessionNode[nodeID]; ok {
				session = &e
			}
		}
		applyLiveness(&sv, session, sessionsHealthy, rates, a, now, staleAfter)
		out = append(out, sv)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// applyLiveness derives Status/SessionConnected/LastSeen/MessageCount/Rate.
func applyLiveness(sv *SourceView, session *SessionEntry, sessionsHealthy bool, rates map[string]*float64, a trackAgg, now time.Time, staleAfter time.Duration) {
	newest := a.newest
	if session != nil && session.LastSeen.After(newest) {
		newest = session.LastSeen
	}
	if !newest.IsZero() {
		t := newest
		sv.LastSeen = &t
	}

	switch {
	case session != nil:
		connected := true
		sv.SessionConnected = &connected
		sv.SessionRemoteAddr = session.RemoteAddr
		sv.MessageCount = session.MessageCount
		if rates != nil {
			sv.MessageRate = rates[session.PeerID]
		}
		switch {
		case session.MessageCount == 0:
			sv.Status = statusLive
			sv.AwaitingFirstMessage = true
		case now.Sub(session.LastSeen) <= staleAfter:
			sv.Status = statusLive
		default:
			sv.Status = statusStale
		}
	case sessionsHealthy:
		connected := false
		sv.SessionConnected = &connected
		sv.Status = statusOffline
	default:
		// No sessions feed (unset or failing): judge by track recency only.
		switch {
		case a.anyTrack && now.Sub(a.newest) <= staleAfter:
			sv.Status = statusLive
		case a.anyTrack:
			sv.Status = statusStale
		default:
			sv.Status = statusUnknown
		}
	}
}

func commerceStatus(meta sapient.CardMeta) CommerceStatus {
	switch meta.CommerceBinding {
	case "", "none":
		return CommerceStatus{Binding: meta.CommerceBinding, Mode: "advertisement-only"}
	default:
		return CommerceStatus{Binding: meta.CommerceBinding, Mode: "advertised: " + meta.CommerceBinding}
	}
}
