package delivery

import "fmt"

// DeliveryErrorKind identifies the category of a delivery protocol error.
// FR-D29: NEURON-DELIVERY-* domain following 006 error taxonomy.
type DeliveryErrorKind string

const (
	// ErrDialFailed — all dial attempts exhausted. FR-D02.
	ErrDialFailed DeliveryErrorKind = "DialFailed"

	// ErrStreamError — stream I/O failure. FR-D03.
	ErrStreamError DeliveryErrorKind = "StreamError"

	// ErrRelayError — relay connection failed or unavailable. FR-D20.
	ErrRelayError DeliveryErrorKind = "RelayError"

	// ErrPeerIDMismatch — remote PeerID does not match connectionSetup. FR-D28.
	ErrPeerIDMismatch DeliveryErrorKind = "PeerIDMismatch"

	// ErrNoCompatibleTransport — no multiaddr matches configured transports. FR-D25.
	ErrNoCompatibleTransport DeliveryErrorKind = "NoCompatibleTransport"

	// ErrInvalidMultiaddr — decrypted multiaddrs are malformed. FR-D15.
	ErrInvalidMultiaddr DeliveryErrorKind = "InvalidMultiaddr"

	// ErrChannelClosed — operation on closed channel. FR-D05.
	ErrChannelClosed DeliveryErrorKind = "ChannelClosed"

	// ErrFrameTooLarge — frame exceeds 4 MiB limit. FR-D22.
	ErrFrameTooLarge DeliveryErrorKind = "FrameTooLarge"

	// ErrBackoffExhausted — max reconnection duration exceeded. FR-D09.
	ErrBackoffExhausted DeliveryErrorKind = "BackoffExhausted"

	// ErrConnectionSetupEncryptionFailed — ECIES decryption failed. FR-D14.
	// Shared with 008 FR-P32.
	ErrConnectionSetupEncryptionFailed DeliveryErrorKind = "ConnectionSetupEncryptionFailed"

	// ErrStreamDirectionViolation — a party opened a stream whose declared
	// direction excludes that party. FR-D-stream-direction (added 2026-05-08
	// per 008 FR-P33a + 009 amendment).
	ErrStreamDirectionViolation DeliveryErrorKind = "StreamDirectionViolation"

	// ErrUnknownStreamDirection — a streams[] catalog entry declared a
	// direction value that is not one of "seller-initiates",
	// "buyer-initiates", or "either". FR-D-stream-direction.
	ErrUnknownStreamDirection DeliveryErrorKind = "UnknownStreamDirection"
)

// DeliveryError is the structured error type for Spec 009.
// Format: "delivery.Operation: [Kind] descriptive message"
type DeliveryError struct {
	kind      DeliveryErrorKind
	operation string
	message   string
	cause     error
}

// NewDeliveryError creates a DeliveryError with the given kind, operation, and message.
func NewDeliveryError(kind DeliveryErrorKind, operation, message string) *DeliveryError {
	return &DeliveryError{kind: kind, operation: operation, message: message}
}

// WrapDeliveryError creates a DeliveryError that wraps an underlying cause.
func WrapDeliveryError(kind DeliveryErrorKind, operation string, cause error) *DeliveryError {
	return &DeliveryError{kind: kind, operation: operation, message: cause.Error(), cause: cause}
}

// Error implements the error interface.
func (e *DeliveryError) Error() string {
	return fmt.Sprintf("delivery.%s: [%s] %s", e.operation, e.kind, e.message)
}

// Kind returns the error category.
func (e *DeliveryError) Kind() DeliveryErrorKind {
	return e.kind
}

// Operation returns the operation that produced this error.
func (e *DeliveryError) Operation() string {
	return e.operation
}

// Unwrap returns the underlying cause, if any.
func (e *DeliveryError) Unwrap() error {
	return e.cause
}

// Is supports errors.Is matching by kind.
func (e *DeliveryError) Is(target error) bool {
	if t, ok := target.(*DeliveryError); ok {
		return e.kind == t.kind
	}
	return false
}
