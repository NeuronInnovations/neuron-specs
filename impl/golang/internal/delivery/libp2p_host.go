package delivery

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	ma "github.com/multiformats/go-multiaddr"
)

// HostOption configures a libp2p host.
type HostOption func(*hostConfig)

type hostConfig struct {
	enableRelay        bool
	enableNAT          bool
	staticRelays       []string
	forcedReachability *network.Reachability
	connMgr            *connMgrConfig
}

type connMgrConfig struct {
	low, high int
	grace     time.Duration
}

// ActiveStreamProtectTag is the tag name passed to the libp2p ConnManager's
// Protect/Unprotect methods around an open delivery channel. Surfacing it as
// an exported constant lets the adapter and any future direct callers use the
// same string, so a peer protected by Connect is correctly unprotected by
// Disconnect.
const ActiveStreamProtectTag = "neuron-active-stream"

// WithRelay enables the libp2p NAT traversal stack: relay client, AutoNAT v2,
// hole punching, UPnP, and — if static relay multiaddrs are provided — autorelay
// with those static relays. FR-D18, FR-D20, FR-D21.
//
// Each staticRelayAddr must be a full multiaddr ending in /p2p/<RELAY_ID>.
// Passing an empty list enables the relay client, AutoNAT v2, hole punching, and
// UPnP without autorelay (useful for tests that connect to relays manually).
func WithRelay(staticRelayAddrs ...string) HostOption {
	return func(c *hostConfig) {
		c.enableRelay = true
		c.staticRelays = staticRelayAddrs
	}
}

// WithAutoNAT enables UPnP + AutoNAT v2 + hole punching for hosts that do not
// need autorelay (e.g., publicly-reachable nodes that still want to probe their
// own reachability). FR-D19.
func WithAutoNAT() HostOption {
	return func(c *hostConfig) {
		c.enableNAT = true
	}
}

// WithForcedReachability forces the host's reachability state, bypassing
// AutoNAT probing. Intended for tests and for nodes with known-public IPs.
// Pass network.ReachabilityPublic or network.ReachabilityPrivate.
func WithForcedReachability(r network.Reachability) HostOption {
	return func(c *hostConfig) {
		c.forcedReachability = &r
	}
}

// WithConnManager configures libp2p's connection manager with custom
// watermarks and a grace period. When the connection count crosses the high
// watermark, the connection manager prunes back to the low watermark; the
// grace period is the minimum time a fresh connection is exempt from being
// trimmed.
//
// The connection manager honors any peer marked with Protect (see
// ActiveStreamProtectTag); the adapter calls Protect on every stream open
// and Unprotect on every stream close so that streams never get pruned out
// from under active deliveries — a failure mode previously observed on a
// production seller under connection-burst load.
//
// Recommended starting values for an edge-seller running side-by-side with a
// production seller on a 499 MB-RAM ARMv7 device: low=320, high=384, grace=90s.
// These are higher than libp2p's defaults (160/192/20s) to absorb fan-out
// from a multi-buyer aggregation, and the grace period is long enough that a
// freshly-dialed buyer survives until its first stream protects the conn.
func WithConnManager(low, high int, grace time.Duration) HostOption {
	return func(c *hostConfig) {
		c.connMgr = &connMgrConfig{low: low, high: high, grace: grace}
	}
}

// NewLibp2pHost creates a libp2p host configured for Neuron delivery.
// DD-D02: Host is created per-agent and shared across delivery channels.
// FR-D25: QUIC transport enabled. FR-D27: transport-layer encryption via TLS 1.3.
// FR-D18-D21: Optional relay, AutoNAT v2, hole punching, and autorelay.
//
// The privKey is the agent's secp256k1 private key (same as NeuronPrivateKey).
// The listenAddr is a multiaddr string (e.g., "/ip4/127.0.0.1/udp/0/quic-v1").
func NewLibp2pHost(privKey *ecdsa.PrivateKey, listenAddr string, opts ...HostOption) (host.Host, error) {
	cfg := &hostConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	libp2pPriv, err := convertSecp256k1Key(privKey)
	if err != nil {
		return nil, err
	}

	libp2pOpts := []libp2p.Option{
		libp2p.Identity(libp2pPriv),
		libp2p.ListenAddrStrings(listenAddr),
	}

	if cfg.connMgr != nil {
		mgr, err := connmgr.NewConnManager(cfg.connMgr.low, cfg.connMgr.high,
			connmgr.WithGracePeriod(cfg.connMgr.grace))
		if err != nil {
			return nil, fmt.Errorf("delivery: build conn manager: %w", err)
		}
		libp2pOpts = append(libp2pOpts, libp2p.ConnectionManager(mgr))
	}

	if cfg.forcedReachability != nil {
		switch *cfg.forcedReachability {
		case network.ReachabilityPublic:
			libp2pOpts = append(libp2pOpts, libp2p.ForceReachabilityPublic())
		case network.ReachabilityPrivate:
			libp2pOpts = append(libp2pOpts, libp2p.ForceReachabilityPrivate())
		}
	}

	switch {
	case cfg.enableRelay:
		// FR-D18, FR-D20, FR-D21: full NAT traversal stack.
		libp2pOpts = append(libp2pOpts,
			libp2p.EnableRelay(),
			libp2p.EnableAutoNATv2(),
			libp2p.EnableHolePunching(),
			libp2p.NATPortMap(),
		)
		if len(cfg.staticRelays) > 0 {
			addrInfos, err := parseStaticRelays(cfg.staticRelays)
			if err != nil {
				return nil, err
			}
			libp2pOpts = append(libp2pOpts, libp2p.EnableAutoRelayWithStaticRelays(
				addrInfos,
				autorelay.WithNumRelays(len(addrInfos)),
				autorelay.WithMinCandidates(1),
			))
		}
	case cfg.enableNAT:
		libp2pOpts = append(libp2pOpts,
			libp2p.NATPortMap(),
			libp2p.EnableAutoNATv2(),
			libp2p.EnableHolePunching(),
		)
	}

	h, err := libp2p.New(libp2pOpts...)
	if err != nil {
		return nil, fmt.Errorf("delivery: create libp2p host: %w", err)
	}

	return h, nil
}

func convertSecp256k1Key(privKey *ecdsa.PrivateKey) (libp2pcrypto.PrivKey, error) {
	privKeyBytes := privKey.D.Bytes()
	if len(privKeyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(privKeyBytes):], privKeyBytes)
		privKeyBytes = padded
	}
	p, err := libp2pcrypto.UnmarshalSecp256k1PrivateKey(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("delivery: convert secp256k1 key to libp2p: %w", err)
	}
	return p, nil
}

// parseStaticRelays converts a list of `/p2p/<RELAY_ID>`-terminated multiaddr
// strings into peer.AddrInfo records for autorelay. Returns an error if any
// entry cannot be parsed — silent drops would hide misconfiguration.
func parseStaticRelays(addrs []string) ([]peer.AddrInfo, error) {
	infos := make([]peer.AddrInfo, 0, len(addrs))
	for _, a := range addrs {
		m, err := ma.NewMultiaddr(a)
		if err != nil {
			return nil, fmt.Errorf("delivery: parse relay multiaddr %q: %w", a, err)
		}
		info, err := peer.AddrInfoFromP2pAddr(m)
		if err != nil {
			return nil, fmt.Errorf("delivery: extract peer info from relay %q: %w", a, err)
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

// ReachabilityTracker subscribes to libp2p's EvtLocalReachabilityChanged event
// bus and caches the most recent reachability value. This is the canonical way
// to read AutoNAT v2 results for populating natStatus in connectionSetup
// (FR-D19). A new tracker must be Close()d to release its event-bus subscription.
type ReachabilityTracker struct {
	mu           sync.RWMutex
	reachability network.Reachability
	cancel       context.CancelFunc
	done         chan struct{}
}

// NewReachabilityTracker starts a goroutine that consumes reachability events
// from the host's event bus. The tracker stops when Close is called.
func NewReachabilityTracker(h host.Host) (*ReachabilityTracker, error) {
	sub, err := h.EventBus().Subscribe(new(event.EvtLocalReachabilityChanged))
	if err != nil {
		return nil, fmt.Errorf("delivery: subscribe reachability events: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	rt := &ReachabilityTracker{
		reachability: network.ReachabilityUnknown,
		cancel:       cancel,
		done:         make(chan struct{}),
	}

	go func() {
		defer close(rt.done)
		defer sub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-sub.Out():
				if !ok {
					return
				}
				evt, ok := e.(event.EvtLocalReachabilityChanged)
				if !ok {
					continue
				}
				rt.mu.Lock()
				rt.reachability = evt.Reachability
				rt.mu.Unlock()
			}
		}
	}()

	return rt, nil
}

// Status returns the current reachability as "public", "private", or "unknown".
func (rt *ReachabilityTracker) Status() string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	switch rt.reachability {
	case network.ReachabilityPublic:
		return "public"
	case network.ReachabilityPrivate:
		return "private"
	default:
		return "unknown"
	}
}

// Close stops the tracker goroutine and releases its event-bus subscription.
func (rt *ReachabilityTracker) Close() {
	rt.cancel()
	<-rt.done
}

// NATStatus returns the host's current reachability based on its listen
// addresses. This is a cheap static check suitable for callers that do not
// keep a ReachabilityTracker; it returns "public" if any non-loopback
// listen address exists and "unknown" otherwise. FR-D19 compliance requires
// the live tracker for accuracy.
func NATStatus(h host.Host) string {
	for _, addr := range h.Addrs() {
		addrStr := addr.String()
		if !contains(addrStr, "/127.0.0.1/") && !contains(addrStr, "/::1/") {
			return "public"
		}
	}
	return "unknown"
}

// WaitForRelayReservation blocks until the host advertises at least one
// /p2p-circuit multiaddr (autorelay secured a relay reservation) or ctx is
// cancelled. Returns nil on success, ctx.Err() on timeout.
//
// The buyer-seller demo's seller path must call this before building
// connectionSetup when --relay is configured, because BuildConnectionSetup
// captures host.Addrs() at that moment. Without the wait, the first
// connectionSetup may omit relay-assisted addresses.
func WaitForRelayReservation(ctx context.Context, h host.Host) error {
	if hasCircuitAddr(h) {
		return nil
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if hasCircuitAddr(h) {
				return nil
			}
		}
	}
}

func hasCircuitAddr(h host.Host) bool {
	for _, a := range h.Addrs() {
		if strings.Contains(a.String(), "/p2p-circuit") {
			return true
		}
	}
	return false
}
