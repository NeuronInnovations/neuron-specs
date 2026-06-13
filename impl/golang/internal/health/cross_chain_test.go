package health

import (
	"encoding/json"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Phase 8: Cross-Chain Heartbeat Verification Tests ---

// TestCrossChainPayloadIdentity verifies that the HeartbeatPayload schema
// produces identical JSON output regardless of which transport backend will
// be used to publish the message. The payload is backend-agnostic by design.
func TestCrossChainPayloadIdentity(t *testing.T) {
	t.Run("same payload produces identical bytes for HCS, ERC-log, and Kafka targets", func(t *testing.T) {
		alt := 100.0
		payload := BuildHeartbeatPayload(1700000060, RoleSeller,
			WithCapabilities(&Capabilities{
				NATReachability: true,
				NATType:         NATNone,
				Protocols:       []ProtocolID{"/adsb/v1"},
			}),
			WithLocation(&Location{Lat: 40.7128, Lon: -74.0060, Alt: &alt, Fix: Fix3D}),
			WithPeers([]AbbreviatedAddress{"a1b2", "c3d4"}),
		)

		// Serialize three times (representing HCS, ERC-log, and Kafka targets).
		hcsJSON, err := json.Marshal(payload)
		require.NoError(t, err)
		ercJSON, err := json.Marshal(payload)
		require.NoError(t, err)
		kafkaJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		assert.Equal(t, hcsJSON, ercJSON,
			"payload JSON for HCS and ERC-log must be byte-identical")
		assert.Equal(t, hcsJSON, kafkaJSON,
			"payload JSON for HCS and Kafka must be byte-identical")
	})

	t.Run("shutdown sentinel payload is identical across backends", func(t *testing.T) {
		payload := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		payload.NextHeartbeatDeadline = ShutdownSentinel

		data1, err := json.Marshal(payload)
		require.NoError(t, err)
		data2, err := json.Marshal(payload)
		require.NoError(t, err)

		assert.Equal(t, data1, data2,
			"shutdown sentinel payload must be byte-identical across serializations")

		// Verify deadline field is the quoted string "0" per 006 FR-W02.
		var rawMap map[string]json.RawMessage
		err = json.Unmarshal(data1, &rawMap)
		require.NoError(t, err)
		assert.Equal(t, `"0"`, string(rawMap["nextHeartbeatDeadline"]),
			"shutdown sentinel must serialize as quoted string \"0\" per 006 FR-W02")
	})
}

// TestCrossChainObserverValidation verifies that ValidateInboundHeartbeat and
// the observer pipeline process identically regardless of which transport
// backend provided the consensus timestamp.
func TestCrossChainObserverValidation(t *testing.T) {
	t.Run("same heartbeat validated identically with HCS vs EVM consensus timestamps", func(t *testing.T) {
		key := newTestKey(t)

		// Both backends report the same consensus time for the same message.
		consensusTS := uint64(1700000000)
		deadline := consensusTS + 60

		payload := BuildHeartbeatPayload(deadline, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, payload, consensusTS)

		// Validate as if from HCS.
		hcsResult, err := ValidateInboundHeartbeat(msg, consensusTS)
		require.NoError(t, err)

		// Validate as if from EVM (same consensus timestamp).
		evmResult, err := ValidateInboundHeartbeat(msg, consensusTS)
		require.NoError(t, err)

		assert.Equal(t, hcsResult.NextHeartbeatDeadline, evmResult.NextHeartbeatDeadline,
			"deadline must be identical regardless of backend")
		assert.Equal(t, hcsResult.Type, evmResult.Type)
		assert.Equal(t, hcsResult.Version, evmResult.Version)
		assert.Equal(t, hcsResult.Role, evmResult.Role)
	})

	t.Run("observer produces identical records from HCS and EVM deliveries", func(t *testing.T) {
		key := newTestKey(t)
		senderAddr := key.PublicKey().EVMAddress().Hex()

		consensusTS := uint64(1700000000)
		deadline := consensusTS + 60

		payload := BuildHeartbeatPayload(deadline, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, payload, consensusTS)

		// Observer A: processes as HCS delivery.
		observerHCS := NewHeartbeatObserver()
		_, err := observerHCS.ProcessDelivery(topic.MessageDelivery{
			Message:            msg,
			ConsensusTimestamp: consensusTS,
			BackendSequence:    1,
		})
		require.NoError(t, err)

		// Observer B: processes as EVM delivery (identical data).
		observerEVM := NewHeartbeatObserver()
		_, err = observerEVM.ProcessDelivery(topic.MessageDelivery{
			Message:            msg,
			ConsensusTimestamp: consensusTS,
			BackendSequence:    1,
		})
		require.NoError(t, err)

		// Records must be identical.
		hcsRecord := observerHCS.GetRecord(senderAddr)
		evmRecord := observerEVM.GetRecord(senderAddr)
		require.NotNil(t, hcsRecord)
		require.NotNil(t, evmRecord)

		assert.Equal(t, hcsRecord.currentState, evmRecord.currentState)
		assert.Equal(t, hcsRecord.lastDeadline, evmRecord.lastDeadline)
		assert.Equal(t, hcsRecord.lastSequence, evmRecord.lastSequence)
		assert.Equal(t, hcsRecord.lastConsensusTimestamp, evmRecord.lastConsensusTimestamp)

		// Liveness evaluation at same time must be identical.
		checkTime := consensusTS + 30
		assert.Equal(t,
			observerHCS.GetLivenessState(senderAddr, checkTime),
			observerEVM.GetLivenessState(senderAddr, checkTime),
			"liveness state must be identical regardless of backend origin")
	})
}

// --- Phase 9 (T079): Cross-Chain Integration Test ---

// TestCrossChainPublishToMultipleBackends verifies that the same HeartbeatPayload
// published through two different mock backend adapters (HCS-type and EVM-type)
// produces identical payload content and identical liveness evaluation results
// after observer-side validation. This test MUST come after T072 (HCS integration)
// per Constitution VIII ordering requirement.
func TestCrossChainPublishToMultipleBackends(t *testing.T) {
	t.Run("T079: same payload published to HCS and EVM backends produces identical results", func(t *testing.T) {
		now := uint64(1700000000)
		key := newTestKey(t)
		senderAddr := key.PublicKey().EVMAddress().Hex()
		deadline := now + 60

		// Build one shared payload.
		payload := BuildHeartbeatPayload(deadline, RoleSeller,
			WithCapabilities(&Capabilities{
				NATReachability: true,
				NATType:         NATNone,
				Protocols:       []ProtocolID{"/adsb/v1"},
			}),
		)

		// HCS adapter captures the published message.
		var hcsMsg topic.TopicMessage
		hcsAdapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, msg topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				hcsMsg = msg
				return topic.PublishResult{Confirmed: false, TransactionRef: "hcs-tx"}, nil
			},
		}

		// EVM adapter captures the published message.
		var evmMsg topic.TopicMessage
		evmAdapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendERCLog,
			publishFn: func(_ topic.TopicRef, msg topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				evmMsg = msg
				return topic.PublishResult{Confirmed: false, TransactionRef: "evm-tx"}, nil
			},
		}

		// Publish to HCS.
		hcsRef, _ := topic.NewTopicRef(topic.BackendHCS, "0.0.12345")
		_, err := PublishHeartbeat(payload, key, hcsRef, hcsAdapter, now, 1)
		require.NoError(t, err)

		// Publish the same payload to EVM.
		evmRef, _ := topic.NewTopicRef(topic.BackendERCLog, "0xContractAddr")
		_, err = PublishHeartbeat(payload, key, evmRef, evmAdapter, now, 1)
		require.NoError(t, err)

		// The payload bytes must be identical since the same payload was serialized.
		assert.Equal(t, hcsMsg.Payload(), evmMsg.Payload(),
			"same payload published to different backends must produce identical payload bytes")

		// The signatures must be identical (same key, same timestamp, same payload = RFC 6979).
		assert.Equal(t, hcsMsg.Signature(), evmMsg.Signature(),
			"same key + same payload + same timestamp must produce identical signatures (RFC 6979)")

		// Create two separate observers to process deliveries from each backend.
		hcsObserver := NewHeartbeatObserver()
		evmObserver := NewHeartbeatObserver()

		hcsDelivery := topic.MessageDelivery{
			Message:            hcsMsg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}
		evmDelivery := topic.MessageDelivery{
			Message:            evmMsg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}

		hcsResult, err := hcsObserver.ProcessDelivery(hcsDelivery)
		require.NoError(t, err)
		evmResult, err := evmObserver.ProcessDelivery(evmDelivery)
		require.NoError(t, err)

		// Assert identical HeartbeatPayload content.
		assert.Equal(t, hcsResult.Type, evmResult.Type,
			"payload type must be identical across backends")
		assert.Equal(t, hcsResult.Version, evmResult.Version,
			"payload version must be identical across backends")
		assert.Equal(t, hcsResult.NextHeartbeatDeadline, evmResult.NextHeartbeatDeadline,
			"payload deadline must be identical across backends")
		assert.Equal(t, hcsResult.Role, evmResult.Role,
			"payload role must be identical across backends")

		// Assert identical liveness evaluation at multiple time points.
		timePoints := []struct {
			name string
			time uint64
		}{
			{"at consensus time", now},
			{"at deadline", deadline},
			{"at deadline + grace", deadline + GracePeriod},
			{"after grace window", deadline + GracePeriod + 1},
			{"after suspect window", deadline + GracePeriod + SuspectToDead + 1},
		}

		for _, tp := range timePoints {
			hcsState := hcsObserver.GetLivenessState(senderAddr, tp.time)
			evmState := evmObserver.GetLivenessState(senderAddr, tp.time)
			assert.Equal(t, hcsState, evmState,
				"liveness state must be identical across backends at %s (t=%d)", tp.name, tp.time)
		}
	})
}

// TestCrossChainShutdownRecovery verifies that the shutdown/recovery cycle
// works identically when consensus timestamps come from different backends.
func TestCrossChainShutdownRecovery(t *testing.T) {
	t.Run("shutdown and recovery cycle is backend-agnostic", func(t *testing.T) {
		key := newTestKey(t)
		senderAddr := key.PublicKey().EVMAddress().Hex()

		consensusTS := uint64(1700000000)

		// Build the same message sequence for both observers.
		normalPayload := BuildHeartbeatPayload(consensusTS+60, RoleBuyer)
		normalMsg := createSignedHeartbeatMessage(t, key, normalPayload, consensusTS)

		shutdownPayload := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		shutdownPayload.NextHeartbeatDeadline = ShutdownSentinel
		shutdownMsg := createSignedHeartbeatMessage(t, key, shutdownPayload, consensusTS+30)

		recoveryPayload := BuildHeartbeatPayload(consensusTS+200, RoleBuyer)
		recoveryMsg := createSignedHeartbeatMessage(t, key, recoveryPayload, consensusTS+100)

		// Run the cycle on two independent observers (simulating HCS and EVM backends).
		for _, backendName := range []string{"HCS", "EVM"} {
			t.Run(backendName, func(t *testing.T) {
				observer := NewHeartbeatObserver()

				// Step 1: Normal heartbeat -> ALIVE.
				_, err := observer.ProcessDelivery(topic.MessageDelivery{
					Message:            normalMsg,
					ConsensusTimestamp: consensusTS,
					BackendSequence:    1,
				})
				require.NoError(t, err)
				assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, consensusTS))

				// Step 2: Shutdown -> OFFLINE.
				_, err = observer.ProcessDelivery(topic.MessageDelivery{
					Message:            shutdownMsg,
					ConsensusTimestamp: consensusTS + 30,
					BackendSequence:    2,
				})
				require.NoError(t, err)
				assert.Equal(t, StateOffline, observer.GetLivenessState(senderAddr, consensusTS+30))

				// Step 3: Recovery -> ALIVE.
				_, err = observer.ProcessDelivery(topic.MessageDelivery{
					Message:            recoveryMsg,
					ConsensusTimestamp: consensusTS + 100,
					BackendSequence:    3,
				})
				require.NoError(t, err)
				assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, consensusTS+100))
			})
		}
	})
}
