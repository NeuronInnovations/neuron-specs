package delivery

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T005: Framing Tests ---

func TestFrameWriter_WriteFrame_RoundTrip(t *testing.T) {
	// FR-D22: write + read round-trip.
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)
	r := NewFrameReader(&buf)

	data := []byte("hello neuron delivery")
	err := w.WriteFrame(data)
	require.NoError(t, err)

	got, err := r.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestFrameWriter_WriteFrame_LengthPrefix(t *testing.T) {
	// FR-D22: 4-byte big-endian length prefix.
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)

	data := []byte("test")
	err := w.WriteFrame(data)
	require.NoError(t, err)

	raw := buf.Bytes()
	assert.Equal(t, 4+len(data), len(raw), "total = 4 prefix + payload")

	length := binary.BigEndian.Uint32(raw[:4])
	assert.Equal(t, uint32(len(data)), length)
	assert.Equal(t, data, raw[4:])
}

func TestFrameWriter_WriteFrame_TooLarge(t *testing.T) {
	// FR-D22: reject > 4 MiB.
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)

	bigData := make([]byte, MaxFrameSize+1)
	err := w.WriteFrame(bigData)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrFrameTooLarge, de.Kind())
}

func TestFrameWriter_WriteFrame_ExactlyMaxSize(t *testing.T) {
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)

	data := make([]byte, MaxFrameSize)
	err := w.WriteFrame(data)
	require.NoError(t, err, "exactly MaxFrameSize should succeed")
}

func TestFrameReader_KeepAlive_Skipped(t *testing.T) {
	// FR-D24: zero-length frame consumed silently.
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)

	// Write: keep-alive, then real data, then keep-alive, then real data.
	w.WriteKeepAlive()
	w.WriteFrame([]byte("first"))
	w.WriteKeepAlive()
	w.WriteKeepAlive()
	w.WriteFrame([]byte("second"))

	r := NewFrameReader(&buf)

	got1, err := r.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, []byte("first"), got1)

	got2, err := r.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, []byte("second"), got2)
}

func TestFrameReader_EOF(t *testing.T) {
	var buf bytes.Buffer
	r := NewFrameReader(&buf)

	_, err := r.ReadFrame()
	assert.ErrorIs(t, err, io.EOF)
}

func TestFrameWriter_WriteKeepAlive(t *testing.T) {
	// FR-D24: keep-alive is 4 zero bytes.
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)

	err := w.WriteKeepAlive()
	require.NoError(t, err)

	raw := buf.Bytes()
	assert.Equal(t, 4, len(raw))
	assert.Equal(t, []byte{0, 0, 0, 0}, raw)
}

func TestFrameRoundTrip_MultipleFrames(t *testing.T) {
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)
	r := NewFrameReader(&buf)

	frames := [][]byte{
		[]byte("frame-1"),
		[]byte("frame-2-longer-content"),
		[]byte{0x00, 0x01, 0x02, 0xFF}, // binary content
	}

	for _, f := range frames {
		require.NoError(t, w.WriteFrame(f))
	}

	for _, expected := range frames {
		got, err := r.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, expected, got)
	}
}

func TestFrameRoundTrip_BinaryPayloadIntegrity(t *testing.T) {
	// FR-D23: opaque bytes survive round-trip unmodified.
	// Includes nulls, high bytes, and all byte values.
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)
	r := NewFrameReader(&buf)

	// All 256 byte values.
	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}

	require.NoError(t, w.WriteFrame(allBytes))

	got, err := r.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, allBytes, got, "arbitrary binary payload must survive round-trip unmodified")
}

func TestFrameRoundTrip_EmptyPayload(t *testing.T) {
	// Empty payload (length=0) is a keep-alive — should be skipped.
	// So writing an empty frame via WriteFrame produces length=0 which is keep-alive.
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)
	r := NewFrameReader(&buf)

	// Write empty frame (keep-alive) then real frame.
	require.NoError(t, w.WriteFrame([]byte{}))
	require.NoError(t, w.WriteFrame([]byte("real")))

	// Reader skips the keep-alive and returns "real".
	got, err := r.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, []byte("real"), got)
}
