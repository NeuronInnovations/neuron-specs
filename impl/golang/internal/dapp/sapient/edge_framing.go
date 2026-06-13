package sapient

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MaxEdgeFrameSize bounds a single SAPIENT-edge frame payload. Mirrors
// internal/delivery.MaxFrameSize (4 MiB); a DetectionReport with the full rid.*
// object_info set is ~2 KiB, so this is a generous safety ceiling.
const MaxEdgeFrameSize = 4 * 1024 * 1024

// edgeLenSize is the SAPIENT-edge length-prefix size (4-byte little-endian uint32).
const edgeLenSize = 4

// writeLEFrame writes one SAPIENT-edge frame: a 4-byte LITTLE-endian length
// prefix (the payload length, excluding the prefix itself) followed by the
// payload. This is the FR-S91 conformant edge framing — BSI Flex 335 v2.0
// protobuf over TCP, per SAPIENT ICD v7 §2.1.2, matching the DSTL Apex reference
// middleware's `struct.pack("<I", …)`.
//
// It is DISTINCT from internal/delivery's 4-byte BIG-endian framing, which
// carries the Neuron p2p inter-proxy lane (/sapient/detection/2.0.0). The two
// MUST NOT be conflated (015 §B / FR-S91): little-endian at the SAPIENT edge,
// big-endian between the proxies.
func writeLEFrame(w io.Writer, payload []byte) error {
	if len(payload) > MaxEdgeFrameSize {
		return fmt.Errorf("sapient.writeLEFrame: payload %d exceeds %d-byte limit", len(payload), MaxEdgeFrameSize)
	}
	var lenBuf [edgeLenSize]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// readLEFrame reads one SAPIENT-edge frame written by writeLEFrame. A clean
// io.EOF before any byte of the length prefix signals end-of-stream; a truncated
// prefix or payload surfaces as io.ErrUnexpectedEOF (never a panic). A length
// over the 4 MiB ceiling is rejected before any payload is allocated.
func readLEFrame(r io.Reader) ([]byte, error) {
	var lenBuf [edgeLenSize]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err // io.EOF (clean) or io.ErrUnexpectedEOF (truncated prefix)
	}
	length := binary.LittleEndian.Uint32(lenBuf[:])
	if length > MaxEdgeFrameSize {
		return nil, fmt.Errorf("sapient.readLEFrame: frame length %d exceeds %d-byte limit", length, MaxEdgeFrameSize)
	}
	if length == 0 {
		return []byte{}, nil
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
