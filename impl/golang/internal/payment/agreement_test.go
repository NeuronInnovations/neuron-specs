package payment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T008: AgreementState & State Machine Tests ---

func TestAgreementStateMachine_InitialState(t *testing.T) {
	// FR-P13: Initial state is IDLE.
	sm := NewAgreementStateMachine("req-001")
	assert.Equal(t, StateIdle, sm.State())
	assert.Equal(t, "req-001", sm.RequestID())
}

func TestAgreementStateMachine_AllValidTransitions(t *testing.T) {
	// FR-P14: All 13+ valid transitions.
	tests := []struct {
		name  string
		from  AgreementState
		event AgreementEvent
		to    AgreementState
	}{
		{"IDLE→REQUESTED", StateIdle, EventServiceRequest, StateRequested},
		{"REQUESTED→NEGOTIATING", StateRequested, EventCounter, StateNegotiating},
		{"REQUESTED→AGREED", StateRequested, EventAccept, StateAgreed},
		{"REQUESTED→REJECTED (reject)", StateRequested, EventReject, StateRejected},
		{"REQUESTED→REJECTED (deadline)", StateRequested, EventDeadlineExpired, StateRejected},
		{"NEGOTIATING→AGREED", StateNegotiating, EventAccept, StateAgreed},
		{"NEGOTIATING→REJECTED (reject)", StateNegotiating, EventReject, StateRejected},
		{"NEGOTIATING→REJECTED (withdraw)", StateNegotiating, EventWithdraw, StateRejected},
		{"NEGOTIATING→REJECTED (deadline)", StateNegotiating, EventDeadlineExpired, StateRejected},
		{"AGREED→FUNDED", StateAgreed, EventEscrowCreated, StateFunded},
		{"FUNDED→ACTIVE", StateFunded, EventDeliveryStarted, StateActive},
		{"ACTIVE→INVOICED", StateActive, EventInvoice, StateInvoiced},
		{"INVOICED→ACTIVE", StateInvoiced, EventInvoiceApproved, StateActive},
		{"INVOICED→TERMINATED (refused)", StateInvoiced, EventInvoiceRefused, StateTerminated},
		{"INVOICED→TERMINATED (timeout)", StateInvoiced, EventTimeout, StateTerminated},
		{"ACTIVE→TERMINATED", StateActive, EventTerminate, StateTerminated},
		{"ACTIVE→COMPLETED", StateActive, EventComplete, StateCompleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewAgreementStateMachine("test")
			sm.state = tt.from

			newState, err := sm.Transition(tt.event)
			require.NoError(t, err)
			assert.Equal(t, tt.to, newState)
			assert.Equal(t, tt.to, sm.State())
		})
	}
}

func TestAgreementStateMachine_RejectInvalidTransitions(t *testing.T) {
	// FR-P14: Invalid transitions must return error.
	tests := []struct {
		name  string
		from  AgreementState
		event AgreementEvent
	}{
		{"IDLE + ACCEPT", StateIdle, EventAccept},
		{"AGREED + INVOICE", StateAgreed, EventInvoice},
		{"FUNDED + ACCEPT", StateFunded, EventAccept},
		{"COMPLETED + anything", StateCompleted, EventServiceRequest},
		{"REJECTED + anything", StateRejected, EventAccept},
		{"TERMINATED + anything", StateTerminated, EventInvoice},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewAgreementStateMachine("test")
			sm.state = tt.from

			_, err := sm.Transition(tt.event)
			require.Error(t, err)
			var pe *PaymentError
			require.True(t, errors.As(err, &pe))
			assert.Equal(t, ErrNegotiationFailed, pe.Kind())
		})
	}
}

func TestAgreementStateMachine_FullLifecycle(t *testing.T) {
	// SC-P01, SC-P04: Full IDLE→...→COMPLETED cycle with 2 invoice rounds (FR-P15).
	sm := NewAgreementStateMachine("full-lifecycle")

	steps := []struct {
		event AgreementEvent
		state AgreementState
	}{
		{EventServiceRequest, StateRequested},
		{EventAccept, StateAgreed},
		{EventEscrowCreated, StateFunded},
		{EventDeliveryStarted, StateActive},
		// First invoice cycle
		{EventInvoice, StateInvoiced},
		{EventInvoiceApproved, StateActive},
		// Second invoice cycle (FR-P15 streaming)
		{EventInvoice, StateInvoiced},
		{EventInvoiceApproved, StateActive},
		// Complete
		{EventComplete, StateCompleted},
	}

	for _, step := range steps {
		newState, err := sm.Transition(step.event)
		require.NoError(t, err, "transition with event %s from %s", step.event, sm.State())
		assert.Equal(t, step.state, newState)
	}
}

func TestAgreementStateMachine_CheckDeadline(t *testing.T) {
	// FR-P07a: Deadline expiry auto-transitions to REJECTED.
	t.Run("expires in REQUESTED", func(t *testing.T) {
		sm := NewAgreementStateMachine("test")
		sm.state = StateRequested
		sm.SetNegotiationDeadline(1000)

		expired, err := sm.CheckDeadline(1001)
		require.NoError(t, err)
		assert.True(t, expired)
		assert.Equal(t, StateRejected, sm.State())
	})

	t.Run("expires in NEGOTIATING", func(t *testing.T) {
		sm := NewAgreementStateMachine("test")
		sm.state = StateNegotiating
		sm.SetNegotiationDeadline(1000)

		expired, err := sm.CheckDeadline(1001)
		require.NoError(t, err)
		assert.True(t, expired)
		assert.Equal(t, StateRejected, sm.State())
	})

	t.Run("not expired yet", func(t *testing.T) {
		sm := NewAgreementStateMachine("test")
		sm.state = StateRequested
		sm.SetNegotiationDeadline(1000)

		expired, err := sm.CheckDeadline(999)
		require.NoError(t, err)
		assert.False(t, expired)
		assert.Equal(t, StateRequested, sm.State())
	})

	t.Run("no deadline set", func(t *testing.T) {
		sm := NewAgreementStateMachine("test")
		sm.state = StateRequested

		expired, err := sm.CheckDeadline(9999)
		require.NoError(t, err)
		assert.False(t, expired)
	})

	t.Run("ignored in AGREED state", func(t *testing.T) {
		sm := NewAgreementStateMachine("test")
		sm.state = StateAgreed
		sm.SetNegotiationDeadline(1000)

		expired, err := sm.CheckDeadline(2000)
		require.NoError(t, err)
		assert.False(t, expired)
		assert.Equal(t, StateAgreed, sm.State())
	})
}

func TestComputeAgreementHash(t *testing.T) {
	// SC-P07: keccak256(canonicalJSON) produces consistent hash.
	input := []byte(`{"action":"accept","requestId":"550e8400","type":"serviceResponse","version":"1.0.0"}`)
	hash1 := ComputeAgreementHash(input)
	hash2 := ComputeAgreementHash(input)

	assert.Equal(t, hash1, hash2, "same input must produce same hash")
	assert.NotEqual(t, [32]byte{}, hash1, "hash must not be zero")
}
