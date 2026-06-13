// Command sapient-fid-display is the SAPIENT-rich Flight Information Display
// for the local SAPIENT Remote ID demo.
//
// It is a deliberately SEPARATE sibling of cmd/fid-display: that binary is
// shared with the public reference demo and is never modified for SAPIENT work
// (only its patterns are reused). This display accepts ONLY the rich
// sapient-track stream from cmd/sapient-fid-consumer (--sapient-output) —
// TaggedFrame{source:"sapient", type:"track"} JSONL over TCP, carrying the
// SapientTrackSnapshot contract (docs/sapient-track-contract.md) — and renders
// drone + operator markers with the military/identity context the remote-id
// projection drops: SAPIENT ids, classification/confidence, RF signal, CoT
// affiliation, and the seller's EIP-8004 Agent Card identity.
//
// Honest-labeling rules (surfaced by the UI, carried as data here):
//   - FRIENDLY is a demo CoT display profile (cot.demoProfile=true), not an
//     assessment.
//   - agent.simulated=true means the EIP-8004 registration is on the local
//     in-memory registry (SIM), not a real chain.
//   - The canonical wire is SAPIENT protobuf (BSI Flex 335 v2.0); this display
//     is a lossy projection.
//
// Endpoints (same shape as fid-display):
//
//	GET /              — embedded Leaflet map UI
//	GET /state.json    — current track state {tracks:[…], count:N} plus an
//	                     additive `focus` block (densest live drone cluster)
//	                     when at least one track has a usable position
//	GET /config.json   — initial map center / zoom from CLI flags
//	GET /events        — SSE stream of TrackSnapshot updates
//
// Ports default to 127.0.0.1:8193 (HTTP) and 127.0.0.1:19194 (TCP) — distinct
// from the public FID (8080/9090) and the demo's compatibility FID (8192/19191).
package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed static/*
var staticFS embed.FS

// TaggedFrame mirrors the consumer's on-the-wire envelope (field names only).
type TaggedFrame struct {
	Source       string          `json:"source"`
	Type         string          `json:"type"`
	SellerPeerID string          `json:"sellerPeerID"`
	ReceivedAt   time.Time       `json:"receivedAt"`
	Frame        json.RawMessage `json:"frame"`
}

// TrackSnapshot is the per-track state surfaced to the UI — the
// SapientTrackSnapshot contract (docs/sapient-track-contract.md v1.0.0)
// mirrored field-NAME-only, plus envelope/display bookkeeping. Nested blocks
// stay pointers: nil = the producer omitted the block, and the UI renders "—"
// rather than zero-filled values.
type TrackSnapshot struct {
	// Contract fields (decoded straight from TaggedFrame.Frame).
	ContractType    string          `json:"type,omitempty"`    // "sapient-track"
	ContractVersion string          `json:"version,omitempty"` // "1.0.0"
	UID             string          `json:"uid"`
	ObjectID        string          `json:"objectId,omitempty"`
	ReportID        string          `json:"reportId,omitempty"`
	NodeID          string          `json:"nodeId,omitempty"`
	Position        *Position       `json:"position,omitempty"`
	Velocity        *Velocity       `json:"velocity,omitempty"`
	Classification  *Classification `json:"classification,omitempty"`
	FeedSource      string          `json:"feedSource,omitempty"`
	Wire            string          `json:"wire,omitempty"`
	RID             *RIDInfo        `json:"rid,omitempty"`
	RF              *RFInfo         `json:"rf,omitempty"`
	CoT             *CoTInfo        `json:"cot,omitempty"`
	Agent           *AgentInfo      `json:"agent,omitempty"`
	// v1.1.0 contract additions (multi-source): both omitted on rid tracks.
	Kind string    `json:"kind,omitempty"` // "" (rid, legacy) | "adsb"
	ADSB *ADSBInfo `json:"adsb,omitempty"` // adsb.* object_info projection

	// Display bookkeeping (stamped by Apply, not part of the contract).
	Source       string    `json:"source"`       // always "sapient"
	SellerPeerID string    `json:"sellerPeerID"` // envelope value (usually empty: proxy stripped it)
	FrameCount   uint64    `json:"frameCount"`
	LastSeen     time.Time `json:"lastSeen"`
}

// ADSBInfo mirrors the consumer's adsb block (field names only): the
// neuron.adsb/1 object_info projection for JetVision aircraft tracks.
type ADSBInfo struct {
	ICAO24          string   `json:"icao24,omitempty"`
	Callsign        string   `json:"callsign,omitempty"`
	Registration    string   `json:"registration,omitempty"`
	TypeCode        string   `json:"typeCode,omitempty"`
	Operator        string   `json:"operator,omitempty"`
	OriginICAO      string   `json:"originIcao,omitempty"`
	DestICAO        string   `json:"destIcao,omitempty"`
	Country         string   `json:"country,omitempty"`
	EmitterCategory string   `json:"emitterCategory,omitempty"`
	Squawk          string   `json:"squawk,omitempty"`
	Emergency       string   `json:"emergency,omitempty"`
	AirGround       string   `json:"airGround,omitempty"`
	Source          string   `json:"source,omitempty"`
	Provenance      string   `json:"provenance,omitempty"`
	SignalDbm       *float64 `json:"signalDbm,omitempty"`
	BaroAltFt       *float64 `json:"baroAltFt,omitempty"`
	GeoAltFt        *float64 `json:"geoAltFt,omitempty"`
}

// Position is the track's WGS84 position.
type Position struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	Alt float64 `json:"alt"`
}

// Velocity is horizontal speed + true-track heading; the block is omitted by
// the producer when the DetectionReport carried no velocity.
type Velocity struct {
	SpeedMps float64 `json:"speedMps"`
	TrackDeg float64 `json:"trackDeg"`
}

// Classification is the SAPIENT classification[0] + confidence.
type Classification struct {
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence,omitempty"`
}

// RIDInfo carries the Remote ID identity fields (rid.* object_info).
// Operator lat/lon are pointers: both-present or absent (half-populated
// operators are suppressed producer-side).
type RIDInfo struct {
	Serial         string   `json:"serial,omitempty"`
	UASID          string   `json:"uasId,omitempty"`
	IDType         string   `json:"idType,omitempty"`
	UAType         string   `json:"uaType,omitempty"`
	MacAddress     string   `json:"macAddress,omitempty"`
	Status         string   `json:"status,omitempty"`
	OperatorID     string   `json:"operatorId,omitempty"`
	OperatorIDType string   `json:"operatorIdType,omitempty"`
	OperatorLat    *float64 `json:"operatorLat,omitempty"`
	OperatorLon    *float64 `json:"operatorLon,omitempty"`
	OperatorAltM   *float64 `json:"operatorAltM,omitempty"`
}

// RFInfo carries the RF signal context (SAPIENT signal[0] + rid transport).
type RFInfo struct {
	RssiDbm     *float64 `json:"rssiDbm,omitempty"`
	FrequencyHz *float64 `json:"frequencyHz,omitempty"`
	Channel     string   `json:"channel,omitempty"`
	Transport   string   `json:"transport,omitempty"`
}

// CoTInfo is the producer's CoT projection metadata. DemoProfile=true flags an
// operator-selected friendly affiliation (display choice, not an assessment).
type CoTInfo struct {
	UID         string `json:"uid,omitempty"`
	Type        string `json:"type,omitempty"`
	How         string `json:"how,omitempty"`
	Affiliation string `json:"affiliation,omitempty"`
	DemoProfile bool   `json:"demoProfile,omitempty"`
}

// AgentInfo is the seller's EIP-8004 Agent Card identity (from the evidence
// record). Simulated=true = in-memory registry (SIM), not real on-chain.
type AgentInfo struct {
	AgentID   string `json:"agentId,omitempty"`
	SellerEVM string `json:"sellerEVM,omitempty"`
	PeerID    string `json:"peerID,omitempty"`
	NodeID    string `json:"nodeId,omitempty"`
	Service   string `json:"service,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	Simulated bool   `json:"simulated,omitempty"`
}

// SSEEvent is the /events payload. Kind is always "sapient-track".
type SSEEvent struct {
	Kind  string         `json:"kind"`
	Track *TrackSnapshot `json:"track,omitempty"`
}

// State holds one TrackSnapshot per source-scoped track key plus the SSE
// fan-out. Single keyspace: this display is SAPIENT-only by design (the
// operator marker is derived client-side from the embedded rid.operator*
// fields).
type State struct {
	mu     sync.RWMutex
	tracks map[string]TrackSnapshot // keyed by trackKey (nodeId|uid)

	subMu sync.Mutex
	subs  map[chan SSEEvent]struct{}
}

// trackKey scopes track identity by the source node: two sellers can mint
// colliding uids, so a track is "nodeId|uid" (multi-source rule: never keyed
// by object identity alone). Legacy frames without a nodeId keep the bare uid.
func trackKey(s TrackSnapshot) string {
	if s.NodeID != "" {
		return s.NodeID + "|" + s.UID
	}
	return s.UID
}

// NewState returns an empty State.
func NewState() *State {
	return &State{
		tracks: make(map[string]TrackSnapshot),
		subs:   make(map[chan SSEEvent]struct{}),
	}
}

// Apply parses one TaggedFrame and updates the track state. Only
// source="sapient" + type="track" is accepted — anything else is an error
// (logged + skipped by the ingest loop); this display is not a generic FID.
func (s *State) Apply(tf TaggedFrame) (SSEEvent, error) {
	if tf.Source != "sapient" {
		return SSEEvent{}, fmt.Errorf("unknown source %q (this display accepts only \"sapient\")", tf.Source)
	}
	if tf.Type != "track" {
		return SSEEvent{}, fmt.Errorf("unknown type %q (want \"track\")", tf.Type)
	}

	var snap TrackSnapshot
	if err := json.Unmarshal(tf.Frame, &snap); err != nil {
		return SSEEvent{}, fmt.Errorf("decode inner sapient-track frame: %w", err)
	}
	if snap.UID == "" {
		return SSEEvent{}, errors.New("inner sapient-track frame missing uid")
	}

	snap.Source = tf.Source
	snap.SellerPeerID = tf.SellerPeerID
	snap.LastSeen = tf.ReceivedAt
	snap.FrameCount = 1

	key := trackKey(snap)
	s.mu.Lock()
	if prev, ok := s.tracks[key]; ok {
		snap.FrameCount = prev.FrameCount + 1
	}
	s.tracks[key] = snap
	s.mu.Unlock()

	event := SSEEvent{Kind: "sapient-track", Track: &snap}
	s.broadcast(event)
	return event, nil
}

// Snapshot returns a copy of every track currently held.
func (s *State) Snapshot() []TrackSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TrackSnapshot, 0, len(s.tracks))
	for _, t := range s.tracks {
		out = append(out, t)
	}
	return out
}

// Evict drops tracks whose LastSeen is older than now-maxAge. maxAge <= 0 is
// a no-op. Returns the count evicted.
func (s *State) Evict(now time.Time, maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}
	cutoff := now.Add(-maxAge)
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	for key, t := range s.tracks {
		if t.LastSeen.Before(cutoff) {
			delete(s.tracks, key)
			n++
		}
	}
	return n
}

// broadcast best-effort fan-out; slow subscribers drop events.
func (s *State) broadcast(event SSEEvent) {
	s.subMu.Lock()
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
	s.subMu.Unlock()
}

// Subscribe registers an SSE listener; pair with Unsubscribe.
func (s *State) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 32)
	s.subMu.Lock()
	s.subs[ch] = struct{}{}
	s.subMu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (s *State) Unsubscribe(ch chan SSEEvent) {
	s.subMu.Lock()
	delete(s.subs, ch)
	s.subMu.Unlock()
}

// MapConfig is the /config.json payload.
type MapConfig struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Zoom int     `json:"zoom"`
}

// ─── Auto-focus ────────────────────────────────────────────────────────────

// Focus is the suggested initial viewport: the densest cluster of live drone
// tracks. Served additively on /state.json; the client applies it on first
// load (and via the "live drones" recenter control) but never after the
// operator has panned/zoomed manually.
type Focus struct {
	Count  int     `json:"count"` // cluster members (drone tracks only)
	Lat    float64 `json:"lat"`   // centroid of member drone positions
	Lon    float64 `json:"lon"`
	MinLat float64 `json:"minLat"` // bounds incl. members' nearby operators
	MinLon float64 `json:"minLon"`
	MaxLat float64 `json:"maxLat"`
	MaxLon float64 `json:"maxLon"`
}

const (
	// focusRadiusM groups drone tracks into one cluster.
	focusRadiusM = 1500.0
	// operatorLeashM bounds how far a member's operator marker may sit from
	// its drone and still be pulled into the viewport. Operators NEVER drive
	// cluster selection — they only extend the bounds for visibility.
	operatorLeashM = 2000.0
)

// validCoord rejects the unusable coordinates the wire can carry: NaN/Inf,
// out-of-range, and the (0,0) null island that absent fields decode to.
func validCoord(lat, lon float64) bool {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		return false
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return false
	}
	return lat != 0 || lon != 0
}

// haversineM is the great-circle distance in metres.
func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6_371_000.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusM * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// FocusFor picks the densest live drone cluster. Selection uses DRONE
// positions only (a track's operator marker never drives focus); every track
// in state is "live" by definition (the eviction loop drops stale ones).
// Returns nil when no track has a valid drone position — the client then
// keeps its default view. Deterministic: ties break on the smallest UID.
//
// Multi-source rule: kind=adsb (aircraft) tracks never drive focus while a
// drone is on the map — an airliner cluster must not hijack the demo's
// viewport. When ONLY aircraft are present (JV standalone) they become the
// fallback candidates so the map still opens somewhere useful.
func FocusFor(snaps []TrackSnapshot) *Focus {
	type cand struct {
		uid      string
		lat, lon float64
		opLat    *float64
		opLon    *float64
	}
	var cands, adsbCands []cand
	for _, s := range snaps {
		if s.Position == nil || !validCoord(s.Position.Lat, s.Position.Lon) {
			continue
		}
		c := cand{uid: s.UID, lat: s.Position.Lat, lon: s.Position.Lon}
		if s.RID != nil {
			c.opLat, c.opLon = s.RID.OperatorLat, s.RID.OperatorLon
		}
		if s.Kind == "adsb" {
			adsbCands = append(adsbCands, c)
			continue
		}
		cands = append(cands, c)
	}
	if len(cands) == 0 {
		cands = adsbCands // aircraft-only state: fall back rather than no focus
	}
	if len(cands) == 0 {
		return nil
	}

	// Densest neighbourhood wins; smallest UID breaks ties deterministically.
	best, bestCount := -1, -1
	for i := range cands {
		n := 0
		for j := range cands {
			if haversineM(cands[i].lat, cands[i].lon, cands[j].lat, cands[j].lon) <= focusRadiusM {
				n++
			}
		}
		if n > bestCount || (n == bestCount && cands[i].uid < cands[best].uid) {
			best, bestCount = i, n
		}
	}

	f := &Focus{MinLat: math.MaxFloat64, MinLon: math.MaxFloat64, MaxLat: -math.MaxFloat64, MaxLon: -math.MaxFloat64}
	var sumLat, sumLon float64
	for _, c := range cands {
		if haversineM(cands[best].lat, cands[best].lon, c.lat, c.lon) > focusRadiusM {
			continue
		}
		f.Count++
		sumLat += c.lat
		sumLon += c.lon
		f.MinLat = math.Min(f.MinLat, c.lat)
		f.MinLon = math.Min(f.MinLon, c.lon)
		f.MaxLat = math.Max(f.MaxLat, c.lat)
		f.MaxLon = math.Max(f.MaxLon, c.lon)
		// Keep the member's pilot visible when plausibly co-located.
		if c.opLat != nil && c.opLon != nil && validCoord(*c.opLat, *c.opLon) &&
			haversineM(c.lat, c.lon, *c.opLat, *c.opLon) <= operatorLeashM {
			f.MinLat = math.Min(f.MinLat, *c.opLat)
			f.MinLon = math.Min(f.MinLon, *c.opLon)
			f.MaxLat = math.Max(f.MaxLat, *c.opLat)
			f.MaxLon = math.Max(f.MaxLon, *c.opLon)
		}
	}
	f.Lat = sumLat / float64(f.Count)
	f.Lon = sumLon / float64(f.Count)
	return f
}

// orUnset renders an optional flag value for the startup log line.
func orUnset(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}

// multiFlag collects a repeatable string flag (--evidence may appear once per
// seller).
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	var evidencePaths multiFlag
	var (
		httpAddr  = flag.String("http", "127.0.0.1:8193", "HTTP UI bind address")
		tcpAddr   = flag.String("tcp", "127.0.0.1:19194", "TCP listen address for sapient-track JSONL (consumer --sapient-output)")
		fixture   = flag.String("fixture", "", "JSONL fixture file to replay at startup (in addition to TCP listener)")
		centerLat = flag.Float64("lat", 50.1027, "initial map center latitude (default: Land's End demo orbit)")
		centerLon = flag.Float64("lon", -5.6705, "initial map center longitude")
		zoom      = flag.Int("zoom", 13, "initial map zoom (1-19)")
		evictAge  = flag.Duration("evict", 10*time.Minute, "drop tracks not seen for this duration; 0 disables")
		// Multi-source context (all additive; everything works unset).
		sessionsURL  = flag.String("sessions-url", "", "buyer /sessions endpoint for per-source session health (e.g. http://127.0.0.1:19201/sessions); empty disables")
		sessionsPoll = flag.Duration("sessions-poll", 5*time.Second, "poll cadence for --sessions-url")
		evidenceDir  = flag.String("evidence-dir", "", "directory of seller agent-evidence JSON files (source cards); additive to --evidence")
		sensorsPath  = flag.String("sensors", "", "operator-provided sensor-locations.json (enables the sensor layer; off when empty)")
		staleAfter   = flag.Duration("stale-after", 60*time.Second, "tracks/sessions older than this render STALE (display threshold, distinct from --evict)")
	)
	flag.Var(&evidencePaths, "evidence", "seller agent-evidence JSON (seller --registry-out) backing a SOURCES card; repeatable, one per seller")
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)

	state := NewState()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prov := &SourceProvider{
		Loader:     newSeedLoader(evidencePaths, *evidenceDir, logger),
		SessionURL: *sessionsURL,
		StaleAfter: *staleAfter,
	}
	if *sessionsURL != "" {
		prov.Poller = NewSessionsPoller(*sessionsURL)
		go prov.Poller.Run(ctx, *sessionsPoll, logger)
		logger.Printf("sessions feed ON: %s every %s", *sessionsURL, *sessionsPoll)
	}
	logger.Printf("sources: %d evidence path(s) + dir=%s (lazy); stale-after=%s; sensors=%s",
		len(evidencePaths), orUnset(*evidenceDir), *staleAfter, orUnset(*sensorsPath))

	if *fixture != "" {
		go replayFixture(ctx, *fixture, state, logger)
	}

	listener, err := net.Listen("tcp", *tcpAddr)
	if err != nil {
		logger.Fatalf("tcp listen %s: %v", *tcpAddr, err)
	}
	defer listener.Close()
	logger.Printf("listening for sapient-track JSONL on tcp://%s", *tcpAddr)
	go acceptLoop(ctx, listener, state, logger)

	if *evictAge > 0 {
		go evictionLoop(ctx, state, *evictAge, logger)
	}

	cfg := MapConfig{Lat: *centerLat, Lon: *centerLon, Zoom: *zoom}
	mux := http.NewServeMux()
	mux.Handle("/", indexHandler())
	mux.HandleFunc("/config.json", configHandler(cfg))
	mux.HandleFunc("/state.json", stateHandlerFull(state, prov))
	mux.HandleFunc("/sources.json", sourcesHandler(state, prov))
	mux.HandleFunc("/sensors.json", sensorsHandler(*sensorsPath))
	mux.HandleFunc("/events", eventsHandler(state))

	srv := &http.Server{
		Addr:              *httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Printf("SAPIENT FID UI on http://%s", *httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("http server: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Printf("shutting down")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = listener.Close()
	logger.Printf("shutdown complete")
}

func acceptLoop(ctx context.Context, ln net.Listener, state *State, logger *log.Logger) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Printf("accept: %v", err)
			continue
		}
		logger.Printf("producer connected: %s", conn.RemoteAddr())
		go handleConn(ctx, conn, state, logger)
	}
}

func handleConn(ctx context.Context, conn net.Conn, state *State, logger *log.Logger) {
	defer conn.Close()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)
	var seen, applied int
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		seen++
		var tf TaggedFrame
		if err := json.Unmarshal(scanner.Bytes(), &tf); err != nil {
			logger.Printf("malformed JSONL from %s: %v", conn.RemoteAddr(), err)
			continue
		}
		if _, err := state.Apply(tf); err != nil {
			logger.Printf("apply from %s: %v", conn.RemoteAddr(), err)
			continue
		}
		applied++
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
		logger.Printf("scanner from %s: %v", conn.RemoteAddr(), err)
	}
	logger.Printf("producer disconnected: %s (seen=%d applied=%d)", conn.RemoteAddr(), seen, applied)
}

func replayFixture(ctx context.Context, path string, state *State, logger *log.Logger) {
	f, err := os.Open(path)
	if err != nil {
		logger.Printf("fixture open: %v", err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)
	var seen, applied int
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		seen++
		var tf TaggedFrame
		if err := json.Unmarshal(scanner.Bytes(), &tf); err != nil {
			continue
		}
		if _, err := state.Apply(tf); err == nil {
			applied++
		}
	}
	logger.Printf("fixture %s replayed seen=%d applied=%d", path, seen, applied)
}

func evictionLoop(ctx context.Context, state *State, maxAge time.Duration, logger *log.Logger) {
	ticker := time.NewTicker(maxAge / 4)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			if n := state.Evict(t, maxAge); n > 0 {
				logger.Printf("evicted %d stale track(s)", n)
			}
		}
	}
}

// HTTP handlers ----------------------------------------------------

func indexHandler() http.HandlerFunc {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(fmt.Errorf("embed static/: %w", err))
	}
	fileServer := http.FileServer(http.FS(sub))
	return func(w http.ResponseWriter, r *http.Request) {
		fileServer.ServeHTTP(w, r)
	}
}

func configHandler(cfg MapConfig) http.HandlerFunc {
	body, _ := json.Marshal(cfg)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
}

// legacyStatePayload is the single source of truth for the original
// /state.json shape (tracks/count/focus). stateHandler serves exactly this;
// stateHandlerFull layers additive fields on top — sharing the builder means
// the legacy contract cannot drift.
func legacyStatePayload(tracks []TrackSnapshot) map[string]any {
	payload := map[string]any{
		"tracks": tracks,
		"count":  len(tracks),
	}
	// Additive: the densest live drone cluster for the client's initial
	// viewport fit; omitted when no track has a usable position.
	if f := FocusFor(tracks); f != nil {
		payload["focus"] = f
	}
	return payload
}

func stateHandler(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(legacyStatePayload(state.Snapshot()))
	}
}

// trackView decorates a TrackSnapshot with display-time fields. Additive by
// construction: the embedded snapshot serializes exactly as before, plus
// ageMs/stale. TrackSnapshot itself stays frozen (tests marshal it directly).
type trackView struct {
	TrackSnapshot
	AgeMs int64 `json:"ageMs"`
	Stale bool  `json:"stale"`
}

func trackViews(tracks []TrackSnapshot, now time.Time, staleAfter time.Duration) []trackView {
	out := make([]trackView, 0, len(tracks))
	for _, t := range tracks {
		age := now.Sub(t.LastSeen)
		out = append(out, trackView{
			TrackSnapshot: t,
			AgeMs:         age.Milliseconds(),
			Stale:         age > staleAfter,
		})
	}
	return out
}

// SourceProvider wires the additive multi-source context: lazily-loaded
// evidence seeds (sellers write evidence AFTER the display starts), the
// optional sessions poller, and the stale threshold.
type SourceProvider struct {
	Loader     *seedLoader
	Poller     *SessionsPoller // nil = no sessions feed configured
	SessionURL string
	StaleAfter time.Duration
}

// Sources builds the per-source views plus the sessions-feed meta block.
func (p *SourceProvider) Sources(tracks []TrackSnapshot, now time.Time) ([]SourceView, map[string]any) {
	var (
		snap  *SessionsSnapshot
		rates map[string]*float64
	)
	meta := map[string]any{"configured": p.Poller != nil}
	if p.Poller != nil {
		cur := p.Poller.Current()
		snap = &cur
		meta["url"] = p.SessionURL
		if !cur.PolledAt.IsZero() {
			meta["lastPollAt"] = cur.PolledAt.UTC().Format(time.RFC3339)
		}
		if cur.Err != "" {
			meta["error"] = cur.Err
		}
		rates = make(map[string]*float64, len(cur.Entries))
		for _, e := range cur.Entries {
			rates[e.PeerID] = p.Poller.RateFor(e.PeerID, now)
		}
	}
	return BuildSources(p.Loader.seeds(), snap, rates, tracks, now, p.StaleAfter), meta
}

// stateHandlerFull serves the additive multi-source /state.json: the exact
// legacy payload plus tracks decorated with ageMs/stale, sources[], the stale
// threshold, the server clock (client skew correction), and sessions-feed meta.
func stateHandlerFull(state *State, prov *SourceProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		tracks := state.Snapshot()
		payload := legacyStatePayload(tracks)
		payload["tracks"] = trackViews(tracks, now, prov.StaleAfter) // superset per track
		sources, sessMeta := prov.Sources(tracks, now)
		payload["sources"] = sources
		payload["staleAfterMs"] = prov.StaleAfter.Milliseconds()
		payload["now"] = now.UTC().Format(time.RFC3339Nano)
		payload["sessions"] = sessMeta
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// sourcesHandler serves the sources block alone (rail polling / curl checks).
func sourcesHandler(state *State, prov *SourceProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		sources, sessMeta := prov.Sources(state.Snapshot(), now)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sources":      sources,
			"count":        len(sources),
			"staleAfterMs": prov.StaleAfter.Milliseconds(),
			"now":          now.UTC().Format(time.RFC3339Nano),
			"sessions":     sessMeta,
		})
	}
}

func eventsHandler(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := state.Subscribe()
		defer state.Unsubscribe(ch)

		// On connect, send the current tracks as a snapshot batch.
		for _, t := range state.Snapshot() {
			payload, _ := json.Marshal(SSEEvent{Kind: "sapient-track", Track: &t})
			fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload)
		}
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case event := <-ch:
				payload, _ := json.Marshal(event)
				fmt.Fprintf(w, "event: update\ndata: %s\n\n", payload)
				flusher.Flush()
			}
		}
	}
}
