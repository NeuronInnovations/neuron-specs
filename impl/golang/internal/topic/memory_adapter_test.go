package topic

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// TestMemoryTopicAdapter_PubSubRoundTrip publishes one message and asserts
// the subscriber receives it intact.
func TestMemoryTopicAdapter_PubSubRoundTrip(t *testing.T) {
	t.Parallel()
	adapter := NewMemoryTopicAdapter()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ref, err := adapter.CreateTopic(CreateTopicOpts{Memo: "round-trip-topic"})
	require.NoError(t, err)
	assert.Equal(t, BackendKindMemory, ref.transport)

	ch, err := adapter.Subscribe(ctx, ref, SubscribeOpts{})
	require.NoError(t, err)

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	msg, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("hello memory"))
	require.NoError(t, err)

	result, err := adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: WaitForConsensus})
	require.NoError(t, err)
	assert.True(t, result.Confirmed)
	require.NotNil(t, result.SequenceNumber)
	assert.Equal(t, uint64(1), *result.SequenceNumber)

	select {
	case got := <-ch:
		assert.Equal(t, msg.payload, got.Message.payload)
		assert.Equal(t, uint64(1), got.BackendSequence)
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive published message within 1s")
	}
}

// TestMemoryTopicAdapter_ConcurrentSubscribers asserts every subscriber
// receives every message published after they subscribe.
func TestMemoryTopicAdapter_ConcurrentSubscribers(t *testing.T) {
	t.Parallel()
	adapter := NewMemoryTopicAdapter()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ref, err := adapter.CreateTopic(CreateTopicOpts{Memo: "fanout-topic"})
	require.NoError(t, err)

	const nSubs = 5
	channels := make([]<-chan MessageDelivery, nSubs)
	for i := 0; i < nSubs; i++ {
		ch, err := adapter.Subscribe(ctx, ref, SubscribeOpts{})
		require.NoError(t, err)
		channels[i] = ch
	}

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	msg, err := NewTopicMessage(&key, 1700000000000000000, 42, []byte("broadcast"))
	require.NoError(t, err)
	_, err = adapter.Publish(ref, msg, PublishOpts{})
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i, ch := range channels {
		wg.Add(1)
		go func(i int, ch <-chan MessageDelivery) {
			defer wg.Done()
			select {
			case got := <-ch:
				assert.Equal(t, msg.payload, got.Message.payload, "subscriber %d", i)
			case <-time.After(time.Second):
				t.Errorf("subscriber %d did not receive", i)
			}
		}(i, ch)
	}
	wg.Wait()
}

// TestMemoryTopicAdapter_SubscribeCancellation cancels the subscription
// context and verifies the delivery channel closes.
func TestMemoryTopicAdapter_SubscribeCancellation(t *testing.T) {
	t.Parallel()
	adapter := NewMemoryTopicAdapter()
	ctx, cancel := context.WithCancel(context.Background())

	ref, err := adapter.CreateTopic(CreateTopicOpts{Memo: "cancel-topic"})
	require.NoError(t, err)

	ch, err := adapter.Subscribe(ctx, ref, SubscribeOpts{})
	require.NoError(t, err)

	cancel()

	select {
	case _, open := <-ch:
		assert.False(t, open, "channel should be closed after ctx cancel")
	case <-time.After(time.Second):
		t.Fatal("subscribe channel did not close within 1s of ctx cancel")
	}
}

// TestMemoryTopicAdapter_PublishUnknownTopic errors out instead of
// silently dropping.
func TestMemoryTopicAdapter_PublishUnknownTopic(t *testing.T) {
	t.Parallel()
	adapter := NewMemoryTopicAdapter()
	ref, err := NewTopicRef(BackendKindMemory, "never-created")
	require.NoError(t, err)

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	msg, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("orphan"))
	require.NoError(t, err)

	_, err = adapter.Publish(ref, msg, PublishOpts{})
	require.Error(t, err)
	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrTopicNotFound, topicErr.Kind)
}

// TestMemoryTopicAdapter_Resolve returns SequenceNumber after publishes.
func TestMemoryTopicAdapter_Resolve(t *testing.T) {
	t.Parallel()
	adapter := NewMemoryTopicAdapter()
	ref, err := adapter.CreateTopic(CreateTopicOpts{Memo: "resolve-topic"})
	require.NoError(t, err)

	meta, err := adapter.Resolve(ref)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), meta.SequenceNumber, "fresh topic has seq=0")

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		msg, err := NewTopicMessage(&key, 1700000000000000000+uint64(i), uint64(i+1), []byte("x"))
		require.NoError(t, err)
		_, err = adapter.Publish(ref, msg, PublishOpts{})
		require.NoError(t, err)
	}
	meta, err = adapter.Resolve(ref)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), meta.SequenceNumber)
}
