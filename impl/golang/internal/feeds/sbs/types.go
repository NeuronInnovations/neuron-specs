package sbs

import "time"

// SBSTrack is one decoded SBS-1 MSG record (transmission type 3, airborne
// position). Units are preserved as upstream emits them — imperial for
// altitude / ground speed / vertical rate; degrees for latitude / longitude
// / track. Conversion to SI metric is the consumer's responsibility (see
// internal/dapp/adsb.FromSBSTrack).
//
// All optional fields default to the zero value of their Go type when absent
// in the upstream record. The presence of zero values does NOT distinguish
// "field absent" from "field present and zero" — that distinction is preserved
// in the *Set companion booleans where it matters. A more elaborate optional
// representation (pointer fields, sql.Null* style) is deferred to v2; the
// current shape is what bsline.go has shipped in production.
type SBSTrack struct {
	// ICAO is the aircraft Mode-S address as uppercase hex (6 chars). Field [4].
	// Always present in a valid MSG record. The parser uppercases on parse.
	ICAO string

	// Callsign is the aircraft callsign, padded with spaces in SBS-1 wire form,
	// trimmed here. Field [10]. Empty when the BaseStation has not yet observed
	// a callsign for this aircraft (MSG type 1 may carry it; v1 of this parser
	// drops type 1 records but field [10] is populated on type 3 records when
	// the BaseStation cached it).
	Callsign string

	// Squawk is the 4-digit octal transponder code as a decimal string (e.g.,
	// "7000"). Field [17]. Empty when absent.
	Squawk string

	// Lat is the geographic latitude in degrees, range [-90, 90]. Field [14].
	// LatSet is true when field [14] was non-empty in the upstream record;
	// a true LatSet with Lat == 0 is a valid "aircraft at the equator" reading,
	// distinguishable from "no position decoded yet" (LatSet == false).
	Lat    float64
	LatSet bool

	// Lon is the geographic longitude in degrees, range [-180, 180]. Field [15].
	// LonSet semantics mirror LatSet.
	Lon    float64
	LonSet bool

	// AltFeet is the barometric altitude in feet. Field [11]. AltSet
	// distinguishes "altitude 0" from "altitude absent".
	AltFeet float64
	AltSet  bool

	// SpdKnots is the ground speed in knots. Field [12].
	SpdKnots float64
	SpdSet   bool

	// TrkDeg is the track over ground in degrees, range [0, 360). Field [13].
	TrkDeg float64
	TrkSet bool

	// VrtFpm is the vertical rate in feet per minute. Field [16]. Sign: positive
	// = climbing, negative = descending.
	VrtFpm float64
	VrtSet bool

	// Receivers is the number of contributing MLAT sensors (mlat-server variant
	// only — field [18] is "alert" in vanilla SBS-1 but mlat-server places the
	// receiver count there). Zero when absent.
	Receivers int

	// EmergRaw is the raw text of the "emerg" column (field [19]). In vanilla
	// SBS-1 it is a boolean; in mlat-server it is the horizontal error in metres
	// on solver success (and a non-numeric solver-failure hint otherwise).
	// Consumers MUST parse this themselves per upstream variant.
	EmergRaw string

	// SPI is the special-position-indicator bit (field [20]).
	SPI bool

	// HorizErrM is the horizontal uncertainty in metres. Sourced from the
	// trailing `herr` column (field [22], mlat-server variant) when present;
	// 0 otherwise. Consumers MAY also derive this from a numeric EmergRaw per
	// mlat-server convention.
	HorizErrM float64

	// Contributors is the list of contributing MLAT sensor identifiers, from
	// the trailing `rcv_users` column (field [23], mlat-server variant). Nil
	// when absent.
	Contributors []string

	// GeneratedAt is the upstream-stamped record timestamp (fields [6] + [7]:
	// generated date YYYY/MM/DD + generated time HH:MM:SS.SSS), parsed as UTC.
	// Falls back to time.Time zero value when the upstream fields are missing
	// or unparseable.
	GeneratedAt time.Time

	// Rx is the local wall-clock instant the parser observed the line. Always
	// populated.
	Rx time.Time
}

// ReplayOptions tunes file-replay behavior for RunBaseStationReplay.
type ReplayOptions struct {
	// Speedup multiplies the natural emit rate based on the records' embedded
	// timestamps. 1.0 = real-time playback; 100.0 = 100x faster; <= 0 treated
	// as 1.0.
	Speedup float64

	// FrameInterval is the delay between consecutive records when the source
	// has no usable embedded timing (e.g., all GeneratedAt are zero). Zero
	// means "go as fast as possible".
	FrameInterval time.Duration

	// Loop, when true, restarts the file from the beginning after EOF.
	Loop bool
}

// SynthOptions tunes RunBaseStationSynth.
type SynthOptions struct {
	// Aircraft is the number of synthetic aircraft to emit. Each is given a
	// stable ICAO derived from the index, a sequential callsign ("SYNTH-NNN"),
	// and a deterministic circular orbit around (CenterLat, CenterLon).
	// Must be >= 1.
	Aircraft int

	// Fps is the per-aircraft frame rate (records per second). Must be >= 1.
	Fps int

	// CenterLat / CenterLon anchor the synthetic orbits. Default (51.4775,
	// -0.4614) — Heathrow — when both are zero.
	CenterLat float64
	CenterLon float64

	// RadiusKm is the orbit radius in kilometres around the centre. Default
	// 10 when zero.
	RadiusKm float64

	// AltitudeFeet is the synthetic cruise altitude. Default 37000 when zero.
	AltitudeFeet float64
}
