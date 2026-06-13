package topic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirmationMode_Values(t *testing.T) {
	assert.Equal(t, ConfirmationMode("FIRE_AND_FORGET"), FireAndForget)
	assert.Equal(t, ConfirmationMode("WAIT_FOR_CONSENSUS"), WaitForConsensus)
}

func TestPublishResult_FireAndForget(t *testing.T) {
	result := PublishResult{
		TransactionRef: "0.0.12345@1234567890.000000001",
		Confirmed:      false,
	}

	assert.Equal(t, "0.0.12345@1234567890.000000001", result.TransactionRef)
	assert.False(t, result.Confirmed)
	assert.Nil(t, result.ConsensusTimestamp)
	assert.Nil(t, result.SequenceNumber)
}

func TestPublishResult_WaitForConsensus(t *testing.T) {
	ts := uint64(1234567890000000001)
	seq := uint64(42)

	result := PublishResult{
		TransactionRef:     "0.0.12345@1234567890.000000001",
		ConsensusTimestamp: &ts,
		SequenceNumber:     &seq,
		Confirmed:          true,
	}

	assert.Equal(t, "0.0.12345@1234567890.000000001", result.TransactionRef)
	assert.True(t, result.Confirmed)
	require.NotNil(t, result.ConsensusTimestamp)
	assert.Equal(t, ts, *result.ConsensusTimestamp)
	require.NotNil(t, result.SequenceNumber)
	assert.Equal(t, seq, *result.SequenceNumber)
}

func TestPublishResult_JSONSerialization_FireAndForget(t *testing.T) {
	result := PublishResult{
		TransactionRef: "tx-abc-123",
		Confirmed:      false,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var deserialized PublishResult
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	assert.Equal(t, result.TransactionRef, deserialized.TransactionRef)
	assert.Equal(t, result.Confirmed, deserialized.Confirmed)
	assert.Nil(t, deserialized.ConsensusTimestamp)
	assert.Nil(t, deserialized.SequenceNumber)
}

func TestPublishResult_JSONSerialization_WaitForConsensus(t *testing.T) {
	ts := uint64(9999999999)
	seq := uint64(100)

	result := PublishResult{
		TransactionRef:     "tx-xyz-789",
		ConsensusTimestamp: &ts,
		SequenceNumber:     &seq,
		Confirmed:          true,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var deserialized PublishResult
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	assert.Equal(t, result.TransactionRef, deserialized.TransactionRef)
	assert.True(t, deserialized.Confirmed)
	require.NotNil(t, deserialized.ConsensusTimestamp)
	assert.Equal(t, ts, *deserialized.ConsensusTimestamp)
	require.NotNil(t, deserialized.SequenceNumber)
	assert.Equal(t, seq, *deserialized.SequenceNumber)
}

func TestPublishResult_JSON_OmitsNilFields(t *testing.T) {
	result := PublishResult{
		TransactionRef: "tx-123",
		Confirmed:      false,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "consensusTimestamp")
	assert.NotContains(t, jsonStr, "sequenceNumber")
	assert.Contains(t, jsonStr, "transactionRef")
	assert.Contains(t, jsonStr, "confirmed")
}

func TestPublishResult_ZeroValue(t *testing.T) {
	var result PublishResult

	assert.Equal(t, "", result.TransactionRef)
	assert.False(t, result.Confirmed)
	assert.Nil(t, result.ConsensusTimestamp)
	assert.Nil(t, result.SequenceNumber)
}
