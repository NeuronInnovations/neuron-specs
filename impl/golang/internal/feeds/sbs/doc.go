// Package sbs parses SBS-1 BaseStation CSV records and emits decoded SBSTrack
// values on a channel, following the source-adapter shape established by
// internal/feeds (RunXxx(ctx, ..., out chan<- T) error).
//
// SBS-1 BaseStation is the decoded text protocol the JetVision Air!Squitter
// emits on TCP port 30003 by default, and the same format the BlueMark
// DroneScout MQTT subscriber exports via modules/tcp_sbs_export.py with a
// configurable port (also defaulting to 30003). It is a CSV protocol with
// 22+ comma-separated fields per record; records are line-terminated with
// either \n or \r\n. The canonical wire-format reference is the woodair.net
// SBS-1 specification: http://woodair.net/sbs/article/barebones42_socket_data.htm
//
// Provenance note: this package's parser is a clean-room implementation
// against the woodair.net specification. An internal read-only fork of a
// deployed seller's SBS parser (160 LOC, encoding/csv based) was used as a
// behavioural reference. The upstream of that fork carries no
// LICENSE file, so no code was copy-ported; this package was written fresh
// from the woodair.net spec and validated to produce field-for-field-equivalent
// output on the same input lines.
//
// Sources currently provided:
//   - RunBaseStationTCP — dials a TCP host:port (e.g. "127.0.0.1:30003") and
//     parses SBS-1 MSG records.
//   - RunBaseStationReplay — replays a recorded .sbs capture file at a
//     configurable speedup / loop policy.
//   - RunBaseStationSynth — emits deterministic synthetic aircraft tracks
//     for offline testing.
//
// All sources share the same shape:
//
//	func RunBaseStationXxx(ctx context.Context, ..., out chan<- SBSTrack) error
//
// They emit SBSTrack values on out until ctx is cancelled (TCP source) or the
// input is exhausted (replay source without Loop). They do NOT close out —
// that is the caller's responsibility.
//
// SBS message-type coverage: this package parses MSG type 3 (airborne
// position) and MSG type 4 (airborne velocity). In vanilla SBS-1 these are
// DISJOINT per-ICAO records: a type-3 record carries lat/lon/altitude with
// EMPTY ground-speed/track/vertical-rate fields, and a type-4 record carries
// ground speed [12] / track [13] / vertical rate [16] with EMPTY position
// fields. Both are required to recover a complete track from a vanilla
// JetVision feed — speed and track are NOT present on type-3 records there.
// (The mlat-server variant happens to pack speed/track into its type-3 records
// as well, which is why a type-3-only parser appeared to work against
// mlat-style captures but produced velocity-less tracks from a real JV box.)
// The seller merges type 3 and type 4 by ICAO downstream
// (internal/dapp/adsb). Type 1 (callsign), type 2 (surface position), and
// types 5/6/7/8 (surveillance / on-ground variants) are dropped in v1.
//
// Units note: SBS-1 wire units are imperial / mixed (altitude in feet, ground
// speed in knots, vertical rate in fpm). This package preserves the upstream
// units in SBSTrack as-is. Conversion to SI metric happens at the
// internal/dapp/adsb layer when building NormalizedTrack per
// docs/normalized-track-contract.md.
//
// Variants: this package handles vanilla SBS-1 (fields [0..21]) and the
// mlat-server variant (additional trailing fields [22] herr, [23] rcv_users).
// The variant is detected by the presence of trailing fields; vanilla output
// silently omits them.
package sbs
