package topic

import (
	"context"
	"encoding/json"
	"fmt"
)

// KafkaMaxMessageSize is the maximum message payload size for the Kafka backend (1 MB).
const KafkaMaxMessageSize = 1_048_576

// KafkaClient abstracts Kafka operations for testability.
// Production code injects a real Kafka client; tests inject a mock.
type KafkaClient interface {
	// CreateTopic creates a new Kafka topic with the given name, partition count,
	// and replication factor.
	CreateTopic(name string, partitions int, replicationFactor int) error
	// Publish submits a message to the given topic without waiting for acknowledgement.
	// Returns the transaction/record ID.
	Publish(topicName string, message []byte) (string, error)
	// PublishAndWait submits a message and blocks until acknowledged by the broker.
	// Returns the transaction ID and offset.
	PublishAndWait(topicName string, message []byte) (string, uint64, error)
	// Subscribe opens a subscription to the given topic.
	// If fromOffset is non-nil, it starts from that offset (inclusive).
	// Returns a channel that delivers KafkaMessage values.
	Subscribe(topicName string, fromOffset *uint64) (<-chan KafkaMessage, error)
	// GetTopicMetadata retrieves metadata for the given topic.
	GetTopicMetadata(topicName string) (TopicMetadata, error)
}

// KafkaMessage is the raw message structure received from a Kafka subscription.
type KafkaMessage struct {
	// Value is the raw message payload bytes.
	Value []byte
	// Offset is the Kafka partition offset for this message.
	Offset uint64
	// Timestamp is the producer-set timestamp in nanoseconds.
	Timestamp uint64
}

// KafkaAdapter implements TopicAdapter for the Kafka backend.
type KafkaAdapter struct {
	client    KafkaClient
	anchoring AnchoringConfig
}

// NewKafkaAdapter constructs a KafkaAdapter with the given Kafka client and anchoring config.
// Returns ErrInvalidConfig if the anchoring config is missing required fields.
func NewKafkaAdapter(client KafkaClient, anchoring AnchoringConfig) (*KafkaAdapter, error) {
	if err := validateAnchoringConfig(anchoring); err != nil {
		return nil, err
	}
	return &KafkaAdapter{
		client:    client,
		anchoring: anchoring,
	}, nil
}

// validateAnchoringConfig validates that all required anchoring fields are present.
func validateAnchoringConfig(cfg AnchoringConfig) error {
	if cfg.Method == "" {
		return NewTopicError(ErrInvalidConfig, "anchoring config missing required field: method")
	}
	if cfg.AnchorTopicId == "" {
		return NewTopicError(ErrInvalidConfig, "anchoring config missing required field: anchorTopicId")
	}
	if cfg.AnchorNetwork == "" {
		return NewTopicError(ErrInvalidConfig, "anchoring config missing required field: anchorNetwork")
	}
	if cfg.Interval == "" {
		return NewTopicError(ErrInvalidConfig, "anchoring config missing required field: interval")
	}
	return nil
}

// CreateTopic creates a new Kafka topic with default configuration.
func (a *KafkaAdapter) CreateTopic(opts CreateTopicOpts) (TopicRef, error) {
	if a.client == nil {
		return TopicRef{}, NewTopicError(ErrBackendUnavailable, "Kafka client is nil")
	}

	// Extract topic name from the memo or config.
	topicName := opts.Memo
	partitions := 1
	replicationFactor := 1

	// Allow overriding via config map.
	if opts.Config != nil {
		if name, ok := opts.Config["topicName"]; ok {
			if s, ok := name.(string); ok {
				topicName = s
			}
		}
		if p, ok := opts.Config["partitions"]; ok {
			if pi, ok := p.(int); ok {
				partitions = pi
			}
		}
		if rf, ok := opts.Config["replicationFactor"]; ok {
			if rfi, ok := rf.(int); ok {
				replicationFactor = rfi
			}
		}
	}

	if topicName == "" {
		return TopicRef{}, NewTopicError(ErrInvalidConfig, "Kafka topic name must not be empty")
	}

	if err := a.client.CreateTopic(topicName, partitions, replicationFactor); err != nil {
		return TopicRef{}, WrapTopicError(ErrBackendUnavailable, "failed to create Kafka topic", err)
	}

	return NewTopicRef(BackendKafka, topicName)
}

// Publish sends a signed TopicMessage to the specified Kafka topic.
// The message is serialized to JSON before submission.
// Returns ErrInvalidSignature if the message signature is missing or invalid.
// Returns ErrMessageTooLarge if the serialized message exceeds KafkaMaxMessageSize.
func (a *KafkaAdapter) Publish(ref TopicRef, msg TopicMessage, opts PublishOpts) (PublishResult, error) {
	// Validate message signature before publishing.
	if err := ValidateTopicMessage(msg); err != nil {
		return PublishResult{}, err
	}

	if a.client == nil {
		return PublishResult{}, NewTopicError(ErrBackendUnavailable, "Kafka client is nil")
	}

	if err := ref.Validate(); err != nil {
		return PublishResult{}, err
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return PublishResult{}, WrapTopicError(ErrInvalidConfig, "failed to serialize message", err)
	}

	if uint64(len(msgBytes)) > KafkaMaxMessageSize {
		return PublishResult{}, NewTopicError(ErrMessageTooLarge,
			fmt.Sprintf("serialized message size %d exceeds Kafka limit %d", len(msgBytes), KafkaMaxMessageSize))
	}

	switch opts.ConfirmationMode {
	case WaitForConsensus:
		txId, offset, err := a.client.PublishAndWait(ref.locator, msgBytes)
		if err != nil {
			return PublishResult{}, WrapTopicError(ErrBackendUnavailable, "failed to publish message with acknowledgement", err)
		}
		return PublishResult{
			TransactionRef: txId,
			SequenceNumber: &offset,
			Confirmed:      true,
		}, nil

	default:
		// FireAndForget or unspecified: submit without waiting.
		txId, err := a.client.Publish(ref.locator, msgBytes)
		if err != nil {
			return PublishResult{}, WrapTopicError(ErrBackendUnavailable, "failed to publish message", err)
		}
		return PublishResult{
			TransactionRef: txId,
			Confirmed:      false,
		}, nil
	}
}

// Subscribe opens a subscription to the specified Kafka topic.
// Returns a channel of MessageDelivery values. The channel is closed when
// the underlying Kafka subscription ends or the context is cancelled.
func (a *KafkaAdapter) Subscribe(ctx context.Context, ref TopicRef, opts SubscribeOpts) (<-chan MessageDelivery, error) {
	if a.client == nil {
		return nil, NewTopicError(ErrBackendUnavailable, "Kafka client is nil")
	}

	if err := ref.Validate(); err != nil {
		return nil, err
	}

	kafkaCh, err := a.client.Subscribe(ref.locator, opts.FromSequence)
	if err != nil {
		return nil, WrapTopicError(ErrBackendUnavailable, "failed to subscribe to Kafka topic", err)
	}

	deliveryCh := make(chan MessageDelivery)
	go func() {
		defer close(deliveryCh)
		for {
			select {
			case <-ctx.Done():
				return
			case kafkaMsg, ok := <-kafkaCh:
				if !ok {
					return
				}
				var topicMsg TopicMessage
				if jsonErr := json.Unmarshal(kafkaMsg.Value, &topicMsg); jsonErr != nil {
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
					ConsensusTimestamp: kafkaMsg.Timestamp,
					BackendSequence:    kafkaMsg.Offset,
				}
			}
		}
	}()

	return deliveryCh, nil
}

// Resolve retrieves metadata for the specified Kafka topic.
func (a *KafkaAdapter) Resolve(ref TopicRef) (TopicMetadata, error) {
	if a.client == nil {
		return TopicMetadata{}, NewTopicError(ErrBackendUnavailable, "Kafka client is nil")
	}

	if err := ref.Validate(); err != nil {
		return TopicMetadata{}, err
	}

	return a.client.GetTopicMetadata(ref.locator)
}

// MaxMessageSize returns the maximum message payload size for Kafka (1 MB).
func (a *KafkaAdapter) MaxMessageSize() uint64 {
	return KafkaMaxMessageSize
}

// EstimatePublishCost returns a zero cost estimate for Kafka (no native cost).
func (a *KafkaAdapter) EstimatePublishCost(_ uint64) (CostEstimate, error) {
	return CostEstimate{Amount: 0, Unit: "none"}, nil
}

// SupportedTransport returns BackendKafka.
func (a *KafkaAdapter) SupportedTransport() BackendKind {
	return BackendKafka
}
