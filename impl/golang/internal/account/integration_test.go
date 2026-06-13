package account

import (
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// T067: Full Parent lifecycle
// build -> validate -> attach to ledger -> verify attachment ->
// resolve payment address -> serialize -> deserialize -> re-validate
// ===========================================================================

func TestT067_ParentLifecycle(t *testing.T) {
	// Step 1: Generate keys and build Parent account.
	privKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pubKey := privKey.PublicKey()

	did, err := GenerateDID(pubKey)
	require.NoError(t, err)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)
	require.NotNil(t, acct)

	// Step 2: Validate — no errors expected.
	errs := acct.Validate()
	assert.Empty(t, errs, "freshly built Parent should pass validation")

	// Step 3: Attach to ledger.
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  pubKey.EVMAddress(),
	}
	acct.AttachToLedger(la)
	require.NotNil(t, acct.LedgerAttachment())
	assert.Equal(t, Attached, acct.LedgerAttachment().state)

	// Step 4: Verify attachment via mock verifier.
	verifier := &mockLedgerVerifier{
		accountResult: VerificationResult{
			Status:  Verified,
			Message: "account exists on ethereum-mainnet",
		},
	}
	result, err := acct.VerifyLedgerAttachment(verifier)
	require.NoError(t, err)
	assert.Equal(t, Verified, result.Status)
	assert.Equal(t, Verified, acct.LedgerAttachment().verificationStatus)

	// Step 5: Resolve payment address.
	payAddr, err := acct.PaymentAddress()
	require.NoError(t, err)
	assert.Equal(t, pubKey.EVMAddress().Hex(), payAddr.Hex(),
		"Parent payment address must be its own EVM address")

	// Step 6: Serialize.
	data, err := Serialize(acct)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Step 7: Deserialize.
	restored, err := Deserialize(data)
	require.NoError(t, err)

	// Step 8: Re-validate deserialized account.
	errs = restored.Validate()
	assert.Empty(t, errs, "deserialized Parent should pass validation")

	// Verify all fields survived the round-trip.
	assert.Equal(t, Parent, restored.AccountType())
	assert.Equal(t, pubKey.Hex(), restored.PublicKey().Hex())
	assert.Equal(t, did.identifier, restored.DID().identifier)
	assert.Equal(t, "ETH", restored.CurrencySymbol())
	require.NotNil(t, restored.LedgerAttachment())
	assert.Equal(t, Attached, restored.LedgerAttachment().state)
	assert.Equal(t, Verified, restored.LedgerAttachment().verificationStatus)
}

// ===========================================================================
// T068: Full Child lifecycle
// build with parent ref and registry binding -> validate -> attach to ledger
// -> verify parent-child -> resolve payment address -> serialize ->
// deserialize -> re-validate
// ===========================================================================

func TestT068_ChildLifecycle(t *testing.T) {
	// Step 1: Generate keys and build Child account.
	childPriv, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPriv, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "42",
	}

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPriv.PublicKey()).
		WithParentPublicKey(parentPriv.PublicKey()).
		WithCurrency("ETH").
		WithRegistryBinding(rb).
		Build()
	require.NoError(t, err)
	require.NotNil(t, acct)

	// Step 2: Validate — no errors expected (Child built with registryBinding).
	errs := acct.Validate()
	assert.Empty(t, errs, "Child with registryBinding should pass validation")

	// Step 3: Attach to ledger.
	la := LedgerAttachment{
		ledgerIdentifier: "ethereum-mainnet",
		attachedAddress:  childPriv.PublicKey().EVMAddress(),
	}
	acct.AttachToLedger(la)
	require.NotNil(t, acct.LedgerAttachment())
	assert.Equal(t, Attached, acct.LedgerAttachment().state)

	// Step 4: Verify parent-child relationship.
	pcVerifier := &mockParentChildVerifier{
		result: VerificationResult{
			Status:  Verified,
			Message: "parent-child relationship confirmed",
		},
	}
	pcResult, err := VerifyParentChild(acct, pcVerifier)
	require.NoError(t, err)
	assert.Equal(t, Verified, pcResult.Status)

	// Step 5: Resolve payment address (should be parent's EVM address).
	payAddr, err := acct.PaymentAddress()
	require.NoError(t, err)
	parentAddr := parentPriv.PublicKey().EVMAddress()
	assert.Equal(t, parentAddr.Hex(), payAddr.Hex(),
		"Child payment address must be parent's EVM address")

	// Step 6: Serialize.
	data, err := Serialize(acct)
	require.NoError(t, err)

	// Step 7: Deserialize.
	restored, err := Deserialize(data)
	require.NoError(t, err)

	// Step 8: Re-validate.
	errs = restored.Validate()
	assert.Empty(t, errs, "deserialized Child should pass validation")

	// Verify round-trip fidelity.
	assert.Equal(t, Child, restored.AccountType())
	assert.Equal(t, childPriv.PublicKey().Hex(), restored.PublicKey().Hex())
	assert.Equal(t, parentPriv.PublicKey().Hex(), restored.ParentPublicKey().Hex())
	require.NotNil(t, restored.RegistryBinding())
	assert.Equal(t, "42", restored.RegistryBinding().externalID)
	require.NotNil(t, restored.P2PHost())
	assert.Equal(t, acct.P2PHost().String(), restored.P2PHost().String())
}

// ===========================================================================
// T069: Full Shared lifecycle
// build with MultisigKey -> validate -> attach to ledger ->
// verify multisig config -> serialize -> deserialize -> re-validate
// ===========================================================================

func TestT069_SharedLifecycle(t *testing.T) {
	// Step 1: Build Shared account.
	mk := testMultisigKey(t)

	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("HBAR").
		Build()
	require.NoError(t, err)
	require.NotNil(t, acct)

	// Step 2: Validate.
	errs := acct.Validate()
	assert.Empty(t, errs, "freshly built Shared should pass validation")

	// Step 3: Attach to ledger.
	auxPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	la := LedgerAttachment{
		ledgerIdentifier: "hedera-mainnet",
		attachedAddress:  auxPK.PublicKey().EVMAddress(),
	}
	acct.AttachToLedger(la)
	require.NotNil(t, acct.LedgerAttachment())
	assert.Equal(t, Attached, acct.LedgerAttachment().state)

	// Step 4: Verify multisig config via mock verifier.
	verifier := &mockLedgerVerifier{
		multisigResult: VerificationResult{
			Status:  Verified,
			Message: "multisig config matches on-chain state",
		},
	}
	result, err := acct.VerifyLedgerAttachment(verifier)
	require.NoError(t, err)
	assert.Equal(t, Verified, result.Status)

	// Verify multisig key metadata.
	require.NotNil(t, acct.MultisigKey())
	assert.Equal(t, "secp256k1-aggregated", acct.MultisigKey().Protocol())
	assert.Equal(t, uint(2), acct.MultisigKey().Threshold())
	assert.Equal(t, uint(2), acct.MultisigKey().TotalKeys())

	// Step 5: Serialize.
	data, err := Serialize(acct)
	require.NoError(t, err)

	// Step 6: Deserialize.
	restored, err := Deserialize(data)
	require.NoError(t, err)

	// Step 7: Re-validate.
	errs = restored.Validate()
	assert.Empty(t, errs, "deserialized Shared should pass validation")

	// Verify multisig metadata survived round-trip.
	require.NotNil(t, restored.MultisigKey())
	assert.Equal(t, mk.Protocol(), restored.MultisigKey().Protocol())
	assert.Equal(t, mk.Threshold(), restored.MultisigKey().Threshold())
	assert.Equal(t, mk.TotalKeys(), restored.MultisigKey().TotalKeys())
	assert.Equal(t, "HBAR", restored.CurrencySymbol())
}

// ===========================================================================
// T070: Performance — account creation and validation in under 100ms
// ===========================================================================

func TestT070_Performance_AccountCreationAndValidation(t *testing.T) {
	start := time.Now()

	// Create one of each account type and validate.
	for i := 0; i < 10; i++ {
		// Parent
		pubKey, did := testKeyAndDID(t)
		parent, err := NewParentAccountBuilder().
			WithPublicKey(pubKey).
			WithDID(did).
			WithCurrency("ETH").
			Build()
		require.NoError(t, err)
		parentErrs := parent.Validate()
		assert.Empty(t, parentErrs)

		// Child
		childPK, err := keylib.NewNeuronPrivateKey()
		require.NoError(t, err)
		child, err := NewChildAccountBuilder().
			WithPublicKey(childPK.PublicKey()).
			WithParentPublicKey(pubKey).
			WithCurrency("ETH").
			WithRegistryBinding(RegistryBinding{
				registryIdentifier: "eip155:1:0xabc",
				externalID:         "1",
			}).
			Build()
		require.NoError(t, err)
		childErrs := child.Validate()
		assert.Empty(t, childErrs)

		// Shared
		mk := testMultisigKey(t)
		shared, err := NewSharedAccountBuilder().
			WithMultisigKey(mk).
			WithCurrency("ETH").
			Build()
		require.NoError(t, err)
		sharedErrs := shared.Validate()
		assert.Empty(t, sharedErrs)
	}

	elapsed := time.Since(start)
	assert.Less(t, elapsed, 100*time.Millisecond,
		"10 iterations of create+validate for all 3 types should complete in <100ms, took %v", elapsed)
}

// ===========================================================================
// T071: Cross-type negative tests
// Parent can't have Child fields, Child can't have Shared fields, etc.
// ===========================================================================

func TestT071_CrossType_ParentCannotHaveChildFields(t *testing.T) {
	// Build a Parent account, then manually inject Child-only fields
	// and verify Validate catches them.
	pubKey, did := testKeyAndDID(t)
	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	// Inject parentPubKey (a Child-only field) into the Parent.
	extraPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	extraPub := extraPK.PublicKey()
	acct.parentPubKey = &extraPub

	errs := acct.Validate()
	require.NotEmpty(t, errs, "Parent with parentPubKey should fail validation")
	foundRule := false
	for _, ve := range errs {
		if ve.RuleCode == "V-PARENT-04" {
			foundRule = true
			break
		}
	}
	assert.True(t, foundRule, "expected V-PARENT-04 violation")
}

func TestT071_CrossType_ParentCannotHaveMultisigKey(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	// Inject multisigKey (a Shared-only field) into the Parent.
	mk := testMultisigKey(t)
	acct.multisigKey = mk

	errs := acct.Validate()
	require.NotEmpty(t, errs)
	foundRule := false
	for _, ve := range errs {
		if ve.RuleCode == "V-PARENT-05" {
			foundRule = true
			break
		}
	}
	assert.True(t, foundRule, "expected V-PARENT-05 violation")
}

func TestT071_CrossType_ChildCannotHaveMultisigKey(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		WithRegistryBinding(RegistryBinding{
			registryIdentifier: "eip155:1:0xabc",
			externalID:         "1",
		}).
		Build()
	require.NoError(t, err)

	// Inject multisigKey (a Shared-only field).
	mk := testMultisigKey(t)
	acct.multisigKey = mk

	errs := acct.Validate()
	require.NotEmpty(t, errs)
	foundRule := false
	for _, ve := range errs {
		if ve.RuleCode == "V-CHILD-05" {
			foundRule = true
			break
		}
	}
	assert.True(t, foundRule, "expected V-CHILD-05 violation")
}

func TestT071_CrossType_ChildCannotHaveDID(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	acct, err := NewChildAccountBuilder().
		WithPublicKey(childPK.PublicKey()).
		WithParentPublicKey(parentPK.PublicKey()).
		WithCurrency("ETH").
		WithRegistryBinding(RegistryBinding{
			registryIdentifier: "eip155:1:0xabc",
			externalID:         "1",
		}).
		Build()
	require.NoError(t, err)

	// Inject DID (a Parent-only field).
	did := &NeuronDID{identifier: "did:key:zQ3stest"}
	acct.did = did

	errs := acct.Validate()
	require.NotEmpty(t, errs)
	foundRule := false
	for _, ve := range errs {
		if ve.RuleCode == "V-CHILD-04" {
			foundRule = true
			break
		}
	}
	assert.True(t, foundRule, "expected V-CHILD-04 violation")
}

func TestT071_CrossType_SharedCannotHavePublicKey(t *testing.T) {
	mk := testMultisigKey(t)
	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	// Inject publicKey (a Parent/Child field).
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := pk.PublicKey()
	acct.publicKey = &pub

	errs := acct.Validate()
	require.NotEmpty(t, errs)
	foundRule := false
	for _, ve := range errs {
		if ve.RuleCode == "V-SHARED-04" {
			foundRule = true
			break
		}
	}
	assert.True(t, foundRule, "expected V-SHARED-04 violation")
}

func TestT071_CrossType_SharedCannotHaveDID(t *testing.T) {
	mk := testMultisigKey(t)
	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	// Inject DID.
	acct.did = &NeuronDID{identifier: "did:key:zQ3stest"}

	errs := acct.Validate()
	require.NotEmpty(t, errs)
	foundRule := false
	for _, ve := range errs {
		if ve.RuleCode == "V-SHARED-02" {
			foundRule = true
			break
		}
	}
	assert.True(t, foundRule, "expected V-SHARED-02 violation")
}

func TestT071_CrossType_SharedCannotHaveParentPubKey(t *testing.T) {
	mk := testMultisigKey(t)
	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	// Inject parentPubKey.
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := pk.PublicKey()
	acct.parentPubKey = &pub

	errs := acct.Validate()
	require.NotEmpty(t, errs)
	foundRule := false
	for _, ve := range errs {
		if ve.RuleCode == "V-SHARED-03" {
			foundRule = true
			break
		}
	}
	assert.True(t, foundRule, "expected V-SHARED-03 violation")
}

func TestT071_CrossType_UnspecifiedRejected(t *testing.T) {
	// Zero-value NeuronAccount has Unspecified type.
	acct := &NeuronAccount{}

	errs := acct.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "V-TYPE-00", errs[0].RuleCode)
	assert.Equal(t, "accountType", errs[0].Field)
}

// ===========================================================================
// T072: Quickstart validation — reproduce quickstart code examples
// ===========================================================================

func TestT072_Quickstart_ParentCreation(t *testing.T) {
	// Reproduce the quickstart example for Parent account creation.
	// 1. Generate a private key.
	privKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// 2. Derive the public key.
	pubKey := privKey.PublicKey()

	// 3. Generate a DID.
	did, err := GenerateDID(pubKey)
	require.NoError(t, err)
	assert.NotEmpty(t, did.identifier)
	assert.Contains(t, did.identifier, "did:key:z")

	// 4. Build the Parent account.
	parent, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)
	require.NotNil(t, parent)

	// 5. Validate.
	errs := parent.Validate()
	assert.Empty(t, errs, "quickstart Parent must validate cleanly")

	// 6. Verify derived fields.
	assert.Equal(t, Parent, parent.AccountType())
	assert.NotNil(t, parent.EVMAddress())
	assert.NotNil(t, parent.PeerID())
	assert.Equal(t, pubKey.Hex(), parent.PublicKey().Hex())
}

func TestT072_Quickstart_ChildCreation(t *testing.T) {
	// Reproduce the quickstart example for Child account creation.
	// 1. Parent key (assume already created).
	parentPriv, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPriv.PublicKey()

	// 2. Child key.
	childPriv, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childPub := childPriv.PublicKey()

	// 3. Build Child with parent reference and registry binding.
	rb := RegistryBinding{
		registryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
		externalID:         "7",
	}

	child, err := NewChildAccountBuilder().
		WithPublicKey(childPub).
		WithParentPublicKey(parentPub).
		WithCurrency("ETH").
		WithRegistryBinding(rb).
		Build()
	require.NoError(t, err)
	require.NotNil(t, child)

	// 4. Validate.
	errs := child.Validate()
	assert.Empty(t, errs, "quickstart Child must validate cleanly")

	// 5. Verify fields.
	assert.Equal(t, Child, child.AccountType())
	assert.Equal(t, childPub.Hex(), child.PublicKey().Hex())
	assert.Equal(t, parentPub.Hex(), child.ParentPublicKey().Hex())
	assert.NotNil(t, child.P2PHost())
	assert.Equal(t, child.PeerID().String(), child.P2PHost().String())

	// 6. Registry binding intact.
	require.NotNil(t, child.RegistryBinding())
	assert.Equal(t, "7", child.RegistryBinding().externalID)

	// 7. Payment address resolves to parent's address.
	payAddr, err := child.PaymentAddress()
	require.NoError(t, err)
	assert.Equal(t, parentPub.EVMAddress().Hex(), payAddr.Hex())
}

func TestT072_Quickstart_SharedCreation(t *testing.T) {
	// Reproduce quickstart Shared account creation.
	// 1. Generate two private keys for multisig.
	pk1, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pk2, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// 2. Create 2-of-2 MultisigKey.
	mk, err := keylib.NewMultisigKey([]keylib.NeuronPrivateKey{pk1, pk2}, 2)
	require.NoError(t, err)

	// 3. Build Shared account.
	shared, err := NewSharedAccountBuilder().
		WithMultisigKey(&mk).
		WithCurrency("HBAR").
		Build()
	require.NoError(t, err)
	require.NotNil(t, shared)

	// 4. Validate.
	errs := shared.Validate()
	assert.Empty(t, errs, "quickstart Shared must validate cleanly")

	// 5. Verify fields.
	assert.Equal(t, Shared, shared.AccountType())
	assert.Equal(t, "secp256k1-aggregated", shared.MultisigKey().Protocol())
	assert.Equal(t, uint(2), shared.MultisigKey().Threshold())
	assert.Equal(t, uint(2), shared.MultisigKey().TotalKeys())

	// 6. Shared must not have single-key fields.
	assert.Nil(t, shared.PublicKey())
	assert.Nil(t, shared.EVMAddress())
	assert.Nil(t, shared.PeerID())
	assert.Nil(t, shared.DID())
}
