package sbs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBaseStationReplay_Vanilla(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	path := filepath.Join("testdata", "vanilla-jv.sbs")
	out := make(chan SBSTrack, 16)
	done := make(chan error, 1)
	go func() {
		// Speedup very high so the fixture flushes quickly (the natural
		// inter-record delta is ~100-300ms; Speedup=1000 collapses that to
		// ~0.1-0.3ms).
		done <- RunBaseStationReplay(ctx, path, ReplayOptions{Speedup: 1000}, out)
	}()

	var got []SBSTrack
	// Wait for goroutine to finish, draining out along the way.
	finishing := false
	for {
		select {
		case tk := <-out:
			got = append(got, tk)
		case err := <-done:
			require.NoError(t, err)
			finishing = true
		case <-ctx.Done():
			t.Fatal("timed out waiting for replay")
		}
		if finishing {
			// Drain any remaining buffered records non-blocking.
			for {
				select {
				case tk := <-out:
					got = append(got, tk)
				default:
					goto check
				}
			}
		}
	}
check:
	assert.Len(t, got, 7, "fixture contains 6 type-3 + 1 type-4 records (other types dropped)")
}

func TestRunBaseStationReplay_FrameInterval(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	path := filepath.Join("testdata", "vanilla-jv.sbs")
	out := make(chan SBSTrack, 16)
	done := make(chan error, 1)

	start := time.Now()
	go func() {
		done <- RunBaseStationReplay(ctx, path, ReplayOptions{FrameInterval: 50 * time.Millisecond}, out)
	}()

	var got []SBSTrack
loop:
	for {
		select {
		case t := <-out:
			got = append(got, t)
		case err := <-done:
			require.NoError(t, err)
			break loop
		case <-ctx.Done():
			t.Fatal("timed out")
		}
	}
	elapsed := time.Since(start)

	require.Len(t, got, 7)
	// 7 records × 50ms minimum = 350ms; the FIRST record also paces (no
	// previous to compute delta from), but we conservatively allow some
	// margin. Ceiling: well within the 3s test timeout.
	assert.GreaterOrEqual(t, elapsed, 250*time.Millisecond, "FrameInterval pacing should produce measurable delay")
	assert.Less(t, elapsed, 2*time.Second, "should still complete well before timeout")
}

func TestRunBaseStationReplay_MissingPath(t *testing.T) {
	t.Parallel()
	out := make(chan SBSTrack, 1)
	err := RunBaseStationReplay(context.Background(), "", ReplayOptions{}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path")
}

func TestRunBaseStationReplay_BadPath(t *testing.T) {
	t.Parallel()
	out := make(chan SBSTrack, 1)
	err := RunBaseStationReplay(context.Background(), "/does/not/exist.sbs", ReplayOptions{}, out)
	require.Error(t, err)
}

func TestRunBaseStationReplay_Loop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := filepath.Join("testdata", "vanilla-jv.sbs")
	out := make(chan SBSTrack, 64)
	go func() {
		_ = RunBaseStationReplay(ctx, path, ReplayOptions{Speedup: 10000, Loop: true}, out)
	}()

	// Read at least 12 records (more than one full pass over the 7-record fixture).
	deadline := time.After(2 * time.Second)
	var got []SBSTrack
	for len(got) < 12 {
		select {
		case t := <-out:
			got = append(got, t)
		case <-deadline:
			t.Fatalf("only got %d records; expected >= 12 across two loop passes", len(got))
		}
	}
	cancel()
	assert.GreaterOrEqual(t, len(got), 12)
}
