package sbs

import (
	"encoding/csv"
	"strings"
	"time"
)

// Sentinel parse errors returned by ParseRIDMSG. The error vocabulary
// intentionally mirrors ParseMSG (ErrEmpty, ErrNotMSG, ErrShortRecord,
// ErrMissingICAO, ErrUnsupportedType) so callers can share the
// log-and-drop convention across both parsers without re-mapping.
//
// ParseRIDMSG accepts MSG transmission types 1, 2, 3, and 4. Any other
// type yields ErrUnsupportedType (re-used from parser.go). This widens
// the strict MSG,3-only contract of ParseMSG, which the JetVision ADS-B
// seller (cmd/adsb-seller) depends on; the two parsers are therefore
// kept as siblings rather than refactoring the legacy code.

// RIDSBSRecord is one decoded SBS-1 MSG record from a Remote-ID-dialect
// SBS exporter (the BlueMark "neuron-rid-bridge" running on VPS 1, or
// any compatible bridge). The dialect splits drone and operator into
// two distinct ICAO records:
//
//   - FF*  → drone records;   MSG,1 carries the serial as Callsign;
//                              MSG,3 carries airborne position;
//                              MSG,4 carries velocity (SpdKnots, TrkDeg).
//   - FE*  → operator records; MSG,1 carries the operator ID;
//                              MSG,2 carries the operator's ground position.
//
// Units are preserved as the upstream emits them — imperial for
// AltFeet/SpdKnots/VrtFpm; degrees for Lat/Lon/TrkDeg. Conversion to SI
// metric is the consumer's responsibility (see
// internal/feeds/remoteid.RunBasestation).
//
// All optional fields use the same *Set companion-boolean convention as
// SBSTrack: presence in the upstream record (non-empty field) is
// recorded via the Set boolean; "field absent" and "field present and
// zero" are distinguishable.
type RIDSBSRecord struct {
	// MSGType is the SBS-1 transmission type from field [1]. One of 1, 2, 3, 4.
	MSGType int

	// ICAO is the record ICAO from field [4], uppercased on parse. The
	// dialect uses the high-nibble prefix to distinguish drones (FF*)
	// from operators (FE*); callers MUST inspect the prefix before
	// dispatch.
	ICAO string

	// Callsign is the trimmed text of field [10]. For drone MSG,1 it is
	// typically the vendor-encoded serial; for operator MSG,1 it is the
	// operator ID. Empty on MSG,2/3/4 records.
	Callsign string

	// Lat / Lon are the geographic coordinates from fields [14] / [15].
	// LatSet / LonSet are true when the upstream field was non-empty.
	// Present on MSG,2 (operator ground position) and MSG,3 (drone
	// airborne position).
	Lat    float64
	LatSet bool
	Lon    float64
	LonSet bool

	// AltFeet is the altitude in feet from field [11]. Present on
	// MSG,3 records.
	AltFeet float64
	AltSet  bool

	// SpdKnots is the ground speed in knots from field [12]. Present
	// on MSG,4 (drone velocity).
	SpdKnots float64
	SpdSet   bool

	// TrkDeg is the heading in degrees true track from field [13].
	// Present on MSG,4 (drone velocity); also sometimes present on
	// MSG,3.
	TrkDeg float64
	TrkSet bool

	// VrtFpm is the vertical rate in feet per minute from field [16].
	// Sign convention matches SBSTrack: positive = climbing.
	VrtFpm float64
	VrtSet bool

	// Rx is the local wall-clock instant at parse time. Always
	// populated; mirrors SBSTrack.Rx.
	Rx time.Time
}

// ParseRIDMSG parses one SBS-1 BaseStation line emitted by a Remote-ID
// dialect exporter. Trims trailing \r\n or \n.
//
// Accepts MSG transmission types 1, 2, 3, 4. Returns
// ErrUnsupportedType for any other type (5, 6, 7, 8, …) and the
// shared ErrEmpty / ErrNotMSG / ErrShortRecord / ErrMissingICAO
// sentinels for the parse-time-rejection cases that ParseMSG already
// defines.
//
// The function intentionally duplicates the small `parseFloat` helper
// from parser.go locally rather than refactoring a shared helper; the
// adsb-seller depends on parser.go's exact byte layout (strict
// MSG,3-only contract) and we keep parser.go untouched on principle.
func ParseRIDMSG(line string) (*RIDSBSRecord, error) {
	line = strings.TrimRight(line, "\r\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, ErrEmpty
	}

	// encoding/csv handles quoted fields containing commas correctly.
	// The bridge does not currently emit quoted commas, but matching
	// parser.go's tolerance avoids future surprises.
	r := csv.NewReader(strings.NewReader(line))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	fields, err := r.Read()
	if err != nil {
		// csv parse errors propagate so callers can distinguish from
		// logical rejections; in practice we treat them the same as
		// ErrShortRecord (log+drop), but the distinction is preserved.
		return nil, err
	}

	if len(fields) == 0 || strings.TrimSpace(fields[0]) != "MSG" {
		return nil, ErrNotMSG
	}
	if len(fields) < 22 {
		return nil, ErrShortRecord
	}

	// Transmission type filter — RID dialect supports 1, 2, 3, 4.
	transType := strings.TrimSpace(fields[1])
	switch transType {
	case "1", "2", "3", "4":
		// supported
	default:
		return nil, ErrUnsupportedType
	}

	icao := strings.ToUpper(strings.TrimSpace(fields[4]))
	if icao == "" {
		return nil, ErrMissingICAO
	}

	rec := &RIDSBSRecord{
		ICAO:     icao,
		Callsign: strings.TrimSpace(fields[10]),
		Rx:       time.Now().UTC(),
	}

	switch transType {
	case "1":
		rec.MSGType = 1
	case "2":
		rec.MSGType = 2
	case "3":
		rec.MSGType = 3
	case "4":
		rec.MSGType = 4
	}

	// All optional fields use the same *Set convention as parser.go.
	// We attempt to parse every field on every record type rather than
	// branching per-type — the upstream bridge tends to leave irrelevant
	// fields empty, and parseFloat (re-used from parser.go in the same
	// package) returns ok=false for empty inputs, so we never spuriously
	// set a value.
	if v, ok := parseFloat(fields[14]); ok {
		rec.Lat, rec.LatSet = v, true
	}
	if v, ok := parseFloat(fields[15]); ok {
		rec.Lon, rec.LonSet = v, true
	}
	if v, ok := parseFloat(fields[11]); ok {
		rec.AltFeet, rec.AltSet = v, true
	}
	if v, ok := parseFloat(fields[12]); ok {
		rec.SpdKnots, rec.SpdSet = v, true
	}
	if v, ok := parseFloat(fields[13]); ok {
		rec.TrkDeg, rec.TrkSet = v, true
	}
	if v, ok := parseFloat(fields[16]); ok {
		rec.VrtFpm, rec.VrtSet = v, true
	}

	return rec, nil
}
