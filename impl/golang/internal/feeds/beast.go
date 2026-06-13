package feeds

import (
	"bufio"
	"context"
	"errors"
	"io"
	"time"
)

// Beast frame type bytes.
//
// References:
//   - https://wiki.jetvision.de/wiki/Mode-S_Beast:Beast_Binary_Protocol
//   - dump1090 / readsb source: net_io.c readBeastMessage
const (
	BeastTypeModeAC     byte = 0x31 // Mode A/C — 2-byte body
	BeastTypeModeSShort byte = 0x32 // Mode S short — 7-byte body
	BeastTypeModeSLong  byte = 0x33 // Mode S long — 14-byte body
	BeastTypeStatus     byte = 0x34 // Air!Squitter status — 14-byte body
	beastEscape         byte = 0x1A
)

// beastBodyLen is timestamp(6) + signal(1) + payload(N) per type.
var beastBodyLen = map[byte]int{
	BeastTypeModeAC:     6 + 1 + 2,
	BeastTypeModeSShort: 6 + 1 + 7,
	BeastTypeModeSLong:  6 + 1 + 14,
	BeastTypeStatus:     6 + 1 + 14,
}

// ErrInvalidFrame is returned when a Beast frame fails framing checks.
// Callers can resync by calling Decode again — the decoder's sync loop will
// scan forward to the next escape+type pair.
var ErrInvalidFrame = errors.New("feeds: invalid beast frame")

// errUnexpectedFrame is the internal recoverable variant of ErrInvalidFrame
// raised when readUnescaped hits a stray 0x1A mid-body. The decoder uses it
// to set havePrefix=true so the next Decode picks up correctly.
var errUnexpectedFrame = errors.New("feeds: unexpected beast frame boundary")

// RawFrame is a parsed Beast frame.
type RawFrame struct {
	Type      byte
	Timestamp [6]byte
	Signal    byte
	Payload   []byte
}

// Decoder decodes a stream of Beast frames from an io.Reader.
//
// Decoder is not safe for concurrent use.
type Decoder struct {
	br         *bufio.Reader
	havePrefix bool // true ⇒ a 0x1A that begins the next frame was already consumed
}

// NewDecoder wraps r in a Beast decoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{br: bufio.NewReader(r)}
}

// Decode reads the next Beast frame into out. The Payload slice is reused
// across calls; copy it if the caller needs to retain the bytes.
//
// Returns io.EOF when the underlying reader is exhausted.
func (d *Decoder) Decode(out *RawFrame) error {
	if out == nil {
		return errors.New("feeds: Decode called with nil RawFrame")
	}

	for {
		// Find escape byte.
		if !d.havePrefix {
			b, err := d.br.ReadByte()
			if err != nil {
				return err
			}
			if b != beastEscape {
				continue
			}
		}
		d.havePrefix = false

		// Peek at the byte after escape to decide what to do.
		peek, err := d.br.Peek(1)
		if err != nil {
			return err
		}
		next := peek[0]

		// Stray escape pair outside a body — consume and keep scanning.
		if next == beastEscape {
			_, _ = d.br.ReadByte()
			continue
		}

		// Not a recognized frame type — drop the leading escape, keep scanning.
		bodyLen, known := beastBodyLen[next]
		if !known {
			continue
		}

		// Consume the type byte and read the body.
		t, _ := d.br.ReadByte()
		body := make([]byte, bodyLen)
		if err := d.readUnescaped(body); err != nil {
			if errors.Is(err, errUnexpectedFrame) {
				// Recoverable: caller can call Decode again to resync.
				return ErrInvalidFrame
			}
			return err
		}

		out.Type = t
		copy(out.Timestamp[:], body[:6])
		out.Signal = body[6]
		out.Payload = append(out.Payload[:0], body[7:]...)
		return nil
	}
}

// readUnescaped fills buf with un-stuffed body bytes (0x1A 0x1A → 0x1A).
// On a stray 0x1A (followed by anything other than 0x1A) it sets havePrefix
// and returns errUnexpectedFrame so Decode can flag the partial frame and
// the next call resyncs cleanly.
func (d *Decoder) readUnescaped(buf []byte) error {
	for i := range buf {
		b, err := d.br.ReadByte()
		if err != nil {
			return err
		}
		if b == beastEscape {
			peek, err := d.br.Peek(1)
			if err != nil {
				return err
			}
			if peek[0] == beastEscape {
				_, _ = d.br.ReadByte()
				buf[i] = beastEscape
				continue
			}
			// Start of a new frame in the middle of this body. The 0x1A is
			// already consumed; mark the next call to skip its leading sync.
			d.havePrefix = true
			return errUnexpectedFrame
		}
		buf[i] = b
	}
	return nil
}

// TimestampGPS interprets a 48-bit Beast timestamp as Air!Squitter
// GPS-synchronized form: top 18 bits = seconds since UTC midnight, bottom
// 30 bits = nanoseconds. Returns (0, 0, false) when the value cannot be a
// valid GPS timestamp.
func TimestampGPS(t [6]byte) (sec uint64, ns uint64, ok bool) {
	raw := uint64(t[0])<<40 | uint64(t[1])<<32 | uint64(t[2])<<24 |
		uint64(t[3])<<16 | uint64(t[4])<<8 | uint64(t[5])
	sec = raw >> 30
	ns = raw & ((uint64(1) << 30) - 1)
	if sec > 86_399 || ns > 999_999_999 {
		return 0, 0, false
	}
	return sec, ns, true
}

// EncodeBeastFrame is the inverse of Decode: produces a wire-format Beast
// frame for the given (type, timestamp, signal, payload). It is used by
// RunSynth and tests; the stuffing rule (0x1A → 0x1A 0x1A) is applied to
// every byte after the leading escape+type pair.
func EncodeBeastFrame(t byte, ts [6]byte, signal byte, payload []byte) []byte {
	out := make([]byte, 0, 2+len(ts)+1+len(payload)*2)
	out = append(out, beastEscape, t)
	stuff := func(b byte) {
		if b == beastEscape {
			out = append(out, beastEscape, beastEscape)
		} else {
			out = append(out, b)
		}
	}
	for _, b := range ts {
		stuff(b)
	}
	stuff(signal)
	for _, b := range payload {
		stuff(b)
	}
	return out
}

// EncodeGPSTimestamp packs (sec, ns) into a 6-byte Air!Squitter GPS
// timestamp. sec must be < 86400 and ns must be < 1_000_000_000; out-of-range
// inputs are clamped (this function is best-effort; callers should validate
// upstream).
func EncodeGPSTimestamp(sec, ns uint64) [6]byte {
	if sec >= 86_400 {
		sec = 86_399
	}
	if ns >= 1_000_000_000 {
		ns = 999_999_999
	}
	raw := (sec << 30) | (ns & ((uint64(1) << 30) - 1))
	return [6]byte{
		byte(raw >> 40),
		byte(raw >> 32),
		byte(raw >> 24),
		byte(raw >> 16),
		byte(raw >> 8),
		byte(raw),
	}
}

// readFrames decodes Beast frames from r and emits FeedFrames on out until
// ctx is cancelled or r returns io.EOF. Mode-AC and status frames are
// silently dropped — only Mode-S short and long frames are forwarded.
//
// Used internally by RunBeastTCP and RunBeastReplay.
func readFrames(ctx context.Context, r io.Reader, out chan<- FeedFrame) error {
	dec := NewDecoder(r)
	var raw RawFrame
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
		sec, ns, _ := TimestampGPS(raw.Timestamp)
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
	}
}
