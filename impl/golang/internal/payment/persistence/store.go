package persistence

import (
	"errors"
	"time"
)

// DefaultCutoff is the default expiry window for new ActiveServiceEntry
// records per FR-P42: 24 hours. Callers MAY override via configuration.
// A zero or negative cutoff disables auto-eviction entirely (entries
// persist until explicit serviceStop/serviceCancel).
const DefaultCutoff = 24 * time.Hour

// Role identifies whether the persisted entry is the buyer side or seller
// side of the agreement. The two sides hold structurally similar entries
// but differ in which party they expect to receive lifecycle messages
// from.
type Role string

const (
	RoleBuyer  Role = "buyer"
	RoleSeller Role = "seller"
)

// ActiveServiceEntry is the persistent record per 008 FR-P40. One entry
// per (requestId, role) combination.
//
// Fields map directly to the spec's enumerated MUST-content set:
//
//   - RequestID, CounterpartEVM, Role, ServiceName, ServiceVersion:
//     identify the agreement and the counterparty.
//   - EscrowRef: opaque binding-specific reference, populated once
//     the agreement reaches FUNDED.
//   - LastConnectionSetup: the most recent ConnectionSetup payload
//     received (or sent) on the agreement. Stored as canonical JSON
//     bytes so the persistence layer doesn't need to know about
//     payment.ConnectionSetup directly; the caller marshals before
//     Save and unmarshals after Load.
//   - LastInvoiceSeq: the highest invoice sequence number seen on the
//     agreement.
//   - State: the agreement's current lifecycle state per FR-P13/P14.
//   - AcceptedAt, ExpiresAt: timestamps in nanoseconds since Unix
//     epoch. ExpiresAt drives eviction per FR-P42.
type ActiveServiceEntry struct {
	RequestID           string `json:"requestId"`
	CounterpartEVM      string `json:"counterpartEvm"`
	Role                Role   `json:"role"`
	ServiceName         string `json:"serviceName"`
	ServiceVersion      string `json:"serviceVersion"`
	EscrowRef           string `json:"escrowRef,omitempty"`
	LastConnectionSetup []byte `json:"lastConnectionSetup,omitempty"` // canonical-JSON bytes of the payment.ConnectionSetup
	LastInvoiceSeq      uint64 `json:"lastInvoiceSeq"`
	State               string `json:"state"`
	AcceptedAt          int64  `json:"acceptedAt"` // nanoseconds since Unix epoch
	ExpiresAt           int64  `json:"expiresAt"`  // nanoseconds since Unix epoch; 0 = no expiry (cutoff disabled)
}

// ActiveServiceStore is the contract every backend implements. Methods are
// expected to be safe for concurrent use by independent goroutines on
// distinct requestIds; callers that perform read-modify-write on the same
// requestId from multiple goroutines MUST serialize externally.
//
// All methods return a wrapped error with the prefix
// "payment/persistence: " on failure.
type ActiveServiceStore interface {
	// Save inserts or updates an entry. The store is expected to write
	// durably before returning (e.g., fsync on a JSON-file backend).
	Save(entry ActiveServiceEntry) error

	// Load returns the entry for the given requestId. Returns
	// (zero-value, ErrNotFound) if no such entry exists.
	Load(requestID string) (ActiveServiceEntry, error)

	// Replay returns ALL non-expired entries (those whose ExpiresAt is
	// either zero or strictly greater than now). The result is suitable
	// for callers to iterate at process startup per FR-P41.
	//
	// Entries past ExpiresAt are not returned and SHOULD be evicted at
	// the same time (Replay is allowed to imply an eviction sweep, but
	// implementations MAY defer that to a separate Evict call).
	Replay(now time.Time) ([]ActiveServiceEntry, error)

	// Evict removes entries whose ExpiresAt is non-zero AND less than or
	// equal to now. Returns the count of evicted entries.
	Evict(now time.Time) (int, error)

	// Delete removes one entry by requestId. Returns nil even if no such
	// entry exists (idempotent).
	Delete(requestID string) error
}

// ErrNotFound is returned by ActiveServiceStore.Load when the requested
// entry does not exist.
var ErrNotFound = errors.New("payment/persistence: entry not found")

// NewExpiresAt is a convenience for callers: returns the ExpiresAt timestamp
// for a new entry given acceptedAt and a cutoff duration. A non-positive
// cutoff returns 0 (no expiry).
func NewExpiresAt(acceptedAt time.Time, cutoff time.Duration) int64 {
	if cutoff <= 0 {
		return 0
	}
	return acceptedAt.Add(cutoff).UnixNano()
}
