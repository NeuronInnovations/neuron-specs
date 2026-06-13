package account

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// T061: Serialize Parent/Child/Shared to JSON — canonical format verification
// ===========================================================================

func TestT061_SerializeParent_CanonicalFormat(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	la := testLedgerAttachment(t, pubKey)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	data, err := Serialize(acct)
	require.NoError(t, err)

	// Parse as generic JSON to verify structure.
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// accountType must be "Parent".
	assert.Equal(t, "Parent", raw["accountType"])

	// publicKey must be compressed hex (starts with 0x02 or 0x03, 66+2 chars).
	pk, ok := raw["publicKey"].(string)
	require.True(t, ok, "publicKey must be a string")
	assert.True(t, strings.HasPrefix(pk, "0x02") || strings.HasPrefix(pk, "0x03"),
		"publicKey must be compressed hex, got: %s", pk)
	assert.Equal(t, 68, len(pk), "compressed hex with 0x prefix = 68 chars")

	// evmAddress must be EIP-55 checksummed (starts with 0x, 42 chars).
	addr, ok := raw["evmAddress"].(string)
	require.True(t, ok, "evmAddress must be a string")
	assert.True(t, strings.HasPrefix(addr, "0x"), "evmAddress must start with 0x")
	assert.Equal(t, 42, len(addr))

	// peerID must be present and non-empty.
	pid, ok := raw["peerID"].(string)
	require.True(t, ok, "peerID must be a string")
	assert.NotEmpty(t, pid)

	// did must be an object with "identifier".
	didObj, ok := raw["did"].(map[string]interface{})
	require.True(t, ok, "did must be an object")
	assert.True(t, strings.HasPrefix(didObj["identifier"].(string), "did:key:z"))

	// currencySymbol must be present.
	assert.Equal(t, "ETH", raw["currencySymbol"])

	// ledgerAttachment must be present.
	laObj, ok := raw["ledgerAttachment"].(map[string]interface{})
	require.True(t, ok, "ledgerAttachment must be an object")
	assert.Equal(t, "ethereum-mainnet", laObj["ledgerIdentifier"])
	assert.Equal(t, "Attached", laObj["state"])

	// Parent must NOT have parentPubKey, multisigKey, p2pHost.
	assert.Empty(t, raw["parentPubKey"])
	assert.Nil(t, raw["multisigKey"])
	assert.Empty(t, raw["p2pHost"])
}

func TestT061_SerializeChild_CanonicalFormat(t *testing.T) {
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

	data, err := Serialize(acct)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, "Child", raw["accountType"])

	// Both publicKey and parentPubKey must be compressed hex.
	pk := raw["publicKey"].(string)
	assert.True(t, strings.HasPrefix(pk, "0x02") || strings.HasPrefix(pk, "0x03"))

	ppk := raw["parentPubKey"].(string)
	assert.True(t, strings.HasPrefix(ppk, "0x02") || strings.HasPrefix(ppk, "0x03"))

	// p2pHost must be present for Child.
	p2p, ok := raw["p2pHost"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, p2p)
	assert.Equal(t, raw["peerID"], raw["p2pHost"], "p2pHost must equal peerID for Child")

	// feePayer must be present.
	assert.Equal(t, "0.0.12345", raw["feePayer"])

	// registryBinding must be present.
	rbObj, ok := raw["registryBinding"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "42", rbObj["externalID"])

	// Child must NOT have did or multisigKey.
	assert.Nil(t, raw["did"])
	assert.Nil(t, raw["multisigKey"])
}

func TestT061_SerializeShared_CanonicalFormat(t *testing.T) {
	mk := testMultisigKey(t)

	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	data, err := Serialize(acct)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, "Shared", raw["accountType"])

	// MultisigKey must include threshold, protocol, totalKeys.
	mkObj, ok := raw["multisigKey"].(map[string]interface{})
	require.True(t, ok, "multisigKey must be an object")
	assert.Equal(t, "secp256k1-aggregated", mkObj["protocol"])
	assert.Equal(t, float64(2), mkObj["threshold"])
	assert.Equal(t, float64(2), mkObj["totalKeys"])

	// Shared must NOT have publicKey, evmAddress, peerID, did, parentPubKey, p2pHost.
	assert.Empty(t, raw["publicKey"])
	assert.Empty(t, raw["evmAddress"])
	assert.Empty(t, raw["peerID"])
	assert.Nil(t, raw["did"])
	assert.Empty(t, raw["parentPubKey"])
	assert.Empty(t, raw["p2pHost"])
}

// ===========================================================================
// T062: Deserialize from JSON — round-trip field verification
// ===========================================================================

func TestT062_DeserializeParent_FieldsMatch(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	now := time.Now().Truncate(time.Second)
	la := LedgerAttachment{
		ledgerIdentifier:   "ethereum-mainnet",
		attachedAddress:    pubKey.EVMAddress(),
		state:              Attached,
		verificationStatus: Verified,
		lastSyncedAt:       &now,
	}

	original, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	data, err := Serialize(original)
	require.NoError(t, err)

	restored, err := Deserialize(data)
	require.NoError(t, err)

	// All fields must match.
	assert.Equal(t, original.AccountType(), restored.AccountType())
	assert.Equal(t, original.PublicKey().Hex(), restored.PublicKey().Hex())
	assert.Equal(t, original.EVMAddress().Hex(), restored.EVMAddress().Hex())
	assert.Equal(t, original.PeerID().String(), restored.PeerID().String())
	assert.Equal(t, original.DID().identifier, restored.DID().identifier)
	assert.Equal(t, original.CurrencySymbol(), restored.CurrencySymbol())

	// Ledger attachment fields.
	require.NotNil(t, restored.LedgerAttachment())
	assert.Equal(t, original.LedgerAttachment().ledgerIdentifier, restored.LedgerAttachment().ledgerIdentifier)
	assert.Equal(t, original.LedgerAttachment().attachedAddress.Hex(), restored.LedgerAttachment().attachedAddress.Hex())
	assert.Equal(t, original.LedgerAttachment().state, restored.LedgerAttachment().state)
	assert.Equal(t, original.LedgerAttachment().verificationStatus, restored.LedgerAttachment().verificationStatus)
	require.NotNil(t, restored.LedgerAttachment().lastSyncedAt)
	assert.True(t, original.LedgerAttachment().lastSyncedAt.Equal(*restored.LedgerAttachment().lastSyncedAt))

	// Nil fields stay nil.
	assert.Nil(t, restored.ParentPublicKey())
	assert.Nil(t, restored.MultisigKey())
	assert.Nil(t, restored.P2PHost())
	assert.Nil(t, restored.CreditBalance())
}

func TestT062_DeserializeChild_FieldsMatch(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "42",
	}
	fp := LedgerAccountId("0.0.12345")

	original, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("HBAR").
		WithRegistryBinding(rb).
		WithFeePayer(fp).
		Build()
	require.NoError(t, err)

	data, err := Serialize(original)
	require.NoError(t, err)

	restored, err := Deserialize(data)
	require.NoError(t, err)

	assert.Equal(t, Child, restored.AccountType())
	assert.Equal(t, original.PublicKey().Hex(), restored.PublicKey().Hex())
	assert.Equal(t, original.EVMAddress().Hex(), restored.EVMAddress().Hex())
	assert.Equal(t, original.PeerID().String(), restored.PeerID().String())
	assert.Equal(t, original.ParentPublicKey().Hex(), restored.ParentPublicKey().Hex())
	assert.Equal(t, original.CurrencySymbol(), restored.CurrencySymbol())

	// p2pHost must be restored for Child.
	require.NotNil(t, restored.P2PHost())
	assert.Equal(t, original.P2PHost().String(), restored.P2PHost().String())

	// Registry binding.
	require.NotNil(t, restored.RegistryBinding())
	assert.Equal(t, original.RegistryBinding().registryIdentifier, restored.RegistryBinding().registryIdentifier)
	assert.Equal(t, original.RegistryBinding().externalID, restored.RegistryBinding().externalID)

	// Fee payer.
	require.NotNil(t, restored.FeePayer())
	assert.Equal(t, *original.FeePayer(), *restored.FeePayer())

	// Child must not have DID or multisigKey.
	assert.Nil(t, restored.DID())
	assert.Nil(t, restored.MultisigKey())
}

func TestT062_DeserializeShared_FieldsMatch(t *testing.T) {
	mk := testMultisigKey(t)

	original, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	data, err := Serialize(original)
	require.NoError(t, err)

	restored, err := Deserialize(data)
	require.NoError(t, err)

	assert.Equal(t, Shared, restored.AccountType())
	assert.Equal(t, original.CurrencySymbol(), restored.CurrencySymbol())

	// MultisigKey metadata must match.
	require.NotNil(t, restored.MultisigKey())
	assert.Equal(t, original.MultisigKey().Protocol(), restored.MultisigKey().Protocol())
	assert.Equal(t, original.MultisigKey().Threshold(), restored.MultisigKey().Threshold())
	assert.Equal(t, original.MultisigKey().TotalKeys(), restored.MultisigKey().TotalKeys())

	// Shared must not have publicKey, evmAddress, peerID, did, parentPubKey, p2pHost.
	assert.Nil(t, restored.PublicKey())
	assert.Nil(t, restored.EVMAddress())
	assert.Nil(t, restored.PeerID())
	assert.Nil(t, restored.DID())
	assert.Nil(t, restored.ParentPublicKey())
	assert.Nil(t, restored.P2PHost())
}

// ===========================================================================
// T063: Edge cases — nil fields, unknown JSON fields
// ===========================================================================

func TestT063_NilCreditBalance(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	// creditBalance is nil at construction.
	assert.Nil(t, acct.CreditBalance())

	data, err := Serialize(acct)
	require.NoError(t, err)

	// creditBalance must be omitted from JSON.
	assert.NotContains(t, string(data), "creditBalance")

	restored, err := Deserialize(data)
	require.NoError(t, err)
	assert.Nil(t, restored.CreditBalance())
}

func TestT063_NilLedgerAttachment(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	assert.Nil(t, acct.LedgerAttachment())

	data, err := Serialize(acct)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "ledgerAttachment")

	restored, err := Deserialize(data)
	require.NoError(t, err)
	assert.Nil(t, restored.LedgerAttachment())
}

func TestT063_NilRegistryBinding(t *testing.T) {
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

	assert.Nil(t, acct.RegistryBinding())

	data, err := Serialize(acct)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "registryBinding")

	// Deserialize now validates, and V-CHILD-03 rejects missing registryBinding.
	_, err = Deserialize(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "V-CHILD-03")
}

func TestT063_NilFeePayer(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0xabc",
		externalID:         "1",
	}

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		WithRegistryBinding(rb).
		Build()
	require.NoError(t, err)

	assert.Nil(t, acct.FeePayer())

	data, err := Serialize(acct)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "feePayer")

	restored, err := Deserialize(data)
	require.NoError(t, err)
	assert.Nil(t, restored.FeePayer())
}

func TestT063_UnknownJSONFieldsIgnored(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	data, err := Serialize(acct)
	require.NoError(t, err)

	// Inject unknown fields into the JSON.
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)
	raw["unknownField1"] = "should be ignored"
	raw["unknownField2"] = 42
	raw["nestedUnknown"] = map[string]interface{}{"foo": "bar"}

	modified, err := json.Marshal(raw)
	require.NoError(t, err)

	// Deserialization must succeed without error.
	restored, err := Deserialize(modified)
	require.NoError(t, err)
	assert.Equal(t, Parent, restored.AccountType())
	assert.Equal(t, pubKey.Hex(), restored.PublicKey().Hex())
}

// ===========================================================================
// T064: Round-trip — serialize -> deserialize -> serialize -> compare JSON
// ===========================================================================

func TestT064_RoundTrip_Parent(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	now := time.Now().Truncate(time.Second)
	la := LedgerAttachment{
		ledgerIdentifier:   "ethereum-mainnet",
		attachedAddress:    pubKey.EVMAddress(),
		state:              Attached,
		verificationStatus: Verified,
		lastSyncedAt:       &now,
	}

	original, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	// First round-trip.
	data1, err := Serialize(original)
	require.NoError(t, err)

	restored, err := Deserialize(data1)
	require.NoError(t, err)

	// Second serialization.
	data2, err := Serialize(restored)
	require.NoError(t, err)

	// JSON bytes must be identical.
	assert.JSONEq(t, string(data1), string(data2))
}

func TestT064_RoundTrip_Child(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "42",
	}
	fp := LedgerAccountId("0.0.12345")
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  childPK.PublicKey().EVMAddress(),
		state:            Attached,
	}

	original, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("HBAR").
		WithRegistryBinding(rb).
		WithFeePayer(fp).
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	data1, err := Serialize(original)
	require.NoError(t, err)

	restored, err := Deserialize(data1)
	require.NoError(t, err)

	data2, err := Serialize(restored)
	require.NoError(t, err)

	assert.JSONEq(t, string(data1), string(data2))
}

func TestT064_RoundTrip_Shared(t *testing.T) {
	mk := testMultisigKey(t)
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	la := LedgerAttachment{
		ledgerIdentifier: "hedera-mainnet",
		attachedAddress:  pk.PublicKey().EVMAddress(),
		state:            Attached,
	}

	original, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("HBAR").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	data1, err := Serialize(original)
	require.NoError(t, err)

	restored, err := Deserialize(data1)
	require.NoError(t, err)

	data2, err := Serialize(restored)
	require.NoError(t, err)

	assert.JSONEq(t, string(data1), string(data2))
}

func TestT064_RoundTrip_WithBalances(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	la := testLedgerAttachment(t, pubKey)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		WithLedgerAttachment(la).
		Build()
	require.NoError(t, err)

	// Manually set a credit balance to test round-trip.
	acct.creditBalance = big.NewInt(1000000)

	data1, err := Serialize(acct)
	require.NoError(t, err)

	// Verify creditBalance is in the JSON.
	assert.Contains(t, string(data1), `"creditBalance":"1000000"`)

	restored, err := Deserialize(data1)
	require.NoError(t, err)
	require.NotNil(t, restored.CreditBalance())
	assert.Equal(t, big.NewInt(1000000), restored.CreditBalance())

	data2, err := Serialize(restored)
	require.NoError(t, err)

	assert.JSONEq(t, string(data1), string(data2))
}
