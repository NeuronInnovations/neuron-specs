package tasking

import (
	"crypto/rand"
	"io"
	"time"
)

// crockford is the ULID alphabet (Crockford base32, excluding I, L, O, U).
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// DefaultIDFunc returns a fresh canonical ULID (48-bit ms timestamp + 80-bit
// crypto/rand entropy). Dependency-free; satisfies the SAPIENT is_ulid fields
// (task_id, report_id). Falls back to a zero-entropy ULID only if the system RNG
// fails (never expected).
func DefaultIDFunc() string {
	s, err := newULID(time.Now(), rand.Reader)
	if err != nil {
		s, _ = newULID(time.Now(), zeroReader{})
	}
	return s
}

// newULID builds a ULID string from a timestamp and an entropy source. Exposed for
// deterministic tests (fixed time + fixed entropy => fixed ULID).
func newULID(t time.Time, entropy io.Reader) (string, error) {
	var id [16]byte
	ms := uint64(t.UnixMilli())
	id[0] = byte(ms >> 40)
	id[1] = byte(ms >> 32)
	id[2] = byte(ms >> 24)
	id[3] = byte(ms >> 16)
	id[4] = byte(ms >> 8)
	id[5] = byte(ms)
	if _, err := io.ReadFull(entropy, id[6:]); err != nil {
		return "", err
	}
	return encodeULID(id), nil
}

// encodeULID renders a 16-byte ULID as 26 Crockford-base32 chars (canonical
// oklog/ulid bit mapping).
func encodeULID(id [16]byte) string {
	dst := make([]byte, 26)
	dst[0] = crockford[(id[0]&224)>>5]
	dst[1] = crockford[id[0]&31]
	dst[2] = crockford[(id[1]&248)>>3]
	dst[3] = crockford[((id[1]&7)<<2)|((id[2]&192)>>6)]
	dst[4] = crockford[(id[2]&62)>>1]
	dst[5] = crockford[((id[2]&1)<<4)|((id[3]&240)>>4)]
	dst[6] = crockford[((id[3]&15)<<1)|((id[4]&128)>>7)]
	dst[7] = crockford[(id[4]&124)>>2]
	dst[8] = crockford[((id[4]&3)<<3)|((id[5]&224)>>5)]
	dst[9] = crockford[id[5]&31]
	dst[10] = crockford[(id[6]&248)>>3]
	dst[11] = crockford[((id[6]&7)<<2)|((id[7]&192)>>6)]
	dst[12] = crockford[(id[7]&62)>>1]
	dst[13] = crockford[((id[7]&1)<<4)|((id[8]&240)>>4)]
	dst[14] = crockford[((id[8]&15)<<1)|((id[9]&128)>>7)]
	dst[15] = crockford[(id[9]&124)>>2]
	dst[16] = crockford[((id[9]&3)<<3)|((id[10]&224)>>5)]
	dst[17] = crockford[id[10]&31]
	dst[18] = crockford[(id[11]&248)>>3]
	dst[19] = crockford[((id[11]&7)<<2)|((id[12]&192)>>6)]
	dst[20] = crockford[(id[12]&62)>>1]
	dst[21] = crockford[((id[12]&1)<<4)|((id[13]&240)>>4)]
	dst[22] = crockford[((id[13]&15)<<1)|((id[14]&128)>>7)]
	dst[23] = crockford[(id[14]&124)>>2]
	dst[24] = crockford[((id[14]&3)<<3)|((id[15]&224)>>5)]
	dst[25] = crockford[id[15]&31]
	return string(dst)
}

// zeroReader yields an endless stream of 0x00 (deterministic fallback entropy).
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
