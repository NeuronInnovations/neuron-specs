package health

import (
	"encoding/json"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Phase 9 (T073): Deterministic Signing Test (Constitution X) ---

// TestDeterministicSigningRFC6979 validates Constitution X (Deterministic message
// signing — RFC 6979 nonces, Keccak256 + ECDSA R||S||V). When the same key signs
// the same payload with the same timestamp, the resulting signatures MUST be
// byte-identical due to RFC 6979 deterministic nonce generation.
func TestDeterministicSigningRFC6979(t *testing.T) {
	t.Run("T073: same key + same payload + same timestamp produces byte-equal R||S||V signatures", func(t *testing.T) {
		key := newTestKey(t)
		now := uint64(1700000000)

		// Build the SAME HeartbeatPayload.
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)

		// Serialize to JSON (deterministic due to custom MarshalJSON).
		payloadJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		// Sign it twice via topic.NewTopicMessage with the same key and timestamp.
		msg1, err := topic.NewTopicMessage(key, now, 0, payloadJSON)
		require.NoError(t, err)

		msg2, err := topic.NewTopicMessage(key, now, 0, payloadJSON)
		require.NoError(t, err)

		// Assert byte-equal R||S||V signatures between the two (RFC 6979).
		assert.Equal(t, msg1.Signature(), msg2.Signature(),
			"RFC 6979 deterministic nonces must produce identical R||S||V signatures "+
				"when signing the same message with the same key")

		// Also verify the sender addresses match (derived from same key).
		assert.Equal(t, msg1.SenderAddress(), msg2.SenderAddress(),
			"sender addresses must be identical for the same key")

		// Verify both messages pass validation.
		require.NoError(t, topic.ValidateTopicMessage(msg1))
		require.NoError(t, topic.ValidateTopicMessage(msg2))
	})

	t.Run("T073: different payloads produce different signatures", func(t *testing.T) {
		key := newTestKey(t)
		now := uint64(1700000000)

		payload1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		payload1JSON, err := json.Marshal(payload1)
		require.NoError(t, err)

		payload2 := BuildHeartbeatPayload(now+120, RoleSeller)
		payload2JSON, err := json.Marshal(payload2)
		require.NoError(t, err)

		msg1, err := topic.NewTopicMessage(key, now, 0, payload1JSON)
		require.NoError(t, err)

		msg2, err := topic.NewTopicMessage(key, now, 0, payload2JSON)
		require.NoError(t, err)

		// Different payloads must produce different signatures.
		assert.NotEqual(t, msg1.Signature(), msg2.Signature(),
			"different payloads must produce different signatures")
	})
}

// --- Phase 5 (T050-T052): Shutdown Sentinel Signing Verification ---

// TestShutdownSentinelSigningVerification verifies that shutdown sentinel payloads
// are correctly signed and can be verified through the standard topic message
// validation pipeline.
func TestShutdownSentinelSigningVerification(t *testing.T) {
	t.Run("T050: shutdown sentinel payload signs and verifies correctly", func(t *testing.T) {
		key := newTestKey(t)
		now := uint64(1700000000)

		// Build shutdown payload.
		payload := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		payload.NextHeartbeatDeadline = ShutdownSentinel

		// Serialize to JSON.
		payloadJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		// Create signed message.
		msg, err := topic.NewTopicMessage(key, now, 0, payloadJSON)
		require.NoError(t, err)

		// Validate the topic message signature.
		err = topic.ValidateTopicMessage(msg)
		assert.NoError(t, err, "shutdown sentinel message must pass signature verification")

		// Validate via ValidateInboundHeartbeat.
		validated, err := ValidateInboundHeartbeat(msg, now)
		require.NoError(t, err, "shutdown sentinel must pass V-OBS-04 bypass")
		assert.Equal(t, ShutdownSentinel, validated.NextHeartbeatDeadline)
	})

	t.Run("T051: shutdown sentinel with tampered payload fails signature check", func(t *testing.T) {
		key := newTestKey(t)
		now := uint64(1700000000)

		// Build and sign shutdown payload.
		payload := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		payload.NextHeartbeatDeadline = ShutdownSentinel
		payloadJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		msg, err := topic.NewTopicMessage(key, now, 0, payloadJSON)
		require.NoError(t, err)

		// Tamper with the payload (change deadline from 0 to 60).
		tamperedPayload := BuildHeartbeatPayload(now+60, RoleBuyer)
		tamperedPayloadJSON, err2 := json.Marshal(tamperedPayload)
		require.NoError(t, err2)
		msg = topic.TopicMessageFromFields(msg.SenderAddress(), msg.Signature(), msg.Timestamp(), msg.SequenceNumber(), tamperedPayloadJSON)

		// Signature check must fail since payload was modified after signing.
		_, err = ValidateInboundHeartbeat(msg, now)
		require.Error(t, err, "tampered shutdown payload must fail validation")
	})

	t.Run("T052: shutdown sentinel round-trips through publish pipeline", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()
		now := uint64(1700000000)

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
		payload := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		payload.NextHeartbeatDeadline = ShutdownSentinel
		_, err := PublishHeartbeat(payload, key, ref, adapter, now, 1)
		require.NoError(t, err)

		// The captured message must pass full signature verification.
		err = topic.ValidateTopicMessage(capturedMsg)
		assert.NoError(t, err, "published shutdown sentinel must pass signature verification")

		// The captured message must pass inbound validation.
		validated, err := ValidateInboundHeartbeat(capturedMsg, now)
		require.NoError(t, err)
		assert.Equal(t, ShutdownSentinel, validated.NextHeartbeatDeadline)

		// The captured message must produce OFFLINE when processed by observer.
		senderAddr := key.PublicKey().EVMAddress().Hex()
		record := UpdateLivenessRecord(nil, senderAddr, validated, now, 1)
		assert.Equal(t, StateOffline, record.currentState,
			"shutdown sentinel must produce OFFLINE record")
	})
}
