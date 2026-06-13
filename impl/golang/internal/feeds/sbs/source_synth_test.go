package sbs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBaseStationSynth_ProducesExpectedRate(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out := make(chan SBSTrack, 128)
	done := make(chan error, 1)
	go func() {
		done <- RunBaseStationSynth(ctx, SynthOptions{Aircraft: 3, Fps: 2}, out)
	}()

	deadline := time.After(1500 * time.Millisecond)
	var got []SBSTrack
loop:
	for {
		select {
		case t := <-out:
			got = append(got, t)
		case <-deadline:
			break loop
		}
	}
	cancel()
	<-done

	// 3 aircraft × 2 fps × ~1 second window = ~6 records minimum (with some
	// slack for tick alignment).
	assert.GreaterOrEqual(t, len(got), 5, "expected >=5 records in ~1s")

	// All synthetic ICAOs start with "SY".
	for _, tk := range got {
		assert.Equal(t, "SY", tk.ICAO[:2], "synthetic ICAOs have SY prefix")
		assert.True(t, tk.LatSet)
		assert.True(t, tk.LonSet)
		assert.True(t, tk.AltSet)
	}
}

func TestRunBaseStationSynth_DistinctAircraft(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	out := make(chan SBSTrack, 64)
	go func() {
		_ = RunBaseStationSynth(ctx, SynthOptions{Aircraft: 5, Fps: 10}, out)
	}()

	icaos := make(map[string]struct{})
	deadline := time.After(700 * time.Millisecond)
	for {
		select {
		case tk := <-out:
			icaos[tk.ICAO] = struct{}{}
			if len(icaos) >= 5 {
				cancel()
				assert.Equal(t, 5, len(icaos), "five distinct ICAOs in 5-aircraft synth")
				return
			}
		case <-deadline:
			t.Fatalf("only saw %d distinct ICAOs; expected 5", len(icaos))
		}
	}
}

func TestRunBaseStationSynth_Defaults(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	out := make(chan SBSTrack, 16)
	go func() {
		_ = RunBaseStationSynth(ctx, SynthOptions{Aircraft: 1, Fps: 5}, out)
	}()

	select {
	case tk := <-out:
		// Default centre is Heathrow (51.4775, -0.4614); radius 10km; altitude 37000 ft.
		assert.InDelta(t, 51.4775, tk.Lat, 0.2, "aircraft within ~22km of Heathrow centre")
		assert.InDelta(t, -0.4614, tk.Lon, 0.2)
		assert.InDelta(t, 37000.0, tk.AltFeet, 1e-9)
	case <-ctx.Done():
		t.Fatal("no synth record produced within timeout")
	}
}

func TestRunBaseStationSynth_Validation(t *testing.T) {
	t.Parallel()

	out := make(chan SBSTrack, 1)
	require.Error(t, RunBaseStationSynth(context.Background(), SynthOptions{Aircraft: 0, Fps: 1}, out))
	require.Error(t, RunBaseStationSynth(context.Background(), SynthOptions{Aircraft: 1, Fps: 0}, out))
}

func TestRunBaseStationSynth_ExitsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan SBSTrack, 16)
	done := make(chan error, 1)
	go func() {
		done <- RunBaseStationSynth(ctx, SynthOptions{Aircraft: 2, Fps: 10}, out)
	}()

	// Drain a few records, then cancel.
	for i := 0; i < 3; i++ {
		select {
		case <-out:
		case <-time.After(500 * time.Millisecond):
			cancel()
			<-done
			t.Fatal("synth did not produce records")
		}
	}
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(1 * time.Second):
		t.Fatal("synth did not exit on ctx cancel")
	}
}
