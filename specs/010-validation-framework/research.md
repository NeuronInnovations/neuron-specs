# Research: Validation Framework

**Phase**: 0 — Outline & Research
**Date**: 2026-03-31

## R1: Evidence Envelope Serialization — Canonical JSON Compatibility

**Decision**: Evidence envelopes use the same canonical JSON rules as all other Neuron payloads (006 FR-W01–W10). No additional serialization rules needed.

**Rationale**: The existing wire format rules handle all field types present in the evidence envelope: strings (type, version, specRef, verdict, evidenceURI), UnsignedInt256 (agentIds — serialized as decimal strings per FR-W02), HexBytes (evidenceHash — serialized as `0x`-prefixed lowercase hex per FR-W06), UnixTimestamp (observationWindow fields — unsigned integers per FR-W02). The field ordering rule (FR-V03) follows the same pattern as HeartbeatPayload (005 FR-H04) and negotiation payloads (008 FR-P12).

**Alternatives considered**: Custom binary encoding (rejected — breaks interoperability with existing TopicMessage infrastructure). Protobuf (rejected — existing protocol is JSON-first; adding a second serialization format increases complexity).

## R2: Validation Registry Code Extension — INCONCLUSIVE

**Decision**: Add response code `3 = INCONCLUSIVE` to the Validation Registry (007 FR-C-29). Extend `getSummary()` to return `inconclusiveCount`.

**Rationale**: 007 FR-C-29 already says "Implementations MAY define additional codes above 2 for extended status in future versions." Code 3 is the natural next value. The `getSummary()` return type changes from `(count, passCount, failCount)` to `(count, passCount, failCount, inconclusiveCount)` — additive, not breaking.

**Alternatives considered**: Using code `2` for both fail and inconclusive (rejected — conflates fundamentally different outcomes). Using a separate off-chain verdict field without on-chain representation (rejected — loses discoverability via `getSummary()`).

## R3: Cross-Registry Check Mechanism

**Decision**: The Validation Registry checks whether caller/validator has an active Identity Registry registration. It does NOT check for the `neuron-validator` service type (which is in the opaque agentURI string).

**Rationale**: The Identity Registry's `lookup(address)` returns `(tokenId, agentURI)`. Checking registration existence is a single contract call. Parsing the agentURI JSON to find a `neuron-validator` service would require on-chain JSON parsing — prohibitively expensive and fragile. Service type validation is an SDK-level concern: consumers verify the `neuron-validator` service when reading the validator's agentURI off-chain.

**Alternatives considered**: On-chain service type registry (rejected — adds contract complexity, requires maintaining a second mapping). Separate validator registry contract (rejected — fragments identity model; validators ARE agents, not a separate entity class).

## R4: compositeRefs Hash Type

**Decision**: `compositeRefs` values are `keccak256(canonicalJSON(envelope))` of individual evidence envelopes — the same computation used for `responseHash` in on-chain anchoring.

**Rationale**: Using envelope payload hashes (rather than `evidenceHash` values from within the envelopes) lets consumers verify composite references by hashing envelopes found on the validator's stdOut topic, without fetching external content at `evidenceURI`. This makes composite verification self-contained within the topic data.

**Alternatives considered**: Using `evidenceHash` values (rejected — requires fetching off-chain documents to verify linkage, defeating the purpose of topic-based observability).

## R5: On-Chain Anchoring Model

**Decision**: On-chain anchoring via `validationResponse()` remains request-initiated (requires prior `validationRequest()` per 007 FR-C-27/C-28). The validator's stdOut topic is the primary autonomous evidence channel.

**Rationale**: Preserves agent consent over who produces on-chain records about them. The stdOut channel already provides consensus-timestamped, append-only, publicly observable evidence. On-chain anchoring adds `getSummary()` discoverability but is not required for evidence validity. No 007 contract change needed for the autonomous use case.

**Alternatives considered**: Unsolicited on-chain verdicts (rejected — removes agent consent gate, enables griefing where any registered agent floods an agent's validation record). Dual model with opt-in flag (rejected — adds contract complexity for V1; can be revisited post-V1).

## R6: Package Placement

**Decision**: Go SDK implementation in `internal/validation/` as a new package.

**Rationale**: Although 010 is cross-cutting like 006, the Go SDK benefits from a dedicated package for validation utility types. The package provides: EvidenceEnvelope type (construction, serialization, hash computation), OracleVerdict type (verdict constants, code mapping), ValidatorService type (agentURI service parsing), and CompositeValidation utilities (compositeRefs computation). It does NOT implement validator logic.

**Alternatives considered**: Distributing types across existing packages (rejected — EvidenceEnvelope doesn't naturally belong in keylib, topic, or registry). No Go package (rejected — 006 has no dedicated Go package because it defines wire format RULES; 010 defines concrete TYPES that need constructors and validation).
