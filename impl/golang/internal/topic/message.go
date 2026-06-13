package topic

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// FR-T02: TopicMessage envelope structure
// TopicMessage is the signed message envelope for topic-based communication.
//
// Fields follow FR-T02 (envelope structure) and FR-T03 (deterministic signing):
//   - senderAddress: EVM address (EIP-55 checksummed hex) of the signer
//   - signature: R||S||V signature bytes (65 bytes, Keccak256 + ECDSA, RFC 6979)
//   - timestamp: Unix timestamp in nanoseconds when the message was created
//   - sequenceNumber: Sender-assigned monotonic sequence number
//   - payload: Opaque application-level payload bytes
type TopicMessage struct {
	senderAddress  string
	signature      []byte
	timestamp      uint64
	sequenceNumber uint64
	payload        []byte
}

// SenderAddress returns the EVM address of the sender.
func (m *TopicMessage) SenderAddress() string { return m.senderAddress }

// Signature returns a copy of the 65-byte signature.
func (m *TopicMessage) Signature() []byte {
	out := make([]byte, len(m.signature))
	copy(out, m.signature)
	return out
}

// Timestamp returns the message timestamp.
func (m *TopicMessage) Timestamp() uint64 { return m.timestamp }

// FR-T06: Per-sender message ordering by sequence number
// SequenceNumber returns the message sequence number.
func (m *TopicMessage) SequenceNumber() uint64 { return m.sequenceNumber }

// Payload returns a copy of the payload bytes.
func (m *TopicMessage) Payload() []byte {
	out := make([]byte, len(m.payload))
	copy(out, m.payload)
	return out
}

// TopicMessageFromFields constructs a TopicMessage from raw components.
// This bypasses signing -- use NewTopicMessage for properly signed messages.
// Primarily useful for deserialization and testing.
func TopicMessageFromFields(senderAddress string, signature []byte, timestamp, sequenceNumber uint64, payload []byte) TopicMessage {
	// Defensive copies of slices.
	var sigCopy []byte
	if signature != nil {
		sigCopy = make([]byte, len(signature))
		copy(sigCopy, signature)
	}
	var payloadCopy []byte
	if payload != nil {
		payloadCopy = make([]byte, len(payload))
		copy(payloadCopy, payload)
	}
	return TopicMessage{
		senderAddress:  senderAddress,
		signature:      sigCopy,
		timestamp:      timestamp,
		sequenceNumber: sequenceNumber,
		payload:        payloadCopy,
	}
}

// MarshalJSON implements json.Marshaler for TopicMessage.
//
// Field order is fixed (canonical per 004 FR-T21 / 006 FR-W05):
//
//	senderAddress, signature, timestamp, sequenceNumber, payload
//
// Per 006 FR-W02, the uint64 fields `timestamp` and `sequenceNumber` are
// emitted as quoted JSON strings (decimal) so values above 2^53 survive
// JavaScript / JSON.parse round-trips. Binary fields are base64 (FR-W03)
// via Go's stdlib encoding for []byte.
func (m TopicMessage) MarshalJSON() ([]byte, error) {
	buf := []byte{'{'}

	// senderAddress
	senderBytes, err := json.Marshal(m.senderAddress)
	if err != nil {
		return nil, fmt.Errorf("marshal senderAddress: %w", err)
	}
	buf = append(buf, `"senderAddress":`...)
	buf = append(buf, senderBytes...)

	// signature (base64-encoded by stdlib)
	sigBytes, err := json.Marshal(m.signature)
	if err != nil {
		return nil, fmt.Errorf("marshal signature: %w", err)
	}
	buf = append(buf, `,"signature":`...)
	buf = append(buf, sigBytes...)

	// timestamp (quoted decimal string per FR-W02)
	buf = append(buf, `,"timestamp":"`...)
	buf = strconv.AppendUint(buf, m.timestamp, 10)
	buf = append(buf, '"')

	// sequenceNumber (quoted decimal string per FR-W02)
	buf = append(buf, `,"sequenceNumber":"`...)
	buf = strconv.AppendUint(buf, m.sequenceNumber, 10)
	buf = append(buf, '"')

	// payload (base64-encoded by stdlib)
	payloadBytes, err := json.Marshal(m.payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	buf = append(buf, `,"payload":`...)
	buf = append(buf, payloadBytes...)

	buf = append(buf, '}')
	return buf, nil
}

// UnmarshalJSON implements json.Unmarshaler for TopicMessage.
//
// Per 006 FR-W02 the canonical wire form has `timestamp` and `sequenceNumber`
// as quoted decimal strings. To stay liberal in what we accept, this method
// also tolerates legacy JSON-number form (used by previous in-memory state),
// converting both to uint64.
func (m *TopicMessage) UnmarshalJSON(data []byte) error {
	type jsonMsg struct {
		SenderAddress  string         `json:"senderAddress"`
		Signature      []byte         `json:"signature"`
		Timestamp      flexibleUint64 `json:"timestamp"`
		SequenceNumber flexibleUint64 `json:"sequenceNumber"`
		Payload        []byte         `json:"payload"`
	}
	var j jsonMsg
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	m.senderAddress = j.SenderAddress
	m.signature = j.Signature
	m.timestamp = uint64(j.Timestamp)
	m.sequenceNumber = uint64(j.SequenceNumber)
	m.payload = j.Payload
	return nil
}

// flexibleUint64 is a uint64 that accepts both quoted-decimal-string and
// JSON-number forms during deserialization. The canonical wire form per
// 006 FR-W02 is the quoted string; the number form is accepted only for
// backward compatibility with legacy in-memory artifacts.
type flexibleUint64 uint64

// UnmarshalJSON parses either `"123"` or `123` into a uint64.
func (f *flexibleUint64) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("flexibleUint64: empty input")
	}
	s := string(data)
	if s[0] == '"' && len(s) >= 2 && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("flexibleUint64: parse %q: %w", s, err)
	}
	*f = flexibleUint64(n)
	return nil
}

// FR-T03: Messages signed with Keccak256 + ECDSA
// TopicMessageSigningInput builds the canonical signing input for a TopicMessage.
// The format is: timestamp(8 bytes big-endian) || sequenceNumber(8 bytes big-endian) || payload.
// This is the raw bytes passed to key.Sign(), which internally applies Keccak256.
func TopicMessageSigningInput(timestamp uint64, sequenceNumber uint64, payload []byte) []byte {
	buf := make([]byte, 8+8+len(payload))
	binary.BigEndian.PutUint64(buf[0:8], timestamp)
	binary.BigEndian.PutUint64(buf[8:16], sequenceNumber)
	copy(buf[16:], payload)
	return buf
}

// FR-T19: Transactional invariants: Sequenced, Immutable, Verifiable, Signed
// NewTopicMessage constructs and signs a TopicMessage.
// The senderAddress is derived from key.PublicKey().EVMAddress().Hex().
// The signing input is: timestamp(8 bytes BE) || sequenceNumber(8 bytes BE) || payload.
// key.Sign() internally applies Keccak256 before ECDSA signing with RFC 6979 nonces.
func NewTopicMessage(key *keylib.NeuronPrivateKey, timestamp uint64, sequenceNumber uint64, payload []byte) (TopicMessage, error) {
	if key == nil {
		return TopicMessage{}, NewTopicError(ErrInvalidSignature, "private key must not be nil")
	}

	signingInput := TopicMessageSigningInput(timestamp, sequenceNumber, payload)

	sig, err := key.Sign(signingInput)
	if err != nil {
		return TopicMessage{}, WrapTopicError(ErrInvalidSignature, "failed to sign message", err)
	}

	senderAddress := key.PublicKey().EVMAddress().Hex()

	// Defensive copy of payload to prevent shared-slice mutation.
	var payloadCopy []byte
	if payload != nil {
		payloadCopy = make([]byte, len(payload))
		copy(payloadCopy, payload)
	}

	return TopicMessage{
		senderAddress:  senderAddress,
		signature:      sig.Bytes(),
		timestamp:      timestamp,
		sequenceNumber: sequenceNumber,
		payload:        payloadCopy,
	}, nil
}

// FR-T21: Deterministic JSON serialization (canonical field order)
// ToJSON serializes the TopicMessage to JSON with canonical field order.
// The field order matches the struct tag order: senderAddress, signature,
// timestamp, sequenceNumber, payload.
func (m TopicMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// TopicMessageFromJSON deserializes a TopicMessage from JSON bytes.
func TopicMessageFromJSON(data []byte) (TopicMessage, error) {
	var msg TopicMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return TopicMessage{}, WrapTopicError(ErrInvalidConfig, "failed to deserialize topic message", err)
	}
	return msg, nil
}
