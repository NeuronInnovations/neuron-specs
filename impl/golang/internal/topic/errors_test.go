package topic

import (
	"errors"
	"fmt"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllTopicErrorKinds_Count(t *testing.T) {
	kinds := AllTopicErrorKinds()
	assert.Len(t, kinds, 11, "there must be exactly 11 error kinds defined")
}

func TestNewTopicError_AllKinds(t *testing.T) {
	tests := []struct {
		kind    TopicErrorKind
		message string
	}{
		{ErrBackendUnavailable, "backend is down"},
		{ErrTopicNotFound, "topic 0.0.12345 not found"},
		{ErrMessageTooLarge, "payload exceeds 1024 bytes"},
		{ErrUnsupportedOperation, "publish not supported on erc-log adapter"},
		{ErrInvalidSignature, "signature verification failed"},
		{ErrSenderMismatch, "recovered address does not match senderAddress"},
		{ErrUnsupportedTransport, "unknown transport: foobar"},
		{ErrInvalidTopicRef, "locator is empty"},
		{ErrReservedChannelName, "metrics is a reserved channel name"},
		{ErrInvalidConfig, "missing required field: adminKey"},
		{ErrBrokenTopicRef, "cross-referenced topic does not exist"},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			err := NewTopicError(tt.kind, tt.message)

			assert.Equal(t, tt.kind, err.Kind)
			assert.Equal(t, tt.message, err.Message)
			assert.Nil(t, err.BackendError)

			expected := fmt.Sprintf("topic: [%s] %s", tt.kind, tt.message)
			assert.Equal(t, expected, err.Error())
		})
	}
}

func TestTopicError_ImplementsErrorInterface(t *testing.T) {
	var err error = NewTopicError(ErrTopicNotFound, "not found")
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "TopicNotFound")
}

func TestWrapTopicError(t *testing.T) {
	cause := errors.New("connection refused")
	err := WrapTopicError(ErrBackendUnavailable, "HCS unreachable", cause)

	assert.Equal(t, ErrBackendUnavailable, err.Kind)
	assert.Equal(t, "HCS unreachable", err.Message)
	assert.Equal(t, cause, err.BackendError)

	expected := "topic: [BackendUnavailable] HCS unreachable"
	assert.Equal(t, expected, err.Error())
}

func TestTopicError_Unwrap(t *testing.T) {
	t.Run("with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := WrapTopicError(ErrInvalidConfig, "bad config", cause)

		unwrapped := err.Unwrap()
		assert.Equal(t, cause, unwrapped)

		// errors.Is should traverse the chain.
		assert.True(t, errors.Is(err, cause))
	})

	t.Run("without cause", func(t *testing.T) {
		err := NewTopicError(ErrInvalidConfig, "bad config")

		unwrapped := err.Unwrap()
		assert.Nil(t, unwrapped)
	})
}

func TestTopicError_ErrorsAs(t *testing.T) {
	err := NewTopicError(ErrMessageTooLarge, "too big")

	var topicErr TopicError
	require.True(t, errors.As(err, &topicErr))
	assert.Equal(t, ErrMessageTooLarge, topicErr.Kind)
}

func TestWrapTopicError_NestedChain(t *testing.T) {
	root := errors.New("TCP reset")
	inner := WrapTopicError(ErrBackendUnavailable, "inner", root)
	outer := fmt.Errorf("outer context: %w", inner)

	// errors.Is should find the root cause through the chain.
	assert.True(t, errors.Is(outer, root))

	// errors.As should find the TopicError in the chain.
	var topicErr TopicError
	require.True(t, errors.As(outer, &topicErr))
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)
}

func TestTopicErrorKind_StringValues(t *testing.T) {
	// Verify the string representations are stable and match the spec.
	assert.Equal(t, TopicErrorKind("BackendUnavailable"), ErrBackendUnavailable)
	assert.Equal(t, TopicErrorKind("TopicNotFound"), ErrTopicNotFound)
	assert.Equal(t, TopicErrorKind("MessageTooLarge"), ErrMessageTooLarge)
	assert.Equal(t, TopicErrorKind("UnsupportedOperation"), ErrUnsupportedOperation)
	assert.Equal(t, TopicErrorKind("InvalidSignature"), ErrInvalidSignature)
	assert.Equal(t, TopicErrorKind("SenderMismatch"), ErrSenderMismatch)
	assert.Equal(t, TopicErrorKind("UnsupportedTransport"), ErrUnsupportedTransport)
	assert.Equal(t, TopicErrorKind("InvalidTopicRef"), ErrInvalidTopicRef)
	assert.Equal(t, TopicErrorKind("ReservedChannelName"), ErrReservedChannelName)
	assert.Equal(t, TopicErrorKind("InvalidConfig"), ErrInvalidConfig)
	assert.Equal(t, TopicErrorKind("BrokenTopicRef"), ErrBrokenTopicRef)
}

// --- Phase 10 (Item 11): Error Kind Coverage Test ---
// Trigger all 11 error kinds through actual API calls (not just construction).

func TestErrorKindCoverage_AllKindsThroughAPICalls(t *testing.T) {
	triggered := make(map[TopicErrorKind]bool)

	assertKind := func(t *testing.T, err error, kind TopicErrorKind) {
		t.Helper()
		require.Error(t, err)
		var topicErr TopicError
		require.True(t, errors.As(err, &topicErr), "expected TopicError, got %T: %v", err, err)
		assert.Equal(t, kind, topicErr.Kind)
		triggered[kind] = true
	}

	t.Run("BackendUnavailable via nil HCS client", func(t *testing.T) {
		adapter := NewHCSAdapter(nil)
		_, err := adapter.CreateTopic(CreateTopicOpts{Memo: "test"})
		assertKind(t, err, ErrBackendUnavailable)
	})

	t.Run("TopicNotFound via FindStdIn on empty slice", func(t *testing.T) {
		_, err := FindStdIn(nil)
		assertKind(t, err, ErrTopicNotFound)
	})

	t.Run("MessageTooLarge via HCS publish oversized message", func(t *testing.T) {
		key, err := keylib.NewNeuronPrivateKey()
		require.NoError(t, err)

		bigPayload := make([]byte, 2000)
		msg, err := NewTopicMessage(&key, 100, 1, bigPayload)
		require.NoError(t, err)

		adapter := NewHCSAdapter(&mockHCSClient{})
		ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
		_, err = adapter.Publish(ref, msg, PublishOpts{})
		assertKind(t, err, ErrMessageTooLarge)
	})

	t.Run("UnsupportedOperation via ERC log CreateTopic", func(t *testing.T) {
		adapter := NewERCLogAdapter(&mockERCLogClient{})
		_, err := adapter.CreateTopic(CreateTopicOpts{Memo: "test"})
		assertKind(t, err, ErrUnsupportedOperation)
	})

	t.Run("InvalidSignature via ValidateTopicMessage with empty signature", func(t *testing.T) {
		msg := TopicMessage{
			senderAddress:  "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD00",
			signature:      nil,
			timestamp:      100,
			sequenceNumber: 1,
			payload:        []byte("test"),
		}
		err := ValidateTopicMessage(msg)
		assertKind(t, err, ErrInvalidSignature)
	})

	t.Run("SenderMismatch via ValidateTopicMessage with tampered payload", func(t *testing.T) {
		key, err := keylib.NewNeuronPrivateKey()
		require.NoError(t, err)

		msg, err := NewTopicMessage(&key, 100, 1, []byte("original"))
		require.NoError(t, err)

		msg.payload = []byte("tampered")
		err = ValidateTopicMessage(msg)
		assertKind(t, err, ErrSenderMismatch)
	})

	t.Run("UnsupportedTransport via GetAdapter for unregistered transport", func(t *testing.T) {
		ResetAdapterRegistry()
		defer ResetAdapterRegistry()

		_, err := GetAdapter(BackendKind("nonexistent"))
		assertKind(t, err, ErrUnsupportedTransport)
	})

	t.Run("InvalidTopicRef via TopicRef.Validate with empty transport", func(t *testing.T) {
		ref := TopicRef{transport: "", locator: "test"}
		err := ref.Validate()
		assertKind(t, err, ErrInvalidTopicRef)
	})

	t.Run("ReservedChannelName via ChannelRoleFromString with bare name", func(t *testing.T) {
		_, err := ChannelRoleFromString("metrics")
		assertKind(t, err, ErrReservedChannelName)
	})

	t.Run("InvalidConfig via NewKafkaAdapter with missing anchoring method", func(t *testing.T) {
		_, err := NewKafkaAdapter(&mockKafkaClient{}, AnchoringConfig{})
		assertKind(t, err, ErrInvalidConfig)
	})

	t.Run("BrokenTopicRef via ValidateCrossReferences with dangling reference", func(t *testing.T) {
		topics := []NeuronTopicServiceDef{
			{
				Transport: "hcs",
				Config: map[string]interface{}{
					"network": "mainnet",
					"topicId": "0.0.11111",
				},
			},
		}
		p2p := []NeuronP2PExchangeServiceDef{
			{
				Name:     "broken-ref",
				TopicRef: "hcs://0.0.99999", // Does not exist in topics.
			},
		}
		err := ValidateCrossReferences(topics, p2p)
		assertKind(t, err, ErrBrokenTopicRef)
	})

	// Final verification: ensure all 11 kinds were triggered.
	allKinds := AllTopicErrorKinds()
	for _, kind := range allKinds {
		assert.True(t, triggered[kind], "error kind %s was not triggered through an API call", kind)
	}
}

// mockHCSClientForErrors is a minimal mock for error coverage tests.
type mockHCSClientForErrors struct{}

func (m *mockHCSClientForErrors) CreateTopic(_ string) (string, error) {
	return "0.0.12345", nil
}
func (m *mockHCSClientForErrors) SubmitMessage(_ string, _ []byte) (string, error) {
	return "tx-001", nil
}
func (m *mockHCSClientForErrors) SubmitMessageAndWait(_ string, _ []byte) (string, uint64, uint64, error) {
	return "tx-002", 100, 1, nil
}
func (m *mockHCSClientForErrors) SubscribeTopic(_ string, _ *uint64) (<-chan HCSMessage, error) {
	ch := make(chan HCSMessage)
	close(ch)
	return ch, nil
}
func (m *mockHCSClientForErrors) GetTopicInfo(_ string) (TopicMetadata, error) {
	return TopicMetadata{}, nil
}

// mockERCLogClientForErrors is used in the error coverage test.
type mockERCLogClientForErrors struct{}

func (m *mockERCLogClientForErrors) SubscribeFilterLogs(_ string, _ string) (<-chan ERCLogEvent, error) {
	ch := make(chan ERCLogEvent)
	close(ch)
	return ch, nil
}
func (m *mockERCLogClientForErrors) FilterLogs(_ string, _ string, _, _ uint64) ([]ERCLogEvent, error) {
	return nil, nil
}
func (m *mockERCLogClientForErrors) GetContractInfo(_ string) (TopicMetadata, error) {
	return TopicMetadata{}, nil
}
