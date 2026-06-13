package topic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockERCLogClient implements ERCLogClient for testing.
type mockERCLogClient struct {
	subscribeFilterLogsFn func(contractAddress string, eventSignature string) (<-chan ERCLogEvent, error)
	filterLogsFn          func(contractAddress string, eventSignature string, fromBlock, toBlock uint64) ([]ERCLogEvent, error)
	getContractInfoFn     func(contractAddress string) (TopicMetadata, error)
}

func (m *mockERCLogClient) SubscribeFilterLogs(contractAddress string, eventSignature string) (<-chan ERCLogEvent, error) {
	if m.subscribeFilterLogsFn != nil {
		return m.subscribeFilterLogsFn(contractAddress, eventSignature)
	}
	ch := make(chan ERCLogEvent)
	close(ch)
	return ch, nil
}

func (m *mockERCLogClient) FilterLogs(contractAddress string, eventSignature string, fromBlock, toBlock uint64) ([]ERCLogEvent, error) {
	if m.filterLogsFn != nil {
		return m.filterLogsFn(contractAddress, eventSignature, fromBlock, toBlock)
	}
	return nil, nil
}

func (m *mockERCLogClient) GetContractInfo(contractAddress string) (TopicMetadata, error) {
	if m.getContractInfoFn != nil {
		return m.getContractInfoFn(contractAddress)
	}
	return TopicMetadata{
		TopicRef:       TopicRef{transport: BackendERCLog, locator: contractAddress},
		SequenceNumber: 100,
		Memo:           "test contract",
	}, nil
}

func TestERCLogAdapter_CreateTopic_Unsupported(t *testing.T) {
	adapter := NewERCLogAdapter(&mockERCLogClient{})

	_, err := adapter.CreateTopic(CreateTopicOpts{Memo: "test"})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrUnsupportedOperation, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "read-only")
}

func TestERCLogAdapter_Publish_Unsupported(t *testing.T) {
	adapter := NewERCLogAdapter(&mockERCLogClient{})

	ref := TopicRef{transport: BackendERCLog, locator: "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD00"}
	_, err := adapter.Publish(ref, TopicMessage{}, PublishOpts{})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrUnsupportedOperation, topicErr.Kind)
}

func TestERCLogAdapter_EstimatePublishCost_Unsupported(t *testing.T) {
	adapter := NewERCLogAdapter(&mockERCLogClient{})

	_, err := adapter.EstimatePublishCost(512)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrUnsupportedOperation, topicErr.Kind)
}

func TestERCLogAdapter_MaxMessageSize(t *testing.T) {
	adapter := NewERCLogAdapter(&mockERCLogClient{})
	assert.Equal(t, uint64(0), adapter.MaxMessageSize())
}

func TestERCLogAdapter_SupportedTransport(t *testing.T) {
	adapter := NewERCLogAdapter(&mockERCLogClient{})
	assert.Equal(t, BackendERCLog, adapter.SupportedTransport())
}

func TestERCLogAdapter_Subscribe_Success(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("erc log event"))
	require.NoError(t, err)

	msgJSON, err := json.Marshal(msg)
	require.NoError(t, err)

	blockTimestamp := uint64(1700000000) // seconds
	blockNumber := uint64(12345)
	logIndex := uint64(3)

	client := &mockERCLogClient{
		subscribeFilterLogsFn: func(contractAddress string, _ string) (<-chan ERCLogEvent, error) {
			assert.Equal(t, "0x742d35Cc", contractAddress)
			ch := make(chan ERCLogEvent, 1)
			ch <- ERCLogEvent{
				Data:           msgJSON,
				BlockTimestamp: blockTimestamp,
				BlockNumber:    blockNumber,
				LogIndex:       logIndex,
			}
			close(ch)
			return ch, nil
		},
	}
	adapter := NewERCLogAdapter(client)

	ref := TopicRef{transport: BackendERCLog, locator: "0x742d35Cc"}
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}

	require.Len(t, deliveries, 1)
	assert.Equal(t, msg.senderAddress, deliveries[0].Message.senderAddress)
	// ConsensusTimestamp = BlockTimestamp * 1_000_000_000 (seconds to nanoseconds)
	assert.Equal(t, blockTimestamp*1_000_000_000, deliveries[0].ConsensusTimestamp)
	// BackendSequence = BlockNumber * 10000 + LogIndex
	assert.Equal(t, blockNumber*10000+logIndex, deliveries[0].BackendSequence)
}

func TestERCLogAdapter_Subscribe_MalformedMessage(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	validMsg, err := NewTopicMessage(&key, 200, 2, []byte("valid"))
	require.NoError(t, err)
	validJSON, err := json.Marshal(validMsg)
	require.NoError(t, err)

	client := &mockERCLogClient{
		subscribeFilterLogsFn: func(_ string, _ string) (<-chan ERCLogEvent, error) {
			ch := make(chan ERCLogEvent, 2)
			ch <- ERCLogEvent{
				Data:           []byte("not valid json"),
				BlockTimestamp: 100,
				BlockNumber:    1,
				LogIndex:       0,
			}
			ch <- ERCLogEvent{
				Data:           validJSON,
				BlockTimestamp: 200,
				BlockNumber:    2,
				LogIndex:       0,
			}
			close(ch)
			return ch, nil
		},
	}
	adapter := NewERCLogAdapter(client)

	ref := TopicRef{transport: BackendERCLog, locator: "0x742d35Cc"}
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}

	// Only the valid message should come through.
	require.Len(t, deliveries, 1)
	assert.Equal(t, uint64(20000), deliveries[0].BackendSequence)
}

func TestERCLogAdapter_Subscribe_Error(t *testing.T) {
	client := &mockERCLogClient{
		subscribeFilterLogsFn: func(_ string, _ string) (<-chan ERCLogEvent, error) {
			return nil, errors.New("RPC connection refused")
		},
	}
	adapter := NewERCLogAdapter(client)

	ref := TopicRef{transport: BackendERCLog, locator: "0x742d35Cc"}
	_, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)
}

func TestERCLogAdapter_Subscribe_InvalidRef(t *testing.T) {
	adapter := NewERCLogAdapter(&mockERCLogClient{})

	// Empty locator should fail validation.
	ref := TopicRef{transport: BackendERCLog, locator: ""}
	_, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.Error(t, err)
}

func TestERCLogAdapter_Resolve(t *testing.T) {
	client := &mockERCLogClient{
		getContractInfoFn: func(addr string) (TopicMetadata, error) {
			return TopicMetadata{
				TopicRef:       TopicRef{transport: BackendERCLog, locator: addr},
				SequenceNumber: 500,
				Memo:           "resolved contract",
			}, nil
		},
	}
	adapter := NewERCLogAdapter(client)

	ref := TopicRef{transport: BackendERCLog, locator: "0x742d35Cc"}
	meta, err := adapter.Resolve(ref)
	require.NoError(t, err)
	assert.Equal(t, uint64(500), meta.SequenceNumber)
	assert.Equal(t, "resolved contract", meta.Memo)
}

func TestERCLogAdapter_Resolve_InvalidRef(t *testing.T) {
	adapter := NewERCLogAdapter(&mockERCLogClient{})

	ref := TopicRef{transport: BackendERCLog, locator: ""}
	_, err := adapter.Resolve(ref)
	require.Error(t, err)
}

func TestERCLogAdapter_NilClient(t *testing.T) {
	adapter := NewERCLogAdapter(nil)

	ref := TopicRef{transport: BackendERCLog, locator: "0x742d35Cc"}

	_, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.Error(t, err)
	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)

	_, err = adapter.Resolve(ref)
	require.Error(t, err)
	topicErr, ok = err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)
}

func TestERCLogAdapter_ImplementsInterface(t *testing.T) {
	var _ TopicAdapter = (*ERCLogAdapter)(nil)
}
