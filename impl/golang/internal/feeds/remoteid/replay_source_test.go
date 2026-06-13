package remoteid

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureTwoDrones = "testdata/two-drones.json"

func TestRunReplay_EmitsAllFixtureEntries(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 16)
	errCh := make(chan error, 1)

	go func() {
		errCh <- RunReplay(ctx, fixtureTwoDrones, ReplayOptions{FrameInterval: time.Millisecond}, out)
	}()

	var frames []DecodedFrame
collect:
	for {
		select {
		case f := <-out:
			frames = append(frames, f)
			if len(frames) == 4 {
				break collect
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for frames; got %d", len(frames))
		}
	}

	require.NoError(t, <-errCh)

	require.Len(t, frames, 4)
	assert.Equal(t, "remote-id-frame", frames[0].Type)
	assert.Equal(t, "MFR1234567890ABC", frames[0].DroneID)
	assert.Equal(t, "MFR2222333344445", frames[2].DroneID)
	assert.Equal(t, "asd-easa", frames[2].RegulatorVariant)
	require.NotNil(t, frames[2].Operator)
	assert.Equal(t, "OP-GB-001", frames[2].Operator.ID)
	assert.Equal(t, "caa", frames[2].Operator.IDType)
}

func TestRunReplay_RespectsPacing(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 16)
	go func() {
		_ = RunReplay(ctx, fixtureTwoDrones, ReplayOptions{Speedup: 50.0}, out)
	}()

	start := time.Now()
	// Total fixture span is 2000ms; at speedup 50, expect ~40ms total.
	expectedMax := 500 * time.Millisecond

	var count int
	for count < 4 {
		select {
		case <-out:
			count++
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for frames; got %d", count)
		}
	}
	elapsed := time.Since(start)
	assert.LessOrEqual(t, elapsed, expectedMax, "replay too slow for speedup=50")
}

func TestRunReplay_LoopRestartsFromBeginning(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 32)
	go func() {
		_ = RunReplay(ctx, fixtureTwoDrones, ReplayOptions{
			FrameInterval: time.Millisecond,
			Loop:          true,
		}, out)
	}()

	// 4 entries × 3 loops = 12 frames; the loop should keep emitting.
	var frames []DecodedFrame
	for len(frames) < 12 {
		select {
		case f := <-out:
			frames = append(frames, f)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for looped frames; got %d", len(frames))
		}
	}

	require.Len(t, frames, 12)
	// First frame of loop 1 and loop 2 should have the same DroneID (the
	// loop restarts from the first fixture entry).
	assert.Equal(t, frames[0].DroneID, frames[4].DroneID)
	assert.Equal(t, frames[0].DroneID, frames[8].DroneID)
}

func TestRunReplay_ErrorsOnMissingFile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 1)
	err := RunReplay(ctx, filepath.Join("testdata", "does-not-exist.json"), ReplayOptions{}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feeds/remoteid: open fixture")
}

func TestRunReplay_ErrorsOnEmptyPath(t *testing.T) {
	t.Parallel()

	out := make(chan DecodedFrame, 1)
	err := RunReplay(context.Background(), "", ReplayOptions{}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a path")
}

func TestRunReplay_ObservedAtSynthesizedWhenAbsent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	before := time.Now().UTC()

	out := make(chan DecodedFrame, 4)
	go func() {
		_ = RunReplay(ctx, fixtureTwoDrones, ReplayOptions{FrameInterval: time.Millisecond}, out)
	}()

	var first DecodedFrame
	select {
	case first = <-out:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for first frame")
	}

	after := time.Now().UTC()
	// observedAt was absent from the fixture; replay should have synthesized
	// it within [before, after].
	assert.False(t, first.ObservedAt.Before(before), "observedAt %v before %v", first.ObservedAt, before)
	assert.False(t, first.ObservedAt.After(after), "observedAt %v after %v", first.ObservedAt, after)
}
