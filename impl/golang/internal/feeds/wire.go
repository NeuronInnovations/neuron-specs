package feeds

import (
	"encoding/binary"
	"errors"
)

// EncodeFeedFrame produces a compact binary envelope for forwarding a
// FeedFrame over a libp2p delivery channel:
//
//	[8 bytes secSinceMid (BE uint64)]
//	[8 bytes nanos       (BE uint64)]
//	[2 bytes payload_len (BE uint16)]
//	[N bytes payload]
//
// Total overhead is 18 bytes per frame. Mode-S payloads are at most 14 bytes
// so a single record fits in a 32-byte libp2p data frame and is far below
// the 4 MiB FrameWriter limit.
func EncodeFeedFrame(f FeedFrame) []byte {
	if len(f.Raw) > 0xFFFF {
		// Truncate; Mode-S is never this large. Defensive only.
		f.Raw = f.Raw[:0xFFFF]
	}
	out := make([]byte, 18+len(f.Raw))
	binary.BigEndian.PutUint64(out[0:8], f.SecondsSinceMidnight)
	binary.BigEndian.PutUint64(out[8:16], f.Nanoseconds)
	binary.BigEndian.PutUint16(out[16:18], uint16(len(f.Raw)))
	copy(out[18:], f.Raw)
	return out
}

// ErrShortFeedFrame is returned when DecodeFeedFrame is given fewer bytes
// than the minimum envelope size, or when the declared payload length
// exceeds the available buffer.
var ErrShortFeedFrame = errors.New("feeds: short feed-frame envelope")

// DecodeFeedFrame parses the binary envelope produced by EncodeFeedFrame.
// The returned FeedFrame's Rx is left zero — receivers can populate it from
// the libp2p DataFrame.ReceivedAt timestamp.
func DecodeFeedFrame(buf []byte) (FeedFrame, error) {
	if len(buf) < 18 {
		return FeedFrame{}, ErrShortFeedFrame
	}
	plen := binary.BigEndian.Uint16(buf[16:18])
	if 18+int(plen) > len(buf) {
		return FeedFrame{}, ErrShortFeedFrame
	}
	out := FeedFrame{
		SecondsSinceMidnight: binary.BigEndian.Uint64(buf[0:8]),
		Nanoseconds:          binary.BigEndian.Uint64(buf[8:16]),
		Raw:                  append([]byte(nil), buf[18:18+int(plen)]...),
	}
	return out, nil
}
