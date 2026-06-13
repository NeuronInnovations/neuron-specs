package sbs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Canonical SBS lines emitted by docs/neuron-rid-seller's bridge per
// README §181–187. The lines below are byte-identical with the README
// fixture — verified at implementation time, 2026-05-18.
const (
	ridLineDroneMSG1 = "MSG,1,,,FF0700,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,15810ABC,,,,,,,,0,0,0,0"
	ridLineDroneMSG3 = "MSG,3,,,FF0700,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,,,,090,50.1027,-5.6705,,,0,0,0,0"
	ridLineDroneMSG4 = "MSG,4,,,FF0700,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,,312,33,090,,,,,0,0,0,0"
	ridLineOpMSG1    = "MSG,1,,,FE5051,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,OP-O-001,,,,,,,,0,0,0,0"
	ridLineOpMSG2    = "MSG,2,,,FE5051,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,,49,0,0,50.1027,-5.6705,,,0,0,0,1"
)

// TestParseRIDMSG_Empty covers blank / whitespace-only / bare-CRLF lines.
func TestParseRIDMSG_Empty(t *testing.T) {
	t.Parallel()
	for _, line := range []string{"", "   ", "\r\n", "\n"} {
		_, err := ParseRIDMSG(line)
		assert.ErrorIs(t, err, ErrEmpty, "line=%q", line)
	}
}

// TestParseRIDMSG_NotMSG covers non-MSG record types (STA/SEL/ID/AIR/CLK
// + garbage). Mirrors ParseMSG behaviour.
func TestParseRIDMSG_NotMSG(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		"STA,1,1,1,A1B2C3,...",
		"SEL,1,1,1,A1B2C3,...",
		"ID,1,1,1,A1B2C3,...",
		"AIR,1,1,1,A1B2C3,...",
		"CLK,1,1,1,A1B2C3,...",
		"garbage,line",
	} {
		_, err := ParseRIDMSG(line)
		assert.ErrorIs(t, err, ErrNotMSG, "line=%q", line)
	}
}

// TestParseRIDMSG_ShortRecord covers truncated MSG lines (< 22 fields).
func TestParseRIDMSG_ShortRecord(t *testing.T) {
	t.Parallel()
	line := "MSG,3,1,1,A1B2C3,1,2026/05/15,12:00:00.100" // 8 fields
	_, err := ParseRIDMSG(line)
	assert.ErrorIs(t, err, ErrShortRecord)
}

// TestParseRIDMSG_UnsupportedTypes covers MSG types 5, 6, 7, 8 — which
// the RID dialect parser does not accept (the bridge never emits them).
func TestParseRIDMSG_UnsupportedTypes(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"5", "6", "7", "8"} {
		line := "MSG," + typ + ",,,FF0700,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,15810ABC,,,,,,,,0,0,0,0"
		_, err := ParseRIDMSG(line)
		assert.ErrorIs(t, err, ErrUnsupportedType, "type=%s", typ)
	}
}

// TestParseRIDMSG_MissingICAO ensures empty ICAO field rejects.
func TestParseRIDMSG_MissingICAO(t *testing.T) {
	t.Parallel()
	line := "MSG,1,,,,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,15810ABC,,,,,,,,0,0,0,0"
	_, err := ParseRIDMSG(line)
	assert.ErrorIs(t, err, ErrMissingICAO)
}

// TestParseRIDMSG_DroneMSG1 — drone identity message; carries Callsign
// (the vendor-encoded serial) but no position / velocity.
func TestParseRIDMSG_DroneMSG1(t *testing.T) {
	t.Parallel()
	got, err := ParseRIDMSG(ridLineDroneMSG1)
	require.NoError(t, err)

	assert.Equal(t, 1, got.MSGType)
	assert.Equal(t, "FF0700", got.ICAO, "FF prefix marks the drone; ICAO uppercased")
	assert.Equal(t, "15810ABC", got.Callsign, "drone MSG,1 carries the serial in field [10]")

	assert.False(t, got.LatSet, "MSG,1 has no position")
	assert.False(t, got.LonSet, "MSG,1 has no position")
	assert.False(t, got.AltSet)
	assert.False(t, got.SpdSet)
	assert.False(t, got.TrkSet)
	assert.False(t, got.VrtSet)
	assert.False(t, got.Rx.IsZero(), "Rx is always populated")
}

// TestParseRIDMSG_DroneMSG3 — drone airborne position; carries lat/lon
// and (per the canonical fixture) a heading on field [13].
func TestParseRIDMSG_DroneMSG3(t *testing.T) {
	t.Parallel()
	got, err := ParseRIDMSG(ridLineDroneMSG3)
	require.NoError(t, err)

	assert.Equal(t, 3, got.MSGType)
	assert.Equal(t, "FF0700", got.ICAO)
	assert.Empty(t, got.Callsign, "MSG,3 does not carry the serial")

	require.True(t, got.LatSet)
	assert.InDelta(t, 50.1027, got.Lat, 1e-9)
	require.True(t, got.LonSet)
	assert.InDelta(t, -5.6705, got.Lon, 1e-9)
	require.True(t, got.TrkSet, "fixture carries trk on field [13]")
	assert.InDelta(t, 90.0, got.TrkDeg, 1e-9)

	assert.False(t, got.AltSet, "fixture leaves altitude empty on MSG,3")
	assert.False(t, got.SpdSet)
	assert.False(t, got.VrtSet)
}

// TestParseRIDMSG_DroneMSG4 — drone velocity; carries SpdKnots and TrkDeg.
func TestParseRIDMSG_DroneMSG4(t *testing.T) {
	t.Parallel()
	got, err := ParseRIDMSG(ridLineDroneMSG4)
	require.NoError(t, err)

	assert.Equal(t, 4, got.MSGType)
	assert.Equal(t, "FF0700", got.ICAO)
	require.True(t, got.SpdSet)
	assert.InDelta(t, 33.0, got.SpdKnots, 1e-9)
	require.True(t, got.TrkSet)
	assert.InDelta(t, 90.0, got.TrkDeg, 1e-9)
	require.True(t, got.AltSet, "fixture carries altitude=312 on field [11]")
	assert.InDelta(t, 312.0, got.AltFeet, 1e-9)
	assert.False(t, got.LatSet, "MSG,4 has no position")
	assert.False(t, got.LonSet)
}

// TestParseRIDMSG_OperatorMSG1 — operator identity message; FE-prefixed
// ICAO and operator-ID in Callsign.
func TestParseRIDMSG_OperatorMSG1(t *testing.T) {
	t.Parallel()
	got, err := ParseRIDMSG(ridLineOpMSG1)
	require.NoError(t, err)

	assert.Equal(t, 1, got.MSGType)
	assert.Equal(t, "FE5051", got.ICAO, "FE prefix marks the operator")
	assert.Equal(t, "OP-O-001", got.Callsign, "operator MSG,1 carries the operator ID")
	assert.False(t, got.LatSet)
	assert.False(t, got.LonSet)
}

// TestParseRIDMSG_OperatorMSG2 — operator ground position.
func TestParseRIDMSG_OperatorMSG2(t *testing.T) {
	t.Parallel()
	got, err := ParseRIDMSG(ridLineOpMSG2)
	require.NoError(t, err)

	assert.Equal(t, 2, got.MSGType)
	assert.Equal(t, "FE5051", got.ICAO)
	require.True(t, got.LatSet)
	assert.InDelta(t, 50.1027, got.Lat, 1e-9)
	require.True(t, got.LonSet)
	assert.InDelta(t, -5.6705, got.Lon, 1e-9)
}

// TestParseRIDMSG_LowercaseICAOUppercased verifies the parser uppercases
// the ICAO field, mirroring ParseMSG.
func TestParseRIDMSG_LowercaseICAOUppercased(t *testing.T) {
	t.Parallel()
	line := "MSG,1,,,ff0700,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,15810ABC,,,,,,,,0,0,0,0"
	got, err := ParseRIDMSG(line)
	require.NoError(t, err)
	assert.Equal(t, "FF0700", got.ICAO)
}

// TestParseRIDMSG_TrimsTrailingCRLF mirrors ParseMSG behaviour.
func TestParseRIDMSG_TrimsTrailingCRLF(t *testing.T) {
	t.Parallel()
	line := ridLineDroneMSG3 + "\r\n"
	got, err := ParseRIDMSG(line)
	require.NoError(t, err)
	assert.Equal(t, "FF0700", got.ICAO)
}
