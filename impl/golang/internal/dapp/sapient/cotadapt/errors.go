package cotadapt

import (
	"errors"
	"fmt"
)

// ErrorKind classifies a cotadapt failure (domain NEURON-DAPP-SAPIENT-COTADAPT-*).
type ErrorKind string

const (
	// ErrNoDetection — the SapientMessage carries no DetectionReport.
	ErrNoDetection ErrorKind = "NoDetection"
	// ErrNoIdentity — the DetectionReport has neither id nor object_id.
	ErrNoIdentity ErrorKind = "NoIdentity"
	// ErrNoLocation — the DetectionReport has no Location (cannot place a CoT point).
	ErrNoLocation ErrorKind = "NoLocation"
	// ErrEncode — XML marshalling failed.
	ErrEncode ErrorKind = "Encode"
)

// Error is the structured error type for the cotadapt package. Format:
// "cotadapt.Operation: [Kind] message".
type Error struct {
	kind      ErrorKind
	operation string
	message   string
	cause     error
}

// New builds a cotadapt error with no wrapped cause.
func New(kind ErrorKind, operation, message string) *Error {
	return &Error{kind: kind, operation: operation, message: message}
}

// Wrap builds a cotadapt error around an underlying cause.
func Wrap(kind ErrorKind, operation string, cause error) *Error {
	return &Error{kind: kind, operation: operation, message: cause.Error(), cause: cause}
}

func (e *Error) Error() string {
	return fmt.Sprintf("cotadapt.%s: [%s] %s", e.operation, e.kind, e.message)
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
