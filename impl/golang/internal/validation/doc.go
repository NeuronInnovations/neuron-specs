// Package validation implements the Neuron evidence-based validation framework (Spec 010).
// It provides: EvidenceEnvelope construction and deterministic JSON serialization,
// OracleVerdict type with registry code mapping, NeuronValidatorService for agentURI
// registration, evidence publication to stdOut, and inbound evidence validation.
//
// # Architecture
//
// The validation framework is cross-cutting — it observes and assesses protocol
// compliance across all Neuron specs. Validators are standard agents (Child accounts
// per Spec 001) that register a neuron-validator service in their agentURI.
//
// Evidence envelopes are published as TopicMessage payloads on the validator's stdOut
// channel. Each envelope carries a three-outcome verdict (compliant / non-compliant /
// inconclusive) and an evidenceHash linking to off-chain supporting documentation.
//
// # Security Model
//
//   - SIGNED ENVELOPES: Every evidence envelope is wrapped in a signed TopicMessage
//     (Spec 004). The validator's NeuronPrivateKey signs the envelope, enabling
//     any observer to verify authenticity via ecrecover.
//
//   - EVIDENCE INTEGRITY: evidenceHash = keccak256(off-chain document bytes).
//     Consumers verify integrity without fetching the full document (FR-V05).
//
//   - ENVELOPE HASH: responseHash = keccak256(canonicalJSON(envelope)) links
//     on-chain Validation Registry verdicts to off-chain evidence (FR-V09).
//
//   - NO METHODOLOGY DISCLOSURE: Envelopes MUST NOT reveal validation methodology,
//     tooling, infrastructure, or network position (FR-V19).
//
//   - VALIDATOR AUTONOMY: Validators independently assess compliance. Divergent
//     verdicts from multiple validators are both accepted (SC-V08).
//
// # Dependency Chain
//
//	keylib (signing) → topic (message transport) → registry (service schema) → validation
package validation
