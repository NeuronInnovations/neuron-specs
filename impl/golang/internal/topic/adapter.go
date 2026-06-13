package topic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CreateTopicOpts holds configuration for creating a new topic on a backend.
type CreateTopicOpts struct {
	// Transport specifies which backend to create the topic on.
	Transport BackendKind
	// AdminKey is the key authorized to manage the topic.
	// Will be NeuronPrivateKey once keylib integration is complete.
	AdminKey interface{}
	// Memo is an optional human-readable description of the topic.
	Memo string
	// Config holds backend-specific configuration parameters.
	// For HCS: submitKey, autoRenewPeriod, etc.
	// For Kafka: partitions, replicationFactor, anchoringConfig, etc.
	Config map[string]interface{}
}

// FR-T23: Confirmation modes: FIRE_AND_FORGET, WAIT_FOR_CONSENSUS
// PublishOpts configures the behavior of a publish operation.
type PublishOpts struct {
	// ConfirmationMode determines whether publish blocks for consensus.
	ConfirmationMode ConfirmationMode
}

// FR-T25: Subscribe resumption from sequence with dedup support
// SubscribeOpts configures the behavior of a subscribe operation.
type SubscribeOpts struct {
	// FromSequence, if non-nil, specifies the sequence number to start
	// reading from (inclusive). Nil means start from the latest message.
	FromSequence *uint64
	// Dedup, if non-nil, enables replay detection. Messages whose signature
	// has already been seen are silently dropped.
	Dedup *DeduplicationTracker
	// ErrorHandler, if non-nil, is called when a received message fails
	// deserialization or signature validation. This enables logging/monitoring
	// of dropped messages.
	ErrorHandler func(error)
}

// TopicMetadata holds backend-reported metadata about a topic.
type TopicMetadata struct {
	// TopicRef is the reference to the topic on the backend.
	TopicRef TopicRef
	// SequenceNumber is the latest message sequence number on the topic.
	SequenceNumber uint64
	// CreatedAt is the topic creation timestamp in Unix nanoseconds.
	CreatedAt uint64
	// AdminKey is the public key authorized to manage the topic.
	// Will be *NeuronPublicKey once keylib integration is complete.
	AdminKey interface{}
	// Memo is the topic description set at creation time.
	Memo string
}

// CostEstimate represents an estimated cost for a topic operation.
type CostEstimate struct {
	// Amount is the estimated cost in the smallest unit of the currency.
	Amount uint64 `json:"amount"`
	// Unit is the currency unit (e.g., "tinybar", "wei", "usd-cent").
	Unit string `json:"unit"`
}

// FR-T04: TopicAdapter interface (createTopic, publish, subscribe, resolve)
// TopicAdapter is the unified interface for backend-specific topic operations.
// Each adapter implementation wraps a particular transport backend (HCS, Kafka, ERC log, etc.)
// and provides a common API for creating topics, publishing messages, subscribing, and resolving metadata.
type TopicAdapter interface {
	// CreateTopic creates a new topic on the backend with the given configuration.
	CreateTopic(opts CreateTopicOpts) (TopicRef, error)
	// Publish sends a signed TopicMessage to the specified topic.
	Publish(ref TopicRef, msg TopicMessage, opts PublishOpts) (PublishResult, error)
	// Subscribe opens a subscription channel that delivers messages from the topic.
	// The context controls the lifetime of the subscription goroutine; cancelling
	// the context stops message delivery and closes the returned channel.
	Subscribe(ctx context.Context, ref TopicRef, opts SubscribeOpts) (<-chan MessageDelivery, error)
	// Resolve retrieves metadata about the specified topic from the backend.
	Resolve(ref TopicRef) (TopicMetadata, error)
	// FR-T27: maxMessageSize query and pre-publish size check
	// MaxMessageSize returns the maximum payload size in bytes for this backend.
	MaxMessageSize() uint64
	// FR-T28: estimatePublishCost for fee-bearing backends
	// EstimatePublishCost returns an estimated cost for publishing a message of the given size.
	EstimatePublishCost(messageSize uint64) (CostEstimate, error)
	// SupportedTransport returns the BackendKind this adapter handles.
	SupportedTransport() BackendKind
}

// adapterMu protects the adapterRegistry map from concurrent access.
var adapterMu sync.RWMutex

// FR-T13: Independent channel backing (different backends per channel)
// adapterRegistry maps backend kinds to their adapter implementations.
var adapterRegistry = map[BackendKind]TopicAdapter{}

// FR-T05: Support >=2 built-in adapters with runtime registration
// RegisterAdapter registers a TopicAdapter for its supported transport.
// Returns an error if an adapter is already registered for the same transport.
func RegisterAdapter(adapter TopicAdapter) error {
	if adapter == nil {
		return NewTopicError(ErrInvalidConfig, "adapter must not be nil")
	}

	transport := adapter.SupportedTransport()

	adapterMu.Lock()
	defer adapterMu.Unlock()

	if _, exists := adapterRegistry[transport]; exists {
		return NewTopicError(ErrInvalidConfig,
			fmt.Sprintf("adapter already registered for transport: %s", transport))
	}

	adapterRegistry[transport] = adapter
	return nil
}

// GetAdapter returns the registered TopicAdapter for the given transport.
// Returns ErrUnsupportedTransport if no adapter is registered for the transport.
func GetAdapter(transport BackendKind) (TopicAdapter, error) {
	adapterMu.RLock()
	defer adapterMu.RUnlock()

	adapter, ok := adapterRegistry[transport]
	if !ok {
		return nil, NewTopicError(ErrUnsupportedTransport,
			fmt.Sprintf("no adapter registered for transport: %s", transport))
	}
	return adapter, nil
}

// ResetAdapterRegistry clears all registered adapters. Intended for testing only.
func ResetAdapterRegistry() {
	adapterMu.Lock()
	defer adapterMu.Unlock()

	adapterRegistry = map[BackendKind]TopicAdapter{}
}

// FR-T26: Exponential backoff retry (baseDelay x 2^attempt, max 5)
// RetryConfig configures exponential backoff retry behavior.
type RetryConfig struct {
	// BaseDelay is the initial delay between retries. Default: 1s.
	BaseDelay time.Duration
	// MaxRetries is the maximum number of retry attempts. Default: 5.
	MaxRetries int
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		BaseDelay:  1 * time.Second,
		MaxRetries: 5,
	}
}

// RetryWithBackoff executes fn with exponential backoff retry.
// Retries on transient errors (TopicError with BackendUnavailable kind).
// Returns immediately on permanent errors or success.
func RetryWithBackoff(config RetryConfig, fn func() error) error {
	if config.BaseDelay <= 0 {
		config.BaseDelay = DefaultRetryConfig().BaseDelay
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = DefaultRetryConfig().MaxRetries
	}

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Only retry on transient (BackendUnavailable) errors.
		var topicErr TopicError
		if errors.As(lastErr, &topicErr) && topicErr.Kind == ErrBackendUnavailable {
			if attempt < config.MaxRetries {
				delay := config.BaseDelay * (1 << uint(attempt))
				time.Sleep(delay)
				continue
			}
		}

		// Permanent error: return immediately.
		return lastErr
	}

	return lastErr
}
