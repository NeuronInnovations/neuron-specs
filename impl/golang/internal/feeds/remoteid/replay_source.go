package remoteid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

// ReplayOptions controls replay pacing and looping.
type ReplayOptions struct {
	// Speedup multiplies the natural inter-frame delay (derived from each
	// fixture entry's offsetMs relative to the first entry). <=0 is treated
	// as 1.0. A speedup of 2.0 emits twice as fast as the fixture timing
	// suggests; a speedup of 0.5 emits half as fast.
	Speedup float64

	// FrameInterval, when non-zero, overrides the per-entry offsetMs pacing
	// and emits one frame per interval. Useful when the fixture has no
	// useful timing or the operator wants a uniform demo cadence.
	FrameInterval time.Duration

	// Loop, when true, restarts the fixture from the beginning on EOF.
	// Each loop re-emits the same entries with timestamps re-anchored to
	// the current wall-clock; consumers that key on ObservedAt MUST tolerate
	// duplicate observed-at across loops.
	Loop bool
}

// RunReplay reads a JSON fixture file at path, decodes each entry into a
// DecodedFrame, and emits each frame at the appropriate cadence on out
// until ctx is cancelled, the fixture is exhausted (Loop=false), or an
// error occurs.
//
// The fixture is a JSON array of objects with the following shape (all
// fields except offsetMs map to DecodedFrame fields with the same names
// using camelCase JSON keys):
//
//	[
//	  {
//	    "offsetMs": 0,
//	    "type": "remote-id-frame",
//	    "version": "1.0.0",
//	    "source": "dronescout-ds400",
//	    "droneId": "MFR1234567890ABC",
//	    "droneIdType": "serial",
//	    "position": {"lat": 51.4775, "lon": -0.4614, "alt": 100.0, "fix": "3D"},
//	    "velocity": {"speedHorizontal": 25.0, "speedVertical": 0.0, "track": 90.0},
//	    "regulatorVariant": "asd-faa"
//	  },
//	  ...
//	]
//
// offsetMs is required and gives the entry's emit time in milliseconds
// relative to the FIRST entry's offsetMs (which is typically 0). Entries
// are sorted by offsetMs before emission; out-of-order fixture authoring
// is tolerated.
//
// If an entry omits "observedAt", the replay synthesizes one from the
// wall-clock at emission time. If "observedAt" is present (RFC3339Nano
// string), it is preserved verbatim — useful when fixtures were captured
// from a real source and the operator wants to test downstream timestamp
// handling.
//
// Errors are wrapped with the prefix "feeds/remoteid: ".
func RunReplay(ctx context.Context, path string, opts ReplayOptions, out chan<- DecodedFrame) error {
	if path == "" {
		return errors.New("feeds/remoteid: RunReplay requires a path")
	}

	entries, err := loadFixture(path)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New("feeds/remoteid: replay fixture is empty")
	}

	speedup := opts.Speedup
	if speedup <= 0 {
		speedup = 1.0
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := replayOnce(ctx, entries, opts, speedup, out); err != nil {
			return err
		}

		if !opts.Loop {
			return nil
		}
	}
}

// replayOnce emits each entry exactly once, applying the configured pacing.
func replayOnce(ctx context.Context, entries []fixtureEntry, opts ReplayOptions, speedup float64, out chan<- DecodedFrame) error {
	start := time.Now().UTC()
	baseOffsetMs := entries[0].OffsetMs

	for i, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Pace.
		if i > 0 {
			var delay time.Duration
			switch {
			case opts.FrameInterval > 0:
				delay = opts.FrameInterval
			default:
				naturalDelta := max(time.Duration(entry.OffsetMs-entries[i-1].OffsetMs)*time.Millisecond, 0)
				delay = time.Duration(float64(naturalDelta) / speedup)
			}
			if delay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}
		}

		frame, err := entry.toDecodedFrame(start, baseOffsetMs)
		if err != nil {
			return fmt.Errorf("feeds/remoteid: fixture entry %d: %w", i, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- frame:
		}
	}

	return nil
}

// loadFixture reads + parses + sorts a fixture file.
func loadFixture(path string) ([]fixtureEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("feeds/remoteid: open fixture: %w", err)
	}
	var entries []fixtureEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("feeds/remoteid: parse fixture: %w", err)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].OffsetMs < entries[j].OffsetMs
	})
	return entries, nil
}

// fixtureEntry mirrors the on-disk JSON shape. Field tags use camelCase to
// match the convention in 017 FR-R05.
type fixtureEntry struct {
	OffsetMs         int64               `json:"offsetMs"`
	Type             string              `json:"type"`
	Version          string              `json:"version"`
	ObservedAt       *string             `json:"observedAt,omitempty"`
	Source           string              `json:"source"`
	DroneID          string              `json:"droneId"`
	DroneIDType      string              `json:"droneIdType"`
	Position         *fixturePosition    `json:"position,omitempty"`
	Velocity         *fixtureVelocity    `json:"velocity,omitempty"`
	Operator         *fixtureOperator    `json:"operator,omitempty"`
	RegulatorVariant string              `json:"regulatorVariant,omitempty"`
}

type fixturePosition struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	Alt float64 `json:"alt"`
	Fix string  `json:"fix"`
}

type fixtureVelocity struct {
	SpeedHorizontal float64 `json:"speedHorizontal"`
	SpeedVertical   float64 `json:"speedVertical"`
	Track           float64 `json:"track"`
}

type fixtureOperator struct {
	IDType   string           `json:"idType"`
	ID       string           `json:"id"`
	Position *fixturePosition `json:"position,omitempty"`
}

// toDecodedFrame converts a fixtureEntry into a DecodedFrame.
// If the entry omits "observedAt", the replay synthesizes a timestamp at
// (start + (entry.OffsetMs - baseOffsetMs) milliseconds).
func (e fixtureEntry) toDecodedFrame(start time.Time, baseOffsetMs int64) (DecodedFrame, error) {
	frame := DecodedFrame{
		Type:             e.Type,
		Version:          e.Version,
		Source:           e.Source,
		DroneID:          e.DroneID,
		DroneIDType:      e.DroneIDType,
		RegulatorVariant: e.RegulatorVariant,
	}

	if frame.Type == "" {
		frame.Type = "remote-id-frame"
	}
	if frame.Version == "" {
		frame.Version = "1.0.0"
	}

	if e.ObservedAt != nil && *e.ObservedAt != "" {
		t, err := time.Parse(time.RFC3339Nano, *e.ObservedAt)
		if err != nil {
			return DecodedFrame{}, fmt.Errorf("parse observedAt %q: %w", *e.ObservedAt, err)
		}
		frame.ObservedAt = t.UTC()
	} else {
		offset := time.Duration(e.OffsetMs-baseOffsetMs) * time.Millisecond
		frame.ObservedAt = start.Add(offset).UTC()
	}

	if e.Position != nil {
		frame.Position = &Position{
			Lat: e.Position.Lat,
			Lon: e.Position.Lon,
			Alt: e.Position.Alt,
			Fix: e.Position.Fix,
		}
	}
	if e.Velocity != nil {
		frame.Velocity = &Velocity{
			SpeedHorizontal: e.Velocity.SpeedHorizontal,
			SpeedVertical:   e.Velocity.SpeedVertical,
			Track:           e.Velocity.Track,
		}
	}
	if e.Operator != nil {
		op := &Operator{
			IDType: e.Operator.IDType,
			ID:     e.Operator.ID,
		}
		if e.Operator.Position != nil {
			op.Position = &Position{
				Lat: e.Operator.Position.Lat,
				Lon: e.Operator.Position.Lon,
				Alt: e.Operator.Position.Alt,
				Fix: e.Operator.Position.Fix,
			}
		}
		frame.Operator = op
	}

	return frame, nil
}
