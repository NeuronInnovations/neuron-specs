package sbs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMSG_Empty(t *testing.T) {
	t.Parallel()
	cases := []string{"", "   ", "\r\n", "\n"}
	for _, line := range cases {
		_, err := ParseMSG(line)
		assert.ErrorIs(t, err, ErrEmpty, "line=%q", line)
	}
}

func TestParseMSG_NotMSG(t *testing.T) {
	t.Parallel()
	cases := []string{
		"STA,1,1,1,A1B2C3,...",
		"SEL,1,1,1,A1B2C3,...",
		"ID,1,1,1,A1B2C3,...",
		"AIR,1,1,1,A1B2C3,...",
		"CLK,1,1,1,A1B2C3,...",
		"garbage,line",
	}
	for _, line := range cases {
		_, err := ParseMSG(line)
		assert.ErrorIs(t, err, ErrNotMSG, "line=%q", line)
	}
}

func TestParseMSG_UnsupportedTypes(t *testing.T) {
	t.Parallel()
	// MSG types 1, 2, 5, 6, 7, 8 are valid SBS-1 records the parser drops.
	// Types 3 (airborne position) and 4 (airborne velocity) ARE parsed — see
	// TestParseMSG_VanillaHappyPath and TestParseMSG_Type4Velocity.
	for _, typ := range []string{"1", "2", "5", "6", "7", "8"} {
		// 22-field SBS-1 record (21 commas). All non-required fields empty.
		line := "MSG," + typ + ",1,1,A1B2C3,1,2026/05/14,14:49:00.123,2026/05/14,14:49:00.123,BAW178,,,,,,,,,,,"
		_, err := ParseMSG(line)
		assert.ErrorIs(t, err, ErrUnsupportedType, "type=%s", typ)
	}
}

func TestParseMSG_Type4Velocity(t *testing.T) {
	t.Parallel()
	// Vanilla SBS-1 MSG type 4 (airborne velocity): ground speed [12], track
	// [13], vertical rate [16] are populated; position [14]/[15] is EMPTY. This
	// is where velocity lives in vanilla JetVision SBS — type 3 does NOT carry it.
	line := "MSG,4,1,1,a1b2c3,1,2026/05/14,14:49:00.300,2026/05/14,14:49:00.300,,,485,290,,,64,,,,,"
	got, err := ParseMSG(line)
	require.NoError(t, err)

	assert.Equal(t, "A1B2C3", got.ICAO, "ICAO must be UPPER-cased by the parser")

	assert.True(t, got.SpdSet, "type-4 ground speed must be set")
	assert.InDelta(t, 485.0, got.SpdKnots, 1e-9)
	assert.True(t, got.TrkSet, "type-4 track must be set")
	assert.InDelta(t, 290.0, got.TrkDeg, 1e-9)
	assert.True(t, got.VrtSet, "type-4 vertical rate must be set")
	assert.InDelta(t, 64.0, got.VrtFpm, 1e-9)

	// Type 4 carries no position/altitude — those stay unset.
	assert.False(t, got.LatSet, "type-4 record has no latitude")
	assert.False(t, got.LonSet, "type-4 record has no longitude")
	assert.False(t, got.AltSet, "type-4 record has no altitude")
	assert.Empty(t, got.Callsign, "type-4 record has no callsign")
}

func TestParseMSG_Type4NoPosition(t *testing.T) {
	t.Parallel()
	// A type-4 record must still carry ICAO identity for merge-by-ICAO, but
	// must not be mistaken for a position fix (no lat/lon).
	line := "MSG,4,1,1,D5E6F7,1,2026/05/14,14:49:00.300,2026/05/14,14:49:00.300,,,420,180,,,0,,,,,"
	got, err := ParseMSG(line)
	require.NoError(t, err)

	assert.Equal(t, "D5E6F7", got.ICAO)
	assert.False(t, got.LatSet, "no position in a velocity record")
	assert.False(t, got.LonSet, "no position in a velocity record")
	assert.True(t, got.SpdSet)
	assert.InDelta(t, 420.0, got.SpdKnots, 1e-9)
	assert.True(t, got.TrkSet)
	assert.InDelta(t, 180.0, got.TrkDeg, 1e-9)
}

func TestParseMSG_ShortRecord(t *testing.T) {
	t.Parallel()
	line := "MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.123" // only 8 fields
	_, err := ParseMSG(line)
	assert.ErrorIs(t, err, ErrShortRecord)
}

func TestParseMSG_MissingICAO(t *testing.T) {
	t.Parallel()
	// All fields present but field [4] (ICAO) is empty.
	line := "MSG,3,1,1,,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,,,51.4700,-0.4543,,7000,0,0,0,0"
	_, err := ParseMSG(line)
	assert.ErrorIs(t, err, ErrMissingICAO)
}

func TestParseMSG_VanillaHappyPath(t *testing.T) {
	t.Parallel()
	line := "MSG,3,1,1,a1b2c3,1,2026/05/14,14:49:00.200,2026/05/14,14:49:00.200,BAW178,37000,,,51.4700,-0.4543,,7000,0,0,0,0"
	got, err := ParseMSG(line)
	require.NoError(t, err)

	assert.Equal(t, "A1B2C3", got.ICAO, "ICAO must be UPPER-cased by the parser")
	assert.Equal(t, "BAW178", got.Callsign)
	assert.Equal(t, "7000", got.Squawk)

	assert.True(t, got.LatSet)
	assert.InDelta(t, 51.4700, got.Lat, 1e-9)
	assert.True(t, got.LonSet)
	assert.InDelta(t, -0.4543, got.Lon, 1e-9)
	assert.True(t, got.AltSet)
	assert.InDelta(t, 37000, got.AltFeet, 1e-9)

	assert.False(t, got.SpdSet, "empty ground speed field must remain unset")
	assert.False(t, got.TrkSet, "empty track field must remain unset")
	assert.False(t, got.VrtSet, "empty vertical rate must remain unset")

	assert.Equal(t, 0, got.Receivers)
	assert.False(t, got.SPI)
	assert.InDelta(t, 0.0, got.HorizErrM, 1e-9, "no trailing herr in vanilla")
	assert.Nil(t, got.Contributors, "no trailing rcv_users in vanilla")

	require.False(t, got.GeneratedAt.IsZero(), "GeneratedAt must parse")
	assert.Equal(t, "2026-05-14T14:49:00.2Z", got.GeneratedAt.UTC().Format(time.RFC3339Nano))
	assert.False(t, got.Rx.IsZero(), "Rx is always populated")
}

func TestParseMSG_MlatVariantWithTrailingFields(t *testing.T) {
	t.Parallel()
	// Single contributor — unquoted, fits in field [23].
	line := "MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,485,290,51.4700,-0.4543,0,7000,3,12.5,0,0,12.5,sensor-1"
	got, err := ParseMSG(line)
	require.NoError(t, err)

	assert.Equal(t, 3, got.Receivers, "mlat-server receivers count in field [18]")
	assert.InDelta(t, 12.5, got.HorizErrM, 1e-9, "mlat-server herr in field [22]")
	require.Len(t, got.Contributors, 1)
	assert.Equal(t, "sensor-1", got.Contributors[0])

	assert.True(t, got.SpdSet)
	assert.InDelta(t, 485.0, got.SpdKnots, 1e-9)
	assert.True(t, got.TrkSet)
	assert.InDelta(t, 290.0, got.TrkDeg, 1e-9)
}

func TestParseMSG_MlatVariantQuotedMultiContributor(t *testing.T) {
	t.Parallel()
	line := `MSG,3,1,1,D5E6F7,1,2026/05/14,14:49:00.500,2026/05/14,14:49:00.500,KLM643,32000,420,180,51.5074,-0.1278,0,7321,5,8.2,0,0,8.2,"sensor-1,sensor-2,sensor-3,sensor-4,sensor-5"`
	got, err := ParseMSG(line)
	require.NoError(t, err)

	assert.Equal(t, 5, got.Receivers)
	assert.InDelta(t, 8.2, got.HorizErrM, 1e-9)
	require.Len(t, got.Contributors, 5, "quoted comma-separated contributors all parsed")
	assert.Equal(t, []string{"sensor-1", "sensor-2", "sensor-3", "sensor-4", "sensor-5"}, got.Contributors)
}

func TestParseMSG_VerticalRateSign(t *testing.T) {
	t.Parallel()
	// Climbing aircraft: positive fpm.
	climbLine := "MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,485,290,51.4700,-0.4543,1280,7000,0,0,0,0"
	got, err := ParseMSG(climbLine)
	require.NoError(t, err)
	assert.True(t, got.VrtSet)
	assert.InDelta(t, 1280.0, got.VrtFpm, 1e-9)

	// Descending: negative fpm.
	descendLine := "MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,485,290,51.4700,-0.4543,-1024,7000,0,0,0,0"
	got2, err := ParseMSG(descendLine)
	require.NoError(t, err)
	assert.True(t, got2.VrtSet)
	assert.InDelta(t, -1024.0, got2.VrtFpm, 1e-9)
}

func TestParseMSG_SPISet(t *testing.T) {
	t.Parallel()
	// Field layout: ...,51.4700,-0.4543,<vrt>,<squawk>,<alert>,<emerg>,<spi>,<onground>
	// indices [14]=lat [15]=lon [16]=vrt [17]=squawk [18]=alert [19]=emerg [20]=spi [21]=onground
	for _, v := range []string{"1", "-1"} {
		line := "MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,,,51.4700,-0.4543,,7000,0,0," + v + ",0"
		got, err := ParseMSG(line)
		require.NoError(t, err, "value=%q", v)
		assert.True(t, got.SPI, "SPI must be true for value %q", v)
	}

	// Anything else: not set.
	line := "MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,,,51.4700,-0.4543,,7000,0,0,0,0"
	got, err := ParseMSG(line)
	require.NoError(t, err)
	assert.False(t, got.SPI)
}

func TestParseMSG_TrimsTrailingCRLF(t *testing.T) {
	t.Parallel()
	line := "MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.200,2026/05/14,14:49:00.200,BAW178,37000,,,51.4700,-0.4543,,7000,0,0,0,0\r\n"
	got, err := ParseMSG(line)
	require.NoError(t, err)
	assert.Equal(t, "A1B2C3", got.ICAO)
}

func TestParseMSG_LatLonZeroDistinguishable(t *testing.T) {
	t.Parallel()
	// Aircraft at the equator + prime meridian: lat=0, lon=0 but both Set==true.
	zeroLine := "MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,485,290,0,0,0,7000,0,0,0,0"
	got, err := ParseMSG(zeroLine)
	require.NoError(t, err)
	assert.True(t, got.LatSet)
	assert.True(t, got.LonSet)
	assert.Equal(t, 0.0, got.Lat)
	assert.Equal(t, 0.0, got.Lon)

	// Aircraft with no position decoded: lat and lon BOTH absent.
	noPosLine := "MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,485,290,,,0,7000,0,0,0,0"
	got2, err := ParseMSG(noPosLine)
	require.NoError(t, err)
	assert.False(t, got2.LatSet)
	assert.False(t, got2.LonSet)
}

func TestParseMSG_VanillaFixtureFileEndToEnd(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "vanilla-jv.sbs")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(string(data), "\n")
	var parsed int
	var unsupported int
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		_, err := ParseMSG(line)
		switch {
		case err == nil:
			parsed++
		case errors.Is(err, ErrUnsupportedType):
			unsupported++
		default:
			t.Errorf("unexpected parse error on line %q: %v", line, err)
		}
	}
	assert.Equal(t, 7, parsed, "fixture has 6 type-3 + 1 type-4 records the parser accepts")
	assert.Equal(t, 3, unsupported, "fixture has 3 dropped records (types 1, 5, 7)")
}

func TestParseMSG_MlatFixtureFileEndToEnd(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "mlat-server.sbs")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(string(data), "\n")
	var tracks []*SBSTrack
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		t2, err := ParseMSG(line)
		require.NoError(t, err)
		tracks = append(tracks, t2)
	}
	require.Len(t, tracks, 3)

	// First track: 1 contributor.
	assert.Equal(t, 3, tracks[0].Receivers)
	require.Len(t, tracks[0].Contributors, 1)

	// Second track: 5 contributors (quoted multi).
	assert.Equal(t, 5, tracks[1].Receivers)
	require.Len(t, tracks[1].Contributors, 5)

	// Third track: 2 contributors (quoted).
	assert.Equal(t, 4, tracks[2].Receivers)
	require.Len(t, tracks[2].Contributors, 2)
}
