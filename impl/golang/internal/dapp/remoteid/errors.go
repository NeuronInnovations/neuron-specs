package remoteid

import "fmt"

// ErrorKind identifies the category of a Remote ID DApp protocol error.
// Domain "NEURON-DAPP-REMOTEID-*" per 017 FR-R13.
type ErrorKind string

const (
	// ErrInvalidFilterParameter — wildcard altitude / radius parameter
	// failed to parse as a positive integer in the allowed range.
	// 017 FR-R04. Surfaced as 009 ErrUnknownProtocol over the wire.
	ErrInvalidFilterParameter ErrorKind = "InvalidFilterParameter"

	// ErrFrameMalformed — a RemoteIdFrame payload failed canonical-JSON
	// decoding, or required fields were missing.
	ErrFrameMalformed ErrorKind = "FrameMalformed"

	// ErrUnsupportedRegulatorVariant — incoming decoded frame's
	// regulatorVariant value is not one of the supported set
	// ("asd-stan", "asd-faa", "asd-easa"). Phase-2 decoders MAY accept
	// only a subset.
	ErrUnsupportedRegulatorVariant ErrorKind = "UnsupportedRegulatorVariant"

	// ErrUpstreamSourceUnavailable — the upstream Remote ID feed source
	// returned no frames within a configured stall window. Informational
	// only per 017 FR-R07: this error MUST NOT cause stream closure;
	// the data plane keeps the stream open and waits for the source to
	// recover. Surfaced as a log line, not a kill signal.
	ErrUpstreamSourceUnavailable ErrorKind = "UpstreamSourceUnavailable"

	// ErrFanoutTopicUnavailable — the seller's gossipsub fan-out topic
	// could not be joined. Phase-6 production work; not used in Phase 2.
	ErrFanoutTopicUnavailable ErrorKind = "FanoutTopicUnavailable"
)

// Error is the structured error type for the Remote ID DApp.
// Format: "remoteid.Operation: [Kind] descriptive message".
type Error struct {
	kind      ErrorKind
	operation string
	message   string
	cause     error
}

// New constructs a new Error.
func New(kind ErrorKind, operation, message string) *Error {
	return &Error{kind: kind, operation: operation, message: message}
}

// Wrap constructs a new Error wrapping the underlying cause.
func Wrap(kind ErrorKind, operation string, cause error) *Error {
	return &Error{kind: kind, operation: operation, message: cause.Error(), cause: cause}
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("remoteid.%s: [%s] %s", e.operation, e.kind, e.message)
}

// Kind returns the error category.
func (e *Error) Kind() ErrorKind { return e.kind }

// Operation returns the operation name where the error originated.
func (e *Error) Operation() string { return e.operation }

// Unwrap returns the wrapped cause, if any.
func (e *Error) Unwrap() error { return e.cause }

// Is supports errors.Is matching by kind.
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.kind == t.kind
	}
	return false
}
