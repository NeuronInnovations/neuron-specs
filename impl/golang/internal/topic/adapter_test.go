package topic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAdapter implements TopicAdapter for registry tests.
type mockAdapter struct {
	transport BackendKind
}

func (m *mockAdapter) CreateTopic(_ CreateTopicOpts) (TopicRef, error) {
	return TopicRef{transport: m.transport, locator: "mock-locator"}, nil
}

func (m *mockAdapter) Publish(_ TopicRef, _ TopicMessage, _ PublishOpts) (PublishResult, error) {
	return PublishResult{TransactionRef: "mock-tx"}, nil
}

func (m *mockAdapter) Subscribe(_ context.Context, _ TopicRef, _ SubscribeOpts) (<-chan MessageDelivery, error) {
	ch := make(chan MessageDelivery)
	close(ch)
	return ch, nil
}

func (m *mockAdapter) Resolve(_ TopicRef) (TopicMetadata, error) {
	return TopicMetadata{}, nil
}

func (m *mockAdapter) MaxMessageSize() uint64 {
	return 4096
}

func (m *mockAdapter) EstimatePublishCost(_ uint64) (CostEstimate, error) {
	return CostEstimate{Amount: 100, Unit: "mock"}, nil
}

func (m *mockAdapter) SupportedTransport() BackendKind {
	return m.transport
}

func TestRegisterAdapter_Success(t *testing.T) {
	ResetAdapterRegistry()
	defer ResetAdapterRegistry()

	adapter := &mockAdapter{transport: BackendHCS}
	err := RegisterAdapter(adapter)
	require.NoError(t, err)
}

func TestRegisterAdapter_DuplicateRejected(t *testing.T) {
	ResetAdapterRegistry()
	defer ResetAdapterRegistry()

	adapter1 := &mockAdapter{transport: BackendHCS}
	adapter2 := &mockAdapter{transport: BackendHCS}

	err := RegisterAdapter(adapter1)
	require.NoError(t, err)

	err = RegisterAdapter(adapter2)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "already registered")
}

func TestRegisterAdapter_NilRejected(t *testing.T) {
	ResetAdapterRegistry()
	defer ResetAdapterRegistry()

	err := RegisterAdapter(nil)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
}

func TestGetAdapter_Success(t *testing.T) {
	ResetAdapterRegistry()
	defer ResetAdapterRegistry()

	adapter := &mockAdapter{transport: BackendKafka}
	err := RegisterAdapter(adapter)
	require.NoError(t, err)

	got, err := GetAdapter(BackendKafka)
	require.NoError(t, err)
	assert.Equal(t, adapter, got)
}

func TestGetAdapter_NotFound(t *testing.T) {
	ResetAdapterRegistry()
	defer ResetAdapterRegistry()

	_, err := GetAdapter(BackendHCS)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrUnsupportedTransport, topicErr.Kind)
}

func TestResetAdapterRegistry(t *testing.T) {
	ResetAdapterRegistry()
	defer ResetAdapterRegistry()

	adapter := &mockAdapter{transport: BackendHCS}
	err := RegisterAdapter(adapter)
	require.NoError(t, err)

	// Verify it is registered.
	_, err = GetAdapter(BackendHCS)
	require.NoError(t, err)

	// Reset and verify it is gone.
	ResetAdapterRegistry()

	_, err = GetAdapter(BackendHCS)
	require.Error(t, err)
}

func TestRegisterAdapter_MultipleTransports(t *testing.T) {
	ResetAdapterRegistry()
	defer ResetAdapterRegistry()

	hcsAdapter := &mockAdapter{transport: BackendHCS}
	kafkaAdapter := &mockAdapter{transport: BackendKafka}

	require.NoError(t, RegisterAdapter(hcsAdapter))
	require.NoError(t, RegisterAdapter(kafkaAdapter))

	got, err := GetAdapter(BackendHCS)
	require.NoError(t, err)
	assert.Equal(t, hcsAdapter, got)

	got, err = GetAdapter(BackendKafka)
	require.NoError(t, err)
	assert.Equal(t, kafkaAdapter, got)
}

// --- Custom Adapter Registration Test (Phase 8, Item 4) ---

// inMemoryAdapter aliases the public MemoryTopicAdapter (memory_adapter.go);
// kept for historical test names. New tests should use MemoryTopicAdapter
// + NewMemoryTopicAdapter directly.
type inMemoryAdapter = MemoryTopicAdapter

// newInMemoryAdapter aliases NewMemoryTopicAdapter for historical tests.
func newInMemoryAdapter() *inMemoryAdapter { return NewMemoryTopicAdapter() }

func TestCustomAdapter_InMemory_FullLifecycle(t *testing.T) {
	ResetAdapterRegistry()
	defer ResetAdapterRegistry()

	adapter := newInMemoryAdapter()

	// Register the custom adapter.
	err := RegisterAdapter(adapter)
	require.NoError(t, err)

	// Retrieve via registry.
	got, err := GetAdapter(BackendKind("custom:inmemory"))
	require.NoError(t, err)
	assert.Equal(t, adapter, got)

	// Create topic.
	ref, err := adapter.CreateTopic(CreateTopicOpts{Memo: "test-channel"})
	require.NoError(t, err)
	assert.Equal(t, BackendKind("custom:inmemory"), ref.transport)
	assert.Equal(t, "test-channel", ref.locator)

	// Subscribe.
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.NoError(t, err)

	// Publish a signed message.
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("custom adapter test"))
	require.NoError(t, err)

	result, err := adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: WaitForConsensus})
	require.NoError(t, err)
	assert.True(t, result.Confirmed)
	assert.Contains(t, result.TransactionRef, "inmemory-tx-")

	// Verify subscriber received the message.
	delivery := <-deliveryCh
	assert.Equal(t, msg.senderAddress, delivery.Message.senderAddress)
	assert.Equal(t, msg.payload, delivery.Message.payload)

	// Validate the message integrity.
	err = ValidateTopicMessage(delivery.Message)
	assert.NoError(t, err)

	// Verify the message can be serialized to JSON and back.
	msgJSON, err := json.Marshal(delivery.Message)
	require.NoError(t, err)

	var restored TopicMessage
	require.NoError(t, json.Unmarshal(msgJSON, &restored))
	assert.Equal(t, msg.senderAddress, restored.senderAddress)
}

// --- Retry Tests (Phase 8, Item 3) ---

func TestRetryWithBackoff_ImmediateSuccess(t *testing.T) {
	callCount := 0
	err := RetryWithBackoff(RetryConfig{BaseDelay: 1 * time.Millisecond, MaxRetries: 3}, func() error {
		callCount++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRetryWithBackoff_PermanentError(t *testing.T) {
	callCount := 0
	err := RetryWithBackoff(RetryConfig{BaseDelay: 1 * time.Millisecond, MaxRetries: 3}, func() error {
		callCount++
		return NewTopicError(ErrMessageTooLarge, "permanent error")
	})
	require.Error(t, err)
	assert.Equal(t, 1, callCount, "should not retry on permanent errors")

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrMessageTooLarge, topicErr.Kind)
}

func TestRetryWithBackoff_TransientThenSuccess(t *testing.T) {
	callCount := 0
	err := RetryWithBackoff(RetryConfig{BaseDelay: 1 * time.Millisecond, MaxRetries: 5}, func() error {
		callCount++
		if callCount < 3 {
			return NewTopicError(ErrBackendUnavailable, "transient error")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestRetryWithBackoff_ExhaustedRetries(t *testing.T) {
	callCount := 0
	err := RetryWithBackoff(RetryConfig{BaseDelay: 1 * time.Millisecond, MaxRetries: 2}, func() error {
		callCount++
		return NewTopicError(ErrBackendUnavailable, "always fails")
	})
	require.Error(t, err)
	// 1 initial attempt + 2 retries = 3 total calls
	assert.Equal(t, 3, callCount)
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	assert.Equal(t, 1*time.Second, cfg.BaseDelay)
	assert.Equal(t, 5, cfg.MaxRetries)
}

func TestRetryWithBackoff_DefaultValues(t *testing.T) {
	// Zero values should use defaults internally.
	callCount := 0
	err := RetryWithBackoff(RetryConfig{}, func() error {
		callCount++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}
