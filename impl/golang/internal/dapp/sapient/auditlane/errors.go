package auditlane

import (
	"errors"
	"fmt"
)

// ErrorKind classifies an auditlane failure (domain NEURON-DAPP-SAPIENT-AUDITLANE-*).
type ErrorKind string

const (
	// ErrLaneClosed — operation attempted on a closed lane.
	ErrLaneClosed ErrorKind = "LaneClosed"
	// ErrEncode — a SapientMessage could not be (un)marshalled for the lane.
	ErrEncode ErrorKind = "Encode"
	// ErrIO — an underlying file/transport read or write failed.
	ErrIO ErrorKind = "IO"
)

// Error is the structured error type for the auditlane package. Format:
// "auditlane.Operation: [Kind] message".
type Error struct {
	kind      ErrorKind
	operation string
	message   string
	cause     error
}

// New builds an auditlane error with no wrapped cause.
func New(kind ErrorKind, operation, message string) *Error {
	return &Error{kind: kind, operation: operation, message: message}
}

// Wrap builds an auditlane error around an underlying cause.
func Wrap(kind ErrorKind, operation string, cause error) *Error {
	return &Error{kind: kind, operation: operation, message: cause.Error(), cause: cause}
}

func (e *Error) Error() string {
	return fmt.Sprintf("auditlane.%s: [%s] %s", e.operation, e.kind, e.message)
}

// Kind returns the error classification.
func (e *Error) Kind() ErrorKind { return e.kind }

// Unwrap exposes the wrapped cause for errors.Is/As.
func (e *Error) Unwrap() error { return e.cause }

// Is matches by ErrorKind so callers can errors.Is(err, &Error{kind: ...}).
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return t.kind == e.kind
}
