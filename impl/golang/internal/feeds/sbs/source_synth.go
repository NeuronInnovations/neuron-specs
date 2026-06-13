package sbs

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// RunBaseStationSynth emits deterministic synthetic SBSTrack values for
// `opts.Aircraft` aircraft, each describing a circular orbit around
// (opts.CenterLat, opts.CenterLon) of radius opts.RadiusKm at altitude
// opts.AltitudeFeet, at `opts.Fps` records per second per aircraft.
//
// Aircraft IDs (ICAOs) are derived deterministically: 0xSYN000 .. 0xSYN0FF
// (capped at 256 aircraft per call). Callsigns: SYNTH-000 .. SYNTH-255.
//
// Defaults:
//   - CenterLat / CenterLon (51.4775, -0.4614) — Heathrow — when both zero.
//   - RadiusKm 10 when zero.
//   - AltitudeFeet 37000 when zero.
//
// Useful for unit tests, mock-bus E2E tests, and offline development.
//
// Does NOT close out.
func RunBaseStationSynth(ctx context.Context, opts SynthOptions, out chan<- SBSTrack) error {
	if opts.Aircraft <= 0 {
		return errors.New("sbs: RunBaseStationSynth opts.Aircraft must be >= 1")
	}
	if opts.Fps <= 0 {
		return errors.New("sbs: RunBaseStationSynth opts.Fps must be >= 1")
	}
	if opts.Aircraft > 256 {
		opts.Aircraft = 256
	}

	centerLat, centerLon := opts.CenterLat, opts.CenterLon
	if centerLat == 0 && centerLon == 0 {
		centerLat, centerLon = 51.4775, -0.4614
	}
	radiusKm := opts.RadiusKm
	if radiusKm <= 0 {
		radiusKm = 10
	}
	altitudeFt := opts.AltitudeFeet
	if altitudeFt <= 0 {
		altitudeFt = 37000
	}

	// One record per (aircraft, tick); the ticker fires fps times per second
	// and the loop emits one record per aircraft per tick.
	interval := time.Second / time.Duration(opts.Fps)
	if interval <= 0 {
		interval = time.Microsecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	startWall := time.Now().UTC()
	var seq uint64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		now := startWall.Add(time.Duration(seq) * interval)
		// Each aircraft completes one orbit in 60 seconds (independent of fps).
		phase := float64(seq) / float64(opts.Fps) / 60.0 // fraction of orbit

		for i := 0; i < opts.Aircraft; i++ {
			theta := 2*math.Pi*phase + 2*math.Pi*float64(i)/float64(opts.Aircraft)
			lat, lon := offsetLatLon(centerLat, centerLon, radiusKm, theta)
			heading := math.Mod((theta*180/math.Pi)+90, 360) // tangent direction

			t := SBSTrack{
				ICAO:        fmt.Sprintf("SY%04X", uint16(i)),
				Callsign:    fmt.Sprintf("SYNTH-%03d", i),
				Lat:         lat,
				LatSet:      true,
				Lon:         lon,
				LonSet:      true,
				AltFeet:     altitudeFt,
				AltSet:      true,
				SpdKnots:    480,
				SpdSet:      true,
				TrkDeg:      heading,
				TrkSet:      true,
				GeneratedAt: now,
				Rx:          time.Now().UTC(),
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- t:
			}
		}

		seq++
	}
}

// offsetLatLon returns the lat/lon at the given bearing (radians) and distance
// (km) from a centre point. Uses the equirectangular approximation — adequate
// for synthetic test orbits at typical demo scales.
func offsetLatLon(centerLat, centerLon, radiusKm, theta float64) (float64, float64) {
	const earthRadiusKm = 6371.0
	dy := radiusKm * math.Sin(theta) / earthRadiusKm * (180.0 / math.Pi)
	dx := radiusKm * math.Cos(theta) / earthRadiusKm * (180.0 / math.Pi) / math.Cos(centerLat*math.Pi/180.0)
	return centerLat + dy, centerLon + dx
}
