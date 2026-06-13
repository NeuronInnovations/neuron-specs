package health

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthErrorKinds(t *testing.T) {
	t.Run("all 15 error kinds exist", func(t *testing.T) {
		// Publisher validation errors (8)
		assert.Equal(t, HealthErrorKind("InvalidPayloadType"), ErrInvalidPayloadType)
		assert.Equal(t, HealthErrorKind("UnsupportedVersion"), ErrUnsupportedVersion)
		assert.Equal(t, HealthErrorKind("DeadlineNotFuture"), ErrDeadlineNotFuture)
		assert.Equal(t, HealthErrorKind("DeadlineTooSoon"), ErrDeadlineTooSoon)
		assert.Equal(t, HealthErrorKind("DeadlineTooFar"), ErrDeadlineTooFar)
		assert.Equal(t, HealthErrorKind("UnrecognizedRole"), ErrUnrecognizedRole)
		assert.Equal(t, HealthErrorKind("PayloadTooLarge"), ErrPayloadTooLarge)
		assert.Equal(t, HealthErrorKind("RateLimited"), ErrRateLimited)

		// Observer validation errors (7)
		assert.Equal(t, HealthErrorKind("SignatureVerificationFailed"), ErrSignatureVerificationFailed)
		assert.Equal(t, HealthErrorKind("SenderAddressMismatch"), ErrSenderAddressMismatch)
		assert.Equal(t, HealthErrorKind("NotHeartbeatMessage"), ErrNotHeartbeatMessage)
		assert.Equal(t, HealthErrorKind("IncompatibleVersion"), ErrIncompatibleVersion)
		assert.Equal(t, HealthErrorKind("DeadlineInPast"), ErrDeadlineInPast)
		assert.Equal(t, HealthErrorKind("DeltaBelowMinimum"), ErrDeltaBelowMinimum)
		assert.Equal(t, HealthErrorKind("DeltaExceedsMaximum"), ErrDeltaExceedsMaximum)
	})

	t.Run("AllHealthErrorKinds returns all 15 kinds", func(t *testing.T) {
		kinds := AllHealthErrorKinds()
		require.Len(t, kinds, 15, "expected exactly 15 error kinds")

		// Verify all kinds are present
		kindSet := make(map[HealthErrorKind]bool)
		for _, k := range kinds {
			kindSet[k] = true
		}
		assert.True(t, kindSet[ErrInvalidPayloadType])
		assert.True(t, kindSet[ErrUnsupportedVersion])
		assert.True(t, kindSet[ErrDeadlineNotFuture])
		assert.True(t, kindSet[ErrDeadlineTooSoon])
		assert.True(t, kindSet[ErrDeadlineTooFar])
		assert.True(t, kindSet[ErrUnrecognizedRole])
		assert.True(t, kindSet[ErrPayloadTooLarge])
		assert.True(t, kindSet[ErrRateLimited])
		assert.True(t, kindSet[ErrSignatureVerificationFailed])
		assert.True(t, kindSet[ErrSenderAddressMismatch])
		assert.True(t, kindSet[ErrNotHeartbeatMessage])
		assert.True(t, kindSet[ErrIncompatibleVersion])
		assert.True(t, kindSet[ErrDeadlineInPast])
		assert.True(t, kindSet[ErrDeltaBelowMinimum])
		assert.True(t, kindSet[ErrDeltaExceedsMaximum])
	})
}

func TestHealthError(t *testing.T) {
	t.Run("Error() format is health: [Kind] Message", func(t *testing.T) {
		err := NewHealthError(ErrInvalidPayloadType, "expected heartbeat")
		assert.Equal(t, `health: [InvalidPayloadType] expected heartbeat`, err.Error())
	})

	t.Run("Error() format with different kinds", func(t *testing.T) {
		tests := []struct {
			kind    HealthErrorKind
			message string
			want    string
		}{
			{ErrDeadlineTooSoon, "delta is 5s", "health: [DeadlineTooSoon] delta is 5s"},
			{ErrSenderAddressMismatch, "recovered != sender", "health: [SenderAddressMismatch] recovered != sender"},
			{ErrRateLimited, "too fast", "health: [RateLimited] too fast"},
		}
		for _, tt := range tests {
			err := NewHealthError(tt.kind, tt.message)
			assert.Equal(t, tt.want, err.Error())
		}
	})

	t.Run("Unwrap returns nil when no cause", func(t *testing.T) {
		err := NewHealthError(ErrInvalidPayloadType, "test")
		assert.Nil(t, err.Unwrap())
	})

	t.Run("Unwrap returns cause when wrapped", func(t *testing.T) {
		cause := fmt.Errorf("underlying error")
		err := WrapHealthError(ErrSignatureVerificationFailed, "sig check failed", cause)
		assert.Equal(t, cause, err.Unwrap())
	})

	t.Run("errors.As works with HealthError", func(t *testing.T) {
		he := NewHealthError(ErrDeadlineInPast, "deadline was in the past")
		// Wrap in a generic error
		wrapped := fmt.Errorf("validation failed: %w", he)

		var target HealthError
		require.True(t, errors.As(wrapped, &target))
		assert.Equal(t, ErrDeadlineInPast, target.Kind)
		assert.Equal(t, "deadline was in the past", target.Message)
	})

	t.Run("errors.As works with wrapped cause chain", func(t *testing.T) {
		rootCause := fmt.Errorf("network timeout")
		he := WrapHealthError(ErrSignatureVerificationFailed, "could not verify", rootCause)
		wrapped := fmt.Errorf("observer error: %w", he)

		var target HealthError
		require.True(t, errors.As(wrapped, &target))
		assert.Equal(t, ErrSignatureVerificationFailed, target.Kind)
		assert.Equal(t, rootCause, target.Unwrap())
	})

	t.Run("HealthError implements error interface", func(t *testing.T) {
		var err error = NewHealthError(ErrPayloadTooLarge, "too big")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "PayloadTooLarge")
	})
}

func TestNewHealthError(t *testing.T) {
	t.Run("constructs error with kind and message", func(t *testing.T) {
		err := NewHealthError(ErrUnrecognizedRole, "role 'miner' is not valid")
		assert.Equal(t, ErrUnrecognizedRole, err.Kind)
		assert.Equal(t, "role 'miner' is not valid", err.Message)
		assert.Nil(t, err.Cause)
	})
}

func TestWrapHealthError(t *testing.T) {
	t.Run("constructs error with kind, message, and cause", func(t *testing.T) {
		cause := fmt.Errorf("crypto error")
		err := WrapHealthError(ErrSignatureVerificationFailed, "ECDSA verify failed", cause)
		assert.Equal(t, ErrSignatureVerificationFailed, err.Kind)
		assert.Equal(t, "ECDSA verify failed", err.Message)
		assert.Equal(t, cause, err.Cause)
	})
}
