package sbs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// RunBaseStationReplay reads an SBS-1 capture file from path line by line,
// parses each line, and emits SBSTrack values on out at a pace governed by
// opts (Speedup, FrameInterval, Loop).
//
// Behaviour:
//   - opts.Speedup > 0 + records have parseable GeneratedAt timestamps: pace
//     by the inter-record delta, divided by Speedup.
//   - opts.FrameInterval > 0 (overrides Speedup pacing): emit one record per
//     interval regardless of embedded timestamps.
//   - Neither set: emit as fast as the consumer drains. Fine for tests.
//   - opts.Loop: restart from line 1 on EOF.
//
// On unrecoverable file I/O error (path doesn't exist, permission denied,
// disk read error), returns an error wrapping the underlying cause.
// Individual line parse failures are silently dropped (same convention as
// RunBaseStationTCP).
//
// Does NOT close out.
func RunBaseStationReplay(ctx context.Context, path string, opts ReplayOptions, out chan<- SBSTrack) error {
	if path == "" {
		return errors.New("sbs: RunBaseStationReplay requires a path")
	}

	speedup := opts.Speedup
	if speedup <= 0 {
		speedup = 1.0
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("sbs: open replay file: %w", err)
		}

		err = replayOnce(ctx, f, opts, speedup, out)
		_ = f.Close()

		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		if !opts.Loop {
			return nil
		}
	}
}

func replayOnce(ctx context.Context, r io.Reader, opts ReplayOptions, speedup float64, out chan<- SBSTrack) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 4096), 64*1024)

	var prevGenerated time.Time
	var havePrev bool

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		t, err := ParseMSG(line)
		if err != nil {
			continue
		}

		// Pace.
		switch {
		case opts.FrameInterval > 0:
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(opts.FrameInterval):
			}
		case havePrev && !t.GeneratedAt.IsZero() && !prevGenerated.IsZero():
			delta := t.GeneratedAt.Sub(prevGenerated)
			if delta > 0 {
				// Cap implausible deltas at 5s to avoid stalling on damaged
				// timestamps in long capture files.
				if delta > 5*time.Second {
					delta = 5 * time.Second
				}
				delay := time.Duration(float64(delta) / speedup)
				if delay > 0 {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(delay):
					}
				}
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- *t:
		}

		if !t.GeneratedAt.IsZero() {
			prevGenerated = t.GeneratedAt
			havePrev = true
		}
	}
	return scanner.Err()
}
