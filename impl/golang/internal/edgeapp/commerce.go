package edgeapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// BuyerSession captures the buyer-side commerce state for one agreement.
//
// Iteration 2 lands the **mock-only** path. The buyer drives a single agreement
// per session — negotiate → fund → (data flows in parallel via RunBuyer) →
// settle on graceful close. Subscription metering is deferred indefinitely
// per plan §11.
//
// All on-chain (testnet) settlement remains gated behind
// `NEURON_EDGE_PAYMENT_MODE=testnet`, which the iteration 2 dispatch returns
// ErrFeatureNotImplemented for. The MemoryEscrow path is exercised by the
// test suite.
type BuyerSession struct {
	cfg       BuyerSessionConfig
	state     *payment.AgreementStateMachine
	hash      [32]byte
	escrow    payment.EscrowAdapter
	escrowRef payment.EscrowRef
	startedAt time.Time
}

// BuyerSessionConfig configures BuyerNegotiateAndFund.
type BuyerSessionConfig struct {
	Bus              topic.TopicAdapter
	Key              *keylib.NeuronPrivateKey
	BuyerStdIn       topic.TopicRef // buyer publishes ack/settlement here? actually seller publishes Invoice here. See doc on each step.
	SellerStdIn      topic.TopicRef // buyer publishes ServiceRequest + EscrowCreated here
	RequestID        string
	ServiceRef       string
	BuyerEVM         string
	SellerEVM        string
	Currency         string
	Price            string
	NegotiationTTL   time.Duration
	AgreementTimeout time.Duration
	Escrow           payment.EscrowAdapter
	Logger           Logger
}

// SellerSessionConfig configures SellerObserveAndAccept.
type SellerSessionConfig struct {
	Bus         topic.TopicAdapter
	Key         *keylib.NeuronPrivateKey
	SellerStdIn topic.TopicRef // seller subscribes here for ServiceRequest + EscrowCreated
	BuyerStdIn  topic.TopicRef // seller publishes ServiceResponse + Invoice here
	// Escrow, when non-nil, is the seller-side escrow adapter used to
	// pre-create the on-chain release request before publishing Invoice.
	// NeuronEscrow's `requestRelease` requires msg.sender to be the seller
	// (or arbiter), so this MUST be wired with the seller's signing key.
	// When nil, IssueInvoice publishes a placeholder ReleaseRequestRef
	// and relies on the buyer's mock-mode flow (MemoryEscrow accepts
	// either party as caller).
	Escrow payment.EscrowAdapter
	// SellerEVM is the seller's recipient address — needed when the seller
	// calls RequestRelease (recipient = seller's address). Must match
	// the address passed to CreateEscrow as `seller` on the buyer side.
	SellerEVM string
	// EscrowBinding identifies the escrow's binding (e.g. "evm-escrow"
	// for NeuronEscrow on EVM, "memory" for MemoryEscrow). Used to
	// reconstruct the payment.EscrowRef when calling RequestRelease.
	// Defaults to "evm-escrow" when Escrow is non-nil and this is empty.
	EscrowBinding string
	Logger        Logger
}

// SellerSession is the seller's side of the same agreement.
type SellerSession struct {
	cfg            SellerSessionConfig
	state          *payment.AgreementStateMachine
	hash           [32]byte
	requestID      string
	currency       string
	amount         string
	escrowRef      string
	escrowBinding  string // "evm-escrow" / "memory" — used to construct payment.EscrowRef
	deliveryStart  time.Time
}

// BuyerNegotiateAndFund runs the buyer side of negotiation + escrow funding.
//
// Steps (all over the configured topic.Bus):
//
//  1. Build + publish ServiceRequest on SellerStdIn.
//  2. Subscribe to BuyerStdIn; wait for ServiceResponse with action=accept.
//  3. Compute agreementHash = keccak256(canonical(ServiceResponse)).
//  4. Call escrow.CreateEscrow + escrow.Deposit.
//  5. Build + publish EscrowCreated on SellerStdIn.
//  6. Return BuyerSession with state=FUNDED (ready for caller to start data
//     plane and eventually call Settle).
//
// The agreement state machine ends this function in StateFunded. A subsequent
// call to s.Settle takes it through ACTIVE → INVOICED → ACTIVE on the
// successful settlement path.
func BuyerNegotiateAndFund(ctx context.Context, cfg BuyerSessionConfig) (*BuyerSession, error) {
	if err := validateBuyerCfg(&cfg); err != nil {
		return nil, err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = nopLogger{}
	}

	state := payment.NewAgreementStateMachine(cfg.RequestID)

	// Step 1: publish ServiceRequest on seller.stdIn.
	deadline := uint64(time.Now().Add(cfg.NegotiationTTL).Unix())
	state.SetNegotiationDeadline(deadline)

	req := payment.ServiceRequest{
		Type:                payment.PayloadServiceRequest,
		Version:             "1.0.0",
		RequestID:           cfg.RequestID,
		ServiceRef:          cfg.ServiceRef,
		SettlementBinding:   "memory",
		ProposedAmount:      cfg.Price,
		ProposedCurrency:    cfg.Currency,
		ProposedInterval:    "PT1H",
		NegotiationDeadline: deadline,
		BuyerStdIn:          cfg.BuyerStdIn.Locator(),
	}

	if err := publishCommerce(cfg.Bus, cfg.Key, cfg.SellerStdIn, req); err != nil {
		return nil, fmt.Errorf("buyer: publish ServiceRequest: %w", err)
	}
	if _, err := state.Transition(payment.EventServiceRequest); err != nil {
		return nil, fmt.Errorf("buyer: transition to REQUESTED: %w", err)
	}
	logger.Printf("[commerce:buyer] ServiceRequest sent (requestID=%s, price=%s %s)",
		cfg.RequestID, cfg.Price, cfg.Currency)

	// Step 2: wait for ServiceResponse on buyer.stdIn.
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	deliveries, err := cfg.Bus.Subscribe(subCtx, cfg.BuyerStdIn, topic.SubscribeOpts{})
	if err != nil {
		return nil, fmt.Errorf("buyer: subscribe stdIn: %w", err)
	}

	resp, respBytes, err := awaitTypedPayload[payment.ServiceResponse](
		ctx, deliveries, payment.PayloadServiceResponse, cfg.RequestID,
		cfg.NegotiationTTL, "ServiceResponse")
	if err != nil {
		return nil, err
	}
	if resp.Action != "accept" {
		return nil, fmt.Errorf("buyer: seller did not accept (action=%q)", resp.Action)
	}

	if _, err := state.Transition(payment.EventAccept); err != nil {
		return nil, fmt.Errorf("buyer: transition to AGREED: %w", err)
	}
	logger.Printf("[commerce:buyer] ServiceResponse accepted")

	// Step 3: compute agreement hash from the canonical response bytes.
	hash := payment.ComputeAgreementHash(respBytes)

	// Step 4: create + deposit escrow.
	timeoutSec := uint64(time.Now().Add(cfg.AgreementTimeout).Unix())
	escrowRef, err := cfg.Escrow.CreateEscrow(ctx,
		cfg.BuyerEVM, cfg.SellerEVM, nil,
		cfg.Currency, 1, hash, timeoutSec)
	if err != nil {
		return nil, fmt.Errorf("buyer: createEscrow: %w", err)
	}
	if _, err := cfg.Escrow.Deposit(ctx, escrowRef, cfg.Price); err != nil {
		return nil, fmt.Errorf("buyer: deposit: %w", err)
	}
	logger.Printf("[commerce:buyer] escrow funded ref=%s amount=%s",
		escrowRef.Locator, cfg.Price)

	// Step 5: publish EscrowCreated on seller.stdIn.
	created := payment.EscrowCreated{
		Type:            payment.PayloadEscrowCreated,
		Version:         "1.0.0",
		RequestID:       cfg.RequestID,
		EscrowRef:       escrowRef.Locator,
		DepositAmount:   cfg.Price,
		DepositCurrency: cfg.Currency,
	}
	if err := publishCommerce(cfg.Bus, cfg.Key, cfg.SellerStdIn, created); err != nil {
		return nil, fmt.Errorf("buyer: publish EscrowCreated: %w", err)
	}
	if _, err := state.Transition(payment.EventEscrowCreated); err != nil {
		return nil, fmt.Errorf("buyer: transition to FUNDED: %w", err)
	}
	logger.Printf("[commerce:buyer] EscrowCreated published; agreement state=%s", state.State())

	return &BuyerSession{
		cfg:       cfg,
		state:     state,
		hash:      hash,
		escrow:    cfg.Escrow,
		escrowRef: escrowRef,
		startedAt: time.Now(),
	}, nil
}

// Settle drives the buyer side of post-delivery settlement: wait for Invoice on
// BuyerStdIn → publish InvoiceAck approved → call escrow RequestRelease +
// ApproveRelease. Returns the final agreement state (StateActive on success
// per the agreement state-machine; a follow-up EventComplete moves it to
// StateCompleted, but Settle stops at the on-chain release).
func (s *BuyerSession) Settle(ctx context.Context) error {
	if s == nil {
		return errors.New("buyer-session: nil")
	}
	logger := s.cfg.Logger
	if logger == nil {
		logger = nopLogger{}
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	deliveries, err := s.cfg.Bus.Subscribe(subCtx, s.cfg.BuyerStdIn, topic.SubscribeOpts{})
	if err != nil {
		return fmt.Errorf("buyer: subscribe stdIn for Invoice: %w", err)
	}

	inv, _, err := awaitTypedPayload[payment.Invoice](
		ctx, deliveries, payment.PayloadInvoice, s.cfg.RequestID,
		s.cfg.AgreementTimeout, "Invoice")
	if err != nil {
		return err
	}

	// Promote FUNDED → ACTIVE (delivery occurred between negotiate-and-fund
	// and Settle). Skipped if already past FUNDED (e.g. caller managed the
	// transition directly).
	if s.state.State() == payment.StateFunded {
		if _, err := s.state.Transition(payment.EventDeliveryStarted); err != nil {
			return fmt.Errorf("buyer: transition FUNDED→ACTIVE: %w", err)
		}
	}
	if _, err := s.state.Transition(payment.EventInvoice); err != nil {
		return fmt.Errorf("buyer: transition to INVOICED: %w", err)
	}
	logger.Printf("[commerce:buyer] Invoice received amount=%s currency=%s ref=%s",
		inv.Amount, inv.Currency, inv.ReleaseRequestRef)

	// On-chain ApproveRelease only — the seller pre-created the release
	// request with msg.sender=seller. NeuronEscrow.requestRelease requires
	// the seller (or arbiter) to be the caller; the buyer side of the
	// pull-pattern is approveRelease + (implicit) withdraw, which is what
	// EVMEscrowAdapter wraps into the single ApproveRelease call below.
	//
	// Mock mode: MemoryEscrow accepts either side as caller, but we still
	// mirror the production flow (seller pre-creates, buyer approves)
	// so the test path matches the on-chain contract semantics.
	releaseRef := payment.ReleaseRequestRef{
		Binding: s.escrowRef.Binding,
		Locator: inv.ReleaseRequestRef,
	}
	releaseResult, err := s.escrow.ApproveRelease(ctx, s.escrowRef, releaseRef)
	if err != nil {
		return fmt.Errorf("buyer: ApproveRelease: %w", err)
	}
	logger.Printf("[commerce:buyer] release approved tx=%s released=%s",
		releaseResult.TransactionRef, releaseResult.Released)

	// Publish InvoiceAck approved.
	ack := payment.InvoiceAck{
		Type:              payment.PayloadInvoiceAck,
		Version:           "1.0.0",
		RequestID:         s.cfg.RequestID,
		ReleaseRequestRef: inv.ReleaseRequestRef,
		Action:            "approved",
	}
	if err := publishCommerce(s.cfg.Bus, s.cfg.Key, s.cfg.SellerStdIn, ack); err != nil {
		return fmt.Errorf("buyer: publish InvoiceAck: %w", err)
	}
	if _, err := s.state.Transition(payment.EventInvoiceApproved); err != nil {
		return fmt.Errorf("buyer: transition INVOICED→ACTIVE: %w", err)
	}
	logger.Printf("[commerce:buyer] InvoiceAck approved; state=%s", s.state.State())
	return nil
}

// State returns the agreement's current state machine state.
func (s *BuyerSession) State() payment.AgreementState { return s.state.State() }

// EscrowRef returns the escrow ref the buyer funded.
func (s *BuyerSession) EscrowRef() payment.EscrowRef { return s.escrowRef }

// AgreementHash returns the keccak256 hash of the accepted ServiceResponse.
func (s *BuyerSession) AgreementHash() [32]byte { return s.hash }

// RequestID returns the per-session unique requestID generated for this
// agreement. Iter-7 P1.1: each call to BuyerNegotiateAndFund uses a
// fresh requestID; the worker that triggered the call needs the same
// value when building the matching ReverseConnectionSetup so the
// seller's idempotency guard recognizes the connectionSetup as part of
// the same agreement.
func (s *BuyerSession) RequestID() string { return s.cfg.RequestID }

// SellerObserveAndAccept runs the seller side of the same flow.
//
// Steps:
//
//  1. Subscribe to seller.stdIn. Wait for ServiceRequest with the configured
//     RequestID (or the first one observed if RequestID is empty in cfg).
//  2. Publish ServiceResponse{action=accept} on buyer.stdIn.
//  3. Compute agreementHash.
//  4. Wait for EscrowCreated on stdIn.
//  5. Return SellerSession (state=FUNDED).
func SellerObserveAndAccept(ctx context.Context, cfg SellerSessionConfig, ttl time.Duration) (*SellerSession, error) {
	if err := validateSellerCfg(&cfg); err != nil {
		return nil, err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = nopLogger{}
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	deliveries, err := cfg.Bus.Subscribe(subCtx, cfg.SellerStdIn, topic.SubscribeOpts{})
	if err != nil {
		return nil, fmt.Errorf("seller: subscribe stdIn: %w", err)
	}

	req, _, err := awaitTypedPayload[payment.ServiceRequest](
		ctx, deliveries, payment.PayloadServiceRequest, "",
		ttl, "ServiceRequest")
	if err != nil {
		return nil, err
	}

	state := payment.NewAgreementStateMachine(req.RequestID)
	if _, err := state.Transition(payment.EventServiceRequest); err != nil {
		return nil, fmt.Errorf("seller: transition to REQUESTED: %w", err)
	}
	logger.Printf("[commerce:seller] ServiceRequest received requestID=%s", req.RequestID)

	// Build accept response.
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

	// Wait for EscrowCreated on stdIn (we re-use the same deliveries channel).
	created, _, err := awaitTypedPayload[payment.EscrowCreated](
		ctx, deliveries, payment.PayloadEscrowCreated, req.RequestID,
		ttl, "EscrowCreated")
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

// IssueInvoice publishes Invoice on buyer.stdIn and waits for InvoiceAck.
// Returns nil when InvoiceAck.Action="approved" arrives, an error otherwise.
//
// When a seller-side EscrowAdapter is configured, IssueInvoice calls
// RequestRelease on-chain BEFORE publishing Invoice, and embeds the resulting
// release ref in the Invoice payload. NeuronEscrow's `requestRelease` requires
// msg.sender to be the seller, so this is the only correct sequencing under
// the EVM escrow contract. The fallback `releaseRequestRef` arg (legacy from
// iter-3 mock mode) is used when no EscrowAdapter is configured.
func (s *SellerSession) IssueInvoice(ctx context.Context, releaseRequestRef string, ttl time.Duration) error {
	if s == nil {
		return errors.New("seller-session: nil")
	}
	logger := s.cfg.Logger
	if logger == nil {
		logger = nopLogger{}
	}

	// Transition ACTIVE first (DELIVERY_STARTED was implicit).
	if s.state.State() == payment.StateFunded {
		if _, err := s.state.Transition(payment.EventDeliveryStarted); err != nil {
			return fmt.Errorf("seller: transition to ACTIVE: %w", err)
		}
	}

	// On-chain release request (when configured). Recipient = seller's
	// own EVM address; evidenceHash binds this release to the agreement
	// (iter-4 uses the agreementHash; iter-5+ will substitute a delivery
	// proof — frame count + bytes hash).
	if s.cfg.Escrow != nil && s.cfg.SellerEVM != "" {
		binding := s.escrowBinding
		if binding == "" {
			binding = "evm-escrow"
		}
		escrowRef := payment.EscrowRef{Binding: binding, Locator: s.escrowRef}
		releaseRefObj, err := s.cfg.Escrow.RequestRelease(ctx, escrowRef,
			s.amount, s.cfg.SellerEVM, s.hash)
		if err != nil {
			return fmt.Errorf("seller: RequestRelease (on-chain): %w", err)
		}
		releaseRequestRef = releaseRefObj.Locator
		logger.Printf("[commerce:seller] on-chain RequestRelease succeeded ref=%s", releaseRequestRef)
	}

	inv := payment.Invoice{
		Type:              payment.PayloadInvoice,
		Version:           "1.0.0",
		RequestID:         s.requestID,
		ReleaseRequestRef: releaseRequestRef,
		EscrowRef:         s.escrowRef,
		Amount:            s.amount,
		Currency:          s.currency,
		Period:            "PT1H",
	}
	if err := publishCommerce(s.cfg.Bus, s.cfg.Key, s.cfg.BuyerStdIn, inv); err != nil {
		return fmt.Errorf("seller: publish Invoice: %w", err)
	}
	if _, err := s.state.Transition(payment.EventInvoice); err != nil {
		return fmt.Errorf("seller: transition to INVOICED: %w", err)
	}
	logger.Printf("[commerce:seller] Invoice published amount=%s ref=%s",
		s.amount, releaseRequestRef)

	// Wait for InvoiceAck on stdIn.
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	deliveries, err := s.cfg.Bus.Subscribe(subCtx, s.cfg.SellerStdIn, topic.SubscribeOpts{})
	if err != nil {
		return fmt.Errorf("seller: subscribe stdIn for InvoiceAck: %w", err)
	}
	ack, _, err := awaitTypedPayload[payment.InvoiceAck](
		ctx, deliveries, payment.PayloadInvoiceAck, s.requestID, ttl, "InvoiceAck")
	if err != nil {
		return err
	}
	if ack.Action != "approved" {
		if _, terr := s.state.Transition(payment.EventInvoiceRefused); terr != nil {
			logger.Printf("[commerce:seller] state-machine transition error after refusal: %v", terr)
		}
		return fmt.Errorf("seller: InvoiceAck refused (action=%q)", ack.Action)
	}
	if _, err := s.state.Transition(payment.EventInvoiceApproved); err != nil {
		return fmt.Errorf("seller: transition INVOICED→ACTIVE: %w", err)
	}
	logger.Printf("[commerce:seller] InvoiceAck approved; state=%s", s.state.State())
	return nil
}

// State returns the seller-side agreement state.
func (s *SellerSession) State() payment.AgreementState { return s.state.State() }

// AgreementHash returns the keccak256 hash of the accepted ServiceResponse.
func (s *SellerSession) AgreementHash() [32]byte { return s.hash }

// publishCommerce signs payload as a TopicMessage and publishes it FireAndForget.
func publishCommerce(bus topic.TopicAdapter, key *keylib.NeuronPrivateKey, target topic.TopicRef, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	now := uint64(time.Now().UnixNano())
	msg, err := topic.NewTopicMessage(key, now, now, data)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	if _, err := bus.Publish(target, msg, topic.PublishOpts{ConfirmationMode: topic.FireAndForget}); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}

// awaitTypedPayload blocks until a delivery whose payload's "type" field
// matches expectedType (and, if reqID non-empty, whose requestId matches)
// arrives on the channel. Returns the parsed payload + the canonical bytes
// of the message (for downstream hashing).
//
// Returns context.DeadlineExceeded when ttl elapses without a match.
func awaitTypedPayload[T any](
	ctx context.Context,
	deliveries <-chan topic.MessageDelivery,
	expectedType string,
	reqID string,
	ttl time.Duration,
	what string,
) (T, []byte, error) {
	var zero T
	deadline := time.NewTimer(ttl)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			return zero, nil, ctx.Err()
		case <-deadline.C:
			return zero, nil, fmt.Errorf("await %s: timeout after %s", what, ttl)
		case d, ok := <-deliveries:
			if !ok {
				return zero, nil, fmt.Errorf("await %s: subscription closed", what)
			}
			if err := topic.ValidateTopicMessage(d.Message); err != nil {
				continue
			}
			var probe struct {
				Type      string `json:"type"`
				RequestID string `json:"requestId"`
			}
			payloadBytes := d.Message.Payload()
			if err := json.Unmarshal(payloadBytes, &probe); err != nil {
				continue
			}
			if probe.Type != expectedType {
				continue
			}
			if reqID != "" && probe.RequestID != reqID {
				continue
			}
			var parsed T
			if err := json.Unmarshal(payloadBytes, &parsed); err != nil {
				return zero, nil, fmt.Errorf("await %s: parse: %w", what, err)
			}
			return parsed, payloadBytes, nil
		}
	}
}

func validateBuyerCfg(c *BuyerSessionConfig) error {
	switch {
	case c == nil:
		return errors.New("buyer-cfg: nil")
	case c.Bus == nil:
		return errors.New("buyer-cfg: Bus required")
	case c.Key == nil:
		return errors.New("buyer-cfg: Key required")
	case c.BuyerStdIn.Locator() == "":
		return errors.New("buyer-cfg: BuyerStdIn required")
	case c.SellerStdIn.Locator() == "":
		return errors.New("buyer-cfg: SellerStdIn required")
	case c.RequestID == "":
		return errors.New("buyer-cfg: RequestID required")
	case c.BuyerEVM == "":
		return errors.New("buyer-cfg: BuyerEVM required")
	case c.SellerEVM == "":
		return errors.New("buyer-cfg: SellerEVM required")
	case c.Currency == "":
		return errors.New("buyer-cfg: Currency required")
	case c.Price == "":
		return errors.New("buyer-cfg: Price required")
	case c.Escrow == nil:
		return errors.New("buyer-cfg: Escrow required")
	}
	if c.NegotiationTTL == 0 {
		c.NegotiationTTL = 30 * time.Second
	}
	if c.AgreementTimeout == 0 {
		c.AgreementTimeout = 24 * time.Hour
	}
	return nil
}

func validateSellerCfg(c *SellerSessionConfig) error {
	switch {
	case c == nil:
		return errors.New("seller-cfg: nil")
	case c.Bus == nil:
		return errors.New("seller-cfg: Bus required")
	case c.Key == nil:
		return errors.New("seller-cfg: Key required")
	case c.SellerStdIn.Locator() == "":
		return errors.New("seller-cfg: SellerStdIn required")
	case c.BuyerStdIn.Locator() == "":
		return errors.New("seller-cfg: BuyerStdIn required")
	}
	return nil
}
