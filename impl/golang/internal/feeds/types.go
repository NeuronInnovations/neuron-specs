package feeds

import "time"

// FeedFrame is a single decoded record from a feed source.
//
// Raw is the Mode-S payload only (typically 7 bytes for short messages, 14
// bytes for long messages). It is exactly what libp2p will forward to the
// buyer side; receivers can decode it with any Mode-S library.
//
// SecondsSinceMidnight and Nanoseconds carry the receiver's GPS-synchronized
// timestamp. JetVision Air!Squitter and other GPS-locked Beast receivers
// encode these in the upper 18 / lower 30 bits of the 48-bit MLAT counter.
// When the receiver is not GPS-locked, both fields are zero (callers can fall
// back to Rx wall-clock time).
//
// Rx is the wall-clock instant the SDK observed the frame. It is always
// populated regardless of GPS availability.
type FeedFrame struct {
	Raw                  []byte
	SecondsSinceMidnight uint64
	Nanoseconds          uint64
	Rx                   time.Time
}

// ReplayOptions tunes file-replay behavior for RunBeastReplay.
type ReplayOptions struct {
	// Speedup multiplies the natural emit rate. 1.0 = real-time playback.
	// Values <= 0 are treated as 1.0. Very large values flush the file as
	// fast as the consumer can drain it (useful for tests).
	Speedup float64

	// FrameInterval is the delay between consecutive frames when the source
	// has no embedded timing. Zero means "go as fast as possible". This is
	// used by replay sources that don't model receiver wall-clock cadence.
	FrameInterval time.Duration

	// Loop, when true, restarts the file from the beginning after EOF.
	// When false (default), Run returns nil on EOF.
	Loop bool
}
