package feeds

import "encoding/hex"

// Mode-S downlink format constants. See ICAO Annex 10 Vol IV §3.1.2.5.
const (
	DFShortAirAirSurveillance byte = 0  // 56-bit
	DFAltitudeReply           byte = 4  // 56-bit
	DFIdentityReply           byte = 5  // 56-bit
	DFAllCallReply            byte = 11 // 56-bit, ICAO in bits 9-32
	DFLongAirAirSurveillance  byte = 16 // 112-bit
	DFExtendedSquitter        byte = 17 // 112-bit, ICAO in bits 9-32 (this is ADS-B)
	DFExtendedSquitterTIS_B   byte = 18 // 112-bit, ICAO in bits 9-32
	DFExtendedSquitterMilitary byte = 19 // 112-bit
	DFCommBAltitude           byte = 20 // 112-bit
	DFCommBIdentity           byte = 21 // 112-bit
	DFCommDExtended           byte = 24 // 112-bit (DF24 starts at top 2 bits = 11)
)

// ModeSMeta is a small struct of fields decodable from a Mode-S payload
// without per-aircraft state (no track maintenance, no CRC parity peeling).
//
// For DF 11, 17, 18, the ICAO24 is carried in the clear in bytes 1-3, so
// extraction is a 3-byte read. For other DFs the ICAO is XOR'd with the
// CRC parity and recovering it requires either (a) a known-aircraft list
// or (b) maintaining a frame-tracking decoder. We don't do that here —
// callers that need ICAO from DF 0/4/5/16/20/21 should layer a real
// Mode-S decoder on top of the FeedFrames they receive.
type ModeSMeta struct {
	// DF is the 5-bit downlink format (0..31). For DF 24 (ELM), the upper
	// 2 bits of byte 0 are `11` and the next 3 bits are reserved; this
	// field reports the value of `(buf[0] >> 3) & 0x1F` regardless.
	DF byte

	// ICAO is the 24-bit aircraft address as a 6-character lowercase hex
	// string (e.g. "4ca853"), or "" if not extractable from this frame.
	ICAO string

	// Long is true for 112-bit (14-byte) frames, false for 56-bit (7-byte).
	Long bool

	// Recovered is true when ICAO was reconstructed from a parity-XOR
	// candidate against a cache of recently-observed plaintext ICAOs (see
	// ICAORecoveryCache). False when ICAO came from a plaintext field
	// (DF 11/17/18) or is empty. Downstream consumers that need a strict
	// chain of custody should prefer Recovered=false sources.
	Recovered bool
}

// DecodeModeSMeta parses what it can from a Mode-S payload. Returns a zero
// ModeSMeta if buf is too short to be valid (< 7 bytes).
//
// The function is intentionally minimal: it covers DF 11/17/18 ICAO recovery
// (the source of all ADS-B traffic) and reports the DF for everything else.
// This is enough for the airport-display use case (ICAO + position would
// come from the same DF 17 frame's payload field, which the buyer can decode
// downstream); callers that need anything more sophisticated should swap in
// a fuller decoder via the Source interface.
func DecodeModeSMeta(buf []byte) ModeSMeta {
	if len(buf) < 7 {
		return ModeSMeta{}
	}
	df := (buf[0] >> 3) & 0x1F
	long := len(buf) >= 14

	switch df {
	case DFAllCallReply, DFExtendedSquitter, DFExtendedSquitterTIS_B, DFExtendedSquitterMilitary:
		// ICAO is in bytes 1-3, plain text.
		return ModeSMeta{
			DF:   df,
			ICAO: hex.EncodeToString(buf[1:4]),
			Long: long,
		}
	}
	// Other DFs carry ICAO XOR'd with parity; we don't recover here.
	return ModeSMeta{DF: df, Long: long}
}
