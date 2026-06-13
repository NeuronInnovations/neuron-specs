package adsb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sbs "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
)

// posTrack builds a vanilla SBS-1 type-3-shaped record: position + altitude +
// callsign + squawk, with velocity fields UNSET (as a real type-3 line is).
func posTrack(icao string, lat, lon, altFeet float64) sbs.SBSTrack {
	return sbs.SBSTrack{
		ICAO:     icao,
		Callsign: "BAW178",
		Squawk:   "7000",
		Lat:      lat, LatSet: true,
		Lon:     lon,
		LonSet:  true,
		AltFeet: altFeet, AltSet: true,
		Rx: time.Now().UTC(),
	}
}

// velTrack builds a vanilla SBS-1 type-4-shaped record: ground speed + track +
// vertical rate, with position/altitude/callsign UNSET (as a real type-4 line).
func velTrack(icao string, spdKnots, trkDeg, vrtFpm float64) sbs.SBSTrack {
	return sbs.SBSTrack{
		ICAO:     icao,
		SpdKnots: spdKnots, SpdSet: true,
		TrkDeg: trkDeg, TrkSet: true,
		VrtFpm: vrtFpm, VrtSet: true,
		Rx: time.Now().UTC(),
	}
}

func TestTrackMerger_PositionThenVelocity(t *testing.T) {
	t.Parallel()
	m := newTrackMerger(0, 0)

	m.merge(posTrack("A1B2C3", 51.47, -0.4543, 37000))
	got := m.merge(velTrack("A1B2C3", 485, 290, 64))

	// Position retained from the earlier type-3 record.
	assert.True(t, got.LatSet)
	assert.InDelta(t, 51.47, got.Lat, 1e-9)
	assert.True(t, got.LonSet)
	assert.True(t, got.AltSet)
	assert.InDelta(t, 37000, got.AltFeet, 1e-9)
	// Velocity from the type-4 record.
	assert.True(t, got.SpdSet)
	assert.InDelta(t, 485, got.SpdKnots, 1e-9)
	assert.True(t, got.TrkSet)
	assert.InDelta(t, 290, got.TrkDeg, 1e-9)
	assert.True(t, got.VrtSet)
	// Callsign/squawk preserved (the type-4 record carries neither).
	assert.Equal(t, "BAW178", got.Callsign)
	assert.Equal(t, "7000", got.Squawk)
}

func TestTrackMerger_VelocityThenPosition(t *testing.T) {
	t.Parallel()
	m := newTrackMerger(0, 0)

	m.merge(velTrack("A1B2C3", 485, 290, 64))
	got := m.merge(posTrack("A1B2C3", 51.47, -0.4543, 37000))

	assert.True(t, got.LatSet, "position from the type-3 record")
	assert.InDelta(t, 51.47, got.Lat, 1e-9)
	assert.True(t, got.SpdSet, "velocity from the earlier type-4 record")
	assert.InDelta(t, 485, got.SpdKnots, 1e-9)
	assert.True(t, got.TrkSet)
	assert.InDelta(t, 290, got.TrkDeg, 1e-9)
}

func TestTrackMerger_PositionOnlyUpdatePreservesVelocity(t *testing.T) {
	t.Parallel()
	m := newTrackMerger(0, 0)

	m.merge(velTrack("A1B2C3", 485, 290, 64))
	m.merge(posTrack("A1B2C3", 51.47, -0.4543, 37000))
	// A later position-only update must NOT zero the previously-merged velocity.
	got := m.merge(posTrack("A1B2C3", 51.48, -0.4530, 37050))

	assert.InDelta(t, 51.48, got.Lat, 1e-9, "latest position wins")
	assert.InDelta(t, 37050, got.AltFeet, 1e-9)
	assert.True(t, got.SpdSet, "velocity must survive a position-only update")
	assert.InDelta(t, 485, got.SpdKnots, 1e-9)
	assert.True(t, got.TrkSet)
	assert.InDelta(t, 290, got.TrkDeg, 1e-9)
}

func TestTrackMerger_VelocityNeverObserved(t *testing.T) {
	t.Parallel()
	m := newTrackMerger(0, 0)

	got := m.merge(posTrack("A1B2C3", 51.47, -0.4543, 37000))

	assert.True(t, got.LatSet)
	assert.False(t, got.SpdSet, "velocity must stay absent when never observed")
	assert.False(t, got.TrkSet)
	assert.False(t, got.VrtSet)
}

func TestTrackMerger_IcaoIsolation(t *testing.T) {
	t.Parallel()
	m := newTrackMerger(0, 0)

	m.merge(velTrack("A1B2C3", 485, 290, 64))
	got := m.merge(posTrack("D5E6F7", 51.5074, -0.1278, 32000))

	assert.Equal(t, "D5E6F7", got.ICAO)
	assert.False(t, got.SpdSet, "one ICAO's velocity must not leak into another")
	assert.False(t, got.TrkSet)
}

func TestTrackMerger_TTLEvictionDropsStaleVelocity(t *testing.T) {
	t.Parallel()
	clock := time.Now().UTC()
	m := newTrackMerger(0, 50*time.Millisecond)
	m.now = func() time.Time { return clock }

	m.merge(velTrack("A1B2C3", 485, 290, 64))
	m.merge(posTrack("A1B2C3", 51.47, -0.4543, 37000)) // has velocity here

	// Advance the clock past the TTL: the entity went silent, so its cached
	// velocity must NOT resurface on a fresh position record.
	clock = clock.Add(2 * time.Second)
	got := m.merge(posTrack("A1B2C3", 51.48, -0.4530, 37050))

	assert.True(t, got.LatSet)
	assert.False(t, got.SpdSet, "stale velocity must be dropped after TTL expiry")
	assert.False(t, got.TrkSet)
}

func TestTrackMerger_CapEvictsOldest(t *testing.T) {
	t.Parallel()
	clock := time.Now().UTC()
	m := newTrackMerger(2, time.Hour) // cap=2, long TTL so only cap evicts
	m.now = func() time.Time { return clock }

	m.merge(posTrack("AAAAAA", 1, 1, 1000))
	clock = clock.Add(time.Second)
	m.merge(posTrack("BBBBBB", 2, 2, 2000))
	clock = clock.Add(time.Second)
	m.merge(posTrack("CCCCCC", 3, 3, 3000)) // exceeds cap → evict oldest (AAAAAA)

	assert.Len(t, m.m, 2, "cap bounds the cache size")
	_, hasA := m.m["AAAAAA"]
	_, hasB := m.m["BBBBBB"]
	_, hasC := m.m["CCCCCC"]
	assert.False(t, hasA, "oldest entry evicted at capacity")
	assert.True(t, hasB)
	assert.True(t, hasC)
}
