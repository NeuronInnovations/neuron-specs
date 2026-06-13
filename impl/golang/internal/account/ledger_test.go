package account

import (
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Existing T006 tests: enum values, string representations, struct fields
// ---------------------------------------------------------------------------

func TestAttachmentState_Values(t *testing.T) {
	assert.Equal(t, AttachmentState(0), Detached)
	assert.Equal(t, AttachmentState(1), Attached)
}

func TestAttachmentState_String(t *testing.T) {
	assert.Equal(t, "Detached", Detached.String())
	assert.Equal(t, "Attached", Attached.String())
	assert.Equal(t, "Unknown", AttachmentState(99).String())
}

func TestVerificationStatus_Values(t *testing.T) {
	assert.Equal(t, VerificationStatus(0), Unverified)
	assert.Equal(t, VerificationStatus(1), Verified)
	assert.Equal(t, VerificationStatus(2), Failed)
}

func TestVerificationStatus_String(t *testing.T) {
	assert.Equal(t, "Unverified", Unverified.String())
	assert.Equal(t, "Verified", Verified.String())
	assert.Equal(t, "Failed", Failed.String())
	assert.Equal(t, "Unknown", VerificationStatus(99).String())
}

func TestLedgerAttachment_Fields(t *testing.T) {
	now := time.Now()
	pk, _ := keylib.NewNeuronPrivateKey()
	addr := pk.PublicKey().EVMAddress()

	la := LedgerAttachment{
		ledgerIdentifier:   "ethereum-mainnet",
		attachedAddress:    addr,
		state:              Attached,
		verificationStatus: Verified,
		lastSyncedAt:       &now,
	}

	assert.Equal(t, "ethereum-mainnet", la.ledgerIdentifier)
	assert.Equal(t, addr.Hex(), la.attachedAddress.Hex())
	assert.Equal(t, Attached, la.state)
	assert.Equal(t, Verified, la.verificationStatus)
	assert.NotNil(t, la.lastSyncedAt)
	assert.Equal(t, now, *la.lastSyncedAt)
}

func TestLedgerAttachment_NilLastSyncedAt(t *testing.T) {
	la := LedgerAttachment{
		ledgerIdentifier: "hedera-mainnet",
		state:            Detached,
	}
	assert.Nil(t, la.lastSyncedAt)
}

func TestVerificationResult_Fields(t *testing.T) {
	vr := VerificationResult{
		Status:  Verified,
		Message: "Account verified on ledger",
	}
	assert.Equal(t, Verified, vr.Status)
	assert.Equal(t, "Account verified on ledger", vr.Message)
}

// ---------------------------------------------------------------------------
// Mock verifiers for testing
// ---------------------------------------------------------------------------

type mockLedgerVerifier struct {
	accountResult  VerificationResult
	accountErr     error
	multisigResult VerificationResult
	multisigErr    error
}

func (m *mockLedgerVerifier) VerifyAccount(_ keylib.EVMAddress, _ string) (VerificationResult, error) {
	return m.accountResult, m.accountErr
}

func (m *mockLedgerVerifier) VerifyMultisigConfig(_ *keylib.MultisigKey, _ string) (VerificationResult, error) {
	return m.multisigResult, m.multisigErr
}

type mockParentChildVerifier struct {
	result VerificationResult
	err    error
}

func (m *mockParentChildVerifier) VerifyRelationship(_, _ keylib.EVMAddress) (VerificationResult, error) {
	return m.result, m.err
}

// ---------------------------------------------------------------------------
// T029: VerifyParentChild
// ---------------------------------------------------------------------------

func TestVerifyParentChild_Success(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	child, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	verifier := &mockParentChildVerifier{
		result: VerificationResult{
			Status:  Verified,
			Message: "parent-child relationship verified",
		},
	}

	result, err := VerifyParentChild(child, verifier)
	require.NoError(t, err)
	assert.Equal(t, Verified, result.Status)
	assert.Contains(t, result.Message, "verified")
}

func TestVerifyParentChild_Failed(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	child, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	verifier := &mockParentChildVerifier{
		result: VerificationResult{
			Status:  Failed,
			Message: "relationship not found on ledger",
		},
	}

	result, err := VerifyParentChild(child, verifier)
	require.NoError(t, err)
	assert.Equal(t, Failed, result.Status)
}

func TestVerifyParentChild_NilAccount(t *testing.T) {
	verifier := &mockParentChildVerifier{}
	_, err := VerifyParentChild(nil, verifier)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestVerifyParentChild_WrongAccountType(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	parent, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	verifier := &mockParentChildVerifier{}
	_, err = VerifyParentChild(parent, verifier)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Child")
}

// ---------------------------------------------------------------------------
// T040: VerifyLedgerAttachment
// ---------------------------------------------------------------------------

func TestVerifyLedgerAttachment_Parent_Success(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  pubKey.EVMAddress(),
		state:            Attached,
	}

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	verifier := &mockLedgerVerifier{
		accountResult: VerificationResult{
			Status:  Verified,
			Message: "account verified",
		},
	}

	result, err := acct.VerifyLedgerAttachment(verifier)
	require.NoError(t, err)
	assert.Equal(t, Verified, result.Status)

	// The verification status should be updated on the attachment
	assert.Equal(t, Verified, acct.LedgerAttachment().verificationStatus)
}

func TestVerifyLedgerAttachment_Child_Success(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  childPK.PublicKey().EVMAddress(),
		state:            Attached,
	}

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	verifier := &mockLedgerVerifier{
		accountResult: VerificationResult{
			Status:  Verified,
			Message: "account verified",
		},
	}

	result, err := acct.VerifyLedgerAttachment(verifier)
	require.NoError(t, err)
	assert.Equal(t, Verified, result.Status)
}

func TestVerifyLedgerAttachment_Shared_Success(t *testing.T) {
	mk := testMultisigKey(t)
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  pk.PublicKey().EVMAddress(),
		state:            Attached,
	}

	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	verifier := &mockLedgerVerifier{
		multisigResult: VerificationResult{
			Status:  Verified,
			Message: "multisig config verified",
		},
	}

	result, err := acct.VerifyLedgerAttachment(verifier)
	require.NoError(t, err)
	assert.Equal(t, Verified, result.Status)
}

func TestVerifyLedgerAttachment_NoAttachment(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	verifier := &mockLedgerVerifier{}
	_, err = acct.VerifyLedgerAttachment(verifier)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no ledger attachment")
}

func TestVerifyLedgerAttachment_Detached(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  pubKey.EVMAddress(),
		state:            Detached,
	}

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	verifier := &mockLedgerVerifier{}
	_, err = acct.VerifyLedgerAttachment(verifier)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not attached")
}

// ---------------------------------------------------------------------------
// T065: Attachment state transitions (AttachToLedger / DetachFromLedger)
// ---------------------------------------------------------------------------

func TestAttachToLedger(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	// Initially no attachment
	assert.Nil(t, acct.LedgerAttachment())

	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  pubKey.EVMAddress(),
	}

	acct.AttachToLedger(la)

	require.NotNil(t, acct.LedgerAttachment())
	assert.Equal(t, Attached, acct.LedgerAttachment().state)
	assert.Equal(t, "ethereum-mainnet", acct.LedgerAttachment().ledgerIdentifier)
}

func TestDetachFromLedger(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  pubKey.EVMAddress(),
		state:            Attached,
	}

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	require.NotNil(t, acct.LedgerAttachment())
	assert.Equal(t, Attached, acct.LedgerAttachment().state)

	acct.DetachFromLedger()

	require.NotNil(t, acct.LedgerAttachment())
	assert.Equal(t, Detached, acct.LedgerAttachment().state)
	assert.Equal(t, Unverified, acct.LedgerAttachment().verificationStatus)
	assert.Nil(t, acct.LedgerAttachment().lastSyncedAt)

	// Balance fields must be cleared
	assert.Nil(t, acct.CreditBalance())
	assert.Nil(t, acct.BalanceAllocation())
	assert.Nil(t, acct.Balance())
}

func TestAttachDetachCycle(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	// Attach
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  pubKey.EVMAddress(),
	}
	acct.AttachToLedger(la)
	assert.Equal(t, Attached, acct.LedgerAttachment().state)

	// Detach
	acct.DetachFromLedger()
	assert.Equal(t, Detached, acct.LedgerAttachment().state)

	// Re-attach
	la2 := LedgerAttachment{
		ledgerIdentifier: "hedera-mainnet",
		attachedAddress:  pubKey.EVMAddress(),
	}
	acct.AttachToLedger(la2)
	assert.Equal(t, Attached, acct.LedgerAttachment().state)
	assert.Equal(t, "hedera-mainnet", acct.LedgerAttachment().ledgerIdentifier)
}

func TestDetachFromLedger_NoAttachment(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	// Calling DetachFromLedger on an account with no attachment should not panic.
	assert.NotPanics(t, func() {
		acct.DetachFromLedger()
	})
}

func TestSetLedgerAttachment_Existing(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  pubKey.EVMAddress(),
		state:            Attached,
	}

	acct.SetLedgerAttachment(la)
	require.NotNil(t, acct.LedgerAttachment())
	assert.Equal(t, "ethereum-mainnet", acct.LedgerAttachment().ledgerIdentifier)
}
