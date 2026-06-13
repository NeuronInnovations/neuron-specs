package feeds

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeedFrameWire_RoundTrip(t *testing.T) {
	f := FeedFrame{
		Raw:                  []byte{0x8D, 0x40, 0x62, 0x1D, 0x58, 0xC3, 0x82, 0xD6, 0x90, 0xC8, 0xAC, 0x28, 0x63, 0xA7},
		SecondsSinceMidnight: 12345,
		Nanoseconds:          678_901_234,
	}
	wire := EncodeFeedFrame(f)
	assert.Len(t, wire, 18+len(f.Raw))

	got, err := DecodeFeedFrame(wire)
	require.NoError(t, err)
	assert.Equal(t, f.SecondsSinceMidnight, got.SecondsSinceMidnight)
	assert.Equal(t, f.Nanoseconds, got.Nanoseconds)
	assert.Equal(t, f.Raw, got.Raw)
}

func TestFeedFrameWire_EmptyPayload(t *testing.T) {
	f := FeedFrame{SecondsSinceMidnight: 1, Nanoseconds: 2}
	wire := EncodeFeedFrame(f)
	assert.Len(t, wire, 18)
	got, err := DecodeFeedFrame(wire)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), got.SecondsSinceMidnight)
	assert.Equal(t, uint64(2), got.Nanoseconds)
	assert.Empty(t, got.Raw)
}

func TestDecodeFeedFrame_ShortBuffer(t *testing.T) {
	_, err := DecodeFeedFrame([]byte{1, 2, 3})
	assert.ErrorIs(t, err, ErrShortFeedFrame)
}

func TestDecodeFeedFrame_TruncatedPayload(t *testing.T) {
	// Header claims 100-byte payload but only 5 bytes follow.
	buf := make([]byte, 18+5)
	buf[16] = 0x00
	buf[17] = 0x64 // 100
	_, err := DecodeFeedFrame(buf)
	assert.ErrorIs(t, err, ErrShortFeedFrame)
}
