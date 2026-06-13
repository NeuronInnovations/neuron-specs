package payment

import "context"

// EscrowRef is an opaque reference to a settlement escrow instance.
// FR-P23: binding + locator, both binding-specific.
type EscrowRef struct {
	Binding string // e.g., "hedera-native", "evm-escrow"
	Locator string // binding-specific (e.g., account ID, contract address)
}

// ReleaseRequestRef is an opaque reference to a pending release request.
// FR-P23: Same structure as EscrowRef.
type ReleaseRequestRef struct {
	Binding string
	Locator string
}

// Balance represents an escrow balance query result. FR-P19.
type Balance struct {
	Available  string // decimal string
	Currency   string
	LastSynced uint64 // Unix epoch seconds (FR-W02a)
}

// DepositResult is the result of a deposit operation. FR-P18.
type DepositResult struct {
	TransactionRef string
	NewBalance     string // decimal string
}

// ReleaseResult is the result of an approved release. FR-P21.
type ReleaseResult struct {
	TransactionRef string
	Released       string // decimal string
	Recipient      string // EVM address
}

// EscrowAdapter is the abstract interface for settlement bindings.
// FR-P16: All settlement bindings MUST implement these six operations.
type EscrowAdapter interface {
	// CreateEscrow creates a new escrow instance. FR-P17.
	// agreementHash = keccak256(canonicalJSON(acceptedServiceResponse)).
	// timeout = Unix epoch seconds for refund eligibility.
	CreateEscrow(ctx context.Context, buyer, seller string, arbiter *string,
		currency string, threshold uint64, agreementHash [32]byte, timeout uint64) (EscrowRef, error)

	// Deposit adds funds to the escrow. FR-P18.
	Deposit(ctx context.Context, escrowRef EscrowRef, amount string) (DepositResult, error)

	// GetBalance queries the current escrow balance. FR-P19.
	GetBalance(ctx context.Context, escrowRef EscrowRef) (Balance, error)

	// RequestRelease records a pending release request. FR-P20.
	// evidenceHash = keccak256(canonicalJSON(deliveryProofTopicMessage)).
	// FR-P25a: MUST fail if amount > available balance.
	RequestRelease(ctx context.Context, escrowRef EscrowRef, amount string,
		recipient string, evidenceHash [32]byte) (ReleaseRequestRef, error)

	// ApproveRelease authorizes fund transfer. FR-P21.
	ApproveRelease(ctx context.Context, escrowRef EscrowRef,
		releaseRef ReleaseRequestRef) (ReleaseResult, error)

	// ClaimRefund returns funds to buyer after timeout. FR-P22.
	// FR-P25b: MUST fail if timeout has not elapsed.
	ClaimRefund(ctx context.Context, escrowRef EscrowRef) error
}
