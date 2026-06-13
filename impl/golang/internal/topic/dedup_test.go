package topic

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeduplicationTracker_FirstSeenFalse_SecondSeenTrue(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("hello"))
	require.NoError(t, err)

	tracker := NewDeduplicationTracker()

	// First time: not a duplicate.
	assert.False(t, tracker.IsDuplicate(msg), "first encounter should not be a duplicate")
	assert.Equal(t, 1, tracker.Size())

	// Second time: is a duplicate.
	assert.True(t, tracker.IsDuplicate(msg), "second encounter should be a duplicate")
	assert.Equal(t, 1, tracker.Size(), "size should not increase for duplicates")
}

func TestDeduplicationTracker_DifferentMessages_NotDuplicates(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg1, err := NewTopicMessage(&key, 100, 1, []byte("payload A"))
	require.NoError(t, err)

	msg2, err := NewTopicMessage(&key, 200, 2, []byte("payload B"))
	require.NoError(t, err)

	tracker := NewDeduplicationTracker()

	assert.False(t, tracker.IsDuplicate(msg1), "msg1 first encounter should not be a duplicate")
	assert.False(t, tracker.IsDuplicate(msg2), "msg2 first encounter should not be a duplicate")
	assert.Equal(t, 2, tracker.Size())
}

func TestDeduplicationTracker_Reset_ClearsTracked(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("hello"))
	require.NoError(t, err)

	tracker := NewDeduplicationTracker()

	assert.False(t, tracker.IsDuplicate(msg))
	assert.Equal(t, 1, tracker.Size())

	// Reset clears all tracked messages.
	tracker.Reset()
	assert.Equal(t, 0, tracker.Size())

	// After reset, the same message should not be a duplicate.
	assert.False(t, tracker.IsDuplicate(msg), "after reset, message should not be a duplicate")
	assert.Equal(t, 1, tracker.Size())
}
