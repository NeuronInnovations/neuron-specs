package feeds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// RunBeastReplay reads a captured Beast byte stream from path, decodes it,
// and emits FeedFrames on out. The captured byte stream is a raw recording
// of what a Beast TCP source would have emitted (e.g. the bytes of a single
// `nc 127.0.0.1 10003 > capture.bin` invocation).
//
// Behavior:
//   - opts.Speedup multiplies the natural emit rate. <=0 is treated as 1.0.
//   - opts.FrameInterval (when non-zero) overrides the natural cadence and
//     paces emission at one frame per interval. Useful when the capture has
//     no usable embedded timing.
//   - opts.Loop, when true, restarts the file from the beginning on EOF.
//
// When neither Speedup nor FrameInterval is set, the function emits frames
// as fast as the consumer can drain them — fine for tests, undesirable for
// long-running demos.
func RunBeastReplay(ctx context.Context, path string, opts ReplayOptions, out chan<- FeedFrame) error {
	if path == "" {
		return errors.New("feeds: RunBeastReplay requires a path")
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
			return fmt.Errorf("feeds: open replay file: %w", err)
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

// replayOnce decodes the file once, applying pacing.
func replayOnce(ctx context.Context, r io.Reader, opts ReplayOptions, speedup float64, out chan<- FeedFrame) error {
	dec := NewDecoder(r)
	var raw RawFrame
	var prevSec, prevNs uint64
	var havePrev bool

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if errors.Is(err, ErrInvalidFrame) {
				continue
			}
			return err
		}
		if raw.Type != BeastTypeModeSShort && raw.Type != BeastTypeModeSLong {
			continue
		}

		sec, ns, gpsOK := TimestampGPS(raw.Timestamp)

		// Pace.
		switch {
		case opts.FrameInterval > 0:
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(opts.FrameInterval):
			}
		case gpsOK && havePrev:
			delta := timestampDelta(prevSec, prevNs, sec, ns)
			if speedup > 0 && delta > 0 {
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

		ff := FeedFrame{
			Raw:                  append([]byte(nil), raw.Payload...),
			SecondsSinceMidnight: sec,
			Nanoseconds:          ns,
			Rx:                   time.Now().UTC(),
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- ff:
		}

		if gpsOK {
			prevSec, prevNs = sec, ns
			havePrev = true
		}
	}
}

// timestampDelta returns the duration between two GPS-encoded timestamps
// (positive when t2 > t1, including the midnight wrap case).
func timestampDelta(s1, n1, s2, n2 uint64) time.Duration {
	cur := time.Duration(s2)*time.Second + time.Duration(n2)*time.Nanosecond
	prev := time.Duration(s1)*time.Second + time.Duration(n1)*time.Nanosecond
	d := cur - prev
	if d < 0 {
		// Wrapped past midnight (s2 < s1). Add a full day.
		d += 24 * time.Hour
	}
	// Cap implausible jumps at 1 s — replay files with damaged timestamps
	// shouldn't stall the source for hours.
	if d > time.Second {
		d = time.Second
	}
	return d
}
