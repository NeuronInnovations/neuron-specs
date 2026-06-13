package adsb

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

// PayloadVersion is the Version field stamped on every adsb negotiation
// envelope. Bumps per Spec 008 / 016 amendments.
const PayloadVersion = "1.0.0"

// ServiceRequestOptions configures BuildServiceRequest. Defaults match the
// R2 "free demo" posture used by the BaseStation slice.
type ServiceRequestOptions struct {
	RequestID           string
	ServiceRef          string
	SettlementBinding   string
	ProposedAmount      string
	ProposedCurrency    string
	ProposedInterval    string
	BuyerStdIn          string
	NegotiationDeadline uint64
}

// BuildServiceRequest constructs a buyer-side ServiceRequest payload.
// 008 FR-P07 anchors.
func BuildServiceRequest(opts ServiceRequestOptions) (payment.ServiceRequest, error) {
	if opts.RequestID == "" {
		return payment.ServiceRequest{}, errors.New("adsb.BuildServiceRequest: RequestID is required")
	}
	if opts.BuyerStdIn == "" {
		return payment.ServiceRequest{}, errors.New("adsb.BuildServiceRequest: BuyerStdIn is required (008 FR-P07 mandatory return-channel locator)")
	}
	if opts.ServiceRef == "" {
		opts.ServiceRef = P2PServiceName
	}
	if opts.SettlementBinding == "" {
		opts.SettlementBinding = DefaultSettlementBinding
	}
	if opts.ProposedAmount == "" {
		// MemoryEscrow rejects amount=0; the smallest positive integer is 1.
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

// BuildServiceResponse constructs a seller-side ServiceResponse.
func BuildServiceResponse(req payment.ServiceRequest, action string) payment.ServiceResponse {
	return payment.ServiceResponse{
		Type:      payment.PayloadServiceResponse,
		Version:   PayloadVersion,
		RequestID: req.RequestID,
		Action:    action,
	}
}

// PublishPayload signs the given payload with key and publishes the
// resulting TopicMessage to ref via adapter.
func PublishPayload(_ context.Context, adapter topic.TopicAdapter, ref topic.TopicRef, key *keylib.NeuronPrivateKey, seq uint64, payload any) (topic.PublishResult, error) {
	if adapter == nil {
		return topic.PublishResult{}, errors.New("adsb.PublishPayload: adapter is required")
	}
	if key == nil {
		return topic.PublishResult{}, errors.New("adsb.PublishPayload: key is required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return topic.PublishResult{}, fmt.Errorf("adsb.PublishPayload: marshal: %w", err)
	}
	ts := uint64(time.Now().UnixNano())
	msg, err := topic.NewTopicMessage(key, ts, seq, body)
	if err != nil {
		return topic.PublishResult{}, fmt.Errorf("adsb.PublishPayload: sign: %w", err)
	}
	result, err := adapter.Publish(ref, msg, topic.PublishOpts{ConfirmationMode: topic.WaitForConsensus})
	if err != nil {
		return topic.PublishResult{}, fmt.Errorf("adsb.PublishPayload: publish: %w", err)
	}
	return result, nil
}

// receiveTraceLogger optionally tracks ReceiveTypedPayload filter decisions.
var receiveTraceLogger *log.Logger

// SetReceiveTraceLogger sets a per-process trace logger.
func SetReceiveTraceLogger(l *log.Logger) { receiveTraceLogger = l }

// ReceiveTypedPayload subscribes to ref and returns the first TopicMessage
// whose JSON payload has both `type == payloadType` and (when requestID is
// non-empty) `requestId == requestID`.
func ReceiveTypedPayload(ctx context.Context, adapter topic.TopicAdapter, ref topic.TopicRef, payloadType, requestID string) ([]byte, topic.TopicMessage, error) {
	if adapter == nil {
		return nil, topic.TopicMessage{}, errors.New("adsb.ReceiveTypedPayload: adapter is required")
	}
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	fromZero := uint64(0)
	ch, err := adapter.Subscribe(subCtx, ref, topic.SubscribeOpts{FromSequence: &fromZero})
	if err != nil {
		return nil, topic.TopicMessage{}, fmt.Errorf("adsb.ReceiveTypedPayload: subscribe: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil, topic.TopicMessage{}, ctx.Err()
		case delivery, ok := <-ch:
			if !ok {
				return nil, topic.TopicMessage{}, errors.New("adsb.ReceiveTypedPayload: subscription closed before matching message arrived")
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

// ReceiveServiceRequest blocks until a serviceRequest envelope arrives.
func ReceiveServiceRequest(ctx context.Context, adapter topic.TopicAdapter, ref topic.TopicRef) (payment.ServiceRequest, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, ref, payment.PayloadServiceRequest, "")
	if err != nil {
		return payment.ServiceRequest{}, topic.TopicMessage{}, err
	}
	var req payment.ServiceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return payment.ServiceRequest{}, topic.TopicMessage{}, fmt.Errorf("adsb.ReceiveServiceRequest: decode: %w", err)
	}
	return req, msg, nil
}

// ReceiveServiceResponse blocks until a serviceResponse with requestID arrives.
func ReceiveServiceResponse(ctx context.Context, adapter topic.TopicAdapter, ref topic.TopicRef, requestID string) (payment.ServiceResponse, topic.TopicMessage, error) {
	body, msg, err := ReceiveTypedPayload(ctx, adapter, ref, payment.PayloadServiceResponse, requestID)
	if err != nil {
		return payment.ServiceResponse{}, topic.TopicMessage{}, err
	}
	var resp payment.ServiceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return payment.ServiceResponse{}, topic.TopicMessage{}, fmt.Errorf("adsb.ReceiveServiceResponse: decode: %w", err)
	}
	return resp, msg, nil
}
