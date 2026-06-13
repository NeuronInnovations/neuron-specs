package edgeapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// RunBuyer drives the public-buyer side of the reverse-connect flow with
// support for multiple concurrent sellers.
//
// Lifecycle (in order):
//
//  1. Create stdIn/stdOut/stdErr topics on cfg.Bus if not pre-supplied.
//  2. Build a libp2p host bound to cfg.LibP2PListenAddr (must be reachable
//     by every seller).
//  3. Register HandleIncoming for cfg.Protocol so the buyer is ready to
//     accept incoming streams from any of the configured sellers.
//  4. Start the heartbeat publisher loop (role="buyer",
//     capabilities.natReachability=true).
//  5. Spawn one per-seller worker goroutine. Each worker:
//      a. Publishes a ReverseConnectionSetup to its seller's stdIn.
//      b. Waits for the seller to dial in (incoming stream tagged by
//         PeerID matches a configured seller's pubkey-derived PeerID).
//      c. Calls delivery.ReceiveStream until the stream ends.
//      d. Reports state transitions via OnSellerStatus.
//      e. Loops with cfg.ReconnectBackoff between attempts until ctx is
//         cancelled.
//  6. RunBuyer returns when ctx is cancelled. All worker goroutines exit
//     cleanly; the libp2p host and adapter are torn down by deferred
//     Close/Disconnect.
func RunBuyer(ctx context.Context, cfg BuyerConfig) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = nopLogger{}
	}

	// 1. Topics. When pre-supplied via cfg, use as-is (legacy explicit path).
	// Otherwise consult the optional persistent state file (iter-7 P1.2):
	// reuse if it exists and matches the running identity, else create
	// fresh + persist. Symmetric with seller.go's resolvePersistentTopics.
	pub := cfg.PrivateKey.PublicKey()
	pid, err := pub.PeerID()
	if err != nil {
		return fmt.Errorf("buyer: derive PeerID: %w", err)
	}
	evmHex := pub.EVMAddress().Hex()
	pubHex := pub.Hex()

	var (
		stdIn, stdOut, stdErr topic.TopicRef
		persistState          *EdgeState
		topicsAreFresh        bool
	)
	if cfg.StdIn.Locator() != "" || cfg.StdOut.Locator() != "" || cfg.StdErr.Locator() != "" {
		stdIn, stdOut, stdErr, err = ensureTopics(cfg.Bus, cfg.StdIn, cfg.StdOut, cfg.StdErr, "edge-buyer")
		topicsAreFresh = true
	} else {
		stdIn, stdOut, stdErr, persistState, topicsAreFresh, err = resolvePersistentTopics(
			cfg.Bus, cfg.StatePath, evmHex, pubHex, pid.String(), "edge-buyer")
	}
	if err != nil {
		return fmt.Errorf("buyer: ensure topics: %w", err)
	}
	cfg.StdIn, cfg.StdOut, cfg.StdErr = stdIn, stdOut, stdErr

	if cfg.StatePath != "" {
		if topicsAreFresh && persistState != nil {
			if saveErr := SaveEdgeState(cfg.StatePath, persistState); saveErr != nil {
				logger.Printf("[buyer] state persist failed (continuing): %v", saveErr)
			} else {
				logger.Printf("[buyer] state: wrote %s (fresh topics)", cfg.StatePath)
			}
		} else if !topicsAreFresh {
			logger.Printf("[buyer] state: reusing topics from %s", cfg.StatePath)
		}
	}

	// Allocate the shared ICAO recovery cache, unless explicitly disabled.
	// One cache per buyer process — shared across all per-seller streams.
	if cfg.ICAOCache == nil && !cfg.DisableICAOCache {
		cfg.ICAOCache = feeds.NewICAORecoveryCache(DefaultICAOCacheCap, DefaultICAOCacheTTL)
	}

	logger.Printf("[buyer] identity: evm=%s peer=%s", evmHex, pid.String())
	logger.Printf("[buyer] topics:   stdIn=%s stdOut=%s stdErr=%s",
		stdIn.Locator(), stdOut.Locator(), stdErr.Locator())
	logger.Printf("[buyer] sellers:  %d configured", len(cfg.Sellers))

	// Optional spec-003 / EIP-8004 registration. Idempotent. Buyer runs
	// the same path as RunSeller — the agentURI binds the buyer's three
	// topics to its EVM address.
	if cfg.Registry != nil {
		uri := buildBuyerAgentURI(evmHex, stdIn, stdOut, stdErr)
		if _, fresh, err := EnsureRegistered(ctx, cfg.Registry, evmHex, uri, true); err != nil {
			logger.Printf("[buyer] registration failed (continuing): %v", err)
		} else if fresh {
			logger.Printf("[buyer] registered (fresh) evm=%s", evmHex)
		} else {
			logger.Printf("[buyer] registration up-to-date evm=%s", evmHex)
		}
	}

	// Pre-compute each seller's libp2p PeerID (string form, matching
	// channel.PeerID) and EVM address from its secp256k1 pubkey.
	sellersByPeerID := make(map[string]*resolvedSeller, len(cfg.Sellers))
	for i := range cfg.Sellers {
		s := &cfg.Sellers[i]
		rs, err := resolveSeller(s)
		if err != nil {
			return fmt.Errorf("buyer: resolve seller[%d]: %w", i, err)
		}
		if _, dup := sellersByPeerID[rs.peerIDString]; dup {
			return fmt.Errorf("buyer: duplicate seller pubkey at index %d (peerID=%s)", i, rs.peerIDString)
		}
		sellersByPeerID[rs.peerIDString] = rs
		logger.Printf("[buyer] seller[%d]: name=%s evm=%s peer=%s stdIn=%s",
			i, rs.displayName, rs.evm, rs.peerIDString, s.StdIn.Locator())
	}

	// 2. Libp2p host.
	ecdsaPriv, err := cfg.PrivateKey.ToBlockchainKey()
	if err != nil {
		return fmt.Errorf("buyer: convert privkey: %w", err)
	}
	host, err := delivery.NewLibp2pHost(ecdsaPriv, cfg.LibP2PListenAddr,
		delivery.WithConnManager(
			DefaultConnMgrLowWatermark,
			DefaultConnMgrHighWatermark,
			DefaultConnMgrGracePeriod,
		),
	)
	if err != nil {
		return fmt.Errorf("buyer: libp2p host: %w", err)
	}
	defer host.Close()
	adapter := delivery.NewLibp2pAdapter(host)
	for _, a := range host.Addrs() {
		logger.Printf("[buyer] listening: %s/p2p/%s", a, host.ID())
	}

	// 3. Single HandleIncoming callback fans out per-seller.
	for _, rs := range sellersByPeerID {
		rs.incoming = make(chan *delivery.DeliveryChannel, 4)
	}
	// Single fan-out handler shared by every advertised protocol.
	streamFanOut := func(ch *delivery.DeliveryChannel) {
		rs, ok := sellersByPeerID[ch.PeerID]
		if !ok {
			logger.Printf("[buyer] dropped incoming stream from unknown peer=%s", ch.PeerID)
			_ = adapter.Disconnect(ch)
			return
		}
		select {
		case rs.incoming <- ch:
		default:
			// Already have a pending channel for this seller; drop the new one.
			_ = adapter.Disconnect(ch)
		}
	}
	// Legacy protocol: every existing seller dials this; preserve verbatim.
	adapter.HandleIncoming(protocol.ID(cfg.Protocol), streamFanOut)
	// Phase-4 alias (reference MVP §1 Phase 4): also accept the spec-016
	// canonical "/jetvision/raw/1.0.0" protocol ID. The buyer advertises this
	// alongside cfg.Protocol in its streams[] catalog so a future seller
	// that prefers the spec-016 path can dial it; today no seller does,
	// but registering the alias makes the advertisement truthful.
	if AdsbProtocolRaw != cfg.Protocol {
		adapter.HandleIncoming(protocol.ID(AdsbProtocolRaw), streamFanOut)
	}

	// 4. Heartbeat publisher.
	var wg sync.WaitGroup
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	wg.Add(1)
	go func() {
		defer wg.Done()
		runHeartbeat(hbCtx, heartbeatConfig{
			Role:       health.RoleBuyer,
			Bus:        cfg.Bus,
			Key:        cfg.PrivateKey,
			StdOut:     stdOut,
			Period:     cfg.HeartbeatPeriod,
			Reachable:  true,
			Location:   cfg.HeartbeatLocation,
			ProtocolID: cfg.Protocol,
			Logger:     logger,
		})
	}()

	// 5. Per-seller workers.
	workersCtx, workersCancel := context.WithCancel(ctx)
	defer workersCancel()

	for _, rs := range sellersByPeerID {
		rs := rs
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSellerWorker(workersCtx, &cfg, adapter, host, rs, stdIn, evmHex, logger)
		}()
	}
	_ = ecdsaPriv // dialerKey is unused on the buyer (dialee) side; retained at import scope for future use

	// 6. Block until ctx cancel.
	<-ctx.Done()
	workersCancel()
	hbCancel()
	wg.Wait()

	logger.Printf("[buyer] graceful shutdown")
	return nil
}

// resolvedSeller is the runtime expansion of a SellerEntry — pubkey resolved
// to a libp2p PeerID string and an EVM address, plus an incoming-channel
// queue keyed by PeerID for HandleIncoming dispatch.
type resolvedSeller struct {
	entry        *SellerEntry
	displayName  string
	evm          string
	peerIDString string
	incoming     chan *delivery.DeliveryChannel
	frames       atomic.Uint64
	lastFrameAt  atomic.Int64 // unix nano
}

func resolveSeller(s *SellerEntry) (*resolvedSeller, error) {
	if s.PubKey == nil {
		return nil, errors.New("nil PubKey")
	}
	npub, err := keylib.NeuronPublicKeyFromBlockchainKey(s.PubKey)
	if err != nil {
		return nil, fmt.Errorf("convert pubkey: %w", err)
	}
	pid, err := npub.PeerID()
	if err != nil {
		return nil, fmt.Errorf("derive PeerID: %w", err)
	}
	evm := npub.EVMAddress().Hex()
	display := s.DisplayName
	if display == "" {
		display = abbrEVM(evm)
	}
	return &resolvedSeller{
		entry:        s,
		displayName:  display,
		evm:          evm,
		peerIDString: pid.String(),
	}, nil
}

// runSellerWorker is the per-seller lifecycle loop.
//
// Each iteration: (optionally) negotiate + fund a per-session escrow →
// publish ReverseConnectionSetup → wait for the seller to dial in →
// ReceiveStream until the stream ends → (optionally) settle the agreement
// → on close, sleep cfg.ReconnectBackoff and loop. Exits cleanly when ctx
// is cancelled.
func runSellerWorker(
	ctx context.Context,
	cfg *BuyerConfig,
	adapter *delivery.Libp2pAdapter,
	h host.Host,
	rs *resolvedSeller,
	buyerStdIn topic.TopicRef,
	buyerEVM string,
	logger Logger,
) {
	for {
		if ctx.Err() != nil {
			return
		}

		// Drain any stale incoming channel that might be left over from a
		// previous iteration so we only react to the current setup-publish.
		drain(rs.incoming, adapter)

		emitStatus(cfg.OnSellerStatus, rs, SellerStateConnecting, "")

		// Optional commerce gate: negotiate + fund an escrow before the
		// data plane starts. The seller's matching SellerObserveAndAccept
		// is gated by SellerConfig.RequireFundedAgreement on the seller side.
		var (
			session *BuyerSession
		)
		if cfg.Escrow != nil {
			s, err := negotiateAndFund(ctx, cfg, rs, buyerStdIn, buyerEVM)
			if err != nil {
				logger.Printf("[buyer:%s] commerce negotiate+fund failed: %v",
					rs.displayName, err)
				emitStatus(cfg.OnSellerStatus, rs, SellerStateError, err.Error())
				if !sleep(ctx, cfg.ReconnectBackoff) {
					return
				}
				continue
			}
			session = s
			logger.Printf("[buyer:%s] agreement funded ref=%s state=%s",
				rs.displayName, session.EscrowRef().Locator, session.State())
		}

		// Build setup. host.Addrs() may have changed since the last attempt
		// (NAT remapping, transport additions); rebuild each loop. Iter-7
		// P1.1: use the per-session requestID from negotiateAndFund when
		// commerce is enabled — that's what the seller's idempotency guard
		// matches against. Falls back to the legacy static format when
		// commerce is off (no session).
		setupRID := cfg.RequestID + "-" + rs.displayName
		if session != nil {
			setupRID = session.RequestID()
		}
		// Phase-4 (reference MVP §1 Phase 4): also advertise the spec-016
		// "/jetvision/raw/1.0.0" protocol in streams[] alongside the legacy
		// cfg.Protocol. Forward-compat advertisement; today's sellers
		// continue to dial cfg.Protocol from the legacy Protocol field.
		setup, err := delivery.BuildReverseConnectionSetup(
			setupRID,
			h,
			cfg.Protocol,
			rs.entry.PubKey,
			delivery.WithStreams(AdsbStreamCatalog()),
		)
		if err != nil {
			logger.Printf("[buyer:%s] build reverse setup: %v", rs.displayName, err)
			emitStatus(cfg.OnSellerStatus, rs, SellerStateError, err.Error())
			if !sleep(ctx, cfg.ReconnectBackoff) {
				return
			}
			continue
		}
		if len(cfg.LibP2PAdvertisedMultiaddrs) > 0 {
			encrypted, err := delivery.EncryptMultiaddrs(cfg.LibP2PAdvertisedMultiaddrs, rs.entry.PubKey)
			if err != nil {
				logger.Printf("[buyer:%s] re-encrypt advertised multiaddrs: %v", rs.displayName, err)
				emitStatus(cfg.OnSellerStatus, rs, SellerStateError, err.Error())
				if !sleep(ctx, cfg.ReconnectBackoff) {
					return
				}
				continue
			}
			setup.EncryptedMultiaddrs = encrypted
		}

		if err := publishPayload(cfg.Bus, cfg.PrivateKey, rs.entry.StdIn, setup); err != nil {
			logger.Printf("[buyer:%s] publish setup: %v", rs.displayName, err)
			emitStatus(cfg.OnSellerStatus, rs, SellerStateError, err.Error())
			if !sleep(ctx, cfg.ReconnectBackoff) {
				return
			}
			continue
		}
		logger.Printf("[buyer:%s] published ReverseConnectionSetup → %s requestID=%s",
			rs.displayName, rs.entry.StdIn.Locator(), setup.RequestID)

		// Wait for incoming stream from this seller.
		var channel *delivery.DeliveryChannel
		select {
		case <-ctx.Done():
			return
		case channel = <-rs.incoming:
		case <-time.After(cfg.SellerDialTimeout):
			logger.Printf("[buyer:%s] dial-in timeout after %s; re-publishing", rs.displayName, cfg.SellerDialTimeout)
			emitStatus(cfg.OnSellerStatus, rs, SellerStateError, "dial-in timeout")
			if !sleep(ctx, cfg.ReconnectBackoff) {
				return
			}
			continue
		}

		logger.Printf("[buyer:%s] accepted stream peer=%s transport=%s",
			rs.displayName, channel.PeerID, channel.Transport)
		emitStatus(cfg.OnSellerStatus, rs, SellerStateConnected, "")

		// streamCtx is what the receive-stream and (optional) liveness
		// tracker share. EnforceDeadlines wires a stale signal that
		// cancels the stream early; without the flag, streamCtx == ctx.
		streamCtx, streamCancel := context.WithCancel(ctx)
		var livenessDone chan error
		if cfg.EnforceDeadlines {
			livenessDone = startSellerLivenessWatch(streamCtx, cfg.Bus, rs, streamCancel, logger)
		}

		err = streamFromSeller(streamCtx, cfg, adapter, channel, rs, logger)
		_ = adapter.Disconnect(channel)
		streamCancel()
		if livenessDone != nil {
			<-livenessDone
		}

		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Printf("[buyer:%s] stream ended with error: %v", rs.displayName, err)
			emitStatus(cfg.OnSellerStatus, rs, SellerStateError, err.Error())
		} else {
			logger.Printf("[buyer:%s] stream closed gracefully", rs.displayName)
			emitStatus(cfg.OnSellerStatus, rs, SellerStateDisconnected, "")
		}

		// Optional commerce settlement: drives the buyer side from
		// FUNDED → ACTIVE → INVOICED → ACTIVE while consuming the seller's
		// Invoice on stdIn. Errors are logged but don't gate the reconnect
		// loop — a stuck escrow will be rescued by the agreement timeout
		// (refund path).
		//
		// Settle uses a fresh context.Background base so it can complete
		// even when the worker's `ctx` is the cancellation that ended the
		// data plane. The 120s ceiling accommodates the EVM escrow path
		// where ApproveRelease internally fires 3 sequential txs (token
		// approve + escrow approveRelease + escrow withdraw) on a slow
		// gateway like Hashio (~10s per tx).
		if session != nil {
			settleCtx, settleCancel := context.WithTimeout(context.Background(), 120*time.Second)
			if settleErr := session.Settle(settleCtx); settleErr != nil {
				logger.Printf("[buyer:%s] settle failed: %v", rs.displayName, settleErr)
			} else {
				logger.Printf("[buyer:%s] settled state=%s", rs.displayName, session.State())
				if cfg.OnAgreementSettled != nil {
					cfg.OnAgreementSettled(session)
				}
			}
			settleCancel()
		}

		if ctx.Err() != nil {
			return
		}
		if !sleep(ctx, cfg.ReconnectBackoff) {
			return
		}
	}
}

// negotiateAndFund is the per-iteration commerce gate. It builds a
// BuyerSessionConfig from the runtime BuyerConfig + the per-seller
// resolvedSeller, runs BuyerNegotiateAndFund, and returns the resulting
// BuyerSession (which the caller will Settle later).
//
// **Iter-7 P1.1 + P1.2:** the agreement's RequestID is GENERATED FRESH on
// every call — `<prefix>-<seller>-<utc-timestamp>-<8-hex-rand>`. This makes
// every retry, every reconnect-loop iteration, every period boundary, and
// every multi-agreement session attestable separately on the validator side
// (whose dedup key uses requestID + agreementHash). The legacy
// `cfg.RequestID + "-" + sellerName` shape is preserved as the prefix so
// log lines remain greppable, but the suffix guarantees uniqueness.
func negotiateAndFund(
	ctx context.Context,
	cfg *BuyerConfig,
	rs *resolvedSeller,
	buyerStdIn topic.TopicRef,
	buyerEVM string,
) (*BuyerSession, error) {
	currency := cfg.PaymentCurrency
	if currency == "" {
		currency = "tinybar"
	}
	price := cfg.PaymentPriceTinybar
	if price == "" {
		price = "100"
	}
	prefix := cfg.PaymentRequestID
	if prefix == "" {
		prefix = cfg.RequestID
	}
	requestID := newSessionRequestID(prefix, rs.displayName)

	bcfg := BuyerSessionConfig{
		Bus:              cfg.Bus,
		Key:              cfg.PrivateKey,
		BuyerStdIn:       buyerStdIn,
		SellerStdIn:      rs.entry.StdIn,
		RequestID:        requestID,
		ServiceRef:       "neuron://service/edge-feed/v1",
		BuyerEVM:         buyerEVM,
		SellerEVM:        rs.evm,
		Currency:         currency,
		Price:            price,
		NegotiationTTL:   cfg.NegotiationTimeout,
		AgreementTimeout: 24 * time.Hour,
		Escrow:           cfg.Escrow,
		Logger:           cfg.Logger,
	}
	return BuyerNegotiateAndFund(ctx, bcfg)
}

// newSessionRequestID returns a globally-unique-per-process requestID of
// the shape `<prefix>-<seller>-<YYYYMMDDTHHMMSSZ>-<8-hex-rand>`. The
// timestamp is human-readable for logs; the random suffix prevents
// collisions when two retries within the same second produce IDs.
//
// Falls back to a deterministic suffix only if crypto/rand fails (which
// shouldn't happen on any non-degraded system).
func newSessionRequestID(prefix, seller string) string {
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		// crypto/rand failure is exceptionally rare; use the nano clock.
		_ = err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	return fmt.Sprintf("%s-%s-%s-%s", prefix, seller, ts, hex.EncodeToString(nonce[:]))
}

// streamFromSeller pulls wire-encoded FeedFrames off the channel, decodes
// each, decorates with seller identity + Mode-S meta, and emits via
// OnAggregatedFrame (or OnFrame as fallback).
func streamFromSeller(
	ctx context.Context,
	cfg *BuyerConfig,
	adapter *delivery.Libp2pAdapter,
	channel *delivery.DeliveryChannel,
	rs *resolvedSeller,
	logger Logger,
) error {
	wireIn := make(chan []byte, 256)
	rcvCtx, rcvCancel := context.WithCancel(ctx)
	defer rcvCancel()

	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)
		for buf := range wireIn {
			f, err := feeds.DecodeFeedFrame(buf)
			if err != nil {
				logger.Printf("[buyer:%s] decode feed-frame error: %v", rs.displayName, err)
				continue
			}
			f.Rx = time.Now().UTC()

			meta := feeds.DecodeModeSMeta(f.Raw)
			// Plaintext ICAO (DF 11/17/18) seeds the recovery cache.
			// Empty ICAO + parity-XOR'd DF (0/4/5/16/20/21) tries to
			// recover; a hit fills meta.ICAO and sets Recovered=true.
			if cfg.ICAOCache != nil {
				if meta.ICAO != "" {
					cfg.ICAOCache.Observe(meta.ICAO)
				} else if isParityICAOBearing(meta.DF) {
					if icao, ok := cfg.ICAOCache.TryRecover(f.Raw); ok {
						meta.ICAO = icao
						meta.Recovered = true
					}
				}
			}
			af := AggregatedFrame{
				SellerEVM:    rs.evm,
				SellerName:   rs.displayName,
				SellerPeerID: rs.peerIDString,
				Frame:        f,
				Meta:         meta,
				ReceivedAt:   time.Now().UTC(),
			}
			if cfg.OnAggregatedFrame != nil {
				cfg.OnAggregatedFrame(af)
			} else {
				cfg.OnFrame(f)
			}
			// Phase 5 — additionally emit the v2-tagged envelope to any
			// dual-stream display consumer that opted in. Best-effort: a
			// slow tagged sink MUST NOT slow down the legacy path; the
			// hook is responsible for its own non-blocking discipline.
			if cfg.OnTaggedAdsb != nil {
				cfg.OnTaggedAdsb(TagAdsbAggregatedFrame(af))
			}
			n := rs.frames.Add(1)
			rs.lastFrameAt.Store(time.Now().UnixNano())
			if n%5000 == 0 {
				logger.Printf("[buyer:%s] received=%d frames", rs.displayName, n)
			}
		}
	}()

	rcvErr := delivery.ReceiveStream(adapter, channel, rcvCtx, wireIn)
	close(wireIn)
	<-pumpDone
	return rcvErr
}

// drain empties any pending channels from rs.incoming and disconnects them.
// Used at the start of each worker iteration to avoid acting on stale dials
// from a previous attempt.
func drain(ch chan *delivery.DeliveryChannel, adapter *delivery.Libp2pAdapter) {
	for {
		select {
		case stale := <-ch:
			_ = adapter.Disconnect(stale)
		default:
			return
		}
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func emitStatus(cb func(SellerStatus), rs *resolvedSeller, state SellerState, errMsg string) {
	if cb == nil {
		return
	}
	st := SellerStatus{
		EVM:            rs.evm,
		DisplayName:    rs.displayName,
		PeerID:         rs.peerIDString,
		State:          state,
		FramesReceived: rs.frames.Load(),
		LastError:      errMsg,
	}
	if ns := rs.lastFrameAt.Load(); ns > 0 {
		st.LastFrameAt = time.Unix(0, ns).UTC()
	}
	cb(st)
}

// isParityICAOBearing returns true when df is one of the Mode-S downlink
// formats whose AP field is parity ⊕ ICAO24 (recoverable via the cache).
//
// DF 11/17/18 are intentionally excluded — they carry plaintext ICAO and
// their AP encodes interrogator/subnetwork bits, not parity ⊕ ICAO.
//
// DF 24 (long ELM) is excluded for now: its addressing semantics differ
// and we haven't validated the recovery on it.
func isParityICAOBearing(df byte) bool {
	switch df {
	case feeds.DFShortAirAirSurveillance, // 0
		feeds.DFAltitudeReply,            // 4
		feeds.DFIdentityReply,            // 5
		feeds.DFLongAirAirSurveillance,   // 16
		feeds.DFCommBAltitude,            // 20
		feeds.DFCommBIdentity:            // 21
		return true
	}
	return false
}

// publishPayload signs payload as a TopicMessage with key and publishes to
// targetTopic via bus. Used for non-heartbeat control messages
// (ReverseConnectionSetup).
func publishPayload(bus topic.TopicAdapter, key *keylib.NeuronPrivateKey, targetTopic topic.TopicRef, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	now := uint64(time.Now().UnixNano())
	msg, err := topic.NewTopicMessage(key, now, now, data)
	if err != nil {
		return fmt.Errorf("create topic message: %w", err)
	}
	if _, err := bus.Publish(targetTopic, msg, topic.PublishOpts{ConfirmationMode: topic.FireAndForget}); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}

