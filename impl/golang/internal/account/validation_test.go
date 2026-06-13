package account

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T005: ValidationError type tests

func TestValidationError_Fields(t *testing.T) {
	ve := ValidationError{
		Field:    "publicKey",
		RuleCode: "V-PARENT-02",
		Message:  "Parent account must have a single NeuronPublicKey",
	}
	assert.Equal(t, "publicKey", ve.Field)
	assert.Equal(t, "V-PARENT-02", ve.RuleCode)
	assert.Equal(t, "Parent account must have a single NeuronPublicKey", ve.Message)
}

func TestValidationError_Error(t *testing.T) {
	ve := ValidationError{
		Field:    "did",
		RuleCode: "V-PARENT-01",
		Message:  "Parent account must have a DID",
	}
	expected := "[V-PARENT-01] did: Parent account must have a DID"
	assert.Equal(t, expected, ve.Error())
}

func TestValidationError_ImplementsError(t *testing.T) {
	var err error = ValidationError{
		Field:    "multisigKey",
		RuleCode: "V-SHARED-01",
		Message:  "Shared account must have a MultisigKey with threshold",
	}
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "V-SHARED-01")
}

func TestValidationError_ActionableMessages(t *testing.T) {
	tests := []struct {
		rule    string
		field   string
		message string
	}{
		{"V-PARENT-01", "did", "Parent account must have a DID"},
		{"V-CHILD-01", "parentPubKey", "Child account must reference a parent NeuronPublicKey"},
		{"V-SHARED-01", "multisigKey", "Shared account must have a MultisigKey with threshold"},
	}
	for _, tt := range tests {
		t.Run(tt.rule, func(t *testing.T) {
			ve := ValidationError{Field: tt.field, RuleCode: tt.rule, Message: tt.message}
			assert.NotEmpty(t, ve.Message, "message must be actionable")
			assert.NotEmpty(t, ve.Field)
			assert.NotEmpty(t, ve.RuleCode)
		})
	}
}

// ===========================================================================
// Helper: find a ValidationError by RuleCode in a slice
// ===========================================================================

func findByRuleCode(errs []ValidationError, code string) *ValidationError {
	for i := range errs {
		if errs[i].RuleCode == code {
			return &errs[i]
		}
	}
	return nil
}

// ===========================================================================
// T035: Parent validation rules — each V-PARENT rule individually
// ===========================================================================

func TestValidate_Parent_VPARENT01_MissingDID(t *testing.T) {
	pubKey, _ := testKeyAndDID(t)
	acct := &NeuronAccount{
		accountType:    Parent,
		publicKey:      &pubKey,
		currencySymbol: "ETH",
		// did is nil => V-PARENT-01 violation
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-PARENT-01")
	require.NotNil(t, ve, "expected V-PARENT-01 violation")
	assert.Equal(t, "did", ve.Field)
}

func TestValidate_Parent_VPARENT01_EmptyDIDIdentifier(t *testing.T) {
	pubKey, _ := testKeyAndDID(t)
	emptyDID := &NeuronDID{identifier: ""}
	acct := &NeuronAccount{
		accountType:    Parent,
		publicKey:      &pubKey,
		did:            emptyDID,
		currencySymbol: "ETH",
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-PARENT-01")
	require.NotNil(t, ve, "expected V-PARENT-01 violation for empty DID identifier")
	assert.Equal(t, "did", ve.Field)
}

func TestValidate_Parent_VPARENT02_MissingPublicKey(t *testing.T) {
	_, did := testKeyAndDID(t)
	acct := &NeuronAccount{
		accountType:    Parent,
		did:            &did,
		currencySymbol: "ETH",
		// publicKey is nil => V-PARENT-02 violation
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-PARENT-02")
	require.NotNil(t, ve, "expected V-PARENT-02 violation")
	assert.Equal(t, "publicKey", ve.Field)
}

func TestValidate_Parent_VPARENT02_ZeroPublicKey(t *testing.T) {
	_, did := testKeyAndDID(t)
	var zeroPK keylib.NeuronPublicKey
	acct := &NeuronAccount{
		accountType:    Parent,
		publicKey:      &zeroPK,
		did:            &did,
		currencySymbol: "ETH",
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-PARENT-02")
	require.NotNil(t, ve, "expected V-PARENT-02 violation for zero public key")
	assert.Equal(t, "publicKey", ve.Field)
}

func TestValidate_Parent_VPARENT03_MissingCurrency(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	acct := &NeuronAccount{
		accountType: Parent,
		publicKey:   &pubKey,
		did:         &did,
		// currencySymbol is "" => V-PARENT-03 violation
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-PARENT-03")
	require.NotNil(t, ve, "expected V-PARENT-03 violation")
	assert.Equal(t, "currencySymbol", ve.Field)
}

func TestValidate_Parent_VPARENT04_HasParentRef(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPK.PublicKey()
	acct := &NeuronAccount{
		accountType:    Parent,
		publicKey:      &pubKey,
		did:            &did,
		currencySymbol: "ETH",
		parentPubKey:   &parentPub,
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-PARENT-04")
	require.NotNil(t, ve, "expected V-PARENT-04 violation")
	assert.Equal(t, "parentPubKey", ve.Field)
}

func TestValidate_Parent_VPARENT05_HasMultisigKey(t *testing.T) {
	pubKey, did := testKeyAndDID(t)
	mk := testMultisigKey(t)
	acct := &NeuronAccount{
		accountType:    Parent,
		publicKey:      &pubKey,
		did:            &did,
		currencySymbol: "ETH",
		multisigKey:    mk,
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-PARENT-05")
	require.NotNil(t, ve, "expected V-PARENT-05 violation")
	assert.Equal(t, "multisigKey", ve.Field)
}

// ===========================================================================
// T036: Child validation rules — each V-CHILD rule individually
// ===========================================================================

func TestValidate_Child_VCHILD01_MissingParentPubKey(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childPub := childPK.PublicKey()
	rb := &RegistryBinding{registryIdentifier: "eip155:1:0xABC", externalID: "1"}
	acct := &NeuronAccount{
		accountType:     Child,
		publicKey:       &childPub,
		registryBinding: rb,
		// parentPubKey is nil => V-CHILD-01 violation
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-CHILD-01")
	require.NotNil(t, ve, "expected V-CHILD-01 violation")
	assert.Equal(t, "parentPubKey", ve.Field)
}

func TestValidate_Child_VCHILD01_ZeroParentPubKey(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childPub := childPK.PublicKey()
	var zeroPK keylib.NeuronPublicKey
	rb := &RegistryBinding{registryIdentifier: "eip155:1:0xABC", externalID: "1"}
	acct := &NeuronAccount{
		accountType:     Child,
		publicKey:       &childPub,
		parentPubKey:    &zeroPK,
		registryBinding: rb,
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-CHILD-01")
	require.NotNil(t, ve, "expected V-CHILD-01 violation for zero parent public key")
	assert.Equal(t, "parentPubKey", ve.Field)
}

func TestValidate_Child_VCHILD02_MissingPublicKey(t *testing.T) {
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPK.PublicKey()
	rb := &RegistryBinding{registryIdentifier: "eip155:1:0xABC", externalID: "1"}
	acct := &NeuronAccount{
		accountType:     Child,
		parentPubKey:    &parentPub,
		registryBinding: rb,
		// publicKey is nil => V-CHILD-02 violation
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-CHILD-02")
	require.NotNil(t, ve, "expected V-CHILD-02 violation")
	assert.Equal(t, "publicKey", ve.Field)
}

func TestValidate_Child_VCHILD02_ZeroPublicKey(t *testing.T) {
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPK.PublicKey()
	var zeroPK keylib.NeuronPublicKey
	rb := &RegistryBinding{registryIdentifier: "eip155:1:0xABC", externalID: "1"}
	acct := &NeuronAccount{
		accountType:     Child,
		publicKey:       &zeroPK,
		parentPubKey:    &parentPub,
		registryBinding: rb,
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-CHILD-02")
	require.NotNil(t, ve, "expected V-CHILD-02 violation for zero public key")
	assert.Equal(t, "publicKey", ve.Field)
}

func TestValidate_Child_VCHILD03_MissingRegistryBinding(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childPub := childPK.PublicKey()
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPK.PublicKey()
	acct := &NeuronAccount{
		accountType:  Child,
		publicKey:    &childPub,
		parentPubKey: &parentPub,
		// registryBinding is nil => V-CHILD-03 violation
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-CHILD-03")
	require.NotNil(t, ve, "expected V-CHILD-03 violation")
	assert.Equal(t, "registryBinding", ve.Field)
}

func TestValidate_Child_VCHILD04_HasDID(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childPub := childPK.PublicKey()
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPK.PublicKey()
	rb := &RegistryBinding{registryIdentifier: "eip155:1:0xABC", externalID: "1"}
	did := &NeuronDID{identifier: "did:key:zQ3stest"}
	acct := &NeuronAccount{
		accountType:     Child,
		publicKey:       &childPub,
		parentPubKey:    &parentPub,
		registryBinding: rb,
		did:             did,
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-CHILD-04")
	require.NotNil(t, ve, "expected V-CHILD-04 violation")
	assert.Equal(t, "did", ve.Field)
}

func TestValidate_Child_VCHILD05_HasMultisigKey(t *testing.T) {
	childPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childPub := childPK.PublicKey()
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPK.PublicKey()
	rb := &RegistryBinding{registryIdentifier: "eip155:1:0xABC", externalID: "1"}
	mk := testMultisigKey(t)
	acct := &NeuronAccount{
		accountType:     Child,
		publicKey:       &childPub,
		parentPubKey:    &parentPub,
		registryBinding: rb,
		multisigKey:     mk,
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-CHILD-05")
	require.NotNil(t, ve, "expected V-CHILD-05 violation")
	assert.Equal(t, "multisigKey", ve.Field)
}

// ===========================================================================
// T037: Shared validation rules — each V-SHARED rule individually
// ===========================================================================

func TestValidate_Shared_VSHARED01_MissingMultisigKey(t *testing.T) {
	acct := &NeuronAccount{
		accountType: Shared,
		// multisigKey is nil => V-SHARED-01 violation
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-SHARED-01")
	require.NotNil(t, ve, "expected V-SHARED-01 violation")
	assert.Equal(t, "multisigKey", ve.Field)
}

func TestValidate_Shared_VSHARED02_HasDID(t *testing.T) {
	mk := testMultisigKey(t)
	did := &NeuronDID{identifier: "did:key:zQ3stest"}
	acct := &NeuronAccount{
		accountType: Shared,
		multisigKey: mk,
		did:         did,
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-SHARED-02")
	require.NotNil(t, ve, "expected V-SHARED-02 violation")
	assert.Equal(t, "did", ve.Field)
}

func TestValidate_Shared_VSHARED03_HasParentRef(t *testing.T) {
	mk := testMultisigKey(t)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPK.PublicKey()
	acct := &NeuronAccount{
		accountType:  Shared,
		multisigKey:  mk,
		parentPubKey: &parentPub,
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-SHARED-03")
	require.NotNil(t, ve, "expected V-SHARED-03 violation")
	assert.Equal(t, "parentPubKey", ve.Field)
}

func TestValidate_Shared_VSHARED04_HasPublicKey(t *testing.T) {
	mk := testMultisigKey(t)
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := pk.PublicKey()
	acct := &NeuronAccount{
		accountType: Shared,
		multisigKey: mk,
		publicKey:   &pub,
	}
	errs := acct.Validate()
	ve := findByRuleCode(errs, "V-SHARED-04")
	require.NotNil(t, ve, "expected V-SHARED-04 violation")
	assert.Equal(t, "publicKey", ve.Field)
}

// ===========================================================================
// T038: Valid accounts produce empty error list
// ===========================================================================

func TestValidate_ValidParent_NoErrors(t *testing.T) {
	pubKey, did := testKeyAndDID(t)

	acct, err := NewParentAccountBuilder().
		WithPublicKey(pubKey).
		WithDID(did).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	errs := acct.Validate()
	assert.Empty(t, errs, "valid Parent account should have no validation errors")
}

func TestValidate_ValidChild_NoErrors(t *testing.T) {
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

	errs := acct.Validate()
	assert.Empty(t, errs, "valid Child account with registry binding should have no validation errors")
}

func TestValidate_ValidShared_NoErrors(t *testing.T) {
	mk := testMultisigKey(t)

	acct, err := NewSharedAccountBuilder().
		WithMultisigKey(mk).
		WithCurrency("ETH").
		Build()
	require.NoError(t, err)

	errs := acct.Validate()
	assert.Empty(t, errs, "valid Shared account should have no validation errors")
}

// ===========================================================================
// T039: Multi-violation detection and Unspecified type rejection
// ===========================================================================

func TestValidate_Parent_MultipleViolations(t *testing.T) {
	// Parent with no DID, no publicKey, no currency => 3 violations
	acct := &NeuronAccount{accountType: Parent}
	errs := acct.Validate()

	require.Len(t, errs, 3, "empty Parent should have exactly 3 violations")

	// Verify all three rule codes are present
	codes := make(map[string]bool)
	for _, e := range errs {
		codes[e.RuleCode] = true
	}
	assert.True(t, codes["V-PARENT-01"], "expected V-PARENT-01")
	assert.True(t, codes["V-PARENT-02"], "expected V-PARENT-02")
	assert.True(t, codes["V-PARENT-03"], "expected V-PARENT-03")
}

func TestValidate_Child_MultipleViolations(t *testing.T) {
	// Child with nothing set => 3 violations (parentPubKey, publicKey, registryBinding)
	acct := &NeuronAccount{accountType: Child}
	errs := acct.Validate()

	require.Len(t, errs, 3, "empty Child should have exactly 3 violations")

	codes := make(map[string]bool)
	for _, e := range errs {
		codes[e.RuleCode] = true
	}
	assert.True(t, codes["V-CHILD-01"], "expected V-CHILD-01")
	assert.True(t, codes["V-CHILD-02"], "expected V-CHILD-02")
	assert.True(t, codes["V-CHILD-03"], "expected V-CHILD-03")
}

func TestValidate_Shared_MultipleViolations(t *testing.T) {
	// Shared with publicKey and DID set but no multisigKey => 3 violations
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := pk.PublicKey()
	did := &NeuronDID{identifier: "did:key:zQ3stest"}
	acct := &NeuronAccount{
		accountType: Shared,
		publicKey:   &pub,
		did:         did,
	}
	errs := acct.Validate()

	require.Len(t, errs, 3, "Shared with missing multisigKey + extra fields should have 3 violations")

	codes := make(map[string]bool)
	for _, e := range errs {
		codes[e.RuleCode] = true
	}
	assert.True(t, codes["V-SHARED-01"], "expected V-SHARED-01")
	assert.True(t, codes["V-SHARED-02"], "expected V-SHARED-02")
	assert.True(t, codes["V-SHARED-04"], "expected V-SHARED-04")
}

func TestValidate_Unspecified_Rejected(t *testing.T) {
	acct := &NeuronAccount{} // accountType defaults to Unspecified (0)
	errs := acct.Validate()

	require.Len(t, errs, 1, "Unspecified account type should produce exactly 1 error")
	assert.Equal(t, "V-TYPE-00", errs[0].RuleCode)
	assert.Equal(t, "accountType", errs[0].Field)
}

func TestValidate_Parent_AllViolations_IncludingMustNotFields(t *testing.T) {
	// Parent with nothing valid AND with forbidden fields set => 5 violations
	mk := testMultisigKey(t)
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPK.PublicKey()

	acct := &NeuronAccount{
		accountType:  Parent,
		parentPubKey: &parentPub, // V-PARENT-04
		multisigKey:  mk,         // V-PARENT-05
		// missing did => V-PARENT-01
		// missing publicKey => V-PARENT-02
		// missing currencySymbol => V-PARENT-03
	}
	errs := acct.Validate()

	require.Len(t, errs, 5, "Parent with all violations should have 5 errors")

	codes := make(map[string]bool)
	for _, e := range errs {
		codes[e.RuleCode] = true
	}
	assert.True(t, codes["V-PARENT-01"], "expected V-PARENT-01")
	assert.True(t, codes["V-PARENT-02"], "expected V-PARENT-02")
	assert.True(t, codes["V-PARENT-03"], "expected V-PARENT-03")
	assert.True(t, codes["V-PARENT-04"], "expected V-PARENT-04")
	assert.True(t, codes["V-PARENT-05"], "expected V-PARENT-05")
}

func TestValidate_Child_AllViolations_IncludingMustNotFields(t *testing.T) {
	// Child with nothing valid AND with forbidden fields set => 5 violations
	mk := testMultisigKey(t)
	did := &NeuronDID{identifier: "did:key:zQ3stest"}

	acct := &NeuronAccount{
		accountType: Child,
		did:         did, // V-CHILD-04
		multisigKey: mk,  // V-CHILD-05
		// missing parentPubKey => V-CHILD-01
		// missing publicKey => V-CHILD-02
		// missing registryBinding => V-CHILD-03
	}
	errs := acct.Validate()

	require.Len(t, errs, 5, "Child with all violations should have 5 errors")

	codes := make(map[string]bool)
	for _, e := range errs {
		codes[e.RuleCode] = true
	}
	assert.True(t, codes["V-CHILD-01"], "expected V-CHILD-01")
	assert.True(t, codes["V-CHILD-02"], "expected V-CHILD-02")
	assert.True(t, codes["V-CHILD-03"], "expected V-CHILD-03")
	assert.True(t, codes["V-CHILD-04"], "expected V-CHILD-04")
	assert.True(t, codes["V-CHILD-05"], "expected V-CHILD-05")
}

func TestValidate_Shared_AllViolations(t *testing.T) {
	// Shared with all forbidden fields set and missing multisigKey => 4 violations
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := pk.PublicKey()
	parentPK, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	parentPub := parentPK.PublicKey()
	did := &NeuronDID{identifier: "did:key:zQ3stest"}

	acct := &NeuronAccount{
		accountType:  Shared,
		publicKey:    &pub,       // V-SHARED-04
		did:          did,        // V-SHARED-02
		parentPubKey: &parentPub, // V-SHARED-03
		// missing multisigKey => V-SHARED-01
	}
	errs := acct.Validate()

	require.Len(t, errs, 4, "Shared with all violations should have 4 errors")

	codes := make(map[string]bool)
	for _, e := range errs {
		codes[e.RuleCode] = true
	}
	assert.True(t, codes["V-SHARED-01"], "expected V-SHARED-01")
	assert.True(t, codes["V-SHARED-02"], "expected V-SHARED-02")
	assert.True(t, codes["V-SHARED-03"], "expected V-SHARED-03")
	assert.True(t, codes["V-SHARED-04"], "expected V-SHARED-04")
}
