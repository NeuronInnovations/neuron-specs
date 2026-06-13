// Package persistence provides the durable storage layer for
// active-service entries per 008 FR-P40–P44.
//
// An active-service entry is the minimum state required to resume serving
// (seller side) or consuming (buyer side) a 008 service across process
// restart. The Core SDK mandates the *presence* of persistence (FR-P40)
// and the *behavior* of replay-on-startup (FR-P41) and eviction past a
// configurable cutoff (FR-P42); the *format* is implementation-defined
// (FR-P43) and verified behaviorally via a restart-and-resume integration
// test rather than a wire-format conformance vector.
//
// This package provides:
//
//   - ActiveServiceEntry: the in-memory representation of one entry.
//   - ActiveServiceStore: the interface every backend implements.
//   - JSONFileStore: a single-file JSON backend with atomic temp+rename
//     writes (mirroring the existing internal/edgeapp state.go pattern).
//     Suitable for the edge demo, the reference MVP, and any deployment where
//     entry counts stay in the low hundreds.
//   - MemoryStore: a non-persistent backend useful for tests.
//
// The package intentionally does NOT couple to any specific buyer/seller
// runtime. Callers (e.g., internal/edgeapp) decide when to Save (e.g.,
// after state-machine transitions) and when to Replay (e.g., at process
// startup before re-establishing P2P connections). The store treats each
// entry's ExpiresAt as authoritative for eviction; callers populate
// ExpiresAt at entry-creation time from the agreed cutoff (default
// 86400 seconds per FR-P42, configurable).
//
// Per FR-P44, entries MUST NOT contain settlement-binding private keys
// or escrow signing material. Recovery uses the agent's existing
// NeuronPrivateKey (002) and binding-specific signing path; persisted
// state stores only the *observed agreement state*, not the means to act
// on it.
package persistence
