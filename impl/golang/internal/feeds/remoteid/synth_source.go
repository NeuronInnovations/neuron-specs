package remoteid

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// SynthCenter is the reference center coordinate used by RunSynth. Drones
// orbit this center on deterministic circular paths. The default is
// 51.4775 N, -0.4614 E (Heathrow approach reference per 017 spec.md
// example fixtures) so that synthetic drones overlap geographically with
// the existing ADS-B aircraft tracks the JV-box receives, making the
// fused-buyer demo visually compelling without any real DS-400.
//
// Override SynthCenter at process start if the demo runs against a
// different geographic frame.
var SynthCenter = Position{
	Lat: 51.4775,
	Lon: -0.4614,
	Alt: 100.0,
	Fix: "3D",
}

// SynthOptions controls the synthetic generator.
type SynthOptions struct {
	// FPS is the aggregate emission rate across all drones. Per-drone rate
	// is FPS / DroneCount. Must be > 0.
	FPS int

	// DroneCount is the number of synthetic drones. Each drone orbits
	// SynthCenter at a unique radius and phase derived from its index.
	// Must be > 0.
	DroneCount int

	// RadiusMeters is the orbit radius of the innermost drone. Subsequent
	// drones orbit at RadiusMeters * (1 + index*0.5). Zero is treated as
	// 200 m.
	RadiusMeters float64

	// AngularSpeedDegPerSec is the angular speed (degrees per second)
	// applied to all drones; relative phase varies per drone. Zero is
	// treated as 20 deg/s (one full orbit every 18 s for the inner drone).
	AngularSpeedDegPerSec float64

	// AltitudeMeters is the altitude reported for all drones. Zero is
	// treated as 100 m.
	AltitudeMeters float64
}

// RunSynth emits synthetic Remote ID DecodedFrames at the configured rate
// until ctx is cancelled. Each drone has:
//   - a deterministic 16-character serial DroneID derived from its index
//     (so consumers can recognize it as synthetic traffic),
//   - a circular trajectory around SynthCenter with a per-drone radius and
//     phase offset,
//   - a velocity vector consistent with the trajectory tangent.
//
// Useful for the Phase 2 fixture vertical slice when no real DS-400 access
// or recorded fixture is available, and for unit/integration tests.
func RunSynth(ctx context.Context, opts SynthOptions, out chan<- DecodedFrame) error {
	if opts.FPS <= 0 {
		return errors.New("feeds/remoteid: RunSynth FPS must be > 0")
	}
	if opts.DroneCount <= 0 {
		return errors.New("feeds/remoteid: RunSynth DroneCount must be > 0")
	}

	radius := opts.RadiusMeters
	if radius <= 0 {
		radius = 200.0
	}
	angVel := opts.AngularSpeedDegPerSec
	if angVel <= 0 {
		angVel = 20.0
	}
	alt := opts.AltitudeMeters
	if alt <= 0 {
		alt = 100.0
	}

	interval := time.Second / time.Duration(opts.FPS)
	if interval <= 0 {
		interval = time.Microsecond
	}

	startWall := time.Now().UTC()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var seq int
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		droneIdx := seq % opts.DroneCount
		droneRadius := radius * (1.0 + float64(droneIdx)*0.5)
		phaseDeg := float64(droneIdx) * (360.0 / float64(opts.DroneCount))

		elapsed := time.Since(startWall).Seconds()
		angleDeg := math.Mod(phaseDeg+angVel*elapsed, 360.0)
		angleRad := angleDeg * math.Pi / 180.0

		// Convert local (radius, angle) to lat/lon offset.
		// 1 deg latitude ≈ 111_320 m; 1 deg longitude ≈ 111_320 * cos(lat) m.
		dx := droneRadius * math.Cos(angleRad) // east-west offset, meters
		dy := droneRadius * math.Sin(angleRad) // north-south offset, meters
		dLat := dy / 111_320.0
		dLon := dx / (111_320.0 * math.Cos(SynthCenter.Lat*math.Pi/180.0))

		// Velocity vector is tangent to the circle.
		// Speed = radius * angularSpeed (rad/s).
		speed := droneRadius * angVel * math.Pi / 180.0
		// Heading (track) is the angle of the velocity vector, measured
		// clockwise from north. For counter-clockwise orbit, the tangent
		// points 90 deg ahead of the radial angle.
		trackDeg := math.Mod(angleDeg+90.0, 360.0)

		droneID := fmt.Sprintf("SYNTH-%010d", droneIdx)

		frame := DecodedFrame{
			Type:        "remote-id-frame",
			Version:     "1.0.0",
			ObservedAt:  time.Now().UTC(),
			Source:      "synth",
			DroneID:     droneID,
			DroneIDType: "serial",
			Position: &Position{
				Lat: SynthCenter.Lat + dLat,
				Lon: SynthCenter.Lon + dLon,
				Alt: alt,
				Fix: "3D",
			},
			Velocity: &Velocity{
				SpeedHorizontal: speed,
				SpeedVertical:   0.0,
				Track:           trackDeg,
			},
			RegulatorVariant: "asd-faa",
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- frame:
		}

		seq++
	}
}
