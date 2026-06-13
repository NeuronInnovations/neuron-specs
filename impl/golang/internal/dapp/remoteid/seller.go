package remoteid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds/remoteid"
)

// FeedSource is the producer signature for Remote ID frames.
//
// Implementations emit DecodedFrames on out until ctx is cancelled or the
// source is exhausted, then return. They MUST NOT close out — the seller
// closes it when no more buyers will consume.
//
// internal/feeds/remoteid provides two ready-made FeedSource implementations:
//
//   - remoteid.RunReplay (fixture-driven)
//   - remoteid.RunSynth (synthetic-orbit drones)
//
// Adapter wrappers are below in this file.
type FeedSource func(ctx context.Context, out chan<- remoteid.DecodedFrame) error

// SellerConfig parameterizes Run. All fields are required unless noted.
type SellerConfig struct {
	// Host is the libp2p host the seller listens on. Caller is responsible
	// for construction (typically via delivery.NewLibp2pHost) and for
	// closing the host on shutdown — the seller does NOT close it.
	Host host.Host

	// Source is the per-stream feed source factory. When a buyer opens
	// the raw stream, the seller calls Source(ctx, out) on a fresh
	// goroutine; the goroutine returns when ctx is cancelled or Source
	// itself returns.
	Source FeedSource

	// Logger receives operational logs. If nil, log.Default() is used.
	Logger *log.Logger

	// ProtocolID overrides the protocol the seller registers. Defaults
	// to ProtocolRaw ("/ds240/raw/1.0.0"). The override is provided
	// for tests that want to isolate protocol-handler routing without
	// real libp2p multistream-select interactions.
	//
	// When ProtocolIDs is non-empty, ProtocolID is ignored.
	ProtocolID string

	// ProtocolIDs is the list of libp2p protocol IDs the seller
	// registers handlers for. Each handler runs serveStream — the
	// frames pumped on each are byte-identical (the protocol-id is
	// purely a discovery / negotiation hint).
	//
	// When non-empty, ProtocolIDs takes precedence over ProtocolID.
	// When empty, behaviour reverts to the single-protocol path
	// driven by ProtocolID (or ProtocolRaw when ProtocolID is empty
	// too). Plan §"Step 5 — Multi-protocol seller handler".
	ProtocolIDs []string
}

// SellerRunningContext is returned by Start and exposes the controls a
// caller needs to wait for an active stream, request shutdown, or
// observe the registered protocol.
type SellerRunningContext struct {
	// Protocol is the libp2p protocol the seller is currently handling.
	Protocol string

	// Cancel stops accepting new streams and signals all in-flight
	// stream goroutines to return. Idempotent.
	Cancel context.CancelFunc

	// Done is closed after all active stream goroutines have exited
	// following a Cancel. Callers waiting for graceful shutdown should
	// select on this channel.
	Done <-chan struct{}
}

// Start registers the configured stream handler on cfg.Host and returns
// immediately. The returned SellerRunningContext lets the caller cancel
// + wait for graceful shutdown. The host is NOT closed by Start; the
// caller owns that.
//
// Per 008 FR-P45/P46 long-lived discipline: the seller does NOT
// proactively close accepted streams. A stream ends when (a) the buyer
// closes it (graceful EOF), (b) the underlying transport faults, or
// (c) the caller invokes ctx.Cancel. The seller does NOT close on
// resource pressure, transient transport faults, or control-plane
// unavailability.
//
// Per 009 FR-D-multi-protocol the seller MAY register additional protocol
// IDs (filtered/* + status), but Phase 2 of the reference MVP only wires the
// raw stream. Additional handlers land alongside Phase 6 fan-out work.
func Start(ctx context.Context, cfg SellerConfig) (*SellerRunningContext, error) {
	if cfg.Host == nil {
		return nil, errors.New("remoteid.Start: SellerConfig.Host is required")
	}
	if cfg.Source == nil {
		return nil, errors.New("remoteid.Start: SellerConfig.Source is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	// ProtocolIDs takes precedence over ProtocolID; both default to
	// [ProtocolRaw] when empty. The first entry of the resolved list
	// becomes SellerRunningContext.Protocol for back-compat with
	// callers that only know about the single-protocol API.
	protos := cfg.ProtocolIDs
	if len(protos) == 0 {
		proto := cfg.ProtocolID
		if proto == "" {
			proto = ProtocolRaw
		}
		protos = []string{proto}
	}

	streamCtx, cancel := context.WithCancel(ctx)
	doneCh := make(chan struct{})

	// Use a simple counter + channel to track in-flight stream handlers
	// so Done closes after they all exit. The capacity is shared
	// across ALL registered protocols — i.e. the bound is total
	// concurrent buyers across both /ds240/raw and
	// /ds240/basestation, not per-protocol.
	var activeStreams = make(chan struct{}, 64) // bounded; backpressure on > 64 concurrent buyers

	for _, p := range protos {
		cfg.Host.SetStreamHandler(protocol.ID(p), func(stream libp2pnetwork.Stream) {
			select {
			case activeStreams <- struct{}{}:
			default:
				// Too many concurrent streams. Refuse the new one cleanly
				// (per FR-P46 we should NOT close ACTIVE streams; this
				// rejects a NEW one before it's ACTIVE).
				logger.Printf("[remoteid-seller] refusing stream: too many active (limit=%d)", cap(activeStreams))
				_ = stream.Reset()
				return
			}
			defer func() { <-activeStreams }()

			serveStream(streamCtx, stream, cfg.Source, logger)
		})
	}

	go func() {
		<-streamCtx.Done()
		for _, p := range protos {
			cfg.Host.RemoveStreamHandler(protocol.ID(p))
		}
		// Drain in-flight streams.
		for i := 0; i < cap(activeStreams); i++ {
			activeStreams <- struct{}{}
		}
		close(doneCh)
	}()

	logger.Printf("[remoteid-seller] listening on protocols=%v peerID=%s addrs=%v",
		protos, cfg.Host.ID(), cfg.Host.Addrs())

	return &SellerRunningContext{
		Protocol: protos[0],
		Cancel:   cancel,
		Done:     doneCh,
	}, nil
}

// serveStream is the per-stream goroutine that pumps frames from the
// feed source through canonical JSON encoding onto the wire.
//
// The pump structure mirrors the existing edge-seller pattern (feed →
// inner encode goroutine → SendStream-like loop) but with two
// simplifications justified by the Phase 2 scope:
//
//   - The seller writes directly to the libp2p stream via a
//     delivery.FrameWriter rather than going through delivery.SendStream.
//     SendStream is a goroutine-based helper that handles ctx and a
//     buffered channel; here we already have a per-stream goroutine and
//     can keep the data path simpler.
//   - There is no AgreementPeriod timer — Phase 2 is data-plane only.
//     Lifecycle (serviceStop) wires in at Phase 5+ when commerce lands.
func serveStream(
	ctx context.Context,
	stream libp2pnetwork.Stream,
	source FeedSource,
	logger *log.Logger,
) {
	remotePeer := stream.Conn().RemotePeer()
	logger.Printf("[remoteid-seller] stream open peer=%s", remotePeer)

	defer func() {
		_ = stream.Close()
		logger.Printf("[remoteid-seller] stream closed peer=%s", remotePeer)
	}()

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	frames := make(chan remoteid.DecodedFrame, 128)
	srcErr := make(chan error, 1)
	go func() {
		srcErr <- source(streamCtx, frames)
		close(frames)
	}()

	writer := delivery.NewFrameWriter(stream)
	var sent uint64

	for frame := range frames {
		canonical, err := json.Marshal(FromDecoded(frame))
		if err != nil {
			// Marshal-failure on a single frame must NOT close the stream
			// (FR-P46: transient faults are recoverable). Skip the frame
			// and continue.
			logger.Printf("[remoteid-seller] marshal error peer=%s: %v", remotePeer, err)
			continue
		}

		if err := writer.WriteFrame(canonical); err != nil {
			// Stream-write failure typically means the buyer closed; not
			// a seller-side fault. Log and exit the pump.
			if !errors.Is(err, context.Canceled) {
				logger.Printf("[remoteid-seller] write error peer=%s sent=%d: %v",
					remotePeer, sent, err)
			}
			cancel()
			break
		}
		sent++
	}

	// Drain source result.
	if err := <-srcErr; err != nil && !errors.Is(err, context.Canceled) {
		logger.Printf("[remoteid-seller] source returned peer=%s sent=%d: %v",
			remotePeer, sent, err)
	} else {
		logger.Printf("[remoteid-seller] source drained peer=%s sent=%d", remotePeer, sent)
	}
}

// ReplaySource wraps remoteid.RunReplay so it satisfies the FeedSource
// signature with a fixed path and options bound at construction time.
func ReplaySource(path string, opts remoteid.ReplayOptions) FeedSource {
	return func(ctx context.Context, out chan<- remoteid.DecodedFrame) error {
		return remoteid.RunReplay(ctx, path, opts, out)
	}
}

// SynthSource wraps remoteid.RunSynth with fixed options.
func SynthSource(opts remoteid.SynthOptions) FeedSource {
	return func(ctx context.Context, out chan<- remoteid.DecodedFrame) error {
		return remoteid.RunSynth(ctx, opts, out)
	}
}

// Verify Start contract at compile time.
var _ = fmt.Sprintf
