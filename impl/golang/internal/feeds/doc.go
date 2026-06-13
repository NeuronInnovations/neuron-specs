// Package feeds provides edge-side data-source adapters for the Neuron Go SDK.
//
// A feed is an unbounded stream of FeedFrame records produced by a producer
// (a hardware receiver, a recording, or a synthetic generator) and consumed
// by a seller binary that forwards the records over a P2P delivery channel.
//
// Sources currently provided:
//   - RunBeastTCP — dials a TCP host:port and decodes Mode S Beast frames
//     (the format JetVision Air!Squitter / dump1090 / readsb / rcd emit).
//   - RunBeastReplay — replays a captured Beast byte stream from a file.
//   - RunSynth — emits synthetic Mode-S short frames at a configurable rate
//     for tests and offline development.
//
// All sources share the same shape:
//
//	func RunXxx(ctx context.Context, ..., out chan<- FeedFrame) error
//
// They emit FeedFrames on out until ctx is cancelled or the source is
// exhausted, then return. They do not close out — that is the caller's
// responsibility (the caller knows when no further sources will write).
//
// The Beast decoder is in-tree (beast.go); it implements the documented Beast
// wire format and Air!Squitter GPS-timestamp encoding without any external
// dependency. If a richer Mode-S decoder is needed (e.g. ICAO + altitude +
// callsign extraction), pair feeds with a downstream decoder of choice.
//
// Wire encoding for libp2p forwarding is provided by EncodeFeedFrame /
// DecodeFeedFrame (wire.go): a length-prefixed binary envelope that preserves
// the GPS timestamp alongside the raw Mode-S payload so receivers can do MLAT
// or timestamp-aware analysis.
package feeds
