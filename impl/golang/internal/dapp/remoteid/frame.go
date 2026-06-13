package remoteid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds/remoteid"
)

// FrameType is the canonical-JSON discriminator for a Remote ID frame.
const FrameType = "remote-id-frame"

// FrameVersion is the current Remote ID frame schema version.
const FrameVersion = "1.0.0"

// RemoteIdFrame is the canonical-JSON wire representation of one decoded
// Remote ID broadcast, per 017 FR-R05.
//
// Canonical field ordering per 006 wire-format.md §2 RemoteIdFrame entry:
//
//	type → version → observedAt → source → droneId → droneIdType →
//	  position* → velocity* → operator* → regulatorVariant*
//
// Optional fields (Position, Velocity, Operator, RegulatorVariant) are
// omitted from the wire when not present (per 006 FR-W04 — do not emit
// null). The internal representation uses pointer types for Position /
// Velocity / Operator to distinguish "absent" from "zero-value".
//
// ObservedAt is encoded on the wire as a string-formatted nanosecond
// Unix timestamp (matching 006 FR-W02a UnsignedInt64 encoding).
//
// This type is the wire-format authority for the Remote ID DApp; the
// decoded in-memory shape lives in internal/feeds/remoteid.DecodedFrame.
// FromDecoded converts the latter into a RemoteIdFrame; MarshalJSON
// produces the canonical bytes.
type RemoteIdFrame struct {
	Type             string
	Version          string
	ObservedAt       time.Time
	Source           string
	DroneID          string
	DroneIDType      string
	Position         *Position
	Velocity         *Velocity
	Operator         *Operator
	RegulatorVariant string
}

// Position is the optional WGS84 position component.
type Position struct {
	Lat float64
	Lon float64
	Alt float64
	Fix string
}

// Velocity is the optional speed + heading component.
type Velocity struct {
	SpeedHorizontal float64
	SpeedVertical   float64
	Track           float64
}

// Operator is the optional operator-identification component.
type Operator struct {
	IDType   string
	ID       string
	Position *Position
}

// FromDecoded converts an internal/feeds/remoteid DecodedFrame into a
// RemoteIdFrame. The conversion is a pure rename — both shapes were
// designed to map field-for-field so seller-side normalization is
// trivial.
func FromDecoded(d remoteid.DecodedFrame) RemoteIdFrame {
	f := RemoteIdFrame{
		Type:             d.Type,
		Version:          d.Version,
		ObservedAt:       d.ObservedAt,
		Source:           d.Source,
		DroneID:          d.DroneID,
		DroneIDType:      d.DroneIDType,
		RegulatorVariant: d.RegulatorVariant,
	}
	if f.Type == "" {
		f.Type = FrameType
	}
	if f.Version == "" {
		f.Version = FrameVersion
	}
	if d.Position != nil {
		f.Position = &Position{
			Lat: d.Position.Lat,
			Lon: d.Position.Lon,
			Alt: d.Position.Alt,
			Fix: d.Position.Fix,
		}
	}
	if d.Velocity != nil {
		f.Velocity = &Velocity{
			SpeedHorizontal: d.Velocity.SpeedHorizontal,
			SpeedVertical:   d.Velocity.SpeedVertical,
			Track:           d.Velocity.Track,
		}
	}
	if d.Operator != nil {
		op := &Operator{
			IDType: d.Operator.IDType,
			ID:     d.Operator.ID,
		}
		if d.Operator.Position != nil {
			op.Position = &Position{
				Lat: d.Operator.Position.Lat,
				Lon: d.Operator.Position.Lon,
				Alt: d.Operator.Position.Alt,
				Fix: d.Operator.Position.Fix,
			}
		}
		f.Operator = op
	}
	return f
}

// MarshalJSON emits the canonical-JSON bytes per 006 wire-format §2.
func (f RemoteIdFrame) MarshalJSON() ([]byte, error) {
	if f.DroneID == "" {
		return nil, New(ErrFrameMalformed, "MarshalJSON", "droneId is required")
	}
	if f.DroneIDType == "" {
		return nil, New(ErrFrameMalformed, "MarshalJSON", "droneIdType is required")
	}

	typ := f.Type
	if typ == "" {
		typ = FrameType
	}
	ver := f.Version
	if ver == "" {
		ver = FrameVersion
	}

	pairs := []orderedPair{
		{"type", typ},
		{"version", ver},
		{"observedAt", strconv.FormatInt(f.ObservedAt.UnixNano(), 10)},
		{"source", f.Source},
		{"droneId", f.DroneID},
		{"droneIdType", f.DroneIDType},
	}
	if f.Position != nil {
		pairs = append(pairs, orderedPair{"position", f.Position})
	}
	if f.Velocity != nil {
		pairs = append(pairs, orderedPair{"velocity", f.Velocity})
	}
	if f.Operator != nil {
		pairs = append(pairs, orderedPair{"operator", f.Operator})
	}
	if f.RegulatorVariant != "" {
		pairs = append(pairs, orderedPair{"regulatorVariant", f.RegulatorVariant})
	}

	return marshalOrdered(pairs)
}

// UnmarshalJSON parses a canonical RemoteIdFrame from JSON. The decoder
// is tolerant of field order on input (canonical ordering is a SERIALIZE
// requirement, not a parse requirement).
func (f *RemoteIdFrame) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type             string         `json:"type"`
		Version          string         `json:"version"`
		ObservedAt       string         `json:"observedAt"`
		Source           string         `json:"source"`
		DroneID          string         `json:"droneId"`
		DroneIDType      string         `json:"droneIdType"`
		Position         *positionJSON  `json:"position,omitempty"`
		Velocity         *velocityJSON  `json:"velocity,omitempty"`
		Operator         *operatorJSON  `json:"operator,omitempty"`
		RegulatorVariant string         `json:"regulatorVariant,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Wrap(ErrFrameMalformed, "UnmarshalJSON", err)
	}

	f.Type = raw.Type
	f.Version = raw.Version
	f.Source = raw.Source
	f.DroneID = raw.DroneID
	f.DroneIDType = raw.DroneIDType
	f.RegulatorVariant = raw.RegulatorVariant

	if raw.ObservedAt != "" {
		ns, err := strconv.ParseInt(raw.ObservedAt, 10, 64)
		if err != nil {
			return Wrap(ErrFrameMalformed, "UnmarshalJSON", fmt.Errorf("observedAt %q: %w", raw.ObservedAt, err))
		}
		f.ObservedAt = time.Unix(0, ns).UTC()
	}

	if raw.Position != nil {
		f.Position = &Position{
			Lat: raw.Position.Lat,
			Lon: raw.Position.Lon,
			Alt: raw.Position.Alt,
			Fix: raw.Position.Fix,
		}
	}
	if raw.Velocity != nil {
		f.Velocity = &Velocity{
			SpeedHorizontal: raw.Velocity.SpeedHorizontal,
			SpeedVertical:   raw.Velocity.SpeedVertical,
			Track:           raw.Velocity.Track,
		}
	}
	if raw.Operator != nil {
		op := &Operator{
			IDType: raw.Operator.IDType,
			ID:     raw.Operator.ID,
		}
		if raw.Operator.Position != nil {
			op.Position = &Position{
				Lat: raw.Operator.Position.Lat,
				Lon: raw.Operator.Position.Lon,
				Alt: raw.Operator.Position.Alt,
				Fix: raw.Operator.Position.Fix,
			}
		}
		f.Operator = op
	}
	return nil
}

// MarshalJSON for Position emits the canonical order:
// lat → lon → alt* → fix*
func (p Position) MarshalJSON() ([]byte, error) {
	pairs := []orderedPair{
		{"lat", p.Lat},
		{"lon", p.Lon},
	}
	if p.Alt != 0 || p.Fix == "3D" {
		// Emit alt when present; "3D" fix implies alt is meaningful even if 0.
		pairs = append(pairs, orderedPair{"alt", p.Alt})
	}
	if p.Fix != "" {
		pairs = append(pairs, orderedPair{"fix", p.Fix})
	}
	return marshalOrdered(pairs)
}

// MarshalJSON for Velocity emits the canonical order:
// speedHorizontal → speedVertical → track
func (v Velocity) MarshalJSON() ([]byte, error) {
	return marshalOrdered([]orderedPair{
		{"speedHorizontal", v.SpeedHorizontal},
		{"speedVertical", v.SpeedVertical},
		{"track", v.Track},
	})
}

// MarshalJSON for Operator emits the canonical order:
// idType → id → position*
func (o Operator) MarshalJSON() ([]byte, error) {
	pairs := []orderedPair{
		{"idType", o.IDType},
		{"id", o.ID},
	}
	if o.Position != nil {
		pairs = append(pairs, orderedPair{"position", o.Position})
	}
	return marshalOrdered(pairs)
}

// Internal JSON-decode shapes (kept private; only RemoteIdFrame /
// Position / Velocity / Operator are part of the public API).

type positionJSON struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	Alt float64 `json:"alt"`
	Fix string  `json:"fix"`
}

type velocityJSON struct {
	SpeedHorizontal float64 `json:"speedHorizontal"`
	SpeedVertical   float64 `json:"speedVertical"`
	Track           float64 `json:"track"`
}

type operatorJSON struct {
	IDType   string        `json:"idType"`
	ID       string        `json:"id"`
	Position *positionJSON `json:"position,omitempty"`
}

// orderedPair + marshalOrdered are the local equivalent of payment's
// canonical-JSON helpers. They are duplicated here intentionally (small,
// no shared dependency) to keep the dapp package free of a back-reference
// to internal/payment for a pure helper.
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
