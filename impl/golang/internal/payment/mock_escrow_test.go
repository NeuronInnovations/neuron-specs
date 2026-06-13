package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- MemoryEscrow Tests ---

func TestMemoryEscrow_FullLifecycle(t *testing.T) {
	// Full lifecycle: create -> deposit -> request release -> approve -> verify state.
	ctx := context.Background()
	m := NewMemoryEscrow()
	m.SetClock(1000)

	agreementHash := [32]byte{0x01, 0x02, 0x03}
	evidenceHash := [32]byte{0xAA, 0xBB, 0xCC}

	// Step 1: Create escrow.
	ref, err := m.CreateEscrow(ctx, "buyer-addr", "seller-addr", nil,
		"USDC", 500, agreementHash, 2000)
	require.NoError(t, err)
	assert.Equal(t, "memory", ref.Binding)
	assert.Equal(t, "mem-escrow-1", ref.Locator)

	// Step 2: Deposit funds.
	depositRes, err := m.Deposit(ctx, ref, "1000")
	require.NoError(t, err)
	assert.Equal(t, "1000", depositRes.NewBalance)
	assert.NotEmpty(t, depositRes.TransactionRef)

	// Step 3: Verify balance.
	bal, err := m.GetBalance(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "1000", bal.Available)
	assert.Equal(t, "USDC", bal.Currency)

	// Step 4: Request partial release.
	releaseRef, err := m.RequestRelease(ctx, ref, "600", "seller-addr", evidenceHash)
	require.NoError(t, err)
	assert.Equal(t, "memory", releaseRef.Binding)
	assert.Equal(t, "mem-release-1", releaseRef.Locator)

	// Step 4b: Available balance should reflect pending release.
	bal, err = m.GetBalance(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "400", bal.Available)

	// Step 5: Approve the release.
	releaseRes, err := m.ApproveRelease(ctx, ref, releaseRef)
	require.NoError(t, err)
	assert.Equal(t, "600", releaseRes.Released)
	assert.Equal(t, "seller-addr", releaseRes.Recipient)
	assert.NotEmpty(t, releaseRes.TransactionRef)

	// Step 6: Verify final balance after release.
	bal, err = m.GetBalance(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "400", bal.Available)
}

func TestMemoryEscrow_InsufficientBalance(t *testing.T) {
	// FR-P25a: RequestRelease MUST fail when amount > available balance.
	ctx := context.Background()
	m := NewMemoryEscrow()

	ref, err := m.CreateEscrow(ctx, "buyer", "seller", nil, "USDC", 0, [32]byte{}, 9999)
	require.NoError(t, err)

	_, err = m.Deposit(ctx, ref, "100")
	require.NoError(t, err)

	// Request more than available.
	_, err = m.RequestRelease(ctx, ref, "200", "seller", [32]byte{})
	require.Error(t, err)

	var payErr *PaymentError
	require.True(t, errors.As(err, &payErr))
	assert.Equal(t, ErrInsufficientBalance, payErr.Kind())
	assert.Equal(t, "RequestRelease", payErr.Operation())
	assert.Contains(t, payErr.Error(), "requested 200 but only 100 available")
}

func TestMemoryEscrow_RefundBeforeTimeout(t *testing.T) {
	// FR-P25b: ClaimRefund MUST fail if timeout has not elapsed.
	ctx := context.Background()
	m := NewMemoryEscrow()
	m.SetClock(500) // Current time is 500.

	ref, err := m.CreateEscrow(ctx, "buyer", "seller", nil, "USDC", 0, [32]byte{}, 1000)
	require.NoError(t, err)

	_, err = m.Deposit(ctx, ref, "500")
	require.NoError(t, err)

	// Attempt refund before timeout (clock=500 < timeout=1000).
	err = m.ClaimRefund(ctx, ref)
	require.Error(t, err)

	var payErr *PaymentError
	require.True(t, errors.As(err, &payErr))
	assert.Equal(t, ErrTimeoutNotElapsed, payErr.Kind())
	assert.Contains(t, payErr.Error(), "current time 500 < timeout 1000")
}

func TestMemoryEscrow_RefundAfterTimeout(t *testing.T) {
	// FR-P22: Refund succeeds when clock >= timeout.
	ctx := context.Background()
	m := NewMemoryEscrow()

	ref, err := m.CreateEscrow(ctx, "buyer", "seller", nil, "USDC", 0, [32]byte{}, 1000)
	require.NoError(t, err)

	_, err = m.Deposit(ctx, ref, "750")
	require.NoError(t, err)

	// Advance clock past timeout.
	m.SetClock(1000)

	err = m.ClaimRefund(ctx, ref)
	require.NoError(t, err)

	// Balance should be zero after refund.
	bal, err := m.GetBalance(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "0", bal.Available)
}

func TestMemoryEscrow_InvalidEscrowRef(t *testing.T) {
	// FR-P23: Operations on a nonexistent escrow must return ErrInvalidEscrowRef.
	ctx := context.Background()
	m := NewMemoryEscrow()

	bogusRef := EscrowRef{Binding: "memory", Locator: "does-not-exist"}

	t.Run("Deposit", func(t *testing.T) {
		_, err := m.Deposit(ctx, bogusRef, "100")
		require.Error(t, err)
		var payErr *PaymentError
		require.True(t, errors.As(err, &payErr))
		assert.Equal(t, ErrInvalidEscrowRef, payErr.Kind())
	})

	t.Run("GetBalance", func(t *testing.T) {
		_, err := m.GetBalance(ctx, bogusRef)
		require.Error(t, err)
		var payErr *PaymentError
		require.True(t, errors.As(err, &payErr))
		assert.Equal(t, ErrInvalidEscrowRef, payErr.Kind())
	})

	t.Run("RequestRelease", func(t *testing.T) {
		_, err := m.RequestRelease(ctx, bogusRef, "100", "seller", [32]byte{})
		require.Error(t, err)
		var payErr *PaymentError
		require.True(t, errors.As(err, &payErr))
		assert.Equal(t, ErrInvalidEscrowRef, payErr.Kind())
	})

	t.Run("ApproveRelease", func(t *testing.T) {
		_, err := m.ApproveRelease(ctx, bogusRef, ReleaseRequestRef{Binding: "memory", Locator: "x"})
		require.Error(t, err)
		var payErr *PaymentError
		require.True(t, errors.As(err, &payErr))
		assert.Equal(t, ErrInvalidEscrowRef, payErr.Kind())
	})

	t.Run("ClaimRefund", func(t *testing.T) {
		err := m.ClaimRefund(ctx, bogusRef)
		require.Error(t, err)
		var payErr *PaymentError
		require.True(t, errors.As(err, &payErr))
		assert.Equal(t, ErrInvalidEscrowRef, payErr.Kind())
	})

	t.Run("WrongBinding", func(t *testing.T) {
		wrongRef := EscrowRef{Binding: "evm-escrow", Locator: "0xABC"}
		_, err := m.GetBalance(ctx, wrongRef)
		require.Error(t, err)
		var payErr *PaymentError
		require.True(t, errors.As(err, &payErr))
		assert.Equal(t, ErrInvalidEscrowRef, payErr.Kind())
		assert.Contains(t, payErr.Error(), "unknown binding")
	})
}

func TestMemoryEscrow_MultipleDeposits(t *testing.T) {
	// Verify cumulative balance across multiple deposits.
	ctx := context.Background()
	m := NewMemoryEscrow()

	ref, err := m.CreateEscrow(ctx, "buyer", "seller", nil, "USDC", 0, [32]byte{}, 9999)
	require.NoError(t, err)

	// First deposit.
	res1, err := m.Deposit(ctx, ref, "300")
	require.NoError(t, err)
	assert.Equal(t, "300", res1.NewBalance)

	// Second deposit.
	res2, err := m.Deposit(ctx, ref, "700")
	require.NoError(t, err)
	assert.Equal(t, "1000", res2.NewBalance)

	// Verify cumulative balance.
	bal, err := m.GetBalance(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "1000", bal.Available)
	assert.Equal(t, "USDC", bal.Currency)
}
