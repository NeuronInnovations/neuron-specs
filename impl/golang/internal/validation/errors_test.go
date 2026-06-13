package validation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationError_Format(t *testing.T) {
	err := NewValidationError(ErrInvalidVerdict, "bad verdict value")
	assert.Equal(t, "validation: [InvalidVerdict] bad verdict value", err.Error())
}

func TestValidationError_Wrap(t *testing.T) {
	cause := fmt.Errorf("underlying failure")
	err := WrapValidationError(ErrHashMismatch, "hash does not match", cause)

	assert.Equal(t, "validation: [HashMismatch] hash does not match", err.Error())
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestValidationError_KindMatch(t *testing.T) {
	err := NewValidationError(ErrMissingRequiredField, "field X is required")
	var ve ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, ErrMissingRequiredField, ve.Kind)
}

func TestValidationError_NilCause(t *testing.T) {
	err := NewValidationError(ErrInvalidEnvelopeField, "bad field")
	assert.Nil(t, errors.Unwrap(err))
}
