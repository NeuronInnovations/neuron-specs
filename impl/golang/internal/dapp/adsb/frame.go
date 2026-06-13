package adsb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sbs "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
)

// FrameType is the canonical-JSON discriminator for a NormalizedTrack.
const FrameType = "normalized-track"

// FrameVersion is the current NormalizedTrack schema version per
// docs/normalized-track-contract.md.
const FrameVersion = "1.0.0"

// Entity types (closed v1 enum per the contract doc §3.1).
const (
	EntityTypeAircraft = "aircraft"
	EntityTypeDrone    = "drone"
)

// Source identifiers (must match the producing DApp's neuron-commerce.name
// per Spec 018 FR-F-03 and the NormalizedTrack contract §3.1 R1).
const (
	SourceAdsb     = "adsb"
	SourceRemoteID = "remote-id"
)

// Unit-conversion constants used by FromSBSTrack (imperial → SI metric).
const (
	feetToMetres    = 0.3048
	knotsToMps      = 0.5144444444444444
	fpmToMps        = 0.00508 // feet-per-minute to metres-per-second
)

// NormalizedTrack is the canonical-JSON wire payload that rides inside
// TaggedFrame.frame on the /jetvision/basestation/1.0.0 stream (and the future
// /ds240/basestation/1.0.0 stream when the Remote ID basestation path
// lands). The schema is normative per docs/normalized-track-contract.md.
//
// Canonical field ordering on the wire per the contract doc §3:
//
//	type → version → observedAt → source → entityType → entityID →
//	  position* → velocity* → callsign* → squawk* → quality*
//
// Optional fields (Position, Velocity, Callsign, Squawk, Quality) are
// omitted entirely from the wire when not present (per Spec 006 FR-W04 —
// do not emit null). Internal representation uses pointer types where
// "absent" needs to be distinguishable from "zero-value".
type NormalizedTrack struct {
	Type       string
	Version    string
	ObservedAt time.Time
	Source     string
	EntityType string
	EntityID   string
	Position   *NormalizedPosition
	Velocity   *NormalizedVelocity
	Callsign   string
	Squawk     string
	Quality    *NormalizedQuality
}

// NormalizedPosition is the optional WGS84 position component. Units are SI
// metric per the contract doc §3.2.
type NormalizedPosition struct {
	Lat       float64
	Lon       float64
	AltitudeM float64
	// AltitudeSet distinguishes "altitude 0 metres" from "altitude omitted".
	AltitudeSet bool
}

// NormalizedVelocity is the optional velocity component. Units are SI metric.
type NormalizedVelocity struct {
	GroundSpeedMps  float64
	GroundSpeedSet  bool
	HeadingDeg      float64
	HeadingSet      bool
	VerticalRateMps float64
	VerticalRateSet bool
}

// NormalizedQuality carries sensor-confidence metadata per the contract
// doc §3.4.
type NormalizedQuality struct {
	Receivers     int
	ReceiversSet  bool
	HorizErrM     float64
	HorizErrSet   bool
	FakePosition  bool
	// FakePositionSet is always true when the Quality block is emitted (the
	// fakePosition field defaults to false per the contract doc).
	FakePositionSet bool
}

// FromSBSTrack converts an internal/feeds/sbs.SBSTrack into a NormalizedTrack,
// performing imperial → SI metric conversion per the contract doc.
//
// The resulting NormalizedTrack has:
//   - source = "adsb"
//   - entityType = "aircraft"
//   - entityID = UPPERCASE ICAO from the SBS-1 record
//   - observedAt = SBS-1 GeneratedAt (UTC); falls back to Rx if zero
//   - position present when LatSet ∧ LonSet (altitude included when AltSet)
//   - velocity present when at least one of {SpdSet, TrkSet, VrtSet} is true
//   - callsign / squawk passed through when non-empty (uppercased)
//   - quality present when Receivers > 0 or HorizErrM > 0 (sensor-confidence
//     metadata from mlat-server variant)
//
// The output struct's *Set boolean fields preserve "absent" vs "zero-value"
// distinctions through to MarshalJSON, which omits absent fields entirely
// per the contract doc R4 (canonical-JSON omitempty discipline).
func FromSBSTrack(t sbs.SBSTrack) NormalizedTrack {
	observed := t.GeneratedAt
	if observed.IsZero() {
		observed = t.Rx
	}

	nt := NormalizedTrack{
		Type:       FrameType,
		Version:    FrameVersion,
		ObservedAt: observed.UTC(),
		Source:     SourceAdsb,
		EntityType: EntityTypeAircraft,
		EntityID:   strings.ToUpper(strings.TrimSpace(t.ICAO)),
		Callsign:   strings.ToUpper(strings.TrimSpace(t.Callsign)),
		Squawk:     strings.TrimSpace(t.Squawk),
	}

	if t.LatSet && t.LonSet {
		pos := &NormalizedPosition{
			Lat: t.Lat,
			Lon: t.Lon,
		}
		if t.AltSet {
			pos.AltitudeM = t.AltFeet * feetToMetres
			pos.AltitudeSet = true
		}
		nt.Position = pos
	}

	if t.SpdSet || t.TrkSet || t.VrtSet {
		vel := &NormalizedVelocity{}
		if t.SpdSet {
			vel.GroundSpeedMps = t.SpdKnots * knotsToMps
			vel.GroundSpeedSet = true
		}
		if t.TrkSet {
			vel.HeadingDeg = t.TrkDeg
			vel.HeadingSet = true
		}
		if t.VrtSet {
			vel.VerticalRateMps = t.VrtFpm * fpmToMps
			vel.VerticalRateSet = true
		}
		nt.Velocity = vel
	}

	if t.Receivers > 0 || t.HorizErrM > 0 {
		nt.Quality = &NormalizedQuality{}
		if t.Receivers > 0 {
			nt.Quality.Receivers = t.Receivers
			nt.Quality.ReceiversSet = true
		}
		if t.HorizErrM > 0 {
			nt.Quality.HorizErrM = t.HorizErrM
			nt.Quality.HorizErrSet = true
		}
		// FakePosition defaults to false; only set when an upstream signal
		// surfaces (currently nothing in SBS-1 carries it, so always false
		// here; the future placeholder source would set this true).
	}

	return nt
}

// MarshalJSON emits canonical JSON per docs/normalized-track-contract.md §3.
// Field ordering is normative; absent optional fields are omitted (not null).
func (n NormalizedTrack) MarshalJSON() ([]byte, error) {
	if n.EntityID == "" {
		return nil, New(ErrFrameMalformed, "MarshalJSON", "entityID is required")
	}
	if n.EntityType != EntityTypeAircraft && n.EntityType != EntityTypeDrone {
		return nil, New(ErrInvalidEntityType, "MarshalJSON",
			fmt.Sprintf("entityType %q not in v1 enum {aircraft, drone}", n.EntityType))
	}
	if n.Source != SourceAdsb && n.Source != SourceRemoteID {
		return nil, New(ErrFrameMalformed, "MarshalJSON",
			fmt.Sprintf("source %q not in v1 vocabulary {adsb, remote-id}", n.Source))
	}
	// R2 — entityType ↔ source pairing.
	if (n.Source == SourceAdsb && n.EntityType != EntityTypeAircraft) ||
		(n.Source == SourceRemoteID && n.EntityType != EntityTypeDrone) {
		return nil, New(ErrInvalidEntityType, "MarshalJSON",
			fmt.Sprintf("source=%q must pair with entityType=%q (contract R2)",
				n.Source,
				map[string]string{SourceAdsb: EntityTypeAircraft, SourceRemoteID: EntityTypeDrone}[n.Source]))
	}

	typ := n.Type
	if typ == "" {
		typ = FrameType
	}
	ver := n.Version
	if ver == "" {
		ver = FrameVersion
	}

	pairs := []orderedPair{
		{"type", typ},
		{"version", ver},
		{"observedAt", n.ObservedAt.UTC().Format(time.RFC3339Nano)},
		{"source", n.Source},
		{"entityType", n.EntityType},
		{"entityID", n.EntityID},
	}
	if n.Position != nil {
		pairs = append(pairs, orderedPair{"position", n.Position})
	}
	if n.Velocity != nil {
		pairs = append(pairs, orderedPair{"velocity", n.Velocity})
	}
	if n.Callsign != "" {
		pairs = append(pairs, orderedPair{"callsign", n.Callsign})
	}
	if n.Squawk != "" {
		pairs = append(pairs, orderedPair{"squawk", n.Squawk})
	}
	if n.Quality != nil {
		pairs = append(pairs, orderedPair{"quality", n.Quality})
	}
	return marshalOrdered(pairs)
}

// UnmarshalJSON parses a canonical NormalizedTrack from JSON. Field-order on
// input is NOT enforced (parsing is tolerant; the canonical order is a
// SERIALIZE-side requirement only).
func (n *NormalizedTrack) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type       string             `json:"type"`
		Version    string             `json:"version"`
		ObservedAt string             `json:"observedAt"`
		Source     string             `json:"source"`
		EntityType string             `json:"entityType"`
		EntityID   string             `json:"entityID"`
		Position   *positionJSON      `json:"position,omitempty"`
		Velocity   *velocityJSON      `json:"velocity,omitempty"`
		Callsign   string             `json:"callsign,omitempty"`
		Squawk     string             `json:"squawk,omitempty"`
		Quality    *qualityJSON       `json:"quality,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Wrap(ErrFrameMalformed, "UnmarshalJSON", err)
	}

	n.Type = raw.Type
	n.Version = raw.Version
	n.Source = raw.Source
	n.EntityType = raw.EntityType
	n.EntityID = raw.EntityID
	n.Callsign = raw.Callsign
	n.Squawk = raw.Squawk

	if raw.ObservedAt != "" {
		t, err := time.Parse(time.RFC3339Nano, raw.ObservedAt)
		if err != nil {
			return Wrap(ErrFrameMalformed, "UnmarshalJSON", fmt.Errorf("observedAt %q: %w", raw.ObservedAt, err))
		}
		n.ObservedAt = t.UTC()
	}

	if raw.Position != nil {
		n.Position = &NormalizedPosition{
			Lat:         raw.Position.Lat,
			Lon:         raw.Position.Lon,
			AltitudeM:   raw.Position.AltitudeM,
			AltitudeSet: raw.Position.AltitudeMSet,
		}
	}
	if raw.Velocity != nil {
		n.Velocity = &NormalizedVelocity{
			GroundSpeedMps:  raw.Velocity.GroundSpeedMps,
			GroundSpeedSet:  raw.Velocity.GroundSpeedMpsSet,
			HeadingDeg:      raw.Velocity.HeadingDeg,
			HeadingSet:      raw.Velocity.HeadingDegSet,
			VerticalRateMps: raw.Velocity.VerticalRateMps,
			VerticalRateSet: raw.Velocity.VerticalRateMpsSet,
		}
	}
	if raw.Quality != nil {
		n.Quality = &NormalizedQuality{
			Receivers:       raw.Quality.Receivers,
			ReceiversSet:    raw.Quality.ReceiversSet,
			HorizErrM:       raw.Quality.HorizErrM,
			HorizErrSet:     raw.Quality.HorizErrMSet,
			FakePosition:    raw.Quality.FakePosition,
			FakePositionSet: raw.Quality.FakePositionSet,
		}
	}
	return nil
}

// MarshalJSON for NormalizedPosition emits the canonical order: lat → lon → altitudeM*.
func (p NormalizedPosition) MarshalJSON() ([]byte, error) {
	pairs := []orderedPair{
		{"lat", p.Lat},
		{"lon", p.Lon},
	}
	if p.AltitudeSet {
		pairs = append(pairs, orderedPair{"altitudeM", p.AltitudeM})
	}
	return marshalOrdered(pairs)
}

// MarshalJSON for NormalizedVelocity emits: groundSpeedMps* → headingDeg* → verticalRateMps*.
func (v NormalizedVelocity) MarshalJSON() ([]byte, error) {
	var pairs []orderedPair
	if v.GroundSpeedSet {
		pairs = append(pairs, orderedPair{"groundSpeedMps", v.GroundSpeedMps})
	}
	if v.HeadingSet {
		pairs = append(pairs, orderedPair{"headingDeg", v.HeadingDeg})
	}
	if v.VerticalRateSet {
		pairs = append(pairs, orderedPair{"verticalRateMps", v.VerticalRateMps})
	}
	if len(pairs) == 0 {
		// Avoid emitting "velocity: {}" — the caller should only include
		// velocity in the parent when at least one field is set.
		return []byte("{}"), nil
	}
	return marshalOrdered(pairs)
}

// MarshalJSON for NormalizedQuality emits: receivers* → horizErrM* → fakePosition*.
func (q NormalizedQuality) MarshalJSON() ([]byte, error) {
	var pairs []orderedPair
	if q.ReceiversSet {
		pairs = append(pairs, orderedPair{"receivers", q.Receivers})
	}
	if q.HorizErrSet {
		pairs = append(pairs, orderedPair{"horizErrM", q.HorizErrM})
	}
	if q.FakePositionSet {
		pairs = append(pairs, orderedPair{"fakePosition", q.FakePosition})
	}
	if len(pairs) == 0 {
		return []byte("{}"), nil
	}
	return marshalOrdered(pairs)
}

// JSON-decode shapes (kept private).

type positionJSON struct {
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	AltitudeM    float64 `json:"altitudeM,omitempty"`
	AltitudeMSet bool    `json:"-"`
}

func (p *positionJSON) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["lat"]; ok {
		_ = json.Unmarshal(v, &p.Lat)
	}
	if v, ok := raw["lon"]; ok {
		_ = json.Unmarshal(v, &p.Lon)
	}
	if v, ok := raw["altitudeM"]; ok {
		_ = json.Unmarshal(v, &p.AltitudeM)
		p.AltitudeMSet = true
	}
	return nil
}

type velocityJSON struct {
	GroundSpeedMps     float64 `json:"-"`
	GroundSpeedMpsSet  bool    `json:"-"`
	HeadingDeg         float64 `json:"-"`
	HeadingDegSet      bool    `json:"-"`
	VerticalRateMps    float64 `json:"-"`
	VerticalRateMpsSet bool    `json:"-"`
}

func (v *velocityJSON) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if x, ok := raw["groundSpeedMps"]; ok {
		_ = json.Unmarshal(x, &v.GroundSpeedMps)
		v.GroundSpeedMpsSet = true
	}
	if x, ok := raw["headingDeg"]; ok {
		_ = json.Unmarshal(x, &v.HeadingDeg)
		v.HeadingDegSet = true
	}
	if x, ok := raw["verticalRateMps"]; ok {
		_ = json.Unmarshal(x, &v.VerticalRateMps)
		v.VerticalRateMpsSet = true
	}
	return nil
}

type qualityJSON struct {
	Receivers       int     `json:"-"`
	ReceiversSet    bool    `json:"-"`
	HorizErrM       float64 `json:"-"`
	HorizErrMSet    bool    `json:"-"`
	FakePosition    bool    `json:"-"`
	FakePositionSet bool    `json:"-"`
}

func (q *qualityJSON) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if x, ok := raw["receivers"]; ok {
		_ = json.Unmarshal(x, &q.Receivers)
		q.ReceiversSet = true
	}
	if x, ok := raw["horizErrM"]; ok {
		_ = json.Unmarshal(x, &q.HorizErrM)
		q.HorizErrMSet = true
	}
	if x, ok := raw["fakePosition"]; ok {
		_ = json.Unmarshal(x, &q.FakePosition)
		q.FakePositionSet = true
	}
	return nil
}

// orderedPair + marshalOrdered are the local canonical-JSON helpers
// (intentionally duplicated from internal/dapp/remoteid/frame.go — small,
// no shared dependency, keeps the dapp package free of a back-reference).
type orderedPair struct {
	key   string
	value any
}

func marshalOrdered(pairs []orderedPair) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, p := range pairs {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyBytes, err := json.Marshal(p.key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		valBytes, err := json.Marshal(p.value)
		if err != nil {
			return nil, err
		}
		buf.Write(valBytes)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
