# Research: Health (Onchain Liveness & Health Status)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `005-health` | **Date**: 2026-02-25 | **Phase**: 0

---

## R1: Cross-Spec Dependency — Spec 004 Layer B (FR-T22..T29)

### Context

Spec 005 normatively references 6 FRs that do not yet exist in Spec 004's spec.md:

| Referenced FR | Concept | Where Used in 005 |
|---------------|---------|-------------------|
| FR-T22 | `PublishResult` type (transactionRef, consensusTimestamp, confirmed) | Related specs, FR-H24 |
| FR-T23 | Confirmation modes (`FIRE_AND_FORGET`, `WAIT_FOR_CONSENSUS`) | US1 scenario 2, FR-H23 |
| FR-T24 | `MessageDelivery` type (message, consensusTimestamp, backendSequence) | FR-H16 |
| FR-T25 | Subscribe resumption via `fromSequence` | Edge cases (subscription gaps) |
| FR-T27 | `maxMessageSize()` per backend | Edge cases, FR-H29 |
| FR-T29 | Clock normalization (backend timestamp → consensus time) | FR-H16 |

These were proposed in `transport-gap-analysis.md` §5.1 as "MUST be added to Spec 004" but are pending.

### Decision

**Spec 004 must be updated with FR-T22..T29 before Spec 005 can be finalized.** This is a blocking cross-spec dependency.

### Rationale

1. Spec 005's liveness model fundamentally depends on consensus timestamps (FR-H16) — without FR-T24 and FR-T29 defining how adapters expose these timestamps, the observer validation algorithm (V-OBS-05, V-OBS-06) has no defined input source.
2. FR-H23 (FIRE_AND_FORGET recommendation) and FR-H24 (deadline scheduling) reference confirmation modes and PublishResult fields that have no formal definition.
3. FR-H29 (payload size budget) references `maxMessageSize()` which requires FR-T27.
4. The transport-gap-analysis.md provides complete FR text for all 8 additions. The content is finalized — only the formal inclusion in Spec 004's spec.md is missing.

### Alternatives Considered

| Alternative | Rejected Because |
|-------------|-----------------|
| Inline the FR-T22..T29 definitions in Spec 005 | Violates AP-1 (health defines payload, 004 defines transport). The layering rule: 005 defines the message that enters the topic system; 004 defines the topic system itself. |
| Remove references and use vague language ("the adapter returns a result") | Constitution IV requires high-level semantic types. "A result" is a primitive, not a type. |
| Treat as non-blocking and proceed | Constitution V requires traceability. Referencing non-existent FRs breaks the dependency chain. |

### Resolution Path

Run `/speckit.specify` on Spec 004 with the Layer B addendum content from `transport-gap-analysis.md` §5.1. This adds FR-T22..T29 to Spec 004's Functional Requirements section. No existing FRs change — this is purely additive.

### Status

**RESOLVED** — FR-T22..T29 added to Spec 004's spec.md as "Publish/Subscribe Execution Binding" subsection. PublishResult, MessageDelivery, and ConfirmationMode entities added to Key Entities. SC-T12..T15 added to Success Criteria. Forward-reference annotations (*) removed from Spec 005.

---

## R2: Spec 004 Current FR Coverage

### Context

Spec 004 currently defines FR-T01 through FR-T21 (21 FRs). These cover:
- Topic abstraction (TopicRef, TopicMessage, TopicAdapter)
- Signing and verification
- Channel roles (stdIn/stdOut/stdErr)
- EIP-8004 integration
- Error types
- Config schemas
- Transactional invariants
- Deterministic JSON serialization

### Decision

FR-T01..T21 are sufficient for Spec 005's payload schema and signing requirements. Only the "Layer B" execution binding (how publish/subscribe actually interact with backends) is missing.

### Rationale

Spec 005 exercises the full publish → observe lifecycle for the first time. Previous specs (003, 004) defined the abstractions but never specified what happens at the adapter-to-chain boundary. Spec 005's protocol-level requirements (consensus timestamp for deadline arithmetic, fire-and-forget semantics, message size constraints) are the forcing function.

---

## R3: LivenessState Count (Four vs Five)

### Context

Source docs (architecture.md §9 AD-05) say "Four-state liveness (ALIVE/SUSPECT/DEAD/OFFLINE)." Spec 005 (FR-H18) defines five states including UNKNOWN. The plan inherits this inconsistency.

### Decision

**Five states is correct.** UNKNOWN is a distinct initial state, not a variant of any other state. The source doc wording "four-state" reflects the four protocol-active states; UNKNOWN is the pre-protocol state.

### Rationale

Without UNKNOWN, an observer encountering a peer for the first time has no defined state. Defaulting to DEAD or ALIVE would be incorrect. UNKNOWN accurately represents "no heartbeat data available for this peer."

### Status

**RESOLVED** — spec is correct at five states.

---

## R4: `validator` Role Status

### Context

FR-H05 MUST-accepts `"validator"` as a role value. Clarifications (Session 2026-02-24) states the validator role is "NOT confirmed as a Spec 005 feature."

### Decision

This is an internal contradiction that requires resolution via `/speckit.clarify` or `/speckit.specify`. The role is included for forward compatibility but the Clarification creates ambiguity.

### Alternatives

| Option | Implication |
|--------|-------------|
| Keep `validator` in MUST set, remove Clarification caveat | Validator is a valid role; any node may declare it |
| Move `validator` to "reserved for future use" with a note | FR-H05 lists 3 MUST roles + notes validator as reserved |
| Remove `validator` entirely | Simplest; re-add when the role is defined |

### Status

**RESOLVED** — `validator` moved to "reserved for future use" in FR-H05. Implementations MUST accept it as a valid value but SHOULD NOT assign semantic behavior until a future spec defines the validator role.

---

## R5: `expectedBackendLatency` in FR-H24

### Context

FR-H24 says: "the publisher SHOULD estimate the publication time as `wallClockAtSubmit + expectedBackendLatency`." But `expectedBackendLatency` is undefined.

### Decision

Simplify to: "the publisher SHOULD use local wall clock at the time of submission as the estimated publication time." Remove the `expectedBackendLatency` term.

### Rationale

1. Backend latency varies by chain, network conditions, and time of day. No single value is correct.
2. The GRACE_PERIOD (30s) already absorbs the estimation error.
3. If consensus timestamp is needed precisely, the publisher can use WAIT_FOR_CONSENSUS mode.

### Status

**RESOLVED** — applied. FR-H24 in spec.md now reads: "the publisher SHOULD use the local wall clock at the time of submission as the publication reference time." The `expectedBackendLatency` term has been removed.
