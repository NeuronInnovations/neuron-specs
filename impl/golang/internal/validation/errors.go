package validation

import "fmt"

// ValidationErrorKind enumerates the structured error categories for the validation framework.
// Each kind corresponds to a specific validation rule from the spec.
type ValidationErrorKind string

// Envelope construction and field validation errors:
const (
	// ErrInvalidVerdict indicates the verdict is not one of the three allowed values. FR-V04.
	ErrInvalidVerdict ValidationErrorKind = "InvalidVerdict"
	// ErrInvalidEnvelopeField indicates an envelope field has an invalid format. FR-V02.
	ErrInvalidEnvelopeField ValidationErrorKind = "InvalidEnvelopeField"
	// ErrHashMismatch indicates an evidence hash does not match the expected value. FR-V05.
	ErrHashMismatch ValidationErrorKind = "HashMismatch"
	// ErrMissingRequiredField indicates a mandatory envelope field is absent. FR-V02.
	ErrMissingRequiredField ValidationErrorKind = "MissingRequiredField"
	// ErrIncompatibleVersion indicates the version major number is not 1. FR-V07.
	ErrIncompatibleVersion ValidationErrorKind = "IncompatibleVersion"
	// ErrInvalidSpecRef indicates the specRef field has an invalid format. FR-V24.
	ErrInvalidSpecRef ValidationErrorKind = "InvalidSpecRef"
)

// Inbound evidence validation errors (observer-side):
const (
	// ErrSignatureVerificationFailed indicates the TopicMessage signature is invalid.
	ErrSignatureVerificationFailed ValidationErrorKind = "SignatureVerificationFailed"
	// ErrSenderAddressMismatch indicates the recovered signer does not match senderAddress.
	ErrSenderAddressMismatch ValidationErrorKind = "SenderAddressMismatch"
	// ErrNotEvidenceMessage indicates the payload type is not "validationEvidence".
	ErrNotEvidenceMessage ValidationErrorKind = "NotEvidenceMessage"
)

// Validator service errors:
const (
	// ErrInvalidDomains indicates the domains array is empty or contains invalid entries.
	ErrInvalidDomains ValidationErrorKind = "InvalidDomains"
	// ErrInvalidVerdictDelivery indicates verdictDelivery is not "topic".
	ErrInvalidVerdictDelivery ValidationErrorKind = "InvalidVerdictDelivery"
	// ErrInvalidServiceType indicates the service type is not "neuron-validator".
	ErrInvalidServiceType ValidationErrorKind = "InvalidServiceType"
)

// ValidationError is a structured error type for the validation framework.
// It carries a Kind discriminator, a human-readable Message, and an optional
// Cause for wrapping underlying errors.
type ValidationError struct {
	Kind    ValidationErrorKind
	Message string
	Cause   error
}

// Error implements the error interface. Format: "validation: [Kind] Message".
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation: [%s] %s", e.Kind, e.Message)
}

// Unwrap returns the underlying Cause, enabling errors.Is/As chain traversal.
func (e ValidationError) Unwrap() error {
	return e.Cause
}

// NewValidationError constructs a ValidationError with no wrapped cause.
func NewValidationError(kind ValidationErrorKind, message string) ValidationError {
	return ValidationError{
		Kind:    kind,
		Message: message,
	}
}

// WrapValidationError constructs a ValidationError that wraps an underlying cause error.
func WrapValidationError(kind ValidationErrorKind, message string, cause error) ValidationError {
	return ValidationError{
		Kind:    kind,
		Message: message,
		Cause:   cause,
	}
}
