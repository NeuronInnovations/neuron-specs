package topic

import (
	"context"
	"encoding/json"
	"fmt"
)

// HCSMaxMessageSize is the maximum message payload size for the Hedera Consensus Service.
// HCS topics support messages up to 1024 bytes.
const HCSMaxMessageSize = 1024

// HCSClient abstracts the Hedera SDK for testability.
// Production code injects a real Hedera client; tests inject a mock.
type HCSClient interface {
	// CreateTopic creates a new HCS topic with the given memo.
	// Returns the topic ID in the format "0.0.NNNNN".
	CreateTopic(memo string) (string, error)
	// SubmitMessage submits a message to the given topic without waiting for consensus.
	// Returns the transaction ID.
	SubmitMessage(topicId string, message []byte) (string, error)
	// SubmitMessageAndWait submits a message and blocks until consensus is reached.
	// Returns the transaction ID, consensus timestamp (Unix nanos), and sequence number.
	SubmitMessageAndWait(topicId string, message []byte) (txId string, consensusTimestamp uint64, sequenceNumber uint64, err error)
	// SubscribeTopic opens a subscription to the given topic.
	// If startSequence is non-nil, it starts from that sequence number (inclusive).
	// Returns a channel that delivers HCSMessage values.
	SubscribeTopic(topicId string, startSequence *uint64) (<-chan HCSMessage, error)
	// GetTopicInfo retrieves metadata for the given topic.
	GetTopicInfo(topicId string) (TopicMetadata, error)
}

// HCSMessage is the raw message structure received from an HCS subscription.
type HCSMessage struct {
	// Contents is the raw message payload bytes.
	Contents []byte
	// ConsensusTimestamp is the Hedera-assigned consensus timestamp in Unix nanoseconds.
	ConsensusTimestamp uint64
	// SequenceNumber is the topic-assigned monotonic sequence number.
	SequenceNumber uint64
}

// HCSAdapter implements TopicAdapter for the Hedera Consensus Service backend.
type HCSAdapter struct {
	client HCSClient
}

// NewHCSAdapter constructs an HCSAdapter with the given HCS client.
func NewHCSAdapter(client HCSClient) *HCSAdapter {
	return &HCSAdapter{client: client}
}

// CreateTopic creates a new HCS topic with the configured memo.
func (a *HCSAdapter) CreateTopic(opts CreateTopicOpts) (TopicRef, error) {
	if a.client == nil {
		return TopicRef{}, NewTopicError(ErrBackendUnavailable, "HCS client is nil")
	}

	topicId, err := a.client.CreateTopic(opts.Memo)
	if err != nil {
		return TopicRef{}, WrapTopicError(ErrBackendUnavailable, "failed to create HCS topic", err)
	}

	return NewTopicRef(BackendHCS, topicId)
}

// Publish sends a signed TopicMessage to the specified HCS topic.
// The message is serialized to JSON before submission.
// Returns ErrInvalidSignature if the message signature is missing or invalid.
// Returns ErrMessageTooLarge if the serialized message exceeds HCSMaxMessageSize.
func (a *HCSAdapter) Publish(ref TopicRef, msg TopicMessage, opts PublishOpts) (PublishResult, error) {
	// Validate message signature before publishing.
	if err := ValidateTopicMessage(msg); err != nil {
		return PublishResult{}, err
	}

	if a.client == nil {
		return PublishResult{}, NewTopicError(ErrBackendUnavailable, "HCS client is nil")
	}

	if err := ref.Validate(); err != nil {
		return PublishResult{}, err
	}

	// Serialize the message to JSON for transport.
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return PublishResult{}, WrapTopicError(ErrInvalidConfig, "failed to serialize message", err)
	}

	if uint64(len(msgBytes)) > HCSMaxMessageSize {
		return PublishResult{}, NewTopicError(ErrMessageTooLarge,
			fmt.Sprintf("serialized message size %d exceeds HCS limit %d", len(msgBytes), HCSMaxMessageSize))
	}

	switch opts.ConfirmationMode {
	case WaitForConsensus:
		txId, consensusTS, seqNum, err := a.client.SubmitMessageAndWait(ref.locator, msgBytes)
		if err != nil {
			return PublishResult{}, WrapTopicError(ErrBackendUnavailable, "failed to publish message with consensus", err)
		}
		return PublishResult{
			TransactionRef:     txId,
			ConsensusTimestamp: &consensusTS,
			SequenceNumber:     &seqNum,
			Confirmed:          true,
		}, nil

	default:
		// FireAndForget or unspecified: submit without waiting.
		txId, err := a.client.SubmitMessage(ref.locator, msgBytes)
		if err != nil {
			return PublishResult{}, WrapTopicError(ErrBackendUnavailable, "failed to publish message", err)
		}
		return PublishResult{
			TransactionRef: txId,
			Confirmed:      false,
		}, nil
	}
}

// Subscribe opens a subscription to the specified HCS topic.
// Returns a channel of MessageDelivery values. The channel is closed when
// the underlying HCS subscription ends or the context is cancelled.
func (a *HCSAdapter) Subscribe(ctx context.Context, ref TopicRef, opts SubscribeOpts) (<-chan MessageDelivery, error) {
	if a.client == nil {
		return nil, NewTopicError(ErrBackendUnavailable, "HCS client is nil")
	}

	if err := ref.Validate(); err != nil {
		return nil, err
	}

	hcsCh, err := a.client.SubscribeTopic(ref.locator, opts.FromSequence)
	if err != nil {
		return nil, WrapTopicError(ErrBackendUnavailable, "failed to subscribe to HCS topic", err)
	}

	deliveryCh := make(chan MessageDelivery)
	go func() {
		defer close(deliveryCh)
		for {
			select {
			case <-ctx.Done():
				return
			case hcsMsg, ok := <-hcsCh:
				if !ok {
					return
				}
				var topicMsg TopicMessage
				if jsonErr := json.Unmarshal(hcsMsg.Contents, &topicMsg); jsonErr != nil {
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
					ConsensusTimestamp: hcsMsg.ConsensusTimestamp,
					BackendSequence:    hcsMsg.SequenceNumber,
				}
			}
		}
	}()

	return deliveryCh, nil
}

// Resolve retrieves metadata for the specified HCS topic.
func (a *HCSAdapter) Resolve(ref TopicRef) (TopicMetadata, error) {
	if a.client == nil {
		return TopicMetadata{}, NewTopicError(ErrBackendUnavailable, "HCS client is nil")
	}

	if err := ref.Validate(); err != nil {
		return TopicMetadata{}, err
	}

	return a.client.GetTopicInfo(ref.locator)
}

// MaxMessageSize returns the maximum message payload size for HCS (1024 bytes).
func (a *HCSAdapter) MaxMessageSize() uint64 {
	return HCSMaxMessageSize
}

// EstimatePublishCost returns an estimated cost in tinybar for publishing
// a message of the given size to an HCS topic.
func (a *HCSAdapter) EstimatePublishCost(messageSize uint64) (CostEstimate, error) {
	if messageSize > HCSMaxMessageSize {
		return CostEstimate{}, NewTopicError(ErrMessageTooLarge,
			fmt.Sprintf("message size %d exceeds HCS limit %d", messageSize, HCSMaxMessageSize))
	}
	// Base cost estimate: HCS submit message transaction fee.
	// Actual fee varies by network load; this is a reasonable default.
	return CostEstimate{
		Amount: 100_000, // ~0.001 HBAR in tinybar
		Unit:   "tinybar",
	}, nil
}

// SupportedTransport returns BackendHCS.
func (a *HCSAdapter) SupportedTransport() BackendKind {
	return BackendHCS
}
