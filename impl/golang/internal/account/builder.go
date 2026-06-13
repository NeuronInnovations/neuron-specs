package account

import (
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// isZeroPublicKey reports whether a NeuronPublicKey is the zero value.
func isZeroPublicKey(pk keylib.NeuronPublicKey) bool {
	compressed := pk.Compressed()
	for _, b := range compressed {
		if b != 0 {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// ParentAccountBuilder
// ---------------------------------------------------------------------------

// FR-011: Fluent builder API for account construction
// ParentAccountBuilder constructs a Parent NeuronAccount using a fluent API.
// A Parent account requires a public key, a DID, and a currency symbol.
// It MUST NOT have a parentPubKey or multisigKey.
type ParentAccountBuilder struct {
	publicKey        *keylib.NeuronPublicKey
	did              *NeuronDID
	currencySymbol   string
	ledgerAttachment *LedgerAttachment
}

// NewParentAccountBuilder creates a new ParentAccountBuilder.
func NewParentAccountBuilder() *ParentAccountBuilder {
	return &ParentAccountBuilder{}
}

// WithPublicKey sets the public key for the Parent account.
func (b *ParentAccountBuilder) WithPublicKey(pk keylib.NeuronPublicKey) *ParentAccountBuilder {
	b.publicKey = &pk
	return b
}

// WithDID sets the DID for the Parent account.
func (b *ParentAccountBuilder) WithDID(did NeuronDID) *ParentAccountBuilder {
	b.did = &did
	return b
}

// WithCurrency sets the currency symbol for the Parent account.
func (b *ParentAccountBuilder) WithCurrency(symbol string) *ParentAccountBuilder {
	b.currencySymbol = symbol
	return b
}

// WithLedgerAttachment sets the ledger attachment for the Parent account.
func (b *ParentAccountBuilder) WithLedgerAttachment(la LedgerAttachment) *ParentAccountBuilder {
	b.ledgerAttachment = &la
	return b
}

// FR-006: Parent account validation rules
// Build constructs and validates a Parent NeuronAccount.
//
// Validation rules:
//   - V-PARENT-02: publicKey MUST be set
//   - V-PARENT-01: DID MUST be set
//   - V-PARENT-03: currencySymbol MUST be set
//   - V-PARENT-04: parentPubKey MUST NOT be set (enforced by builder: not settable)
//   - V-PARENT-05: multisigKey MUST NOT be set (enforced by builder: not settable)
//
// Derives evmAddress and peerID from publicKey.
func (b *ParentAccountBuilder) Build() (*NeuronAccount, error) {
	// V-PARENT-02: publicKey required
	if b.publicKey == nil || isZeroPublicKey(*b.publicKey) {
		return nil, ValidationError{
			Field:    "publicKey",
			RuleCode: "V-PARENT-02",
			Message:  "Parent account must have a single NeuronPublicKey",
		}
	}

	// V-PARENT-01: DID required
	if b.did == nil || b.did.identifier == "" {
		return nil, ValidationError{
			Field:    "did",
			RuleCode: "V-PARENT-01",
			Message:  "Parent account must have a DID",
		}
	}

	// V-PARENT-03: currency required
	if b.currencySymbol == "" {
		return nil, ValidationError{
			Field:    "currencySymbol",
			RuleCode: "V-PARENT-03",
			Message:  "Parent account must have a currency symbol",
		}
	}

	// Derive EVM address from public key.
	evmAddr := b.publicKey.EVMAddress()

	// Derive PeerID from public key.
	peerID, err := b.publicKey.PeerID()
	if err != nil {
		return nil, ValidationError{
			Field:    "publicKey",
			RuleCode: "V-PARENT-02",
			Message:  "failed to derive PeerID from public key: " + err.Error(),
		}
	}

	acct := &NeuronAccount{
		accountType:    Parent,
		publicKey:      b.publicKey,
		evmAddress:     &evmAddr,
		peerID:         &peerID,
		did:            b.did,
		currencySymbol: b.currencySymbol,
	}

	if b.ledgerAttachment != nil {
		acct.ledgerAttach = b.ledgerAttachment
	}

	return acct, nil
}

// BuildComplete constructs a Parent account and additionally requires that
// a ledger attachment is present.
func (b *ParentAccountBuilder) BuildComplete() (*NeuronAccount, error) {
	acct, err := b.Build()
	if err != nil {
		return nil, err
	}

	if acct.ledgerAttach == nil {
		return nil, ValidationError{
			Field:    "ledgerAttachment",
			RuleCode: "V-PARENT-06",
			Message:  "BuildComplete requires a ledger attachment",
		}
	}

	return acct, nil
}

// ---------------------------------------------------------------------------
// ChildAccountBuilder
// ---------------------------------------------------------------------------

// ChildAccountBuilder constructs a Child NeuronAccount using a fluent API.
// A Child account requires a public key, a parent public key, and a currency symbol.
// It MUST NOT have a DID or multisigKey.
type ChildAccountBuilder struct {
	publicKey        *keylib.NeuronPublicKey
	parentPubKey     *keylib.NeuronPublicKey
	currencySymbol   string
	registryBinding  *RegistryBinding
	feePayer         *LedgerAccountId
	ledgerAttachment *LedgerAttachment
}

// NewChildAccountBuilder creates a new ChildAccountBuilder.
func NewChildAccountBuilder() *ChildAccountBuilder {
	return &ChildAccountBuilder{}
}

// WithPublicKey sets the public key for the Child account.
func (b *ChildAccountBuilder) WithPublicKey(pk keylib.NeuronPublicKey) *ChildAccountBuilder {
	b.publicKey = &pk
	return b
}

// WithParentPublicKey sets the parent's public key for the Child account.
func (b *ChildAccountBuilder) WithParentPublicKey(pk keylib.NeuronPublicKey) *ChildAccountBuilder {
	b.parentPubKey = &pk
	return b
}

// WithCurrency sets the currency symbol for the Child account.
func (b *ChildAccountBuilder) WithCurrency(symbol string) *ChildAccountBuilder {
	b.currencySymbol = symbol
	return b
}

// WithRegistryBinding sets the registry binding for the Child account.
func (b *ChildAccountBuilder) WithRegistryBinding(rb RegistryBinding) *ChildAccountBuilder {
	b.registryBinding = &rb
	return b
}

// WithFeePayer sets the fee payer ledger account ID for the Child account.
func (b *ChildAccountBuilder) WithFeePayer(fp LedgerAccountId) *ChildAccountBuilder {
	b.feePayer = &fp
	return b
}

// WithLedgerAttachment sets the ledger attachment for the Child account.
func (b *ChildAccountBuilder) WithLedgerAttachment(la LedgerAttachment) *ChildAccountBuilder {
	b.ledgerAttachment = &la
	return b
}

// FR-007: Child account validation rules
// Build constructs and validates a Child NeuronAccount.
//
// Validation rules:
//   - V-CHILD-02: publicKey MUST be set
//   - V-CHILD-01: parentPubKey MUST be set
//   - V-CHILD-03: currencySymbol MUST be set
//   - V-CHILD-04: DID MUST NOT be set (enforced by builder: not settable)
//   - V-CHILD-05: multisigKey MUST NOT be set (enforced by builder: not settable)
//
// Derives evmAddress, peerID, and p2pHost (= peerID) from publicKey.
func (b *ChildAccountBuilder) Build() (*NeuronAccount, error) {
	// V-CHILD-02: publicKey required
	if b.publicKey == nil || isZeroPublicKey(*b.publicKey) {
		return nil, ValidationError{
			Field:    "publicKey",
			RuleCode: "V-CHILD-02",
			Message:  "Child account must have a single NeuronPublicKey",
		}
	}

	// V-CHILD-01: parentPubKey required
	if b.parentPubKey == nil || isZeroPublicKey(*b.parentPubKey) {
		return nil, ValidationError{
			Field:    "parentPubKey",
			RuleCode: "V-CHILD-01",
			Message:  "Child account must reference a parent NeuronPublicKey",
		}
	}

	// V-CHILD-03: currency required
	if b.currencySymbol == "" {
		return nil, ValidationError{
			Field:    "currencySymbol",
			RuleCode: "V-CHILD-03",
			Message:  "Child account must have a currency symbol",
		}
	}

	// Derive EVM address from public key.
	evmAddr := b.publicKey.EVMAddress()

	// Derive PeerID from public key.
	peerID, err := b.publicKey.PeerID()
	if err != nil {
		return nil, ValidationError{
			Field:    "publicKey",
			RuleCode: "V-CHILD-02",
			Message:  "failed to derive PeerID from public key: " + err.Error(),
		}
	}

	// p2pHost = peerID for Child accounts.
	p2pHost := peerID

	acct := &NeuronAccount{
		accountType:    Child,
		publicKey:      b.publicKey,
		evmAddress:     &evmAddr,
		peerID:         &peerID,
		parentPubKey:   b.parentPubKey,
		currencySymbol: b.currencySymbol,
		p2pHost:        &p2pHost,
	}

	if b.registryBinding != nil {
		acct.registryBinding = b.registryBinding
	}

	if b.feePayer != nil {
		acct.feePayer = b.feePayer
	}

	if b.ledgerAttachment != nil {
		acct.ledgerAttach = b.ledgerAttachment
	}

	return acct, nil
}

// BuildComplete constructs a Child account and additionally requires that
// both a ledger attachment and a registry binding are present.
func (b *ChildAccountBuilder) BuildComplete() (*NeuronAccount, error) {
	acct, err := b.Build()
	if err != nil {
		return nil, err
	}

	if acct.ledgerAttach == nil {
		return nil, ValidationError{
			Field:    "ledgerAttachment",
			RuleCode: "V-CHILD-06",
			Message:  "BuildComplete requires a ledger attachment",
		}
	}

	if acct.registryBinding == nil {
		return nil, ValidationError{
			Field:    "registryBinding",
			RuleCode: "V-CHILD-07",
			Message:  "BuildComplete requires a registry binding",
		}
	}

	return acct, nil
}

// ---------------------------------------------------------------------------
// SharedAccountBuilder
// ---------------------------------------------------------------------------

// SharedAccountBuilder constructs a Shared NeuronAccount using a fluent API.
// A Shared account requires a multisig key and a currency symbol.
// It MUST NOT have a DID, parentPubKey, or publicKey.
type SharedAccountBuilder struct {
	multisigKey      *keylib.MultisigKey
	currencySymbol   string
	ledgerAttachment *LedgerAttachment
}

// NewSharedAccountBuilder creates a new SharedAccountBuilder.
func NewSharedAccountBuilder() *SharedAccountBuilder {
	return &SharedAccountBuilder{}
}

// WithMultisigKey sets the multisig key for the Shared account.
func (b *SharedAccountBuilder) WithMultisigKey(mk *keylib.MultisigKey) *SharedAccountBuilder {
	b.multisigKey = mk
	return b
}

// WithCurrency sets the currency symbol for the Shared account.
func (b *SharedAccountBuilder) WithCurrency(symbol string) *SharedAccountBuilder {
	b.currencySymbol = symbol
	return b
}

// WithLedgerAttachment sets the ledger attachment for the Shared account.
func (b *SharedAccountBuilder) WithLedgerAttachment(la LedgerAttachment) *SharedAccountBuilder {
	b.ledgerAttachment = &la
	return b
}

// Build constructs and validates a Shared NeuronAccount.
//
// Validation rules:
//   - V-SHARED-01: multisigKey MUST be set
//   - V-SHARED-05: currencySymbol MUST be set
//   - V-SHARED-02: DID MUST NOT be set (enforced by builder: not settable)
//   - V-SHARED-03: parentPubKey MUST NOT be set (enforced by builder: not settable)
//   - V-SHARED-04: publicKey MUST NOT be set (enforced by builder: not settable)
//
// No key derivation is performed for Shared accounts.
func (b *SharedAccountBuilder) Build() (*NeuronAccount, error) {
	// V-SHARED-01: multisigKey required
	if b.multisigKey == nil {
		return nil, ValidationError{
			Field:    "multisigKey",
			RuleCode: "V-SHARED-01",
			Message:  "Shared account must have a MultisigKey with threshold",
		}
	}

	// V-SHARED-05: currency required
	if b.currencySymbol == "" {
		return nil, ValidationError{
			Field:    "currencySymbol",
			RuleCode: "V-SHARED-05",
			Message:  "Shared account must have a currency symbol",
		}
	}

	acct := &NeuronAccount{
		accountType:    Shared,
		multisigKey:    b.multisigKey,
		currencySymbol: b.currencySymbol,
	}

	if b.ledgerAttachment != nil {
		acct.ledgerAttach = b.ledgerAttachment
	}

	return acct, nil
}

// BuildComplete constructs a Shared account and additionally requires that
// a ledger attachment is present.
func (b *SharedAccountBuilder) BuildComplete() (*NeuronAccount, error) {
	acct, err := b.Build()
	if err != nil {
		return nil, err
	}

	if acct.ledgerAttach == nil {
		return nil, ValidationError{
			Field:    "ledgerAttachment",
			RuleCode: "V-SHARED-06",
			Message:  "BuildComplete requires a ledger attachment",
		}
	}

	return acct, nil
}
