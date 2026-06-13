package adsb

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

// SellerSessionOptions configures the seller's per-request commerce session.
// Mirrors internal/dapp/remoteid.SellerSessionOptions.
type SellerSessionOptions struct {
	Key           *keylib.NeuronPrivateKey
	Adapter       topic.TopicAdapter
	SellerStdIn   topic.TopicRef
	Descriptor    ServiceDescriptor
	Host          host.Host
	Escrow        payment.EscrowAdapter
	EscrowBinding string
	Mode          string
	Logger        *log.Logger
	FrameSummary  func() (uint64, uint64, uint64)
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

// RunSellerSession runs the seller's commerce flow for ONE ServiceRequest.
// Mirrors internal/dapp/remoteid.RunSellerSession with the ADS-B helpers
// (BuildServiceResponse, PublishConnectionSetup, BuildInvoice with ADS-B
// EvidenceHash prefix).
func RunSellerSession(ctx context.Context, opts SellerSessionOptions) (SellerSessionResult, error) {
	if opts.Key == nil {
		return SellerSessionResult{}, errors.New("adsb.RunSellerSession: Key required")
	}
	if opts.Adapter == nil {
		return SellerSessionResult{}, errors.New("adsb.RunSellerSession: Adapter required")
	}
	if opts.Host == nil {
		return SellerSessionResult{}, errors.New("adsb.RunSellerSession: Host required")
	}
	mode := opts.Mode
	if mode == "" {
		mode = CommerceModeFull
	}
	if mode == CommerceModeFull && opts.Escrow == nil {
		return SellerSessionResult{}, errors.New("adsb.RunSellerSession: Escrow required when Mode=full")
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(discardWriter{}, "", 0)
	}

	logger.Printf("[seller-stage] awaiting serviceRequest on %s", opts.SellerStdIn.Locator())
	req, reqMsg, err := ReceiveServiceRequest(ctx, opts.Adapter, opts.SellerStdIn)
	if err != nil {
		return SellerSessionResult{}, fmt.Errorf("adsb.RunSellerSession: receive serviceRequest: %w", err)
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
		return SellerSessionResult{}, fmt.Errorf("adsb.RunSellerSession: lifecycle receive: %w", err)
	}

	result := SellerSessionResult{
		RequestID: req.RequestID,
		BuyerEVM:  reqMsg.SenderAddress(),
	}

	buyerPub, err := ExtractSenderECDSAPublicKey(reqMsg)
	if err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: extract buyer pubkey: %w", err)
	}

	buyerStdIn, err := topic.NewTopicRef(opts.SellerStdIn.Transport(), req.BuyerStdIn)
	if err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: parse BuyerStdIn: %w", err)
	}
	result.BuyerStdIn = buyerStdIn

	var seq uint64 = 1
	resp := BuildServiceResponse(req, "accept")
	if _, err := PublishPayload(ctx, opts.Adapter, buyerStdIn, opts.Key, seq, resp); err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: publish serviceResponse: %w", err)
	}
	if err := lc.Accept(); err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: lifecycle accept: %w", err)
	}
	seq++

	escrowBinding := opts.EscrowBinding
	if escrowBinding == "" {
		escrowBinding = "memory"
	}
	var sellerEscrowRef payment.EscrowRef
	if mode == CommerceModeFull {
		logger.Printf("[seller-stage] awaiting escrowCreated")
		created, _, err := ReceiveEscrowCreated(ctx, opts.Adapter, opts.SellerStdIn, req.RequestID)
		if err != nil {
			return result, fmt.Errorf("adsb.RunSellerSession: receive escrowCreated: %w", err)
		}
		result.EscrowRef = created.EscrowRef
		sellerEscrowRef = payment.EscrowRef{
			Binding: escrowBinding,
			Locator: created.EscrowRef,
		}
		if err := lc.Funded(); err != nil {
			return result, fmt.Errorf("adsb.RunSellerSession: lifecycle funded: %w", err)
		}
	}

	_, err = PublishConnectionSetup(ctx, opts.Adapter, buyerStdIn, opts.Key, seq,
		req.RequestID, opts.Host, buyerPub, opts.Descriptor.Streams)
	if err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: publish connectionSetup: %w", err)
	}
	seq++
	if mode == CommerceModeFull {
		if err := lc.StartDelivery(); err != nil {
			return result, fmt.Errorf("adsb.RunSellerSession: lifecycle startDelivery: %w", err)
		}
	}

	if mode == CommerceModeRegistrationOnly {
		result.FinalState = lc.State()
		return result, nil
	}

	logger.Printf("[seller-stage] awaiting serviceStop")
	_, _, err = ReceiveServiceStop(ctx, opts.Adapter, opts.SellerStdIn, req.RequestID)
	if err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: receive serviceStop: %w", err)
	}
	logger.Printf("[seller-stage] received serviceStop; requesting escrow release")

	var frameCount, firstAt, lastAt uint64
	if opts.FrameSummary != nil {
		frameCount, firstAt, lastAt = opts.FrameSummary()
	}
	result.EvidenceHash = EvidenceHashFor(opts.Host.ID().String(), frameCount, firstAt, lastAt)
	evidenceHashBytes := evidenceHashTo32Bytes(result.EvidenceHash)

	releaseRef, err := opts.Escrow.RequestRelease(ctx, sellerEscrowRef,
		req.ProposedAmount, reqMsg.SenderAddress(), evidenceHashBytes)
	if err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: escrow.RequestRelease: %w", err)
	}
	logger.Printf("[seller-stage] escrow release requested ref=%s amount=%s", releaseRef.Locator, req.ProposedAmount)

	invoice, err := BuildInvoice(InvoiceOptions{
		Request:           req,
		EscrowRef:         result.EscrowRef,
		ReleaseRequestRef: releaseRef.Locator,
		EvidenceHash:      result.EvidenceHash,
	})
	if err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: build invoice: %w", err)
	}
	if _, err := PublishInvoice(ctx, opts.Adapter, buyerStdIn, opts.Key, seq, invoice); err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: publish invoice: %w", err)
	}
	logger.Printf("[seller-stage] invoice published on buyerStdIn=%s", buyerStdIn.Locator())
	seq++
	if err := lc.BeginInvoice(); err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: lifecycle beginInvoice: %w", err)
	}

	logger.Printf("[seller-stage] awaiting invoiceAck")
	ack, _, err := ReceiveInvoiceAck(ctx, opts.Adapter, opts.SellerStdIn, req.RequestID)
	if err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: receive invoiceAck: %w", err)
	}
	if ack.Action != "approved" {
		if err := lc.RefuseInvoice(); err != nil {
			return result, fmt.Errorf("adsb.RunSellerSession: lifecycle refuseInvoice: %w", err)
		}
		result.FinalState = lc.State()
		return result, fmt.Errorf("adsb.RunSellerSession: buyer refused invoice")
	}

	if rel, err := opts.Escrow.ApproveRelease(ctx, sellerEscrowRef, releaseRef); err != nil {
		logger.Printf("[settle] seller-side ApproveRelease note: %v", err)
	} else {
		logger.Printf("[settle] seller-side ApproveRelease tx=%s released=%s recipient=%s", rel.TransactionRef, rel.Released, rel.Recipient)
	}

	if err := lc.ApproveInvoice(); err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: lifecycle approveInvoice: %w", err)
	}

	if err := lc.Complete(); err != nil {
		return result, fmt.Errorf("adsb.RunSellerSession: lifecycle complete: %w", err)
	}
	result.FinalState = lc.State()
	return result, nil
}

func evidenceHashTo32Bytes(hexHash string) [32]byte {
	var out [32]byte
	if decoded, err := hex.DecodeString(hexHash); err == nil && len(decoded) >= 32 {
		copy(out[:], decoded[:32])
		return out
	}
	sum := sha256.Sum256([]byte(hexHash))
	out = sum
	return out
}

// BuyerSessionOptions configures the buyer's per-request commerce flow.
type BuyerSessionOptions struct {
	Key                  *keylib.NeuronPrivateKey
	EcdsaPriv            *ecdsa.PrivateKey
	Adapter              topic.TopicAdapter
	SellerStdIn          topic.TopicRef
	BuyerStdIn           topic.TopicRef
	RequestID            string
	ExpectedSellerPeerID string
	Mode                 string
	Escrow               payment.EscrowAdapter
	EscrowBinding        string
	ServiceRequest       *payment.ServiceRequest
	SellerEVM            string
	Logger               *log.Logger
}

// BuyerSessionResult captures the buyer-side trace for evidence.
type BuyerSessionResult struct {
	RequestID   string
	EscrowRef   string
	Discovery   *payment.ConnectionSetup
	SellerAddr  peer.AddrInfo
	InvoiceHash string
	FinalAction string
	FinalState  payment.AgreementState
}

// RunBuyerSession runs the buyer's commerce flow up to ConnectionSetup receipt.
func RunBuyerSession(ctx context.Context, opts BuyerSessionOptions) (BuyerSessionResult, error) {
	if opts.Key == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunBuyerSession: Key required")
	}
	if opts.EcdsaPriv == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunBuyerSession: EcdsaPriv required")
	}
	if opts.Adapter == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunBuyerSession: Adapter required")
	}
	if opts.RequestID == "" {
		return BuyerSessionResult{}, errors.New("adsb.RunBuyerSession: RequestID required")
	}
	if opts.ExpectedSellerPeerID == "" {
		return BuyerSessionResult{}, errors.New("adsb.RunBuyerSession: ExpectedSellerPeerID required (registry-derived identity binding)")
	}
	mode := opts.Mode
	if mode == "" {
		mode = CommerceModeFull
	}
	if mode == CommerceModeFull && opts.Escrow == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunBuyerSession: Escrow required when Mode=full")
	}
	if mode == CommerceModeFull && opts.SellerEVM == "" {
		return BuyerSessionResult{}, errors.New("adsb.RunBuyerSession: SellerEVM required when Mode=full")
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
			return BuyerSessionResult{}, fmt.Errorf("adsb.RunBuyerSession: build serviceRequest: %w", err)
		}
		req = &built
	}

	result := BuyerSessionResult{RequestID: req.RequestID}

	if _, err := PublishPayload(ctx, opts.Adapter, opts.SellerStdIn, opts.Key, seq, *req); err != nil {
		return result, fmt.Errorf("adsb.RunBuyerSession: publish serviceRequest: %w", err)
	}
	seq++

	resp, _, err := ReceiveServiceResponse(ctx, opts.Adapter, opts.BuyerStdIn, req.RequestID)
	if err != nil {
		return result, fmt.Errorf("adsb.RunBuyerSession: receive serviceResponse: %w", err)
	}
	if resp.Action != "accept" {
		return result, fmt.Errorf("adsb.RunBuyerSession: seller %s", resp.Action)
	}

	if mode == CommerceModeFull {
		agreementHash := sha256.Sum256(
			[]byte(req.RequestID + "|" + opts.Key.PublicKey().EVMAddress().Hex() + "|" + opts.SellerEVM),
		)
		buyerEVM := opts.Key.PublicKey().EVMAddress().Hex()
		escrowRef, err := opts.Escrow.CreateEscrow(ctx, buyerEVM, opts.SellerEVM, nil,
			req.ProposedCurrency, uint64(1), agreementHash, uint64(time.Now().Unix())+3600)
		if err != nil {
			return result, fmt.Errorf("adsb.RunBuyerSession: escrow.CreateEscrow: %w", err)
		}
		result.EscrowRef = escrowRef.Locator

		depositRes, err := opts.Escrow.Deposit(ctx, escrowRef, req.ProposedAmount)
		if err != nil {
			return result, fmt.Errorf("adsb.RunBuyerSession: escrow.Deposit: %w", err)
		}
		logger.Printf("[escrow] created escrowRef=%s deposited=%s depositTx=%s", escrowRef.Locator, req.ProposedAmount, depositRes.TransactionRef)

		created := BuildEscrowCreated(*req, escrowRef.Locator, req.ProposedAmount, req.ProposedCurrency)
		if _, err := PublishEscrowCreated(ctx, opts.Adapter, opts.SellerStdIn, opts.Key, seq, created); err != nil {
			return result, fmt.Errorf("adsb.RunBuyerSession: publish escrowCreated: %w", err)
		}
		seq++
	}

	setup, addrInfo, err := ReceiveConnectionSetup(ctx, opts.Adapter, opts.BuyerStdIn,
		req.RequestID, opts.ExpectedSellerPeerID, opts.EcdsaPriv)
	if err != nil {
		return result, fmt.Errorf("adsb.RunBuyerSession: receive connectionSetup: %w", err)
	}
	result.Discovery = setup
	result.SellerAddr = addrInfo
	return result, nil
}

// FinaliseBuyerSession publishes ServiceStop, waits for Invoice, approves
// release, and publishes InvoiceAck.
func FinaliseBuyerSession(ctx context.Context, opts BuyerSessionOptions, partial BuyerSessionResult, seq uint64) (BuyerSessionResult, error) {
	if opts.Mode == CommerceModeRegistrationOnly {
		partial.FinalAction = "registration-only-skip"
		return partial, nil
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(discardWriter{}, "", 0)
	}

	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	fromZero := uint64(0)
	invoiceCh, err := opts.Adapter.Subscribe(subCtx, opts.BuyerStdIn, topic.SubscribeOpts{FromSequence: &fromZero})
	if err != nil {
		return partial, fmt.Errorf("adsb.FinaliseBuyerSession: subscribe buyerStdIn: %w", err)
	}

	stop := BuildServiceStop(payment.ServiceRequest{RequestID: opts.RequestID}, "buyer-session-end", 0)
	if _, err := PublishServiceStop(ctx, opts.Adapter, opts.SellerStdIn, opts.Key, seq, stop); err != nil {
		return partial, fmt.Errorf("adsb.FinaliseBuyerSession: publish serviceStop: %w", err)
	}
	seq++
	logger.Printf("[settle] published serviceStop requestID=%s", opts.RequestID)

	var inv payment.Invoice
	for {
		select {
		case <-ctx.Done():
			return partial, fmt.Errorf("adsb.FinaliseBuyerSession: %w", ctx.Err())
		case delivery, ok := <-invoiceCh:
			if !ok {
				return partial, errors.New("adsb.FinaliseBuyerSession: buyerStdIn subscription closed before invoice arrived")
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
				return partial, fmt.Errorf("adsb.FinaliseBuyerSession: decode invoice: %w", perr)
			}
			goto invoiceReceived
		}
	}
invoiceReceived:
	partial.InvoiceHash = inv.ReleaseRequestRef
	logger.Printf("[settle] received invoice evidenceHash=%s releaseRef=%s", inv.ReleaseRequestRef, inv.ReleaseRequestRef)

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
		return partial, fmt.Errorf("adsb.FinaliseBuyerSession: escrow.ApproveRelease: %w", err)
	}
	logger.Printf("[settle] buyer-side ApproveRelease tx=%s released=%s recipient=%s", rel.TransactionRef, rel.Released, rel.Recipient)

	ack := BuildInvoiceAck(inv, "approved")
	if _, err := PublishInvoiceAck(ctx, opts.Adapter, opts.SellerStdIn, opts.Key, seq, ack); err != nil {
		return partial, fmt.Errorf("adsb.FinaliseBuyerSession: publish invoiceAck: %w", err)
	}
	logger.Printf("[settle] published invoiceAck action=approved requestID=%s", opts.RequestID)
	partial.FinalAction = "approved"
	return partial, nil
}

func jsonUnmarshalProbe(body []byte, target any) error {
	return json.Unmarshal(body, target)
}
