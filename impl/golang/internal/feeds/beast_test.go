package feeds

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeBeastFrame_RoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		ftype   byte
		ts      [6]byte
		signal  byte
		payload []byte
	}{
		{"short", BeastTypeModeSShort, [6]byte{1, 2, 3, 4, 5, 6}, 0xAA, bytes.Repeat([]byte{0x5D}, 7)},
		{"long", BeastTypeModeSLong, [6]byte{0, 0, 0, 0, 0, 0}, 0x00, bytes.Repeat([]byte{0x8D}, 14)},
		{"with_escape_in_payload", BeastTypeModeSShort, [6]byte{0x1A, 0, 0, 0, 0, 0}, 0x1A, []byte{0x1A, 0, 0x1A, 0, 0x1A, 0, 0x1A}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wire := EncodeBeastFrame(c.ftype, c.ts, c.signal, c.payload)
			dec := NewDecoder(bytes.NewReader(wire))
			var got RawFrame
			require.NoError(t, dec.Decode(&got))
			assert.Equal(t, c.ftype, got.Type)
			assert.Equal(t, c.ts, got.Timestamp)
			assert.Equal(t, c.signal, got.Signal)
			assert.Equal(t, c.payload, got.Payload)
		})
	}
}

func TestDecoder_MultipleFrames(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 10; i++ {
		buf.Write(EncodeBeastFrame(BeastTypeModeSShort,
			[6]byte{byte(i), 0, 0, 0, 0, 0}, byte(i),
			[]byte{byte(i), 0, 0, 0, 0, 0, 0}))
	}

	dec := NewDecoder(&buf)
	var raw RawFrame
	for i := 0; i < 10; i++ {
		require.NoError(t, dec.Decode(&raw))
		assert.Equal(t, BeastTypeModeSShort, raw.Type)
		assert.Equal(t, byte(i), raw.Timestamp[0])
	}
	err := dec.Decode(&raw)
	assert.ErrorIs(t, err, io.EOF)
}

func TestDecoder_SkipsUnknownTypes(t *testing.T) {
	// Garbage prefix (no escape), then a valid short frame.
	garbage := []byte{0x99, 0x88, 0xAA}
	frame := EncodeBeastFrame(BeastTypeModeSShort, [6]byte{1, 2, 3, 4, 5, 6}, 0xFF,
		[]byte{1, 2, 3, 4, 5, 6, 7})

	dec := NewDecoder(bytes.NewReader(append(garbage, frame...)))
	var raw RawFrame
	require.NoError(t, dec.Decode(&raw))
	assert.Equal(t, BeastTypeModeSShort, raw.Type)
}

func TestDecoder_StatusFrames(t *testing.T) {
	// Status frame interleaved with data frames is decoded but readFrames will
	// drop it. Test the decoder itself surfaces it.
	status := EncodeBeastFrame(BeastTypeStatus, [6]byte{0, 0, 0, 0, 0, 0}, 0,
		bytes.Repeat([]byte{0xCC}, 14))
	data := EncodeBeastFrame(BeastTypeModeSShort, [6]byte{0, 0, 0, 0, 0, 0}, 0,
		[]byte{1, 2, 3, 4, 5, 6, 7})
	dec := NewDecoder(bytes.NewReader(append(status, data...)))

	var raw RawFrame
	require.NoError(t, dec.Decode(&raw))
	assert.Equal(t, BeastTypeStatus, raw.Type)

	require.NoError(t, dec.Decode(&raw))
	assert.Equal(t, BeastTypeModeSShort, raw.Type)
}

func TestTimestampGPS(t *testing.T) {
	ts := EncodeGPSTimestamp(46_800, 123_456_789) // 13:00:00 UTC
	sec, ns, ok := TimestampGPS(ts)
	assert.True(t, ok)
	assert.Equal(t, uint64(46_800), sec)
	assert.Equal(t, uint64(123_456_789), ns)
}

func TestTimestampGPS_Invalid(t *testing.T) {
	// Sec field intentionally too large — encoded raw, not via Encode helper.
	bad := [6]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	_, _, ok := TimestampGPS(bad)
	assert.False(t, ok)
}

func TestReadFrames_DropsNonModeS(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(EncodeBeastFrame(BeastTypeStatus, [6]byte{}, 0, bytes.Repeat([]byte{0}, 14)))
	buf.Write(EncodeBeastFrame(BeastTypeModeAC, [6]byte{}, 0, []byte{0, 0}))
	for i := 0; i < 5; i++ {
		buf.Write(EncodeBeastFrame(BeastTypeModeSShort,
			EncodeGPSTimestamp(uint64(i), 0), 0,
			[]byte{byte(i), 0, 0, 0, 0, 0, 0}))
	}

	out := make(chan FeedFrame, 16)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, readFrames(ctx, &buf, out))

	close(out)
	var got []FeedFrame
	for f := range out {
		got = append(got, f)
	}
	assert.Len(t, got, 5, "only Mode-S short frames forwarded")
	for i, f := range got {
		assert.Equal(t, uint64(i), f.SecondsSinceMidnight)
		assert.Len(t, f.Raw, 7)
	}
}

func TestReadFrames_RespectsContext(t *testing.T) {
	// Reader that blocks forever.
	pr, pw := io.Pipe()
	defer pw.Close()

	out := make(chan FeedFrame, 1)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- readFrames(ctx, pr, out) }()

	cancel()
	select {
	case err := <-done:
		assert.True(t, errors.Is(err, context.Canceled) || errors.Is(err, io.ErrClosedPipe),
			"unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("readFrames did not return after ctx cancel")
	}
}
