package delivery

import (
	"encoding/binary"
	"io"
)

// MaxFrameSize is the maximum payload size for a single data frame.
// FR-D22: 4 MiB (4,194,304 bytes).
const MaxFrameSize = 4 * 1024 * 1024

// frameLenSize is the size of the length prefix (4 bytes, big-endian uint32).
const frameLenSize = 4

// FrameWriter wraps an io.Writer and writes length-prefixed frames.
// FR-D22: 4-byte unsigned big-endian length prefix + payload bytes.
type FrameWriter struct {
	w io.Writer
}

// NewFrameWriter creates a FrameWriter wrapping the given writer.
func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{w: w}
}

// WriteFrame writes a length-prefixed data frame.
// FR-D22: rejects payloads > MaxFrameSize with FrameTooLarge error.
func (fw *FrameWriter) WriteFrame(data []byte) error {
	if len(data) > MaxFrameSize {
		return NewDeliveryError(ErrFrameTooLarge, "WriteFrame",
			"frame payload exceeds 4 MiB limit")
	}

	// Write 4-byte big-endian length prefix.
	var lenBuf [frameLenSize]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := fw.w.Write(lenBuf[:]); err != nil {
		return WrapDeliveryError(ErrStreamError, "WriteFrame", err)
	}

	// Write payload.
	if len(data) > 0 {
		if _, err := fw.w.Write(data); err != nil {
			return WrapDeliveryError(ErrStreamError, "WriteFrame", err)
		}
	}

	return nil
}

// WriteKeepAlive writes a zero-length frame (keep-alive sentinel).
// FR-D24: 4 bytes of zeros, no payload.
func (fw *FrameWriter) WriteKeepAlive() error {
	var zeroBuf [frameLenSize]byte
	if _, err := fw.w.Write(zeroBuf[:]); err != nil {
		return WrapDeliveryError(ErrStreamError, "WriteKeepAlive", err)
	}
	return nil
}

// FrameReader wraps an io.Reader and reads length-prefixed frames.
// FR-D22, FR-D24: silently skips zero-length keep-alive frames.
type FrameReader struct {
	r io.Reader
}

// NewFrameReader creates a FrameReader wrapping the given reader.
func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{r: r}
}

// ReadFrame reads the next data frame, skipping keep-alive sentinels.
// Returns the payload bytes. Returns io.EOF when the reader is exhausted.
// FR-D24: zero-length frames are consumed silently.
func (fr *FrameReader) ReadFrame() ([]byte, error) {
	for {
		// Read 4-byte length prefix.
		var lenBuf [frameLenSize]byte
		if _, err := io.ReadFull(fr.r, lenBuf[:]); err != nil {
			return nil, err // io.EOF or io.ErrUnexpectedEOF
		}

		length := binary.BigEndian.Uint32(lenBuf[:])

		// FR-D24: zero-length = keep-alive sentinel, skip silently.
		if length == 0 {
			continue
		}

		if length > MaxFrameSize {
			return nil, NewDeliveryError(ErrFrameTooLarge, "ReadFrame",
				"received frame exceeds 4 MiB limit")
		}

		// Read payload.
		payload := make([]byte, length)
		if _, err := io.ReadFull(fr.r, payload); err != nil {
			return nil, err
		}

		return payload, nil
	}
}
