package remoteid

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSynth_EmitsExpectedShape(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	out := make(chan DecodedFrame, 64)
	go func() {
		_ = RunSynth(ctx, SynthOptions{FPS: 50, DroneCount: 2}, out)
	}()

	var frames []DecodedFrame
	timeout := time.After(400 * time.Millisecond)
collect:
	for {
		select {
		case f := <-out:
			frames = append(frames, f)
		case <-timeout:
			break collect
		}
	}

	require.GreaterOrEqual(t, len(frames), 10, "expected at least 10 frames at FPS=50 in 400ms")

	first := frames[0]
	assert.Equal(t, "remote-id-frame", first.Type)
	assert.Equal(t, "1.0.0", first.Version)
	assert.Equal(t, "synth", first.Source)
	assert.True(t, strings.HasPrefix(first.DroneID, "SYNTH-"), "expected SYNTH-prefixed drone id, got %q", first.DroneID)
	assert.Equal(t, "serial", first.DroneIDType)
	require.NotNil(t, first.Position)
	assert.InDelta(t, SynthCenter.Lat, first.Position.Lat, 0.01)
	assert.InDelta(t, SynthCenter.Lon, first.Position.Lon, 0.01)
	require.NotNil(t, first.Velocity)
	assert.Greater(t, first.Velocity.SpeedHorizontal, 0.0)
}

func TestRunSynth_RejectsZeroFPS(t *testing.T) {
	t.Parallel()

	out := make(chan DecodedFrame, 1)
	err := RunSynth(context.Background(), SynthOptions{FPS: 0, DroneCount: 1}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FPS must be > 0")
}

func TestRunSynth_RejectsZeroDroneCount(t *testing.T) {
	t.Parallel()

	out := make(chan DecodedFrame, 1)
	err := RunSynth(context.Background(), SynthOptions{FPS: 10, DroneCount: 0}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DroneCount must be > 0")
}

func TestRunSynth_AlternatesDronesInRoundRobin(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	out := make(chan DecodedFrame, 32)
	go func() {
		_ = RunSynth(ctx, SynthOptions{FPS: 50, DroneCount: 3}, out)
	}()

	// Read 12 frames; with DroneCount=3 we expect each unique DroneID at
	// least 3 times in round-robin (0, 1, 2, 0, 1, 2, ...).
	frames := make([]DecodedFrame, 0, 12)
	timeout := time.After(400 * time.Millisecond)
collect:
	for len(frames) < 12 {
		select {
		case f := <-out:
			frames = append(frames, f)
		case <-timeout:
			break collect
		}
	}

	require.GreaterOrEqual(t, len(frames), 12)
	assert.Equal(t, "SYNTH-0000000000", frames[0].DroneID)
	assert.Equal(t, "SYNTH-0000000001", frames[1].DroneID)
	assert.Equal(t, "SYNTH-0000000002", frames[2].DroneID)
	assert.Equal(t, "SYNTH-0000000000", frames[3].DroneID)
}
