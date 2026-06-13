# Contract: Amendment Log

**Spec**: 006-protocol-determinism | **Date**: 2026-03-03
**Scope**: Traceability matrix mapping every audit item to its resolution in spec 006
**Input**: `specs/machine-determinism-audit.md`

---

## Category A: Algorithm by Go Library Reference

| Audit ID | Description | Resolved By | Status |
|----------|-------------|-------------|--------|
| A-1 | PeerID derivation is a Go library call, not an algorithm | `algorithm-reference.md` §5 (FR-A05) — full byte-level algorithm: protobuf wrap → multihash → identity-vs-SHA256 → base58btc | Resolved |
| A-2 | EIP-55 referenced by name only | `algorithm-reference.md` §4 (FR-A04) — full 4-step algorithm with nibble comparison | Resolved |
| A-3 | ToBlockchainKey/FromBlockchainKey accepts Go `interface{}` | `data-model.md` Go-ism Replacement Table (FR-X02) — defines accepted byte formats | Resolved |
| A-4 | Canonical JSON previously defined by implementation runtime behavior, not protocol rules | `wire-format.md` §1–§2 (FR-W01, FR-W05) — explicit compact JSON rules and field ordering | Resolved |
| A-5 | base64 variant unspecified | `wire-format.md` §4 (FR-W03) — RFC 4648 §4 standard alphabet with `=` padding | Resolved |
| A-6 | Multicodec varint encoding not explained | `algorithm-reference.md` §6 (FR-A06) — `0xe7 0x01` literal bytes explained as unsigned-LEB128 of value 231 | Resolved |
| A-7 | Ed25519 detection criteria missing | `algorithm-reference.md` §14 (FR-A14) — detection by OID, protobuf KeyType, prefix byte, and raw byte handling | Resolved |

---

## Category B: Wire Format Ambiguities

| Audit ID | Description | Resolved By | Status |
|----------|-------------|-------------|--------|
| B-1 | uint64 JSON encoding (number vs string) | `wire-format.md` §3 (FR-W02) — JSON string for all uint64 fields | Resolved |
| B-2 | base64 variant unspecified | `wire-format.md` §4 (FR-W03) — RFC 4648 §4 with padding | Resolved |
| B-3 | Optional field null vs omission | `wire-format.md` §5 (FR-W04) — omit absent fields, never null | Resolved |
| B-4 | big.Int serialization unspecified | `wire-format.md` §7 (FR-W07) — JSON string decimal, omit if nil | Resolved |
| B-5 | senderAddress EIP-55 vs lowercase | `wire-format.md` §6 (FR-W06, FR-W09) — EIP-55 checksummed | Resolved |

---

## Category C: Missing Specifications

| Audit ID | Description | Resolved By | Status |
|----------|-------------|-------------|--------|
| C-1 | DID Document JSON schema undefined | — | Deferred |
| C-2 | UTF-8 encoding never mandated | `wire-format.md` §9 (FR-W08) — explicit UTF-8 mandate for all string-to-bytes conversions | Resolved |
| C-3 | No test vectors | `test-vectors.md` (FR-V01–FR-V04) — 4 golden chains with intermediate values | Resolved |
| C-4 | Peer Registry has no implementable interface | — | Deferred |
| C-5 | Signing responsibility contradiction (004 ↔ 005) | `spec.md` FR-X01, `research.md` R1 — caller signs, adapter validates | Resolved |
| C-6 | Argon2id default parameters missing | `algorithm-reference.md` §11 (FR-A11) — time=1, memory=65536, threads=4, salt=16, tag=32 (corrected to match spec 002 canonical values) | Resolved |
| C-7 | secp256k1 point compression parity rule missing | `algorithm-reference.md` §2 (FR-A02) — 0x02 even Y, 0x03 odd Y | Resolved |

### Deferred Items — Justification

**C-1 — DID Document JSON Schema**: The DID document structure is a spec 001 concern that requires its own clarification cycle. Spec 006 provides the `did:key` identifier construction (FR-A06) which is sufficient for the current SDK scope. The DID document schema should be addressed in a spec 001 amendment when the Parent account creation flow is fully specified. The `did:key` spec (W3C) implies a minimal DID document can be deterministically derived from the `did:key` identifier, but the Neuron-specific document structure (with `service` endpoints for topic system) is not yet defined.

**C-4 — Peer Registry Interface**: Spec 003 has placeholder user stories and no implementable interface. This is a full-spec gap, not a determinism gap. It requires a complete rewrite of spec 003 as a separate effort. Spec 006 provides the registration bridge concept (FR-X04) mapping account data to registry inputs, but cannot define the registry interface itself.

---

## Category D: Implementation-Specific Types Requiring Language-Neutral Abstraction

> **Note**: The audit IDs below reference implementation-specific types from the original audit. The resolutions in spec 006 define protocol-level equivalents.

| Audit ID | Description | Resolved By | Status |
|----------|-------------|-------------|--------|
| D-1 | `<-chan MessageDelivery` | `data-model.md` Go-ism Replacement Table (FR-X02) — async stream/iterator | Resolved |
| D-2 | `map[string]interface{}` for Config | `data-model.md` Go-ism Replacement Table (FR-X02) — JSON object | Resolved |
| D-3 | `*big.Int (nilable)` | `data-model.md` Go-ism Replacement Table + `wire-format.md` §7 (FR-W07, FR-X02) — optional arbitrary-precision integer | Resolved |
| D-4 | `interface{}` for blockchain key | `data-model.md` Go-ism Replacement Table (FR-X02) — raw 32-byte scalar or typed ECDSA key | Resolved |
| D-5 | `error` return type | `data-model.md` Go-ism Replacement Table + `error-taxonomy.md` (FR-X02) — structured error with code/message/cause | Resolved |
| D-6 | `CreateTopicOpts` / `PublishOpts` structs | `data-model.md` Go-ism Replacement Table (FR-X02) — options object with named fields | Resolved |

---

## Cross-Spec Gaps

| Audit ID | Description | Resolved By | Status |
|----------|-------------|-------------|--------|
| X-1 | Signing responsibility contradiction (004 ↔ 005) | `spec.md` FR-X01, `research.md` R1 — caller signs, adapter validates | Resolved |
| X-2 | No test vectors | `test-vectors.md` (FR-V01–FR-V04) — 4 golden chains | Resolved |
| X-3 | No wire format specification document | `wire-format.md` (FR-W01–FR-W10) — complete normative wire format contract | Resolved |
| X-4 | Registration operations bridge (001 ↔ 003) | `spec.md` FR-X04 — defines mapping from account data to registry inputs | Partially Resolved |
| X-5 | No unified error taxonomy | `error-taxonomy.md` — cross-spec error code namespace | Resolved |
| X-6 | External standards not inlined | `algorithm-reference.md` §1–§14 (FR-X03) — all algorithms described at byte level | Resolved |
| **A-7** | **Commerce + DApp canonical payload vectors required** (2026-05-08 amendment). Adds 9 commerce payload types (6 pre-existing per 008 FR-P06, 3 new per 008 FR-P36/P37/P38) plus StreamCatalogEntry (008 FR-P33a) plus RemoteIdFrame (017 FR-R05) to wire-format §2 and test-vectors Chain 5 / Chain 6. | `wire-format.md` §2 (11 new canonical orderings); `test-vectors.md` Chain 5 (10 entries) + Chain 6 (2 entries) with deterministic inputs; `TODO(impl-generated)` placeholders for signatures/hashes pending the conformance-vector generator. Subsumes pre-existing gap C-8. | **Partially Resolved** (orderings + skeletons in place; signatures/hashes pending impl-side generator per the resolution convention in test-vectors.md). |
| **C-8** | **Pre-existing gap**: original 008 commerce payloads (`serviceRequest`, `serviceResponse`, `connectionSetup`, `escrowCreated`, `invoice`, `invoiceAck`) were defined in 008 FR-P06–P12 but never propagated into 006. Surfaced 2026-05-08 when the new lifecycle payloads (FR-P36/P37/P38) made the absence visible. | Subsumed by A-7. | **Subsumed** (no separate resolution; closure tracked under A-7). |

### Partially Resolved Items

**X-4 — Registration Bridge**: FR-X04 defines the conceptual mapping (which account fields → which registry inputs), but the actual registry function signatures cannot be specified until spec 003 is promoted to implementation-ready (blocked by C-4). The bridge definition provides enough information for implementors to understand the data flow; the concrete interface awaits spec 003's rewrite.

---

## Summary

| Category | Total Items | Resolved | Partially Resolved | Deferred | Subsumed |
|----------|:-----------:|:--------:|:------------------:|:--------:|:--------:|
| A (Algorithm) | 7 | 7 | 0 | 0 | 0 |
| B (Wire Format) | 5 | 5 | 0 | 0 | 0 |
| C (Missing) | 8 | 5 | 0 | 2 | 1 |
| D (Go-isms) | 6 | 6 | 0 | 0 | 0 |
| X (Cross-Spec) | 6 | 5 | 1 | 0 | 0 |
| A (2026-05-08) | 1 | 0 | 1 | 0 | 0 |
| **Total** | **33** | **28** | **2** | **2** | **1** |

**Resolution rate**: 28/33 = 84.8% fully resolved, 30/33 = 90.9% resolved or partially resolved (pending impl-generated signatures for A-7).

**Remaining deferred items** (C-1, C-4) require spec-level rewrites of 001 and 003 respectively, which are beyond the scope of this protocol determinism overlay.

**Pending impl resolution**: A-7 will move from "Partially Resolved" to "Resolved" once the conformance-vector generator produces the actual signatures/hashes for Chain 5 and Chain 6 placeholders, per the resolution convention documented in `test-vectors.md`.
