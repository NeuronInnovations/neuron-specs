package sapient

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLEFrame_RoundTrip(t *testing.T) {
	payloads := [][]byte{
		[]byte("hello"),
		{},                                // empty payload round-trips
		bytes.Repeat([]byte{0xAB}, 1<<16), // 64 KiB
	}
	var buf bytes.Buffer
	for _, p := range payloads {
		require.NoError(t, writeLEFrame(&buf, p))
	}
	for _, want := range payloads {
		got, err := readLEFrame(&buf)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	// Stream is now exhausted → clean EOF.
	_, err := readLEFrame(&buf)
	require.ErrorIs(t, err, io.EOF)
}

// TestLEFrame_LittleEndianPrefix is the on-the-wire proof: the 4-byte length
// prefix is little-endian (low byte first), NOT big-endian.
func TestLEFrame_LittleEndianPrefix(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeLEFrame(&buf, make([]byte, 5)))
	require.Equal(t, []byte{0x05, 0x00, 0x00, 0x00}, buf.Bytes()[:4], "len 5 → LE prefix 05 00 00 00")

	buf.Reset()
	require.NoError(t, writeLEFrame(&buf, make([]byte, 258)))
	require.Equal(t, []byte{0x02, 0x01, 0x00, 0x00}, buf.Bytes()[:4], "len 258 → LE prefix 02 01 00 00")
	// Sanity: big-endian would have been 00 00 01 02.
}

func TestLEFrame_OversizeRejectedOnWrite(t *testing.T) {
	var buf bytes.Buffer
	err := writeLEFrame(&buf, make([]byte, MaxEdgeFrameSize+1))
	require.Error(t, err)
	require.Equal(t, 0, buf.Len(), "nothing written when oversize")
}

func TestLEFrame_OversizeRejectedOnRead(t *testing.T) {
	// Craft a prefix claiming > 4 MiB, with no payload. Must reject on the length
	// check before allocating.
	var prefix [4]byte
	binary.LittleEndian.PutUint32(prefix[:], MaxEdgeFrameSize+1)
	_, err := readLEFrame(bytes.NewReader(prefix[:]))
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds")
}

func TestLEFrame_TruncatedPrefix(t *testing.T) {
	_, err := readLEFrame(bytes.NewReader([]byte{0x05, 0x00})) // only 2 of 4 prefix bytes
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestLEFrame_TruncatedPayload(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(10)) // claims 10 bytes…
	buf.Write([]byte{1, 2, 3})                          // …but only 3 follow
	_, err := readLEFrame(&buf)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestLEFrame_CleanEOFOnEmpty(t *testing.T) {
	_, err := readLEFrame(bytes.NewReader(nil))
	require.ErrorIs(t, err, io.EOF)
}
