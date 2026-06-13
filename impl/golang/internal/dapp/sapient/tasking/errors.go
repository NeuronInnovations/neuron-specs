package tasking

import (
	"errors"
	"fmt"
)

// ErrorKind classifies a tasking failure (domain NEURON-DAPP-SAPIENT-TASKING-*).
type ErrorKind string

const (
	// ErrLane — the auditable lane subscribe/publish failed.
	ErrLane ErrorKind = "Lane"
	// ErrConfig — the Manager was constructed with an invalid configuration.
	ErrConfig ErrorKind = "Config"
)

// Error is the structured error type for the tasking package. Format:
// "tasking.Operation: [Kind] message".
type Error struct {
	kind      ErrorKind
	operation string
	message   string
	cause     error
}

// New builds a tasking error with no wrapped cause.
func New(kind ErrorKind, operation, message string) *Error {
	return &Error{kind: kind, operation: operation, message: message}
}

// Wrap builds a tasking error around an underlying cause.
func Wrap(kind ErrorKind, operation string, cause error) *Error {
	return &Error{kind: kind, operation: operation, message: cause.Error(), cause: cause}
}

func (e *Error) Error() string {
	return fmt.Sprintf("tasking.%s: [%s] %s", e.operation, e.kind, e.message)
}

// Kind returns the error classification.
func (e *Error) Kind() ErrorKind { return e.kind }

// Unwrap exposes the wrapped cause.
func (e *Error) Unwrap() error { return e.cause }

// Is matches by ErrorKind.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return t.kind == e.kind
}
