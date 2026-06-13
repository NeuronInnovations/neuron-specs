package health

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers for observer tests ---

// createSignedHeartbeatMessage builds a signed TopicMessage containing a heartbeat payload.
func createSignedHeartbeatMessage(t *testing.T, key *keylib.NeuronPrivateKey, payload HeartbeatPayload, timestamp uint64) topic.TopicMessage {
	t.Helper()
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	msg, err := topic.NewTopicMessage(key, timestamp, 0, payloadJSON)
	require.NoError(t, err)
	return msg
}

// --- ValidateInboundHeartbeat Tests ---

func TestValidateInboundHeartbeat(t *testing.T) {
	now := uint64(1700000000)

	t.Run("V-OBS-01a: invalid signature is rejected", func(t *testing.T) {
		key := newTestKey(t)
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, payload, now)

		// Replace signature with an invalid-length byte slice so recovery fails entirely.
		msg = topic.TopicMessageFromFields(msg.SenderAddress(), []byte{0x01, 0x02, 0x03}, msg.Timestamp(), msg.SequenceNumber(), msg.Payload())

		_, err := ValidateInboundHeartbeat(msg, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrSignatureVerificationFailed, he.Kind)
	})

	t.Run("V-OBS-01a: corrupted signature is rejected", func(t *testing.T) {
		key := newTestKey(t)
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, payload, now)

		// Corrupt a signature byte. Depending on which byte is altered, the recovery
		// may fail entirely (SignatureVerificationFailed) or recover a different key
		// (SenderAddressMismatch). Both are valid V-OBS-01 rejections.
		tamperedSig := msg.Signature()
		tamperedSig[10] ^= 0xFF
		msg = topic.TopicMessageFromFields(msg.SenderAddress(), tamperedSig, msg.Timestamp(), msg.SequenceNumber(), msg.Payload())

		_, err := ValidateInboundHeartbeat(msg, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.True(t, he.Kind == ErrSignatureVerificationFailed || he.Kind == ErrSenderAddressMismatch,
			"corrupted signature must produce SignatureVerificationFailed or SenderAddressMismatch, got %s", he.Kind)
	})

	t.Run("V-OBS-01b: parent-signed heartbeat with child address is rejected (sender mismatch)", func(t *testing.T) {
		parentKey := newTestKey(t)
		childKey := newTestKey(t)

		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		payloadJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		// Sign with parent key.
		msg, err := topic.NewTopicMessage(parentKey, now, 0, payloadJSON)
		require.NoError(t, err)

		// Replace senderAddress with child's address.
		msg = topic.TopicMessageFromFields(childKey.PublicKey().EVMAddress().Hex(), msg.Signature(), msg.Timestamp(), msg.SequenceNumber(), msg.Payload())

		_, err = ValidateInboundHeartbeat(msg, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrSenderAddressMismatch, he.Kind)
	})

	t.Run("V-OBS-02: non-heartbeat payload type is rejected", func(t *testing.T) {
		key := newTestKey(t)

		// Build a payload with wrong type.
		nonHB := HeartbeatPayload{
			Type:                  "status-update",
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now + 60,
			Role:                  RoleBuyer,
		}
		msg := createSignedHeartbeatMessage(t, key, nonHB, now)

		_, err := ValidateInboundHeartbeat(msg, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrNotHeartbeatMessage, he.Kind)
	})

	t.Run("V-OBS-03: incompatible version (major != 1) is rejected", func(t *testing.T) {
		key := newTestKey(t)

		tests := []struct {
			name    string
			version string
		}{
			{"major 2", "2.0.0"},
			{"major 0", "0.9.0"},
			{"major 3", "3.1.0"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p := HeartbeatPayload{
					Type:                  PayloadTypeHeartbeat,
					Version:               tt.version,
					NextHeartbeatDeadline: now + 60,
					Role:                  RoleBuyer,
				}
				msg := createSignedHeartbeatMessage(t, key, p, now)

				_, err := ValidateInboundHeartbeat(msg, now)
				require.Error(t, err)
				var he HealthError
				require.True(t, errors.As(err, &he))
				assert.Equal(t, ErrIncompatibleVersion, he.Kind)
			})
		}
	})

	t.Run("V-OBS-03: compatible versions (major == 1) are accepted", func(t *testing.T) {
		key := newTestKey(t)

		versions := []string{"1.0.0", "1.1.0", "1.2.3", "1.99.0"}
		for _, v := range versions {
			t.Run(v, func(t *testing.T) {
				p := HeartbeatPayload{
					Type:                  PayloadTypeHeartbeat,
					Version:               v,
					NextHeartbeatDeadline: now + 60,
					Role:                  RoleBuyer,
				}
				msg := createSignedHeartbeatMessage(t, key, p, now)

				payload, err := ValidateInboundHeartbeat(msg, now)
				require.NoError(t, err)
				assert.Equal(t, v, payload.Version)
			})
		}
	})

	t.Run("V-OBS-04: shutdown sentinel bypasses delta checks", func(t *testing.T) {
		key := newTestKey(t)

		p := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		// Manually set deadline to 0 since BuildHeartbeatPayload sets it directly.
		p.NextHeartbeatDeadline = ShutdownSentinel
		msg := createSignedHeartbeatMessage(t, key, p, now)

		payload, err := ValidateInboundHeartbeat(msg, now)
		require.NoError(t, err)
		assert.Equal(t, ShutdownSentinel, payload.NextHeartbeatDeadline)
	})

	t.Run("V-OBS-05: deadline in past is rejected", func(t *testing.T) {
		key := newTestKey(t)

		// Deadline equal to consensus timestamp.
		p := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now,
			Role:                  RoleBuyer,
		}
		msg := createSignedHeartbeatMessage(t, key, p, now)

		_, err := ValidateInboundHeartbeat(msg, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeadlineInPast, he.Kind)

		// Deadline before consensus timestamp.
		p2 := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now - 1,
			Role:                  RoleBuyer,
		}
		msg2 := createSignedHeartbeatMessage(t, key, p2, now)

		_, err2 := ValidateInboundHeartbeat(msg2, now)
		require.Error(t, err2)
		require.True(t, errors.As(err2, &he))
		assert.Equal(t, ErrDeadlineInPast, he.Kind)
	})

	t.Run("V-OBS-06: delta below minimum is rejected", func(t *testing.T) {
		key := newTestKey(t)

		// Delta = 5, MinDeadlineDelta = 10.
		p := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now + 5,
			Role:                  RoleBuyer,
		}
		msg := createSignedHeartbeatMessage(t, key, p, now)

		_, err := ValidateInboundHeartbeat(msg, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeltaBelowMinimum, he.Kind)
	})

	t.Run("V-OBS-06: delta above maximum is rejected", func(t *testing.T) {
		key := newTestKey(t)

		p := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now + MaxDeadlineDelta + 1,
			Role:                  RoleBuyer,
		}
		msg := createSignedHeartbeatMessage(t, key, p, now)

		_, err := ValidateInboundHeartbeat(msg, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeltaExceedsMaximum, he.Kind)
	})

	t.Run("accept valid heartbeat with minimum delta", func(t *testing.T) {
		key := newTestKey(t)

		p := BuildHeartbeatPayload(now+MinDeadlineDelta, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, p, now)

		payload, err := ValidateInboundHeartbeat(msg, now)
		require.NoError(t, err)
		assert.Equal(t, now+MinDeadlineDelta, payload.NextHeartbeatDeadline)
	})

	t.Run("accept valid heartbeat with maximum delta", func(t *testing.T) {
		key := newTestKey(t)

		p := BuildHeartbeatPayload(now+MaxDeadlineDelta, RoleSeller)
		msg := createSignedHeartbeatMessage(t, key, p, now)

		payload, err := ValidateInboundHeartbeat(msg, now)
		require.NoError(t, err)
		assert.Equal(t, now+MaxDeadlineDelta, payload.NextHeartbeatDeadline)
	})

	t.Run("consensus timestamp authority: all delta checks use consensusTimestamp", func(t *testing.T) {
		key := newTestKey(t)

		// Deadline is in the future relative to consensus but not relative to some
		// hypothetical "local clock". Only consensus matters.
		consensusTS := uint64(1700000000)
		deadline := consensusTS + 60

		p := BuildHeartbeatPayload(deadline, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, p, now)

		payload, err := ValidateInboundHeartbeat(msg, consensusTS)
		require.NoError(t, err)
		assert.Equal(t, deadline, payload.NextHeartbeatDeadline)
	})
}

// --- UpdateLivenessRecord Tests ---

func TestUpdateLivenessRecord(t *testing.T) {
	addr := "0x1234567890abcdef1234567890abcdef12345678"

	t.Run("nil record creates new record with ALIVE state", func(t *testing.T) {
		payload := BuildHeartbeatPayload(1700000060, RoleBuyer)
		record := UpdateLivenessRecord(nil, addr, &payload, 1700000000, 1)
		require.NotNil(t, record)
		assert.Equal(t, addr, record.senderAddress)
		assert.Equal(t, StateAlive, record.currentState)
		assert.Equal(t, uint64(1700000060), record.lastDeadline)
		assert.Equal(t, uint64(1), record.lastSequence)
		assert.Equal(t, uint64(1700000000), record.lastConsensusTimestamp)
	})

	t.Run("nil record with shutdown sentinel creates OFFLINE record", func(t *testing.T) {
		payload := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		payload.NextHeartbeatDeadline = ShutdownSentinel
		record := UpdateLivenessRecord(nil, addr, &payload, 1700000000, 1)
		require.NotNil(t, record)
		assert.Equal(t, StateOffline, record.currentState)
		assert.Equal(t, ShutdownSentinel, record.lastDeadline)
	})

	t.Run("higher sequence number updates record", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress:         addr,
			currentState:          StateAlive,
			lastDeadline:          1700000060,
			lastSequence:          5,
			lastConsensusTimestamp: 1700000000,
		}

		payload := BuildHeartbeatPayload(1700000120, RoleBuyer)
		record := UpdateLivenessRecord(existing, addr, &payload, 1700000060, 6)
		assert.Equal(t, StateAlive, record.currentState)
		assert.Equal(t, uint64(1700000120), record.lastDeadline)
		assert.Equal(t, uint64(6), record.lastSequence)
		assert.Equal(t, uint64(1700000060), record.lastConsensusTimestamp)
	})

	t.Run("lower sequence number is ignored (FR-H17)", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress:         addr,
			currentState:          StateAlive,
			lastDeadline:          1700000060,
			lastSequence:          10,
			lastConsensusTimestamp: 1700000000,
		}

		payload := BuildHeartbeatPayload(1700000120, RoleBuyer)
		record := UpdateLivenessRecord(existing, addr, &payload, 1700000060, 5)
		// Record should be unchanged.
		assert.Equal(t, StateAlive, record.currentState)
		assert.Equal(t, uint64(1700000060), record.lastDeadline)
		assert.Equal(t, uint64(10), record.lastSequence)
		assert.Equal(t, uint64(1700000000), record.lastConsensusTimestamp)
	})

	t.Run("equal sequence number is ignored (FR-H17)", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress:         addr,
			currentState:          StateAlive,
			lastDeadline:          1700000060,
			lastSequence:          10,
			lastConsensusTimestamp: 1700000000,
		}

		payload := BuildHeartbeatPayload(1700000120, RoleBuyer)
		record := UpdateLivenessRecord(existing, addr, &payload, 1700000060, 10)
		// Record should be unchanged.
		assert.Equal(t, uint64(1700000060), record.lastDeadline)
		assert.Equal(t, uint64(10), record.lastSequence)
	})

	t.Run("shutdown sentinel transitions to OFFLINE", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress:         addr,
			currentState:          StateAlive,
			lastDeadline:          1700000060,
			lastSequence:          5,
			lastConsensusTimestamp: 1700000000,
		}

		payload := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: ShutdownSentinel,
			Role:                  RoleBuyer,
		}
		record := UpdateLivenessRecord(existing, addr, &payload, 1700000060, 6)
		assert.Equal(t, StateOffline, record.currentState)
		assert.Equal(t, ShutdownSentinel, record.lastDeadline)
	})

	t.Run("normal heartbeat transitions to ALIVE from any state", func(t *testing.T) {
		states := []LivenessState{StateUnknown, StateAlive, StateSuspect, StateDead, StateOffline}
		for _, state := range states {
			t.Run("from_"+state.String(), func(t *testing.T) {
				existing := &LivenessRecord{
					senderAddress:         addr,
					currentState:          state,
					lastDeadline:          1700000000,
					lastSequence:          5,
					lastConsensusTimestamp: 1700000000,
				}

				payload := BuildHeartbeatPayload(1700000120, RoleBuyer)
				record := UpdateLivenessRecord(existing, addr, &payload, 1700000060, 6)
				assert.Equal(t, StateAlive, record.currentState,
					"any valid heartbeat from %s should transition to ALIVE", state)
			})
		}
	})
}

// --- HeartbeatObserver Tests ---

func TestHeartbeatObserver(t *testing.T) {
	now := uint64(1700000000)

	t.Run("ProcessDelivery: happy path creates record", func(t *testing.T) {
		key := newTestKey(t)
		observer := NewHeartbeatObserver()
		senderAddr := key.PublicKey().EVMAddress().Hex()

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
		assert.Equal(t, now+60, result.NextHeartbeatDeadline)

		// Verify the record was created.
		record := observer.GetRecord(senderAddr)
		require.NotNil(t, record)
		assert.Equal(t, StateAlive, record.currentState)
		assert.Equal(t, uint64(now+60), record.lastDeadline)
	})

	t.Run("ProcessDelivery: multiple heartbeats update record", func(t *testing.T) {
		key := newTestKey(t)
		observer := NewHeartbeatObserver()
		senderAddr := key.PublicKey().EVMAddress().Hex()

		// First heartbeat.
		p1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg1 := createSignedHeartbeatMessage(t, key, p1, now)
		delivery1 := topic.MessageDelivery{
			Message:            msg1,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}
		_, err := observer.ProcessDelivery(delivery1)
		require.NoError(t, err)

		// Second heartbeat with higher sequence.
		p2 := BuildHeartbeatPayload(now+120, RoleBuyer)
		msg2 := createSignedHeartbeatMessage(t, key, p2, now+60)
		delivery2 := topic.MessageDelivery{
			Message:            msg2,
			ConsensusTimestamp: now + 60,
			BackendSequence:    2,
		}
		_, err = observer.ProcessDelivery(delivery2)
		require.NoError(t, err)

		record := observer.GetRecord(senderAddr)
		require.NotNil(t, record)
		assert.Equal(t, uint64(now+120), record.lastDeadline)
		assert.Equal(t, uint64(2), record.lastSequence)
	})

	t.Run("ProcessDelivery: invalid message returns error", func(t *testing.T) {
		observer := NewHeartbeatObserver()

		// Message with invalid signature.
		msg := topic.TopicMessageFromFields(
			"0xdeadbeef",
			[]byte{0x01, 0x02, 0x03},
			now,
			1,
			[]byte(`{"type":"heartbeat","version":"1.0.0"}`),
		)

		delivery := topic.MessageDelivery{
			Message:            msg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}

		_, err := observer.ProcessDelivery(delivery)
		require.Error(t, err)
	})

	t.Run("GetLivenessState: returns UNKNOWN for unknown sender", func(t *testing.T) {
		observer := NewHeartbeatObserver()
		state := observer.GetLivenessState("0xunknown", now)
		assert.Equal(t, StateUnknown, state)
	})

	t.Run("GetLivenessState: returns correct state based on time", func(t *testing.T) {
		key := newTestKey(t)
		observer := NewHeartbeatObserver()
		senderAddr := key.PublicKey().EVMAddress().Hex()

		deadline := now + 60
		payload := BuildHeartbeatPayload(deadline, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, payload, now)

		delivery := topic.MessageDelivery{
			Message:            msg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}

		_, err := observer.ProcessDelivery(delivery)
		require.NoError(t, err)

		// Within grace window -> ALIVE.
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, deadline))
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, deadline+GracePeriod))

		// Past grace -> SUSPECT.
		assert.Equal(t, StateSuspect, observer.GetLivenessState(senderAddr, deadline+GracePeriod+1))

		// Past suspect -> DEAD.
		assert.Equal(t, StateDead, observer.GetLivenessState(senderAddr, deadline+GracePeriod+SuspectToDead+1))
	})

	t.Run("GetRecord: returns nil for unknown sender", func(t *testing.T) {
		observer := NewHeartbeatObserver()
		record := observer.GetRecord("0xunknown")
		assert.Nil(t, record)
	})

	t.Run("tracks multiple senders independently", func(t *testing.T) {
		key1 := newTestKey(t)
		key2 := newTestKey(t)
		observer := NewHeartbeatObserver()
		addr1 := key1.PublicKey().EVMAddress().Hex()
		addr2 := key2.PublicKey().EVMAddress().Hex()

		// Sender 1: heartbeat with deadline T+60.
		p1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg1 := createSignedHeartbeatMessage(t, key1, p1, now)
		_, err := observer.ProcessDelivery(topic.MessageDelivery{
			Message:            msg1,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		})
		require.NoError(t, err)

		// Sender 2: heartbeat with deadline T+120.
		p2 := BuildHeartbeatPayload(now+120, RoleSeller)
		msg2 := createSignedHeartbeatMessage(t, key2, p2, now)
		_, err = observer.ProcessDelivery(topic.MessageDelivery{
			Message:            msg2,
			ConsensusTimestamp: now,
			BackendSequence:    2,
		})
		require.NoError(t, err)

		// At time T+60+GP+1, sender 1 is SUSPECT, sender 2 is still ALIVE.
		checkTime := now + 60 + GracePeriod + 1
		assert.Equal(t, StateSuspect, observer.GetLivenessState(addr1, checkTime))
		assert.Equal(t, StateAlive, observer.GetLivenessState(addr2, checkTime))
	})

	t.Run("subscribe filtering: non-heartbeat messages are rejected", func(t *testing.T) {
		key := newTestKey(t)
		observer := NewHeartbeatObserver()

		// Build a non-heartbeat payload.
		nonHBPayload := `{"type":"status-update","version":"1.0.0","nextHeartbeatDeadline":1700000060,"role":"buyer"}`
		msg, err := topic.NewTopicMessage(key, now, 0, []byte(nonHBPayload))
		require.NoError(t, err)

		delivery := topic.MessageDelivery{
			Message:            msg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}

		_, err = observer.ProcessDelivery(delivery)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrNotHeartbeatMessage, he.Kind)

		// Verify no record was created.
		senderAddr := key.PublicKey().EVMAddress().Hex()
		record := observer.GetRecord(senderAddr)
		assert.Nil(t, record, "no record should be created for non-heartbeat messages")
	})
}

// --- Phase 9 (T074): Parent-Signed Heartbeat Rejection (FR-H27a) ---

// TestParentSignedHeartbeatRejection explicitly validates FR-H27a: a heartbeat
// signed by a PARENT key but claiming a CHILD's senderAddress in the TopicMessage
// envelope MUST be rejected with ErrSenderAddressMismatch (V-OBS-01).
//
// This complements the existing V-OBS-01b test with an explicitly named T074 label
// and more detailed assertion context referencing the FR-H27a requirement.
func TestParentSignedHeartbeatRejection(t *testing.T) {
	now := uint64(1700000000)

	t.Run("T074: FR-H27a — parent-signed heartbeat with child senderAddress is rejected", func(t *testing.T) {
		parentKey := newTestKey(t)
		childKey := newTestKey(t)

		// Build a valid heartbeat payload.
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		payloadJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		// Sign with the PARENT's key — this is the authentic signer.
		msg, err := topic.NewTopicMessage(parentKey, now, 0, payloadJSON)
		require.NoError(t, err)

		// Override the senderAddress to the CHILD's EVMAddress.
		// This simulates a forgery attempt where the parent tries to heartbeat
		// on behalf of the child, which FR-H27a must reject.
		msg = topic.TopicMessageFromFields(childKey.PublicKey().EVMAddress().Hex(), msg.Signature(), msg.Timestamp(), msg.SequenceNumber(), msg.Payload())

		// ValidateInboundHeartbeat must reject: the recovered signer (parent)
		// does not match the claimed sender (child).
		_, err = ValidateInboundHeartbeat(msg, now)
		require.Error(t, err, "FR-H27a: parent-signed heartbeat with child address must be rejected")
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrSenderAddressMismatch, he.Kind,
			"FR-H27a: must produce ErrSenderAddressMismatch, not a generic signature failure")
	})

	t.Run("T074: child-signed heartbeat with child senderAddress is accepted", func(t *testing.T) {
		childKey := newTestKey(t)

		// The child signs its own heartbeat — this is the legitimate case.
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, childKey, payload, now)

		// Must succeed: signer matches senderAddress.
		validated, err := ValidateInboundHeartbeat(msg, now)
		require.NoError(t, err, "child-signed heartbeat with matching address must be accepted")
		assert.Equal(t, now+60, validated.NextHeartbeatDeadline)
	})
}

// --- Phase 9 (T075a): FR-H22 Observer-Side stdOut Enforcement ---

// TestObserverStdOutEnforcementStrategy documents the FR-H22 enforcement model.
//
// FR-H22 states: "heartbeat MUST be published on stdOut only."
//
// DESIGN DECISION: The observer (ProcessDelivery) is intentionally agnostic to
// which topic the heartbeat was received on. It validates the message content
// (signature, type, version, deadline) but does NOT check which topic the delivery
// came from. This is because:
//
//  1. The observer does not know about TopicRef at validation time — it only
//     receives a MessageDelivery with a consensus timestamp and sequence number.
//  2. Topic-level filtering is the APPLICATION's responsibility. The caller
//     decides which topics to subscribe to. If the caller only subscribes to
//     the peer's stdOut topic (as FR-H22 requires), then all heartbeats
//     received through that subscription are inherently from stdOut.
//  3. This design keeps the observer pure and transport-agnostic.
//
// The test below demonstrates that ProcessDelivery accepts valid heartbeats
// regardless of how the delivery was constructed, and that topic filtering
// is the caller's responsibility.
func TestObserverStdOutEnforcementStrategy(t *testing.T) {
	now := uint64(1700000000)

	t.Run("T075a: ProcessDelivery is topic-agnostic (FR-H22 enforced at subscription layer)", func(t *testing.T) {
		key := newTestKey(t)
		observer := NewHeartbeatObserver()
		senderAddr := key.PublicKey().EVMAddress().Hex()

		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, payload, now)

		// Construct a delivery. Note: MessageDelivery has no TopicRef field —
		// it is transport-agnostic by design. The observer has no way to know
		// (and does not need to know) which topic this came from.
		delivery := topic.MessageDelivery{
			Message:            msg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		}

		// ProcessDelivery validates content, not topic origin.
		result, err := observer.ProcessDelivery(delivery)
		require.NoError(t, err,
			"FR-H22: observer processes valid heartbeats regardless of topic origin; "+
				"stdOut enforcement is the caller's responsibility at the subscription layer")
		assert.Equal(t, PayloadTypeHeartbeat, result.Type)
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, now))
	})
}

// --- Phase 5 (T047): Shutdown Sentinel Observer OFFLINE Transition ---

func TestObserverShutdownSentinelOfflineTransition(t *testing.T) {
	now := uint64(1700000000)

	t.Run("T047: receive shutdown heartbeat and transition to OFFLINE", func(t *testing.T) {
		key := newTestKey(t)
		observer := NewHeartbeatObserver()
		senderAddr := key.PublicKey().EVMAddress().Hex()

		// First, establish ALIVE state with a normal heartbeat.
		normalPayload := BuildHeartbeatPayload(now+60, RoleBuyer)
		normalMsg := createSignedHeartbeatMessage(t, key, normalPayload, now)
		_, err := observer.ProcessDelivery(topic.MessageDelivery{
			Message:            normalMsg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		})
		require.NoError(t, err)
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, now),
			"peer must be ALIVE after normal heartbeat")

		// Now send shutdown heartbeat (deadline=0).
		shutdownPayload := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		shutdownPayload.NextHeartbeatDeadline = ShutdownSentinel
		shutdownMsg := createSignedHeartbeatMessage(t, key, shutdownPayload, now+10)

		// Validate via ValidateInboundHeartbeat — V-OBS-04 must accept.
		validatedPayload, err := ValidateInboundHeartbeat(shutdownMsg, now+10)
		require.NoError(t, err, "V-OBS-04: shutdown sentinel must be accepted by inbound validation")
		assert.Equal(t, ShutdownSentinel, validatedPayload.NextHeartbeatDeadline)

		// Process the shutdown delivery.
		_, err = observer.ProcessDelivery(topic.MessageDelivery{
			Message:            shutdownMsg,
			ConsensusTimestamp: now + 10,
			BackendSequence:    2,
		})
		require.NoError(t, err)

		// Verify UpdateLivenessRecord transitions to OFFLINE.
		record := observer.GetRecord(senderAddr)
		require.NotNil(t, record)
		assert.Equal(t, StateOffline, record.currentState,
			"peer must transition to OFFLINE after shutdown heartbeat")
		assert.Equal(t, ShutdownSentinel, record.lastDeadline)

		// EvaluateLiveness must also return OFFLINE.
		assert.Equal(t, StateOffline, observer.GetLivenessState(senderAddr, now+100),
			"EvaluateLiveness must return OFFLINE regardless of time elapsed")
	})
}

// --- Phase 5 (T049): Integrated Shutdown/Recovery Cycle Test ---

func TestObserverShutdownRecoveryCycle(t *testing.T) {
	now := uint64(1700000000)

	t.Run("T049: full cycle — ALIVE -> OFFLINE -> ALIVE via ProcessDelivery", func(t *testing.T) {
		key := newTestKey(t)
		observer := NewHeartbeatObserver()
		senderAddr := key.PublicKey().EVMAddress().Hex()

		// Step 1: Publish normal heartbeat -> ALIVE.
		p1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		msg1 := createSignedHeartbeatMessage(t, key, p1, now)
		_, err := observer.ProcessDelivery(topic.MessageDelivery{
			Message:            msg1,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		})
		require.NoError(t, err)
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, now),
			"step 1: peer must be ALIVE")

		// Step 2: Publish shutdown heartbeat -> OFFLINE.
		pShutdown := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		pShutdown.NextHeartbeatDeadline = ShutdownSentinel
		msg2 := createSignedHeartbeatMessage(t, key, pShutdown, now+30)
		_, err = observer.ProcessDelivery(topic.MessageDelivery{
			Message:            msg2,
			ConsensusTimestamp: now + 30,
			BackendSequence:    2,
		})
		require.NoError(t, err)
		assert.Equal(t, StateOffline, observer.GetLivenessState(senderAddr, now+30),
			"step 2: peer must be OFFLINE after shutdown")

		// Step 3: Publish recovery heartbeat -> ALIVE again (FR-H21).
		pRecovery := BuildHeartbeatPayload(now+200, RoleBuyer)
		msg3 := createSignedHeartbeatMessage(t, key, pRecovery, now+100)
		_, err = observer.ProcessDelivery(topic.MessageDelivery{
			Message:            msg3,
			ConsensusTimestamp: now + 100,
			BackendSequence:    3,
		})
		require.NoError(t, err)
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, now+100),
			"step 3: peer must be ALIVE again after recovery (FR-H21)")

		// Verify record is fully updated.
		record := observer.GetRecord(senderAddr)
		require.NotNil(t, record)
		assert.Equal(t, StateAlive, record.currentState)
		assert.Equal(t, uint64(now+200), record.lastDeadline)
		assert.Equal(t, uint64(3), record.lastSequence)
	})
}

// --- Phase 6 (T057): Observer-Side Delta Boundary Tests ---

func TestValidateInboundHeartbeatDeltaBoundaries(t *testing.T) {
	now := uint64(1700000000)
	key := newTestKey(t)

	t.Run("T057: accept delta=MinDeadlineDelta (10)", func(t *testing.T) {
		p := BuildHeartbeatPayload(now+MinDeadlineDelta, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, p, now)
		payload, err := ValidateInboundHeartbeat(msg, now)
		require.NoError(t, err, "delta=10 must be accepted by observer")
		assert.Equal(t, now+MinDeadlineDelta, payload.NextHeartbeatDeadline)
	})

	t.Run("T057: accept delta=MaxDeadlineDelta (86400)", func(t *testing.T) {
		p := BuildHeartbeatPayload(now+MaxDeadlineDelta, RoleBuyer)
		msg := createSignedHeartbeatMessage(t, key, p, now)
		payload, err := ValidateInboundHeartbeat(msg, now)
		require.NoError(t, err, "delta=86400 must be accepted by observer")
		assert.Equal(t, now+MaxDeadlineDelta, payload.NextHeartbeatDeadline)
	})

	t.Run("T057: reject delta=MinDeadlineDelta-1 (9)", func(t *testing.T) {
		p := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now + MinDeadlineDelta - 1,
			Role:                  RoleBuyer,
		}
		msg := createSignedHeartbeatMessage(t, key, p, now)
		_, err := ValidateInboundHeartbeat(msg, now)
		require.Error(t, err, "delta=9 must be rejected by observer")
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeltaBelowMinimum, he.Kind)
	})

	t.Run("T057: reject delta=MaxDeadlineDelta+1 (86401)", func(t *testing.T) {
		p := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now + MaxDeadlineDelta + 1,
			Role:                  RoleBuyer,
		}
		msg := createSignedHeartbeatMessage(t, key, p, now)
		_, err := ValidateInboundHeartbeat(msg, now)
		require.Error(t, err, "delta=86401 must be rejected by observer")
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeltaExceedsMaximum, he.Kind)
	})
}

// --- Phase 7 (T063): Peers Trust Warning Test ---

func TestObserverPeersTrustWarning(t *testing.T) {
	now := uint64(1700000000)

	// T063: Assert observer does NOT use peers field for trust/routing/identity
	// decisions. The peers data is passed through as informational only (FR-H27).
	t.Run("T063: peers field is informational only and not used for trust decisions", func(t *testing.T) {
		key := newTestKey(t)
		observer := NewHeartbeatObserver()
		senderAddr := key.PublicKey().EVMAddress().Hex()

		// Build payload with peers.
		peers := []AbbreviatedAddress{"a1b2", "c3d4", "dead"}
		payload := BuildHeartbeatPayload(now+60, RoleBuyer, WithPeers(peers))
		msg := createSignedHeartbeatMessage(t, key, payload, now)

		result, err := observer.ProcessDelivery(topic.MessageDelivery{
			Message:            msg,
			ConsensusTimestamp: now,
			BackendSequence:    1,
		})
		require.NoError(t, err)

		// The returned payload carries peers data for informational purposes.
		require.Len(t, result.Peers, 3)
		assert.Equal(t, AbbreviatedAddress("a1b2"), result.Peers[0])

		// The liveness record does NOT store or use peers for any decision.
		// Only senderAddress, currentState, lastDeadline, lastSequence, lastConsensusTimestamp
		// are tracked. Peers are ephemeral gossip-grade data (FR-H27).
		record := observer.GetRecord(senderAddr)
		require.NotNil(t, record)
		assert.Equal(t, StateAlive, record.currentState,
			"liveness state is determined only by deadline-based evaluation, not peers")

		// Verify state evaluation does not change based on peers content.
		// A second heartbeat with different peers should produce the same state behavior.
		peers2 := []AbbreviatedAddress{"ffff", "0000"}
		payload2 := BuildHeartbeatPayload(now+120, RoleBuyer, WithPeers(peers2))
		msg2 := createSignedHeartbeatMessage(t, key, payload2, now+30)
		_, err = observer.ProcessDelivery(topic.MessageDelivery{
			Message:            msg2,
			ConsensusTimestamp: now + 30,
			BackendSequence:    2,
		})
		require.NoError(t, err)
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, now+30),
			"peers content must not affect liveness evaluation")
	})
}

// --- Phase 8 (T068): EVM block.timestamp as Consensus Source ---

func TestObserverEVMBlockTimestampConsensus(t *testing.T) {
	// T068: Construct a MessageDelivery with consensusTimestamp sourced from
	// EVM block.timestamp, run validation and liveness evaluation. The observer
	// is transport-agnostic: it only cares about the abstract uint64 consensusTimestamp,
	// not where it came from. This proves EVM block.timestamp works as consensus source.
	t.Run("T068: EVM block.timestamp as consensus timestamp", func(t *testing.T) {
		key := newTestKey(t)
		observer := NewHeartbeatObserver()
		senderAddr := key.PublicKey().EVMAddress().Hex()

		// Simulate an EVM block.timestamp (typically Unix seconds, not nanoseconds).
		evmBlockTimestamp := uint64(1700000000) // block.timestamp from an EVM chain
		deadline := evmBlockTimestamp + 60

		payload := BuildHeartbeatPayload(deadline, RoleSeller)
		msg := createSignedHeartbeatMessage(t, key, payload, evmBlockTimestamp)

		// Deliver with EVM block.timestamp as the consensus timestamp.
		delivery := topic.MessageDelivery{
			Message:            msg,
			ConsensusTimestamp: evmBlockTimestamp,
			BackendSequence:    42, // e.g., log index or block number
		}

		result, err := observer.ProcessDelivery(delivery)
		require.NoError(t, err)
		assert.Equal(t, deadline, result.NextHeartbeatDeadline)

		// Verify liveness evaluation works with EVM-sourced timestamp.
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, evmBlockTimestamp),
			"peer must be ALIVE when evaluated at the EVM block.timestamp")
		assert.Equal(t, StateAlive, observer.GetLivenessState(senderAddr, deadline+GracePeriod),
			"peer must be ALIVE within grace window using EVM timestamps")
		assert.Equal(t, StateSuspect, observer.GetLivenessState(senderAddr, deadline+GracePeriod+1),
			"peer must be SUSPECT after grace window using EVM timestamps")
	})
}

// --- parseMajorVersion Tests ---

func TestParseMajorVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    int
		wantErr bool
	}{
		{"standard 1.0.0", "1.0.0", 1, false},
		{"minor bump 1.1.0", "1.1.0", 1, false},
		{"major 2", "2.0.0", 2, false},
		{"major 0", "0.9.0", 0, false},
		{"major only", "3", 3, false},
		{"empty string", "", 0, true},
		{"non-numeric", "abc.1.0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMajorVersion(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
