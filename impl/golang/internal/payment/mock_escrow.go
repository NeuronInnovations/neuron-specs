package payment

import (
	"context"
	"fmt"
	"math/big"
	"sync"
)

// pendingRelease represents a release request awaiting approval.
type pendingRelease struct {
	amount       *big.Int
	recipient    string
	evidenceHash [32]byte
}

// memoryEscrowState holds the full state of a single in-memory escrow instance.
type memoryEscrowState struct {
	buyer         string
	seller        string
	arbiter       *string
	currency      string
	threshold     uint64
	agreementHash [32]byte
	timeout       uint64
	balance       *big.Int
	released      *big.Int
	state         string // "created", "funded", "released", "refunded"
	releases      map[string]*pendingRelease
}

// pendingTotal returns the sum of all pending (not yet approved) release amounts.
func (s *memoryEscrowState) pendingTotal() *big.Int {
	total := new(big.Int)
	for _, r := range s.releases {
		total.Add(total, r.amount)
	}
	return total
}

// available returns balance minus the sum of pending release amounts.
func (s *memoryEscrowState) available() *big.Int {
	return new(big.Int).Sub(s.balance, s.pendingTotal())
}

// MemoryEscrow is an in-memory implementation of EscrowAdapter for testing
// and demo purposes. It is safe for concurrent use.
//
// Not intended for production — all state is lost when the process exits.
type MemoryEscrow struct {
	mu        sync.Mutex
	escrows   map[string]*memoryEscrowState
	nextID    int
	releaseID int
	clock     uint64
}

// NewMemoryEscrow creates a new in-memory escrow adapter.
func NewMemoryEscrow() *MemoryEscrow {
	return &MemoryEscrow{
		escrows: make(map[string]*memoryEscrowState),
	}
}

// SetClock sets the simulated current time (Unix epoch seconds) used for
// timeout comparisons. This allows tests to control time without wall-clock
// dependencies.
func (m *MemoryEscrow) SetClock(now uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clock = now
}

// lookup retrieves an escrow state by ref, returning a PaymentError if not found.
// Caller must hold m.mu.
func (m *MemoryEscrow) lookup(ref EscrowRef, operation string) (*memoryEscrowState, error) {
	if ref.Binding != "memory" {
		return nil, NewPaymentError(ErrInvalidEscrowRef, operation,
			fmt.Sprintf("unknown binding %q, expected \"memory\"", ref.Binding))
	}
	state, ok := m.escrows[ref.Locator]
	if !ok {
		return nil, NewPaymentError(ErrInvalidEscrowRef, operation,
			fmt.Sprintf("escrow %q not found", ref.Locator))
	}
	return state, nil
}

// CreateEscrow creates a new escrow instance in memory. FR-P17.
func (m *MemoryEscrow) CreateEscrow(_ context.Context, buyer, seller string, arbiter *string,
	currency string, threshold uint64, agreementHash [32]byte, timeout uint64) (EscrowRef, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	locator := fmt.Sprintf("mem-escrow-%d", m.nextID)

	m.escrows[locator] = &memoryEscrowState{
		buyer:         buyer,
		seller:        seller,
		arbiter:       arbiter,
		currency:      currency,
		threshold:     threshold,
		agreementHash: agreementHash,
		timeout:       timeout,
		balance:       new(big.Int),
		released:      new(big.Int),
		state:         "created",
		releases:      make(map[string]*pendingRelease),
	}

	return EscrowRef{Binding: "memory", Locator: locator}, nil
}

// Deposit adds funds to an existing escrow. FR-P18.
func (m *MemoryEscrow) Deposit(_ context.Context, escrowRef EscrowRef, amount string) (DepositResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.lookup(escrowRef, "Deposit")
	if err != nil {
		return DepositResult{}, err
	}

	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok || amt.Sign() <= 0 {
		return DepositResult{}, NewPaymentError(ErrEscrowCreationFailed, "Deposit",
			fmt.Sprintf("invalid deposit amount %q", amount))
	}

	state.balance.Add(state.balance, amt)
	if state.state == "created" {
		state.state = "funded"
	}

	txRef := fmt.Sprintf("mem-tx-deposit-%s-%s", escrowRef.Locator, amount)

	return DepositResult{
		TransactionRef: txRef,
		NewBalance:     state.balance.String(),
	}, nil
}

// GetBalance returns the current available balance of an escrow. FR-P19.
// Available = balance - sum(pending release amounts).
func (m *MemoryEscrow) GetBalance(_ context.Context, escrowRef EscrowRef) (Balance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.lookup(escrowRef, "GetBalance")
	if err != nil {
		return Balance{}, err
	}

	return Balance{
		Available:  state.available().String(),
		Currency:   state.currency,
		LastSynced: m.clock,
	}, nil
}

// RequestRelease records a pending release request. FR-P20, FR-P25a.
// Fails if amount > available balance.
func (m *MemoryEscrow) RequestRelease(_ context.Context, escrowRef EscrowRef, amount string,
	recipient string, evidenceHash [32]byte) (ReleaseRequestRef, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.lookup(escrowRef, "RequestRelease")
	if err != nil {
		return ReleaseRequestRef{}, err
	}

	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok || amt.Sign() <= 0 {
		return ReleaseRequestRef{}, NewPaymentError(ErrInsufficientBalance, "RequestRelease",
			fmt.Sprintf("invalid release amount %q", amount))
	}

	avail := state.available()
	if amt.Cmp(avail) > 0 {
		// FR-P25a: MUST fail if amount > available balance.
		return ReleaseRequestRef{}, NewPaymentError(ErrInsufficientBalance, "RequestRelease",
			fmt.Sprintf("requested %s but only %s available", amt.String(), avail.String()))
	}

	m.releaseID++
	releaseLocator := fmt.Sprintf("mem-release-%d", m.releaseID)

	state.releases[releaseLocator] = &pendingRelease{
		amount:       amt,
		recipient:    recipient,
		evidenceHash: evidenceHash,
	}

	return ReleaseRequestRef{Binding: "memory", Locator: releaseLocator}, nil
}

// ApproveRelease authorizes a pending release, transferring funds. FR-P21.
func (m *MemoryEscrow) ApproveRelease(_ context.Context, escrowRef EscrowRef,
	releaseRef ReleaseRequestRef) (ReleaseResult, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.lookup(escrowRef, "ApproveRelease")
	if err != nil {
		return ReleaseResult{}, err
	}

	pending, ok := state.releases[releaseRef.Locator]
	if !ok {
		return ReleaseResult{}, NewPaymentError(ErrReleaseNotAuthorized, "ApproveRelease",
			fmt.Sprintf("release request %q not found", releaseRef.Locator))
	}

	// Move from pending to released.
	state.balance.Sub(state.balance, pending.amount)
	state.released.Add(state.released, pending.amount)
	delete(state.releases, releaseRef.Locator)

	state.state = "released"

	txRef := fmt.Sprintf("mem-tx-release-%s-%s", escrowRef.Locator, releaseRef.Locator)

	return ReleaseResult{
		TransactionRef: txRef,
		Released:       pending.amount.String(),
		Recipient:      pending.recipient,
	}, nil
}

// ClaimRefund returns all remaining funds to the buyer after timeout. FR-P22, FR-P25b.
// Fails if the simulated clock has not reached the escrow's timeout.
func (m *MemoryEscrow) ClaimRefund(_ context.Context, escrowRef EscrowRef) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.lookup(escrowRef, "ClaimRefund")
	if err != nil {
		return err
	}

	// FR-P25b: MUST fail if timeout has not elapsed.
	if m.clock < state.timeout {
		return NewPaymentError(ErrTimeoutNotElapsed, "ClaimRefund",
			fmt.Sprintf("current time %d < timeout %d", m.clock, state.timeout))
	}

	state.state = "refunded"
	state.balance.SetInt64(0)
	// Clear any pending releases.
	state.releases = make(map[string]*pendingRelease)

	return nil
}

// Compile-time assertion: MemoryEscrow implements EscrowAdapter.
var _ EscrowAdapter = (*MemoryEscrow)(nil)
