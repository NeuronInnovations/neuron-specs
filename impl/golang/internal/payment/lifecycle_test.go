package payment

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FR-P36: ServiceStop minimal payload — only mandatory fields.
func TestServiceStop_CanonicalJSON_Minimal(t *testing.T) {
	t.Parallel()

	stop := ServiceStop{
		Type:      PayloadServiceStop,
		Version:   "1.0.0",
		RequestID: "01234567-89ab-cdef-0123-456789abcdef",
	}

	got, err := json.Marshal(stop)
	require.NoError(t, err)

	const want = `{"type":"serviceStop","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef"}`
	assert.Equal(t, want, string(got))
}

// FR-P36: ServiceStop full payload — with reason and effectiveAt.
func TestServiceStop_CanonicalJSON_Full(t *testing.T) {
	t.Parallel()

	stop := ServiceStop{
		Type:        PayloadServiceStop,
		Version:     "1.0.0",
		RequestID:   "01234567-89ab-cdef-0123-456789abcdef",
		Reason:      "buyer-finished",
		EffectiveAt: 1704153600000000000,
	}

	got, err := json.Marshal(stop)
	require.NoError(t, err)

	const want = `{"type":"serviceStop","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","reason":"buyer-finished","effectiveAt":"1704153600000000000"}`
	assert.Equal(t, want, string(got))
}

func TestServiceStop_UnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	original := ServiceStop{
		Type:        PayloadServiceStop,
		Version:     "1.0.0",
		RequestID:   "abc-def",
		Reason:      "test",
		EffectiveAt: 42,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ServiceStop
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

// FR-P37: ServiceCancel minimal payload.
func TestServiceCancel_CanonicalJSON_Minimal(t *testing.T) {
	t.Parallel()

	cancel := ServiceCancel{
		Type:      PayloadServiceCancel,
		Version:   "1.0.0",
		RequestID: "abc",
	}

	got, err := json.Marshal(cancel)
	require.NoError(t, err)

	const want = `{"type":"serviceCancel","version":"1.0.0","requestId":"abc"}`
	assert.Equal(t, want, string(got))
}

func TestServiceCancel_CanonicalJSON_WithRefundRequested(t *testing.T) {
	t.Parallel()

	cancel := ServiceCancel{
		Type:            PayloadServiceCancel,
		Version:         "1.0.0",
		RequestID:       "abc",
		Reason:          "buyer-aborting-test",
		RefundRequested: true,
	}

	got, err := json.Marshal(cancel)
	require.NoError(t, err)

	const want = `{"type":"serviceCancel","version":"1.0.0","requestId":"abc","reason":"buyer-aborting-test","refundRequested":true}`
	assert.Equal(t, want, string(got))
}

func TestServiceCancel_UnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	original := ServiceCancel{
		Type:            PayloadServiceCancel,
		Version:         "1.0.0",
		RequestID:       "abc",
		RefundRequested: true,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ServiceCancel
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

// FR-P38: ServiceRenew payload.
func TestServiceRenew_CanonicalJSON(t *testing.T) {
	t.Parallel()

	renew := ServiceRenew{
		Type:        PayloadServiceRenew,
		Version:     "1.0.0",
		RequestID:   "abc",
		ExtendUntil: 1704153600000000000,
	}

	got, err := json.Marshal(renew)
	require.NoError(t, err)

	const want = `{"type":"serviceRenew","version":"1.0.0","requestId":"abc","extendUntil":"1704153600000000000"}`
	assert.Equal(t, want, string(got))
}

func TestServiceRenew_WithReason(t *testing.T) {
	t.Parallel()

	renew := ServiceRenew{
		Type:        PayloadServiceRenew,
		Version:     "1.0.0",
		RequestID:   "abc",
		ExtendUntil: 1704153600000000000,
		Reason:      "extending-for-soak",
	}

	got, err := json.Marshal(renew)
	require.NoError(t, err)

	const want = `{"type":"serviceRenew","version":"1.0.0","requestId":"abc","extendUntil":"1704153600000000000","reason":"extending-for-soak"}`
	assert.Equal(t, want, string(got))
}

func TestServiceRenew_UnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	original := ServiceRenew{
		Type:        PayloadServiceRenew,
		Version:     "1.0.0",
		RequestID:   "abc",
		ExtendUntil: 1704153600000000000,
		Reason:      "x",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ServiceRenew
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

// FR-P14 (amended): ACTIVE + EventStop → TERMINATED.
func TestAgreement_ActiveStopTransitionsToTerminated(t *testing.T) {
	t.Parallel()

	sm := NewAgreementStateMachine("req-001")
	// Drive to ACTIVE.
	_, err := sm.Transition(EventServiceRequest)
	require.NoError(t, err)
	_, err = sm.Transition(EventAccept)
	require.NoError(t, err)
	_, err = sm.Transition(EventEscrowCreated)
	require.NoError(t, err)
	_, err = sm.Transition(EventDeliveryStarted)
	require.NoError(t, err)
	require.Equal(t, StateActive, sm.State())

	// EventStop should transition ACTIVE → TERMINATED.
	newState, err := sm.Transition(EventStop)
	require.NoError(t, err)
	assert.Equal(t, StateTerminated, newState)
}

// FR-P14 (amended): EventCancel from any pre-COMPLETED state → TERMINATED.
func TestAgreement_CancelFromVariousStates(t *testing.T) {
	t.Parallel()

	type setup func(*AgreementStateMachine)

	cases := map[string]setup{
		"from REQUESTED": func(sm *AgreementStateMachine) {
			_, _ = sm.Transition(EventServiceRequest)
		},
		"from NEGOTIATING": func(sm *AgreementStateMachine) {
			_, _ = sm.Transition(EventServiceRequest)
			_, _ = sm.Transition(EventCounter)
		},
		"from AGREED": func(sm *AgreementStateMachine) {
			_, _ = sm.Transition(EventServiceRequest)
			_, _ = sm.Transition(EventAccept)
		},
		"from FUNDED": func(sm *AgreementStateMachine) {
			_, _ = sm.Transition(EventServiceRequest)
			_, _ = sm.Transition(EventAccept)
			_, _ = sm.Transition(EventEscrowCreated)
		},
		"from ACTIVE": func(sm *AgreementStateMachine) {
			_, _ = sm.Transition(EventServiceRequest)
			_, _ = sm.Transition(EventAccept)
			_, _ = sm.Transition(EventEscrowCreated)
			_, _ = sm.Transition(EventDeliveryStarted)
		},
		"from INVOICED": func(sm *AgreementStateMachine) {
			_, _ = sm.Transition(EventServiceRequest)
			_, _ = sm.Transition(EventAccept)
			_, _ = sm.Transition(EventEscrowCreated)
			_, _ = sm.Transition(EventDeliveryStarted)
			_, _ = sm.Transition(EventInvoice)
		},
	}

	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sm := NewAgreementStateMachine("req")
			prepare(sm)
			newState, err := sm.Transition(EventCancel)
			require.NoError(t, err, "unexpected error transitioning from %s", sm.State())
			assert.Equal(t, StateTerminated, newState)
		})
	}
}

// EventStop is invalid in REQUESTED state (must use EventCancel pre-AGREED).
func TestAgreement_StopFromRequestedRejected(t *testing.T) {
	t.Parallel()

	sm := NewAgreementStateMachine("req")
	_, _ = sm.Transition(EventServiceRequest)
	require.Equal(t, StateRequested, sm.State())

	_, err := sm.Transition(EventStop)
	assert.Error(t, err)
}
