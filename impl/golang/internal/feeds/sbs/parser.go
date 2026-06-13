package sbs

import (
	"encoding/csv"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Sentinel parse errors returned by ParseMSG. Callers typically log+drop and
// continue rather than propagate, since SBS-1 streams routinely include lines
// of types we don't parse (1/4/5/6/7/8) and the occasional malformed line
// from an upstream decoder restart.
var (
	// ErrEmpty is returned for a blank or whitespace-only line.
	ErrEmpty = errors.New("sbs: empty line")
	// ErrNotMSG is returned when the record's first field is not "MSG"
	// (SBS-1 also defines STA/SEL/ID/AIR/CLK records — out of scope here).
	ErrNotMSG = errors.New("sbs: not a MSG line")
	// ErrUnsupportedType is returned for MSG records whose transmission type
	// (field [1]) is neither 3 (airborne position) nor 4 (airborne velocity) —
	// the two types this parser surfaces.
	ErrUnsupportedType = errors.New("sbs: unsupported MSG transmission type")
	// ErrShortRecord is returned when the record has fewer than 22 CSV fields.
	// Vanilla SBS-1 always has 22+ fields ([0..21]); shorter records indicate
	// upstream truncation or wire corruption.
	ErrShortRecord = errors.New("sbs: record has fewer than 22 fields")
	// ErrMissingICAO is returned when field [4] is empty. Every SBS-1 MSG
	// record must carry an ICAO; an empty one is malformed.
	ErrMissingICAO = errors.New("sbs: missing ICAO in field [4]")
)

// ParseMSG parses one SBS-1 BaseStation line (single logical record). The
// line MAY end with \r\n or \n (both are trimmed). The line MUST NOT contain
// embedded record terminators; readers split on \n before calling this.
//
// The parser handles both vanilla SBS-1 (22 fields) and the mlat-server variant
// (24 fields, adding trailing herr and rcv_users columns).
//
// Returns ErrEmpty for blank lines, ErrNotMSG for non-MSG records,
// ErrUnsupportedType for MSG records of types other than 3, ErrShortRecord
// for truncated records, and ErrMissingICAO when field [4] is empty. Other
// individual field parse failures are silently treated as "field absent"
// (the corresponding *Set boolean is left false).
func ParseMSG(line string) (*SBSTrack, error) {
	line = strings.TrimRight(line, "\r\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, ErrEmpty
	}

	// encoding/csv handles quoted fields containing commas correctly (some
	// callsigns / squawks / emerg values can contain commas in the
	// mlat-server variant).
	r := csv.NewReader(strings.NewReader(line))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	fields, err := r.Read()
	if err != nil {
		// csv parse errors propagate so callers can distinguish from logical
		// rejections; in practice we treat them the same as ErrShortRecord
		// (log+drop), but the distinction is preserved here.
		return nil, err
	}

	if len(fields) == 0 || strings.TrimSpace(fields[0]) != "MSG" {
		return nil, ErrNotMSG
	}
	if len(fields) < 22 {
		return nil, ErrShortRecord
	}

	// Transmission type filter: we parse type 3 (airborne position) AND type 4
	// (airborne velocity). In vanilla SBS-1 these are DISJOINT records per ICAO:
	// type 3 carries lat/lon/altitude with EMPTY ground-speed/track/vertical-rate
	// fields, while type 4 carries ground speed [12] / track [13] / vertical rate
	// [16] with EMPTY position fields. Capturing both is required to recover
	// velocity from a vanilla JetVision feed; the seller merges the two by ICAO
	// downstream (see internal/dapp/adsb merge cache). Types 1/2/5/6/7/8 remain
	// out of scope in v1.
	transType := strings.TrimSpace(fields[1])
	if transType != "3" && transType != "4" {
		return nil, ErrUnsupportedType
	}

	icao := strings.ToUpper(strings.TrimSpace(fields[4]))
	if icao == "" {
		return nil, ErrMissingICAO
	}

	t := &SBSTrack{
		ICAO:     icao,
		Callsign: strings.TrimSpace(fields[10]),
		Squawk:   strings.TrimSpace(fields[17]),
		EmergRaw: strings.TrimSpace(fields[19]),
		Rx:       time.Now().UTC(),
	}

	if v, ok := parseFloat(fields[14]); ok {
		t.Lat, t.LatSet = v, true
	}
	if v, ok := parseFloat(fields[15]); ok {
		t.Lon, t.LonSet = v, true
	}
	if v, ok := parseFloat(fields[11]); ok {
		t.AltFeet, t.AltSet = v, true
	}
	if v, ok := parseFloat(fields[12]); ok {
		t.SpdKnots, t.SpdSet = v, true
	}
	if v, ok := parseFloat(fields[13]); ok {
		t.TrkDeg, t.TrkSet = v, true
	}
	if v, ok := parseFloat(fields[16]); ok {
		t.VrtFpm, t.VrtSet = v, true
	}

	if v, err := strconv.Atoi(strings.TrimSpace(fields[18])); err == nil && v >= 0 {
		t.Receivers = v
	}

	// SPI: per SBS-1 spec field [20] is "0" or "1"; mlat-server sometimes
	// emits "-1" as an alternative "set" sentinel. Treat both 1 and -1 as
	// "SPI set"; everything else as unset.
	spi := strings.TrimSpace(fields[20])
	if spi == "1" || spi == "-1" {
		t.SPI = true
	}

	// Trailing herr column (mlat-server variant) — field [22], metres.
	if len(fields) >= 23 {
		if v, ok := parseFloat(fields[22]); ok && v >= 0 && v < 1e7 {
			t.HorizErrM = v
		}
	}

	// Trailing rcv_users column (mlat-server variant) — field [23],
	// comma-separated within the column (which csv.Reader has already
	// extracted as a single field due to the trailing-column being unquoted
	// when no commas inside — but if commas exist within rcv_users the
	// upstream MUST quote the column. We split on commas defensively.)
	if len(fields) >= 24 {
		if c := splitContributors(fields[23]); len(c) > 0 {
			t.Contributors = c
		}
	}

	// GeneratedAt: combine fields [6] (YYYY/MM/DD) + [7] (HH:MM:SS.SSS) as UTC.
	if g, ok := parseGenerated(fields[6], fields[7]); ok {
		t.GeneratedAt = g
	}

	return t, nil
}

// parseFloat is the canonical "optional float field" parser: empty / unparseable
// → (0, false); otherwise (value, true).
func parseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// splitContributors splits a comma-separated rcv_users column into trimmed
// non-empty strings. Returns nil for an empty / all-blank input.
func splitContributors(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseGenerated combines an SBS-1 generated-date field ("YYYY/MM/DD") and
// generated-time field ("HH:MM:SS.SSS") into a UTC time.Time. Returns
// (time.Time{}, false) on any parse failure.
func parseGenerated(dateField, timeField string) (time.Time, bool) {
	d := strings.TrimSpace(dateField)
	tm := strings.TrimSpace(timeField)
	if d == "" || tm == "" {
		return time.Time{}, false
	}
	combined := d + " " + tm
	// SBS-1 uses YYYY/MM/DD with slash separators and HH:MM:SS.SSS with
	// millisecond precision. Some upstreams emit nanosecond precision; the
	// .000 → .999999 range covers both.
	for _, layout := range []string{
		"2006/01/02 15:04:05.000",
		"2006/01/02 15:04:05.000000",
		"2006/01/02 15:04:05.000000000",
		"2006/01/02 15:04:05",
	} {
		if t, err := time.Parse(layout, combined); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
