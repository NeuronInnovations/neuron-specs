package payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- T009: EscrowAdapter Interface & Value Types ---

func TestEscrowRef_Construction(t *testing.T) {
	// FR-P23: EscrowRef is opaque with binding + locator.
	ref := EscrowRef{Binding: "evm-escrow", Locator: "0xContractAddress:42"}
	assert.Equal(t, "evm-escrow", ref.Binding)
	assert.Equal(t, "0xContractAddress:42", ref.Locator)
}

func TestReleaseRequestRef_Construction(t *testing.T) {
	// FR-P23: Same structure as EscrowRef.
	ref := ReleaseRequestRef{Binding: "hedera-native", Locator: "0.0.12345:schedule:1"}
	assert.Equal(t, "hedera-native", ref.Binding)
	assert.Equal(t, "0.0.12345:schedule:1", ref.Locator)
}

func TestBalance_Construction(t *testing.T) {
	// FR-P19: Balance with decimal string, currency, timestamp.
	bal := Balance{Available: "1000000", Currency: "USDC", LastSynced: 1700000000}
	assert.Equal(t, "1000000", bal.Available)
	assert.Equal(t, "USDC", bal.Currency)
	assert.Equal(t, uint64(1700000000), bal.LastSynced)
}

func TestDepositResult_Construction(t *testing.T) {
	// FR-P18: DepositResult with transactionRef and newBalance.
	result := DepositResult{TransactionRef: "0xtxhash", NewBalance: "2000000"}
	assert.Equal(t, "0xtxhash", result.TransactionRef)
	assert.Equal(t, "2000000", result.NewBalance)
}

func TestReleaseResult_Construction(t *testing.T) {
	// FR-P21: ReleaseResult with transactionRef, released amount, recipient.
	result := ReleaseResult{
		TransactionRef: "0xtxhash",
		Released:       "500000",
		Recipient:      "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
	}
	assert.Equal(t, "500000", result.Released)
	assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28", result.Recipient)
}

// --- T034: Escrow Precondition Error Tests ---

func TestEscrowPreconditionErrors(t *testing.T) {
	// FR-P22, FR-P25a, FR-P25b: Settlement precondition errors.

	t.Run("RefundNotEligible error", func(t *testing.T) {
		err := NewPaymentError(ErrRefundNotEligible, "ClaimRefund", "timeout has not elapsed")
		assert.Equal(t, ErrRefundNotEligible, err.Kind())
		assert.Contains(t, err.Error(), "RefundNotEligible")
	})

	t.Run("TimeoutNotElapsed error", func(t *testing.T) {
		err := NewPaymentError(ErrTimeoutNotElapsed, "ClaimRefund", "current time < timeout")
		assert.Equal(t, ErrTimeoutNotElapsed, err.Kind())
		assert.Contains(t, err.Error(), "TimeoutNotElapsed")
	})

	t.Run("InsufficientBalance error (FR-P25a)", func(t *testing.T) {
		// SC-P09: requestRelease MUST fail when amount > available.
		err := NewPaymentError(ErrInsufficientBalance, "RequestRelease",
			"requested 100 but only 50 available")
		assert.Equal(t, ErrInsufficientBalance, err.Kind())
		assert.Contains(t, err.Error(), "InsufficientBalance")
		assert.Contains(t, err.Error(), "RequestRelease")
	})

	t.Run("InvalidEscrowRef error", func(t *testing.T) {
		err := NewPaymentError(ErrInvalidEscrowRef, "GetBalance", "escrow not found")
		assert.Equal(t, ErrInvalidEscrowRef, err.Kind())
	})
}
