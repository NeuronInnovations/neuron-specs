package account

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T054: RegistryBinding storage and retrieval.
func TestRegistryBinding_StoredOnChild(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "42",
	}

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		WithRegistryBinding(rb).
		Build()
	require.NoError(t, err)

	require.NotNil(t, acct.RegistryBinding())
	assert.Equal(t, "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		acct.RegistryBinding().registryIdentifier)
	assert.Equal(t, "42", acct.RegistryBinding().externalID)
}

// TestRegistryBinding_NotOnParent verifies Parent accounts do not have a registry binding.
func TestRegistryBinding_NotOnParent(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)
	assert.Nil(t, acct.RegistryBinding(), "Parent accounts must not have registry binding")
}

// TestRegistryBinding_NotOnShared verifies Shared accounts do not have a registry binding.
func TestRegistryBinding_NotOnShared(t *testing.T) {
	mk := testMultisigKey(t)

	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)
	assert.Nil(t, acct.RegistryBinding(), "Shared accounts must not have registry binding")
}

// TestValidateRegistryBinding_Valid verifies that a valid binding passes validation.
func TestValidateRegistryBinding_Valid(t *testing.T) {
	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "42",
	}
	err := ValidateRegistryBinding(rb)
	assert.NoError(t, err)
}

// TestValidateRegistryBinding_EmptyIdentifier verifies that empty RegistryIdentifier fails.
func TestValidateRegistryBinding_EmptyIdentifier(t *testing.T) {
	rb := RegistryBinding{
		registryIdentifier: "",
		externalID:         "42",
	}
	err := ValidateRegistryBinding(rb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RegistryIdentifier")
}

// TestValidateRegistryBinding_EmptyExternalID verifies that empty ExternalID fails.
func TestValidateRegistryBinding_EmptyExternalID(t *testing.T) {
	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "",
	}
	err := ValidateRegistryBinding(rb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ExternalID")
}

// TestRegistryBinding_ChildWithoutBinding verifies a Child can be built without registry binding.
func TestRegistryBinding_ChildWithoutBinding(t *testing.T) {
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
	assert.Nil(t, acct.RegistryBinding(), "Registry binding is optional on Build")
}
