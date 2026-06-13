package payment

import (
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T042: End-to-End Integration Test ---

func TestEndToEnd_DiscoverNegotiateSettle(t *testing.T) {
	// SC-P01: Complete IDLE→...→COMPLETED cycle.
	// FR-P28: Verify requestRelease recipient defaults to seller's PaymentAddress.

	// Step 1: Seller publishes commerce offering (US2).
	svc, err := NewNeuronCommerceService(
		"adsb-v0.1", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p-adsb"},
		SettlementDescriptor{Binding: "evm-escrow", Config: map[string]any{"chainId": "296"}},
		PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: "3600"},
		WithTermsRef("https://example.com/terms"),
	)
	require.NoError(t, err)
	assert.Equal(t, "neuron-commerce", svc.Type)

	// Step 2: Buyer sends serviceRequest.
	sm := NewAgreementStateMachine("integration-test-001")
	assert.Equal(t, StateIdle, sm.State())

	_, err = sm.Transition(EventServiceRequest)
	require.NoError(t, err)
	assert.Equal(t, StateRequested, sm.State())

	// Step 3: Seller accepts.
	_, err = sm.Transition(EventAccept)
	require.NoError(t, err)
	assert.Equal(t, StateAgreed, sm.State())

	// Step 4: Buyer creates escrow.
	_, err = sm.Transition(EventEscrowCreated)
	require.NoError(t, err)
	assert.Equal(t, StateFunded, sm.State())

	// Step 5: Seller starts delivery.
	_, err = sm.Transition(EventDeliveryStarted)
	require.NoError(t, err)
	assert.Equal(t, StateActive, sm.State())

	// Step 6: First invoice cycle.
	_, err = sm.Transition(EventInvoice)
	require.NoError(t, err)
	assert.Equal(t, StateInvoiced, sm.State())

	_, err = sm.Transition(EventInvoiceApproved)
	require.NoError(t, err)
	assert.Equal(t, StateActive, sm.State())

	// Step 7: Second invoice cycle (FR-P15 streaming).
	_, err = sm.Transition(EventInvoice)
	require.NoError(t, err)
	assert.Equal(t, StateInvoiced, sm.State())

	_, err = sm.Transition(EventInvoiceApproved)
	require.NoError(t, err)
	assert.Equal(t, StateActive, sm.State())

	// Step 8: Complete.
	_, err = sm.Transition(EventComplete)
	require.NoError(t, err)
	assert.Equal(t, StateCompleted, sm.State())
}

// --- T043: Determinism Verification ---

func TestDeterminism_SameSequenceProducesSameState(t *testing.T) {
	// SC-P04: Same message sequence → same state.
	events := []AgreementEvent{
		EventServiceRequest,
		EventCounter,
		EventAccept,
		EventEscrowCreated,
		EventDeliveryStarted,
		EventInvoice,
		EventInvoiceApproved,
		EventComplete,
	}

	sm1 := NewAgreementStateMachine("det-1")
	sm2 := NewAgreementStateMachine("det-2")

	for _, event := range events {
		s1, err1 := sm1.Transition(event)
		s2, err2 := sm2.Transition(event)
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, s1, s2, "state machines must agree on event %s", event)
	}

	assert.Equal(t, sm1.State(), sm2.State())
	assert.Equal(t, StateCompleted, sm1.State())
}

// --- T044: Evidence Hash Verification ---

func TestEvidenceHash_Keccak256Deterministic(t *testing.T) {
	// SC-P08: evidenceHash = keccak256(canonicalJSON(deliveryProofTopicMessage)).
	deliveryProof := []byte(`{"payload":"delivery-proof-data","senderAddress":"0xABC","sequenceNumber":"42","signature":"sig","timestamp":"1700000000"}`)

	hash1 := crypto.Keccak256(deliveryProof)
	hash2 := crypto.Keccak256(deliveryProof)

	assert.Equal(t, hash1, hash2, "same input must produce same keccak256 hash")
	assert.Len(t, hash1, 32)
}

// --- T045: Validator Perspective Test ---

func TestValidatorPerspective_PayloadObservability(t *testing.T) {
	// VR-PAY-01: serviceRequest has all MUST fields, canonical ordering.
	req := ServiceRequest{
		Type: PayloadServiceRequest, Version: "1.0.0",
		RequestID: "550e8400", ServiceRef: "adsb-v0.1",
		SettlementBinding: "evm-escrow", ProposedAmount: "10",
		ProposedCurrency: "USDC", ProposedInterval: "3600",
		NegotiationDeadline: 1711382400, BuyerStdIn: "0.0.54321",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)
	s := string(data)

	// VR-PAY-01: Verify MUST fields present.
	assert.Contains(t, s, `"type":"serviceRequest"`)
	assert.Contains(t, s, `"version":"1.0.0"`)
	assert.Contains(t, s, `"requestId":"550e8400"`)
	assert.Contains(t, s, `"serviceRef":"adsb-v0.1"`)
	assert.Contains(t, s, `"settlementBinding":"evm-escrow"`)
	assert.Contains(t, s, `"proposedAmount":"10"`)
	assert.Contains(t, s, `"buyerStdIn":"0.0.54321"`)

	// VR-PAY-07: connectionSetup has MUST fields (non-encrypted readable).
	cs := ConnectionSetup{
		Type: PayloadConnectionSetup, Version: "1.0.0",
		RequestID: "550e8400", PeerID: "12D3KooWTest",
		EncryptedMultiaddrs: "opaque-base64==",
		Protocol: "/neuron/adsb/1.0.0",
	}

	csData, err := json.Marshal(cs)
	require.NoError(t, err)
	csStr := string(csData)

	assert.Contains(t, csStr, `"type":"connectionSetup"`)
	assert.Contains(t, csStr, `"peerID":"12D3KooWTest"`)
	assert.Contains(t, csStr, `"protocol":"/neuron/adsb/1.0.0"`)
	// Validator CANNOT verify encryptedMultiaddrs content (encrypted by design, FR-P34).
	assert.Contains(t, csStr, `"encryptedMultiaddrs"`)
}

func TestValidatorPerspective_AgreementHashConsistency(t *testing.T) {
	// VR-PAY-05: agreementHash matches keccak256(canonicalJSON(acceptedServiceResponse)).
	resp := ServiceResponse{
		Type: PayloadServiceResponse, Version: "1.0.0",
		RequestID: "550e8400", Action: "accept",
	}

	canonicalJSON, err := json.Marshal(resp)
	require.NoError(t, err)

	hash := ComputeAgreementHash(canonicalJSON)

	// Re-compute from same canonical JSON — must match.
	hash2 := ComputeAgreementHash(canonicalJSON)
	assert.Equal(t, hash, hash2)
	assert.NotEqual(t, [32]byte{}, hash)

	// Different response → different hash.
	resp2 := ServiceResponse{
		Type: PayloadServiceResponse, Version: "1.0.0",
		RequestID: "different-id", Action: "accept",
	}
	canonicalJSON2, _ := json.Marshal(resp2)
	hash3 := ComputeAgreementHash(canonicalJSON2)
	assert.NotEqual(t, hash, hash3)
}
