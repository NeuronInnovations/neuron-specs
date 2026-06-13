package health

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// ValidateOutboundHeartbeat validates a HeartbeatPayload before publishing.
// Checks are performed in strict order (V-PUB-01..07) and the first failure
// is returned immediately.
//
// senderClock is the publisher's current timestamp (abstract uint64, typically
// Unix seconds). The shutdown sentinel (deadline==0) bypasses all delta checks.
func ValidateOutboundHeartbeat(payload HeartbeatPayload, senderClock uint64) error {
	// V-PUB-01: payload type must be "heartbeat".
	if payload.Type != PayloadTypeHeartbeat {
		return NewHealthError(ErrInvalidPayloadType,
			fmt.Sprintf("expected payload type %q, got %q", PayloadTypeHeartbeat, payload.Type))
	}

	// V-PUB-02: version must be the current supported version.
	if payload.Version != CurrentVersion {
		return NewHealthError(ErrUnsupportedVersion,
			fmt.Sprintf("expected version %q, got %q", CurrentVersion, payload.Version))
	}

	// V-PUB-03: shutdown sentinel (deadline==0) bypasses all delta checks.
	if payload.NextHeartbeatDeadline == ShutdownSentinel {
		return nil
	}

	// V-PUB-04: deadline must be strictly in the future.
	if payload.NextHeartbeatDeadline <= senderClock {
		return NewHealthError(ErrDeadlineNotFuture,
			fmt.Sprintf("nextHeartbeatDeadline %d is not after senderClock %d",
				payload.NextHeartbeatDeadline, senderClock))
	}

	// V-PUB-05: delta must be at least MinDeadlineDelta.
	delta := payload.NextHeartbeatDeadline - senderClock
	if delta < MinDeadlineDelta {
		return NewHealthError(ErrDeadlineTooSoon,
			fmt.Sprintf("delta %d is below minimum %d", delta, MinDeadlineDelta))
	}

	// V-PUB-06: delta must not exceed MaxDeadlineDelta.
	if delta > MaxDeadlineDelta {
		return NewHealthError(ErrDeadlineTooFar,
			fmt.Sprintf("delta %d exceeds maximum %d", delta, MaxDeadlineDelta))
	}

	// V-PUB-07: role must be a recognized NodeRole.
	if !ValidNodeRole(payload.Role) {
		return NewHealthError(ErrUnrecognizedRole,
			fmt.Sprintf("unrecognized role %q", payload.Role))
	}

	return nil
}

// TrimPayload progressively trims optional fields from a HeartbeatPayload
// to bring its serialized JSON size under maxSize.
//
// Trimming order (least-to-most important optional data):
//  1. Peers
//  2. Capabilities
//  3. Location
//
// If the mandatory-fields-only payload still exceeds maxSize, returns ErrPayloadTooLarge.
func TrimPayload(payload *HeartbeatPayload, maxSize int) error {
	// Check current size.
	data, err := json.Marshal(payload)
	if err != nil {
		return WrapHealthError(ErrPayloadTooLarge, "failed to serialize payload for size check", err)
	}
	if len(data) <= maxSize {
		return nil
	}

	// Trim peers first (least important optional field).
	payload.Peers = nil
	data, err = json.Marshal(payload)
	if err != nil {
		return WrapHealthError(ErrPayloadTooLarge, "failed to serialize payload after trimming peers", err)
	}
	if len(data) <= maxSize {
		return nil
	}

	// Trim capabilities next.
	payload.Capabilities = nil
	data, err = json.Marshal(payload)
	if err != nil {
		return WrapHealthError(ErrPayloadTooLarge, "failed to serialize payload after trimming capabilities", err)
	}
	if len(data) <= maxSize {
		return nil
	}

	// Trim location last.
	payload.Location = nil
	data, err = json.Marshal(payload)
	if err != nil {
		return WrapHealthError(ErrPayloadTooLarge, "failed to serialize payload after trimming location", err)
	}
	if len(data) <= maxSize {
		return nil
	}

	// Mandatory fields alone exceed the limit.
	return NewHealthError(ErrPayloadTooLarge,
		fmt.Sprintf("mandatory fields alone (%d bytes) exceed maximum size %d", len(data), maxSize))
}

// PublishHeartbeat validates, trims, signs, and publishes a heartbeat payload.
//
// Steps:
//  1. Validate with ValidateOutboundHeartbeat
//  2. Trim payload to fit adapter's MaxMessageSize
//  3. Serialize to JSON
//  4. Create signed TopicMessage via topic.NewTopicMessage
//  5. Publish via adapter.Publish with FireAndForget mode
//
// No retry is attempted on publish failure (FR-H25).
func PublishHeartbeat(
	payload HeartbeatPayload,
	key *keylib.NeuronPrivateKey,
	stdOutRef topic.TopicRef,
	adapter topic.TopicAdapter,
	senderClock uint64,
	sequenceNumber uint64,
) (topic.PublishResult, error) {
	// Step 1: Validate.
	if err := ValidateOutboundHeartbeat(payload, senderClock); err != nil {
		return topic.PublishResult{}, err
	}

	// Step 2: Trim to fit backend size limit.
	maxSize := int(adapter.MaxMessageSize())
	if err := TrimPayload(&payload, maxSize); err != nil {
		return topic.PublishResult{}, err
	}

	// Step 3: Serialize payload to JSON.
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return topic.PublishResult{}, WrapHealthError(ErrPayloadTooLarge, "failed to serialize payload", err)
	}

	// Step 4: Create signed TopicMessage.
	msg, err := topic.NewTopicMessage(key, senderClock, sequenceNumber, payloadJSON)
	if err != nil {
		return topic.PublishResult{}, WrapHealthError(ErrInvalidPayloadType, "failed to create topic message", err)
	}

	// Step 5: Publish with FireAndForget.
	result, err := adapter.Publish(stdOutRef, msg, topic.PublishOpts{
		ConfirmationMode: topic.FireAndForget,
	})
	if err != nil {
		return topic.PublishResult{}, err
	}

	return result, nil
}

// ScheduleNextHeartbeat computes the wall-clock time at which the next heartbeat
// should be sent.
//
// If the publish result is confirmed and carries a consensus timestamp, that
// timestamp is used as the reference time. Otherwise, the submitWallClock
// (sender's local clock at submission time) is used.
//
// Returns referenceTime + chosenDelta.
func ScheduleNextHeartbeat(result topic.PublishResult, chosenDelta uint64, submitWallClock uint64) uint64 {
	var referenceTime uint64
	if result.Confirmed && result.ConsensusTimestamp != nil {
		referenceTime = *result.ConsensusTimestamp
	} else {
		referenceTime = submitWallClock
	}
	return referenceTime + chosenDelta
}

// HeartbeatPublisher manages rate-limited heartbeat publishing.
// It ensures that heartbeats are not published more frequently than MinDeadlineDelta.
//
// HeartbeatPublisher is safe for concurrent use.
type HeartbeatPublisher struct {
	key             *keylib.NeuronPrivateKey
	stdOutRef       topic.TopicRef
	adapter         topic.TopicAdapter
	lastPublishTime uint64
	sequenceNumber  uint64
	mu              sync.Mutex
}

// NewHeartbeatPublisher constructs a HeartbeatPublisher.
func NewHeartbeatPublisher(
	key *keylib.NeuronPrivateKey,
	stdOutRef topic.TopicRef,
	adapter topic.TopicAdapter,
) *HeartbeatPublisher {
	return &HeartbeatPublisher{
		key:       key,
		stdOutRef: stdOutRef,
		adapter:   adapter,
	}
}

// Publish validates and publishes a heartbeat, enforcing rate limiting.
// If senderClock - lastPublishTime < MinDeadlineDelta, the call is rejected
// with ErrRateLimited. On success, lastPublishTime is updated to senderClock.
//
// Concurrency note: The mutex is held for the entire publish operation, including
// the network call to the adapter. This serializes concurrent publish attempts.
// This is acceptable because rate-limiting already enforces MinDeadlineDelta (10s)
// gaps between publishes, making concurrent contention practically impossible.
func (p *HeartbeatPublisher) Publish(payload HeartbeatPayload, senderClock uint64) (topic.PublishResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Rate limit check: reject if publishing too frequently.
	if p.lastPublishTime > 0 && senderClock-p.lastPublishTime < MinDeadlineDelta {
		return topic.PublishResult{}, NewHealthError(ErrRateLimited,
			fmt.Sprintf("must wait at least %d seconds between publishes, elapsed %d",
				MinDeadlineDelta, senderClock-p.lastPublishTime))
	}

	p.sequenceNumber++
	result, err := PublishHeartbeat(payload, p.key, p.stdOutRef, p.adapter, senderClock, p.sequenceNumber)
	if err != nil {
		p.sequenceNumber--
		return topic.PublishResult{}, err
	}

	p.lastPublishTime = senderClock
	return result, nil
}
