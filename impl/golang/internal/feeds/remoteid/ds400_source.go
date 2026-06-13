package remoteid

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// DroneScout DS-400 adapter — Phase-3 entry point.
//
// **Status**: STUB. No DS-400 device has been available for protocol
// capture as of 2026-05-08. The vertical slice
// proved end-to-end using RunReplay + RunSynth; this file holds the
// shape into which the real protocol decoder will drop once the device
// is reachable and we can capture sample frames. The shape is designed
// so the real swap-in is **source-adapter only** — neither
// internal/dapp/remoteid (the DApp seller) nor cmd/remoteid-seller (the
// binary) needs to change.
//
// Required to lift the stub:
//
//  1. DS-400 device access for protocol capture.
//  2. Captured fixtures under `testdata/remoteid/ds400-*.{bin,pcap,json}`.
//  3. A concrete `FrameDecoder` registered via `RegisterFrameDecoder`
//     (lives in a future ds400_decoder_<variant>.go file once the wire
//     format is confirmed). The decoder converts raw inbound bytes into
//     `DecodedFrame` records.
//
// Until then `RunDS400` returns `ErrDS400Unavailable` immediately and
// operators rely on `--synth` or `--replay`.

// ErrDS400Unavailable is returned by RunDS400 when no FrameDecoder has
// been registered for the requested transport. This is the steady-state
// stub behavior: the source exists as a contract, but no implementation
// is wired yet.
var ErrDS400Unavailable = errors.New("feeds/remoteid: DS-400 source unavailable (no decoder registered; DS-400 capture fixtures required)")

// DS400Transport identifies the wire protocol used by a DS-400 unit. The
// real value to use is unknown until vendor docs land (Phase 0 Q1 P0.2);
// the three enum values here cover the plausible options DroneScout
// exposes on similar products.
type DS400Transport string

const (
	// DS400TransportUDP — UDP unicast or broadcast to a configured port.
	// Many Remote ID receivers expose decoded data over UDP for low-latency
	// fan-out to LAN consumers.
	DS400TransportUDP DS400Transport = "udp"

	// DS400TransportTCP — TCP stream from the device's local LAN address.
	// Common when the receiver runs a small server that ships decoded
	// records.
	DS400TransportTCP DS400Transport = "tcp"

	// DS400TransportHTTP — HTTP polling or long-poll endpoint. Some
	// Remote ID products expose a /api/drones JSON endpoint or an SSE
	// feed.
	DS400TransportHTTP DS400Transport = "http"
)

// DS400Config parameterizes RunDS400.
//
// Operators set `Transport` to match the device's actual feed (still
// TBD); `Address` is the host:port (for UDP/TCP) or full URL (for HTTP).
// `BufferSize` bounds how many DecodedFrames may be queued before the
// source applies back-pressure on the network read; the default (256)
// matches the per-stream buffer in the DApp seller.
type DS400Config struct {
	// Transport is one of DS400TransportUDP, TCP, HTTP.
	Transport DS400Transport

	// Address is the transport-dependent endpoint string:
	//
	//   - UDP: "host:port" listen address (e.g., "0.0.0.0:14550") or
	//     the multicast group "239.0.0.1:30100".
	//   - TCP: "host:port" remote address to dial.
	//   - HTTP: full URL (e.g., "http://ds400.local:8080/api/drones").
	Address string

	// BufferSize is the channel buffer between the network read goroutine
	// and the consumer. 0 means use the default (256).
	BufferSize int

	// Source is the value to stamp into DecodedFrame.Source for every
	// frame produced by this adapter. Defaults to "dronescout-ds400"
	// when empty.
	Source string
}

// FrameDecoder converts one raw inbound payload (UDP datagram, TCP
// length-prefixed record, or HTTP response chunk — whichever transport
// the operator configured) into one or more DecodedFrame records. The
// decoder MUST NOT block on I/O — the network read loop calls it
// synchronously and any blocking would stall the source.
//
// Implementations return:
//
//   - (frames, nil) when the input parses cleanly into >=1 frames;
//   - (nil, err) when the input is malformed; the source logs the error
//     and continues with the next payload.
//
// A FrameDecoder is registered per-transport via RegisterFrameDecoder.
// Phase-2 ships ZERO registered decoders, so RunDS400 returns
// ErrDS400Unavailable for every transport.
type FrameDecoder func(payload []byte) ([]DecodedFrame, error)

// frameDecoders is the registry mapping DS400Transport → FrameDecoder.
// A nil entry means "no decoder for this transport"; a non-nil entry
// indicates the operator has linked a build with a real decoder.
//
// The map is intentionally process-global: a single binary serves a
// single device family. Test code that needs a custom decoder registers
// + unregisters around the test (see ds400_source_test.go).
//
// Concurrent access is guarded by frameDecodersMu. The mutex was added
// 2026-05-13 alongside the Stage C-lite race-check gate (CLAUDE.md);
// pre-existing tests under `go test -race` race on this map when run
// in parallel.
var (
	frameDecodersMu sync.RWMutex
	frameDecoders   = map[DS400Transport]FrameDecoder{}
)

// RegisterFrameDecoder installs a decoder for one transport. Returns
// the previously-registered decoder, if any, so tests can restore state
// in a deferred call. Subsequent calls with the same transport replace
// the prior decoder.
//
// Production swap-in for a real DS-400 deployment:
//
//	package main
//
//	func init() {
//	    remoteid.RegisterFrameDecoder(remoteid.DS400TransportUDP, decodeDS400UDP)
//	}
//
// where decodeDS400UDP lives in a separate file alongside
// ds400_source.go.
func RegisterFrameDecoder(t DS400Transport, decoder FrameDecoder) FrameDecoder {
	frameDecodersMu.Lock()
	defer frameDecodersMu.Unlock()
	prev := frameDecoders[t]
	if decoder == nil {
		delete(frameDecoders, t)
	} else {
		frameDecoders[t] = decoder
	}
	return prev
}

// LookupFrameDecoder returns the currently-registered decoder for the
// transport, or nil if none. Mostly useful for diagnostics; callers
// inside RunDS400 just consult the map directly.
func LookupFrameDecoder(t DS400Transport) FrameDecoder {
	frameDecodersMu.RLock()
	defer frameDecodersMu.RUnlock()
	return frameDecoders[t]
}

// lookupFrameDecoder is the internal, lock-acquiring read used by
// RunDS400. Exported LookupFrameDecoder also routes through this.
func lookupFrameDecoder(t DS400Transport) (FrameDecoder, bool) {
	frameDecodersMu.RLock()
	defer frameDecodersMu.RUnlock()
	d, ok := frameDecoders[t]
	return d, ok
}

// RunDS400 is the FeedSource-compatible entry point for the DroneScout
// DS-400 receiver. The signature matches RunReplay / RunSynth so the
// DApp seller plugs in a DS-400 source the same way it plugs in a
// fixture replay:
//
//	source := func(ctx context.Context, out chan<- DecodedFrame) error {
//	    return remoteid.RunDS400(ctx, cfg, out)
//	}
//
// Behavior when ErrDS400Unavailable applies:
//
//   - The function returns the error IMMEDIATELY, with no frames
//     produced. The caller is expected to fall back to a Phase-2
//     source (--synth or --replay).
//   - The error is structured so callers can detect via errors.Is and
//     branch their fallback logic cleanly.
//
// Behavior once a decoder is registered (future):
//
//   - UDP: opens a packet socket on cfg.Address, reads datagrams in a
//     loop, calls decoder per packet, emits DecodedFrames.
//   - TCP: dials cfg.Address, reads framed records (frame discipline
//     TBD per vendor docs), calls decoder.
//   - HTTP: long-polls cfg.Address; the decoder receives one response
//     body per poll iteration.
//
// The real implementations land alongside this file once vendor docs
// resolve.
func RunDS400(ctx context.Context, cfg DS400Config, out chan<- DecodedFrame) error {
	if err := cfg.validate(); err != nil {
		return err
	}

	decoder, ok := lookupFrameDecoder(cfg.Transport)
	if !ok || decoder == nil {
		return fmt.Errorf("%w (transport=%s address=%s; register a FrameDecoder via remoteid.RegisterFrameDecoder before calling RunDS400)",
			ErrDS400Unavailable, cfg.Transport, cfg.Address)
	}

	// Decoder is registered → delegate to the transport-specific reader.
	// Phase 2 ships no readers; the placeholder branches below are kept
	// as a single point of update for the future implementation. Each
	// branch is expected to:
	//
	//   - open the transport,
	//   - read inbound payloads until ctx is cancelled or the transport
	//     is exhausted,
	//   - call decoder(payload) per payload,
	//   - emit returned frames on out,
	//   - respect ctx.Done() at every select.
	switch cfg.Transport {
	case DS400TransportUDP:
		return runDS400UDP(ctx, cfg, decoder, out)
	case DS400TransportTCP:
		return runDS400TCP(ctx, cfg, decoder, out)
	case DS400TransportHTTP:
		return runDS400HTTP(ctx, cfg, decoder, out)
	default:
		return fmt.Errorf("feeds/remoteid: unknown DS-400 transport %q", cfg.Transport)
	}
}

// validate enforces the minimum invariants on DS400Config.
func (c *DS400Config) validate() error {
	switch c.Transport {
	case DS400TransportUDP, DS400TransportTCP, DS400TransportHTTP:
		// ok
	case "":
		return errors.New("feeds/remoteid: DS400Config.Transport is required")
	default:
		return fmt.Errorf("feeds/remoteid: unknown DS-400 transport %q (want udp|tcp|http)", c.Transport)
	}
	if c.Address == "" {
		return errors.New("feeds/remoteid: DS400Config.Address is required")
	}
	return nil
}

// SourceTag returns the DecodedFrame.Source value to stamp on frames
// from this config. Falls back to "dronescout-ds400" when cfg.Source is
// empty.
func (c *DS400Config) SourceTag() string {
	if c.Source != "" {
		return c.Source
	}
	return "dronescout-ds400"
}

// Per-transport readers — STUBBED. Each returns ErrDS400Unavailable
// even when a decoder is registered, because the network-read loops
// themselves are not implemented yet. Splitting them out keeps the
// future swap-in surgical: each runner gets implemented once we know
// the actual wire details (probably one at a time as vendor docs
// confirm).
//
// Test code that wants to exercise the decode path bypasses these
// functions and calls the FrameDecoder directly.

func runDS400UDP(_ context.Context, cfg DS400Config, _ FrameDecoder, _ chan<- DecodedFrame) error {
	return fmt.Errorf("%w (transport=udp address=%s: network read loop not yet implemented; DS-400 capture fixtures required to unblock)",
		ErrDS400Unavailable, cfg.Address)
}

func runDS400TCP(_ context.Context, cfg DS400Config, _ FrameDecoder, _ chan<- DecodedFrame) error {
	return fmt.Errorf("%w (transport=tcp address=%s: network read loop not yet implemented)",
		ErrDS400Unavailable, cfg.Address)
}

func runDS400HTTP(_ context.Context, cfg DS400Config, _ FrameDecoder, _ chan<- DecodedFrame) error {
	return fmt.Errorf("%w (transport=http address=%s: network read loop not yet implemented)",
		ErrDS400Unavailable, cfg.Address)
}
