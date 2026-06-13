package remoteid

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Test_FR_P09_EscrowCreatedRoundTrip — buyer publishes EscrowCreated;
// seller receives, decoded payload identical.
func Test_FR_P09_EscrowCreatedRoundTrip(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	buyerKey := newTestKey(t)
	adapter, sellerStdIn, _ := setupMemoryTopic(t, "escrow-rt")

	req, _ := BuildServiceRequest(ServiceRequestOptions{RequestID: "req-esc", BuyerStdIn: "x"})

	type recv struct {
		msg payment.EscrowCreated
		err error
	}
	resultCh := make(chan recv, 1)
	go func() {
		got, _, err := ReceiveEscrowCreated(ctx, adapter, sellerStdIn, "req-esc")
		resultCh <- recv{got, err}
	}()
	time.Sleep(20 * time.Millisecond)

	created := BuildEscrowCreated(req, "memory-escrow-1", "0", "USDC")
	_, err := PublishEscrowCreated(ctx, adapter, sellerStdIn, buyerKey, 1, created)
	require.NoError(t, err)

	r := <-resultCh
	require.NoError(t, r.err)
	assert.Equal(t, payment.PayloadEscrowCreated, r.msg.Type)
	assert.Equal(t, "req-esc", r.msg.RequestID)
	assert.Equal(t, "memory-escrow-1", r.msg.EscrowRef)
	assert.Equal(t, "0", r.msg.DepositAmount)
	assert.Equal(t, "USDC", r.msg.DepositCurrency)
}

// Test_FR_P36_ServiceStopFlow — buyer signals end-of-stream via
// ServiceStop; seller receives.
func Test_FR_P36_ServiceStopFlow(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	buyerKey := newTestKey(t)
	adapter, sellerStdIn, _ := setupMemoryTopic(t, "stop-rt")

	req, _ := BuildServiceRequest(ServiceRequestOptions{RequestID: "req-stop", BuyerStdIn: "x"})

	type recv struct {
		msg payment.ServiceStop
		err error
	}
	resultCh := make(chan recv, 1)
	go func() {
		got, _, err := ReceiveServiceStop(ctx, adapter, sellerStdIn, "req-stop")
		resultCh <- recv{got, err}
	}()
	time.Sleep(20 * time.Millisecond)

	stop := BuildServiceStop(req, "frame-limit-reached", 0)
	_, err := PublishServiceStop(ctx, adapter, sellerStdIn, buyerKey, 1, stop)
	require.NoError(t, err)

	r := <-resultCh
	require.NoError(t, r.err)
	assert.Equal(t, payment.PayloadServiceStop, r.msg.Type)
	assert.Equal(t, "req-stop", r.msg.RequestID)
	assert.Equal(t, "frame-limit-reached", r.msg.Reason)
	assert.Equal(t, uint64(0), r.msg.EffectiveAt, "EffectiveAt=0 means immediate")
}

// Test_FR_P10_InvoiceRoundTrip — seller publishes Invoice; buyer receives.
func Test_FR_P10_InvoiceRoundTrip(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	sellerKey := newTestKey(t)
	adapter, buyerStdIn, _ := setupMemoryTopic(t, "inv-rt")

	req, _ := BuildServiceRequest(ServiceRequestOptions{RequestID: "req-inv", BuyerStdIn: "x"})

	type recv struct {
		msg payment.Invoice
		err error
	}
	resultCh := make(chan recv, 1)
	go func() {
		got, _, err := ReceiveInvoice(ctx, adapter, buyerStdIn, "req-inv")
		resultCh <- recv{got, err}
	}()
	time.Sleep(20 * time.Millisecond)

	inv, err := BuildInvoice(InvoiceOptions{
		Request:           req,
		EscrowRef:         "memory-escrow-1",
		ReleaseRequestRef: "release-1",
		EvidenceHash:      "deadbeef",
	})
	require.NoError(t, err)
	_, err = PublishInvoice(ctx, adapter, buyerStdIn, sellerKey, 1, inv)
	require.NoError(t, err)

	r := <-resultCh
	require.NoError(t, r.err)
	assert.Equal(t, payment.PayloadInvoice, r.msg.Type)
	assert.Equal(t, "req-inv", r.msg.RequestID)
	assert.Equal(t, "memory-escrow-1", r.msg.EscrowRef)
	assert.Equal(t, "release-1", r.msg.ReleaseRequestRef)
	assert.Equal(t, "1", r.msg.Amount, "defaults to req.ProposedAmount (Stage-2 minimum positive unit)")
	assert.Equal(t, "USDC", r.msg.Currency, "defaults to req.ProposedCurrency")
	assert.Equal(t, "PT0S", r.msg.Period, "defaults to one-shot")
}

// Test_FR_P10_InvoiceRequiresEscrowAndReleaseRef — BuildInvoice rejects
// missing required fields.
func Test_FR_P10_InvoiceRequiresEscrowAndReleaseRef(t *testing.T) {
	t.Parallel()
	req, _ := BuildServiceRequest(ServiceRequestOptions{RequestID: "x", BuyerStdIn: "x"})
	_, err := BuildInvoice(InvoiceOptions{Request: req, ReleaseRequestRef: "r"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EscrowRef required")

	_, err = BuildInvoice(InvoiceOptions{Request: req, EscrowRef: "e"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ReleaseRequestRef required")
}

// Test_FR_P11_InvoiceAckRoundTrip — buyer approves; seller receives.
func Test_FR_P11_InvoiceAckRoundTrip(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	buyerKey := newTestKey(t)
	adapter, sellerStdIn, _ := setupMemoryTopic(t, "ack-rt")

	inv := payment.Invoice{
		Type:              payment.PayloadInvoice,
		Version:           PayloadVersion,
		RequestID:         "req-ack",
		ReleaseRequestRef: "release-1",
		EscrowRef:         "memory-escrow-1",
		Amount:            "0",
		Currency:          "USDC",
		Period:            "PT0S",
	}

	type recv struct {
		msg payment.InvoiceAck
		err error
	}
	resultCh := make(chan recv, 1)
	go func() {
		got, _, err := ReceiveInvoiceAck(ctx, adapter, sellerStdIn, "req-ack")
		resultCh <- recv{got, err}
	}()
	time.Sleep(20 * time.Millisecond)

	ack := BuildInvoiceAck(inv, "approved")
	_, err := PublishInvoiceAck(ctx, adapter, sellerStdIn, buyerKey, 1, ack)
	require.NoError(t, err)

	r := <-resultCh
	require.NoError(t, r.err)
	assert.Equal(t, payment.PayloadInvoiceAck, r.msg.Type)
	assert.Equal(t, "req-ack", r.msg.RequestID)
	assert.Equal(t, "release-1", r.msg.ReleaseRequestRef)
	assert.Equal(t, "approved", r.msg.Action)
}

// TestEvidenceHashFor_Deterministic checks the D7 frame-summary hash is
// stable for identical inputs and differs for varying inputs.
func TestEvidenceHashFor_Deterministic(t *testing.T) {
	t.Parallel()
	a := EvidenceHashFor("12D3KooTest", 100, 1000, 2000)
	b := EvidenceHashFor("12D3KooTest", 100, 1000, 2000)
	c := EvidenceHashFor("12D3KooTest", 101, 1000, 2000)
	d := EvidenceHashFor("12D3KooDifferent", 100, 1000, 2000)
	zero := EvidenceHashFor("", 0, 0, 0)

	assert.Equal(t, a, b, "identical inputs produce identical hashes")
	assert.NotEqual(t, a, c, "frame-count delta changes hash")
	assert.NotEqual(t, a, d, "peerID delta changes hash")
	assert.NotEqual(t, a, zero)
	assert.Len(t, a, 64, "hex of SHA-256 is 64 chars")
	assert.Len(t, zero, 64)
}

// Ensure the topic import is referenced — silences an unused-import nag
// if the test file later loses one of the explicit references.
var _ topic.TopicAdapter
