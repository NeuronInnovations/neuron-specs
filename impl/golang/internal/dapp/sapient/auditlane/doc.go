// Package auditlane is a LOCAL stand-in for the Neuron 004 auditable topic lane
// (the "private audit ledger" lane in the spec-015 architecture diagram, p3).
//
// Spec 015 §B binds the low-rate, high-consequence SAPIENT control traffic to the
// auditable lane as signed TopicMessages, split across the standard 004 channels:
// inbound to the ASM (Task, AlertAck) on the ASM's stdIn; routine ASM output
// (TaskAck, StatusReport) on the ASM's stdOut; exceptional signals (Alert, Error)
// on stdErr. The full message is anchored (CL-3 / FR-S32) — not a hash.
//
// This package models exactly that channel structure (Role + Channel) and carries
// whole SapientMessages, but over a LOCAL transport — never Hedera/HCS/testnet:
//
//   - MemoryLane — in-process pub/sub for tests and the single-process demo.
//   - FileLane   — append-only NDJSON (one cat-able audit file) for the
//     cross-process demo (a seller process and a tasking CLI sharing a file).
//
// It is strictly additive and does NOT touch the 009 p2p DetectionReport path or
// the /ds240/* compatibility paths. The real 004 Topic System (signed
// TopicMessages on HCS) is the production replacement — see the package doc and
// the SAPIENT tasking local-prototype notes.
package auditlane
