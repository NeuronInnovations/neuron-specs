package browserprofile

import (
	"fmt"
	"sync"
)

// EscrowState mirrors impl/typescript/src/server-demo/mock-escrow.ts:7.
type EscrowState string

const (
	EscrowProposed EscrowState = "proposed"
	EscrowFunded   EscrowState = "funded"
	EscrowReleased EscrowState = "released"
	EscrowRefunded EscrowState = "refunded"
)

// escrowEntry mirrors the TS Entry interface.
type escrowEntry struct {
	state            EscrowState
	priceAtto        string
	invoiceSha256Hex string
}

// MockEscrow is a thread-safe in-memory escrow tracker, parity with
// impl/typescript/src/server-demo/mock-escrow.ts. V1 exercises only
// proposed -> released; refunded is a terminal state accepted for
// completeness.
type MockEscrow struct {
	mu      sync.Mutex
	entries map[string]*escrowEntry
}

// NewMockEscrow creates an empty escrow.
func NewMockEscrow() *MockEscrow {
	return &MockEscrow{entries: make(map[string]*escrowEntry)}
}

// Propose transitions a fresh agreementHash into the proposed state.
// Duplicate hashes error out (matches TS throw).
func (e *MockEscrow) Propose(agreementHash, priceAtto, invoiceSha256Hex string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.entries[agreementHash]; ok {
		return fmt.Errorf("escrow already proposed for agreementHash %s", agreementHash)
	}
	e.entries[agreementHash] = &escrowEntry{
		state:            EscrowProposed,
		priceAtto:        priceAtto,
		invoiceSha256Hex: invoiceSha256Hex,
	}
	return nil
}

// State returns the current state of the given agreementHash or the empty
// string if the hash is unknown.
func (e *MockEscrow) State(agreementHash string) EscrowState {
	e.mu.Lock()
	defer e.mu.Unlock()
	if entry, ok := e.entries[agreementHash]; ok {
		return entry.state
	}
	return ""
}

// Release transitions proposed -> released. Idempotent on already-released
// entries (matches TS).
func (e *MockEscrow) Release(agreementHash string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	entry, ok := e.entries[agreementHash]
	if !ok {
		return fmt.Errorf("no such escrow: %s", agreementHash)
	}
	if entry.state == EscrowReleased {
		return nil
	}
	if entry.state != EscrowProposed {
		return fmt.Errorf("cannot release from state %s", entry.state)
	}
	entry.state = EscrowReleased
	return nil
}

// Refund transitions any non-refunded state to refunded.
func (e *MockEscrow) Refund(agreementHash string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	entry, ok := e.entries[agreementHash]
	if !ok {
		return fmt.Errorf("no such escrow: %s", agreementHash)
	}
	if entry.state == EscrowRefunded {
		return nil
	}
	entry.state = EscrowRefunded
	return nil
}
