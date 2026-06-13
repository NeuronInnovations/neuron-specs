package remoteid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// PayloadVersion is the Version field stamped on every Stage-2 remote-id
// negotiation envelope. Bumps per 008 / 017 amendments.
const PayloadVersion = "1.0.0"

// ServiceRequestOptions configures BuildServiceRequest. Defaults match the
// R2 "free demo" posture (FR-R01 pricing.unit=frame, FR-P58 commerceMode=full
// at the descriptor level — note that commerceMode lives on the heartbeat,
// not on the serviceRequest; here we're only filling the negotiation
// payload).
type ServiceRequestOptions struct {
	// RequestID identifies this negotiation. Required. The buyer is the
	// natural owner of the value; the seller echoes it on every reply
	// (ServiceResponse / ConnectionSetup / Invoice).
	RequestID string

	// ServiceRef cross-references a neuron-p2p-exchange service name in
	// the seller's AgentURI. Defaults to P2PServiceName.
	ServiceRef string

	// SettlementBinding identifies the escrow contract type. Defaults to
	// DefaultSettlementBinding ("evm-escrow"). Memory escrow tests use
	// the same string; the binding is opaque to the negotiation layer.
	SettlementBinding string

	// Pricing proposal. Defaults: amount "0", currency "USDC", interval "0".
	ProposedAmount   string
	ProposedCurrency string
	ProposedInterval string

	// BuyerStdIn is the locator of the buyer's stdIn topic. Required —
	// every seller→buyer reply (ServiceResponse, ConnectionSetup, Invoice)
	// publishes here.
	BuyerStdIn string

	// NegotiationDeadline is Unix epoch seconds (FR-W02a). 0 = no deadline.
	NegotiationDeadline uint64
}

// BuildServiceRequest constructs a buyer-side ServiceRequest payload.
// 008 FR-P07 anchors.
func BuildServiceRequest(opts ServiceRequestOptions) (payment.ServiceRequest, error) {
	if opts.RequestID == "" {
		return payment.ServiceRequest{}, errors.New("remoteid.BuildServiceRequest: RequestID is required")
	}
	if opts.BuyerStdIn == "" {
		return payment.ServiceRequest{}, errors.New("remoteid.BuildServiceRequest: BuyerStdIn is required (008 FR-P07 mandatory return-channel locator)")
	}
	if opts.ServiceRef == "" {
		opts.ServiceRef = P2PServiceName
	}
	if opts.SettlementBinding == "" {
		opts.SettlementBinding = DefaultSettlementBinding
	}
	if opts.ProposedAmount == "" {
		// MemoryEscrow rejects amount=0 in Deposit + RequestRelease;
		// the smallest positive integer that satisfies its check is 1.
		// The R2 demo is still "free" semantically (1 unit of USDC = a
		// negligible monetary value); the wire form just needs >0 so
		// the state machine + memory escrow round-trip cleanly.
		opts.ProposedAmount = "1"
	}
	if opts.ProposedCurrency == "" {
		opts.ProposedCurrency = "USDC"
	}
	if opts.ProposedInterval == "" {
		opts.ProposedInterval = "0"
	}
	return payment.ServiceRequest{
		Type:                payment.PayloadServiceRequest,
		Version:             PayloadVersion,
		RequestID:           opts.RequestID,
		ServiceRef:          opts.ServiceRef,
		SettlementBinding:   opts.SettlementBinding,
		ProposedAmount:      opts.ProposedAmount,
		ProposedCurrency:    opts.ProposedCurrency,
		ProposedInterval:    opts.ProposedInterval,
		NegotiationDeadline: opts.NegotiationDeadline,
		BuyerStdIn:          opts.BuyerStdIn,
	}, nil
}

// BuildServiceResponse constructs a seller-side ServiceResponse for the
// given action. Counter actions are out of scope for R2 (sellers always
// accept the R2 demo's free service).
func BuildServiceResponse(req payment.ServiceRequest, action string) payment.ServiceResponse {
	return payment.ServiceResponse{
		Type:      payment.PayloadServiceResponse,
		Version:   PayloadVersion,
		RequestID: req.RequestID,
		Action:    action,
	}
}

// PublishPayload signs the given payload with key and publishes the
// resulting TopicMessage to ref via adapter. The single helper covers
// every Stage-2 publish-side envelope (ServiceRequest, ServiceResponse,
// ConnectionSetup, EscrowCreated, Invoice, InvoiceAck, ServiceStop).
//
// seq MUST be monotonically increasing per sender per 004 FR-T06; the
// caller is responsible for tracking it.
func PublishPayload(_ context.Context, adapter topic.TopicAdapter, ref topic.TopicRef, key *keylib.NeuronPrivateKey, seq uint64, payload any) (topic.PublishResult, error) {
	if adapter == nil {
		return topic.PublishResult{}, errors.New("remoteid.PublishPayload: adapter is required")
	}
	if key == nil {
		return topic.PublishResult{}, errors.New("remoteid.PublishPayload: key is required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return topic.PublishResult{}, fmt.Errorf("remoteid.PublishPayload: marshal: %w", err)
	}
	ts := uint64(time.Now().UnixNano())
	msg, err := topic.NewTopicMessage(key, ts, seq, body)
	if err != nil {
		return topic.PublishResult{}, fmt.Errorf("remoteid.PublishPayload: sign: %w", err)
	}
	result, err := adapter.Publish(ref, msg, topic.PublishOpts{ConfirmationMode: topic.WaitForConsensus})
	if err != nil {
		return topic.PublishResult{}, fmt.Errorf("remoteid.PublishPayload: publish: %w", err)
	}
	return result, nil
}

// receiveTraceLogger lets tests inject a logger that observes each
// message ReceiveTypedPayload sifts through. Production never sets this.
var receiveTraceLogger *log.Logger

// SetReceiveTraceLogger configures a per-process trace logger for
// ReceiveTypedPayload. Tests use this to surface the off-type/off-request
// skips that otherwise stay silent. Calling with nil disables tracing.
func SetReceiveTraceLogger(l *log.Logger) { receiveTraceLogger = l }

// ReceiveTypedPayload subscribes to ref and returns the first TopicMessage
// whose JSON payload has both `type == payloadType` and (when requestID is
// non-empty) `requestId == requestID`. Off-type and off-request envelopes
// are silently skipped. Returns ctx.Err() if ctx is cancelled.
//
// Used by every Stage-2 receive-side helper; centralises the filtering so
// each envelope decoder only handles its happy path.
func ReceiveTypedPayload(ctx context.Context, adapter topic.TopicAdapter, ref topic.TopicRef, payloadType, requestID string) ([]byte, topic.TopicMessage, error) {
	if adapter == nil {
		return nil, topic.TopicMessage{}, errors.New("remoteid.ReceiveTypedPayload: adapter is required")
	}
	// Subscribe BEFORE looking at the topic so we get all live messages.
	// FromSequence=&0 asks for a full backfill + live stream — this lets
	// a late-subscribing seller / buyer catch up on messages the
	// counterparty already published, eliminating subscribe-after-publish
	// races on the MemoryTopicAdapter (real HCS adapter supports the
	// same semantic). The subscription's ctx MUST be a child of the
	// caller's so cancellation propagates; we ALSO cancel locally on
	// return so the adapter goroutine drains and the channel is freed
	// instead of receiving every subsequent message forever.
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	fromZero := uint64(0)
	ch, err := adapter.Subscribe(subCtx, ref, topic.SubscribeOpts{FromSequence: &fromZero})
	if err != nil {
		return nil, topic.TopicMessage{}, fmt.Errorf("remoteid.ReceiveTypedPayload: subscribe: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil, topic.TopicMessage{}, ctx.Err()
		case delivery, ok := <-ch:
			if !ok {
				return nil, topic.TopicMessage{}, errors.New("remoteid.ReceiveTypedPayload: subscription closed before matching message arrived")
			}
			body := delivery.Message.Payload()
			var probe struct {
				Type      string `json:"type"`
				RequestID string `json:"requestId"`
			}
			if err := json.Unmarshal(body, &probe); err != nil {
				if receiveTraceLogger != nil {
					receiveTraceLogger.Printf("[recv-trace] topic=%s want=%s/%s unmarshal-fail: %v", ref.Locator(), payloadType, requestID, err)
				}
				continue
			}
			if receiveTraceLogger != nil {
				receiveTraceLogger.Printf("[recv-trace] topic=%s want=%s/%s got=%s/%s seq=%d", ref.Locator(), payloadType, requestID, probe.Type, probe.RequestID, delivery.BackendSequence)
			}
			if probe.Type != payloadType {
				continue
			}
			if requestID != "" && probe.RequestID != requestID {
				continue
			}
			return body, delivery.Message, nil
		}
	}
}

// ReceiveServiceRequest blocks until a serviceRequest envelope arrives on
// ref (or ctx is cancelled). Seller-side.
func ReceiveServiceRequest(ctx context.Context, adapter topic.TopicAdapter, ref topic.TopicRef) (payment.ServiceRequest, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, ref, payment.PayloadServiceRequest, "")
	if err != nil {
		return payment.ServiceRequest{}, topic.TopicMessage{}, err
	}
	var req payment.ServiceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return payment.ServiceRequest{}, topic.TopicMessage{}, fmt.Errorf("remoteid.ReceiveServiceRequest: decode: %w", err)
	}
	return req, msg, nil
}

// ReceiveServiceResponse blocks until a serviceResponse with the given
// requestID arrives on ref. Buyer-side.
func ReceiveServiceResponse(ctx context.Context, adapter topic.TopicAdapter, ref topic.TopicRef, requestID string) (payment.ServiceResponse, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, ref, payment.PayloadServiceResponse, requestID)
	if err != nil {
		return payment.ServiceResponse{}, topic.TopicMessage{}, err
	}
	var resp payment.ServiceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return payment.ServiceResponse{}, topic.TopicMessage{}, fmt.Errorf("remoteid.ReceiveServiceResponse: decode: %w", err)
	}
	return resp, msg, nil
}
