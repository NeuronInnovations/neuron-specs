package adsb

import (
	"time"

	sbs "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
)

// Per-ICAO merge cache defaults. Mirrors internal/feeds.ICAORecoveryCache
// sizing: busy European terminal airspace rarely holds more than a few hundred
// simultaneous aircraft, and 60s without an update means the aircraft has left
// the coverage area (so its cached position/velocity must not resurface).
const (
	defaultMergeCap = 512
	defaultMergeTTL = 60 * time.Second
)

// trackMerger reconstructs complete aircraft tracks from the DISJOINT SBS-1
// records a vanilla JetVision feed emits. In vanilla SBS-1, MSG type 3
// (position) and MSG type 4 (velocity) arrive as separate lines per ICAO —
// neither carries the other's fields. trackMerger overlays the fields present
// on each incoming record onto a per-ICAO cache so the returned track carries
// the latest-known position AND the latest-known velocity, letting the seller
// emit one complete NormalizedTrack per record.
//
// NOT safe for concurrent use. By design it is goroutine-local: one merger per
// seller stream pump goroutine (see serveStream), so it needs no mutex and the
// race detector observes no shared access. This deliberately differs from
// internal/feeds.ICAORecoveryCache (which is shared and therefore mutex-guarded).
type trackMerger struct {
	cap int
	ttl time.Duration
	now func() time.Time // injectable for deterministic tests; defaults to time.Now

	m map[string]*mergedEntry // keyed by ICAO (as parsed — already UPPER-cased)
}

type mergedEntry struct {
	track    sbs.SBSTrack
	lastSeen time.Time
}

// newTrackMerger returns a merger with the given capacity and TTL. Non-positive
// values fall back to defaultMergeCap / defaultMergeTTL.
func newTrackMerger(capacity int, ttl time.Duration) *trackMerger {
	if capacity <= 0 {
		capacity = defaultMergeCap
	}
	if ttl <= 0 {
		ttl = defaultMergeTTL
	}
	return &trackMerger{
		cap: capacity,
		ttl: ttl,
		now: time.Now,
		m:   make(map[string]*mergedEntry),
	}
}

// merge overlays the fields explicitly present on `in` onto the cached track
// for in.ICAO and returns the merged result. Fields absent on `in` (their
// *Set boolean false, or an empty string) are preserved from prior records for
// the same ICAO. An ICAO not seen within ttl starts fresh — stale velocity from
// a departed aircraft never resurfaces. Capacity is bounded by evicting the
// oldest entry when full (lazy TTL otherwise, mirroring ICAORecoveryCache).
func (tm *trackMerger) merge(in sbs.SBSTrack) sbs.SBSTrack {
	now := tm.now()
	icao := in.ICAO

	e, ok := tm.m[icao]
	if !ok || now.Sub(e.lastSeen) > tm.ttl {
		// New or expired entry: seed verbatim from the incoming record.
		tm.ensureCapacity()
		tm.m[icao] = &mergedEntry{track: in, lastSeen: now}
		return in
	}

	merged := e.track
	overlay(&merged, in)
	e.track = merged
	e.lastSeen = now
	return merged
}

// overlay copies the fields present on src onto dst. Position (lat/lon/alt) and
// velocity (speed/track/vertical-rate) are governed by their *Set flags;
// string/list/flag fields are copied only when non-empty/non-zero so a
// velocity record (empty callsign/squawk) cannot wipe a cached value. ICAO is
// the cache key and identical on both.
func overlay(dst *sbs.SBSTrack, src sbs.SBSTrack) {
	if src.LatSet {
		dst.Lat, dst.LatSet = src.Lat, true
	}
	if src.LonSet {
		dst.Lon, dst.LonSet = src.Lon, true
	}
	if src.AltSet {
		dst.AltFeet, dst.AltSet = src.AltFeet, true
	}
	if src.SpdSet {
		dst.SpdKnots, dst.SpdSet = src.SpdKnots, true
	}
	if src.TrkSet {
		dst.TrkDeg, dst.TrkSet = src.TrkDeg, true
	}
	if src.VrtSet {
		dst.VrtFpm, dst.VrtSet = src.VrtFpm, true
	}
	if src.Callsign != "" {
		dst.Callsign = src.Callsign
	}
	if src.Squawk != "" {
		dst.Squawk = src.Squawk
	}
	if src.Receivers > 0 {
		dst.Receivers = src.Receivers
	}
	if src.HorizErrM > 0 {
		dst.HorizErrM = src.HorizErrM
	}
	if len(src.Contributors) > 0 {
		dst.Contributors = src.Contributors
	}
	if src.SPI {
		dst.SPI = true
	}
	if src.EmergRaw != "" {
		dst.EmergRaw = src.EmergRaw
	}
	if !src.GeneratedAt.IsZero() {
		dst.GeneratedAt = src.GeneratedAt
	}
	if !src.Rx.IsZero() {
		dst.Rx = src.Rx
	}
}

// ensureCapacity evicts the oldest entry (by lastSeen) when the cache is at
// capacity, bounding memory. O(N) with N <= cap. Mirrors the eviction policy in
// internal/feeds.ICAORecoveryCache.Observe.
func (tm *trackMerger) ensureCapacity() {
	if len(tm.m) < tm.cap {
		return
	}
	var oldestKey string
	var oldestT time.Time
	for k, e := range tm.m {
		if oldestKey == "" || e.lastSeen.Before(oldestT) {
			oldestKey, oldestT = k, e.lastSeen
		}
	}
	if oldestKey != "" {
		delete(tm.m, oldestKey)
	}
}
