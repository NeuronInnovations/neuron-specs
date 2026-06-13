package validation

import (
	"encoding/json"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// PublishEvidence validates, serializes, signs, and publishes an evidence envelope.
//
// Steps:
//  1. Serialize envelope to canonical JSON
//  2. Create signed TopicMessage via topic.NewTopicMessage
//  3. Publish via adapter.Publish with WaitForConsensus mode (FR-V17 SHOULD)
//
// FR-V16: Evidence envelopes are published to the validator's stdOut.
func PublishEvidence(
	envelope *EvidenceEnvelope,
	key *keylib.NeuronPrivateKey,
	stdOutRef topic.TopicRef,
	adapter topic.TopicAdapter,
	senderClock uint64,
	sequenceNumber uint64,
) (topic.PublishResult, error) {
	// Step 1: Serialize to canonical JSON.
	payloadJSON, err := json.Marshal(envelope)
	if err != nil {
		return topic.PublishResult{}, WrapValidationError(ErrInvalidEnvelopeField,
			"failed to serialize evidence envelope", err)
	}

	// Step 2: Create signed TopicMessage.
	msg, err := topic.NewTopicMessage(key, senderClock, sequenceNumber, payloadJSON)
	if err != nil {
		return topic.PublishResult{}, WrapValidationError(ErrInvalidEnvelopeField,
			"failed to create topic message for evidence", err)
	}

	// Step 3: Publish with WaitForConsensus. FR-V17.
	result, err := adapter.Publish(stdOutRef, msg, topic.PublishOpts{
		ConfirmationMode: topic.WaitForConsensus,
	})
	if err != nil {
		return topic.PublishResult{}, err
	}

	return result, nil
}
