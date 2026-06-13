package topic

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- BackendKind Tests (T004) ---

func TestParseBackendKind_KnownKinds(t *testing.T) {
	tests := []struct {
		input    string
		expected BackendKind
	}{
		{"hcs", BackendHCS},
		{"erc-log", BackendERCLog},
		{"kafka", BackendKafka},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			kind, err := ParseBackendKind(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, kind)
		})
	}
}

func TestParseBackendKind_CustomKinds(t *testing.T) {
	tests := []struct {
		input    string
		expected BackendKind
	}{
		{"custom:redis", BackendKind("custom:redis")},
		{"custom:nats", BackendKind("custom:nats")},
		{"custom:my-backend", BackendKind("custom:my-backend")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			kind, err := ParseBackendKind(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, kind)
		})
	}
}

func TestParseBackendKind_RejectsEmpty(t *testing.T) {
	_, err := ParseBackendKind("")
	require.Error(t, err)

	var topicErr TopicError
	require.True(t, errors.As(err, &topicErr))
	assert.Equal(t, ErrUnsupportedTransport, topicErr.Kind)
}

func TestParseBackendKind_RejectsUnknown(t *testing.T) {
	tests := []string{"foobar", "HCS", "Kafka", "erc_log", "unknown"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := ParseBackendKind(input)
			require.Error(t, err)

			var topicErr TopicError
			require.True(t, errors.As(err, &topicErr))
			assert.Equal(t, ErrUnsupportedTransport, topicErr.Kind)
		})
	}
}

func TestParseBackendKind_RejectsEmptyCustomName(t *testing.T) {
	_, err := ParseBackendKind("custom:")
	require.Error(t, err)

	var topicErr TopicError
	require.True(t, errors.As(err, &topicErr))
	assert.Equal(t, ErrUnsupportedTransport, topicErr.Kind)
}

func TestIsCustomBackend(t *testing.T) {
	tests := []struct {
		kind     BackendKind
		expected bool
	}{
		{BackendHCS, false},
		{BackendERCLog, false},
		{BackendKafka, false},
		{BackendKind("custom:redis"), true},
		{BackendKind("custom:nats"), true},
		{BackendKind("foobar"), false},
		{BackendKind(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			assert.Equal(t, tt.expected, IsCustomBackend(tt.kind))
		})
	}
}

// --- TopicRef Tests (T005) ---

func TestNewTopicRef_ValidConstruction(t *testing.T) {
	tests := []struct {
		name      string
		transport BackendKind
		locator   string
	}{
		{"hcs", BackendHCS, "0.0.12345"},
		{"erc-log", BackendERCLog, "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD"},
		{"kafka", BackendKafka, "my-topic-name"},
		{"custom", BackendKind("custom:redis"), "channel-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := NewTopicRef(tt.transport, tt.locator)
			require.NoError(t, err)
			assert.Equal(t, tt.transport, ref.transport)
			assert.Equal(t, tt.locator, ref.locator)
		})
	}
}

func TestNewTopicRef_RejectsEmptyTransport(t *testing.T) {
	_, err := NewTopicRef("", "some-locator")
	require.Error(t, err)

	var topicErr TopicError
	require.True(t, errors.As(err, &topicErr))
	assert.Equal(t, ErrInvalidTopicRef, topicErr.Kind)
}

func TestNewTopicRef_RejectsEmptyLocator(t *testing.T) {
	_, err := NewTopicRef(BackendHCS, "")
	require.Error(t, err)

	var topicErr TopicError
	require.True(t, errors.As(err, &topicErr))
	assert.Equal(t, ErrInvalidTopicRef, topicErr.Kind)
}

func TestNewTopicRef_RejectsUnknownTransport(t *testing.T) {
	_, err := NewTopicRef(BackendKind("foobar"), "some-locator")
	require.Error(t, err)

	var topicErr TopicError
	require.True(t, errors.As(err, &topicErr))
	assert.Equal(t, ErrInvalidTopicRef, topicErr.Kind)
}

func TestTopicRef_Validate(t *testing.T) {
	t.Run("valid ref passes", func(t *testing.T) {
		ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
		assert.NoError(t, ref.Validate())
	})

	t.Run("empty transport fails", func(t *testing.T) {
		ref := TopicRef{transport: "", locator: "0.0.12345"}
		assert.Error(t, ref.Validate())
	})

	t.Run("empty locator fails", func(t *testing.T) {
		ref := TopicRef{transport: BackendHCS, locator: ""}
		assert.Error(t, ref.Validate())
	})
}

func TestTopicRef_URI_KnownSchemes(t *testing.T) {
	tests := []struct {
		name     string
		ref      TopicRef
		expected string
	}{
		{
			name:     "hcs",
			ref:      TopicRef{transport: BackendHCS, locator: "0.0.12345"},
			expected: "hcs://0.0.12345",
		},
		{
			name:     "erc-log",
			ref:      TopicRef{transport: BackendERCLog, locator: "0x742d35Cc"},
			expected: "erc-log://0x742d35Cc",
		},
		{
			name:     "kafka",
			ref:      TopicRef{transport: BackendKafka, locator: "my-topic"},
			expected: "kafka+ledger://my-topic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ref.URI())
		})
	}
}

func TestTopicRef_URI_CustomScheme(t *testing.T) {
	ref := TopicRef{transport: BackendKind("custom:redis"), locator: "channel-1"}
	assert.Equal(t, "redis://channel-1", ref.URI())
}

func TestTopicRefFromURI_KnownSchemes(t *testing.T) {
	tests := []struct {
		uri               string
		expectedTransport BackendKind
		expectedLocator   string
	}{
		{"hcs://0.0.12345", BackendHCS, "0.0.12345"},
		{"erc-log://0x742d35Cc", BackendERCLog, "0x742d35Cc"},
		{"kafka+ledger://my-topic", BackendKafka, "my-topic"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			ref, err := TopicRefFromURI(tt.uri)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTransport, ref.transport)
			assert.Equal(t, tt.expectedLocator, ref.locator)
		})
	}
}

func TestTopicRefFromURI_UnknownScheme_TreatedAsCustom(t *testing.T) {
	ref, err := TopicRefFromURI("redis://channel-1")
	require.NoError(t, err)
	assert.Equal(t, BackendKind("custom:redis"), ref.transport)
	assert.Equal(t, "channel-1", ref.locator)
}

func TestTopicRefFromURI_RejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"empty", ""},
		{"no separator", "hcs:0.0.12345"},
		{"single slash", "hcs:/0.0.12345"},
		{"empty scheme", "://0.0.12345"},
		{"empty locator", "hcs://"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := TopicRefFromURI(tt.uri)
			require.Error(t, err)

			var topicErr TopicError
			require.True(t, errors.As(err, &topicErr))
			assert.Equal(t, ErrInvalidTopicRef, topicErr.Kind)
		})
	}
}

func TestTopicRef_URI_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		transport BackendKind
		locator   string
	}{
		{"hcs", BackendHCS, "0.0.12345"},
		{"erc-log", BackendERCLog, "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD"},
		{"kafka", BackendKafka, "my-topic-name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original, err := NewTopicRef(tt.transport, tt.locator)
			require.NoError(t, err)

			uri := original.URI()
			roundTripped, err := TopicRefFromURI(uri)
			require.NoError(t, err)

			assert.Equal(t, original.transport, roundTripped.transport)
			assert.Equal(t, original.locator, roundTripped.locator)
			assert.Equal(t, uri, roundTripped.URI())
		})
	}
}

func TestTopicRef_URI_RoundTrip_Custom(t *testing.T) {
	original, err := NewTopicRef(BackendKind("custom:redis"), "channel-1")
	require.NoError(t, err)

	uri := original.URI()
	assert.Equal(t, "redis://channel-1", uri)

	roundTripped, err := TopicRefFromURI(uri)
	require.NoError(t, err)

	assert.Equal(t, original.transport, roundTripped.transport)
	assert.Equal(t, original.locator, roundTripped.locator)
}
