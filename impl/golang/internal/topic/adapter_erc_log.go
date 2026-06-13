package topic

import (
	"context"
	"encoding/json"
)

// ERCLogClient abstracts Ethereum event log operations for testability.
// Production code injects a real Ethereum client; tests inject a mock.
type ERCLogClient interface {
	// SubscribeFilterLogs opens a live subscription to contract events.
	SubscribeFilterLogs(contractAddress string, eventSignature string) (<-chan ERCLogEvent, error)
	// FilterLogs queries historical contract events.
	FilterLogs(contractAddress string, eventSignature string, fromBlock, toBlock uint64) ([]ERCLogEvent, error)
	// GetContractInfo retrieves metadata for the given contract.
	GetContractInfo(contractAddress string) (TopicMetadata, error)
}

// ERCLogEvent is the raw event structure from an Ethereum log subscription.
type ERCLogEvent struct {
	// Data is the raw event data payload.
	Data []byte
	// BlockTimestamp is the block.timestamp in seconds.
	BlockTimestamp uint64
	// BlockNumber is the block number containing the event.
	BlockNumber uint64
	// LogIndex is the index of the log within the block.
	LogIndex uint64
}

// ERCLogAdapter implements TopicAdapter for the ERC event log backend.
// It is a read-only adapter: CreateTopic, Publish, and EstimatePublishCost
// return ErrUnsupportedOperation.
type ERCLogAdapter struct {
	client ERCLogClient
}

// NewERCLogAdapter constructs an ERCLogAdapter with the given ERC log client.
func NewERCLogAdapter(client ERCLogClient) *ERCLogAdapter {
	return &ERCLogAdapter{client: client}
}

// CreateTopic is not supported on the ERC event log adapter (read-only).
func (a *ERCLogAdapter) CreateTopic(_ CreateTopicOpts) (TopicRef, error) {
	return TopicRef{}, NewTopicError(ErrUnsupportedOperation, "CreateTopic is not supported on the ERC event log adapter (read-only)")
}

// Publish is not supported on the ERC event log adapter (read-only).
func (a *ERCLogAdapter) Publish(_ TopicRef, _ TopicMessage, _ PublishOpts) (PublishResult, error) {
	return PublishResult{}, NewTopicError(ErrUnsupportedOperation, "Publish is not supported on the ERC event log adapter (read-only)")
}

// Subscribe opens a subscription to Ethereum contract events and converts them
// to MessageDelivery values. Malformed messages (non-JSON or invalid TopicMessage)
// are reported via ErrorHandler if configured, then skipped.
func (a *ERCLogAdapter) Subscribe(ctx context.Context, ref TopicRef, opts SubscribeOpts) (<-chan MessageDelivery, error) {
	if a.client == nil {
		return nil, NewTopicError(ErrBackendUnavailable, "ERC log client is nil")
	}

	if err := ref.Validate(); err != nil {
		return nil, err
	}

	// Use the locator as contract address and a wildcard event signature.
	eventCh, err := a.client.SubscribeFilterLogs(ref.locator, "*")
	if err != nil {
		return nil, WrapTopicError(ErrBackendUnavailable, "failed to subscribe to ERC event logs", err)
	}

	deliveryCh := make(chan MessageDelivery)
	go func() {
		defer close(deliveryCh)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				var topicMsg TopicMessage
				if jsonErr := json.Unmarshal(event.Data, &topicMsg); jsonErr != nil {
					if opts.ErrorHandler != nil {
						opts.ErrorHandler(WrapTopicError(ErrInvalidConfig, "failed to deserialize message", jsonErr))
					}
					continue
				}
				if valErr := ValidateTopicMessage(topicMsg); valErr != nil {
					if opts.ErrorHandler != nil {
						opts.ErrorHandler(valErr)
					}
					continue
				}
				if opts.Dedup != nil && opts.Dedup.IsDuplicate(topicMsg) {
					continue
				}
				deliveryCh <- MessageDelivery{
					Message:            topicMsg,
					ConsensusTimestamp: event.BlockTimestamp * 1_000_000_000, // seconds to nanoseconds
					BackendSequence:    event.BlockNumber*10000 + event.LogIndex,
				}
			}
		}
	}()

	return deliveryCh, nil
}

// Resolve retrieves metadata for the specified ERC contract.
func (a *ERCLogAdapter) Resolve(ref TopicRef) (TopicMetadata, error) {
	if a.client == nil {
		return TopicMetadata{}, NewTopicError(ErrBackendUnavailable, "ERC log client is nil")
	}

	if err := ref.Validate(); err != nil {
		return TopicMetadata{}, err
	}

	return a.client.GetContractInfo(ref.locator)
}

// MaxMessageSize returns 0 because the ERC event log adapter has no publish capability.
func (a *ERCLogAdapter) MaxMessageSize() uint64 {
	return 0
}

// EstimatePublishCost is not supported on the ERC event log adapter (read-only).
func (a *ERCLogAdapter) EstimatePublishCost(_ uint64) (CostEstimate, error) {
	return CostEstimate{}, NewTopicError(ErrUnsupportedOperation, "EstimatePublishCost is not supported on the ERC event log adapter (read-only)")
}

// SupportedTransport returns BackendERCLog.
func (a *ERCLogAdapter) SupportedTransport() BackendKind {
	return BackendERCLog
}
