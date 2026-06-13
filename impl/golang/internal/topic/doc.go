// Package topic implements the Neuron topic-based communication system.
// It provides: TopicMessage construction with deterministic signing and
// canonical JSON serialization, a pluggable TopicAdapter interface for
// multiple transport backends (HCS, ERC event logs, Kafka), channel
// role management (stdIn, stdOut, stdErr, custom), and EIP-8004 service
// schema integration for peer discovery.
//
// # Architecture
//
// The topic system forms the communication substrate of the Neuron SDK:
//
//	NeuronPrivateKey (keylib) → sign → TopicMessage → TopicAdapter → backend
//	                                                                    ↓
//	                            validate ← MessageDelivery ← subscribe
//
// Messages are signed envelopes (senderAddress, signature, timestamp,
// sequenceNumber, payload) transported over pluggable backends.
//
// # Transport Adapters
//
// Three built-in adapters are provided, registered at runtime:
//
//   - HCS (Hedera Consensus Service): Primary ledger-backed transport
//     with consensus timestamps and immutable ordering.
//   - ERC Event Log: EVM-based transport using contract event emissions.
//   - Kafka: Off-ledger transport with optional anchoring configuration
//     for auditability.
//
// Custom adapters can be registered via RegisterAdapter().
//
// # Channel Roles
//
// Each agent exposes three standard channels plus optional custom channels:
//
//   - stdIn:  Inbound commands and structured signaling
//   - stdOut: Outbound state (heartbeats, status updates)
//   - stdErr: Error and diagnostic output
//   - custom:<name>: Application-defined channels
//
// Channels are independently backed — each may use a different transport.
//
// # Dependencies
//
//   - keylib: NeuronPrivateKey for signing, NeuronPublicKey for verification
//   - Spec 006: Deterministic signing (Keccak256 + RFC 6979 ECDSA R||S||V),
//     canonical JSON field ordering, wire format compliance
package topic
