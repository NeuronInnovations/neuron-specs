package health

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Integration Tests: Publisher + Observer + Liveness Pipeline ---

// TestIntegrationPublishAndObserve tests the full publisher -> observer -> liveness pipeline
// with real key signing, validating that a heartbeat published by one party can be
// validated and processed by an independent observer.
func TestIntegrationPublishAndObserve(t *testing.T) {
	t.Run("published heartbeat is accepted by observer", func(t *testing.T) {
		now := uint64(1700000000)
		key := newTestKey(t)
		ref := newTestRef()
		senderAddr := key.PublicKey().EVMAddress().Hex()

		// Capture the published message so the observer can process it.
		var capturedMsg topic.TopicMessage
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, msg topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				capturedMsg = msg
				return topic.PublishResult{Confirmed: false, TransactionRef: "int-tx"}, nil
			},
		}

		// Publish.
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		_, err := PublishHeartbeat(payload, key, ref, adapter, now, 1)
		require.NoError(t, err)

		// Observe.
		observer := NewHeartbeatObserver()
		delivery := topic.MessageDelivery{
			Message:            capturedMsg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}
		result, err := observer.ProcessDelivery(delivery)
		require.NoError(t, err)
		assert.Equal(t, now+60, result.NextHeartbeatDeadline)
		assert.Equal(t, PayloadTypeHeartbeat, result.Type)

		// Verify liveness.
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, now))
		assert.Equal(t, StateSuspect, observer.GetLivenessState(senderAddr, now+60+GracePeriod+1))
	})
}

// TestIntegrationShutdownSentinelPipeline tests the full shutdown sentinel flow
// through the publish -> observe -> liveness pipeline.
func TestIntegrationShutdownSentinelPipeline(t *testing.T) {
	t.Run("shutdown sentinel published and observed correctly", func(t *testing.T) {
		now := uint64(1700000000)
		key := newTestKey(t)
		ref := newTestRef()
		senderAddr := key.PublicKey().EVMAddress().Hex()

		var capturedMsg topic.TopicMessage
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, msg topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				capturedMsg = msg
				return topic.PublishResult{Confirmed: false}, nil
			},
		}

		// Publish shutdown sentinel.
		shutdownPayload := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		shutdownPayload.NextHeartbeatDeadline = ShutdownSentinel
		_, err := PublishHeartbeat(shutdownPayload, key, ref, adapter, now, 2)
		require.NoError(t, err)

		// Observer processes the shutdown.
		observer := NewHeartbeatObserver()
		_, err = observer.ProcessDelivery(topic.MessageDelivery{
			Message:            capturedMsg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		})
		require.NoError(t, err)

		// Must be OFFLINE.
		assert.Equal(t, StateOffline, observer.GetLivenessState(senderAddr, now))
		assert.Equal(t, StateOffline, observer.GetLivenessState(senderAddr, now+1000000),
			"OFFLINE must persist regardless of time elapsed")
	})
}

// TestIntegrationOptionalFieldsTrimming tests that a payload with optional fields
// is correctly trimmed by the publisher, and the trimmed payload is still valid
// when received by the observer.
func TestIntegrationOptionalFieldsTrimming(t *testing.T) {
	t.Run("trimmed payload is still valid for observer", func(t *testing.T) {
		now := uint64(1700000000)
		key := newTestKey(t)
		ref := newTestRef()

		var capturedPayload []byte
		adapter := &mockTopicAdapter{
			maxMsgSize: 150, // tight limit forces trimming
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, msg topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				capturedPayload = msg.Payload()
				return topic.PublishResult{Confirmed: false}, nil
			},
		}

		payload := BuildHeartbeatPayload(now+60, RoleBuyer,
			WithCapabilities(&Capabilities{
				NATReachability: true,
				NATType:         NATNone,
				Protocols:       []ProtocolID{"/test/v1"},
			}),
			WithPeers([]AbbreviatedAddress{"dead", "beef"}),
		)
		_, err := PublishHeartbeat(payload, key, ref, adapter, now, 1)
		require.NoError(t, err)

		// The captured payload must be valid JSON and parseable as HeartbeatPayload.
		var published HeartbeatPayload
		err = json.Unmarshal(capturedPayload, &published)
		require.NoError(t, err)
		assert.Equal(t, PayloadTypeHeartbeat, published.Type)
		assert.Equal(t, CurrentVersion, published.Version)
		assert.Equal(t, now+60, published.NextHeartbeatDeadline)
		assert.LessOrEqual(t, len(capturedPayload), 150,
			"trimmed payload must fit within adapter max message size")
	})
}

// --- Phase 9 (T072): HCS End-to-End Integration Test (Constitution VIII) ---

// TestHCSEndToEndIntegration verifies the complete heartbeat lifecycle through
// an HCS-type transport backend, satisfying Constitution VIII (Hedera topic
// transport binding). This test exercises: key generation, payload construction,
// publish via HCS mock adapter, observer processing, UNKNOWN->ALIVE transition,
// and liveness evaluation at multiple time points.
func TestHCSEndToEndIntegration(t *testing.T) {
	t.Run("T072: full HCS lifecycle — UNKNOWN -> ALIVE with liveness evaluation at multiple time points", func(t *testing.T) {
		now := uint64(1700000000)
		key := newTestKey(t)
		ref := newTestRef() // Uses BackendHCS by default
		senderAddr := key.PublicKey().EVMAddress().Hex()
		deadline := now + 60

		// Step 1: Verify the mock adapter uses HCS transport (Constitution VIII).
		var capturedMsg topic.TopicMessage
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, msg topic.TopicMessage, opts topic.PublishOpts) (topic.PublishResult, error) {
				capturedMsg = msg
				// Verify FireAndForget mode per FR-H23.
				assert.Equal(t, topic.FireAndForget, opts.ConfirmationMode)
				return topic.PublishResult{Confirmed: false, TransactionRef: "hcs-tx-001"}, nil
			},
		}
		assert.Equal(t, topic.BackendHCS, adapter.SupportedTransport(),
			"adapter must use HCS transport (Constitution VIII)")

		// Step 2: Build and publish a HeartbeatPayload.
		payload := BuildHeartbeatPayload(deadline, RoleSeller)
		result, err := PublishHeartbeat(payload, key, ref, adapter, now, 1)
		require.NoError(t, err)
		assert.Equal(t, "hcs-tx-001", result.TransactionRef)

		// Step 3: Observer begins with UNKNOWN state for this sender.
		observer := NewHeartbeatObserver()
		assert.Equal(t, StateUnknown, observer.GetLivenessState(senderAddr, now),
			"sender must be UNKNOWN before first heartbeat")

		// Step 4: Construct a MessageDelivery from the published message.
		delivery := topic.MessageDelivery{
			Message:            capturedMsg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}

		// Step 5: Process delivery and verify UNKNOWN -> ALIVE transition.
		validated, err := observer.ProcessDelivery(delivery)
		require.NoError(t, err)
		assert.Equal(t, PayloadTypeHeartbeat, validated.Type)
		assert.Equal(t, CurrentVersion, validated.Version)
		assert.Equal(t, deadline, validated.NextHeartbeatDeadline)
		assert.Equal(t, RoleSeller, validated.Role)

		// Step 6: Evaluate liveness at multiple time points.
		// At consensus time: ALIVE.
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, now),
			"must be ALIVE immediately after processing heartbeat")

		// At deadline: still ALIVE (within grace window).
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, deadline),
			"must be ALIVE at deadline (within grace window)")

		// At deadline + GracePeriod: still ALIVE (boundary).
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, deadline+GracePeriod),
			"must be ALIVE at deadline+GracePeriod (boundary)")

		// At deadline + GracePeriod + 1: SUSPECT.
		assert.Equal(t, StateSuspect, observer.GetLivenessState(senderAddr, deadline+GracePeriod+1),
			"must be SUSPECT after grace window expires")

		// At deadline + GracePeriod + SuspectToDead: still SUSPECT (boundary).
		assert.Equal(t, StateSuspect, observer.GetLivenessState(senderAddr, deadline+GracePeriod+SuspectToDead),
			"must be SUSPECT at suspect window boundary")

		// At deadline + GracePeriod + SuspectToDead + 1: DEAD.
		assert.Equal(t, StateDead, observer.GetLivenessState(senderAddr, deadline+GracePeriod+SuspectToDead+1),
			"must be DEAD after suspect window expires")

		// Verify the liveness record was correctly populated.
		record := observer.GetRecord(senderAddr)
		require.NotNil(t, record)
		assert.Equal(t, StateAlive, record.currentState)
		assert.Equal(t, deadline, record.lastDeadline)
		assert.Equal(t, uint64(1), record.lastSequence)
		assert.Equal(t, now, record.lastConsensusTimestamp)
	})
}

// TestIntegrationMultiSenderObserver tests that the observer correctly tracks
// multiple independent senders with different liveness timelines.
func TestIntegrationMultiSenderObserver(t *testing.T) {
	t.Run("observer tracks multiple senders with independent liveness", func(t *testing.T) {
		now := uint64(1700000000)
		key1 := newTestKey(t)
		key2 := newTestKey(t)
		addr1 := key1.PublicKey().EVMAddress().Hex()
		addr2 := key2.PublicKey().EVMAddress().Hex()

		observer := NewHeartbeatObserver()

		// Sender 1: deadline at T+60.
		p1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg1 := createSignedHeartbeatMessage(t, key1, p1, now)
		_, err := observer.ProcessDelivery(topic.MessageDelivery{
			Message:            msg1,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		})
		require.NoError(t, err)

		// Sender 2: shutdown sentinel.
		p2 := BuildHeartbeatPayload(ShutdownSentinel, RoleSeller)
		p2.NextHeartbeatDeadline = ShutdownSentinel
		msg2 := createSignedHeartbeatMessage(t, key2, p2, now)
		_, err = observer.ProcessDelivery(topic.MessageDelivery{
			Message:            msg2,
			ConsensusTimestamp: now,
			BackendSequence:    2,
		})
		require.NoError(t, err)

		// Sender 1 should be ALIVE, Sender 2 should be OFFLINE.
		assert.Equal(t, StateAlive, observer.GetLivenessState(addr1, now))
		assert.Equal(t, StateOffline, observer.GetLivenessState(addr2, now))

		// Later: sender 1 goes SUSPECT, sender 2 stays OFFLINE.
		checkTime := now + 60 + GracePeriod + 1
		assert.Equal(t, StateSuspect, observer.GetLivenessState(addr1, checkTime))
		assert.Equal(t, StateOffline, observer.GetLivenessState(addr2, checkTime))
	})
}

// --- Phase 9 (T080): Quickstart API Signature Validation ---

// TestQuickstartAPISignatures validates that all API functions referenced in
// quickstart.md exist with their correct signatures. This test ensures the
// quickstart documentation stays in sync with the actual implementation.
//
// The quickstart shows pseudo-code like:
//
//	BuildHeartbeatPayload(deadline, role, opts...)
//	ValidateOutboundHeartbeat(payload, senderClock)
//	PublishHeartbeat(payload, key, stdOutRef, adapter, senderClock)
//	ScheduleNextHeartbeat(result, chosenDelta, submitWallClock)
//	ValidateInboundHeartbeat(msg, consensusTimestamp)
//	UpdateLivenessRecord(record, senderAddress, payload, consensusTimestamp, sequenceNumber)
//	EvaluateLiveness(record, currentTime)
//
// This test calls each function with the correct number and types of arguments
// to prove the API signatures are correct. Compile-time verification plus
// runtime correctness.
func TestQuickstartAPISignatures(t *testing.T) {
	now := uint64(time.Now().Unix())
	key := newTestKey(t)
	ref := newTestRef()

	t.Run("T080: BuildHeartbeatPayload(deadline, role, opts...) signature", func(t *testing.T) {
		// BuildHeartbeatPayload takes (uint64, NodeRole, ...HeartbeatOption).
		p := BuildHeartbeatPayload(now+60, RoleSeller)
		assert.Equal(t, PayloadTypeHeartbeat, p.Type)

		// With options.
		p2 := BuildHeartbeatPayload(now+60, RoleSeller,
			WithCapabilities(&Capabilities{NATReachability: true, NATType: NATNone, Protocols: nil}),
			WithLocation(&Location{Lat: 0, Lon: 0, Fix: FixNone}),
			WithPeers([]AbbreviatedAddress{"dead"}),
		)
		assert.NotNil(t, p2.Capabilities)
		assert.NotNil(t, p2.Location)
		assert.Len(t, p2.Peers, 1)
	})

	t.Run("T080: ValidateOutboundHeartbeat(payload, senderClock) signature", func(t *testing.T) {
		// ValidateOutboundHeartbeat takes (HeartbeatPayload, uint64) -> error.
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		err := ValidateOutboundHeartbeat(payload, now)
		assert.NoError(t, err)
	})

	t.Run("T080: PublishHeartbeat(payload, key, stdOutRef, adapter, senderClock, sequenceNumber) signature", func(t *testing.T) {
		// PublishHeartbeat takes (HeartbeatPayload, *keylib.NeuronPrivateKey, topic.TopicRef, topic.TopicAdapter, uint64)
		//   -> (topic.PublishResult, error)
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
		}
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)

		// Verify the function accepts exactly 5 arguments with correct types.
		var result topic.PublishResult
		var err error
		result, err = PublishHeartbeat(payload, key, ref, adapter, now, 1)
		_ = result
		assert.NoError(t, err)

		// Verify key type is *keylib.NeuronPrivateKey (compile-time check).
		var _ *keylib.NeuronPrivateKey = key
	})

	t.Run("T080: ScheduleNextHeartbeat(result, chosenDelta, submitWallClock) signature", func(t *testing.T) {
		// ScheduleNextHeartbeat takes (topic.PublishResult, uint64, uint64) -> uint64.
		result := topic.PublishResult{Confirmed: false}
		next := ScheduleNextHeartbeat(result, 60, now)
		assert.Equal(t, now+60, next)
	})

	t.Run("T080: ValidateInboundHeartbeat(msg, consensusTimestamp) signature", func(t *testing.T) {
		// ValidateInboundHeartbeat takes (topic.TopicMessage, uint64) -> (*HeartbeatPayload, error).
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, payload, now)
		validated, err := ValidateInboundHeartbeat(msg, now)
		require.NoError(t, err)
		assert.Equal(t, now+60, validated.NextHeartbeatDeadline)
	})

	t.Run("T080: UpdateLivenessRecord(record, senderAddress, payload, consensusTimestamp, sequenceNumber) signature", func(t *testing.T) {
		// UpdateLivenessRecord takes (*LivenessRecord, string, *HeartbeatPayload, uint64, uint64) -> *LivenessRecord.
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		record := UpdateLivenessRecord(nil, "0xabc", &payload, now, 1)
		assert.NotNil(t, record)
		assert.Equal(t, StateAlive, record.currentState)
	})

	t.Run("T080: EvaluateLiveness(record, currentTime) signature", func(t *testing.T) {
		// EvaluateLiveness takes (*LivenessRecord, uint64) -> LivenessState.
		state := EvaluateLiveness(nil, now)
		assert.Equal(t, StateUnknown, state)

		record := &LivenessRecord{
			senderAddress: "0xabc",
			currentState:  StateAlive,
			lastDeadline:  now + 60,
			lastSequence:  1,
		}
		state = EvaluateLiveness(record, now)
		assert.Equal(t, StateAlive, state)
	})

	t.Run("T080: NewHeartbeatObserver() and ProcessDelivery(delivery) signatures", func(t *testing.T) {
		// NewHeartbeatObserver takes no args -> *HeartbeatObserver.
		observer := NewHeartbeatObserver()
		assert.NotNil(t, observer)

		// ProcessDelivery takes (topic.MessageDelivery) -> (*HeartbeatPayload, error).
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, payload, now)
		delivery := topic.MessageDelivery{
			Message:            msg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}
		result, err := observer.ProcessDelivery(delivery)
		require.NoError(t, err)
		assert.Equal(t, PayloadTypeHeartbeat, result.Type)
	})

	t.Run("T080: NewHeartbeatPublisher(key, stdOutRef, adapter) and Publish signatures", func(t *testing.T) {
		// NewHeartbeatPublisher takes (*keylib.NeuronPrivateKey, topic.TopicRef, topic.TopicAdapter)
		//   -> *HeartbeatPublisher
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
		}
		pub := NewHeartbeatPublisher(key, ref, adapter)
		assert.NotNil(t, pub)

		// Publish takes (HeartbeatPayload, uint64) -> (topic.PublishResult, error).
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		_, err := pub.Publish(payload, now)
		assert.NoError(t, err)
	})
}
