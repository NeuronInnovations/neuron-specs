# Research: Protocol Determinism

**Branch**: `006-protocol-determinism` | **Date**: 2026-03-03 | **Source**: spec.md, machine-determinism-audit.md

---

## Design Decisions

### R1 — Signing Responsibility (Audit C-5, X-1)

**Question**: Who constructs and signs the TopicMessage — the caller before passing to `Publish()`, or the adapter inside `Publish()`?

**Context**: Two contradictory statements exist:
- `specs/005-health/contracts/health-publisher.md`: "Wrap payload into TopicMessage envelope — **adapter handles signing** (FR-T03)"
- `specs/004-topic-system/contracts/topic-adapter.md`: "**Validate msg has a valid signature** (FR-T03). Reject with InvalidSignature if unsigned or invalid."

**Evidence**: The signing responsibility follows separation of concerns: the entity with access to private key material (the caller) signs; the transport layer (the adapter) validates but never holds key material. The adapter's contract (spec 004) requires "validate msg has a valid signature", which implies it receives a pre-signed message. The health publisher's contract (spec 005) must therefore sign before calling Publish().

**Decision**: **Caller signs before `Publish()`; adapter validates, never signs.** The caller (e.g., health publisher) constructs the TopicMessage, signs it with the sender's NeuronPrivateKey, and then passes the signed message to `TopicAdapter.Publish()`. The adapter validates the signature and rejects unsigned or invalid messages with `InvalidSignature`.

**Rationale**: The adapter is a transport abstraction — it should not need access to private key material. The caller has the key and knows the message content. Separation of concerns: signing = caller responsibility, transport = adapter responsibility.

---

### R2 — uint64 JSON Encoding (Audit B-1)

**Question**: Should uint64 fields (`timestamp`, `sequenceNumber`, `nextHeartbeatDeadline`) be encoded as JSON numbers or JSON strings?

**Context**: Many JSON parsers (notably JavaScript's `JSON.parse`) lose precision for integers above 2^53 (~9.007×10^18). Current Neuron timestamps are nanoseconds since epoch (~1.7×10^18) which is above 2^53. Sequence numbers could theoretically exceed 2^53 in long-running systems.

**Options considered**:
1. JSON number (Go default) — simple but breaks JavaScript precision
2. JSON string (decimal) — safe for all languages, minor serialization overhead
3. JSON string for timestamp only, number for sequenceNumber — inconsistent

**Decision**: **JSON string for all uint64 fields.** All uint64 values (`timestamp`, `sequenceNumber`, `nextHeartbeatDeadline`, protocol constants when serialized) MUST be encoded as JSON strings containing the decimal representation. No leading zeros except for the value `"0"`.

**Rationale**: Consistency (all uint64 fields use the same rule), safety (no precision loss in any language), and simplicity (one rule to remember). The overhead of quoting numbers is negligible. This is a protocol-level decision; each language's implementation must ensure uint64 values serialize as JSON strings.

---

### R3 — Optional Field Null vs Omission (Audit B-3)

**Question**: When an optional field is absent, should the JSON contain `"field": null` or omit the key entirely?

**Context**: HeartbeatPayload has optional fields (`capabilities`, `location`, `peers`). Different representations produce different bytes → different Keccak256 hashes → different signatures.

**Options considered**:
1. Omit absent fields — produces smaller JSON, simpler canonical form
2. Serialize as `null` — makes field presence explicit, but larger JSON and requires explicit null handling in most languages
3. Mixed (omit for some, null for others) — inconsistent

**Decision**: **Omit absent fields.** Optional fields that are absent/nil MUST be omitted from the JSON object entirely. They MUST NOT be serialized as `null`.

**Rationale**: Omitting absent fields produces a simpler canonical form with fewer bytes (important for on-chain cost). All languages can implement "don't include the key" trivially — it's simpler than "include the key with null".

---

### R4 — base64 Variant (Audit B-2, A-5)

**Question**: Which base64 variant should be used for binary fields (`signature`, `payload`) in JSON?

**Context**: Multiple base64 variants exist: standard (`+/=`), URL-safe (`-_`), with/without padding.

**Options considered**:
1. RFC 4648 §4 standard (`+/=`) — most common, widest library support
2. RFC 4648 §5 URL-safe (`-_`) — better for URLs, less common in JSON
3. RFC 4648 §4 without padding — compact but less universally supported

**Decision**: **RFC 4648 §4 standard alphabet with padding.** Binary fields MUST use base64 with alphabet `A-Za-z0-9+/` and `=` padding.

**Rationale**: RFC 4648 §4 standard base64 is the most widely supported variant across all languages and is the default in most JSON libraries. It is explicitly defined in the base64 RFC as the standard encoding.

---

### R5 — Constitution Principle VI Reconciliation

**Question**: How does a language-neutral protocol spec reconcile with Constitution Principle VI ("Golang-First SDK")?

**Context**: Principle VI states "Go is the primary implementation language." This spec defines the protocol in language-neutral terms, which appears to conflict.

**Decision**: **No conflict.** Principle VI governs the SDK implementation priority, not the protocol definition. The spec says "Go-first SDK" — meaning Go is the first language in which the SDK is implemented. It does not say "Go-only protocol." This spec defines the protocol that all SDKs (Go first, then others) implement. Test vectors are mathematically derived from the algorithm definitions in this spec and can be independently computed by any implementation. Cross-verification across two or more languages confirms correctness.

**Analogy**: HTTP/2 (RFC 7540) is a language-neutral protocol spec. The fact that Google's reference implementation was in C++ doesn't make the RFC "C++-first" — it means C++ was simply the first implementation.

---

### R6 — Test Vector Generation Approach

**Question**: How should golden test vectors be generated and verified?

**Options considered**:
1. Hand-calculate all values (error-prone, time-consuming)
2. Compute from algorithm definitions and verify in one language (moderate confidence)
3. Compute from algorithm definitions and verify across two or more languages (highest confidence)

**Decision**: **Compute from algorithm definitions. Verify by implementing the derivation chain in at least two languages and confirming byte-identical results.**

**Process**:
1. Choose well-known test private keys (e.g., `0x0000...0001` for deterministic results)
2. Implement the algorithm steps from `algorithm-reference.md` in any language
3. Record every intermediate byte value in hex
4. Cross-verify critical values (EVM address, Keccak256 hash, ECDSA signature) against a second independent implementation in a different language
5. Document the complete chain in `contracts/test-vectors.md`

**Rationale**: The algorithm definitions in this spec are the source of truth. Any conforming implementation produces the same byte values. Cross-verification with a second language catches implementation-specific bugs. Using simple, deterministic private keys (like `0x01`) makes test vectors reproducible.
