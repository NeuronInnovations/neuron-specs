package adsb

import (
	"errors"
	"fmt"
)

// ErrorKind enumerates ADS-B DApp domain errors per the
// NEURON-DAPP-ADSB-* namespace (spec 006 error taxonomy).
type ErrorKind string

const (
	// ErrFrameMalformed indicates a NormalizedTrack payload failed schema
	// validation (missing required field, type mismatch, out-of-range value).
	ErrFrameMalformed ErrorKind = "NEURON-DAPP-ADSB-FrameMalformed"

	// ErrInvalidEntityType indicates entityType is not in the v1 closed
	// enum {"aircraft", "drone"}. Aircraft DApp emits "aircraft" only;
	// "drone" is reserved for the Remote ID basestation path.
	ErrInvalidEntityType ErrorKind = "NEURON-DAPP-ADSB-InvalidEntityType"

	// ErrUpstreamSourceUnavailable indicates the BaseStation TCP source
	// dropped — informational only; does NOT close streams per 016 FR-A08
	// (mirrors remoteid's FR-R07 long-lived stream discipline).
	ErrUpstreamSourceUnavailable ErrorKind = "NEURON-DAPP-ADSB-UpstreamSourceUnavailable"
)

// Error is the structured error returned by this package.
type Error struct {
	Kind      ErrorKind
	Operation string
	Message   string
	Cause     error
}

// New constructs a fresh Error with a literal message.
func New(kind ErrorKind, operation, message string) *Error {
	return &Error{Kind: kind, Operation: operation, Message: message}
}

// Wrap constructs an Error wrapping a downstream cause.
func Wrap(kind ErrorKind, operation string, cause error) *Error {
	if cause == nil {
		return nil
	}
	return &Error{Kind: kind, Operation: operation, Cause: cause}
}

// Error implements error.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("adsb.%s: [%s] %s", e.Operation, e.Kind, e.Cause.Error())
	}
	return fmt.Sprintf("adsb.%s: [%s] %s", e.Operation, e.Kind, e.Message)
}

// Unwrap implements errors.Unwrap.
func (e *Error) Unwrap() error { return e.Cause }

// Is implements errors.Is by matching ErrorKind.
func (e *Error) Is(target error) bool {
	var other *Error
	if errors.As(target, &other) {
		return e.Kind == other.Kind
	}
	return false
}
