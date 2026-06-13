// Command fid-display is the reference demo Flight Information Display.
//
// It accepts the consolidated TaggedFrame JSONL stream from
// cmd/multistream-buyer over TCP (the buyer dials; the display listens)
// and renders ADS-B aircraft, Remote ID drones, and Remote ID operators
// on a simple Leaflet map served at http://<httpAddr>/.
//
// Endpoints:
//
//	GET /              — embedded HTML map with live drone markers
//	GET /state.json    — current fused display state
//	GET /config.json   — initial map center / zoom from CLI flags
//	GET /events        — Server-Sent Events stream of DroneSnapshot updates
//
// Flags:
//
//	--http <addr>       HTTP UI bind address (default 127.0.0.1:8080)
//	--tcp <addr>        TCP listen address for buyer connections (default 127.0.0.1:9090)
//	--fixture <path>    JSONL file to replay once at startup (in lieu of TCP)
//	--lat <float>       initial map center latitude (default 51.4775 — Heathrow approach)
//	--lon <float>       initial map center longitude (default -0.4614)
//	--zoom <int>        initial map zoom (default 13)
//	--evict <duration>  drop drones not seen for this duration; 0 disables (default 5m)
//
// Run end-to-end with the local fixture components:
//
//	# Terminal 1 — display
//	go run ./cmd/fid-display
//
//	# Terminal 2 — seller (synthetic-orbit source)
//	go run ./cmd/remoteid-seller --synth --synth-fps=2 --synth-drones=3
//	# (note the printed multiaddr)
//
//	# Terminal 3 — multistream buyer
//	go run ./cmd/multistream-buyer --mode=fixture-direct \
//	  --seller=role=remoteid,multiaddr=<multiaddr>,protocol=/ds240/raw/1.0.0 \
//	  --output=tcp:127.0.0.1:9090
//
//	# Browser
//	open http://127.0.0.1:8080
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
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

// TaggedFrame mirrors the cmd/multistream-buyer on-the-wire envelope. We
// keep the Frame field as RawMessage so we can pluck position +
// velocity from any nested-frame shape without binding to one specific
// DApp type.
type TaggedFrame struct {
	Source       string          `json:"source"`
	Type         string          `json:"type"`
	SellerPeerID string          `json:"sellerPeerID"`
	ReceivedAt   time.Time       `json:"receivedAt"`
	Frame        json.RawMessage `json:"frame"`
}

// DroneSnapshot is the per-drone state surfaced to the UI. One snapshot
// per droneId. SSE events carry the snapshot; /state.json returns the
// full set.
//
// FrameSource (added 2026-05-18, MVP Phase 1) propagates the inner
// RemoteIdFrame.Source field through to the browser so the UI can
// distinguish synthetic feeds (e.g. "basestation-tcp-synthetic" from
// the VPS-1 fake-DS240 bridge) from real-hardware feeds (e.g.
// "dronescout-ds400"). The browser flags substring-matches against
// "synth"/"synthetic" with a small SYN badge — see static/index.html.
type DroneSnapshot struct {
	DroneID          string    `json:"droneId"`
	Source           string    `json:"source"`
	SellerPeerID     string    `json:"sellerPeerID"`
	Lat              float64   `json:"lat"`
	Lon              float64   `json:"lon"`
	Alt              float64   `json:"alt"`
	Fix              string    `json:"fix"`
	SpeedHorizontal  float64   `json:"speedHorizontal"`
	Track            float64   `json:"track"`
	RegulatorVariant string    `json:"regulatorVariant,omitempty"`
	FrameSource      string    `json:"frameSource,omitempty"`
	LastSeen         time.Time `json:"lastSeen"`
}

// AircraftSnapshot is the per-aircraft state surfaced to the UI for
// ADS-B (spec 016) tagged frames. One snapshot per ICAO24 hex address.
//
// Position note: Phase 5 ships WITHOUT a Mode-S CPR position decoder.
// The Lat/Lon fields hold a deterministic placeholder position derived
// from the ICAO hash, scattered within a small bounding box around the
// configured map center. Each aircraft therefore renders at a stable
// (but FAKE) position so the dual-stream demo is visibly populated.
// Phase 6A (full ADS-B DApp migration) replaces this with real CPR
// position decode from DF=17 Extended Squitter frames.
//
// PositionFake = true is set on every aircraft as a UI cue.
type AircraftSnapshot struct {
	ICAO         string    `json:"icao"`
	Source       string    `json:"source"` // always "adsb"
	SellerEVM    string    `json:"sellerEVM"`
	SellerName   string    `json:"sellerName"`
	SellerPeerID string    `json:"sellerPeerID"`
	DF           int       `json:"df"`           // Mode-S downlink format
	Lat          float64   `json:"lat"`          // placeholder per ICAO hash
	Lon          float64   `json:"lon"`          // placeholder per ICAO hash
	PositionFake bool      `json:"positionFake"` // always true in Phase 5
	FrameCount   uint64    `json:"frameCount"`   // total frames seen for this ICAO
	LastSeen     time.Time `json:"lastSeen"`
}

// NormalizedTrackSnapshot is the per-aircraft state surfaced to the UI
// for the BaseStation decoded-track fast path (cmd/multistream-buyer →
// TaggedFrame{source:"adsb", type:"normalized-track"}). One snapshot per
// entityID (UPPERCASE per docs/normalized-track-contract.md §3.1).
//
// Critical contrast with AircraftSnapshot: the Lat/Lon here are REAL —
// they come straight out of the upstream BaseStation decoder (JV port
// 30003 or BlueMark SBS export). FakePosition mirrors the contract's
// quality.fakePosition field and defaults to false; the BaseStation
// stream may still emit synthetic positions (e.g., from the
// adsb-seller --feed-source=synthetic path) in which case the producer
// is expected to set fakePosition=true.
//
// BaseStation is `additive` / `demo-grade` / `decoded-track shortcut` /
// `lower-fidelity than raw paths` / `not sufficient by itself for raw
// evidence claims`. The five labels are normative per the audit; this
// struct surfaces them onto the wire purely as data — UI policy is
// owned by the HTML layer.
type NormalizedTrackSnapshot struct {
	EntityID        string    `json:"entityID"`   // UPPERCASE; ICAO24 for aircraft
	EntityType      string    `json:"entityType"` // "aircraft" (v1)
	Source          string    `json:"source"`     // always "adsb" for v1 of this slice
	SellerPeerID    string    `json:"sellerPeerID"`
	Lat             float64   `json:"lat"`             // REAL — from upstream decoder
	Lon             float64   `json:"lon"`             // REAL
	AltitudeM       float64   `json:"altitudeM"`       // metres (contract §3.2 normative SI)
	GroundSpeedMps  float64   `json:"groundSpeedMps"`  // m/s; meaningful only when HasGroundSpeed
	HasGroundSpeed  bool      `json:"hasGroundSpeed"`  // false ⇒ velocity not decoded → UI shows "—", not 0.0
	HeadingDeg      float64   `json:"headingDeg"`      // degrees true track [0, 360); valid only when HasHeading
	HasHeading      bool      `json:"hasHeading"`      // false ⇒ heading not decoded → UI shows "—"
	VerticalRateMps float64   `json:"verticalRateMps"` // m/s; +climb / -descent; valid only when HasVerticalRate
	HasVerticalRate bool      `json:"hasVerticalRate"`
	Callsign        string    `json:"callsign,omitempty"`
	Squawk          string    `json:"squawk,omitempty"`
	FakePosition    bool      `json:"fakePosition"` // contract quality.fakePosition; defaults false
	FrameCount      uint64    `json:"frameCount"`
	LastSeen        time.Time `json:"lastSeen"`
}

// OperatorSnapshot is the per-operator (pilot) state surfaced to the UI
// for the Remote ID path. One snapshot per operatorId. SSE events carry
// it alongside the drone snapshot when the inner RemoteIdFrame has an
// operator object with a ground position; /state.json includes an
// `operators` array.
//
// Added 2026-05-18: the synthetic RID demo renders TWO markers — a
// moving drone in a ~500m orbit and a stationary pilot/operator at the
// orbit center. Operators live in their own State.operators map rather
// than nested inside DroneSnapshot, matching the existing drones /
// aircraft / normalizedTracks pattern for independent eviction +
// map-marker styling.
//
// DroneID is a SOFT pairing hint sourced from the same RemoteIdFrame
// that carried the operator. The basestation_source pairing cache is
// single-pair today (`// TODO(multi-drone)`); the field becomes
// authoritative only when multi-drone correlation lands.
type OperatorSnapshot struct {
	OperatorID     string    `json:"operatorId"`
	OperatorIDType string    `json:"operatorIdType,omitempty"`
	DroneID        string    `json:"droneId,omitempty"`
	Source         string    `json:"source"`
	SellerPeerID   string    `json:"sellerPeerID"`
	Lat            float64   `json:"lat"`
	Lon            float64   `json:"lon"`
	FrameSource    string    `json:"frameSource,omitempty"`
	LastSeen       time.Time `json:"lastSeen"`
}

// frameInner is the structural subset we extract from the canonical
// RemoteIdFrame embedded in TaggedFrame.Frame. We only depend on field
// NAMES (canonical-JSON ordering on the wire), not types from the
// remoteid package — keeps the display free of an internal package
// import.
//
// Operator (added 2026-05-18) propagates the optional operator
// object from the canonical wire (matches Operator.MarshalJSON shape in
// internal/dapp/remoteid/frame.go) so the display can render a separate
// pilot/operator marker for the synthetic-RID shape: one moving drone +
// one stationary operator at the orbit center.
type frameInner struct {
	DroneID          string              `json:"droneId"`
	DroneIDType      string              `json:"droneIdType"`
	Source           string              `json:"source,omitempty"`
	Position         *position           `json:"position,omitempty"`
	Velocity         *velocity           `json:"velocity,omitempty"`
	Operator         *frameOperatorInner `json:"operator,omitempty"`
	RegulatorVariant string              `json:"regulatorVariant,omitempty"`
}

// frameOperatorInner mirrors the canonical Operator JSON shape emitted
// by internal/dapp/remoteid/frame.go's Operator.MarshalJSON. Position
// reuses the same `position` struct already defined for drone position
// (lat/lon/alt/fix) — the operator's ground position is structurally
// identical to a drone's airborne position.
type frameOperatorInner struct {
	IDType   string    `json:"idType,omitempty"`
	ID       string    `json:"id"`
	Position *position `json:"position,omitempty"`
}

// aircraftInner is the structural subset we extract from the
// AggregatedFrame embedded in TaggedFrame.Frame for source="adsb".
// Mirrors edgeapp.AggregatedFrame JSON shape — same field-NAME-only
// dependency rule as frameInner to keep the display free of an
// internal package import.
type aircraftInner struct {
	SellerEVM    string       `json:"sellerEVM"`
	SellerName   string       `json:"sellerName"`
	SellerPeerID string       `json:"sellerPeerID"`
	Meta         aircraftMeta `json:"meta"`
}

type aircraftMeta struct {
	DF   int    `json:"DF"`
	ICAO string `json:"ICAO"`
}

type position struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	Alt float64 `json:"alt"`
	Fix string  `json:"fix"`
}

type velocity struct {
	SpeedHorizontal float64 `json:"speedHorizontal"`
	SpeedVertical   float64 `json:"speedVertical"`
	Track           float64 `json:"track"`
}

// normalizedTrackInner is the structural subset we extract from the
// canonical NormalizedTrack embedded in TaggedFrame.Frame when
// source="adsb" and type="normalized-track". Mirrors the schema in
// docs/normalized-track-contract.md §3 — field-NAME-only dependency,
// no internal/dapp/adsb package import in the display.
type normalizedTrackInner struct {
	Type       string              `json:"type"`
	Version    string              `json:"version"`
	ObservedAt string              `json:"observedAt"`
	Source     string              `json:"source"`
	EntityType string              `json:"entityType"`
	EntityID   string              `json:"entityID"`
	Position   *normalizedPosition `json:"position,omitempty"`
	Velocity   *normalizedVelocity `json:"velocity,omitempty"`
	Callsign   string              `json:"callsign,omitempty"`
	Squawk     string              `json:"squawk,omitempty"`
	Quality    *normalizedQuality  `json:"quality,omitempty"`
}

type normalizedPosition struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	AltitudeM float64 `json:"altitudeM"`
}

// normalizedVelocity uses pointer fields so an absent sub-field (omitted on the
// wire per docs/normalized-track-contract.md §3.3 — "omitted when missing") is
// distinguishable from a present zero. A nil pointer ⇒ "not decoded", which the
// snapshot surfaces as HasX=false so the UI renders "—" instead of a fake 0.
type normalizedVelocity struct {
	GroundSpeedMps  *float64 `json:"groundSpeedMps"`
	HeadingDeg      *float64 `json:"headingDeg"`
	VerticalRateMps *float64 `json:"verticalRateMps"`
}

type normalizedQuality struct {
	Receivers    int     `json:"receivers,omitempty"`
	HorizErrM    float64 `json:"horizErrM,omitempty"`
	FakePosition bool    `json:"fakePosition,omitempty"`
}

// State is the display's in-memory representation: one DroneSnapshot
// per droneId, one AircraftSnapshot per ICAO (placeholder-position ADS-B
// from cmd/edge-buyer's AggregatedFrame envelope), one
// NormalizedTrackSnapshot per entityID (real-position ADS-B from
// cmd/multistream-buyer's NormalizedTrack envelope), plus a fan-out for SSE
// subscribers. The three keyspaces are isolated so the dual-source
// independence rule is naturally satisfied: a stalled BaseStation feed
// evicts only normalizedTracks, a stalled BEAST/AggregatedFrame feed
// evicts only aircraft, a stalled Remote ID feed evicts only drones.
type State struct {
	mu               sync.RWMutex
	drones           map[string]DroneSnapshot           // keyed by droneId
	aircraft         map[string]AircraftSnapshot        // keyed by ICAO24 hex (lower)
	normalizedTracks map[string]NormalizedTrackSnapshot // keyed by entityID (UPPER)
	operators        map[string]OperatorSnapshot        // keyed by operatorId (Phase 3, 2026-05-18)

	subMu sync.Mutex
	subs  map[chan SSEEvent]struct{}

	// Map center used to derive aircraft placeholder positions. Set by
	// the operator via --lat/--lon; defaults to the synthetic-orbit
	// reference center. Only the placeholder AircraftSnapshot path uses
	// this — NormalizedTrack positions are real and bypass the anchor.
	mapCenterLat float64
	mapCenterLon float64
}

// SSEEvent is what gets fanned out on /events. The Kind field tells the
// browser which marker layer to update.
//
// Phase 3 (2026-05-18): a "drone" event MAY also carry a non-nil
// Operator pointer when the inner RemoteIdFrame had an operator object
// with a position. Existing consumers reading event.Drone keep working
// unchanged; new consumers also read event.Operator and update the
// pilot marker. This is an additive extension — no API break.
type SSEEvent struct {
	Kind            string                   `json:"kind"` // "drone" | "aircraft" | "normalized-track"
	Drone           *DroneSnapshot           `json:"drone,omitempty"`
	Aircraft        *AircraftSnapshot        `json:"aircraft,omitempty"`
	NormalizedTrack *NormalizedTrackSnapshot `json:"normalizedTrack,omitempty"`
	Operator        *OperatorSnapshot        `json:"operator,omitempty"` // Phase 3
}

// NewState returns an empty State. Use NewStateWithCenter to control
// the aircraft placeholder-position anchor.
func NewState() *State {
	return NewStateWithCenter(51.4775, -0.4614)
}

// NewStateWithCenter returns an empty State whose aircraft placeholder
// positions are anchored around (lat, lon).
func NewStateWithCenter(lat, lon float64) *State {
	return &State{
		drones:           make(map[string]DroneSnapshot),
		aircraft:         make(map[string]AircraftSnapshot),
		normalizedTracks: make(map[string]NormalizedTrackSnapshot),
		operators:        make(map[string]OperatorSnapshot),
		subs:             make(map[chan SSEEvent]struct{}),
		mapCenterLat:     lat,
		mapCenterLon:     lon,
	}
}

// Apply parses one TaggedFrame, updates the appropriate per-source
// snapshot, and notifies SSE subscribers. Dispatches by (tf.Source, tf.Type):
//
//   - source="remote-id"                     → drone path (Phase-2, unchanged)
//   - source="adsb", type="normalized-track" → normalized-track path (Phase-5
//     BaseStation decoded-track fast
//     path; REAL lat/lon from upstream)
//   - source="adsb", any other type          → aircraft path (legacy placeholder;
//     placeholder lat/lon via
//     placeholderPositionFromICAO)
//
// Returns SSEEvent describing what was applied (the broadcast payload
// also reaches subscribers). Errors when the inner frame is malformed
// or missing required identification.
func (s *State) Apply(tf TaggedFrame) (SSEEvent, error) {
	switch tf.Source {
	case "remote-id":
		return s.applyDrone(tf)
	case "adsb":
		if tf.Type == "normalized-track" {
			return s.applyNormalizedTrack(tf)
		}
		return s.applyAircraft(tf)
	default:
		return SSEEvent{}, fmt.Errorf("unknown source %q (want \"remote-id\" or \"adsb\")", tf.Source)
	}
}

func (s *State) applyDrone(tf TaggedFrame) (SSEEvent, error) {
	var inner frameInner
	if err := json.Unmarshal(tf.Frame, &inner); err != nil {
		return SSEEvent{}, fmt.Errorf("decode inner remote-id frame: %w", err)
	}
	if inner.DroneID == "" {
		return SSEEvent{}, errors.New("inner remote-id frame missing droneId")
	}
	if inner.Position == nil {
		return SSEEvent{}, errors.New("inner remote-id frame missing position")
	}

	snap := DroneSnapshot{
		DroneID:          inner.DroneID,
		Source:           tf.Source,
		SellerPeerID:     tf.SellerPeerID,
		Lat:              inner.Position.Lat,
		Lon:              inner.Position.Lon,
		Alt:              inner.Position.Alt,
		Fix:              inner.Position.Fix,
		LastSeen:         tf.ReceivedAt,
		RegulatorVariant: inner.RegulatorVariant,
		FrameSource:      inner.Source,
	}
	if inner.Velocity != nil {
		snap.SpeedHorizontal = inner.Velocity.SpeedHorizontal
		snap.Track = inner.Velocity.Track
	}

	// Phase 3 (2026-05-18): if the inner RemoteIdFrame carried an
	// operator with a ground position, materialise a parallel
	// OperatorSnapshot so the map renders two markers (moving drone +
	// stationary pilot). Operators without a position are intentionally
	// skipped — we have nowhere to place the marker.
	var opSnap *OperatorSnapshot
	if inner.Operator != nil && inner.Operator.ID != "" && inner.Operator.Position != nil {
		op := OperatorSnapshot{
			OperatorID:     inner.Operator.ID,
			OperatorIDType: inner.Operator.IDType,
			DroneID:        inner.DroneID, // soft pairing hint
			Source:         tf.Source,
			SellerPeerID:   tf.SellerPeerID,
			Lat:            inner.Operator.Position.Lat,
			Lon:            inner.Operator.Position.Lon,
			FrameSource:    inner.Source,
			LastSeen:       tf.ReceivedAt,
		}
		opSnap = &op
	}

	s.mu.Lock()
	s.drones[snap.DroneID] = snap
	if opSnap != nil {
		s.operators[opSnap.OperatorID] = *opSnap
	}
	s.mu.Unlock()

	event := SSEEvent{Kind: "drone", Drone: &snap, Operator: opSnap}
	s.broadcast(event)
	return event, nil
}

func (s *State) applyAircraft(tf TaggedFrame) (SSEEvent, error) {
	var inner aircraftInner
	if err := json.Unmarshal(tf.Frame, &inner); err != nil {
		return SSEEvent{}, fmt.Errorf("decode inner adsb frame: %w", err)
	}
	icao := strings.ToLower(strings.TrimSpace(inner.Meta.ICAO))
	if icao == "" {
		// ADS-B frames without an ICAO are common (DF=0/4/5 short surveillance
		// frames without recoverable ICAO). We can't track them on the map.
		return SSEEvent{}, errors.New("inner adsb frame missing icao")
	}

	s.mu.Lock()
	prev, hadPrev := s.aircraft[icao]
	lat, lon := placeholderPositionFromICAO(icao, s.mapCenterLat, s.mapCenterLon)
	snap := AircraftSnapshot{
		ICAO:         icao,
		Source:       "adsb",
		SellerEVM:    inner.SellerEVM,
		SellerName:   inner.SellerName,
		SellerPeerID: tf.SellerPeerID,
		DF:           inner.Meta.DF,
		Lat:          lat,
		Lon:          lon,
		PositionFake: true,
		FrameCount:   1,
		LastSeen:     tf.ReceivedAt,
	}
	if hadPrev {
		snap.FrameCount = prev.FrameCount + 1
	}
	s.aircraft[icao] = snap
	s.mu.Unlock()

	event := SSEEvent{Kind: "aircraft", Aircraft: &snap}
	s.broadcast(event)
	return event, nil
}

// applyNormalizedTrack handles the BaseStation decoded-track fast path
// (cmd/multistream-buyer's TaggedFrame{source:"adsb", type:"normalized-track"}).
// Real lat/lon comes straight out of the upstream BaseStation decoder
// (JV 30003 or BlueMark SBS export) per docs/normalized-track-contract.md
// §3.2 — no ICAO-hash placeholder math runs on this branch.
//
// The function is additive: it does NOT touch State.aircraft (Phase-2
// AggregatedFrame placeholder branch) or State.drones (Remote ID branch).
// EntityID is stored UPPERCASE per contract §3.1 R3.
func (s *State) applyNormalizedTrack(tf TaggedFrame) (SSEEvent, error) {
	var inner normalizedTrackInner
	if err := json.Unmarshal(tf.Frame, &inner); err != nil {
		return SSEEvent{}, fmt.Errorf("decode inner normalized-track frame: %w", err)
	}
	entityID := strings.ToUpper(strings.TrimSpace(inner.EntityID))
	if entityID == "" {
		return SSEEvent{}, errors.New("inner normalized-track frame missing entityID")
	}

	s.mu.Lock()
	prev, hadPrev := s.normalizedTracks[entityID]
	snap := NormalizedTrackSnapshot{
		EntityID:     entityID,
		EntityType:   inner.EntityType,
		Source:       tf.Source,
		SellerPeerID: tf.SellerPeerID,
		FrameCount:   1,
		LastSeen:     tf.ReceivedAt,
	}
	if inner.Position != nil {
		snap.Lat = inner.Position.Lat
		snap.Lon = inner.Position.Lon
		snap.AltitudeM = inner.Position.AltitudeM
	}
	if inner.Velocity != nil {
		// Per-sub-field presence: a NormalizedTrack may carry speed without
		// heading (or neither). Only set value+flag for what the wire actually
		// carried, so the UI shows "—" for undecoded velocity, not a fake 0.0.
		if inner.Velocity.GroundSpeedMps != nil {
			snap.GroundSpeedMps = *inner.Velocity.GroundSpeedMps
			snap.HasGroundSpeed = true
		}
		if inner.Velocity.HeadingDeg != nil {
			snap.HeadingDeg = *inner.Velocity.HeadingDeg
			snap.HasHeading = true
		}
		if inner.Velocity.VerticalRateMps != nil {
			snap.VerticalRateMps = *inner.Velocity.VerticalRateMps
			snap.HasVerticalRate = true
		}
	}
	snap.Callsign = inner.Callsign
	snap.Squawk = inner.Squawk
	if inner.Quality != nil {
		snap.FakePosition = inner.Quality.FakePosition
	}
	if hadPrev {
		snap.FrameCount = prev.FrameCount + 1
	}
	s.normalizedTracks[entityID] = snap
	s.mu.Unlock()

	event := SSEEvent{Kind: "normalized-track", NormalizedTrack: &snap}
	s.broadcast(event)
	return event, nil
}

// broadcast best-effort fan-out to SSE subscribers. Slow subscribers
// drop events rather than blocking the data path.
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

// placeholderPositionFromICAO returns a deterministic (lat, lon) within
// roughly ±5km of (centerLat, centerLon) derived from the ICAO hash.
// Phase 5 placeholder — replaced by real CPR decode in Phase 6A.
func placeholderPositionFromICAO(icao string, centerLat, centerLon float64) (float64, float64) {
	h := sha256.Sum256([]byte(icao))
	// First two bytes drive lat offset, next two drive lon offset.
	// uint16 / 65535 maps to [0, 1]; we map that to ±5km.
	// 1° lat ≈ 111_320 m → 5_000 m ≈ 0.0449°.
	dy := (float64(uint16(h[0])<<8|uint16(h[1]))/65535.0 - 0.5) * 2 * 0.0449
	dx := (float64(uint16(h[2])<<8|uint16(h[3]))/65535.0 - 0.5) * 2 * 0.0449 / math.Cos(centerLat*math.Pi/180.0)
	return centerLat + dy, centerLon + dx
}

// Snapshot returns a copy of the current state, sorted-stable by droneId
// for deterministic output.
// Snapshot returns a copy of every drone currently tracked. Preserved
// for back-compat with the Phase-2 /state.json drones list.
func (s *State) Snapshot() []DroneSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DroneSnapshot, 0, len(s.drones))
	for _, d := range s.drones {
		out = append(out, d)
	}
	return out
}

// AircraftSnapshot returns a copy of every aircraft currently tracked.
func (s *State) AircraftSnapshot() []AircraftSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AircraftSnapshot, 0, len(s.aircraft))
	for _, a := range s.aircraft {
		out = append(out, a)
	}
	return out
}

// NormalizedTracksSnapshot returns a copy of every NormalizedTrack
// currently tracked (BaseStation decoded-track fast path).
func (s *State) NormalizedTracksSnapshot() []NormalizedTrackSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]NormalizedTrackSnapshot, 0, len(s.normalizedTracks))
	for _, n := range s.normalizedTracks {
		out = append(out, n)
	}
	return out
}

// OperatorSnapshots returns a copy of every operator (pilot) currently
// tracked. Added Phase 3 (2026-05-18); mirrors Snapshot() /
// AircraftSnapshot() / NormalizedTracksSnapshot() shape.
func (s *State) OperatorSnapshots() []OperatorSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]OperatorSnapshot, 0, len(s.operators))
	for _, o := range s.operators {
		out = append(out, o)
	}
	return out
}

// Subscribe registers a new SSE listener. The caller MUST call
// Unsubscribe when done; the returned channel is closed only by
// Unsubscribe.
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

// Evict drops drone, aircraft, normalizedTrack, AND operator snapshots
// whose LastSeen is older than now-maxAge. Returns the total count
// evicted across the four maps. maxAge <= 0 is a no-op. Per
// docs/fid-display-contract.md the independence test relies on this
// dropping ONLY entries whose own LastSeen has lapsed — each keyspace
// is evicted independently so a stalled feed for one source leaves the
// others intact. (Phase 3 2026-05-18 added the operators keyspace.)
func (s *State) Evict(now time.Time, maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}
	cutoff := now.Add(-maxAge)
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	for id, d := range s.drones {
		if d.LastSeen.Before(cutoff) {
			delete(s.drones, id)
			n++
		}
	}
	for id, a := range s.aircraft {
		if a.LastSeen.Before(cutoff) {
			delete(s.aircraft, id)
			n++
		}
	}
	for id, nt := range s.normalizedTracks {
		if nt.LastSeen.Before(cutoff) {
			delete(s.normalizedTracks, id)
			n++
		}
	}
	for id, op := range s.operators {
		if op.LastSeen.Before(cutoff) {
			delete(s.operators, id)
			n++
		}
	}
	return n
}

// MapConfig is the payload returned by /config.json.
type MapConfig struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Zoom int     `json:"zoom"`
}

func main() {
	var (
		httpAddr  = flag.String("http", "127.0.0.1:8080", "HTTP UI bind address")
		tcpAddr   = flag.String("tcp", "127.0.0.1:9090", "TCP listen address for buyer connections")
		fixture   = flag.String("fixture", "", "JSONL fixture file to replay at startup (in addition to TCP listener)")
		centerLat = flag.Float64("lat", 51.4775, "initial map center latitude")
		centerLon = flag.Float64("lon", -0.4614, "initial map center longitude")
		zoom      = flag.Int("zoom", 13, "initial map zoom (1-19)")
		// In-code default is 5m; the canonical reference demo deployment overrides
		// to 10m via scripts/tevv/demo-start.sh (`./bin/fid-display --evict=10m`)
		// to tolerate the synthetic-drone broadcast cadence + reconnect windows.
		evictAge = flag.Duration("evict", 5*time.Minute, "drop drones not seen for this duration; 0 disables")
	)
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)

	state := NewState()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Replay fixture if requested.
	if *fixture != "" {
		go replayFixture(ctx, *fixture, state, logger)
	}

	// TCP listener for buyers.
	listener, err := net.Listen("tcp", *tcpAddr)
	if err != nil {
		logger.Fatalf("tcp listen %s: %v", *tcpAddr, err)
	}
	defer listener.Close()
	logger.Printf("listening for tagged JSONL on tcp://%s", *tcpAddr)
	go acceptLoop(ctx, listener, state, logger)

	// Periodic eviction.
	if *evictAge > 0 {
		go evictionLoop(ctx, state, *evictAge, logger)
	}

	// HTTP server.
	cfg := MapConfig{Lat: *centerLat, Lon: *centerLon, Zoom: *zoom}
	mux := http.NewServeMux()
	mux.Handle("/", indexHandler())
	mux.HandleFunc("/config.json", configHandler(cfg))
	mux.HandleFunc("/state.json", stateHandler(state))
	mux.HandleFunc("/events", eventsHandler(state))

	srv := &http.Server{
		Addr:              *httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Printf("HTTP UI on http://%s", *httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("http server: %v", err)
		}
	}()

	// Wait for signal.
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
			if ctx.Err() != nil {
				return
			}
			// Listener closed or other transient error.
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Printf("accept: %v", err)
			continue
		}
		logger.Printf("buyer connected: %s", conn.RemoteAddr())
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
	logger.Printf("buyer disconnected: %s (seen=%d applied=%d)", conn.RemoteAddr(), seen, applied)
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
				logger.Printf("evicted %d stale drone(s)", n)
			}
		}
	}
}

// HTTP handlers ----------------------------------------------------

func indexHandler() http.HandlerFunc {
	// Serve the embedded static/ tree (index.html + app.css + app.js +
	// SVG assets). http.FileServer automatically serves index.html for
	// "/" and returns 404 for unknown paths. /config.json, /state.json,
	// and /events are mounted on the same mux as more-specific patterns
	// so ServeMux routes them to their dedicated handlers before falling
	// through to this one.
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

func stateHandler(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		drones := state.Snapshot()
		aircraft := state.AircraftSnapshot()
		normalizedTracks := state.NormalizedTracksSnapshot()
		operators := state.OperatorSnapshots()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"drones":           drones,
			"aircraft":         aircraft,
			"normalizedTracks": normalizedTracks,
			"operators":        operators,
			"count":            len(drones) + len(aircraft) + len(normalizedTracks) + len(operators),
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

		// On connect, send the current state as a single snapshot batch.
		// Drones, then placeholder-aircraft, then normalized-tracks —
		// order is arbitrary but stable per call so tests can predict it.
		for _, d := range state.Snapshot() {
			payload, _ := json.Marshal(SSEEvent{Kind: "drone", Drone: &d})
			fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload)
		}
		for _, a := range state.AircraftSnapshot() {
			payload, _ := json.Marshal(SSEEvent{Kind: "aircraft", Aircraft: &a})
			fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload)
		}
		for _, nt := range state.NormalizedTracksSnapshot() {
			payload, _ := json.Marshal(SSEEvent{Kind: "normalized-track", NormalizedTrack: &nt})
			fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload)
		}
		flusher.Flush()

		// Then stream updates.
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
