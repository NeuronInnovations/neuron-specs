package adsb

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/libp2p/go-libp2p/core/host"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	sbs "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
)

// FeedSource is the producer signature for ADS-B BaseStation records.
// Implementations emit sbs.SBSTrack values on out until ctx is cancelled
// or the source is exhausted, then return. They MUST NOT close out.
//
// internal/feeds/sbs provides three ready-made FeedSource implementations:
//   - sbs.RunBaseStationTCP (read-only TCP dial; e.g. JV port 30003)
//   - sbs.RunBaseStationReplay (capture-file replay)
//   - sbs.RunBaseStationSynth (deterministic synthetic aircraft)
//
// Adapter wrappers are below in this file.
type FeedSource func(ctx context.Context, out chan<- sbs.SBSTrack) error

// SellerConfig parameterizes Start.
type SellerConfig struct {
	Host       host.Host
	Source     FeedSource
	Logger     *log.Logger
	ProtocolID string // override; defaults to ProtocolBaseStation
}

// SellerRunningContext exposes the controls a caller needs after Start.
type SellerRunningContext struct {
	Protocol string
	Cancel   context.CancelFunc
	Done     <-chan struct{}
}

// Start registers the configured stream handler on cfg.Host. Mirrors
// internal/dapp/remoteid.Start with the SBS → NormalizedTrack conversion
// substituted into serveStream. The host is NOT closed by Start.
func Start(ctx context.Context, cfg SellerConfig) (*SellerRunningContext, error) {
	if cfg.Host == nil {
		return nil, errors.New("adsb.Start: SellerConfig.Host is required")
	}
	if cfg.Source == nil {
		return nil, errors.New("adsb.Start: SellerConfig.Source is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	proto := cfg.ProtocolID
	if proto == "" {
		proto = ProtocolBaseStation
	}

	streamCtx, cancel := context.WithCancel(ctx)
	doneCh := make(chan struct{})

	var activeStreams = make(chan struct{}, 64)

	cfg.Host.SetStreamHandler(protocol.ID(proto), func(stream libp2pnetwork.Stream) {
		select {
		case activeStreams <- struct{}{}:
		default:
			logger.Printf("[adsb-seller] refusing stream: too many active (limit=%d)", cap(activeStreams))
			_ = stream.Reset()
			return
		}
		defer func() { <-activeStreams }()

		serveStream(streamCtx, stream, cfg.Source, logger)
	})

	go func() {
		<-streamCtx.Done()
		cfg.Host.RemoveStreamHandler(protocol.ID(proto))
		for i := 0; i < cap(activeStreams); i++ {
			activeStreams <- struct{}{}
		}
		close(doneCh)
	}()

	logger.Printf("[adsb-seller] listening on protocol=%s peerID=%s addrs=%v",
		proto, cfg.Host.ID(), cfg.Host.Addrs())

	return &SellerRunningContext{
		Protocol: proto,
		Cancel:   cancel,
		Done:     doneCh,
	}, nil
}

// serveStream pumps SBSTrack records from the feed source through
// FromSBSTrack → NormalizedTrack canonical JSON onto the libp2p stream.
func serveStream(
	ctx context.Context,
	stream libp2pnetwork.Stream,
	source FeedSource,
	logger *log.Logger,
) {
	remotePeer := stream.Conn().RemotePeer()
	logger.Printf("[adsb-seller] stream open peer=%s", remotePeer)

	defer func() {
		_ = stream.Close()
		logger.Printf("[adsb-seller] stream closed peer=%s", remotePeer)
	}()

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tracks := make(chan sbs.SBSTrack, 128)
	srcErr := make(chan error, 1)
	go func() {
		srcErr <- source(streamCtx, tracks)
		close(tracks)
	}()

	writer := delivery.NewFrameWriter(stream)
	merger := newTrackMerger(defaultMergeCap, defaultMergeTTL)
	var sent uint64

	for track := range tracks {
		// Reconstruct a complete track by merging the latest position (MSG
		// type 3) and velocity (MSG type 4) for each ICAO — vanilla SBS-1
		// splits these across separate records. The merger is goroutine-local
		// to this pump loop, so it needs no mutex (race-free by construction).
		merged := merger.merge(track)

		// Hold a record until a position is known: a velocity-only (type-4)
		// record arriving before any position would otherwise serialize a
		// position-less NormalizedTrack that the display renders at (0,0). The
		// velocity stays cached and surfaces on the next positioned record.
		if !merged.LatSet || !merged.LonSet {
			continue
		}

		nt := FromSBSTrack(merged)
		canonical, err := json.Marshal(nt)
		if err != nil {
			// Drop a single malformed frame (e.g. missing ICAO); FR-A09
			// keeps the stream open per long-lived discipline.
			logger.Printf("[adsb-seller] marshal error peer=%s: %v", remotePeer, err)
			continue
		}

		if err := writer.WriteFrame(canonical); err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Printf("[adsb-seller] write error peer=%s sent=%d: %v",
					remotePeer, sent, err)
			}
			cancel()
			break
		}
		sent++
	}

	if err := <-srcErr; err != nil && !errors.Is(err, context.Canceled) {
		logger.Printf("[adsb-seller] source returned peer=%s sent=%d: %v",
			remotePeer, sent, err)
	} else {
		logger.Printf("[adsb-seller] source drained peer=%s sent=%d", remotePeer, sent)
	}
}

// BaseStationTCPSource wraps sbs.RunBaseStationTCP into a FeedSource bound
// to a fixed host:port. The host:port is typically "127.0.0.1:30003" for
// the JetVision Air!Squitter, or the value of BlueMark's sbs_server_port
// for a DroneScout-fed BaseStation seller.
func BaseStationTCPSource(hostPort string) FeedSource {
	return func(ctx context.Context, out chan<- sbs.SBSTrack) error {
		return sbs.RunBaseStationTCP(ctx, hostPort, out)
	}
}

// BaseStationReplaySource wraps sbs.RunBaseStationReplay with bound options.
func BaseStationReplaySource(path string, opts sbs.ReplayOptions) FeedSource {
	return func(ctx context.Context, out chan<- sbs.SBSTrack) error {
		return sbs.RunBaseStationReplay(ctx, path, opts, out)
	}
}

// BaseStationSynthSource wraps sbs.RunBaseStationSynth with bound options.
func BaseStationSynthSource(opts sbs.SynthOptions) FeedSource {
	return func(ctx context.Context, out chan<- sbs.SBSTrack) error {
		return sbs.RunBaseStationSynth(ctx, opts, out)
	}
}
