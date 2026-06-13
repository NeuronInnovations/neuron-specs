package health

import "fmt"

// HealthErrorKind enumerates the structured error categories for the health system.
// Each kind corresponds to a specific validation rule from the spec.
type HealthErrorKind string

// Publisher validation errors (V-PUB-01..07):
const (
	// ErrInvalidPayloadType indicates the payload type field is not "heartbeat". V-PUB-01.
	ErrInvalidPayloadType HealthErrorKind = "InvalidPayloadType"
	// ErrUnsupportedVersion indicates the version field is not a supported version. V-PUB-02.
	ErrUnsupportedVersion HealthErrorKind = "UnsupportedVersion"
	// ErrDeadlineNotFuture indicates nextHeartbeatDeadline is not in the future. V-PUB-04.
	ErrDeadlineNotFuture HealthErrorKind = "DeadlineNotFuture"
	// ErrDeadlineTooSoon indicates the delta between now and deadline is below MinDeadlineDelta. V-PUB-05.
	ErrDeadlineTooSoon HealthErrorKind = "DeadlineTooSoon"
	// ErrDeadlineTooFar indicates the delta between now and deadline exceeds MaxDeadlineDelta. V-PUB-06.
	ErrDeadlineTooFar HealthErrorKind = "DeadlineTooFar"
	// ErrUnrecognizedRole indicates the role field is not a known NodeRole. V-PUB-07.
	ErrUnrecognizedRole HealthErrorKind = "UnrecognizedRole"
	// ErrPayloadTooLarge indicates the serialized payload exceeds the size limit. FR-H29.
	ErrPayloadTooLarge HealthErrorKind = "PayloadTooLarge"
	// ErrRateLimited indicates a publish attempt was rejected due to rate limiting. FR-H14.
	ErrRateLimited HealthErrorKind = "RateLimited"
)

// Observer validation errors (V-OBS-01..06):
const (
	// ErrSignatureVerificationFailed indicates the signature could not be verified. V-OBS-01.
	ErrSignatureVerificationFailed HealthErrorKind = "SignatureVerificationFailed"
	// ErrSenderAddressMismatch indicates the recovered signer address does not match senderAddress. V-OBS-01.
	ErrSenderAddressMismatch HealthErrorKind = "SenderAddressMismatch"
	// ErrNotHeartbeatMessage indicates the payload type is not "heartbeat". V-OBS-02.
	ErrNotHeartbeatMessage HealthErrorKind = "NotHeartbeatMessage"
	// ErrIncompatibleVersion indicates the version major number is not 1. V-OBS-03.
	ErrIncompatibleVersion HealthErrorKind = "IncompatibleVersion"
	// ErrDeadlineInPast indicates nextHeartbeatDeadline is in the past relative to consensus. V-OBS-05.
	ErrDeadlineInPast HealthErrorKind = "DeadlineInPast"
	// ErrDeltaBelowMinimum indicates the delta is below MinDeadlineDelta. V-OBS-06.
	ErrDeltaBelowMinimum HealthErrorKind = "DeltaBelowMinimum"
	// ErrDeltaExceedsMaximum indicates the delta exceeds MaxDeadlineDelta. V-OBS-06.
	ErrDeltaExceedsMaximum HealthErrorKind = "DeltaExceedsMaximum"
)

// AllHealthErrorKinds returns all defined HealthErrorKind values.
func AllHealthErrorKinds() []HealthErrorKind {
	return []HealthErrorKind{
		// Publisher errors
		ErrInvalidPayloadType,
		ErrUnsupportedVersion,
		ErrDeadlineNotFuture,
		ErrDeadlineTooSoon,
		ErrDeadlineTooFar,
		ErrUnrecognizedRole,
		ErrPayloadTooLarge,
		ErrRateLimited,
		// Observer errors
		ErrSignatureVerificationFailed,
		ErrSenderAddressMismatch,
		ErrNotHeartbeatMessage,
		ErrIncompatibleVersion,
		ErrDeadlineInPast,
		ErrDeltaBelowMinimum,
		ErrDeltaExceedsMaximum,
	}
}

// HealthError is a structured error type for the health system.
// It carries a Kind discriminator, a human-readable Message, and an optional
// Cause for wrapping underlying errors.
type HealthError struct {
	Kind    HealthErrorKind
	Message string
	Cause   error
}

// Error implements the error interface. Format: "health: [Kind] Message".
func (e HealthError) Error() string {
	return fmt.Sprintf("health: [%s] %s", e.Kind, e.Message)
}

// Unwrap returns the underlying Cause, enabling errors.Is/As chain traversal.
func (e HealthError) Unwrap() error {
	return e.Cause
}

// NewHealthError constructs a HealthError with no wrapped cause.
func NewHealthError(kind HealthErrorKind, message string) HealthError {
	return HealthError{
		Kind:    kind,
		Message: message,
	}
}

// WrapHealthError constructs a HealthError that wraps an underlying cause error.
func WrapHealthError(kind HealthErrorKind, message string, cause error) HealthError {
	return HealthError{
		Kind:    kind,
		Message: message,
		Cause:   cause,
	}
}
