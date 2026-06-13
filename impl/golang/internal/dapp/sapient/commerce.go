package sapient

// SAPIENT rid full-payment orchestration (008 lifecycle) for the
// reverse-connect topology: the BUYER listens and publishes its
// ECIES-encrypted multiaddrs in the ConnectionSetup; the SELLER decrypts and
// dials /sapient/detection/2.0.0 — and ONLY after it has verified the
// buyer's escrow funding on the settlement layer (the pre-stream gate).
//
// Message plumbing (Publish*/Receive* helpers, Lifecycle wrapper around
// payment.AgreementStateMachine, ECIES ConnectionSetup build/decrypt) is
// reused from internal/dapp/remoteid — those helpers are role-agnostic 008
// machinery operating on payment.* + topic.* types only. The remoteid
// sources are NOT modified.
//
// Sequence (buyer left, seller right; topics = seller stdIn + buyer stdIn):
//
//	ServiceRequest ───────────────▶ (await on seller stdIn)
//	(await) ◀─────────────── ServiceResponse(accept)
//	mint shortfall + CreateEscrow + Deposit
//	EscrowCreated ────────────────▶ escrow.GetBalance ≥ amount  [escrow-verify]
//	ConnectionSetup(buyer addrs,
//	  streams[seller-initiates]) ─▶ decrypt → cleared to dial
//	(admit stream: PeerID+FUNDED) ◀── libp2p dial + DetectionReport frames
//	ServiceStop ──────────────────▶ evidenceHash → RequestRelease
//	(await) ◀─────────────────────── Invoice
//	ApproveRelease (+withdraw)
//	InvoiceAck(approved) ─────────▶ verify release → COMPLETED
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Commerce-mode values for the SAPIENT seller/buyer CLIs (008 FR-P58 subset).
const (
	CommerceModeOff  = "off"
	CommerceModeFull = "full"

	// SettlementBindingMemory / SettlementBindingEVM are the adapter-level
	// escrow binding strings (mirroring remoteid's conventions).
	SettlementBindingMemory = "memory"
	SettlementBindingEVM    = "evm-escrow"

	// DefaultPricingCurrency is the demo settlement currency — the NTT
	// TestToken deployed alongside the escrow contract.
	DefaultPricingCurrency = "NTT"
)

// TopicLocatorFor extracts the resolvable topic locator ("topicId") for a
// standard channel from a resolved Agent Card. Returns "" when absent (an
// advertisement-only card carries no locators).
func TopicLocatorFor(uri registry.AgentURI, channel string) string {
	for _, ts := range uri.TopicServices() {
		if ts.Channel != channel {
			continue
		}
		if v, ok := ts.Config["topicId"].(string); ok {
			return v
		}
	}
	return ""
}

// parseAmount parses the decimal-string amounts the 008 payloads carry.
func parseAmount(s string) (*big.Int, error) {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Sign() < 0 {
		return nil, fmt.Errorf("invalid amount %q (want non-negative decimal integer)", s)
	}
	return v, nil
}

// ─── Seller side ───────────────────────────────────────────────────────────

// SellerCommerceOptions configures the seller's per-request commerce session.
type SellerCommerceOptions struct {
	// Key signs every outbound TopicMessage AND is the seller identity
	// (PeerID / EVM / node_id) — the escrow release recipient.
	Key *keylib.NeuronPrivateKey

	// Adapter is the topic bus shared with the buyer (memory or HCS).
	Adapter topic.TopicAdapter

	// SellerStdIn is the topic the buyer publishes ServiceRequest /
	// EscrowCreated / ConnectionSetup / ServiceStop / InvoiceAck on. The
	// locator must match the card's stdIn topicId.
	SellerStdIn topic.TopicRef

	// Host is the seller's libp2p host — the DIALER in reverse-connect.
	Host host.Host

	// Escrow is the settlement adapter (MemoryEscrow or EVMEscrowAdapter).
	Escrow payment.EscrowAdapter

	// EscrowBinding is the adapter-level binding string ("memory" or
	// "evm-escrow"). Defaults to "memory".
	EscrowBinding string

	// TokenBalance, when non-nil, returns the seller's settlement-token
	// balance — sampled before/after settlement so the seller can VERIFY it
	// actually received the release (evm runs wire TestToken.BalanceOf).
	TokenBalance func(ctx context.Context) (*big.Int, error)

	// Logger receives the [seller-commerce] / [escrow-verify] / [settle]
	// evidence lines. Optional.
	Logger *log.Logger
}

// SellerCommerceResult is the seller-side evidence trace.
type SellerCommerceResult struct {
	RequestID         string
	BuyerEVM          string
	BuyerPeerID       string
	EscrowRef         string
	EscrowAvailable   string // observed at the pre-stream gate
	EvidenceHash      string
	ReleaseRequestRef string
	InvoiceAckAction  string
	TokenDelta        string // seller token-balance increase across settlement ("" when unverifiable)
	FinalState        payment.AgreementState
}

// SellerCommerceSession is the live state between the cleared-to-dial point
// and settlement. The CLI dials BuyerAddr, streams DetectionReports, then
// calls Finalise.
type SellerCommerceSession struct {
	// BuyerAddr is the buyer's decrypted AddrInfo — the dial target.
	BuyerAddr peer.AddrInfo

	// Request is the accepted ServiceRequest (pricing, requestID).
	Request payment.ServiceRequest

	opts         SellerCommerceOptions
	logger       *log.Logger
	lc           *remoteid.Lifecycle
	buyerStdIn   topic.TopicRef
	escrowRef    payment.EscrowRef
	seq          uint64
	tokenBefore  *big.Int
	stopCh       <-chan topic.MessageDelivery
	stopCancel   context.CancelFunc
	partialState SellerCommerceResult
}

// StartSellerCommerce runs the seller's commerce flow up to the
// cleared-to-dial point: await ServiceRequest → accept → await EscrowCreated
// → VERIFY escrow funding (GetBalance ≥ amount — the pre-stream gate; the
// seller refuses to stream against an underfunded escrow) → await the
// buyer's reverse ConnectionSetup → decrypt the dial target.
func StartSellerCommerce(ctx context.Context, opts SellerCommerceOptions) (*SellerCommerceSession, error) {
	if opts.Key == nil {
		return nil, errors.New("sapient.StartSellerCommerce: Key required")
	}
	if opts.Adapter == nil {
		return nil, errors.New("sapient.StartSellerCommerce: Adapter required")
	}
	if opts.Host == nil {
		return nil, errors.New("sapient.StartSellerCommerce: Host required")
	}
	if opts.Escrow == nil {
		return nil, errors.New("sapient.StartSellerCommerce: Escrow required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(discard{}, "", 0)
	}
	binding := opts.EscrowBinding
	if binding == "" {
		binding = SettlementBindingMemory
	}

	// Step 1: ServiceRequest.
	logger.Printf("[seller-commerce] awaiting serviceRequest on %s", opts.SellerStdIn.Locator())
	req, reqMsg, err := remoteid.ReceiveServiceRequest(ctx, opts.Adapter, opts.SellerStdIn)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: receive serviceRequest: %w", err)
	}
	if req.ServiceRef != P2PServiceName {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: serviceRequest.serviceRef %q != %q — not a SAPIENT rid request",
			req.ServiceRef, P2PServiceName)
	}
	logger.Printf("[seller-commerce] serviceRequest requestID=%s buyerStdIn=%s amount=%s %s",
		req.RequestID, req.BuyerStdIn, req.ProposedAmount, req.ProposedCurrency)

	lc, err := remoteid.NewLifecycle(remoteid.LifecycleOptions{
		RequestID: req.RequestID,
		Mode:      remoteid.CommerceModeFull,
		Logger:    logger,
	})
	if err != nil {
		return nil, err
	}
	if err := lc.Receive(req); err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: lifecycle receive: %w", err)
	}

	// The buyer's pubkey, recovered from its signed ServiceRequest, pins BOTH
	// the ECIES recipient identity and the expected PeerID of the reverse
	// ConnectionSetup (and later the inbound-dial admission on the buyer side
	// mirrors this with the seller's identity).
	buyerPubECDSA, err := remoteid.ExtractSenderECDSAPublicKey(reqMsg)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: recover buyer pubkey: %w", err)
	}
	buyerPub, err := keylib.NeuronPublicKeyFromBlockchainKey(buyerPubECDSA)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: wrap buyer pubkey: %w", err)
	}
	buyerPeerID, err := buyerPub.PeerID()
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: derive buyer peerID: %w", err)
	}

	buyerStdIn, err := topic.NewTopicRef(opts.SellerStdIn.Transport(), req.BuyerStdIn)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: parse buyerStdIn: %w", err)
	}

	// Step 2: accept.
	var seq uint64 = 1
	resp := remoteid.BuildServiceResponse(req, "accept")
	if _, err := remoteid.PublishPayload(ctx, opts.Adapter, buyerStdIn, opts.Key, seq, resp); err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: publish serviceResponse: %w", err)
	}
	seq++
	if err := lc.Accept(); err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: lifecycle accept: %w", err)
	}

	// Step 3: EscrowCreated.
	logger.Printf("[seller-commerce] awaiting escrowCreated")
	created, _, err := remoteid.ReceiveEscrowCreated(ctx, opts.Adapter, opts.SellerStdIn, req.RequestID)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: receive escrowCreated: %w", err)
	}
	escrowRef := payment.EscrowRef{Binding: binding, Locator: created.EscrowRef}
	if err := lc.Funded(); err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: lifecycle funded: %w", err)
	}

	// Step 4: THE PRE-STREAM GATE — verify the escrow is actually funded on
	// the settlement layer before a single DetectionReport leaves the seller.
	bal, err := opts.Escrow.GetBalance(ctx, escrowRef)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: escrow.GetBalance: %w", err)
	}
	available, err := parseAmount(bal.Available)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: escrow balance: %w", err)
	}
	required, err := parseAmount(req.ProposedAmount)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: proposed amount: %w", err)
	}
	if available.Cmp(required) < 0 {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: [escrow-verify] FAIL escrowRef=%s available=%s required=%s — refusing to stream against an underfunded escrow",
			created.EscrowRef, bal.Available, req.ProposedAmount)
	}
	logger.Printf("[escrow-verify] PASS escrowRef=%s available=%s required=%s — cleared to stream",
		created.EscrowRef, bal.Available, req.ProposedAmount)

	// Step 5: the buyer's reverse ConnectionSetup — its encrypted multiaddrs,
	// decrypted with the seller key; PeerID pinned to the ServiceRequest
	// signer.
	sellerECDSA, err := opts.Key.ToBlockchainKey()
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: seller key: %w", err)
	}
	setup, buyerAddr, err := remoteid.ReceiveConnectionSetup(ctx, opts.Adapter, opts.SellerStdIn,
		req.RequestID, buyerPeerID.String(), sellerECDSA)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: receive reverse connectionSetup: %w", err)
	}
	logger.Printf("[seller-commerce] reverse connectionSetup: buyer peerID=%s addrs=%d streams=%d",
		setup.PeerID, len(buyerAddr.Addrs), len(setup.Streams))

	if err := lc.StartDelivery(); err != nil {
		return nil, fmt.Errorf("sapient.StartSellerCommerce: lifecycle startDelivery: %w", err)
	}

	// Pre-open the ServiceStop subscription BEFORE the caller starts
	// streaming so a fast buyer's stop cannot race past Finalise.
	stopCtx, stopCancel := context.WithCancel(ctx)
	fromZero := uint64(0)
	stopCh, err := opts.Adapter.Subscribe(stopCtx, opts.SellerStdIn, topic.SubscribeOpts{FromSequence: &fromZero})
	if err != nil {
		stopCancel()
		return nil, fmt.Errorf("sapient.StartSellerCommerce: subscribe for serviceStop: %w", err)
	}

	var tokenBefore *big.Int
	if opts.TokenBalance != nil {
		if v, terr := opts.TokenBalance(ctx); terr == nil {
			tokenBefore = v
		} else {
			logger.Printf("[settle] token balance probe (before) failed: %v", terr)
		}
	}

	return &SellerCommerceSession{
		BuyerAddr:   buyerAddr,
		Request:     req,
		opts:        opts,
		logger:      logger,
		lc:          lc,
		buyerStdIn:  buyerStdIn,
		escrowRef:   escrowRef,
		seq:         seq,
		tokenBefore: tokenBefore,
		stopCh:      stopCh,
		stopCancel:  stopCancel,
		partialState: SellerCommerceResult{
			RequestID:       req.RequestID,
			BuyerEVM:        reqMsg.SenderAddress(),
			BuyerPeerID:     buyerPeerID.String(),
			EscrowRef:       created.EscrowRef,
			EscrowAvailable: bal.Available,
		},
	}, nil
}

// Finalise completes the seller's settlement after the data plane is done:
// await ServiceStop → evidenceHash over the stream summary → RequestRelease
// (recipient = the SELLER — this is a paid service, not the remoteid free
// demo) → Invoice → await InvoiceAck → verify the release landed.
func (s *SellerCommerceSession) Finalise(ctx context.Context, frameSummary func() (uint64, uint64, uint64)) (SellerCommerceResult, error) {
	result := s.partialState
	defer s.stopCancel()

	// Step 6: ServiceStop on the pre-opened subscription.
	s.logger.Printf("[seller-commerce] awaiting serviceStop")
	if err := awaitTyped(ctx, s.stopCh, payment.PayloadServiceStop, s.Request.RequestID, nil); err != nil {
		return result, fmt.Errorf("sapient.SellerCommerce.Finalise: receive serviceStop: %w", err)
	}
	s.logger.Printf("[seller-commerce] serviceStop received; settling")

	// Step 7: evidenceHash binds the release to the delivered stream.
	var frameCount, firstAt, lastAt uint64
	if frameSummary != nil {
		frameCount, firstAt, lastAt = frameSummary()
	}
	result.EvidenceHash = remoteid.EvidenceHashFor(s.opts.Host.ID().String(), frameCount, firstAt, lastAt)

	// Step 8: RequestRelease — recipient is the SELLER's EVM address (the
	// service is paid; the released funds land with the seller).
	releaseRef, err := s.opts.Escrow.RequestRelease(ctx, s.escrowRef,
		s.Request.ProposedAmount, s.opts.Key.PublicKey().EVMAddress().Hex(), evidenceHashTo32(result.EvidenceHash))
	if err != nil {
		return result, fmt.Errorf("sapient.SellerCommerce.Finalise: escrow.RequestRelease: %w", err)
	}
	result.ReleaseRequestRef = releaseRef.Locator
	s.logger.Printf("[settle] release requested ref=%s amount=%s recipient=%s evidenceHash=%s",
		releaseRef.Locator, s.Request.ProposedAmount, s.opts.Key.PublicKey().EVMAddress().Hex(), result.EvidenceHash)

	// Step 9: Invoice → buyer.
	invoice, err := remoteid.BuildInvoice(remoteid.InvoiceOptions{
		Request:           s.Request,
		EscrowRef:         result.EscrowRef,
		ReleaseRequestRef: releaseRef.Locator,
		EvidenceHash:      result.EvidenceHash,
	})
	if err != nil {
		return result, fmt.Errorf("sapient.SellerCommerce.Finalise: build invoice: %w", err)
	}
	if _, err := remoteid.PublishInvoice(ctx, s.opts.Adapter, s.buyerStdIn, s.opts.Key, s.seq, invoice); err != nil {
		return result, fmt.Errorf("sapient.SellerCommerce.Finalise: publish invoice: %w", err)
	}
	s.seq++
	if err := s.lc.BeginInvoice(); err != nil {
		return result, fmt.Errorf("sapient.SellerCommerce.Finalise: lifecycle beginInvoice: %w", err)
	}

	// Step 10: InvoiceAck.
	s.logger.Printf("[seller-commerce] awaiting invoiceAck")
	ack, _, err := remoteid.ReceiveInvoiceAck(ctx, s.opts.Adapter, s.opts.SellerStdIn, s.Request.RequestID)
	if err != nil {
		return result, fmt.Errorf("sapient.SellerCommerce.Finalise: receive invoiceAck: %w", err)
	}
	result.InvoiceAckAction = ack.Action
	if ack.Action != "approved" {
		if lerr := s.lc.RefuseInvoice(); lerr != nil {
			return result, fmt.Errorf("sapient.SellerCommerce.Finalise: lifecycle refuseInvoice: %w", lerr)
		}
		result.FinalState = s.lc.State()
		return result, fmt.Errorf("sapient.SellerCommerce.Finalise: buyer refused invoice")
	}

	// Step 11: verify the release actually landed. The buyer's
	// ApproveRelease (+withdraw, pull-pattern) is authoritative; the
	// seller-side mirror call is a non-fatal trace-symmetry note (the evm
	// adapter's second approve typically reverts — already released).
	if rel, aerr := s.opts.Escrow.ApproveRelease(ctx, s.escrowRef, releaseRef); aerr != nil {
		s.logger.Printf("[settle] seller-side ApproveRelease note: %v", aerr)
	} else {
		s.logger.Printf("[settle] seller-side ApproveRelease tx=%s released=%s recipient=%s",
			rel.TransactionRef, rel.Released, rel.Recipient)
	}
	if s.opts.TokenBalance != nil && s.tokenBefore != nil {
		// Token settlement can lag the ack by a block; poll briefly.
		deadline := time.Now().Add(30 * time.Second)
		for {
			after, terr := s.opts.TokenBalance(ctx)
			if terr == nil {
				delta := new(big.Int).Sub(after, s.tokenBefore)
				if delta.Sign() > 0 || time.Now().After(deadline) {
					result.TokenDelta = delta.String()
					s.logger.Printf("[settle] seller token balance delta=%s (release verified=%v)",
						delta.String(), delta.Sign() > 0)
					break
				}
			} else if time.Now().After(deadline) {
				s.logger.Printf("[settle] token balance probe (after) failed: %v", terr)
				break
			}
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}

	if err := s.lc.ApproveInvoice(); err != nil {
		return result, fmt.Errorf("sapient.SellerCommerce.Finalise: lifecycle approveInvoice: %w", err)
	}
	if err := s.lc.Complete(); err != nil {
		return result, fmt.Errorf("sapient.SellerCommerce.Finalise: lifecycle complete: %w", err)
	}
	result.FinalState = s.lc.State()
	return result, nil
}

// ─── Buyer side ────────────────────────────────────────────────────────────

// BuyerCommerceOptions configures the buyer's commerce flow.
type BuyerCommerceOptions struct {
	// Key signs every outbound TopicMessage AND is the buyer identity.
	Key *keylib.NeuronPrivateKey

	// Host is the buyer's libp2p LISTENING host — its multiaddrs are
	// ECIES-encrypted into the reverse ConnectionSetup.
	Host host.Host

	// Adapter is the topic bus shared with the seller (memory or HCS).
	Adapter topic.TopicAdapter

	// Escrow is the settlement adapter; EscrowBinding mirrors the seller's.
	Escrow        payment.EscrowAdapter
	EscrowBinding string

	// Contract + RegistryAddress + ChainID locate the seller's Agent Card.
	Contract        registry.RegistryContract
	RegistryAddress keylib.EVMAddress
	ChainID         uint64

	// SellerEVM is the seller identity to engage (registry lookup key).
	SellerEVM keylib.EVMAddress

	// PricingAmount is the proposed amount. Empty = adopt the card's
	// advertised pricing.
	PricingAmount string

	// EnsureFunds, when non-nil, runs before CreateEscrow with the agreed
	// amount — the evm path mints the TestToken shortfall (auto-mint, demo
	// token). Returns an optional tx hash for evidence.
	EnsureFunds func(ctx context.Context, amount string) (string, error)

	// Logger receives the [buyer-commerce] / [escrow] / [settle] lines.
	Logger *log.Logger
}

// BuyerCommerceResult is the buyer-side evidence trace.
type BuyerCommerceResult struct {
	RequestID         string
	SellerEVM         string
	SellerAgentID     string
	SellerPeerID      string
	SellerStdIn       string
	BuyerStdIn        string
	PricingAmount     string
	PricingCurrency   string
	MintTx            string
	EscrowRef         string
	DepositTx         string
	InvoiceAmount     string
	InvoiceCurrency   string
	ReleaseRequestRef string
	ApproveTx         string
	ReleasedAmount    string
	ReleaseRecipient  string
	FinalAction       string
}

// BuyerCommerceSession is the live state between funding (cleared-to-admit)
// and settlement. The CLI admits the seller's inbound stream (PeerID +
// FUNDED-state gate), counts frames opaquely, then calls Finalise.
type BuyerCommerceSession struct {
	// ExpectedSellerPeerID is the admission identity: the inbound stream's
	// RemotePeer MUST match it.
	ExpectedSellerPeerID string

	// RequestID identifies the negotiation.
	RequestID string

	opts        BuyerCommerceOptions
	logger      *log.Logger
	sellerStdIn topic.TopicRef
	buyerStdIn  topic.TopicRef
	seq         uint64
	invoiceCh   <-chan topic.MessageDelivery
	invCancel   context.CancelFunc
	partial     BuyerCommerceResult
}

// StartBuyerCommerce drives the buyer's flow up to the cleared-to-admit
// point: resolve + VERIFY the seller's card from the registry → ServiceRequest
// → await accept (and pin the responder's recovered identity to the card) →
// ensure funds (auto-mint) → CreateEscrow + Deposit → EscrowCreated → publish
// the reverse ConnectionSetup carrying the buyer's encrypted multiaddrs.
func StartBuyerCommerce(ctx context.Context, opts BuyerCommerceOptions) (*BuyerCommerceSession, error) {
	if opts.Key == nil {
		return nil, errors.New("sapient.StartBuyerCommerce: Key required")
	}
	if opts.Host == nil {
		return nil, errors.New("sapient.StartBuyerCommerce: Host required")
	}
	if opts.Adapter == nil {
		return nil, errors.New("sapient.StartBuyerCommerce: Adapter required")
	}
	if opts.Escrow == nil {
		return nil, errors.New("sapient.StartBuyerCommerce: Escrow required")
	}
	if opts.Contract == nil {
		return nil, errors.New("sapient.StartBuyerCommerce: Contract required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(discard{}, "", 0)
	}
	binding := opts.EscrowBinding
	if binding == "" {
		binding = SettlementBindingMemory
	}

	// Step 1: resolve + verify the seller's card.
	reg, err := registry.LookupRegistration(ctx, opts.RegistryAddress, opts.ChainID,
		registry.ByEVMAddress(opts.SellerEVM), opts.Contract)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: lookup seller: %w", err)
	}
	uri := reg.AgentURI()
	for _, c := range VerifyResolvedCard(uri, opts.SellerEVM) {
		logger.Printf("[registry] card check [%s] %s — %s", c.Status, c.Name, c.Detail)
		if c.Status == CheckFail {
			return nil, fmt.Errorf("sapient.StartBuyerCommerce: card verification failed: %s — %s", c.Name, c.Detail)
		}
	}
	p2ps := uri.P2PServices()
	if len(p2ps) == 0 {
		return nil, errors.New("sapient.StartBuyerCommerce: seller card has no p2p service")
	}
	sellerPeerID := p2ps[0].PeerID
	tokenID := "-"
	if reg.TokenId() != nil {
		tokenID = reg.TokenId().String()
	}
	logger.Printf("[registry] discovered seller EVM=%s agentId=%s peerID=%s",
		opts.SellerEVM.Hex(), tokenID, sellerPeerID)

	// Pricing: adopt the card's advertised rid pricing unless overridden.
	amount := opts.PricingAmount
	currency := DefaultPricingCurrency
	for _, c := range uri.CommerceServices() {
		if c.Name == CommerceServiceName {
			if amount == "" {
				amount = c.Pricing.Amount
			}
			if c.Pricing.Currency != "" {
				currency = c.Pricing.Currency
			}
			break
		}
	}
	if amount == "" {
		amount = "1"
	}
	if binding == SettlementBindingEVM {
		if err := remoteid.ValidatePricingForEVM(amount); err != nil {
			return nil, fmt.Errorf("sapient.StartBuyerCommerce: %w", err)
		}
	}

	sellerStdInLoc := TopicLocatorFor(uri, "stdIn")
	if sellerStdInLoc == "" {
		return nil, errors.New("sapient.StartBuyerCommerce: seller card has no stdIn topic locator (advertisement-only card? seller must run --commerce-mode=full)")
	}
	sellerStdIn, err := topic.NewTopicRef(opts.Adapter.SupportedTransport(), sellerStdInLoc)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: seller stdIn ref: %w", err)
	}

	buyerStdIn, err := opts.Adapter.CreateTopic(topic.CreateTopicOpts{
		Memo: "sapient-buyer-stdin-" + opts.Key.PublicKey().EVMAddress().Hex()[2:10],
	})
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: create buyerStdIn: %w", err)
	}

	requestID := fmt.Sprintf("sapient-buyer-%s-%d", opts.Key.PublicKey().EVMAddress().Hex()[2:10], time.Now().UnixNano())
	req, err := remoteid.BuildServiceRequest(remoteid.ServiceRequestOptions{
		RequestID:         requestID,
		ServiceRef:        P2PServiceName,
		SettlementBinding: binding,
		ProposedAmount:    amount,
		ProposedCurrency:  currency,
		BuyerStdIn:        buyerStdIn.Locator(),
	})
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: build serviceRequest: %w", err)
	}

	result := BuyerCommerceResult{
		RequestID:       requestID,
		SellerEVM:       opts.SellerEVM.Hex(),
		SellerAgentID:   tokenID,
		SellerPeerID:    sellerPeerID,
		SellerStdIn:     sellerStdInLoc,
		BuyerStdIn:      buyerStdIn.Locator(),
		PricingAmount:   amount,
		PricingCurrency: currency,
	}

	// Step 2: ServiceRequest → await accept.
	var seq uint64 = 1
	if _, err := remoteid.PublishPayload(ctx, opts.Adapter, sellerStdIn, opts.Key, seq, req); err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: publish serviceRequest: %w", err)
	}
	seq++
	resp, respMsg, err := remoteid.ReceiveServiceResponse(ctx, opts.Adapter, buyerStdIn, requestID)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: receive serviceResponse: %w", err)
	}
	if resp.Action != "accept" {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: seller %s", resp.Action)
	}

	// Identity pin: the entity that ACCEPTED must be the registered seller —
	// recover its pubkey from the signed ServiceResponse and require the
	// derived PeerID to equal the card's. This is also the ECIES recipient
	// for the reverse ConnectionSetup.
	sellerPubECDSA, err := remoteid.ExtractSenderECDSAPublicKey(respMsg)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: recover seller pubkey: %w", err)
	}
	sellerPub, err := keylib.NeuronPublicKeyFromBlockchainKey(sellerPubECDSA)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: wrap seller pubkey: %w", err)
	}
	respPeerID, err := sellerPub.PeerID()
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: derive responder peerID: %w", err)
	}
	if respPeerID.String() != sellerPeerID {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: serviceResponse signer peerID=%s != registered=%s — refusing to fund",
			respPeerID, sellerPeerID)
	}

	// Step 3: funds — auto-mint the shortfall (demo TestToken), then escrow.
	if opts.EnsureFunds != nil {
		mintTx, ferr := opts.EnsureFunds(ctx, amount)
		if ferr != nil {
			return nil, fmt.Errorf("sapient.StartBuyerCommerce: ensure funds: %w", ferr)
		}
		result.MintTx = mintTx
		if mintTx != "" {
			logger.Printf("[escrow] minted settlement-token shortfall mintTx=%s", mintTx)
		}
	}
	buyerEVM := opts.Key.PublicKey().EVMAddress().Hex()
	agreementHash := sha256.Sum256([]byte(requestID + "|" + buyerEVM + "|" + opts.SellerEVM.Hex()))
	escrowRef, err := opts.Escrow.CreateEscrow(ctx, buyerEVM, opts.SellerEVM.Hex(), nil,
		currency, uint64(1), agreementHash, uint64(time.Now().Unix())+3600)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: escrow.CreateEscrow: %w", err)
	}
	result.EscrowRef = escrowRef.Locator
	dep, err := opts.Escrow.Deposit(ctx, escrowRef, amount)
	if err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: escrow.Deposit: %w", err)
	}
	result.DepositTx = dep.TransactionRef
	logger.Printf("[escrow] created escrowRef=%s deposited=%s %s depositTx=%s",
		escrowRef.Locator, amount, currency, dep.TransactionRef)

	created := remoteid.BuildEscrowCreated(req, escrowRef.Locator, amount, currency)
	if _, err := remoteid.PublishEscrowCreated(ctx, opts.Adapter, sellerStdIn, opts.Key, seq, created); err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: publish escrowCreated: %w", err)
	}
	seq++

	// Step 4: the REVERSE ConnectionSetup — the buyer (listener) publishes
	// its own multiaddrs, ECIES-encrypted to the seller, with the streams[]
	// catalog declaring seller-initiates (013 v2 stream-init-direction).
	streams := BuildSapientStreamCatalog(DefaultCatalogOptions())
	if _, err := remoteid.PublishConnectionSetup(ctx, opts.Adapter, sellerStdIn, opts.Key, seq,
		requestID, opts.Host, sellerPubECDSA, streams); err != nil {
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: publish reverse connectionSetup: %w", err)
	}
	seq++
	logger.Printf("[buyer-commerce] reverse connectionSetup published (buyer listens; seller dials) requestID=%s", requestID)

	// Pre-open the Invoice subscription so the seller's invoice cannot race
	// past Finalise (remoteid.FinaliseBuyerSession discipline).
	invCtx, invCancel := context.WithCancel(ctx)
	fromZero := uint64(0)
	invoiceCh, err := opts.Adapter.Subscribe(invCtx, buyerStdIn, topic.SubscribeOpts{FromSequence: &fromZero})
	if err != nil {
		invCancel()
		return nil, fmt.Errorf("sapient.StartBuyerCommerce: subscribe for invoice: %w", err)
	}

	return &BuyerCommerceSession{
		ExpectedSellerPeerID: sellerPeerID,
		RequestID:            requestID,
		opts:                 opts,
		logger:               logger,
		sellerStdIn:          sellerStdIn,
		buyerStdIn:           buyerStdIn,
		seq:                  seq,
		invoiceCh:            invoiceCh,
		invCancel:            invCancel,
		partial:              result,
	}, nil
}

// Finalise completes the buyer's settlement after the data plane is done:
// publish ServiceStop → await Invoice → ApproveRelease (authoritative; the
// evm adapter withdraws to the recipient in the same call) → InvoiceAck.
func (b *BuyerCommerceSession) Finalise(ctx context.Context) (BuyerCommerceResult, error) {
	result := b.partial
	defer b.invCancel()

	// Step 5: ServiceStop.
	stop := remoteid.BuildServiceStop(payment.ServiceRequest{RequestID: b.RequestID}, "buyer-session-end", 0)
	if _, err := remoteid.PublishServiceStop(ctx, b.opts.Adapter, b.sellerStdIn, b.opts.Key, b.seq, stop); err != nil {
		return result, fmt.Errorf("sapient.BuyerCommerce.Finalise: publish serviceStop: %w", err)
	}
	b.seq++
	b.logger.Printf("[settle] published serviceStop requestID=%s", b.RequestID)

	// Step 6: Invoice on the pre-opened subscription.
	var inv payment.Invoice
	if err := awaitTyped(ctx, b.invoiceCh, payment.PayloadInvoice, b.RequestID, &inv); err != nil {
		return result, fmt.Errorf("sapient.BuyerCommerce.Finalise: receive invoice: %w", err)
	}
	result.InvoiceAmount = inv.Amount
	result.InvoiceCurrency = inv.Currency
	result.ReleaseRequestRef = inv.ReleaseRequestRef
	b.logger.Printf("[settle] received invoice amount=%s %s releaseRef=%s escrowRef=%s",
		inv.Amount, inv.Currency, inv.ReleaseRequestRef, inv.EscrowRef)

	// Step 7: ApproveRelease — authoritative settlement (the evm adapter
	// chains approveRelease + withdraw; funds land with the recipient the
	// seller named, i.e. the seller itself).
	binding := b.opts.EscrowBinding
	if binding == "" {
		binding = SettlementBindingMemory
	}
	rel, err := b.opts.Escrow.ApproveRelease(ctx,
		payment.EscrowRef{Binding: binding, Locator: result.EscrowRef},
		payment.ReleaseRequestRef{Binding: binding, Locator: inv.ReleaseRequestRef})
	if err != nil {
		return result, fmt.Errorf("sapient.BuyerCommerce.Finalise: escrow.ApproveRelease: %w", err)
	}
	result.ApproveTx = rel.TransactionRef
	result.ReleasedAmount = rel.Released
	result.ReleaseRecipient = rel.Recipient
	b.logger.Printf("[settle] approveRelease tx=%s released=%s recipient=%s",
		rel.TransactionRef, rel.Released, rel.Recipient)

	// Step 8: InvoiceAck(approved).
	ack := remoteid.BuildInvoiceAck(inv, "approved")
	if _, err := remoteid.PublishInvoiceAck(ctx, b.opts.Adapter, b.sellerStdIn, b.opts.Key, b.seq, ack); err != nil {
		return result, fmt.Errorf("sapient.BuyerCommerce.Finalise: publish invoiceAck: %w", err)
	}
	b.logger.Printf("[settle] published invoiceAck action=approved requestID=%s", b.RequestID)
	result.FinalAction = "approved"
	return result, nil
}

// evidenceHashTo32 converts the hex-encoded SHA-256 string into the [32]byte
// form payment.EscrowAdapter.RequestRelease expects (mirror of remoteid's
// unexported helper).
func evidenceHashTo32(hexHash string) [32]byte {
	var out [32]byte
	if decoded, err := hex.DecodeString(hexHash); err == nil && len(decoded) >= 32 {
		copy(out[:], decoded[:32])
		return out
	}
	return sha256.Sum256([]byte(hexHash))
}

// awaitTyped consumes deliveries from a pre-opened subscription until a
// payload with the wanted type + requestId arrives, optionally decoding it
// into target.
func awaitTyped(ctx context.Context, ch <-chan topic.MessageDelivery, payloadType, requestID string, target any) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-ch:
			if !ok {
				return fmt.Errorf("subscription closed before %s arrived", payloadType)
			}
			var probe struct {
				Type      string `json:"type"`
				RequestID string `json:"requestId"`
			}
			body := d.Message.Payload()
			if err := json.Unmarshal(body, &probe); err != nil {
				continue
			}
			if probe.Type != payloadType || probe.RequestID != requestID {
				continue
			}
			if target != nil {
				if err := json.Unmarshal(body, target); err != nil {
					return fmt.Errorf("decode %s: %w", payloadType, err)
				}
			}
			return nil
		}
	}
}

// discard is a no-op writer for the default logger.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
