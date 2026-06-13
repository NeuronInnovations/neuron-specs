// Package health implements the Neuron onchain liveness and health status protocol.
// It provides: HeartbeatPayload construction and deterministic JSON serialization,
// publisher-side validation (V-PUB-01..07), observer-side validation (V-OBS-01..06),
// and a five-state liveness machine (UNKNOWN, ALIVE, SUSPECT, DEAD, OFFLINE).
//
// # Security Model (T077)
//
// The health protocol operates over PUBLIC channels. The following security
// properties and limitations apply:
//
//   - NO ENCRYPTION (FR-H26): HeartbeatPayload messages are transmitted in
//     cleartext over public topic channels (e.g., HCS, EVM event logs). Any
//     observer with access to the topic can read the heartbeat content, including
//     the declared deadline, role, capabilities, location, and peers data.
//     Applications must NOT include sensitive or private data in heartbeat fields.
//
//   - GOSSIP-GRADE PEERS DATA (FR-H27): The peers field in HeartbeatPayload
//     is informational gossip data ONLY. It carries abbreviated peer addresses
//     as 4-character hex strings for neighbor discovery hints. This data MUST NOT
//     be used for trust decisions, routing decisions, or identity verification.
//     An attacker could publish heartbeats with fabricated peer lists. The observer
//     treats peers as ephemeral metadata — it is never stored in the LivenessRecord
//     and does not influence liveness evaluation.
//
//   - SIGNATURE VERIFICATION (Spec 002 keylib): Every HeartbeatPayload is wrapped
//     in a signed TopicMessage (Spec 004). The observer validates the ECDSA
//     signature using secp256k1 ecrecover (V-OBS-01) and verifies that the
//     recovered signer address matches the declared SenderAddress in the
//     TopicMessage envelope. This prevents forgery — an attacker cannot publish
//     heartbeats on behalf of another agent without possessing their private key.
//     Signatures use RFC 6979 deterministic nonces (Constitution X) and
//     Keccak256 hashing, producing deterministic R||S||V output.
//
//   - NO TRUST ESTABLISHMENT: Observing a heartbeat does NOT establish a trust
//     relationship with the sender. The heartbeat proves the sender possesses a
//     specific private key and is alive at the declared timestamp. It does NOT
//     prove the sender's identity, reputation, authorization, or good behavior.
//     Trust relationships must be established through separate mechanisms
//     (e.g., Spec 003 Peer Registry, on-chain reputation contracts).
//
//   - CONSENSUS TIMESTAMP AUTHORITY (FR-H16): All deadline arithmetic uses the
//     consensus-assigned timestamp (e.g., HCS consensus, EVM block.timestamp),
//     never the local clock. This prevents clock-skew attacks where a malicious
//     sender could manipulate their local clock to extend liveness windows.
//
//   - SEQUENCE ORDERING (FR-H17): The observer enforces strict sequence ordering.
//     Messages with sequence numbers equal to or lower than the last processed
//     sequence are silently ignored. This prevents replay attacks where an
//     attacker resubmits old heartbeat messages to reset liveness state.
package health
