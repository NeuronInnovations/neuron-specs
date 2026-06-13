package account

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T055: Parent payment address returns own EVM address.
func TestPaymentAddress_Parent(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	payAddr, err := acct.PaymentAddress()
	require.NoError(t, err)
	assert.Equal(t, pubKey.EVMAddress().Hex(), payAddr.Hex(),
		"Parent payment address must be own EVM address")
}

// T056: Child payment address resolves to parent's EVM address.
func TestPaymentAddress_Child(t *testing.T) {
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

	payAddr, err := acct.PaymentAddress()
	require.NoError(t, err)

	expectedParentAddr := parentPK.PublicKey().EVMAddress()
	assert.Equal(t, expectedParentAddr.Hex(), payAddr.Hex(),
		"Child payment address must resolve to parent's EVM address")
}

// T057: Child payment address is different from child's own EVM address.
func TestPaymentAddress_Child_NotOwnAddress(t *testing.T) {
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

	payAddr, err := acct.PaymentAddress()
	require.NoError(t, err)

	assert.NotEqual(t, acct.EVMAddress().Hex(), payAddr.Hex(),
		"Child payment address must not be the child's own EVM address")
}

// T058: Shared account payment address returns error.
func TestPaymentAddress_Shared_Error(t *testing.T) {
	mk := testMultisigKey(t)

	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	_, err = acct.PaymentAddress()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Shared accounts have no payment address")
}

// TestPaymentAddress_Unspecified verifies that an account with unspecified type
// returns an error.
func TestPaymentAddress_Unspecified(t *testing.T) {
	acct := &NeuronAccount{accountType: Unspecified}
	_, err := acct.PaymentAddress()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unspecified account type")
}
