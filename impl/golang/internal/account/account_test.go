package account

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T004: AccountType enum values and string representations

func TestAccountType_Values(t *testing.T) {
	assert.Equal(t, AccountType(0), Unspecified)
	assert.Equal(t, AccountType(1), Parent)
	assert.Equal(t, AccountType(2), Child)
	assert.Equal(t, AccountType(3), Shared)
}

func TestAccountType_String(t *testing.T) {
	tests := []struct {
		at   AccountType
		want string
	}{
		{Parent, "Parent"},
		{Child, "Child"},
		{Shared, "Shared"},
		{Unspecified, "Unspecified"},
		{AccountType(99), "Unspecified"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.at.String())
		})
	}
}

func TestAccountType_IsValid(t *testing.T) {
	assert.True(t, Parent.IsValid())
	assert.True(t, Child.IsValid())
	assert.True(t, Shared.IsValid())
	assert.False(t, Unspecified.IsValid())
	assert.False(t, AccountType(0).IsValid(), "zero value must be invalid")
	assert.False(t, AccountType(99).IsValid())
}

// T019: Parent account has no reachability fields (p2pHost must be nil).
func TestParentAccount_NoReachability(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	assert.Nil(t, acct.P2PHost(), "Parent account must not have p2pHost")
	assert.Nil(t, acct.RegistryBinding(), "Parent account must not have registryBinding")
	assert.Nil(t, acct.FeePayer(), "Parent account must not have feePayer")
	assert.Nil(t, acct.ParentPublicKey(), "Parent account must not have parentPubKey")
	assert.Nil(t, acct.MultisigKey(), "Parent account must not have multisigKey")
}

// T030: Child account has no reachability fields beyond p2pHost, and p2pHost = peerID.
func TestChildAccount_P2PHostEqualsPeerID(t *testing.T) {
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

	assert.NotNil(t, acct.PeerID())
	assert.NotNil(t, acct.P2PHost())
	assert.Equal(t, acct.PeerID().String(), acct.P2PHost().String(),
		"Child p2pHost must equal peerID")

	// Child must not have DID or MultisigKey
	assert.Nil(t, acct.DID(), "Child must not have DID")
	assert.Nil(t, acct.MultisigKey(), "Child must not have multisigKey")
}

// TestNeuronAccount_Getters verifies all getters return expected values for a Parent account.
func TestNeuronAccount_Getters_Parent(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	assert.Equal(t, Parent, acct.AccountType())
	assert.NotNil(t, acct.PublicKey())
	assert.NotNil(t, acct.EVMAddress())
	assert.NotNil(t, acct.PeerID())
	assert.NotNil(t, acct.DID())
	assert.Equal(t, "ETH", acct.CurrencySymbol())

	// Nil fields
	assert.Nil(t, acct.ParentPublicKey())
	assert.Nil(t, acct.MultisigKey())
	assert.Nil(t, acct.CreditBalance())
	assert.Nil(t, acct.BalanceAllocation())
	assert.Nil(t, acct.Balance())
	assert.Nil(t, acct.LedgerAttachment())
	assert.Nil(t, acct.RegistryBinding())
	assert.Nil(t, acct.FeePayer())
	assert.Nil(t, acct.P2PHost())
}

// TestNeuronAccount_Getters_Child verifies all getters return expected values for a Child account.
func TestNeuronAccount_Getters_Child(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("HBAR").
		Build()
	require.NoError(t, err)

	assert.Equal(t, Child, acct.AccountType())
	assert.NotNil(t, acct.PublicKey())
	assert.NotNil(t, acct.EVMAddress())
	assert.NotNil(t, acct.PeerID())
	assert.NotNil(t, acct.ParentPublicKey())
	assert.NotNil(t, acct.P2PHost())
	assert.Equal(t, "HBAR", acct.CurrencySymbol())

	// Nil fields
	assert.Nil(t, acct.DID())
	assert.Nil(t, acct.MultisigKey())
	assert.Nil(t, acct.CreditBalance())
	assert.Nil(t, acct.BalanceAllocation())
	assert.Nil(t, acct.Balance())
}

// TestNeuronAccount_Getters_Shared verifies all getters return expected values for a Shared account.
func TestNeuronAccount_Getters_Shared(t *testing.T) {
	mk := testMultisigKey(t)

	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	assert.Equal(t, Shared, acct.AccountType())
	assert.NotNil(t, acct.MultisigKey())
	assert.Equal(t, "ETH", acct.CurrencySymbol())

	// Nil fields
	assert.Nil(t, acct.PublicKey())
	assert.Nil(t, acct.EVMAddress())
	assert.Nil(t, acct.PeerID())
	assert.Nil(t, acct.DID())
	assert.Nil(t, acct.ParentPublicKey())
	assert.Nil(t, acct.P2PHost())
	assert.Nil(t, acct.CreditBalance())
	assert.Nil(t, acct.BalanceAllocation())
	assert.Nil(t, acct.Balance())
}
