package feeds

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSynth_EmitsAtRequestedRate(t *testing.T) {
	out := make(chan FeedFrame, 64)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- RunSynth(ctx, 50, out) }()

	deadline := time.After(500 * time.Millisecond)
	var count int
loop:
	for {
		select {
		case f := <-out:
			assert.Len(t, f.Raw, 7)
			assert.Equal(t, byte(0x00), f.Raw[0]) // DF=0
			count++
		case <-deadline:
			break loop
		}
	}
	cancel()
	<-done

	// At 50 fps over ~500ms we expect ~25 frames; allow a generous lower bound
	// for slow CI machines.
	assert.GreaterOrEqual(t, count, 10, "expected at least 10 synth frames in 500ms")
}

func TestRunSynth_StopsOnContextCancel(t *testing.T) {
	out := make(chan FeedFrame, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunSynth(ctx, 100, out) }()

	// Drain so the producer doesn't block.
	go func() {
		for range out {
		}
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("RunSynth did not return after cancel")
	}
}

func TestRunBeastReplay_FromFile(t *testing.T) {
	// Build a 50-frame capture file.
	var buf bytes.Buffer
	for i := 0; i < 50; i++ {
		buf.Write(EncodeBeastFrame(BeastTypeModeSShort,
			EncodeGPSTimestamp(uint64(i/10), uint64((i%10)*100_000_000)), 0xAA,
			[]byte{byte(i), 0, 0, 0, 0, 0, 0}))
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "capture.beast")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))

	out := make(chan FeedFrame, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// FrameInterval=0, Speedup=large => emit as fast as possible.
	require.NoError(t, RunBeastReplay(ctx, path,
		ReplayOptions{Speedup: 1_000_000, Loop: false}, out))

	close(out)
	var frames []FeedFrame
	for f := range out {
		frames = append(frames, f)
	}
	assert.Len(t, frames, 50)
	for i, f := range frames {
		assert.Equal(t, byte(i), f.Raw[0], "frame %d sequence mismatch", i)
	}
}

func TestRunBeastReplay_LoopsWhenRequested(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 5; i++ {
		buf.Write(EncodeBeastFrame(BeastTypeModeSShort,
			[6]byte{byte(i), 0, 0, 0, 0, 0}, 0,
			[]byte{byte(i), 0, 0, 0, 0, 0, 0}))
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "loop.beast")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))

	out := make(chan FeedFrame, 64)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- RunBeastReplay(ctx, path,
			ReplayOptions{Speedup: 1_000_000, Loop: true}, out)
	}()

	// Collect 12 frames (would require >2 loops over the 5-frame file).
	var got []FeedFrame
	deadline := time.After(2 * time.Second)
	for len(got) < 12 {
		select {
		case f := <-out:
			got = append(got, f)
		case <-deadline:
			t.Fatalf("only got %d frames before deadline; expected 12", len(got))
		}
	}
	cancel()
	<-done

	assert.GreaterOrEqual(t, len(got), 12)
}

func TestRunBeastTCP_DialFailureRetries(t *testing.T) {
	out := make(chan FeedFrame, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Use a port that's almost certainly closed.
	err := RunBeastTCP(ctx, "127.0.0.1:1", out)
	// We expect ctx deadline / canceled, not the underlying dial error.
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
		"unexpected error: %v", err)
}

func TestRunBeastTCP_ReadsFromMockListener(t *testing.T) {
	// Start a mock TCP server that emits 10 short frames then closes.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		for i := 0; i < 10; i++ {
			frame := EncodeBeastFrame(BeastTypeModeSShort,
				EncodeGPSTimestamp(uint64(i), 0), 0,
				[]byte{byte(i), 0, 0, 0, 0, 0, 0})
			if _, err := conn.Write(frame); err != nil {
				return
			}
		}
	}()

	out := make(chan FeedFrame, 16)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- RunBeastTCP(ctx, ln.Addr().String(), out) }()

	var got []FeedFrame
	deadline := time.After(2 * time.Second)
	for len(got) < 10 {
		select {
		case f := <-out:
			got = append(got, f)
		case <-deadline:
			t.Fatalf("only received %d frames", len(got))
		}
	}
	cancel()
	<-done
	assert.GreaterOrEqual(t, len(got), 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, byte(i), got[i].Raw[0])
	}
}
