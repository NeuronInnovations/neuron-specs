package edgeapp

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// FeedSource produces FeedFrames on out until ctx is cancelled.
//
// Sources in internal/feeds (RunBeastTCP, RunBeastReplay, RunSynth) all match
// this signature once their other parameters are bound. The seller config
// supplies one wrapped to its specific source; this keeps edgeapp ignorant
// of source-specific configuration knobs.
type FeedSource func(ctx context.Context, out chan<- feeds.FeedFrame) error

// Logger is the minimal logging surface edgeapp uses. *log.Logger satisfies
// it, as does any custom adapter. Pass nil to suppress all logging.
type Logger interface {
	Printf(format string, a ...any)
}

// SellerConfig configures RunSeller.
type SellerConfig struct {
	// Bus is the topic.TopicAdapter the seller publishes heartbeats to and
	// observes ReverseConnectionSetup messages on. MemoryBus or HCSAdapter.
	Bus topic.TopicAdapter

	// PrivateKey is the seller's secp256k1 private key (signs all topic
	// messages, derives PeerID + EVM address).
	PrivateKey *keylib.NeuronPrivateKey

	// Topics are the seller's stdIn/stdOut/stdErr topic refs. If any field
	// is the zero value, RunSeller will create the topic on Bus before
	// proceeding. Pre-created topics are required when both seller and
	// buyer must agree on a TopicRef before either binary starts.
	StdIn  topic.TopicRef
	StdOut topic.TopicRef
	StdErr topic.TopicRef

	// LibP2PListenAddr is the multiaddr the seller's libp2p host listens on.
	// "/ip4/0.0.0.0/udp/0/quic-v1" is fine for a NAT'd seller — the OS picks
	// a port and libp2p discovers it via NAT-traversal logic.
	LibP2PListenAddr string

	// Protocol is the libp2p stream protocol ID used between seller and
	// buyer. Must match BuyerConfig.Protocol.
	Protocol string

	// HeartbeatPeriod is the cadence at which the seller publishes a
	// HeartbeatPayload to its own stdOut. Must satisfy
	// health.MinDeadlineDelta (10s) <= HeartbeatPeriod (recommended 60s).
	HeartbeatPeriod time.Duration

	// HeartbeatLocation, when non-nil, is included in every heartbeat.
	HeartbeatLocation *health.Location

	// FeedSource produces frames the seller forwards to the buyer.
	FeedSource FeedSource

	// StatePath, when non-empty, opts the seller into persistent identity:
	// HCS topic IDs are loaded from this file on startup and reused across
	// restarts (no new CreateTopic calls), then re-persisted on graceful
	// shutdown. The default (empty) preserves Phase C.2 behavior — fresh
	// topics every run.
	//
	// File format and atomic-write semantics are documented on EdgeState.
	// Schema-version mismatch, identity mismatch (key rotation), or any
	// parse / locator-validation failure all fall back to fresh-create
	// with no error to the caller. Callers that need to *force* fresh
	// topics should clear or delete the file before launch.
	StatePath string

	// PublishProfileDescriptor, when true, publishes a spec-013 Profile E
	// descriptor on stdOut at startup. Re-publish is gated on hash change
	// against EdgeState.ProfileDescriptorHash. Default false (Phase C.2:
	// no descriptor on stdOut; consumers fall back to seller-bootstrap.json).
	PublishProfileDescriptor bool

	// Registry, when non-nil, opts the seller into spec-003 / EIP-8004
	// registration: at startup, EnsureRegistered is called with a small
	// agentURI describing the seller's services + topic refs. Default nil
	// (skip).
	//
	// MemoryRegistry / NewDisabledRegistry both satisfy this interface.
	// Iteration 2 ships only those two; the EVM-backed adapter lands in
	// iteration 3+.
	Registry RegistryAdapter

	// Escrow, when non-nil, opts the seller into the spec-008 commerce
	// gate. Before accepting a ReverseConnectionSetup, the seller waits
	// for a ServiceRequest on stdIn, accepts it, and waits for an
	// EscrowCreated payload. Only then does it process the dial-in.
	// On graceful close it issues an Invoice and waits for InvoiceAck.
	//
	// payment.MemoryEscrow is the only adapter wired in iteration 3 mock
	// mode. Default nil (skip the gate).
	Escrow payment.EscrowAdapter

	// SellerEVM is the seller's EVM address (0x... 40 hex). Required when
	// Escrow != nil so RequestRelease can declare the recipient. When
	// empty, RunSeller derives it from PrivateKey.
	SellerEVM string

	// AgreementPeriod, when > 0, opts the seller into period-driven
	// settlement: every AgreementPeriod the active stream is gracefully
	// closed, the existing Invoice/InvoiceAck cycle runs, and the seller
	// loops back to the commerce gate to negotiate a fresh agreement.
	// When 0 (default), behavior is the legacy single-shot flow — one
	// agreement per process; settlement only on SIGINT.
	//
	// Recommended values: 5–10 min for demos, ≥ 1 h for production.
	// Below 1 min the per-agreement gas overhead dominates throughput.
	//
	// Long-lived discipline (008 FR-P45/P46): a non-zero AgreementPeriod
	// is a CONTRACTUAL DURATION configured by the operator, NOT a
	// resource-pressure or transient-fault response. With the new
	// lifecycle messages (008 FR-P36 serviceStop / FR-P37 serviceCancel
	// / FR-P38 serviceRenew added 2026-05-08), buyers can drive stop
	// explicitly over the wire; AgreementPeriod remains the LEGACY
	// auto-stop path for deployments that pre-date wire-level stop
	// signaling. New deployments SHOULD set AgreementPeriod = 0 and rely
	// on buyer-issued serviceStop instead.
	AgreementPeriod time.Duration

	// IdleFundedTimeout, when > 0, bounds how long the seller waits for a
	// ReverseConnectionSetup after observing EscrowCreated (state=FUNDED).
	// On expiry the seller abandons the current agreement and loops back
	// to the commerce gate. When 0 (default), the seller waits until the
	// outer ctx cancels — which is the iter-6 deadlock root cause when the
	// buyer crashes mid-handshake.
	//
	// Recommended: 5 min for demos, ≥ 30 min for production. The buyer's
	// retry loop reconnects within seconds of a fresh start; 5 min is a
	// generous upper bound on legitimate setup-publish latency.
	IdleFundedTimeout time.Duration

	// Logger receives info-level lifecycle events. Optional.
	Logger Logger
}

// validate checks the seller config and returns the first problem found.
func (c *SellerConfig) validate() error {
	if c == nil {
		return errors.New("edgeapp.SellerConfig: nil")
	}
	if c.Bus == nil {
		return errors.New("edgeapp.SellerConfig.Bus required")
	}
	if c.PrivateKey == nil {
		return errors.New("edgeapp.SellerConfig.PrivateKey required")
	}
	if c.LibP2PListenAddr == "" {
		c.LibP2PListenAddr = "/ip4/0.0.0.0/udp/0/quic-v1"
	}
	if c.Protocol == "" {
		c.Protocol = DefaultProtocol
	}
	if c.HeartbeatPeriod == 0 {
		c.HeartbeatPeriod = DefaultHeartbeatPeriod
	}
	if c.HeartbeatPeriod < time.Duration(health.MinDeadlineDelta)*time.Second {
		return errors.New("edgeapp.SellerConfig.HeartbeatPeriod must be >= 10s (health.MinDeadlineDelta)")
	}
	if c.FeedSource == nil {
		return errors.New("edgeapp.SellerConfig.FeedSource required")
	}
	return nil
}

// BuyerConfig configures RunBuyer.
type BuyerConfig struct {
	// Bus is the topic.TopicAdapter the buyer publishes heartbeats to and
	// reverse-connection setup messages on.
	Bus topic.TopicAdapter

	// PrivateKey is the buyer's secp256k1 private key.
	PrivateKey *keylib.NeuronPrivateKey

	// Buyer's own topic refs (created on Bus if zero).
	StdIn  topic.TopicRef
	StdOut topic.TopicRef
	StdErr topic.TopicRef

	// StatePath, when non-empty, opts the buyer into persistent identity
	// (symmetric to SellerConfig.StatePath). HCS topic IDs are loaded from
	// this file on startup and reused across restarts; on first run, fresh
	// topics are created and the file is written. Empty (default) preserves
	// pre-iter-7 behavior — fresh topics every restart.
	StatePath string

	// Sellers is the list of sellers the buyer should connect to. RunBuyer
	// publishes a ReverseConnectionSetup to each entry's StdIn topic and
	// accepts an incoming libp2p stream from each. Required when the
	// legacy single-seller fields below are unset.
	Sellers []SellerEntry

	// SellerStdIn / SellerPubKey are the legacy single-seller fields.
	// When Sellers is empty and these are set, RunBuyer treats them as a
	// one-element Sellers slice (with no DisplayName). Set Sellers
	// explicitly for any new code.
	SellerStdIn  topic.TopicRef
	SellerPubKey *ecdsa.PublicKey

	// LibP2PListenAddr is the multiaddr the buyer listens on. Should be a
	// publicly reachable address — sellers dial this. For Demo 1 across
	// loopback or LAN, "/ip4/0.0.0.0/udp/<port>/quic-v1" works.
	LibP2PListenAddr string

	// LibP2PAdvertisedMultiaddrs, when non-empty, overrides what the buyer
	// announces to sellers via ReverseConnectionSetup. Use this when the
	// host's listen addresses don't reflect public reachability (e.g. behind
	// a firewall that forwards a known external port).
	LibP2PAdvertisedMultiaddrs []string

	// Protocol must match every SellerConfig.Protocol.
	Protocol string

	// RequestID correlates the ReverseConnectionSetup with downstream
	// payment / validation messages. For Demo 1 a constant like
	// "edge-feed-001" is fine. Each seller receives a derived requestID
	// (RequestID + "-" + sellerName) so observers can disambiguate.
	RequestID string

	// HeartbeatPeriod, HeartbeatLocation: same semantics as seller side.
	HeartbeatPeriod   time.Duration
	HeartbeatLocation *health.Location

	// ReconnectBackoff is the delay between reconnection attempts when a
	// seller's stream closes. Zero defaults to DefaultReconnectBackoff (10s).
	ReconnectBackoff time.Duration

	// SellerDialTimeout is how long the buyer waits, after publishing its
	// ReverseConnectionSetup, for the seller to dial in before giving up
	// and (depending on context) retrying. Zero defaults to DefaultSellerDialTimeout (60s).
	SellerDialTimeout time.Duration

	// NegotiationTimeout bounds how long the buyer waits for ServiceResponse
	// after publishing ServiceRequest. Iter-6 used SellerDialTimeout (60 s)
	// for this; iter-7 separates the two so a fast NAT'd dial can stay tight
	// while a slow Hashio JSON-RPC negotiation gets a longer fuse. Zero
	// defaults to DefaultBuyerNegotiationTimeout (120 s).
	NegotiationTimeout time.Duration

	// OnAggregatedFrame is the multi-seller aware frame hook. RunBuyer
	// calls it for every received feed-frame, with seller identity and a
	// best-effort Mode-S decoding attached.
	//
	// If OnAggregatedFrame is nil, OnFrame is used (with just feeds.FeedFrame).
	// At least one of OnAggregatedFrame / OnFrame must be set.
	OnAggregatedFrame func(AggregatedFrame)

	// OnFrame is the legacy single-seller frame hook. Receives the raw
	// feeds.FeedFrame only, no seller identity. Used as a fallback when
	// OnAggregatedFrame is nil.
	OnFrame func(feeds.FeedFrame)

	// OnTaggedAdsb is the optional v2-tagged frame hook (Phase 5 — reference
	// MVP fused-buyer / dual-stream display contract). When non-nil,
	// RunBuyer fires this hook for every received frame ALONGSIDE
	// OnAggregatedFrame; the tagged envelope wraps the AggregatedFrame
	// in {source:"adsb", type:"aircraft", sellerPeerID, receivedAt}.
	// Wiring is additive: legacy ADS-B consumers continue to see the
	// untagged AggregatedFrame via OnAggregatedFrame, while a dual-
	// stream display (cmd/fid-display) consumes the tagged variant.
	// See docs/fid-display-contract.md and internal/edgeapp/tagged_adsb.go.
	OnTaggedAdsb func(TaggedAdsbFrame)

	// OnSellerStatus, when non-nil, is called on every seller state
	// transition (connecting, connected, disconnected, error). Optional.
	OnSellerStatus func(SellerStatus)

	// ICAOCache, when non-nil, is shared across every per-seller stream
	// in this buyer. Plaintext ICAOs (DF 11/17/18) are recorded into it;
	// frames whose ICAO field is empty (DF 0/4/5/16/20/21) try parity-XOR
	// recovery against it before being emitted. When nil, RunBuyer
	// allocates a default cache (cap=512, ttl=60s).
	//
	// Pass an explicit nil if you want to disable ICAO recovery (e.g. to
	// preserve a strict chain-of-custody requirement that all attributed
	// ICAOs come from plaintext). To do so, set DisableICAOCache=true.
	ICAOCache *feeds.ICAORecoveryCache

	// DisableICAOCache, when true, skips both recording and recovery.
	// All AggregatedFrame.Meta.Recovered values will be false.
	DisableICAOCache bool

	// EnforceDeadlines opts the buyer into spec-005 deadline enforcement
	// per seller. When true, every per-seller worker subscribes to the
	// seller's stdOut topic in parallel with the data stream and tears
	// down the stream when the seller's evaluated state crosses out of
	// ALIVE (SUSPECT, DEAD, or OFFLINE). Default: false — Phase C.2 behavior
	// (rely solely on libp2p stream death).
	//
	// Requires every SellerEntry.StdOut to be set; missing StdOut on any
	// entry is a config error when this flag is true.
	EnforceDeadlines bool

	// Registry, when non-nil, opts the buyer into spec-003 / EIP-8004
	// registration on startup (mirror of SellerConfig.Registry). Default
	// nil (skip).
	Registry RegistryAdapter

	// Escrow, when non-nil, opts the buyer into the spec-008 commerce
	// gate per seller. For each seller:
	//   1. Before publishing ReverseConnectionSetup, run BuyerNegotiateAndFund.
	//   2. After the data stream ends gracefully, run BuyerSession.Settle.
	// Default nil (skip; legacy reverse-connect path runs as Phase C.2).
	Escrow payment.EscrowAdapter

	// PaymentPriceTinybar / PaymentCurrency / PaymentRequestID parameterize
	// the per-seller agreement when commerce is enabled. PriceTinybar is a
	// decimal string (e.g. "100"); Currency typically "tinybar".
	PaymentPriceTinybar string
	PaymentCurrency     string
	PaymentRequestID    string // optional; defaults to RequestID

	// OnAgreementSettled, when non-nil, is called after each per-seller
	// agreement reaches the post-Settle state (state.Active after release).
	// Useful for tests + the validator to observe completion without
	// subscribing separately.
	OnAgreementSettled func(*BuyerSession)

	// Logger receives info-level lifecycle events. Optional.
	Logger Logger
}

// DefaultReconnectBackoff is the default per-seller delay between
// reconnection attempts when a stream closes.
const DefaultReconnectBackoff = 10 * time.Second

// DefaultSellerDialTimeout is the default per-seller wait for the seller
// to dial in after the buyer publishes ReverseConnectionSetup.
const DefaultSellerDialTimeout = 60 * time.Second

// DefaultBuyerNegotiationTimeout is the default ServiceResponse wait.
// Set to 120 s in iter-7 to absorb Hashio JSON-RPC variance + multi-second
// HCS publish latencies that triggered 4-6 retries during iter-6's
// post-deadlock re-negotiation.
const DefaultBuyerNegotiationTimeout = 120 * time.Second

// DefaultIdleFundedTimeout bounds how long the seller stays in the
// FUNDED state waiting for the buyer's ReverseConnectionSetup. Iter-6
// hit a 12-minute deadlock when the buyer crashed mid-handshake; iter-7
// breaks the agreement after this timeout and accepts a fresh
// ServiceRequest. Zero on SellerConfig means "wait forever" (legacy).
const DefaultIdleFundedTimeout = 5 * time.Minute

// DefaultICAOCacheCap and DefaultICAOCacheTTL govern the auto-allocated
// recovery cache when BuyerConfig.ICAOCache is nil.
const (
	DefaultICAOCacheCap = 512
	DefaultICAOCacheTTL = 60 * time.Second
)

func (c *BuyerConfig) validate() error {
	if c == nil {
		return errors.New("edgeapp.BuyerConfig: nil")
	}
	if c.Bus == nil {
		return errors.New("edgeapp.BuyerConfig.Bus required")
	}
	if c.PrivateKey == nil {
		return errors.New("edgeapp.BuyerConfig.PrivateKey required")
	}

	// Coerce legacy single-seller fields into Sellers if the slice is empty.
	if len(c.Sellers) == 0 && c.SellerStdIn.Locator() != "" && c.SellerPubKey != nil {
		c.Sellers = []SellerEntry{{StdIn: c.SellerStdIn, PubKey: c.SellerPubKey}}
	}
	if len(c.Sellers) == 0 {
		return errors.New("edgeapp.BuyerConfig.Sellers (or legacy SellerStdIn+SellerPubKey) required")
	}
	for i, s := range c.Sellers {
		if s.PubKey == nil {
			return fmt.Errorf("edgeapp.BuyerConfig.Sellers[%d].PubKey required", i)
		}
		if s.StdIn.Locator() == "" {
			return fmt.Errorf("edgeapp.BuyerConfig.Sellers[%d].StdIn required", i)
		}
	}

	if c.LibP2PListenAddr == "" {
		c.LibP2PListenAddr = "/ip4/0.0.0.0/udp/0/quic-v1"
	}
	if c.Protocol == "" {
		c.Protocol = DefaultProtocol
	}
	if c.RequestID == "" {
		c.RequestID = "edge-feed-001"
	}
	if c.HeartbeatPeriod == 0 {
		c.HeartbeatPeriod = DefaultHeartbeatPeriod
	}
	if c.HeartbeatPeriod < time.Duration(health.MinDeadlineDelta)*time.Second {
		return errors.New("edgeapp.BuyerConfig.HeartbeatPeriod must be >= 10s (health.MinDeadlineDelta)")
	}
	if c.ReconnectBackoff == 0 {
		c.ReconnectBackoff = DefaultReconnectBackoff
	}
	if c.SellerDialTimeout == 0 {
		c.SellerDialTimeout = DefaultSellerDialTimeout
	}
	if c.NegotiationTimeout == 0 {
		c.NegotiationTimeout = DefaultBuyerNegotiationTimeout
	}
	if c.OnAggregatedFrame == nil && c.OnFrame == nil {
		return errors.New("edgeapp.BuyerConfig: one of OnAggregatedFrame or OnFrame required")
	}
	if c.EnforceDeadlines {
		for i, s := range c.Sellers {
			if s.StdOut.Locator() == "" {
				return fmt.Errorf("edgeapp.BuyerConfig.EnforceDeadlines requires Sellers[%d].StdOut", i)
			}
		}
	}
	return nil
}

// DefaultProtocol is the libp2p stream protocol ID for Neuron edge feed
// streaming (BEAST → Mode-S envelope).
const DefaultProtocol = "/neuron/edge-feed/1.0.0"

// DefaultHeartbeatPeriod aligns with spec 005's recommended 60s cadence.
const DefaultHeartbeatPeriod = 60 * time.Second

// Connection-manager watermarks applied to the libp2p host built by
// RunSeller / RunBuyer. The defaults derive from the production seller's
// burst-investigation findings
// — high enough that fan-in won't trigger pruning waves, with a 90 s grace
// period so freshly-dialed peers survive until their first stream Protects
// the underlying connection.
const (
	DefaultConnMgrLowWatermark  = 320
	DefaultConnMgrHighWatermark = 384
	DefaultConnMgrGracePeriod   = 90 * time.Second
)

