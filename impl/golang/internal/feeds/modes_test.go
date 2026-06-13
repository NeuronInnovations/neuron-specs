package feeds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeModeSMeta_DF17_ExtractsICAO(t *testing.T) {
	// Real ADS-B DF=17 frame — first byte 0x8D = 10001101 → DF = 0b10001 = 17.
	// Bytes 1-3 are the ICAO24 (4ca853 = an Aer Lingus A320 in this synthetic example).
	buf := []byte{0x8D, 0x4C, 0xA8, 0x53, 0x58, 0xC3, 0x82, 0xD6, 0x90, 0xC8, 0xAC, 0x28, 0x63, 0xA7}
	got := DecodeModeSMeta(buf)
	assert.Equal(t, byte(DFExtendedSquitter), got.DF)
	assert.Equal(t, "4ca853", got.ICAO)
	assert.True(t, got.Long)
}

func TestDecodeModeSMeta_DF11_ExtractsICAO(t *testing.T) {
	// DF=11 all-call reply. First byte 0x58 = 01011000 → DF = 0b01011 = 11.
	// Length is short (7 bytes).
	buf := []byte{0x58, 0xAB, 0xCD, 0xEF, 0x12, 0x34, 0x56}
	got := DecodeModeSMeta(buf)
	assert.Equal(t, byte(DFAllCallReply), got.DF)
	assert.Equal(t, "abcdef", got.ICAO)
	assert.False(t, got.Long)
}

func TestDecodeModeSMeta_DF4_NoICAOExtraction(t *testing.T) {
	// DF=4 surveillance altitude reply. First byte 0x20 = 00100000 → DF=4.
	// ICAO is in CRC parity, which we don't recover.
	buf := []byte{0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	got := DecodeModeSMeta(buf)
	assert.Equal(t, byte(DFAltitudeReply), got.DF)
	assert.Equal(t, "", got.ICAO)
	assert.False(t, got.Long)
}

func TestDecodeModeSMeta_DF21_LongFrameNoICAO(t *testing.T) {
	// DF=21 Comm-B identity reply, 112-bit.
	buf := []byte{0xA8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	got := DecodeModeSMeta(buf)
	assert.Equal(t, byte(DFCommBIdentity), got.DF)
	assert.Empty(t, got.ICAO)
	assert.True(t, got.Long)
}

func TestDecodeModeSMeta_TooShort(t *testing.T) {
	got := DecodeModeSMeta([]byte{0x8D, 0x4C})
	assert.Equal(t, ModeSMeta{}, got)
}
