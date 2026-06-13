// Package remoteid — settlement helpers for the Stage-2 008-commerce flow.
//
// These helpers wrap the 008 escrow + invoice + lifecycle payloads
// (payment.EscrowCreated, ServiceStop, Invoice, InvoiceAck) and turn them
// into one Build / Publish / Subscribe trio per envelope. The remote-id
// CLIs and the integration tests consume these helpers — they do NOT
// re-implement signing or TopicMessage envelope handling.
//
// EvidenceHash convention (D7 from the Stage 2 plan):
//   The Remote ID seller streams continuous frames; there's no natural
//   "end of evidence" file. The seller's Invoice carries an evidenceHash
//   computed as
//
//     SHA-256("remote-id|" + sellerPeerID + "|" +
//             frameCount + "|" + firstFrameSentAt + "|" + lastFrameSentAt)
//
//   …a frame-count summary, not a content commitment. The wire shape
//   (Invoice.EvidenceHash carries opaque hex) does not change for
//   future content-commitment schemes; only the helper that computes
//   the hash does.
package remoteid

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// EscrowCreated --------------------------------------------------------------

// BuildEscrowCreated constructs the buyer-side EscrowCreated payload that
// confirms escrow has been funded for `req`. EscrowRef is the
// binding-specific opaque handle returned by the escrow adapter.
func BuildEscrowCreated(req payment.ServiceRequest, escrowRef, depositAmount, depositCurrency string) payment.EscrowCreated {
	if depositCurrency == "" {
		depositCurrency = req.ProposedCurrency
	}
	return payment.EscrowCreated{
		Type:            payment.PayloadEscrowCreated,
		Version:         PayloadVersion,
		RequestID:       req.RequestID,
		EscrowRef:       escrowRef,
		DepositAmount:   depositAmount,
		DepositCurrency: depositCurrency,
	}
}

// PublishEscrowCreated signs + publishes the buyer-side EscrowCreated
// envelope to the seller's stdIn topic.
func PublishEscrowCreated(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, signingKey *keylib.NeuronPrivateKey, seq uint64, msg payment.EscrowCreated) (topic.PublishResult, error) {
	return PublishPayload(ctx, adapter, sellerStdIn, signingKey, seq, msg)
}

// ReceiveEscrowCreated blocks until an EscrowCreated with the given
// requestID arrives on sellerStdIn.
func ReceiveEscrowCreated(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, requestID string) (payment.EscrowCreated, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, sellerStdIn, payment.PayloadEscrowCreated, requestID)
	if err != nil {
		return payment.EscrowCreated{}, topic.TopicMessage{}, err
	}
	var out payment.EscrowCreated
	if err := json.Unmarshal(body, &out); err != nil {
		return payment.EscrowCreated{}, topic.TopicMessage{}, fmt.Errorf("remoteid.ReceiveEscrowCreated: decode: %w", err)
	}
	return out, msg, nil
}

// ServiceStop ----------------------------------------------------------------

// BuildServiceStop constructs the buyer→seller ServiceStop signal. Reason
// is optional + informational; effectiveAt = 0 means "immediate" (FR-P36).
func BuildServiceStop(req payment.ServiceRequest, reason string, effectiveAt uint64) payment.ServiceStop {
	return payment.ServiceStop{
		Type:        payment.PayloadServiceStop,
		Version:     PayloadVersion,
		RequestID:   req.RequestID,
		Reason:      reason,
		EffectiveAt: effectiveAt,
	}
}

// PublishServiceStop publishes the buyer's ServiceStop envelope on the
// seller's stdIn topic.
func PublishServiceStop(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, signingKey *keylib.NeuronPrivateKey, seq uint64, msg payment.ServiceStop) (topic.PublishResult, error) {
	return PublishPayload(ctx, adapter, sellerStdIn, signingKey, seq, msg)
}

// ReceiveServiceStop blocks until a ServiceStop with the given requestID
// arrives on sellerStdIn (seller-side consumer).
func ReceiveServiceStop(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, requestID string) (payment.ServiceStop, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, sellerStdIn, payment.PayloadServiceStop, requestID)
	if err != nil {
		return payment.ServiceStop{}, topic.TopicMessage{}, err
	}
	var out payment.ServiceStop
	if err := json.Unmarshal(body, &out); err != nil {
		return payment.ServiceStop{}, topic.TopicMessage{}, fmt.Errorf("remoteid.ReceiveServiceStop: decode: %w", err)
	}
	return out, msg, nil
}

// Invoice --------------------------------------------------------------------

// InvoiceOptions configures BuildInvoice. EvidenceHash and ReleaseRequestRef
// have no defaults — the seller computes them from the escrow adapter's
// release-request return value and the served-frame summary respectively.
type InvoiceOptions struct {
	Request           payment.ServiceRequest
	EscrowRef         string
	ReleaseRequestRef string
	Amount            string // defaults to req.ProposedAmount
	Currency          string // defaults to req.ProposedCurrency
	Period            string // ISO 8601 interval; defaults to "PT0S" (one-shot)
	EvidenceHash      string // hex-encoded SHA-256; see EvidenceHashFor()
}

// BuildInvoice constructs the seller-side Invoice payload. FR-P10.
func BuildInvoice(opts InvoiceOptions) (payment.Invoice, error) {
	if opts.EscrowRef == "" {
		return payment.Invoice{}, errors.New("remoteid.BuildInvoice: EscrowRef required")
	}
	if opts.ReleaseRequestRef == "" {
		return payment.Invoice{}, errors.New("remoteid.BuildInvoice: ReleaseRequestRef required (escrow.RequestRelease return)")
	}
	if opts.Amount == "" {
		opts.Amount = opts.Request.ProposedAmount
	}
	if opts.Currency == "" {
		opts.Currency = opts.Request.ProposedCurrency
	}
	if opts.Period == "" {
		opts.Period = "PT0S"
	}
	return payment.Invoice{
		Type:              payment.PayloadInvoice,
		Version:           PayloadVersion,
		RequestID:         opts.Request.RequestID,
		ReleaseRequestRef: opts.ReleaseRequestRef,
		EscrowRef:         opts.EscrowRef,
		Amount:            opts.Amount,
		Currency:          opts.Currency,
		Period:            opts.Period,
	}, nil
}

// PublishInvoice signs + publishes the seller-side Invoice on the buyer's
// stdIn topic.
func PublishInvoice(ctx context.Context, adapter topic.TopicAdapter, buyerStdIn topic.TopicRef, signingKey *keylib.NeuronPrivateKey, seq uint64, msg payment.Invoice) (topic.PublishResult, error) {
	return PublishPayload(ctx, adapter, buyerStdIn, signingKey, seq, msg)
}

// ReceiveInvoice blocks until an Invoice with the given requestID arrives
// on buyerStdIn.
func ReceiveInvoice(ctx context.Context, adapter topic.TopicAdapter, buyerStdIn topic.TopicRef, requestID string) (payment.Invoice, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, buyerStdIn, payment.PayloadInvoice, requestID)
	if err != nil {
		return payment.Invoice{}, topic.TopicMessage{}, err
	}
	var out payment.Invoice
	if err := json.Unmarshal(body, &out); err != nil {
		return payment.Invoice{}, topic.TopicMessage{}, fmt.Errorf("remoteid.ReceiveInvoice: decode: %w", err)
	}
	return out, msg, nil
}

// InvoiceAck -----------------------------------------------------------------

// BuildInvoiceAck constructs the buyer-side InvoiceAck for the given action
// ("approved" or "refused"). FR-P11.
func BuildInvoiceAck(inv payment.Invoice, action string) payment.InvoiceAck {
	return payment.InvoiceAck{
		Type:              payment.PayloadInvoiceAck,
		Version:           PayloadVersion,
		RequestID:         inv.RequestID,
		ReleaseRequestRef: inv.ReleaseRequestRef,
		Action:            action,
	}
}

// PublishInvoiceAck publishes the buyer-side InvoiceAck on the seller's
// stdIn topic.
func PublishInvoiceAck(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, signingKey *keylib.NeuronPrivateKey, seq uint64, msg payment.InvoiceAck) (topic.PublishResult, error) {
	return PublishPayload(ctx, adapter, sellerStdIn, signingKey, seq, msg)
}

// ReceiveInvoiceAck blocks until an InvoiceAck with the given requestID
// arrives on sellerStdIn.
func ReceiveInvoiceAck(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, requestID string) (payment.InvoiceAck, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, sellerStdIn, payment.PayloadInvoiceAck, requestID)
	if err != nil {
		return payment.InvoiceAck{}, topic.TopicMessage{}, err
	}
	var out payment.InvoiceAck
	if err := json.Unmarshal(body, &out); err != nil {
		return payment.InvoiceAck{}, topic.TopicMessage{}, fmt.Errorf("remoteid.ReceiveInvoiceAck: decode: %w", err)
	}
	return out, msg, nil
}

// Evidence hash --------------------------------------------------------------

// EvidenceHashFor computes the canonical R2 invoice-evidence hash per D7.
// frameCount = 0 + firstFrameSentAt = 0 + lastFrameSentAt = 0 is the
// degenerate "no frames served" case (ServiceStop arrived before any data
// flowed); the seller still invoices honestly with the resulting hash.
func EvidenceHashFor(sellerPeerID string, frameCount uint64, firstFrameSentAt, lastFrameSentAt uint64) string {
	h := sha256.New()
	h.Write([]byte("remote-id|"))
	h.Write([]byte(sellerPeerID))
	h.Write([]byte{'|'})
	h.Write([]byte(strconv.FormatUint(frameCount, 10)))
	h.Write([]byte{'|'})
	h.Write([]byte(strconv.FormatUint(firstFrameSentAt, 10)))
	h.Write([]byte{'|'})
	h.Write([]byte(strconv.FormatUint(lastFrameSentAt, 10)))
	return hex.EncodeToString(h.Sum(nil))
}
