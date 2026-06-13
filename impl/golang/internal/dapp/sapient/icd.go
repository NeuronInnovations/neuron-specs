package sapient

import "time"

// This file is the ICD-shaped Go model of a SAPIENT DetectionReport — the
// content the sensor-side bridge produces and hands to the Neuron seller. It is
// ported verbatim-in-shape from the reference bridge
// (neuron-rid-bridge/internal/sapient/detection.go); encode.go maps it onto the
// generated BSI Flex 335 v2.0 protobuf (sapientpb). The camelCase JSON tags are
// for the inspection projection / debugging only — the wire form is protobuf.
//
// The model carries NO node_id: identity is stamped by the seller (the Neuron
// runtime), not by the sensor — see EncodeMessage and the runtime-boundary doc.

// DetectionReport is one SAPIENT detection about a single observed object. Per
// spec 017 FR-R-D01 exactly one report is emitted per decoded drone with a
// usable identity (the craft); the operator, when present, rides on this same
// report via rid.operator* ObjectInfo.
type DetectionReport struct {
	ReportID            string           `json:"reportID"`
	Timestamp           time.Time        `json:"timestamp"`
	SourceID            string           `json:"sourceID"`     // the ASM (sensor) identity
	ObjectID            string           `json:"objectID"`     // ULID assigned by the ASM (FR-R-D02)
	ID                  string           `json:"id,omitempty"` // native tail-number analogue = RID serial (CL-R2)
	Location            *Location        `json:"location,omitempty"`
	DetectionConfidence float64          `json:"detectionConfidence"`
	Velocity            *ENUVelocity     `json:"velocity,omitempty"`
	Classification      []Classification `json:"classification,omitempty"`
	Signal              *Signal          `json:"signal,omitempty"`
	ObjectInfo          []ObjectInfo     `json:"objectInfo,omitempty"`
	// AssociatedDetection is reserved for the case where the operator (or
	// another object) is independently tracked as its own detection — the
	// documented alternative in spec 017 note O1. The default DroneScout path
	// emits none, and encode.go does not map it.
	AssociatedDetection []Association `json:"associatedDetection,omitempty"`
}

// Location is a geodetic position with an explicit altitude datum and an
// uncertainty envelope. AltDatum distinguishes geometric (GNSS) from barometric.
type Location struct {
	Latitude         float64        `json:"latitude"`
	Longitude        float64        `json:"longitude"`
	AltitudeM        float64        `json:"altitudeM"`
	AltDatum         string         `json:"altDatum"` // "geometric" | "barometric"
	CoordinateSystem string         `json:"coordinateSystem"`
	Error            *LocationError `json:"error,omitempty"`
}

// LocationError carries the position uncertainty in metres, derived from the
// OpenDroneID accuracy enums. Absent when the source reports unknown.
type LocationError struct {
	HorizontalM float64 `json:"horizontalM,omitempty"`
	VerticalM   float64 `json:"verticalM,omitempty"`
}

// ENUVelocity is SAPIENT's native velocity vector in metres/second, East /
// North / Up (spec 017 M2): E = speed·sin(heading), N = speed·cos(heading),
// U = vertical rate.
type ENUVelocity struct {
	EastMPS  float64 `json:"eastMps"`
	NorthMPS float64 `json:"northMps"`
	// Not omitempty: a 0 Up component with a present Location means "level
	// flight", which is distinct from "no velocity".
	UpMPS float64 `json:"upMps"`
}

// Classification is one object-type hypothesis with a confidence in [0,1].
type Classification struct {
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	SubClass   string  `json:"subClass,omitempty"`
}

// Signal is the native SAPIENT RF signal block. The model carries the centre
// frequency in MHz (the channel-plan unit); encode.go converts to Hz (SI) at the
// wire boundary, since SAPIENT v2.0 makes centre_frequency mandatory when a
// signal is present and carries no unit enum (spec 017 FR-R-M08).
type Signal struct {
	Amplitude          int     `json:"amplitude"`          // RSSI, dBm
	CentreFrequencyMHz float64 `json:"centreFrequencyMHz"` // mandatory when signal present
}

// ObjectInfo is a structured type/value attribute. The Remote ID payload that
// has no native SAPIENT field rides here under the "rid.*" namespace. Per
// SAPIENT, every value is a string.
type ObjectInfo struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Units string `json:"units,omitempty"`
}

// Association links this object to another (e.g. operator ↔ drone).
type Association struct {
	ObjectID string `json:"objectID"`
	Relation string `json:"relation"`
}
