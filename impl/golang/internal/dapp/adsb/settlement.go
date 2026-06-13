// Package adsb — settlement helpers for the Stage-2 008-commerce flow.
//
// These helpers wrap the 008 escrow + invoice + lifecycle payloads
// (payment.EscrowCreated, ServiceStop, Invoice, InvoiceAck) and turn them
// into one Build / Publish / Subscribe trio per envelope. Mirrors
// internal/dapp/remoteid/settlement.go with the EvidenceHash prefix swapped
// to "adsb|".
//
// EvidenceHash convention (D7 from the Stage 2 plan, ADS-B variant):
//
//	SHA-256("adsb|" + sellerPeerID + "|" +
//	        frameCount + "|" + firstFrameSentAt + "|" + lastFrameSentAt)
//
// Per-DApp prefix segregation lets a fused-buyer evidence trail
// disambiguate ADS-B settlement hashes from Remote ID settlement hashes
// even when sellerPeerIDs collide (improbable but defensive).
package adsb

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

// BuildEscrowCreated constructs the buyer-side EscrowCreated payload.
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

// PublishEscrowCreated signs + publishes the buyer-side EscrowCreated envelope.
func PublishEscrowCreated(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, signingKey *keylib.NeuronPrivateKey, seq uint64, msg payment.EscrowCreated) (topic.PublishResult, error) {
	return PublishPayload(ctx, adapter, sellerStdIn, signingKey, seq, msg)
}

// ReceiveEscrowCreated blocks until an EscrowCreated with requestID arrives.
func ReceiveEscrowCreated(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, requestID string) (payment.EscrowCreated, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, sellerStdIn, payment.PayloadEscrowCreated, requestID)
	if err != nil {
		return payment.EscrowCreated{}, topic.TopicMessage{}, err
	}
	var out payment.EscrowCreated
	if err := json.Unmarshal(body, &out); err != nil {
		return payment.EscrowCreated{}, topic.TopicMessage{}, fmt.Errorf("adsb.ReceiveEscrowCreated: decode: %w", err)
	}
	return out, msg, nil
}

// ServiceStop ----------------------------------------------------------------

// BuildServiceStop constructs the buyer→seller ServiceStop signal.
func BuildServiceStop(req payment.ServiceRequest, reason string, effectiveAt uint64) payment.ServiceStop {
	return payment.ServiceStop{
		Type:        payment.PayloadServiceStop,
		Version:     PayloadVersion,
		RequestID:   req.RequestID,
		Reason:      reason,
		EffectiveAt: effectiveAt,
	}
}

// PublishServiceStop publishes the buyer's ServiceStop envelope.
func PublishServiceStop(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, signingKey *keylib.NeuronPrivateKey, seq uint64, msg payment.ServiceStop) (topic.PublishResult, error) {
	return PublishPayload(ctx, adapter, sellerStdIn, signingKey, seq, msg)
}

// ReceiveServiceStop blocks until a ServiceStop with requestID arrives.
func ReceiveServiceStop(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, requestID string) (payment.ServiceStop, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, sellerStdIn, payment.PayloadServiceStop, requestID)
	if err != nil {
		return payment.ServiceStop{}, topic.TopicMessage{}, err
	}
	var out payment.ServiceStop
	if err := json.Unmarshal(body, &out); err != nil {
		return payment.ServiceStop{}, topic.TopicMessage{}, fmt.Errorf("adsb.ReceiveServiceStop: decode: %w", err)
	}
	return out, msg, nil
}

// Invoice --------------------------------------------------------------------

// InvoiceOptions configures BuildInvoice.
type InvoiceOptions struct {
	Request           payment.ServiceRequest
	EscrowRef         string
	ReleaseRequestRef string
	Amount            string
	Currency          string
	Period            string
	EvidenceHash      string
}

// BuildInvoice constructs the seller-side Invoice payload.
func BuildInvoice(opts InvoiceOptions) (payment.Invoice, error) {
	if opts.EscrowRef == "" {
		return payment.Invoice{}, errors.New("adsb.BuildInvoice: EscrowRef required")
	}
	if opts.ReleaseRequestRef == "" {
		return payment.Invoice{}, errors.New("adsb.BuildInvoice: ReleaseRequestRef required (escrow.RequestRelease return)")
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

// PublishInvoice signs + publishes the seller-side Invoice.
func PublishInvoice(ctx context.Context, adapter topic.TopicAdapter, buyerStdIn topic.TopicRef, signingKey *keylib.NeuronPrivateKey, seq uint64, msg payment.Invoice) (topic.PublishResult, error) {
	return PublishPayload(ctx, adapter, buyerStdIn, signingKey, seq, msg)
}

// ReceiveInvoice blocks until an Invoice with requestID arrives.
func ReceiveInvoice(ctx context.Context, adapter topic.TopicAdapter, buyerStdIn topic.TopicRef, requestID string) (payment.Invoice, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, buyerStdIn, payment.PayloadInvoice, requestID)
	if err != nil {
		return payment.Invoice{}, topic.TopicMessage{}, err
	}
	var out payment.Invoice
	if err := json.Unmarshal(body, &out); err != nil {
		return payment.Invoice{}, topic.TopicMessage{}, fmt.Errorf("adsb.ReceiveInvoice: decode: %w", err)
	}
	return out, msg, nil
}

// InvoiceAck -----------------------------------------------------------------

// BuildInvoiceAck constructs the buyer-side InvoiceAck.
func BuildInvoiceAck(inv payment.Invoice, action string) payment.InvoiceAck {
	return payment.InvoiceAck{
		Type:              payment.PayloadInvoiceAck,
		Version:           PayloadVersion,
		RequestID:         inv.RequestID,
		ReleaseRequestRef: inv.ReleaseRequestRef,
		Action:            action,
	}
}

// PublishInvoiceAck publishes the buyer-side InvoiceAck.
func PublishInvoiceAck(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, signingKey *keylib.NeuronPrivateKey, seq uint64, msg payment.InvoiceAck) (topic.PublishResult, error) {
	return PublishPayload(ctx, adapter, sellerStdIn, signingKey, seq, msg)
}

// ReceiveInvoiceAck blocks until an InvoiceAck with requestID arrives.
func ReceiveInvoiceAck(ctx context.Context, adapter topic.TopicAdapter, sellerStdIn topic.TopicRef, requestID string) (payment.InvoiceAck, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, sellerStdIn, payment.PayloadInvoiceAck, requestID)
	if err != nil {
		return payment.InvoiceAck{}, topic.TopicMessage{}, err
	}
	var out payment.InvoiceAck
	if err := json.Unmarshal(body, &out); err != nil {
		return payment.InvoiceAck{}, topic.TopicMessage{}, fmt.Errorf("adsb.ReceiveInvoiceAck: decode: %w", err)
	}
	return out, msg, nil
}

// Evidence hash --------------------------------------------------------------

// EvidenceHashFor computes the canonical R2 invoice-evidence hash per D7
// using the ADS-B prefix.
func EvidenceHashFor(sellerPeerID string, frameCount uint64, firstFrameSentAt, lastFrameSentAt uint64) string {
	h := sha256.New()
	h.Write([]byte("adsb|"))
	h.Write([]byte(sellerPeerID))
	h.Write([]byte{'|'})
	h.Write([]byte(strconv.FormatUint(frameCount, 10)))
	h.Write([]byte{'|'})
	h.Write([]byte(strconv.FormatUint(firstFrameSentAt, 10)))
	h.Write([]byte{'|'})
	h.Write([]byte(strconv.FormatUint(lastFrameSentAt, 10)))
	return hex.EncodeToString(h.Sum(nil))
}
