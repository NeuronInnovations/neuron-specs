package topic

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelRoleFromString_StandardRoles(t *testing.T) {
	tests := []struct {
		input    string
		expected ChannelRole
	}{
		{"stdIn", ChannelStdIn},
		{"stdOut", ChannelStdOut},
		{"stdErr", ChannelStdErr},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			role, err := ChannelRoleFromString(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, role)
		})
	}
}

func TestChannelRoleFromString_CustomRoles(t *testing.T) {
	tests := []struct {
		input    string
		expected ChannelRole
	}{
		{"custom:metrics", ChannelRole("custom:metrics")},
		{"custom:heartbeat", ChannelRole("custom:heartbeat")},
		{"custom:my-channel", ChannelRole("custom:my-channel")},
		{"custom:channel_1", ChannelRole("custom:channel_1")},
		{"custom:A1-b2_c3", ChannelRole("custom:A1-b2_c3")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			role, err := ChannelRoleFromString(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, role)
		})
	}
}

func TestChannelRoleFromString_CustomStdIn_IsValid(t *testing.T) {
	// "custom:stdIn" is a valid custom channel -- it is NOT the reserved stdIn role.
	role, err := ChannelRoleFromString("custom:stdIn")
	require.NoError(t, err)
	assert.Equal(t, ChannelRole("custom:stdIn"), role)
	assert.True(t, IsCustomChannel(role))
}

func TestChannelRoleFromString_RejectsReservedNameCollision(t *testing.T) {
	// Non-standard names without "custom:" prefix are rejected.
	tests := []string{"metrics", "heartbeat", "debug", "foobar"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := ChannelRoleFromString(input)
			require.Error(t, err)

			var topicErr TopicError
			require.True(t, errors.As(err, &topicErr))
			assert.Equal(t, ErrReservedChannelName, topicErr.Kind)
		})
	}
}

func TestChannelRoleFromString_RejectsEmpty(t *testing.T) {
	_, err := ChannelRoleFromString("")
	require.Error(t, err)

	var topicErr TopicError
	require.True(t, errors.As(err, &topicErr))
	assert.Equal(t, ErrReservedChannelName, topicErr.Kind)
}

func TestChannelRoleFromString_RejectsEmptyCustomName(t *testing.T) {
	_, err := ChannelRoleFromString("custom:")
	require.Error(t, err)

	var topicErr TopicError
	require.True(t, errors.As(err, &topicErr))
	assert.Equal(t, ErrReservedChannelName, topicErr.Kind)
}

func TestChannelRoleFromString_RejectsInvalidCustomName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"spaces", "custom:my channel"},
		{"special chars", "custom:my@channel"},
		{"leading hyphen", "custom:-leading"},
		{"leading underscore", "custom:_leading"},
		{"dots", "custom:my.channel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ChannelRoleFromString(tt.input)
			require.Error(t, err)

			var topicErr TopicError
			require.True(t, errors.As(err, &topicErr))
			assert.Equal(t, ErrReservedChannelName, topicErr.Kind)
		})
	}
}

func TestStandardChannelRoles(t *testing.T) {
	roles := StandardChannelRoles()
	require.Len(t, roles, 3)
	assert.Equal(t, ChannelStdIn, roles[0])
	assert.Equal(t, ChannelStdOut, roles[1])
	assert.Equal(t, ChannelStdErr, roles[2])
}

func TestIsCustomChannel(t *testing.T) {
	tests := []struct {
		role     ChannelRole
		expected bool
	}{
		{ChannelStdIn, false},
		{ChannelStdOut, false},
		{ChannelStdErr, false},
		{ChannelRole("custom:metrics"), true},
		{ChannelRole("custom:heartbeat"), true},
		{ChannelRole("custom:stdIn"), true},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			assert.Equal(t, tt.expected, IsCustomChannel(tt.role))
		})
	}
}

func TestChannelRole_String(t *testing.T) {
	assert.Equal(t, "stdIn", ChannelStdIn.String())
	assert.Equal(t, "stdOut", ChannelStdOut.String())
	assert.Equal(t, "stdErr", ChannelStdErr.String())
	assert.Equal(t, "custom:metrics", ChannelRole("custom:metrics").String())
}

// --- Phase 9 (US7): Custom Named Channel Tests ---

func TestCustomChannel_MetricsAndHeartbeat_Valid(t *testing.T) {
	// custom:metrics and custom:heartbeat are valid custom channels.
	tests := []struct {
		input    string
		expected ChannelRole
	}{
		{"custom:metrics", ChannelRole("custom:metrics")},
		{"custom:heartbeat", ChannelRole("custom:heartbeat")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			role, err := ChannelRoleFromString(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, role)
			assert.True(t, IsCustomChannel(role))
		})
	}
}

func TestCustomChannel_StdIn_IsCustomNotReserved(t *testing.T) {
	// "custom:stdIn" is a valid custom channel, not the reserved "stdIn" role.
	role, err := ChannelRoleFromString("custom:stdIn")
	require.NoError(t, err)
	assert.Equal(t, ChannelRole("custom:stdIn"), role)
	assert.True(t, IsCustomChannel(role))

	// The bare "stdIn" is the standard role, not custom.
	standardRole, err := ChannelRoleFromString("stdIn")
	require.NoError(t, err)
	assert.Equal(t, ChannelStdIn, standardRole)
	assert.False(t, IsCustomChannel(standardRole))
}

func TestCustomChannel_BareMetrics_Rejected(t *testing.T) {
	// Bare "metrics" without "custom:" prefix should be rejected.
	_, err := ChannelRoleFromString("metrics")
	require.Error(t, err)

	var topicErr TopicError
	require.True(t, errors.As(err, &topicErr))
	assert.Equal(t, ErrReservedChannelName, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "custom:")
}
