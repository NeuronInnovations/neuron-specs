package adsb

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sbs "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
)

func TestFromSBSTrack_FullRecord(t *testing.T) {
	t.Parallel()
	in := sbs.SBSTrack{
		ICAO:        "A1B2C3",
		Callsign:    "BAW178",
		Squawk:      "7000",
		Lat:         51.4700,
		LatSet:      true,
		Lon:         -0.4543,
		LonSet:      true,
		AltFeet:     37000,
		AltSet:      true,
		SpdKnots:    485,
		SpdSet:      true,
		TrkDeg:      290,
		TrkSet:      true,
		VrtFpm:      1280,
		VrtSet:      true,
		Receivers:   3,
		HorizErrM:   12.5,
		GeneratedAt: time.Date(2026, 5, 14, 14, 49, 0, 200_000_000, time.UTC),
	}

	nt := FromSBSTrack(in)

	assert.Equal(t, FrameType, nt.Type)
	assert.Equal(t, FrameVersion, nt.Version)
	assert.Equal(t, "2026-05-14T14:49:00.2Z", nt.ObservedAt.Format(time.RFC3339Nano))
	assert.Equal(t, SourceAdsb, nt.Source)
	assert.Equal(t, EntityTypeAircraft, nt.EntityType)
	assert.Equal(t, "A1B2C3", nt.EntityID)
	assert.Equal(t, "BAW178", nt.Callsign)
	assert.Equal(t, "7000", nt.Squawk)

	require.NotNil(t, nt.Position)
	assert.InDelta(t, 51.4700, nt.Position.Lat, 1e-9)
	assert.InDelta(t, -0.4543, nt.Position.Lon, 1e-9)
	assert.True(t, nt.Position.AltitudeSet)
	assert.InDelta(t, 11277.6, nt.Position.AltitudeM, 0.1, "37000 ft → 11277.6 m")

	require.NotNil(t, nt.Velocity)
	assert.True(t, nt.Velocity.GroundSpeedSet)
	assert.InDelta(t, 249.5, nt.Velocity.GroundSpeedMps, 1.0, "485 kn → ~249.5 m/s")
	assert.True(t, nt.Velocity.HeadingSet)
	assert.InDelta(t, 290.0, nt.Velocity.HeadingDeg, 1e-9)
	assert.True(t, nt.Velocity.VerticalRateSet)
	assert.InDelta(t, 6.5, nt.Velocity.VerticalRateMps, 0.1, "1280 fpm → ~6.5 m/s")

	require.NotNil(t, nt.Quality)
	assert.True(t, nt.Quality.ReceiversSet)
	assert.Equal(t, 3, nt.Quality.Receivers)
	assert.True(t, nt.Quality.HorizErrSet)
	assert.InDelta(t, 12.5, nt.Quality.HorizErrM, 1e-9)
}

func TestFromSBSTrack_PositionOnlyNoVelocityNoQuality(t *testing.T) {
	t.Parallel()
	in := sbs.SBSTrack{
		ICAO:        "D5E6F7",
		Callsign:    "KLM643",
		Squawk:      "7321",
		Lat:         51.5074,
		LatSet:      true,
		Lon:         -0.1278,
		LonSet:      true,
		AltFeet:     32000,
		AltSet:      true,
		GeneratedAt: time.Date(2026, 5, 14, 14, 49, 0, 500_000_000, time.UTC),
	}
	nt := FromSBSTrack(in)

	require.NotNil(t, nt.Position)
	assert.Nil(t, nt.Velocity, "no velocity fields set in upstream → omit Velocity")
	assert.Nil(t, nt.Quality, "no quality fields → omit Quality")
}

func TestFromSBSTrack_FallsBackToRxWhenGeneratedAtZero(t *testing.T) {
	t.Parallel()
	rx := time.Date(2026, 5, 14, 14, 49, 0, 0, time.UTC)
	in := sbs.SBSTrack{
		ICAO:   "A1B2C3",
		LatSet: true, Lat: 51.0,
		LonSet: true, Lon: 0.0,
		Rx: rx,
	}
	nt := FromSBSTrack(in)
	assert.Equal(t, rx, nt.ObservedAt, "missing GeneratedAt falls back to Rx")
}

func TestNormalizedTrack_MarshalJSONCanonicalOrder(t *testing.T) {
	t.Parallel()
	in := sbs.SBSTrack{
		ICAO:        "A1B2C3",
		Callsign:    "BAW178",
		Squawk:      "7000",
		Lat:         51.4700,
		LatSet:      true,
		Lon:         -0.4543,
		LonSet:      true,
		AltFeet:     37000,
		AltSet:      true,
		GeneratedAt: time.Date(2026, 5, 14, 14, 49, 0, 200_000_000, time.UTC),
	}
	nt := FromSBSTrack(in)

	b, err := json.Marshal(nt)
	require.NoError(t, err)
	got := string(b)

	// Field ordering: type, version, observedAt, source, entityType, entityID,
	// position, (no velocity in this fixture), callsign, squawk, (no quality).
	wantOrder := []string{
		`"type":"normalized-track"`,
		`"version":"1.0.0"`,
		`"observedAt":"2026-05-14T14:49:00.2Z"`,
		`"source":"adsb"`,
		`"entityType":"aircraft"`,
		`"entityID":"A1B2C3"`,
		`"position":`,
		`"callsign":"BAW178"`,
		`"squawk":"7000"`,
	}
	prev := -1
	for _, key := range wantOrder {
		idx := strings.Index(got, key)
		require.GreaterOrEqual(t, idx, 0, "key %q must appear in canonical JSON: %s", key, got)
		assert.Greater(t, idx, prev, "key %q must appear after the previous canonical-order key", key)
		prev = idx
	}

	// No velocity / no quality in this fixture; assert they're absent.
	assert.NotContains(t, got, `"velocity"`, "velocity must be omitted when not set")
	assert.NotContains(t, got, `"quality"`, "quality must be omitted when not set")
	assert.NotContains(t, got, `null`, "absent fields must be omitted, not null")
}

func TestNormalizedTrack_MarshalJSON_RoundTripsAllOptionalsSet(t *testing.T) {
	t.Parallel()
	original := FromSBSTrack(sbs.SBSTrack{
		ICAO:        "A1B2C3",
		Callsign:    "BAW178",
		Squawk:      "7000",
		Lat:         51.4700,
		LatSet:      true,
		Lon:         -0.4543,
		LonSet:      true,
		AltFeet:     37000,
		AltSet:      true,
		SpdKnots:    485,
		SpdSet:      true,
		TrkDeg:      290,
		TrkSet:      true,
		VrtFpm:      1280,
		VrtSet:      true,
		Receivers:   3,
		HorizErrM:   12.5,
		GeneratedAt: time.Date(2026, 5, 14, 14, 49, 0, 200_000_000, time.UTC),
	})

	b, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded NormalizedTrack
	require.NoError(t, json.Unmarshal(b, &decoded))

	assert.Equal(t, original.Type, decoded.Type)
	assert.Equal(t, original.Version, decoded.Version)
	assert.True(t, original.ObservedAt.Equal(decoded.ObservedAt))
	assert.Equal(t, original.Source, decoded.Source)
	assert.Equal(t, original.EntityType, decoded.EntityType)
	assert.Equal(t, original.EntityID, decoded.EntityID)
	assert.Equal(t, original.Callsign, decoded.Callsign)
	assert.Equal(t, original.Squawk, decoded.Squawk)

	require.NotNil(t, decoded.Position)
	assert.InDelta(t, original.Position.Lat, decoded.Position.Lat, 1e-9)
	assert.InDelta(t, original.Position.Lon, decoded.Position.Lon, 1e-9)
	assert.InDelta(t, original.Position.AltitudeM, decoded.Position.AltitudeM, 1e-9)
	assert.True(t, decoded.Position.AltitudeSet)

	require.NotNil(t, decoded.Velocity)
	assert.InDelta(t, original.Velocity.GroundSpeedMps, decoded.Velocity.GroundSpeedMps, 1e-9)
	assert.True(t, decoded.Velocity.GroundSpeedSet)

	require.NotNil(t, decoded.Quality)
	assert.Equal(t, 3, decoded.Quality.Receivers)
	assert.InDelta(t, 12.5, decoded.Quality.HorizErrM, 1e-9)
}

func TestMarshalJSON_RejectsMalformed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		track NormalizedTrack
	}{
		{
			name: "missing entityID",
			track: NormalizedTrack{
				Source: SourceAdsb, EntityType: EntityTypeAircraft,
				ObservedAt: time.Now(),
			},
		},
		{
			name: "invalid entityType",
			track: NormalizedTrack{
				Source: SourceAdsb, EntityType: "vehicle", EntityID: "ABC",
				ObservedAt: time.Now(),
			},
		},
		{
			name: "source/entityType mismatch — adsb+drone",
			track: NormalizedTrack{
				Source: SourceAdsb, EntityType: EntityTypeDrone, EntityID: "ABC",
				ObservedAt: time.Now(),
			},
		},
		{
			name: "source/entityType mismatch — remote-id+aircraft",
			track: NormalizedTrack{
				Source: SourceRemoteID, EntityType: EntityTypeAircraft, EntityID: "ABC",
				ObservedAt: time.Now(),
			},
		},
		{
			name: "unknown source",
			track: NormalizedTrack{
				Source: "vessel", EntityType: EntityTypeAircraft, EntityID: "ABC",
				ObservedAt: time.Now(),
			},
		},
	}
	for _, tc := range cases {
		_, err := json.Marshal(tc.track)
		assert.Error(t, err, "case=%s", tc.name)
	}
}

func TestCatalog_AdvertisesBasestationEntry(t *testing.T) {
	t.Parallel()
	entries := BuildAdsbBasestationStreamCatalog(DefaultCatalogOptions())
	require.Len(t, entries, 1, "v1 advertises only the basestation stream")
	assert.Equal(t, ProtocolBaseStation, entries[0].ProtocolID)
	assert.Equal(t, "basestation", entries[0].Name)
	assert.Equal(t, SchemaURL, entries[0].Schema)
}
