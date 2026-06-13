package account

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helper: create a valid key pair and DID for builder tests
// ---------------------------------------------------------------------------

func testKeyAndDID(t *testing.T) (keylib.NeuronPublicKey, NeuronDID) {
	t.Helper()
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pubKey := pk.PublicKey()
	did, err := GenerateDID(pubKey)
	require.NoError(t, err)
	return pubKey, did
}

func testLedgerAttachment(t *testing.T, pubKey keylib.NeuronPublicKey) LedgerAttachment {
	t.Helper()
	return LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  pubKey.EVMAddress(),
		state:            Attached,
	}
}

// ===========================================================================
// T015-T018: ParentAccountBuilder
// ===========================================================================

func TestParentAccountBuilder_Build_Success(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()

	require.NoError(t, err)
	require.NotNil(t, acct)

	assert.Equal(t, Parent, acct.AccountType())
	assert.NotNil(t, acct.PublicKey())
	assert.Equal(t, pubKey.Hex(), acct.PublicKey().Hex())
	assert.NotNil(t, acct.DID())
	assert.Equal(t, did.identifier, acct.DID().identifier)
	assert.Equal(t, "ETH", acct.CurrencySymbol())

	// Derived fields
	assert.NotNil(t, acct.EVMAddress())
	assert.Equal(t, pubKey.EVMAddress().Hex(), acct.EVMAddress().Hex())
	assert.NotNil(t, acct.PeerID())

	// Fields that MUST NOT be set for Parent
	assert.Nil(t, acct.ParentPublicKey(), "Parent must not have parentPubKey")
	assert.Nil(t, acct.MultisigKey(), "Parent must not have multisigKey")
	assert.Nil(t, acct.P2PHost(), "Parent must not have p2pHost")

	// Balance fields are always nil at construction
	assert.Nil(t, acct.CreditBalance())
	assert.Nil(t, acct.BalanceAllocation())
	assert.Nil(t, acct.Balance())
}

func TestParentAccountBuilder_Build_MissingPublicKey(t *testing.T) {
	_, did := testKeyAndDID(t)

	_, err := NewParentAccountBuilder().
		WithDID(did).
		WithCurrency("ETH").
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-PARENT-02", ve.RuleCode)
	assert.Equal(t, "publicKey", ve.Field)
}

func TestParentAccountBuilder_Build_MissingDID(t *testing.T) {
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	_, err = NewParentAccountBuilder().
		WithPublicKey(pk.PublicKey()).
		WithCurrency("ETH").
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-PARENT-01", ve.RuleCode)
	assert.Equal(t, "did", ve.Field)
}

func TestParentAccountBuilder_Build_MissingCurrency(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	_, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-PARENT-03", ve.RuleCode)
	assert.Equal(t, "currencySymbol", ve.Field)
}

func TestParentAccountBuilder_Build_EmptyDID(t *testing.T) {
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	_, err = NewParentAccountBuilder().
		WithPublicKey(pk.PublicKey()).
		WithDID(NeuronDID{}).
		WithCurrency("ETH").
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-PARENT-01", ve.RuleCode)
}

func TestParentAccountBuilder_BuildComplete_Success(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	la := testLedgerAttachment(t, pubKey)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		BuildComplete()

	require.NoError(t, err)
	require.NotNil(t, acct)
	assert.NotNil(t, acct.LedgerAttachment())
	assert.Equal(t, "ethereum-mainnet", acct.LedgerAttachment().ledgerIdentifier)
}

func TestParentAccountBuilder_BuildComplete_MissingLedger(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	_, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		BuildComplete()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-PARENT-06", ve.RuleCode)
	assert.Equal(t, "ledgerAttachment", ve.Field)
}

func TestParentAccountBuilder_FluentChain(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	// Verify fluent chain returns non-nil builder at each step
	builder := NewParentAccountBuilder()
	require.NotNil(t, builder)

	b1 := builder.WithPublicKey(pubKey)
	require.NotNil(t, b1)
	assert.Same(t, builder, b1)

	b2 := b1.WithDID(did)
	require.NotNil(t, b2)
	assert.Same(t, builder, b2)

	b3 := b2.WithCurrency("HBAR")
	require.NotNil(t, b3)
	assert.Same(t, builder, b3)
}

// ===========================================================================
// T024-T028: ChildAccountBuilder
// ===========================================================================

func TestChildAccountBuilder_Build_Success(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		Build()

	require.NoError(t, err)
	require.NotNil(t, acct)

	assert.Equal(t, Child, acct.AccountType())
	assert.NotNil(t, acct.PublicKey())
	assert.Equal(t, childPK.PublicKey().Hex(), acct.PublicKey().Hex())
	assert.NotNil(t, acct.ParentPublicKey())
	assert.Equal(t, parentPK.PublicKey().Hex(), acct.ParentPublicKey().Hex())
	assert.Equal(t, "ETH", acct.CurrencySymbol())

	// Derived fields
	assert.NotNil(t, acct.EVMAddress())
	assert.NotNil(t, acct.PeerID())

	// p2pHost = peerID for Child
	assert.NotNil(t, acct.P2PHost())
	assert.Equal(t, acct.PeerID().String(), acct.P2PHost().String(),
		"p2pHost must equal peerID for Child accounts")

	// Fields that MUST NOT be set for Child
	assert.Nil(t, acct.DID(), "Child must not have DID")
	assert.Nil(t, acct.MultisigKey(), "Child must not have multisigKey")

	// Balance fields are always nil at construction
	assert.Nil(t, acct.CreditBalance())
	assert.Nil(t, acct.BalanceAllocation())
	assert.Nil(t, acct.Balance())
}

func TestChildAccountBuilder_Build_MissingPublicKey(t *testing.T) {
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	_, err = NewChildAccountBuilder().
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-CHILD-02", ve.RuleCode)
	assert.Equal(t, "publicKey", ve.Field)
}

func TestChildAccountBuilder_Build_MissingParentKey(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	_, err = NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithCurrency("ETH").
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-CHILD-01", ve.RuleCode)
	assert.Equal(t, "parentPubKey", ve.Field)
}

func TestChildAccountBuilder_Build_MissingCurrency(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	_, err = NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-CHILD-03", ve.RuleCode)
	assert.Equal(t, "currencySymbol", ve.Field)
}

func TestChildAccountBuilder_Build_WithOptionalFields(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "42",
	}
	fp := LedgerAccountId("0.0.12345")

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("HBAR").
		WithRegistryBinding(rb).
		WithFeePayer(fp).
		Build()

	require.NoError(t, err)
	require.NotNil(t, acct)
	assert.NotNil(t, acct.RegistryBinding())
	assert.Equal(t, "42", acct.RegistryBinding().externalID)
	assert.NotNil(t, acct.FeePayer())
	assert.Equal(t, LedgerAccountId("0.0.12345"), *acct.FeePayer())
}

func TestChildAccountBuilder_BuildComplete_Success(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "42",
	}
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  childPK.PublicKey().EVMAddress(),
		state:            Attached,
	}

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		WithRegistryBinding(rb).
		WithLedgerAttachment(la).
		BuildComplete()

	require.NoError(t, err)
	require.NotNil(t, acct)
	assert.NotNil(t, acct.LedgerAttachment())
	assert.NotNil(t, acct.RegistryBinding())
}

func TestChildAccountBuilder_BuildComplete_MissingLedger(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "42",
	}

	_, err = NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		WithRegistryBinding(rb).
		BuildComplete()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-CHILD-06", ve.RuleCode)
}

func TestChildAccountBuilder_BuildComplete_MissingRegistry(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  childPK.PublicKey().EVMAddress(),
		state:            Attached,
	}

	_, err = NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		BuildComplete()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-CHILD-07", ve.RuleCode)
}

func TestChildAccountBuilder_FluentChain(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	builder := NewChildAccountBuilder()
	b1 := builder.WithPublicKey(childPK.PublicKey())
	assert.Same(t, builder, b1)

	b2 := b1.WithParentPublicKey(parentPK.PublicKey())
	assert.Same(t, builder, b2)

	b3 := b2.WithCurrency("ETH")
	assert.Same(t, builder, b3)
}

// ===========================================================================
// T047-T050: SharedAccountBuilder
// ===========================================================================

func testMultisigKey(t *testing.T) *keylib.MultisigKey {
	t.Helper()
	pk1, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pk2, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	mk, err := keylib.NewMultisigKey([]keylib.NeuronPrivateKey{pk1, pk2}, 2)
	require.NoError(t, err)
	return &mk
}

func TestSharedAccountBuilder_Build_Success(t *testing.T) {
	mk := testMultisigKey(t)

	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()

	require.NoError(t, err)
	require.NotNil(t, acct)

	assert.Equal(t, Shared, acct.AccountType())
	assert.NotNil(t, acct.MultisigKey())
	assert.Equal(t, "secp256k1-aggregated", acct.MultisigKey().Protocol())
	assert.Equal(t, uint(2), acct.MultisigKey().Threshold())
	assert.Equal(t, "ETH", acct.CurrencySymbol())

	// Fields that MUST NOT be set for Shared
	assert.Nil(t, acct.PublicKey(), "Shared must not have publicKey")
	assert.Nil(t, acct.EVMAddress(), "Shared must not have evmAddress")
	assert.Nil(t, acct.PeerID(), "Shared must not have peerID")
	assert.Nil(t, acct.DID(), "Shared must not have DID")
	assert.Nil(t, acct.ParentPublicKey(), "Shared must not have parentPubKey")
	assert.Nil(t, acct.P2PHost(), "Shared must not have p2pHost")

	// Balance fields are always nil at construction
	assert.Nil(t, acct.CreditBalance())
	assert.Nil(t, acct.BalanceAllocation())
	assert.Nil(t, acct.Balance())
}

func TestSharedAccountBuilder_Build_MissingMultisigKey(t *testing.T) {
	_, err := NewSharedAccountBuilder().
		WithCurrency("ETH").
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-SHARED-01", ve.RuleCode)
	assert.Equal(t, "multisigKey", ve.Field)
}

func TestSharedAccountBuilder_Build_MissingCurrency(t *testing.T) {
	mk := testMultisigKey(t)

	_, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-SHARED-05", ve.RuleCode)
	assert.Equal(t, "currencySymbol", ve.Field)
}

func TestSharedAccountBuilder_BuildComplete_Success(t *testing.T) {
	mk := testMultisigKey(t)
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	la := LedgerAttachment{
		ledgerIdentifier: "hedera-mainnet",
		attachedAddress:  pk.PublicKey().EVMAddress(),
		state:            Attached,
	}

	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("HBAR").
		WithLedgerAttachment(la).
		BuildComplete()

	require.NoError(t, err)
	require.NotNil(t, acct)
	assert.NotNil(t, acct.LedgerAttachment())
}

func TestSharedAccountBuilder_BuildComplete_MissingLedger(t *testing.T) {
	mk := testMultisigKey(t)

	_, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		BuildComplete()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-SHARED-06", ve.RuleCode)
	assert.Equal(t, "ledgerAttachment", ve.Field)
}

func TestSharedAccountBuilder_FluentChain(t *testing.T) {
	mk := testMultisigKey(t)

	builder := NewSharedAccountBuilder()
	b1 := builder.WithMultisigKey(mk)
	assert.Same(t, builder, b1)

	b2 := b1.WithCurrency("ETH")
	assert.Same(t, builder, b2)
}

// ===========================================================================
// Cross-cutting: zero-value public key rejection
// ===========================================================================

func TestParentAccountBuilder_Build_ZeroPublicKey(t *testing.T) {
	var zeroPK keylib.NeuronPublicKey
	did := NeuronDID{identifier: "did:key:zQ3stest"}

	_, err := NewParentAccountBuilder().
		WithPublicKey(zeroPK).
		WithDID(did).
		WithCurrency("ETH").
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-PARENT-02", ve.RuleCode)
}

func TestChildAccountBuilder_Build_ZeroPublicKey(t *testing.T) {
	var zeroPK keylib.NeuronPublicKey
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	_, err = NewChildAccountBuilder().
		WithPublicKey(zeroPK).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-CHILD-02", ve.RuleCode)
}

func TestChildAccountBuilder_Build_ZeroParentPublicKey(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	var zeroPK keylib.NeuronPublicKey

	_, err = NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(zeroPK).
		WithCurrency("ETH").
		Build()

	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "V-CHILD-01", ve.RuleCode)
}
