package remoteid

import "time"

// DecodedFrame is the in-memory representation of one Remote ID broadcast
// detection, after source-side decoding but before canonical-JSON envelope
// production.
//
// Field semantics follow spec 017 FR-R05 (the RemoteIdFrame canonical-JSON
// shape). The mapping to the wire format is intentionally one-to-one so
// that downstream normalization (internal/dapp/remoteid/frame.go) is a
// pure rename rather than a transformation:
//
//	DecodedFrame.Type             ↔ RemoteIdFrame "type"
//	DecodedFrame.Version          ↔ RemoteIdFrame "version"
//	DecodedFrame.ObservedAt       ↔ RemoteIdFrame "observedAt" (UnixNs)
//	DecodedFrame.Source           ↔ RemoteIdFrame "source"
//	DecodedFrame.DroneID          ↔ RemoteIdFrame "droneId"
//	DecodedFrame.DroneIDType      ↔ RemoteIdFrame "droneIdType"
//	DecodedFrame.Position         ↔ RemoteIdFrame "position" (omitted if nil)
//	DecodedFrame.Velocity         ↔ RemoteIdFrame "velocity" (omitted if nil)
//	DecodedFrame.Operator         ↔ RemoteIdFrame "operator" (omitted if nil)
//	DecodedFrame.RegulatorVariant ↔ RemoteIdFrame "regulatorVariant" (omitted if "")
//
// Optional fields use pointer or zero-value-elision semantics. The canonical
// JSON encoder MUST omit absent fields per 006 FR-W04 (do not emit null).
type DecodedFrame struct {
	// Type discriminator. Always "remote-id-frame" per 017 FR-R05.
	Type string

	// Version is the RemoteIdFrame schema version (semver). "1.0.0" for now.
	Version string

	// ObservedAt is the receiver-side timestamp of broadcast detection.
	// Source-supplied; preserved by replay; synthesized from wall-clock by
	// synthetic sources.
	ObservedAt time.Time

	// Source identifies the receiver model (e.g., "dronescout-ds400",
	// "replay", "synth"). Informational; not used for routing.
	Source string

	// DroneID is the Open Drone ID UAS Identification (ASTM F3411-22a Basic
	// ID message). For DroneIDType="serial", typically a vendor-encoded
	// 16-character serial number.
	DroneID string

	// DroneIDType is one of "serial", "caa", "utm", "specific-session" per
	// ASTM F3411-22a § 5.4.4.4.
	DroneIDType string

	// Position is optional; present when the upstream Location message has
	// been decoded.
	Position *Position

	// Velocity is optional; present when the upstream Location message
	// included velocity fields.
	Velocity *Velocity

	// Operator is optional; present when an Operator-ID + System message
	// pair has been decoded.
	Operator *Operator

	// RegulatorVariant identifies the upstream regulatory variant of the
	// broadcast. One of "asd-stan", "asd-faa", "asd-easa". Empty when
	// unknown.
	RegulatorVariant string
}

// Position is a WGS84 position with optional altitude and fix-quality.
type Position struct {
	Lat float64 // WGS84 latitude in degrees
	Lon float64 // WGS84 longitude in degrees
	Alt float64 // meters above WGS84 ellipsoid; zero when Fix=="none"
	Fix string  // "none" | "2D" | "3D"
}

// Velocity is a horizontal+vertical speed pair with a heading.
type Velocity struct {
	SpeedHorizontal float64 // m/s
	SpeedVertical   float64 // m/s; positive = up
	Track           float64 // degrees clockwise from true north
}

// Operator describes the registered operator of the drone, when an Open
// Drone ID Operator-ID + System message pair has been observed.
type Operator struct {
	IDType   string    // typically "caa" for civil aviation authority registration IDs
	ID       string    // operator-id assigned by the regulator
	Position *Position // optional; operator location reported in the System message
}
