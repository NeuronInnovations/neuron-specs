package account

import (
	"math/big"
	"sync"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// FR-013: Three valid account types (Parent, Child, Shared)
// AccountType discriminates between the three account kinds.
// The zero value (Unspecified) is invalid and will be rejected by validation.
type AccountType int

const (
	// Unspecified is the zero value and is always invalid.
	Unspecified AccountType = iota
	// Parent is the root identity account with DID and credit balance.
	Parent
	// Child is an agent/device account referencing a Parent.
	Child
	// Shared is a multisig threshold account with MultisigKey.
	Shared
)

// String returns the human-readable name of the AccountType.
func (t AccountType) String() string {
	switch t {
	case Parent:
		return "Parent"
	case Child:
		return "Child"
	case Shared:
		return "Shared"
	default:
		return "Unspecified"
	}
}

// IsValid returns true for Parent, Child, and Shared; false for Unspecified or unknown values.
func (t AccountType) IsValid() bool {
	return t == Parent || t == Child || t == Shared
}

// LedgerAccountId is an opaque identifier for a ledger account used as a fee payer.
type LedgerAccountId string

// FR-001: Parent account cryptographic identity
// FR-002: Child account with parent reference
// NeuronAccount is the primary entity representing an agent identity in the Neuron network.
// It is constructed via type-safe builders (NewParentAccountBuilder, NewChildAccountBuilder,
// NewSharedAccountBuilder) and validated via Validate().
//
// Fields are type-specific — see data-model.md for the MUST/MUST NOT matrix.
type NeuronAccount struct {
	accountType     AccountType
	publicKey       *keylib.NeuronPublicKey
	evmAddress      *keylib.EVMAddress
	peerID          *keylib.PeerID
	did             *NeuronDID
	parentPubKey    *keylib.NeuronPublicKey
	multisigKey     *keylib.MultisigKey
	currencySymbol  string
	creditBalance   *big.Int // Parent only; nil until ledger sync
	balanceAllocat  *big.Int // Child only; nil until ledger sync
	balance         *big.Int // Shared only; nil until ledger sync
	ledgerAttach    *LedgerAttachment
	registryBinding *RegistryBinding
	feePayer        *LedgerAccountId
	p2pHost         *keylib.PeerID // Child only; = peerID
	mu              sync.Mutex
}

// AccountType returns the type discriminator (Parent, Child, or Shared).
func (a *NeuronAccount) AccountType() AccountType {
	return a.accountType
}

// PublicKey returns the account's NeuronPublicKey, or nil for Shared accounts.
func (a *NeuronAccount) PublicKey() *keylib.NeuronPublicKey {
	return a.publicKey
}

// FR-008: Identity derivation from public key
// EVMAddress returns the derived EVM address, or nil for Shared accounts.
func (a *NeuronAccount) EVMAddress() *keylib.EVMAddress {
	return a.evmAddress
}

// PeerID returns the derived PeerID, or nil for Shared accounts.
func (a *NeuronAccount) PeerID() *keylib.PeerID {
	return a.peerID
}

// DID returns the NeuronDID, or nil for Child and Shared accounts.
func (a *NeuronAccount) DID() *NeuronDID {
	return a.did
}

// ParentPublicKey returns the parent's NeuronPublicKey for Child accounts, nil otherwise.
func (a *NeuronAccount) ParentPublicKey() *keylib.NeuronPublicKey {
	return a.parentPubKey
}

// FR-021: Shared accounts with MultisigKey
// MultisigKey returns the MultisigKey for Shared accounts, nil otherwise.
func (a *NeuronAccount) MultisigKey() *keylib.MultisigKey {
	return a.multisigKey
}

// FR-020: Currency symbol identification
// CurrencySymbol returns the currency symbol (e.g. "ETH", "HBAR").
func (a *NeuronAccount) CurrencySymbol() string {
	return a.currencySymbol
}

// FR-016: Balance management (credit, allocation)
// CreditBalance returns the Parent's credit balance, or nil if not yet synced.
func (a *NeuronAccount) CreditBalance() *big.Int {
	return a.creditBalance
}

// BalanceAllocation returns the Child's balance allocation, or nil if not yet synced.
func (a *NeuronAccount) BalanceAllocation() *big.Int {
	return a.balanceAllocat
}

// Balance returns the Shared account's balance, or nil if not yet synced.
func (a *NeuronAccount) Balance() *big.Int {
	return a.balance
}

// FR-018: Ledger attachment via derived EVM address
// LedgerAttachment returns the ledger attachment, or nil if not attached.
func (a *NeuronAccount) LedgerAttachment() *LedgerAttachment {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ledgerAttach
}

// FR-022: Child registry binding (identifier + external id)
// RegistryBinding returns the registry binding for Child accounts, nil otherwise.
func (a *NeuronAccount) RegistryBinding() *RegistryBinding {
	return a.registryBinding
}

// FR-026: Child fee payer for transaction sponsorship
// FeePayer returns the fee payer ledger account ID for Child accounts, nil otherwise.
func (a *NeuronAccount) FeePayer() *LedgerAccountId {
	return a.feePayer
}

// P2PHost returns the p2p host PeerID for Child accounts (= PeerID), nil otherwise.
func (a *NeuronAccount) P2PHost() *keylib.PeerID {
	return a.p2pHost
}

// SetLedgerAttachment sets or replaces the ledger attachment on this account.
func (a *NeuronAccount) SetLedgerAttachment(la LedgerAttachment) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ledgerAttach = &la
}
