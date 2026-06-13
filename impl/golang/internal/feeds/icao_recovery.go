package feeds

import (
	"encoding/hex"
	"sync"
	"time"
)

// Mode-S CRC polynomial — 24-bit, MSB-first. ICAO Annex 10 Vol IV §3.1.2.3.4.
//
// G(x) = x^24 + x^23 + x^22 + x^21 + x^20 + x^19 + x^18 + x^17 + x^16 +
//        x^15 + x^14 + x^13 + x^12 + x^10 + x^3 + 1
//
// In 25-bit form that's 0x1_FFF409. The implicit top-bit (x^24) is supplied
// by the shift-out from the 24-bit register; we XOR with the lower 24 bits.
const modesCRCPoly uint32 = 0xFFF409

// modesCRCTable is the byte-at-a-time CRC table for Mode-S.
var modesCRCTable [256]uint32

func init() {
	for i := range modesCRCTable {
		c := uint32(i) << 16
		for j := 0; j < 8; j++ {
			if c&0x800000 != 0 {
				c = (c << 1) ^ modesCRCPoly
			} else {
				c <<= 1
			}
		}
		modesCRCTable[i] = c & 0x00FFFFFF
	}
}

// ModeSCRC computes the 24-bit Mode-S CRC over msg. Returns a value in
// [0, 1<<24).
func ModeSCRC(msg []byte) uint32 {
	var crc uint32
	for _, b := range msg {
		idx := byte(crc>>16) ^ b
		crc = (crc << 8) ^ modesCRCTable[idx]
	}
	return crc & 0x00FFFFFF
}

// ICAORecoveryCache maps recently-seen ICAO24 addresses (sourced from
// plaintext DF 11/17/18 frames) to a last-seen timestamp, and offers a
// TryRecover method that attempts to attribute a parity-XOR'd Mode-S frame
// (DF 0/4/5/16/20/21) to one of the cached ICAOs.
//
// The cache is a fixed-capacity bounded set with TTL eviction:
//   - cap (max entries) prevents unbounded growth in pathological
//     traffic conditions
//   - ttl drops aircraft no longer transmitting plaintext frames
//
// Both eviction paths are lazy: stale entries linger in the map until
// touched, but never escape Has / TryRecover. The cache is safe for
// concurrent use.
type ICAORecoveryCache struct {
	mu  sync.Mutex
	ttl time.Duration
	cap int
	m   map[string]time.Time
	now func() time.Time // injectable for tests
}

// NewICAORecoveryCache returns a cache with the given capacity and TTL.
// Recommended values for an airport-display use case in busy European
// airspace: cap=512, ttl=60s.
func NewICAORecoveryCache(cap int, ttl time.Duration) *ICAORecoveryCache {
	if cap <= 0 {
		cap = 512
	}
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &ICAORecoveryCache{
		ttl: ttl,
		cap: cap,
		m:   make(map[string]time.Time),
		now: time.Now,
	}
}

// Observe records that an ICAO24 was seen as plaintext (DF 11/17/18) at the
// current time. Idempotent — calling repeatedly with the same ICAO simply
// refreshes its last-seen timestamp.
func (c *ICAORecoveryCache) Observe(icao string) {
	if icao == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.cap {
		// Bound capacity: evict the oldest entry. O(N) but N <= cap and
		// the call rate is bounded by airspace activity (~few hundred /sec).
		var oldestKey string
		var oldestT time.Time
		for k, t := range c.m {
			if oldestKey == "" || t.Before(oldestT) {
				oldestKey, oldestT = k, t
			}
		}
		if oldestKey != "" {
			delete(c.m, oldestKey)
		}
	}
	c.m[icao] = c.now()
}

// Has returns true if icao was Observed within the TTL window.
func (c *ICAORecoveryCache) Has(icao string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.m[icao]
	if !ok {
		return false
	}
	if c.now().Sub(t) > c.ttl {
		delete(c.m, icao)
		return false
	}
	return true
}

// Size returns the current entry count. Stale entries that haven't been
// touched still count — Has / TryRecover are the eviction paths.
func (c *ICAORecoveryCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// TryRecover attempts to attribute a parity-XOR'd Mode-S frame to a cached
// ICAO. Accepts the raw Mode-S bytes (7 for short, 14 for long); other
// lengths are rejected.
//
// Algorithm: compute CRC over body (first len-3 bytes), XOR with the AP
// field (last 3 bytes) → candidate ICAO. If the candidate is in the cache
// (and within TTL), return it.
//
// Caller is responsible for filtering DFs that actually carry XOR'd
// addresses (i.e. NOT DF 11/17/18 — those have plaintext ICAO already and
// their AP field encodes interrogator/subnetwork bits, not parity^ICAO).
//
// Returns ("", false) if the message is malformed, too short, or the
// candidate ICAO isn't in the cache.
func (c *ICAORecoveryCache) TryRecover(modeS []byte) (string, bool) {
	if len(modeS) != 7 && len(modeS) != 14 {
		return "", false
	}
	body := modeS[:len(modeS)-3]
	ap := modeS[len(modeS)-3:]
	crc := ModeSCRC(body)
	apVal := uint32(ap[0])<<16 | uint32(ap[1])<<8 | uint32(ap[2])
	candidate := apVal ^ crc
	icao := hex.EncodeToString([]byte{
		byte(candidate >> 16),
		byte(candidate >> 8),
		byte(candidate),
	})
	if !c.Has(icao) {
		return "", false
	}
	return icao, true
}
