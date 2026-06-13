package topic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Spec 006 FR-W02: UnsignedInt64 fields (timestamp, sequenceNumber) MUST be
// encoded as JSON strings containing the decimal representation, so values
// above 2^53 survive cross-language JSON parsing.
//
// These tests lock the canonical wire form for TopicMessage and prevent any
// regression to JSON-number encoding.

func TestTopicMessage_FR_W02_TimestampIsQuotedString(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// Use a timestamp above 2^53 (the JavaScript safe-integer ceiling) to
	// guarantee any number-form regression would corrupt the value.
	timestamp := uint64(1700000000000000000) // > 2^53
	seqNum := uint64(9007199254740993)        // 2^53 + 1, also > JS safe int

	msg, err := NewTopicMessage(&key, timestamp, seqNum, []byte("FR-W02"))
	require.NoError(t, err)

	jsonBytes, err := msg.ToJSON()
	require.NoError(t, err)

	jsonStr := string(jsonBytes)
	assert.Contains(t, jsonStr, `"timestamp":"1700000000000000000"`,
		"FR-W02: timestamp must be a quoted decimal string in canonical wire form")
	assert.Contains(t, jsonStr, `"sequenceNumber":"9007199254740993"`,
		"FR-W02: sequenceNumber must be a quoted decimal string in canonical wire form")

	// Negative assertions: the legacy number form must NOT appear.
	assert.NotContains(t, jsonStr, `"timestamp":1700000000000000000`,
		"FR-W02 regression: timestamp must not be emitted as a JSON number")
	assert.NotContains(t, jsonStr, `"sequenceNumber":9007199254740993`,
		"FR-W02 regression: sequenceNumber must not be emitted as a JSON number")
}

func TestTopicMessage_FR_W02_CanonicalFieldOrder(t *testing.T) {
	// 004 FR-T21 / 006 FR-W05: senderAddress, signature, timestamp, sequenceNumber, payload.
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("order"))
	require.NoError(t, err)

	jsonBytes, err := msg.ToJSON()
	require.NoError(t, err)

	s := string(jsonBytes)
	idxSender := strings.Index(s, `"senderAddress"`)
	idxSig := strings.Index(s, `"signature"`)
	idxTS := strings.Index(s, `"timestamp"`)
	idxSeq := strings.Index(s, `"sequenceNumber"`)
	idxPayload := strings.Index(s, `"payload"`)

	require.NotEqual(t, -1, idxSender)
	require.NotEqual(t, -1, idxSig)
	require.NotEqual(t, -1, idxTS)
	require.NotEqual(t, -1, idxSeq)
	require.NotEqual(t, -1, idxPayload)

	assert.Less(t, idxSender, idxSig, "senderAddress before signature")
	assert.Less(t, idxSig, idxTS, "signature before timestamp")
	assert.Less(t, idxTS, idxSeq, "timestamp before sequenceNumber")
	assert.Less(t, idxSeq, idxPayload, "sequenceNumber before payload")
}

func TestTopicMessage_FR_W02_RoundTripQuotedForm(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	original, err := NewTopicMessage(&key, 1700000000000000000, 42, []byte("rtq"))
	require.NoError(t, err)

	jsonBytes, err := original.ToJSON()
	require.NoError(t, err)

	var restored TopicMessage
	require.NoError(t, json.Unmarshal(jsonBytes, &restored))

	assert.Equal(t, original.SenderAddress(), restored.SenderAddress())
	assert.Equal(t, original.Signature(), restored.Signature())
	assert.Equal(t, original.Timestamp(), restored.Timestamp())
	assert.Equal(t, original.SequenceNumber(), restored.SequenceNumber())
	assert.Equal(t, original.Payload(), restored.Payload())
}

func TestTopicMessage_FR_W02_AcceptsLegacyNumberForm(t *testing.T) {
	// Liberal in what we accept: a TopicMessage previously serialized with
	// JSON-number form (e.g. by an older in-memory artifact) must still
	// parse cleanly.
	legacyJSON := []byte(`{"senderAddress":"0x0000000000000000000000000000000000000001",` +
		`"signature":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",` +
		`"timestamp":1700000000000000000,` +
		`"sequenceNumber":42,` +
		`"payload":"aGVsbG8="}`)

	var msg TopicMessage
	require.NoError(t, json.Unmarshal(legacyJSON, &msg))

	assert.Equal(t, uint64(1700000000000000000), msg.Timestamp())
	assert.Equal(t, uint64(42), msg.SequenceNumber())
	assert.Equal(t, []byte("hello"), msg.Payload())
}

func TestTopicMessage_FR_W02_BinarySigningInputUnchanged(t *testing.T) {
	// FR-A08: the binary signing pre-image is timestamp(8 BE) || seq(8 BE) || payload.
	// The FR-W02 wire-format change must NOT affect the signing pre-image, otherwise
	// existing signatures would silently break.
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	timestamp := uint64(1700000000000000000)
	seqNum := uint64(7)
	payload := []byte("binary stays binary")

	msg, err := NewTopicMessage(&key, timestamp, seqNum, payload)
	require.NoError(t, err)

	// Verify the signature against the binary pre-image directly.
	signingInput := TopicMessageSigningInput(timestamp, seqNum, payload)
	sig, err := keylib.SignatureFromBytes(msg.Signature())
	require.NoError(t, err)
	assert.True(t, sig.Verify(signingInput, key.PublicKey()),
		"signature must still verify against the binary FR-A08 pre-image after FR-W02 wire fix")
}
