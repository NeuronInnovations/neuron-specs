package topic

import "fmt"

// FR-T11: Structured error types
// TopicErrorKind enumerates the structured error categories for the topic system.
// Each kind corresponds to a specific failure mode defined in FR-T11.
type TopicErrorKind string

const (
	// ErrBackendUnavailable indicates the backend transport is unreachable or unhealthy.
	ErrBackendUnavailable TopicErrorKind = "BackendUnavailable"
	// ErrTopicNotFound indicates the referenced topic does not exist on the backend.
	ErrTopicNotFound TopicErrorKind = "TopicNotFound"
	// ErrMessageTooLarge indicates the message payload exceeds the backend's maximum size.
	ErrMessageTooLarge TopicErrorKind = "MessageTooLarge"
	// ErrUnsupportedOperation indicates the operation is not supported by this adapter
	// (e.g., publish on a read-only ERC event log adapter).
	ErrUnsupportedOperation TopicErrorKind = "UnsupportedOperation"
	// ErrInvalidSignature indicates the message signature failed verification.
	ErrInvalidSignature TopicErrorKind = "InvalidSignature"
	// ErrSenderMismatch indicates the recovered signer address does not match senderAddress.
	ErrSenderMismatch TopicErrorKind = "SenderMismatch"
	// ErrUnsupportedTransport indicates the BackendKind is not recognized or registered.
	ErrUnsupportedTransport TopicErrorKind = "UnsupportedTransport"
	// ErrInvalidTopicRef indicates the TopicRef is malformed or fails validation.
	ErrInvalidTopicRef TopicErrorKind = "InvalidTopicRef"
	// ErrReservedChannelName indicates an attempt to use a reserved channel name
	// (stdIn, stdOut, stdErr) without the standard role type.
	ErrReservedChannelName TopicErrorKind = "ReservedChannelName"
	// ErrInvalidConfig indicates a configuration parameter is invalid or missing.
	ErrInvalidConfig TopicErrorKind = "InvalidConfig"
	// ErrBrokenTopicRef indicates a TopicRef cross-reference points to a non-existent topic.
	ErrBrokenTopicRef TopicErrorKind = "BrokenTopicRef"
)

// AllTopicErrorKinds returns all defined TopicErrorKind values.
func AllTopicErrorKinds() []TopicErrorKind {
	return []TopicErrorKind{
		ErrBackendUnavailable,
		ErrTopicNotFound,
		ErrMessageTooLarge,
		ErrUnsupportedOperation,
		ErrInvalidSignature,
		ErrSenderMismatch,
		ErrUnsupportedTransport,
		ErrInvalidTopicRef,
		ErrReservedChannelName,
		ErrInvalidConfig,
		ErrBrokenTopicRef,
	}
}

// TopicError is a structured error type for the topic system.
// It carries a Kind discriminator, a human-readable Message, and an optional
// BackendError for wrapping underlying transport errors.
type TopicError struct {
	Kind         TopicErrorKind
	Message      string
	BackendError error
}

// Error implements the error interface. Format: "topic: [Kind] Message".
func (e TopicError) Error() string {
	return fmt.Sprintf("topic: [%s] %s", e.Kind, e.Message)
}

// Unwrap returns the underlying BackendError, enabling errors.Is/As chain traversal.
func (e TopicError) Unwrap() error {
	return e.BackendError
}

// NewTopicError constructs a TopicError with no wrapped cause.
func NewTopicError(kind TopicErrorKind, message string) TopicError {
	return TopicError{
		Kind:    kind,
		Message: message,
	}
}

// WrapTopicError constructs a TopicError that wraps an underlying cause error.
func WrapTopicError(kind TopicErrorKind, message string, cause error) TopicError {
	return TopicError{
		Kind:         kind,
		Message:      message,
		BackendError: cause,
	}
}
