package health

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// ValidateInboundHeartbeat validates a received TopicMessage as a heartbeat.
// Checks are performed in strict order (V-OBS-01..06) and the first failure
// is returned immediately.
//
// consensusTimestamp is the backend-assigned consensus time for the message
// (abstract uint64, typically Unix seconds). This value is backend-agnostic:
// it may come from HCS consensus, EVM block.timestamp, Kafka broker timestamp,
// or any other transport backend. The observer does not differentiate between
// backends — all time arithmetic operates on this abstract uint64 (T070).
//
// Cross-chain property (T071): Because the payload schema is transport-independent
// and all validation rules use relative delta comparisons against the abstract
// consensusTimestamp, a heartbeat produced for one chain can be verified on
// another chain provided the consensus timestamps are in the same epoch and unit.
//
// On success, returns the deserialized HeartbeatPayload. The shutdown sentinel
// (deadline==0) bypasses all delta checks (V-OBS-04).
func ValidateInboundHeartbeat(msg topic.TopicMessage, consensusTimestamp uint64) (*HeartbeatPayload, error) {
	// V-OBS-01: Validate message signature and sender address.
	if err := topic.ValidateTopicMessage(msg); err != nil {
		// Map topic error kinds to health error kinds.
		var topicErr topic.TopicError
		if errors.As(err, &topicErr) {
			switch topicErr.Kind {
			case topic.ErrSenderMismatch:
				return nil, WrapHealthError(ErrSenderAddressMismatch,
					"sender address does not match recovered signer", err)
			default:
				return nil, WrapHealthError(ErrSignatureVerificationFailed,
					"signature verification failed", err)
			}
		}
		return nil, WrapHealthError(ErrSignatureVerificationFailed,
			"signature verification failed", err)
	}

	// V-OBS-02: Deserialize and check payload type.
	var payload HeartbeatPayload
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		return nil, WrapHealthError(ErrNotHeartbeatMessage,
			"failed to deserialize payload as heartbeat", err)
	}
	if payload.Type != PayloadTypeHeartbeat {
		return nil, NewHealthError(ErrNotHeartbeatMessage,
			fmt.Sprintf("expected payload type %q, got %q", PayloadTypeHeartbeat, payload.Type))
	}

	// V-OBS-03: Check version compatibility (major must be 1).
	major, err := parseMajorVersion(payload.Version)
	if err != nil {
		return nil, WrapHealthError(ErrIncompatibleVersion,
			fmt.Sprintf("failed to parse version %q", payload.Version), err)
	}
	if major != 1 {
		return nil, NewHealthError(ErrIncompatibleVersion,
			fmt.Sprintf("incompatible major version %d (expected 1)", major))
	}

	// V-OBS-04: Shutdown sentinel bypasses delta checks.
	if payload.NextHeartbeatDeadline == ShutdownSentinel {
		return &payload, nil
	}

	// V-OBS-05: Deadline must be in the future relative to consensus timestamp.
	if payload.NextHeartbeatDeadline <= consensusTimestamp {
		return nil, NewHealthError(ErrDeadlineInPast,
			fmt.Sprintf("nextHeartbeatDeadline %d is not after consensusTimestamp %d",
				payload.NextHeartbeatDeadline, consensusTimestamp))
	}

	// V-OBS-06: Delta range checks.
	delta := payload.NextHeartbeatDeadline - consensusTimestamp
	if delta < MinDeadlineDelta {
		return nil, NewHealthError(ErrDeltaBelowMinimum,
			fmt.Sprintf("delta %d is below minimum %d", delta, MinDeadlineDelta))
	}
	if delta > MaxDeadlineDelta {
		return nil, NewHealthError(ErrDeltaExceedsMaximum,
			fmt.Sprintf("delta %d exceeds maximum %d", delta, MaxDeadlineDelta))
	}

	return &payload, nil
}

// parseMajorVersion extracts the major version number from a semver string.
// Accepts formats like "1.0.0", "1.1.0", "2.0.0".
func parseMajorVersion(version string) (int, error) {
	parts := strings.SplitN(version, ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("empty version string")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}
	return major, nil
}

// TransitionState applies a validated heartbeat to a LivenessRecord, producing
// an updated record.
//
// Rules:
//   - If record is nil, a new record is created with the given senderAddress.
//   - If sequenceNumber <= record.lastSequence, the record is returned unchanged
//     (FR-H17: ordering enforcement).
//   - If payload.NextHeartbeatDeadline == ShutdownSentinel, state transitions to OFFLINE.
//   - Otherwise, state transitions to ALIVE (FR-H21: recovery invariant — any valid
//     heartbeat from any state produces ALIVE).
//   - lastDeadline, lastSequence, and lastConsensusTimestamp are updated.
func TransitionState(record *LivenessRecord, senderAddress string, payload *HeartbeatPayload, consensusTimestamp uint64, sequenceNumber uint64) *LivenessRecord {
	if record == nil {
		record = &LivenessRecord{
			senderAddress: senderAddress,
			currentState:  StateUnknown,
		}
	}

	// FR-H17: reject out-of-order or duplicate sequences.
	if record.lastSequence > 0 && sequenceNumber <= record.lastSequence {
		return record
	}

	// Determine new state.
	if payload.NextHeartbeatDeadline == ShutdownSentinel {
		record.currentState = StateOffline
	} else {
		// FR-H21: any valid heartbeat from any state -> ALIVE.
		record.currentState = StateAlive
	}

	record.lastDeadline = payload.NextHeartbeatDeadline
	record.lastSequence = sequenceNumber
	record.lastConsensusTimestamp = consensusTimestamp

	return record
}

// UpdateLivenessRecord is a convenience wrapper around TransitionState that
// also manages the senderAddress field.
//
// If record is nil, a new record is created with the provided senderAddress.
// Otherwise, delegates to TransitionState logic.
func UpdateLivenessRecord(record *LivenessRecord, senderAddress string, payload *HeartbeatPayload, consensusTimestamp uint64, sequenceNumber uint64) *LivenessRecord {
	return TransitionState(record, senderAddress, payload, consensusTimestamp, sequenceNumber)
}

// HeartbeatObserver tracks liveness state for multiple peers by processing
// inbound heartbeat messages.
//
// HeartbeatObserver is safe for concurrent use.
type HeartbeatObserver struct {
	records map[string]*LivenessRecord // keyed by senderAddress
	mu      sync.RWMutex
}

// NewHeartbeatObserver constructs an empty HeartbeatObserver.
func NewHeartbeatObserver() *HeartbeatObserver {
	return &HeartbeatObserver{
		records: make(map[string]*LivenessRecord),
	}
}

// ProcessDelivery validates an inbound message delivery, and if it is a valid
// heartbeat, updates the sender's liveness record.
//
// T066 (FR-H27): The peers field in HeartbeatPayload is gossip-grade data.
// It is passed through to the caller as part of the returned HeartbeatPayload,
// but MUST NOT be used by the observer for trust, routing, or identity decisions.
// Liveness evaluation is based exclusively on deadline-based arithmetic,
// not on any gossip content such as the peers field.
//
// Returns the deserialized HeartbeatPayload on success, or an error if
// validation fails.
func (o *HeartbeatObserver) ProcessDelivery(delivery topic.MessageDelivery) (*HeartbeatPayload, error) {
	// Step 1: Validate inbound heartbeat.
	payload, err := ValidateInboundHeartbeat(delivery.Message, delivery.ConsensusTimestamp)
	if err != nil {
		return nil, err
	}

	// Step 2: Update liveness record.
	senderAddress := delivery.Message.SenderAddress()
	o.mu.Lock()
	existing := o.records[senderAddress]
	updated := UpdateLivenessRecord(existing, senderAddress, payload, delivery.ConsensusTimestamp, delivery.BackendSequence)
	o.records[senderAddress] = updated
	o.mu.Unlock()

	return payload, nil
}

// GetLivenessState returns the evaluated liveness state for the given sender
// at the specified currentTime.
//
// If no record exists for the sender, returns StateUnknown.
func (o *HeartbeatObserver) GetLivenessState(senderAddress string, currentTime uint64) LivenessState {
	o.mu.RLock()
	record := o.records[senderAddress]
	o.mu.RUnlock()

	return EvaluateLiveness(record, currentTime)
}

// GetRecord returns the liveness record for the given sender, or nil if
// no heartbeat has been received from that sender.
// The returned record is a defensive copy; mutations will not affect the
// observer's internal state.
func (o *HeartbeatObserver) GetRecord(senderAddress string) *LivenessRecord {
	o.mu.RLock()
	defer o.mu.RUnlock()

	r := o.records[senderAddress]
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}
