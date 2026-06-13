package feeds

import (
	"context"
	"encoding/binary"
	"errors"
	"time"
)

// RunSynth emits synthetic Mode-S short frames at fps frames per second
// until ctx is cancelled. Each frame carries:
//   - a strictly-monotonic GPS timestamp anchored at the wall-clock UTC time
//     of the call (subsequent frames advance by 1/fps),
//   - a synthetic Mode-S DF=0 short payload that embeds a 24-bit ICAO of
//     0xFFFE00 + (sequence & 0xFF), so consumers can recognize it as test
//     traffic and verify monotonicity.
//
// Useful for unit tests, mock-bus E2E tests, and offline development.
func RunSynth(ctx context.Context, fps int, out chan<- FeedFrame) error {
	if fps <= 0 {
		return errors.New("feeds: RunSynth fps must be > 0")
	}

	interval := time.Second / time.Duration(fps)
	if interval <= 0 {
		interval = time.Microsecond
	}

	startWall := time.Now().UTC()
	startSec := uint64(startWall.Hour()*3600 + startWall.Minute()*60 + startWall.Second())
	startNs := uint64(startWall.Nanosecond())

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var seq uint64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		offset := time.Duration(seq) * interval
		secOff := uint64(offset / time.Second)
		nsOff := uint64(offset % time.Second)

		sec := (startSec + secOff) % 86_400
		ns := startNs + nsOff
		if ns >= 1_000_000_000 {
			sec = (sec + 1) % 86_400
			ns -= 1_000_000_000
		}

		// Synthetic Mode-S short payload (7 bytes): DF byte + 3 reserved + ICAO(3).
		var payload [7]byte
		payload[0] = 0x00 // DF=0 short air-air surveillance
		binary.BigEndian.PutUint32(payload[3:], uint32(0xFFFE0000|uint32(seq&0xFF)))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- FeedFrame{
			Raw:                  append([]byte(nil), payload[:]...),
			SecondsSinceMidnight: sec,
			Nanoseconds:          ns,
			Rx:                   time.Now().UTC(),
		}:
		}

		seq++
	}
}
