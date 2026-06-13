package validation

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// ValidateInboundEvidence validates a received TopicMessage as an evidence envelope.
//
// Validation order:
//  1. Verify TopicMessage signature and sender address match
//  2. Unmarshal payload as JSON
//  3. Check type == "validationEvidence"
//  4. Check version compatibility (major must be 1)
//  5. Validate all mandatory fields present
//  6. Validate verdict is one of three allowed values
func ValidateInboundEvidence(msg topic.TopicMessage) (*EvidenceEnvelope, error) {
	// Step 1: Validate TopicMessage signature.
	if err := topic.ValidateTopicMessage(msg); err != nil {
		var topicErr topic.TopicError
		if errors.As(err, &topicErr) {
			switch topicErr.Kind {
			case topic.ErrSenderMismatch:
				return nil, WrapValidationError(ErrSenderAddressMismatch,
					"evidence sender address does not match recovered signer", err)
			default:
				return nil, WrapValidationError(ErrSignatureVerificationFailed,
					"evidence topic message signature verification failed", err)
			}
		}
		return nil, WrapValidationError(ErrSignatureVerificationFailed,
			"evidence topic message signature verification failed", err)
	}

	// Step 2: Unmarshal payload.
	var envelope EvidenceEnvelope
	if err := json.Unmarshal(msg.Payload(), &envelope); err != nil {
		return nil, WrapValidationError(ErrNotEvidenceMessage,
			"failed to parse evidence envelope from payload", err)
	}

	// Step 3: Check type discriminator. FR-V01.
	if envelope.envelopeType != PayloadTypeEvidence {
		return nil, NewValidationError(ErrNotEvidenceMessage,
			fmt.Sprintf("expected payload type %q, got %q", PayloadTypeEvidence, envelope.envelopeType))
	}

	// Step 4: Check version compatibility. FR-V07.
	if err := validateVersion(envelope.version); err != nil {
		return nil, err
	}

	// Step 5: Validate mandatory fields. FR-V02.
	if envelope.validatorAgentId == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "validatorAgentId is missing")
	}
	if envelope.subjectAgentId == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "subjectAgentId is missing")
	}
	if envelope.specRef == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "specRef is missing")
	}
	if envelope.evidenceHash == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "evidenceHash is missing")
	}
	if envelope.evidenceURI == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "evidenceURI is missing")
	}

	// Step 6: Validate verdict. FR-V04.
	if !IsValidVerdict(envelope.verdict) {
		return nil, NewValidationError(ErrInvalidVerdict,
			fmt.Sprintf("verdict %q is not valid", envelope.verdict))
	}

	return &envelope, nil
}
