package edgeapp

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// RunSeller drives the NAT'd-seller side of the reverse-connect flow.
//
// Steps (in order):
//
//  1. Create stdIn/stdOut/stdErr topics on cfg.Bus if not pre-supplied.
//  2. Build a libp2p host bound to cfg.LibP2PListenAddr (outbound-only is OK).
//  3. Start the heartbeat publisher loop, publishing a spec-005 envelope to
//     stdOut every cfg.HeartbeatPeriod with role="seller" and
//     capabilities.natReachability=false.
//  4. Subscribe to stdIn and wait for a payment.ConnectionSetup published by
//     the buyer (a ReverseConnectionSetup payload — same envelope, opposite
//     direction).
//  5. Decrypt + dial via delivery.ConnectFromReverseSetup → DeliveryChannel.
//  6. Spawn the configured FeedSource. Encode each FeedFrame via
//     feeds.EncodeFeedFrame and SendStream them to the buyer.
//  7. Run until ctx is cancelled. On shutdown: stop feed → close adapter →
//     close libp2p host. Returns nil on graceful exit.
//
// RunSeller is blocking — call it from main or a goroutine.
func RunSeller(ctx context.Context, cfg SellerConfig) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = nopLogger{}
	}

	// 1. Topics. When pre-supplied via cfg, use as-is (legacy explicit-config
	// path). Otherwise consult the optional persistent state file: reuse if
	// it exists and matches the running identity, else create fresh + persist.
	pub := cfg.PrivateKey.PublicKey()
	pid, err := pub.PeerID()
	if err != nil {
		return fmt.Errorf("seller: derive PeerID: %w", err)
	}
	evmHex := pub.EVMAddress().Hex()
	pubHex := pub.Hex() // compressed secp256k1, "0x"-prefixed; MatchesIdentity normalizes

	var (
		stdIn, stdOut, stdErr topic.TopicRef
		persistState          *EdgeState
		topicsAreFresh        bool
	)
	if cfg.StdIn.Locator() != "" || cfg.StdOut.Locator() != "" || cfg.StdErr.Locator() != "" {
		// Legacy / explicit path — caller pre-created some or all topics.
		// Statefile is bypassed; persistState stays nil so we don't write.
		stdIn, stdOut, stdErr, err = ensureTopics(cfg.Bus, cfg.StdIn, cfg.StdOut, cfg.StdErr, "edge-seller")
		topicsAreFresh = true
	} else {
		stdIn, stdOut, stdErr, persistState, topicsAreFresh, err = resolvePersistentTopics(
			cfg.Bus, cfg.StatePath, evmHex, pubHex, pid.String(), "edge-seller")
	}
	if err != nil {
		return fmt.Errorf("seller: ensure topics: %w", err)
	}
	cfg.StdIn, cfg.StdOut, cfg.StdErr = stdIn, stdOut, stdErr

	if cfg.StatePath != "" {
		if topicsAreFresh && persistState != nil {
			if saveErr := SaveEdgeState(cfg.StatePath, persistState); saveErr != nil {
				logger.Printf("[seller] state persist failed (continuing): %v", saveErr)
			} else {
				logger.Printf("[seller] state: wrote %s (fresh topics)", cfg.StatePath)
			}
		} else if !topicsAreFresh {
			logger.Printf("[seller] state: reusing topics from %s", cfg.StatePath)
		}
	}

	logger.Printf("[seller] identity: evm=%s peer=%s", evmHex, pid.String())
	logger.Printf("[seller] topics:   stdIn=%s stdOut=%s stdErr=%s",
		stdIn.Locator(), stdOut.Locator(), stdErr.Locator())

	// 1a. Optional spec-013 Profile E descriptor publish. Skipped when the
	// flag is off; mock-default for the JV box keeps the existing
	// seller-bootstrap.json fallback unchanged.
	if cfg.PublishProfileDescriptor {
		desc := BuildProfileDescriptor(evmHex, pid.String(), cfg.Protocol, 1)
		if hash, published, err := EnsurePublishedDescriptor(cfg.Bus, cfg.PrivateKey, stdOut, desc, persistState); err != nil {
			logger.Printf("[seller] descriptor publish failed (continuing): %v", err)
		} else if published {
			logger.Printf("[seller] profile descriptor published hash=%s", hash[:16])
			if cfg.StatePath != "" && persistState != nil {
				if saveErr := SaveEdgeState(cfg.StatePath, persistState); saveErr != nil {
					logger.Printf("[seller] state persist after descriptor failed (continuing): %v", saveErr)
				}
			}
		} else {
			logger.Printf("[seller] profile descriptor unchanged (hash=%s); skipped", hash[:16])
		}
	}

	// 1b. Optional spec-003 / EIP-8004 registration. Idempotent.
	if cfg.Registry != nil {
		uri := buildSellerAgentURI(evmHex, stdIn, stdOut, stdErr)
		if _, fresh, err := EnsureRegistered(ctx, cfg.Registry, evmHex, uri, true); err != nil {
			logger.Printf("[seller] registration failed (continuing): %v", err)
		} else if fresh {
			logger.Printf("[seller] registered (fresh) evm=%s agentURI=%s", evmHex, uri)
		} else {
			logger.Printf("[seller] registration up-to-date evm=%s", evmHex)
		}
	}

	// 2. Libp2p host.
	ecdsaPriv, err := cfg.PrivateKey.ToBlockchainKey()
	if err != nil {
		return fmt.Errorf("seller: convert privkey: %w", err)
	}
	host, err := delivery.NewLibp2pHost(ecdsaPriv, cfg.LibP2PListenAddr,
		delivery.WithConnManager(
			DefaultConnMgrLowWatermark,
			DefaultConnMgrHighWatermark,
			DefaultConnMgrGracePeriod,
		),
	)
	if err != nil {
		return fmt.Errorf("seller: libp2p host: %w", err)
	}
	defer host.Close()
	adapter := delivery.NewLibp2pAdapter(host)
	logger.Printf("[seller] libp2p host id=%s listening on %d addr(s)",
		host.ID().String(), len(host.Addrs()))

	// 3. Heartbeat publisher loop.
	var wg sync.WaitGroup
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	wg.Add(1)
	go func() {
		defer wg.Done()
		runHeartbeat(hbCtx, heartbeatConfig{
			Role:       health.RoleSeller,
			Bus:        cfg.Bus,
			Key:        cfg.PrivateKey,
			StdOut:     stdOut,
			Period:     cfg.HeartbeatPeriod,
			Reachable:  false, // NAT'd seller — published authoritatively
			Location:   cfg.HeartbeatLocation,
			ProtocolID: cfg.Protocol,
			Logger:     logger,
		})
	}()

	// Iter-7 P1.4: when AgreementPeriod > 0, RunSeller loops one
	// agreement at a time — each iteration runs the commerce gate, the
	// data stream, and Invoice/InvoiceAck before going back to the gate
	// for the next agreement. When AgreementPeriod is 0, the loop runs
	// exactly once (legacy SIGINT-only path).
	for {
		if ctx.Err() != nil {
			break
		}
		stop, err := runOneAgreement(ctx, cfg, adapter, ecdsaPriv, stdIn, logger)
		if err != nil {
			hbCancel()
			wg.Wait()
			return err
		}
		if stop {
			break
		}
	}
	hbCancel()
	wg.Wait()
	return nil
}

// runOneAgreement runs the per-agreement portion of RunSeller's lifecycle:
// commerce gate → wait for ReverseConnectionSetup → dial → stream frames
// → settle. Returns (stop=true) when the loop should exit (legacy
// single-shot mode, or ctx cancelled). Returns (stop=false, err=nil) when
// the iteration completed cleanly + the outer loop should continue with
// the next agreement (period-driven mode).
//
// On idle-FUNDED timeout, returns (stop=false, err=nil) so the outer loop
// re-enters the commerce gate immediately. The buyer's reconnect loop
// will publish a fresh ServiceRequest within ReconnectBackoff.
func runOneAgreement(
	ctx context.Context,
	cfg SellerConfig,
	adapter *delivery.Libp2pAdapter,
	ecdsaPriv *ecdsa.PrivateKey,
	stdIn topic.TopicRef,
	logger Logger,
) (bool, error) {
	// 3a. Optional commerce gate: wait for ServiceRequest + EscrowCreated
	// on stdIn before accepting any ReverseConnectionSetup. This pairs
	// with BuyerConfig.Escrow on the other side. Without the gate the
	// dial-in is unauthenticated as in Phase C.2.
	var sellerSession *SellerSession
	if cfg.Escrow != nil {
		ss, err := sellerCommerceAcceptFromStdIn(ctx, cfg, stdIn, logger)
		if err != nil {
			if ctx.Err() != nil {
				return true, nil
			}
			return false, fmt.Errorf("seller: commerce accept: %w", err)
		}
		sellerSession = ss
		logger.Printf("[seller] agreement funded requestID=%s state=%s",
			sellerSession.requestID, sellerSession.State())
	}

	// 4. Wait for ReverseConnectionSetup on stdIn.
	// Iter-7 P1.3: if cfg.IdleFundedTimeout > 0, the seller abandons the
	// FUNDED state after the timeout and lets the outer loop re-enter the
	// commerce gate. If a duplicate ServiceRequest arrives during this
	// wait with the SAME rid, the seller idempotently re-publishes the
	// existing ServiceResponse (the buyer lost track and needs the ack
	// replayed). With a DIFFERENT rid, the seller abandons the current
	// agreement and the outer loop accepts the new ServiceRequest as
	// fresh.
	logger.Printf("[seller] subscribing to stdIn for ReverseConnectionSetup")
	setupCtx, setupCancel := context.WithCancel(ctx)
	defer setupCancel()
	in, err := cfg.Bus.Subscribe(setupCtx, stdIn, topic.SubscribeOpts{})
	if err != nil {
		return false, fmt.Errorf("seller: subscribe stdIn: %w", err)
	}

	idleTimeout := cfg.IdleFundedTimeout
	var idleTimer *time.Timer
	var idleC <-chan time.Time
	if idleTimeout > 0 && sellerSession != nil {
		idleTimer = time.NewTimer(idleTimeout)
		idleC = idleTimer.C
		defer idleTimer.Stop()
	}

	var setup *payment.ConnectionSetup
waitSetup:
	for {
		select {
		case <-ctx.Done():
			return true, nil
		case <-idleC:
			logger.Printf("[seller] idle FUNDED timeout (%s) requestID=%s — abandoning agreement",
				idleTimeout, sellerSession.requestID)
			return false, nil
		case msg, ok := <-in:
			if !ok {
				return false, errors.New("seller: stdIn subscription closed before connection setup arrived")
			}
			if err := topic.ValidateTopicMessage(msg.Message); err != nil {
				logger.Printf("[seller] dropped invalid signed message on stdIn: %v", err)
				continue
			}
			var probe struct {
				Type      string `json:"type"`
				RequestID string `json:"requestId"`
			}
			payload := msg.Message.Payload()
			if err := json.Unmarshal(payload, &probe); err != nil {
				logger.Printf("[seller] dropped non-JSON payload on stdIn")
				continue
			}
			switch probe.Type {
			case "connectionSetup":
				var cs payment.ConnectionSetup
				if err := json.Unmarshal(payload, &cs); err != nil {
					logger.Printf("[seller] dropped malformed connectionSetup: %v", err)
					continue
				}
				if sellerSession != nil && cs.RequestID != sellerSession.requestID {
					logger.Printf("[seller] ignoring connectionSetup with mismatched requestID got=%s want=%s",
						cs.RequestID, sellerSession.requestID)
					continue
				}
				setup = &cs
				break waitSetup
			case payment.PayloadServiceRequest:
				if sellerSession == nil {
					// No active agreement: defer to outer commerce gate.
					logger.Printf("[seller] stray ServiceRequest with no active agreement — outer loop will pick it up")
					continue
				}
				if probe.RequestID == sellerSession.requestID {
					// Duplicate ServiceRequest for the SAME agreement —
					// buyer lost track of our ServiceResponse. Re-publish.
					if err := replayServiceResponse(cfg, sellerSession, logger); err != nil {
						logger.Printf("[seller] replay ServiceResponse failed: %v", err)
					}
					continue
				}
				// Different requestID = the buyer abandoned the old one
				// and started a fresh agreement. Abandon current FUNDED
				// state; outer loop will re-enter commerce gate.
				logger.Printf("[seller] new ServiceRequest requestID=%s during FUNDED state for %s — abandoning old agreement",
					probe.RequestID, sellerSession.requestID)
				return false, nil
			default:
				logger.Printf("[seller] ignoring %q payload on stdIn requestID=%s", probe.Type, probe.RequestID)
				continue
			}
		}
	}
	setupCancel()
	logger.Printf("[seller] received ReverseConnectionSetup requestID=%s peer=%s",
		setup.RequestID, setup.PeerID)

	// 5. Decrypt and dial.
	channel, err := delivery.ConnectFromReverseSetup(adapter, setup, ecdsaPriv)
	if err != nil {
		return false, fmt.Errorf("seller: dial buyer: %w", err)
	}
	defer adapter.Disconnect(channel)
	logger.Printf("[seller] dialed buyer transport=%s remote=%s",
		channel.Transport, channel.Path.RemoteMultiaddr)

	// 6. Pump frames from FeedSource → SendStream. When AgreementPeriod
	// > 0, a timer fires streamCancel() to gracefully end the stream
	// (which triggers the settle path below).
	//
	// 008 FR-P45/P46 (long-lived discipline): this seller-initiated close
	// is acceptable because AgreementPeriod is a CONTRACTUAL DURATION
	// configured by the operator (see SellerConfig.AgreementPeriod doc),
	// not a resource-pressure or transient-fault response. New deployments
	// SHOULD prefer buyer-issued serviceStop (FR-P36) and leave
	// AgreementPeriod = 0.
	streamCtx, streamCancel := context.WithCancel(ctx)
	defer streamCancel()
	if cfg.AgreementPeriod > 0 {
		t := time.AfterFunc(cfg.AgreementPeriod, func() {
			logger.Printf("[seller] AgreementPeriod=%s elapsed — closing stream for graceful settlement (legacy auto-stop; new deployments should use serviceStop per 008 FR-P36)",
				cfg.AgreementPeriod)
			streamCancel()
		})
		defer t.Stop()
	}

	frames := make(chan feeds.FeedFrame, 256)
	wireOut := make(chan []byte, 256)

	feedCtx, feedCancel := context.WithCancel(streamCtx)
	defer feedCancel()
	var streamWG sync.WaitGroup
	streamWG.Add(1)
	go func() {
		defer streamWG.Done()
		err := cfg.FeedSource(feedCtx, frames)
		close(frames)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Printf("[seller] feed source returned: %v", err)
		}
	}()

	streamWG.Add(1)
	go func() {
		defer streamWG.Done()
		defer close(wireOut)
		var sent uint64
		nextLog := time.Now().Add(5 * time.Second)
		for f := range frames {
			wire := feeds.EncodeFeedFrame(f)
			select {
			case <-feedCtx.Done():
				return
			case wireOut <- wire:
				sent++
				if time.Now().After(nextLog) {
					logger.Printf("[seller] feed → wire: sent=%d frames", sent)
					nextLog = time.Now().Add(5 * time.Second)
				}
			}
		}
	}()

	sendErr := delivery.SendStream(adapter, channel, streamCtx, wireOut)
	feedCancel()
	_ = adapter.Disconnect(channel)
	streamWG.Wait()

	if sendErr != nil && !errors.Is(sendErr, context.Canceled) {
		return false, fmt.Errorf("seller: send stream: %w", sendErr)
	}
	logger.Printf("[seller] graceful shutdown")

	// 7. Optional commerce settlement: issue Invoice, wait for InvoiceAck.
	// Errors are surfaced but don't fail the agreement loop — graceful
	// shutdown already happened, the data plane is closed, and the buyer's
	// escrow timeout will refund if InvoiceAck never arrives.
	if sellerSession != nil {
		invoiceCtx, invoiceCancel := context.WithTimeout(context.Background(), 180*time.Second)
		releaseRef := "release-" + sellerSession.requestID
		if err := sellerSession.IssueInvoice(invoiceCtx, releaseRef, 180*time.Second); err != nil {
			logger.Printf("[seller] invoice settlement failed: %v", err)
		} else {
			logger.Printf("[seller] invoice settled state=%s", sellerSession.State())
		}
		invoiceCancel()
	}

	// stop=true when AgreementPeriod is unset (legacy single-shot mode);
	// stop=false when period-driven (outer loop continues to next agreement).
	if cfg.AgreementPeriod == 0 {
		return true, nil
	}
	return ctx.Err() != nil, nil
}

// replayServiceResponse re-publishes the cached ServiceResponse for the
// active sellerSession. Used by the iter-7 P1.3 idempotent-on-duplicate
// path: a buyer that lost track of our first ServiceResponse can publish a
// duplicate ServiceRequest and we ack it without restarting state.
func replayServiceResponse(cfg SellerConfig, s *SellerSession, logger Logger) error {
	resp := payment.ServiceResponse{
		Type:      payment.PayloadServiceResponse,
		Version:   "1.0.0",
		RequestID: s.requestID,
		Action:    "accept",
	}
	if err := publishCommerce(cfg.Bus, cfg.PrivateKey, s.cfg.BuyerStdIn, resp); err != nil {
		return err
	}
	logger.Printf("[seller] replayed ServiceResponse for duplicate ServiceRequest requestID=%s", s.requestID)
	return nil
}

// sellerCommerceAcceptFromStdIn is RunSeller's adapter for
// SellerObserveAndAccept. It parses the buyerStdIn locator out of the
// ServiceRequest payload to wire the SellerSession's BuyerStdIn (the
// topic Invoice + InvoiceAck flow on). Returns the funded SellerSession
// or an error if the negotiation deadline expires.
func sellerCommerceAcceptFromStdIn(
	ctx context.Context,
	cfg SellerConfig,
	stdIn topic.TopicRef,
	logger Logger,
) (*SellerSession, error) {
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	deliveries, err := cfg.Bus.Subscribe(subCtx, stdIn, topic.SubscribeOpts{})
	if err != nil {
		return nil, fmt.Errorf("subscribe stdIn for ServiceRequest: %w", err)
	}

	req, _, err := awaitTypedPayload[payment.ServiceRequest](
		ctx, deliveries, payment.PayloadServiceRequest, "",
		2*time.Minute, "ServiceRequest")
	if err != nil {
		return nil, err
	}

	// Resolve buyer.stdIn from the ServiceRequest's `buyerStdIn` field.
	if req.BuyerStdIn == "" {
		return nil, errors.New("ServiceRequest missing buyerStdIn")
	}
	buyerStdIn, err := topic.NewTopicRef(cfg.Bus.SupportedTransport(), req.BuyerStdIn)
	if err != nil {
		return nil, fmt.Errorf("buyerStdIn ref: %w", err)
	}

	// Now hand off to SellerObserveAndAccept by re-publishing the
	// captured ServiceRequest into a fresh in-process channel. Simpler:
	// build the SellerSession directly from the captured req.
	sellerEVM := cfg.PrivateKey.PublicKey().EVMAddress().Hex()
	binding := escrowBindingOf(cfg.Escrow)
	sscfg := SellerSessionConfig{
		Bus:           cfg.Bus,
		Key:           cfg.PrivateKey,
		SellerStdIn:   stdIn,
		BuyerStdIn:    buyerStdIn,
		Escrow:        cfg.Escrow,
		SellerEVM:     sellerEVM,
		EscrowBinding: binding,
		Logger:        logger,
	}
	return sellerObserveFromCaptured(ctx, sscfg, req, deliveries)
}

// escrowBindingOf reports the canonical Binding string for an EscrowAdapter
// instance. Used by the seller-side commerce gate which receives only the
// escrow's locator string (not the full EscrowRef) from the buyer's
// EscrowCreated payload, but still needs to construct a valid EscrowRef
// when calling RequestRelease.
//
// Returns "" for a nil adapter (commerce gate disabled).
func escrowBindingOf(a payment.EscrowAdapter) string {
	switch a.(type) {
	case nil:
		return ""
	case *payment.MemoryEscrow:
		return "memory"
	default:
		return "evm-escrow"
	}
}

// sellerObserveFromCaptured continues the seller-side negotiation flow
// after a ServiceRequest has already been observed (and the deliveries
// channel still has remaining messages). It publishes ServiceResponse
// (accept), computes agreementHash, and waits for EscrowCreated on the
// same deliveries channel.
//
// Mirrors the post-ServiceRequest portion of SellerObserveAndAccept; we
// can't call that function directly because it does its own subscription
// from scratch, which would race with the live subscription we already hold.
func sellerObserveFromCaptured(
	ctx context.Context,
	cfg SellerSessionConfig,
	req payment.ServiceRequest,
	deliveries <-chan topic.MessageDelivery,
) (*SellerSession, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = nopLogger{}
	}
	state := payment.NewAgreementStateMachine(req.RequestID)
	if _, err := state.Transition(payment.EventServiceRequest); err != nil {
		return nil, fmt.Errorf("seller: transition to REQUESTED: %w", err)
	}
	logger.Printf("[commerce:seller] ServiceRequest received requestID=%s", req.RequestID)

	resp := payment.ServiceResponse{
		Type:      payment.PayloadServiceResponse,
		Version:   "1.0.0",
		RequestID: req.RequestID,
		Action:    "accept",
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("seller: marshal ServiceResponse: %w", err)
	}
	if err := publishCommerce(cfg.Bus, cfg.Key, cfg.BuyerStdIn, resp); err != nil {
		return nil, fmt.Errorf("seller: publish ServiceResponse: %w", err)
	}
	if _, err := state.Transition(payment.EventAccept); err != nil {
		return nil, fmt.Errorf("seller: transition to AGREED: %w", err)
	}
	hash := payment.ComputeAgreementHash(respBytes)
	logger.Printf("[commerce:seller] ServiceResponse accepted")

	created, _, err := awaitTypedPayload[payment.EscrowCreated](
		ctx, deliveries, payment.PayloadEscrowCreated, req.RequestID,
		2*time.Minute, "EscrowCreated")
	if err != nil {
		return nil, err
	}
	if _, err := state.Transition(payment.EventEscrowCreated); err != nil {
		return nil, fmt.Errorf("seller: transition to FUNDED: %w", err)
	}
	logger.Printf("[commerce:seller] EscrowCreated observed ref=%s", created.EscrowRef)

	binding := cfg.EscrowBinding
	if binding == "" && cfg.Escrow != nil {
		binding = "evm-escrow"
	} else if binding == "" {
		binding = "memory"
	}
	return &SellerSession{
		cfg:           cfg,
		state:         state,
		hash:          hash,
		requestID:     req.RequestID,
		currency:      req.ProposedCurrency,
		amount:        req.ProposedAmount,
		escrowRef:     created.EscrowRef,
		escrowBinding: binding,
		deliveryStart: time.Now(),
	}, nil
}
