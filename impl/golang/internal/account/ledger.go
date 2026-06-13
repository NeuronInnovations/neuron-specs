package account

import (
	"fmt"
	"math/big"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// AttachmentState represents whether an account is attached to a ledger.
type AttachmentState int

const (
	// Detached indicates the account is not attached to any ledger.
	Detached AttachmentState = iota
	// Attached indicates the account is attached to a ledger.
	Attached
)

// String returns the human-readable name of the AttachmentState.
func (s AttachmentState) String() string {
	switch s {
	case Detached:
		return "Detached"
	case Attached:
		return "Attached"
	default:
		return "Unknown"
	}
}

// VerificationStatus represents the outcome of a ledger verification check.
type VerificationStatus int

const (
	// Unverified indicates verification has not been performed.
	Unverified VerificationStatus = iota
	// Verified indicates the ledger verification succeeded.
	Verified
	// Failed indicates the ledger verification failed.
	Failed
)

// String returns the human-readable name of the VerificationStatus.
func (s VerificationStatus) String() string {
	switch s {
	case Unverified:
		return "Unverified"
	case Verified:
		return "Verified"
	case Failed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// LedgerAttachment captures the state of an account's binding to a specific ledger.
// It records which ledger the account is attached to, the on-chain address used,
// the current attachment and verification status, and the last sync timestamp.
type LedgerAttachment struct {
	ledgerIdentifier   string
	attachedAddress    keylib.EVMAddress
	state              AttachmentState
	verificationStatus VerificationStatus
	lastSyncedAt       *time.Time
}

// LedgerIdentifier returns the ledger identifier string.
func (la *LedgerAttachment) LedgerIdentifier() string { return la.ledgerIdentifier }

// AttachedAddress returns the EVM address attached to this ledger.
func (la *LedgerAttachment) AttachedAddress() keylib.EVMAddress { return la.attachedAddress }

// State returns the current attachment state.
func (la *LedgerAttachment) State() AttachmentState { return la.state }

// VerificationStatus returns the current verification status.
func (la *LedgerAttachment) VerificationStatus() VerificationStatus { return la.verificationStatus }

// LastSyncedAt returns a copy of the last sync timestamp pointer.
func (la *LedgerAttachment) LastSyncedAt() *time.Time {
	if la.lastSyncedAt == nil {
		return nil
	}
	t := *la.lastSyncedAt
	return &t
}

// VerificationResult holds the outcome and an explanatory message from a
// ledger verification operation.
type VerificationResult struct {
	Status  VerificationStatus
	Message string
}

// FR-019: Ledger attachment proof
// LedgerVerifier verifies account existence and ownership on a ledger.
// Implementations are injected at runtime to keep the account module
// transport-agnostic.
type LedgerVerifier interface {
	// VerifyAccount checks that the given EVM address exists and is owned
	// by the expected entity on the specified ledger.
	VerifyAccount(address keylib.EVMAddress, ledgerID string) (VerificationResult, error)

	// VerifyMultisigConfig checks that the multisig key configuration matches
	// the on-chain state on the specified ledger.
	VerifyMultisigConfig(mk *keylib.MultisigKey, ledgerID string) (VerificationResult, error)
}

// FR-017: Parent-child relationship verification
// ParentChildVerifier verifies the parent-child relationship against on-ledger data.
// This is used during Child account construction to confirm the claimed parent
// relationship is recorded on-chain.
type ParentChildVerifier interface {
	// VerifyRelationship checks that the child address is registered as a child
	// of the parent address on the ledger.
	VerifyRelationship(childAddress, parentAddress keylib.EVMAddress) (VerificationResult, error)
}

// VerifyParentChild verifies the parent-child relationship for a Child account
// by delegating to the provided ParentChildVerifier.
//
// The child account must be of type Child, and both the child's EVM address and
// the parent's public key (from which the parent address is derived) must be present.
func VerifyParentChild(child *NeuronAccount, verifier ParentChildVerifier) (VerificationResult, error) {
	if child == nil {
		return VerificationResult{}, fmt.Errorf("child account must not be nil")
	}

	if child.accountType != Child {
		return VerificationResult{}, fmt.Errorf("account type must be Child, got %s", child.accountType)
	}

	if child.evmAddress == nil {
		return VerificationResult{}, fmt.Errorf("child account has no EVM address")
	}

	if child.parentPubKey == nil {
		return VerificationResult{}, fmt.Errorf("child account has no parent public key")
	}

	parentAddr := child.parentPubKey.EVMAddress()

	return verifier.VerifyRelationship(*child.evmAddress, parentAddr)
}

// VerifyLedgerAttachment verifies this account's ledger attachment by delegating
// to the provided LedgerVerifier.
//
// For Parent and Child accounts, it uses VerifyAccount with the account's EVM address.
// For Shared accounts, it uses VerifyMultisigConfig with the account's MultisigKey.
//
// Returns an error if the account is not attached to a ledger.
func (a *NeuronAccount) VerifyLedgerAttachment(verifier LedgerVerifier) (VerificationResult, error) {
	if a.ledgerAttach == nil {
		return VerificationResult{}, fmt.Errorf("account has no ledger attachment")
	}

	if a.ledgerAttach.state != Attached {
		return VerificationResult{}, fmt.Errorf("account is not attached to a ledger")
	}

	ledgerID := a.ledgerAttach.ledgerIdentifier

	switch a.accountType {
	case Parent, Child:
		if a.evmAddress == nil {
			return VerificationResult{}, fmt.Errorf("account has no EVM address for verification")
		}
		result, err := verifier.VerifyAccount(*a.evmAddress, ledgerID)
		if err != nil {
			return VerificationResult{}, err
		}
		a.ledgerAttach.verificationStatus = result.Status
		return result, nil

	case Shared:
		if a.multisigKey == nil {
			return VerificationResult{}, fmt.Errorf("shared account has no multisig key for verification")
		}
		result, err := verifier.VerifyMultisigConfig(a.multisigKey, ledgerID)
		if err != nil {
			return VerificationResult{}, err
		}
		a.ledgerAttach.verificationStatus = result.Status
		return result, nil

	default:
		return VerificationResult{}, fmt.Errorf("cannot verify ledger attachment for account type %s", a.accountType)
	}
}

// AttachToLedger sets the ledger attachment on this account and marks the state
// as Attached.
func (a *NeuronAccount) AttachToLedger(la LedgerAttachment) {
	la.state = Attached
	a.ledgerAttach = &la
}

// DetachFromLedger removes the ledger attachment from this account, setting the
// state to Detached and clearing all balance fields (creditBalance, balanceAllocation,
// balance) to nil.
func (a *NeuronAccount) DetachFromLedger() {
	if a.ledgerAttach != nil {
		a.ledgerAttach.state = Detached
		a.ledgerAttach.verificationStatus = Unverified
		a.ledgerAttach.lastSyncedAt = nil
	}
	// Clear all balance fields on detach.
	a.creditBalance = (*big.Int)(nil)
	a.balanceAllocat = (*big.Int)(nil)
	a.balance = (*big.Int)(nil)
}
