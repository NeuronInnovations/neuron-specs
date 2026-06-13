package remoteid

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
)

// Test_FR_P13_AgreementLifecycle proves the full-commerce path drives the
// state machine through IDLE → REQUESTED → AGREED → FUNDED → ACTIVE →
// INVOICED → ACTIVE → COMPLETED.
func Test_FR_P13_AgreementLifecycle(t *testing.T) {
	t.Run("full-commerce-path", TestLifecycle_FullCommercePath)
	t.Run("rejects-skipped-transition", TestLifecycle_RejectsSkippedTransition)
	t.Run("logs-each-transition", TestLifecycle_LogsEachTransition)
}

// Test_FR_P58_RegistrationOnlyShortCircuit proves the registration-only
// mode stops at AGREED and refuses funded/invoice transitions.
func Test_FR_P58_RegistrationOnlyShortCircuit(t *testing.T) {
	t.Run("agreed-state-after-accept", TestLifecycle_RegistrationOnlyStopsAtAgreed)
	t.Run("rejects-funded", TestLifecycle_RegistrationOnlyRejectsFunded)
	t.Run("rejects-invoice", TestLifecycle_RegistrationOnlyRejectsInvoice)
}

// --- Underlying tests ---

func newLifecycleForTest(t *testing.T, mode string) *Lifecycle {
	t.Helper()
	lc, err := NewLifecycle(LifecycleOptions{
		RequestID: "lf-test",
		Mode:      mode,
		Logger:    log.New(&bytes.Buffer{}, "", 0),
	})
	require.NoError(t, err)
	return lc
}

func TestLifecycle_FullCommercePath(t *testing.T) {
	t.Parallel()
	lc := newLifecycleForTest(t, CommerceModeFull)

	require.NoError(t, lc.Receive(payment.ServiceRequest{}))
	assert.Equal(t, payment.StateRequested, lc.State())

	require.NoError(t, lc.Accept())
	assert.Equal(t, payment.StateAgreed, lc.State())

	require.NoError(t, lc.Funded())
	assert.Equal(t, payment.StateFunded, lc.State())

	require.NoError(t, lc.StartDelivery())
	assert.Equal(t, payment.StateActive, lc.State())

	require.NoError(t, lc.BeginInvoice())
	assert.Equal(t, payment.StateInvoiced, lc.State())

	require.NoError(t, lc.ApproveInvoice())
	assert.Equal(t, payment.StateActive, lc.State(),
		"008 ApproveInvoice returns ACTIVE (subscription model); seller fires Complete next")

	require.NoError(t, lc.Complete())
	assert.Equal(t, payment.StateCompleted, lc.State())
}

func TestLifecycle_RejectsSkippedTransition(t *testing.T) {
	t.Parallel()
	lc := newLifecycleForTest(t, CommerceModeFull)
	// Cannot fund before requesting + agreeing.
	require.Error(t, lc.Funded(), "Funded() from IDLE must fail")
	require.NoError(t, lc.Receive(payment.ServiceRequest{}))
	require.Error(t, lc.Funded(), "Funded() from REQUESTED must fail")
}

func TestLifecycle_LogsEachTransition(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lc, err := NewLifecycle(LifecycleOptions{
		RequestID: "lf-log",
		Mode:      CommerceModeFull,
		Logger:    log.New(&buf, "", 0),
	})
	require.NoError(t, err)

	require.NoError(t, lc.Receive(payment.ServiceRequest{}))
	require.NoError(t, lc.Accept())
	require.NoError(t, lc.Funded())
	require.NoError(t, lc.StartDelivery())
	require.NoError(t, lc.BeginInvoice())
	require.NoError(t, lc.ApproveInvoice())
	require.NoError(t, lc.Complete())

	logs := buf.String()
	assert.Contains(t, logs, "IDLE→REQUESTED")
	assert.Contains(t, logs, "REQUESTED→AGREED")
	assert.Contains(t, logs, "AGREED→FUNDED")
	assert.Contains(t, logs, "FUNDED→ACTIVE")
	assert.Contains(t, logs, "ACTIVE→INVOICED")
	assert.Contains(t, logs, "INVOICED→ACTIVE")
	assert.Contains(t, logs, "ACTIVE→COMPLETED")
	assert.Contains(t, logs, "requestID=lf-log")
}

func TestLifecycle_RegistrationOnlyStopsAtAgreed(t *testing.T) {
	t.Parallel()
	lc := newLifecycleForTest(t, CommerceModeRegistrationOnly)
	require.NoError(t, lc.Receive(payment.ServiceRequest{}))
	require.NoError(t, lc.Accept())
	assert.Equal(t, payment.StateAgreed, lc.State())
}

func TestLifecycle_RegistrationOnlyRejectsFunded(t *testing.T) {
	t.Parallel()
	lc := newLifecycleForTest(t, CommerceModeRegistrationOnly)
	require.NoError(t, lc.Receive(payment.ServiceRequest{}))
	require.NoError(t, lc.Accept())
	err := lc.Funded()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registration-only")
}

func TestLifecycle_RegistrationOnlyRejectsInvoice(t *testing.T) {
	t.Parallel()
	lc := newLifecycleForTest(t, CommerceModeRegistrationOnly)
	require.NoError(t, lc.Receive(payment.ServiceRequest{}))
	require.NoError(t, lc.Accept())
	require.Error(t, lc.BeginInvoice())
	require.Error(t, lc.ApproveInvoice())
	require.Error(t, lc.Complete())
}

func TestLifecycle_RequiresRequestID(t *testing.T) {
	t.Parallel()
	_, err := NewLifecycle(LifecycleOptions{Mode: CommerceModeFull})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RequestID required")
}

func TestLifecycle_RejectsUnknownMode(t *testing.T) {
	t.Parallel()
	_, err := NewLifecycle(LifecycleOptions{RequestID: "x", Mode: "weird"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mode")
}
