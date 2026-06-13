package remoteid

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// SellerSessionOptions configures the seller's per-request commerce
// session. ServiceRequest arrives on SellerStdIn; the seller sends every
// reply on the buyer's stdIn (extracted from the ServiceRequest).
type SellerSessionOptions struct {
	// Key signs every outbound TopicMessage.
	Key *keylib.NeuronPrivateKey

	// Adapter is the topic bus shared with the buyer.
	Adapter topic.TopicAdapter

	// SellerStdIn is the topic the buyer publishes ServiceRequest / EscrowCreated /
	// ServiceStop / InvoiceAck on.
	SellerStdIn topic.TopicRef

	// Descriptor carries the seller's Streams catalog + commerce-mode posture.
	Descriptor ServiceDescriptor

	// Host is the seller's libp2p host. Used for the seller's PeerID +
	// multiaddrs (ECIES-encrypted into ConnectionSetup).
	Host host.Host

	// Escrow lets the seller request and (after InvoiceAck) approve
	// release of the buyer's deposit. Required when Mode=full.
	Escrow payment.EscrowAdapter

	// EscrowBinding is the binding string the seller stamps on every
	// EscrowRef it passes to the escrow adapter. Defaults to "memory"
	// (matches MemoryEscrow). Production CLIs that wire EVMEscrowAdapter
	// pass "evm-escrow" (or whatever binding the adapter expects).
	// Distinct from the commerce-service descriptor's wire binding
	// because the adapter-level binding is implementation-specific.
	EscrowBinding string

	// Mode is the commerce posture: CommerceModeFull (default) or
	// CommerceModeRegistrationOnly. The descriptor's CommerceMode is
	// the spec-disclosed truth; this field controls runtime behaviour.
	Mode string

	// Logger receives one `[lifecycle] …` and `[settle] …` line per
	// transition for evidence. Optional.
	Logger *log.Logger

	// FrameSummary, when non-nil, is consulted at INVOICED-time to
	// produce the evidenceHash (D7). Returns (frameCount, firstSentAt,
	// lastSentAt). Used by integration tests to inject deterministic
	// values; CLI uses a real counter.
	FrameSummary func() (uint64, uint64, uint64)
}

// SellerSessionResult records the per-request outcome for evidence.
type SellerSessionResult struct {
	RequestID    string
	FinalState   payment.AgreementState
	EscrowRef    string
	EvidenceHash string
	BuyerEVM     string
	BuyerStdIn   topic.TopicRef
}

// RunSellerSession runs the seller's commerce flow for ONE ServiceRequest:
// subscribe to SellerStdIn → drive lifecycle → publish ConnectionSetup →
// (full mode) await ServiceStop → publish Invoice → await InvoiceAck →
// approve release → COMPLETED.
//
// Returns when the lifecycle finishes (success, abort, or context-cancelled).
// Concurrent buyers require running this function once per buyer; the
// Stage 2 CLI runs a single session, then exits.
func RunSellerSession(ctx context.Context, opts SellerSessionOptions) (SellerSessionResult, error) {
	if opts.Key == nil {
		return SellerSessionResult{}, errors.New("remoteid.RunSellerSession: Key required")
	}
	if opts.Adapter == nil {
		return SellerSessionResult{}, errors.New("remoteid.RunSellerSession: Adapter required")
	}
	if opts.Host == nil {
		return SellerSessionResult{}, errors.New("remoteid.RunSellerSession: Host required")
	}
	mode := opts.Mode
	if mode == "" {
		mode = CommerceModeFull
	}
	if mode == CommerceModeFull && opts.Escrow == nil {
		return SellerSessionResult{}, errors.New("remoteid.RunSellerSession: Escrow required when Mode=full")
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(discardWriter{}, "", 0)
	}

	// Step 1: receive ServiceRequest.
	logger.Printf("[seller-stage] awaiting serviceRequest on %s", opts.SellerStdIn.Locator())
	req, reqMsg, err := ReceiveServiceRequest(ctx, opts.Adapter, opts.SellerStdIn)
	if err != nil {
		return SellerSessionResult{}, fmt.Errorf("remoteid.RunSellerSession: receive serviceRequest: %w", err)
	}
	logger.Printf("[seller-stage] received serviceRequest requestID=%s buyerStdIn=%s", req.RequestID, req.BuyerStdIn)
	lc, err := NewLifecycle(LifecycleOptions{
		RequestID: req.RequestID,
		Mode:      mode,
		Logger:    logger,
	})
	if err != nil {
		return SellerSessionResult{}, err
	}
	if err := lc.Receive(req); err != nil {
		return SellerSessionResult{}, fmt.Errorf("remoteid.RunSellerSession: lifecycle receive: %w", err)
	}

	result := SellerSessionResult{
		RequestID: req.RequestID,
		BuyerEVM:  reqMsg.SenderAddress(),
	}

	// Recover the buyer's secp256k1 pubkey from the ServiceRequest TopicMessage
	// signature for ECIES encryption of the ConnectionSetup multiaddrs.
	buyerPub, err := ExtractSenderECDSAPublicKey(reqMsg)
	if err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: extract buyer pubkey: %w", err)
	}

	buyerStdIn, err := topic.NewTopicRef(opts.SellerStdIn.Transport(), req.BuyerStdIn)
	if err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: parse BuyerStdIn: %w", err)
	}
	result.BuyerStdIn = buyerStdIn

	// Step 2: publish ServiceResponse(accept).
	var seq uint64 = 1
	resp := BuildServiceResponse(req, "accept")
	if _, err := PublishPayload(ctx, opts.Adapter, buyerStdIn, opts.Key, seq, resp); err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: publish serviceResponse: %w", err)
	}
	if err := lc.Accept(); err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: lifecycle accept: %w", err)
	}
	seq++

	escrowBinding := opts.EscrowBinding
	if escrowBinding == "" {
		escrowBinding = "memory"
	}
	var sellerEscrowRef payment.EscrowRef
	if mode == CommerceModeFull {
		logger.Printf("[seller-stage] awaiting escrowCreated")
		// Step 3: wait for EscrowCreated.
		created, _, err := ReceiveEscrowCreated(ctx, opts.Adapter, opts.SellerStdIn, req.RequestID)
		if err != nil {
			return result, fmt.Errorf("remoteid.RunSellerSession: receive escrowCreated: %w", err)
		}
		result.EscrowRef = created.EscrowRef
		sellerEscrowRef = payment.EscrowRef{
			Binding: escrowBinding,
			Locator: created.EscrowRef,
		}
		if err := lc.Funded(); err != nil {
			return result, fmt.Errorf("remoteid.RunSellerSession: lifecycle funded: %w", err)
		}
	}

	// Step 4: publish ConnectionSetup with streams[] + ECIES multiaddrs.
	_, err = PublishConnectionSetup(ctx, opts.Adapter, buyerStdIn, opts.Key, seq,
		req.RequestID, opts.Host, buyerPub, opts.Descriptor.Streams)
	if err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: publish connectionSetup: %w", err)
	}
	seq++
	if mode == CommerceModeFull {
		if err := lc.StartDelivery(); err != nil {
			return result, fmt.Errorf("remoteid.RunSellerSession: lifecycle startDelivery: %w", err)
		}
	}

	if mode == CommerceModeRegistrationOnly {
		// Short-circuit: never engage escrow / invoice / completed.
		result.FinalState = lc.State()
		return result, nil
	}

	logger.Printf("[seller-stage] awaiting serviceStop")
	// Step 5: wait for ServiceStop (buyer ends the session).
	_, _, err = ReceiveServiceStop(ctx, opts.Adapter, opts.SellerStdIn, req.RequestID)
	if err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: receive serviceStop: %w", err)
	}
	logger.Printf("[seller-stage] received serviceStop; requesting escrow release")

	// Compute evidenceHash (D7).
	var frameCount, firstAt, lastAt uint64
	if opts.FrameSummary != nil {
		frameCount, firstAt, lastAt = opts.FrameSummary()
	}
	result.EvidenceHash = EvidenceHashFor(opts.Host.ID().String(), frameCount, firstAt, lastAt)
	evidenceHashBytes := evidenceHashTo32Bytes(result.EvidenceHash)

	// Step 6: request escrow release (FR-P25a; amount must be ≤ available
	// balance — for R2 demo amount=0 always satisfies this).
	releaseRef, err := opts.Escrow.RequestRelease(ctx, sellerEscrowRef,
		req.ProposedAmount, reqMsg.SenderAddress(), evidenceHashBytes)
	if err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: escrow.RequestRelease: %w", err)
	}
	logger.Printf("[seller-stage] escrow release requested ref=%s amount=%s", releaseRef.Locator, req.ProposedAmount)

	// Step 7: build + publish Invoice.
	invoice, err := BuildInvoice(InvoiceOptions{
		Request:           req,
		EscrowRef:         result.EscrowRef,
		ReleaseRequestRef: releaseRef.Locator,
		EvidenceHash:      result.EvidenceHash,
	})
	if err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: build invoice: %w", err)
	}
	if _, err := PublishInvoice(ctx, opts.Adapter, buyerStdIn, opts.Key, seq, invoice); err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: publish invoice: %w", err)
	}
	logger.Printf("[seller-stage] invoice published on buyerStdIn=%s", buyerStdIn.Locator())
	seq++
	if err := lc.BeginInvoice(); err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: lifecycle beginInvoice: %w", err)
	}

	logger.Printf("[seller-stage] awaiting invoiceAck")
	// Step 8: wait for InvoiceAck on SellerStdIn.
	ack, _, err := ReceiveInvoiceAck(ctx, opts.Adapter, opts.SellerStdIn, req.RequestID)
	if err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: receive invoiceAck: %w", err)
	}
	if ack.Action != "approved" {
		if err := lc.RefuseInvoice(); err != nil {
			return result, fmt.Errorf("remoteid.RunSellerSession: lifecycle refuseInvoice: %w", err)
		}
		result.FinalState = lc.State()
		return result, fmt.Errorf("remoteid.RunSellerSession: buyer refused invoice")
	}

	// Step 9: approve release on the seller's side too (mock escrow
	// requires both parties' approval semantics; the MemoryEscrow
	// implements the buyer-approves-only flow but we mirror the
	// approveRelease here to make the trace symmetric).
	if rel, err := opts.Escrow.ApproveRelease(ctx, sellerEscrowRef, releaseRef); err != nil {
		// Buyer-already-approved is the common case; not a fatal error.
		logger.Printf("[settle] seller-side ApproveRelease note: %v", err)
	} else {
		logger.Printf("[settle] seller-side ApproveRelease tx=%s released=%s recipient=%s", rel.TransactionRef, rel.Released, rel.Recipient)
	}

	if err := lc.ApproveInvoice(); err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: lifecycle approveInvoice: %w", err)
	}

	if err := lc.Complete(); err != nil {
		return result, fmt.Errorf("remoteid.RunSellerSession: lifecycle complete: %w", err)
	}
	result.FinalState = lc.State()
	return result, nil
}

// evidenceHashTo32Bytes converts the hex-encoded SHA-256 string into the
// [32]byte form payment.EscrowAdapter.RequestRelease expects.
func evidenceHashTo32Bytes(hexHash string) [32]byte {
	var out [32]byte
	if decoded, err := hex.DecodeString(hexHash); err == nil && len(decoded) >= 32 {
		copy(out[:], decoded[:32])
		return out
	}
	// Fallback: re-hash the input so we still produce 32 bytes.
	sum := sha256.Sum256([]byte(hexHash))
	out = sum
	return out
}

// BuyerSessionOptions configures the buyer's per-request commerce flow.
type BuyerSessionOptions struct {
	// Key signs every outbound TopicMessage.
	Key *keylib.NeuronPrivateKey

	// EcdsaPriv is the buyer's secp256k1 private key (ECIES recipient).
	EcdsaPriv *ecdsa.PrivateKey

	// Adapter is the topic bus shared with the seller.
	Adapter topic.TopicAdapter

	// SellerStdIn is the topic the buyer publishes outbound envelopes on
	// (ServiceRequest, EscrowCreated, ServiceStop, InvoiceAck).
	SellerStdIn topic.TopicRef

	// BuyerStdIn is the topic the seller publishes seller→buyer envelopes
	// on (ServiceResponse, ConnectionSetup, Invoice). The buyer creates
	// this ad-hoc and includes its locator in ServiceRequest.BuyerStdIn.
	BuyerStdIn topic.TopicRef

	// RequestID is the per-request identifier shared between buyer +
	// seller. Required.
	RequestID string

	// ExpectedSellerPeerID is the PeerID the buyer read out of the
	// registered AgentURI. The Stage-2 R2 check fires here when the
	// ConnectionSetup's PeerID does not match.
	ExpectedSellerPeerID string

	// Mode is the commerce posture: CommerceModeFull (default) or
	// CommerceModeRegistrationOnly.
	Mode string

	// Escrow lets the buyer fund + later approve release. Required
	// when Mode=full.
	Escrow payment.EscrowAdapter

	// EscrowBinding mirrors SellerSessionOptions.EscrowBinding — the
	// adapter-level binding string the buyer stamps when re-constructing
	// EscrowRef / ReleaseRequestRef during FinaliseBuyerSession.
	// Defaults to "memory".
	EscrowBinding string

	// ServiceRequest overrides the auto-built ServiceRequest. When nil
	// the buyer constructs one from BuildServiceRequest with sensible
	// defaults (pricing=0, etc.).
	ServiceRequest *payment.ServiceRequest

	// SellerEVM is the seller's EVM address (for escrow CreateEscrow).
	// Required when Mode=full.
	SellerEVM string

	// Logger receives one `[settle] …` line per transition.
	Logger *log.Logger
}

// BuyerSessionResult captures the buyer-side trace for evidence.
type BuyerSessionResult struct {
	RequestID   string
	EscrowRef   string
	Discovery   *payment.ConnectionSetup
	SellerAddr  peer.AddrInfo // decrypted multiaddrs + decoded PeerID; ready to dial
	InvoiceHash string
	FinalAction string // "approved" or "refused" — the InvoiceAck action
	FinalState  payment.AgreementState
}

// RunBuyerSession runs the buyer's commerce flow up to the
// ConnectionSetup-receipt point: publish ServiceRequest → await
// ServiceResponse → fund escrow → await ConnectionSetup. After the
// caller has consumed N frames on its own libp2p stream, the caller
// invokes FinaliseBuyerSession to publish ServiceStop, wait for
// Invoice, approve, and publish InvoiceAck.
//
// Split into two halves because the data-plane (libp2p stream
// reads) lives outside this orchestrator — it stays in the CLI / test.
func RunBuyerSession(ctx context.Context, opts BuyerSessionOptions) (BuyerSessionResult, error) {
	if opts.Key == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunBuyerSession: Key required")
	}
	if opts.EcdsaPriv == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunBuyerSession: EcdsaPriv required")
	}
	if opts.Adapter == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunBuyerSession: Adapter required")
	}
	if opts.RequestID == "" {
		return BuyerSessionResult{}, errors.New("remoteid.RunBuyerSession: RequestID required")
	}
	if opts.ExpectedSellerPeerID == "" {
		return BuyerSessionResult{}, errors.New("remoteid.RunBuyerSession: ExpectedSellerPeerID required (registry-derived identity binding)")
	}
	mode := opts.Mode
	if mode == "" {
		mode = CommerceModeFull
	}
	if mode == CommerceModeFull && opts.Escrow == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunBuyerSession: Escrow required when Mode=full")
	}
	if mode == CommerceModeFull && opts.SellerEVM == "" {
		return BuyerSessionResult{}, errors.New("remoteid.RunBuyerSession: SellerEVM required when Mode=full")
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(discardWriter{}, "", 0)
	}

	var seq uint64 = 1
	req := opts.ServiceRequest
	if req == nil {
		built, err := BuildServiceRequest(ServiceRequestOptions{
			RequestID:  opts.RequestID,
			BuyerStdIn: opts.BuyerStdIn.Locator(),
		})
		if err != nil {
			return BuyerSessionResult{}, fmt.Errorf("remoteid.RunBuyerSession: build serviceRequest: %w", err)
		}
		req = &built
	}

	result := BuyerSessionResult{RequestID: req.RequestID}

	// Step 1: publish ServiceRequest.
	if _, err := PublishPayload(ctx, opts.Adapter, opts.SellerStdIn, opts.Key, seq, *req); err != nil {
		return result, fmt.Errorf("remoteid.RunBuyerSession: publish serviceRequest: %w", err)
	}
	seq++

	// Step 2: wait for ServiceResponse.
	resp, _, err := ReceiveServiceResponse(ctx, opts.Adapter, opts.BuyerStdIn, req.RequestID)
	if err != nil {
		return result, fmt.Errorf("remoteid.RunBuyerSession: receive serviceResponse: %w", err)
	}
	if resp.Action != "accept" {
		return result, fmt.Errorf("remoteid.RunBuyerSession: seller %s", resp.Action)
	}

	// Step 3 (full mode): fund escrow + publish EscrowCreated.
	if mode == CommerceModeFull {
		// agreementHash binds the escrow to the negotiation per FR-P17.
		// For R2 demo we use a SHA-256 of "requestID|buyer|seller" rather
		// than the full canonicalJSON(serviceResponse) the spec would
		// require; the wire shape (32 bytes) is identical.
		agreementHash := sha256.Sum256(
			[]byte(req.RequestID + "|" + opts.Key.PublicKey().EVMAddress().Hex() + "|" + opts.SellerEVM),
		)
		buyerEVM := opts.Key.PublicKey().EVMAddress().Hex()
		escrowRef, err := opts.Escrow.CreateEscrow(ctx, buyerEVM, opts.SellerEVM, nil,
			req.ProposedCurrency, uint64(1), agreementHash, uint64(time.Now().Unix())+3600)
		if err != nil {
			return result, fmt.Errorf("remoteid.RunBuyerSession: escrow.CreateEscrow: %w", err)
		}
		result.EscrowRef = escrowRef.Locator

		// Deposit (amount=0 for free demo).
		depositRes, err := opts.Escrow.Deposit(ctx, escrowRef, req.ProposedAmount)
		if err != nil {
			return result, fmt.Errorf("remoteid.RunBuyerSession: escrow.Deposit: %w", err)
		}
		logger.Printf("[escrow] created escrowRef=%s deposited=%s depositTx=%s", escrowRef.Locator, req.ProposedAmount, depositRes.TransactionRef)

		created := BuildEscrowCreated(*req, escrowRef.Locator, req.ProposedAmount, req.ProposedCurrency)
		if _, err := PublishEscrowCreated(ctx, opts.Adapter, opts.SellerStdIn, opts.Key, seq, created); err != nil {
			return result, fmt.Errorf("remoteid.RunBuyerSession: publish escrowCreated: %w", err)
		}
		seq++
	}

	// Step 4: wait for ConnectionSetup + decrypt.
	setup, addrInfo, err := ReceiveConnectionSetup(ctx, opts.Adapter, opts.BuyerStdIn,
		req.RequestID, opts.ExpectedSellerPeerID, opts.EcdsaPriv)
	if err != nil {
		return result, fmt.Errorf("remoteid.RunBuyerSession: receive connectionSetup: %w", err)
	}
	result.Discovery = setup
	result.SellerAddr = addrInfo
	return result, nil
}

// FinaliseBuyerSession publishes ServiceStop on the seller's stdIn,
// waits for the seller's Invoice, calls escrow.ApproveRelease, and
// publishes InvoiceAck. Called by the caller after it has finished
// consuming the libp2p stream.
//
// Subscribes to opts.BuyerStdIn BEFORE publishing ServiceStop so the
// seller's Invoice cannot race past us (MemoryTopicAdapter does not
// backfill past messages).
func FinaliseBuyerSession(ctx context.Context, opts BuyerSessionOptions, partial BuyerSessionResult, seq uint64) (BuyerSessionResult, error) {
	if opts.Mode == CommerceModeRegistrationOnly {
		// Short-circuit: no Invoice round-trip.
		partial.FinalAction = "registration-only-skip"
		return partial, nil
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(discardWriter{}, "", 0)
	}

	// Step 4.5: open the Invoice subscription FIRST. Cancel-scoped so we
	// don't leak the subscriber goroutine on the adapter. FromSequence=&0
	// matches the rest of the remote-id helpers (backfill-friendly).
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	fromZero := uint64(0)
	invoiceCh, err := opts.Adapter.Subscribe(subCtx, opts.BuyerStdIn, topic.SubscribeOpts{FromSequence: &fromZero})
	if err != nil {
		return partial, fmt.Errorf("remoteid.FinaliseBuyerSession: subscribe buyerStdIn: %w", err)
	}

	// Step 5: publish ServiceStop.
	stop := BuildServiceStop(payment.ServiceRequest{RequestID: opts.RequestID}, "buyer-session-end", 0)
	if _, err := PublishServiceStop(ctx, opts.Adapter, opts.SellerStdIn, opts.Key, seq, stop); err != nil {
		return partial, fmt.Errorf("remoteid.FinaliseBuyerSession: publish serviceStop: %w", err)
	}
	seq++
	logger.Printf("[settle] published serviceStop requestID=%s", opts.RequestID)

	// Step 6: wait for Invoice on the pre-opened subscription.
	var inv payment.Invoice
	for {
		select {
		case <-ctx.Done():
			return partial, fmt.Errorf("remoteid.FinaliseBuyerSession: %w", ctx.Err())
		case delivery, ok := <-invoiceCh:
			if !ok {
				return partial, errors.New("remoteid.FinaliseBuyerSession: buyerStdIn subscription closed before invoice arrived")
			}
			var probe struct {
				Type      string `json:"type"`
				RequestID string `json:"requestId"`
			}
			body := delivery.Message.Payload()
			if perr := jsonUnmarshalProbe(body, &probe); perr != nil {
				continue
			}
			if probe.Type != payment.PayloadInvoice || probe.RequestID != opts.RequestID {
				continue
			}
			if perr := jsonUnmarshalProbe(body, &inv); perr != nil {
				return partial, fmt.Errorf("remoteid.FinaliseBuyerSession: decode invoice: %w", perr)
			}
			goto invoiceReceived
		}
	}
invoiceReceived:
	partial.InvoiceHash = inv.ReleaseRequestRef
	logger.Printf("[settle] received invoice evidenceHash=%s releaseRef=%s", inv.ReleaseRequestRef, inv.ReleaseRequestRef)

	// Step 7: approve via escrow. The binding string mirrors what the
	// buyer's CreateEscrow returned (always "memory" for MemoryEscrow,
	// production CLIs pass "evm-escrow" via opts.EscrowBinding).
	binding := opts.EscrowBinding
	if binding == "" {
		binding = "memory"
	}
	escrowRef := payment.EscrowRef{
		Binding: binding,
		Locator: partial.EscrowRef,
	}
	releaseRef := payment.ReleaseRequestRef{
		Binding: binding,
		Locator: inv.ReleaseRequestRef,
	}
	rel, err := opts.Escrow.ApproveRelease(ctx, escrowRef, releaseRef)
	if err != nil {
		return partial, fmt.Errorf("remoteid.FinaliseBuyerSession: escrow.ApproveRelease: %w", err)
	}
	logger.Printf("[settle] buyer-side ApproveRelease tx=%s released=%s recipient=%s", rel.TransactionRef, rel.Released, rel.Recipient)

	// Step 8: publish InvoiceAck(approved).
	ack := BuildInvoiceAck(inv, "approved")
	if _, err := PublishInvoiceAck(ctx, opts.Adapter, opts.SellerStdIn, opts.Key, seq, ack); err != nil {
		return partial, fmt.Errorf("remoteid.FinaliseBuyerSession: publish invoiceAck: %w", err)
	}
	logger.Printf("[settle] published invoiceAck action=approved requestID=%s", opts.RequestID)
	partial.FinalAction = "approved"
	return partial, nil
}

// jsonUnmarshalProbe wraps json.Unmarshal so the call sites in this file
// can stay terse.
func jsonUnmarshalProbe(body []byte, target any) error {
	return json.Unmarshal(body, target)
}
