# Feature Specification: Validation Framework (Evidence-Based Validation)

**Feature Branch**: `010-validation-framework`
**Created**: 2026-03-31
**Status**: Draft

## Related Specs

- **[002 Key Library](../002-key-library/spec.md)** — NeuronPrivateKey / NeuronPublicKey; signatures for evidence envelope signing (R||S||V format per 002 FR-014)
- **[003 Peer Registry (EIP-8004)](../003-peer-registry/spec.md)** — agentURI with service types; this spec requires a `neuron-validator` service type for validator registration
- **[004 Topic System](../004-topic-system/spec.md)** — TopicMessage envelope; evidence envelopes are TopicMessage payloads published to the validator's stdOut
- **[005 Health](../005-health/spec.md)** — HeartbeatPayload, liveness model; the Zero-to-Heartbeat scenario is the reference cross-spec validation example
- **[006 Protocol Determinism](../006-protocol-determinism/spec.md)** — Wire format rules (canonical JSON field ordering, numeric encoding, binary encoding); evidence envelope serialization MUST follow 006 rules
- **[007 Identity Registry Smart Contract](../007-identity-contract/spec.md)** — Identity, Reputation, and Validation Registries; this spec extends the Validation Registry with the INCONCLUSIVE response code and requires validators to be registered agents
- **[008 Payment](../008-payment/spec.md)** — Commerce protocol; defines the advisory-to-enforcement path where validators serve as escrow arbiters (deferred to post-V1)
- **External: EIP-8004** — Agent registry standard; validator registration follows the same EIP-8004 pattern as all agents
- **External: EIP-3008** (informative) — Informative reference for the thumbs-up/thumbs-down oracle model

## Purpose

This spec is a **cross-cutting framework** (like 006-Protocol-Determinism) that defines how third-party validators independently assess whether deployed agents comply with protocol specifications. It does not implement a single SDK module — it defines protocol-wide validation concerns that all other specs reference.

It provides:

1. **Validator agent model** — How validators register as agents with a `neuron-validator` service type, making them discoverable, ratable, and validatable through the same registries as any other agent
2. **Evidence envelope** — A standardized TopicMessage payload type for publishing validation verdicts with linked evidence, following 006 wire format rules
3. **Oracle verdict schema** — Three-outcome model (compliant / non-compliant / inconclusive) mapped to the Validation Registry response codes
4. **Evidence lifecycle** — How evidence flows from observation through publication to on-chain anchoring
5. **Cross-spec validation scenarios** — Composite validation flows spanning multiple specs (reference: Zero-to-Heartbeat)

After this spec is complete, any developer can build a validator agent that registers on the Neuron network, observes other agents' publicly observable behaviour, publishes defensible evidence, and anchors verdicts on-chain — without access to any agent's internals and without disclosing their own validation methods.

## Out of Scope

- **Validator methods**: How validators gather evidence, what infrastructure they use, their proprietary techniques — these are validator-sovereign and explicitly NOT specified (Constitution XI: Evidence Over Disclosure)
- **Per-spec Evidence & Validation sections**: Each spec defines its own observable signals, evidence rules, and non-observable areas. This spec defines the framework those sections follow, not the per-spec content.
- **Commerce enforcement**: Validator-as-arbiter in escrow transactions, arbiterFee in EscrowAdapter, dispute resolution flows (deferred to 008 amendment, post-V1)
- **Infrastructure validator compensation**: ValidationRewardAdapter interface and concrete bindings, tokenomics, protocol-level fees (deferred pending economic design)
- **Meta-validation guidance**: Detailed framework for how validators validate other validators (deferred; recursive trust via existing Reputation + Validation Registries is sufficient for V1)
- **Full retroactive Evidence & Validation sections**: Updating all specs 001–009 to the new section format (sequential follow-up work, not part of this spec)

---

## Clarifications

- Q: Is this spec an implementation module or a cross-cutting framework? → A: Cross-cutting framework, like 006-Protocol-Determinism. It defines protocol-wide concerns referenced by all other specs. It does not produce a dedicated SDK package for agent functionality, though SDK implementations MAY provide validation utility types (evidence envelope builders, verdict publishers).
- Q: Do validators need their own account type? → A: No. Validators are standard Child agents (001) with a `neuron-validator` service type in their agentURI (003). No new account type is required.
- Q: Does the evidence envelope replace the Validation Registry? → A: No. The evidence envelope is the off-chain evidence document published to the validator's stdOut topic. The Validation Registry `validationResponse()` is the on-chain verdict anchor. The `responseHash` in the Validation Registry links to the evidence envelope. Two-layer chain: on-chain verdict → off-chain envelope → off-chain evidence details.
- Q: Are existing VR-_ identifiers in specs 008/009 affected? → A: No. Existing VR-PAY-_ and VR-DEL-\* remain valid. Under the new framework they are classified as evidence rules (suggested interpretations, not mandated procedures).
- Q: What is the relationship between this spec and Constitution Principle XI? → A: Principle XI (v1.5.0) defines the constitutional requirements. This spec provides the concrete protocol artifacts (evidence envelope format, verdict schema, validator service type) that make Principle XI implementable.

### Session 2026-03-31

- Q: What hash type do `compositeRefs` values represent — hashes of individual envelopes' off-chain documents (`evidenceHash`) or hashes of the individual envelope payloads (`keccak256(canonicalJSON(envelope))`)? → A: Hashes of the individual envelope payloads' canonical JSON (same computation as `responseHash`). This lets consumers verify composite references by hashing envelopes found on the validator's stdOut, without fetching external content.
- Q: Should on-chain verdict anchoring remain request-initiated (requiring prior `validationRequest()` per 007 FR-C-27/C-28), or should validators be able to submit unsolicited on-chain verdicts? → A: On-chain anchoring stays request-initiated. The validator's stdOut topic is the primary autonomous evidence channel — validators publish freely without any on-chain prerequisite. On-chain anchoring via `validationResponse()` requires a prior `validationRequest()` by the agent owner, preserving agent consent over who produces on-chain records about them. No 007 contract amendment is needed for unsolicited verdicts.

---

## User Scenarios & Testing

### User Story 1 — Publish a Validation Verdict (Priority: P1)

A validator agent observes another agent's publicly available behaviour (e.g., heartbeat messages on stdOut) and needs to publish a structured verdict with linked evidence. The validator builds an evidence envelope, publishes it as a TopicMessage on their own stdOut, and optionally anchors the verdict on-chain via the Validation Registry.

**Why this priority**: The evidence envelope is the core artifact of the entire framework. Without a standardized way to publish verdicts, nothing else works.

**Independent Test**: A validator agent constructs an evidence envelope with known field values, serializes it to canonical JSON per 006 rules, publishes it as a TopicMessage payload on stdOut, and the resulting message is retrievable with all fields intact and signature valid.

**Acceptance Scenarios**:

1. **Given** a registered validator agent with a `neuron-validator` service and a stdOut topic, **When** the validator builds an evidence envelope with `type: "validationEvidence"`, `version: "1.0.0"`, `verdict: "compliant"`, a valid `subjectAgentId`, `specRef`, `evidenceHash`, and `evidenceURI`, **Then** the envelope serializes to canonical JSON with fields in the order defined by FR-V03, passes schema validation, and the TopicMessage is publishable to stdOut.

2. **Given** a published evidence envelope on the validator's stdOut AND a prior `validationRequest()` by the subject agent's owner, **When** the validator calls `validationResponse()` on the Validation Registry with the keccak256 hash of the evidence envelope as `responseHash` and response code `1` (PASS), **Then** the on-chain record links to the off-chain envelope and `getSummary()` reflects the new verdict.

3. **Given** a validator attempting to build an evidence envelope with `verdict: "unknown"`, **When** schema validation runs, **Then** it rejects with an error indicating the verdict is not one of the three recognized values.

4. **Given** two validators independently evaluating the same agent for the same spec, **When** both produce evidence envelopes with different verdicts (one "compliant", one "inconclusive"), **Then** both envelopes are valid — the framework permits divergent conclusions from different validators.

---

### User Story 2 — Register as a Validator Agent (Priority: P1)

A developer wants to deploy a validator that is discoverable by other agents on the Neuron network. The developer registers a standard Child agent with a `neuron-validator` service type in its agentURI, declaring which domains the validator covers and how it delivers verdicts.

**Why this priority**: Co-equal with US1 — validators must be registrable agents before they can publish verdicts. This is the identity foundation.

**Independent Test**: A Child agent registers with a `neuron-validator` service in its agentURI. Another agent discovers the validator via Identity Registry lookup and reads the validator's domain declarations.

**Acceptance Scenarios**:

1. **Given** a Child agent with a NeuronPrivateKey, **When** it registers with an agentURI containing a `neuron-validator` service with `domains: ["005-health"]` and `verdictDelivery: "topic"`, **Then** the registration succeeds and the agentURI is retrievable via `lookup()`.

2. **Given** a registered validator agent, **When** another agent looks up the validator's agentURI and parses the `neuron-validator` service, **Then** the `domains`, `verdictDelivery`, and `version` fields are present and parseable.

3. **Given** a validator attempting to call `validationResponse()` on the Validation Registry, **When** the validator is NOT registered in the Identity Registry, **Then** the call MUST fail (cross-registry check).

4. **Given** a registered validator, **When** another agent calls `giveFeedback()` on the Reputation Registry for the validator's agentId, **Then** the feedback is recorded — validators are ratable like any other agent.

---

### User Story 3 — Compose a Cross-Spec Validation Scenario (Priority: P2)

A validator needs to perform an end-to-end check spanning multiple specs — for example, verifying that a newly registered agent is alive by checking its registration (003/007), discovering its stdOut topic, and observing heartbeat compliance (005). This is the "Zero-to-Heartbeat" scenario.

**Why this priority**: Cross-spec scenarios are an explicit design requirement and demonstrate the composability of per-spec evidence. However, they depend on US1 (evidence envelope) and US2 (validator identity) being in place.

**Independent Test**: A validator executes the Zero-to-Heartbeat scenario against a test agent, produces evidence envelopes at each step, and publishes a composite verdict referencing the individual evidence.

**Acceptance Scenarios**:

1. **Given** a registered agent with a stdOut topic publishing heartbeats, **When** a validator executes the Zero-to-Heartbeat scenario, **Then** the validator can observe: (a) the agent's registration in the Identity Registry, (b) the agentURI's stdOut topic service, (c) heartbeat messages on stdOut, (d) deadline compliance based on `nextHeartbeatDeadline`.

2. **Given** a registered agent whose last heartbeat declared `nextHeartbeatDeadline: T`, **When** a validator observes no new heartbeat at consensus timestamp T + tolerance, **Then** the validator publishes an evidence envelope with `verdict: "non-compliant"`, `specRef: "005-health"`, and evidence describing the missed deadline.

3. **Given** a registered agent whose stdOut topic is unreachable (network partition), **When** a validator cannot subscribe to the topic, **Then** the validator publishes an evidence envelope with `verdict: "inconclusive"` — the absence of evidence is not evidence of absence.

4. **Given** a cross-spec scenario spanning two specs, **When** the validator publishes evidence for each spec-level check and a composite verdict, **Then** each individual evidence envelope is independently verifiable and the composite verdict references them via `evidenceHash` linkage.

---

### User Story 4 — Read and Interpret Validation Evidence (Priority: P2)

An agent, platform operator, or another validator needs to discover, read, and interpret validation evidence published by validators. This includes subscribing to a validator's stdOut, parsing evidence envelopes, verifying integrity via `evidenceHash`, and querying the Validation Registry for on-chain verdict summaries.

**Why this priority**: The evidence model only works if consumers can read and verify evidence. This completes the read side of the evidence lifecycle.

**Independent Test**: An agent subscribes to a known validator's stdOut topic, reads an evidence envelope, verifies the `evidenceHash` matches the content at `evidenceURI`, and confirms the on-chain verdict matches the envelope verdict.

**Acceptance Scenarios**:

1. **Given** a validator's stdOut topic containing evidence envelopes, **When** a consumer subscribes and reads a `validationEvidence` message, **Then** the consumer can parse all fields per the canonical JSON schema and verify the envelope signature against the validator's registered EVMAddress.

2. **Given** an evidence envelope with `evidenceHash` and `evidenceURI`, **When** a consumer fetches the content at `evidenceURI` and computes `keccak256(content)`, **Then** the result MUST match `evidenceHash` — integrity is verifiable.

3. **Given** a validator's agentId, **When** a consumer calls `getSummary()` on the Validation Registry for that validator's subjects, **Then** the summary includes pass, fail, and inconclusive counts — reflecting the three-outcome model.

---

### Edge Cases

- What happens when a validator publishes an evidence envelope for an agent that is not registered? The envelope is valid (validators may observe agents before or after registration), but on-chain `validationRequest()` requires a valid agentId — the agent must be registered for on-chain anchoring.
- What happens when the `evidenceURI` becomes unavailable after publication? The evidence envelope on the validator's stdOut topic remains the authoritative record (consensus-timestamped, append-only). The `evidenceHash` provides integrity verification if the content is later recovered from any source.
- What happens when a validator's own registration is revoked after publishing evidence? Previously published evidence envelopes remain on the topic (append-only). Consumers SHOULD check validator registration status when evaluating evidence freshness and credibility.
- What happens when two validators produce contradictory verdicts for the same agent? Both are valid. The framework explicitly permits divergent conclusions — different validators may have different observation positions, methods, or timing. Consumers use the Reputation Registry to weight validator credibility.

---

## Requirements _(mandatory)_

### Functional Requirements

#### Evidence Envelope

- **FR-V01**: The evidence envelope MUST be a TopicMessage payload type with `type: "validationEvidence"` as the discriminator field.
- **FR-V02**: The evidence envelope MUST contain the following mandatory fields: `type` (always `"validationEvidence"`), `version` (semver string), `validatorAgentId` (UnsignedInt256 — the validator's on-chain agentId from the Identity Registry), `subjectAgentId` (UnsignedInt256 — the agent being validated), `specRef` (string — which spec or domain this evidence relates to, e.g. `"005-health"`), `verdict` (string — one of `"compliant"`, `"non-compliant"`, `"inconclusive"`), `evidenceHash` (HexBytes — keccak256 of the off-chain evidence document), `evidenceURI` (string — pointer to the full evidence, e.g. IPFS CID or HTTPS URL).
- **FR-V03**: Evidence envelope fields MUST be serialized in canonical JSON in the following order: `type` → `version` → `validatorAgentId` → `subjectAgentId` → `specRef` → `verdict` → `evidenceHash` → `evidenceURI`. Optional fields, when present, follow after `evidenceURI` in alphabetical order. Serialization MUST follow 006 wire format rules (FR-W01 through FR-W10).
- **FR-V04**: The `verdict` field MUST accept exactly three values: `"compliant"`, `"non-compliant"`, `"inconclusive"`. Implementations MUST reject any other value.
- **FR-V05**: The `evidenceHash` field MUST contain the keccak256 hash (per 002 FR-017) of the off-chain evidence document referenced by `evidenceURI`. This enables integrity verification without fetching the full document from the validator.
- **FR-V06**: The evidence envelope MAY include optional fields: `observationWindow` (object with `end` and `start` as UnixTimestamp, alphabetical field order per 006 — the time range the observation covers), `compositeRefs` (array of HexBytes — each value is `keccak256(canonicalJSON(envelope))` of a related individual evidence envelope, the same computation used for `responseHash` in FR-V09; this lets consumers verify composite references by hashing envelopes found on the validator's stdOut without fetching external content).
- **FR-V07**: Evidence envelope version compatibility MUST follow the same rule as 005 FR-H28: envelopes with `version` major `1` (e.g., `"1.0.0"`, `"1.1.0"`) MUST be accepted; unknown fields in minor/patch versions MUST be ignored; envelopes with `version` major `2` or higher MUST be rejected until the receiving agent explicitly upgrades.

#### Oracle Verdict Schema

- **FR-V08**: The Validation Registry (007) MUST support three response codes: `1` (PASS — maps to envelope verdict `"compliant"`), `2` (FAIL — maps to `"non-compliant"`), `3` (INCONCLUSIVE — maps to `"inconclusive"`). Code `0` remains PENDING.
- **FR-V09**: When a validator calls `validationResponse()` (which requires a prior `validationRequest()` by the agent owner per 007 FR-C-27/C-28), the `responseHash` MUST be the keccak256 hash of the canonical JSON serialization of the evidence envelope published to the validator's stdOut. This creates a two-layer evidence chain: on-chain verdict → off-chain envelope → off-chain evidence details. On-chain anchoring is OPTIONAL — the evidence envelope on stdOut is the primary, autonomous evidence record.
- **FR-V10**: The `getSummary()` function on the Validation Registry MUST return counts for all three outcome types: `passCount`, `failCount`, and `inconclusiveCount`.

#### Validator Agent Model

- **FR-V11**: Validators MUST register as standard agents (Child accounts per 001) with an agentURI containing a `neuron-validator` service object.
- **FR-V12**: The `neuron-validator` service object MUST include: `type` (always `"neuron-validator"`), `name` (string — service name, e.g. `"validation"`), `version` (semver string). The service object MUST also include: `domains` (array of strings — spec references or domain tags the validator covers, e.g. `["005-health", "008-commerce"]`), `verdictDelivery` (string — how verdicts are published, value `"topic"` for stdOut publication).
- **FR-V13**: The Validation Registry `validationResponse()` MUST verify that the caller (`msg.sender`) is registered in the Identity Registry with an active registration. If the caller has no active registration, the call MUST revert.
- **FR-V14**: The Validation Registry `validationRequest()` MUST verify that the `validatorAddress` parameter corresponds to a registered agent in the Identity Registry. If the validator address has no active registration, the call MUST revert.
- **FR-V15**: Validators MUST have the same mandatory services as any registered agent: three `neuron-topic` services (stdIn, stdOut, stdErr) and one `neuron-p2p-exchange` service (per 003 FR-R02). The `neuron-validator` service is an additional service.

#### Evidence Lifecycle

- **FR-V16**: Evidence envelopes MUST be published as TopicMessage payloads on the validator's stdOut topic. The TopicMessage envelope provides: sender identity (EVMAddress), signature (R||S||V), consensus timestamp, and sequence number. Publication to stdOut is autonomous — it does not require any on-chain prerequisite or agent consent. This is the primary evidence channel.
- **FR-V17**: Evidence envelopes published to stdOut SHOULD use `WaitForConsensus` confirmation mode (004 FR-T23) to ensure the evidence is provably anchored with a consensus timestamp before the on-chain `validationResponse()` references it.
- **FR-V18**: The off-chain evidence document (referenced by `evidenceURI`) is opaque to the protocol. Its format is defined by the validator. The protocol enforces only that `keccak256(document)` equals `evidenceHash`.
- **FR-V19**: The evidence envelope MUST NOT contain any field that requires the validator to disclose their validation methodology, tooling, infrastructure, or network position. The envelope captures WHAT was observed and WHAT was concluded, not HOW.

#### Cross-Spec Validation Scenarios

- **FR-V20**: This spec defines the **Zero-to-Heartbeat** scenario as a reference composite validation spanning specs 002, 001, 003, 004, and 005. The scenario verifies: (a) the agent's registration in the Identity Registry (007 `lookup()`), (b) the agentURI contains valid stdOut topic service, (c) heartbeat messages appear on stdOut, (d) the `nextHeartbeatDeadline` is honoured.
- **FR-V21**: Composite validation scenarios MAY produce multiple evidence envelopes (one per spec-level check) linked via the `compositeRefs` field in the final composite envelope. Each individual envelope MUST be independently verifiable.
- **FR-V22**: The Zero-to-Heartbeat scenario observable signals are:
  - **Identity check**: `lookup(agentAddress)` returns a valid `(tokenId, agentURI)` on the Identity Registry
  - **Service check**: agentURI `services[]` contains a `neuron-topic` entry with `channel: "stdOut"`
  - **Heartbeat presence**: At least one TopicMessage with payload `type: "heartbeat"` on the stdOut topic
  - **Deadline compliance**: The most recent heartbeat's `nextHeartbeatDeadline` has not elapsed without a subsequent heartbeat appearing

#### General

- **FR-V23**: Evidence envelope field types MUST use the semantic types defined across specs 001–006: `UnsignedInt256` for agentIds (per 006 FR-W02), `HexBytes` for hashes (per 006 FR-W06), `UnixTimestamp` for time fields (per 005 FR-H02), semver strings for version fields.
- **FR-V24**: The `specRef` field MUST use the format `"NNN-short-name"` matching the spec directory name (e.g. `"005-health"`, `"008-payment"`) for standard spec references. Custom domain strings (e.g. `"aviation"`, `"adsb-timing"`) are permitted for application-specific validation domains not covered by a Neuron spec.

### Key Entities

- **EvidenceEnvelope**: A structured TopicMessage payload containing a validator's verdict about an agent's spec compliance, with linked evidence. Published to the validator's stdOut topic. Fields: type, version, validatorAgentId, subjectAgentId, specRef, verdict, evidenceHash, evidenceURI, optional observationWindow, optional compositeRefs.
- **ValidatorService**: A `neuron-validator` service object in a validator agent's agentURI. Declares the validator's domains, verdict delivery mechanism, and version. Follows the same service pattern as `neuron-topic`, `neuron-p2p-exchange`, and `neuron-commerce`.
- **OracleVerdict**: The three-outcome result of a validation: compliant (Validation Registry code 1), non-compliant (code 2), inconclusive (code 3). The verdict is produced off-chain in the evidence envelope and anchored on-chain via `validationResponse()`.
- **CompositeValidation**: A cross-spec validation scenario that produces multiple evidence envelopes linked via `compositeRefs`. Each `compositeRefs` entry is the keccak256 of an individual envelope's canonical JSON (same computation as `responseHash`). Each individual envelope is independently verifiable; the composite envelope aggregates them into an end-to-end verdict.

---

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-V01**: An evidence envelope with known field values serializes to canonical JSON that is byte-identical across Go and TypeScript implementations — verified by cross-implementation test vector.
- **SC-V02**: An evidence envelope round-trips through TopicMessage publication and retrieval with all fields intact and signature valid — verified by end-to-end publish/read test.
- **SC-V03**: The Validation Registry accepts response codes 1 (PASS), 2 (FAIL), and 3 (INCONCLUSIVE) and `getSummary()` returns accurate counts for all three — verified by contract test.
- **SC-V04**: A validator registered with a `neuron-validator` service is discoverable via Identity Registry `lookup()` and the service's `domains` and `verdictDelivery` fields are parseable — verified by registration round-trip test.
- **SC-V05**: The `validationResponse()` call reverts when the caller is not registered in the Identity Registry — verified by negative test.
- **SC-V06**: The Zero-to-Heartbeat scenario can be executed by a validator against a test agent, producing evidence envelopes at each step (identity check, service check, heartbeat presence, deadline compliance) — verified by integration test.
- **SC-V07**: A consumer can verify evidence integrity by fetching content at `evidenceURI` and confirming `keccak256(content) == evidenceHash` — verified by integrity verification test.
- **SC-V08**: Divergent verdicts from two different validators for the same agent are both accepted by the framework (both envelopes valid, both on-chain records created) — verified by multi-validator test.

---

## Evidence & Validation _(mandatory)_

### Verification Tier

**`topic-observable`** for evidence envelope publication. **`on-chain-only`** for verdict anchoring.

A third-party observer can verify the validation framework's own compliance by reading evidence envelopes from validators' stdOut topics (topic-observable) and querying the Validation Registry for on-chain verdict records (on-chain-only).

### Observable Signals

- **Evidence envelopes on validator stdOut**: Any observer can subscribe to a validator's stdOut topic and read `validationEvidence` payloads — including the verdict, subject agentId, specRef, and evidence hash
- **Validation Registry on-chain records**: `validationResponse()` transactions emit events; `getSummary()` returns aggregated pass/fail/inconclusive counts
- **Validator registration in Identity Registry**: `lookup(validatorAddress)` returns the agentURI with `neuron-validator` service, revealing the validator's declared domains
- **Reputation Registry feedback**: `giveFeedback()` transactions targeting validator agentIds record advisory trust signals

### Evidence Rules

- **VR-VF-01**: A validator's stdOut topic contains `validationEvidence` payloads with valid canonical JSON and a verifiable TopicMessage signature → suggests the validator is actively producing evidence (compliant with evidence publication requirements).
- **VR-VF-02**: The `responseHash` in a `validationResponse()` on-chain record matches the keccak256 of an evidence envelope on the validator's stdOut → suggests the on-chain/off-chain evidence chain is intact (compliant). Mismatch → suggests broken evidence chain (non-compliant).
- **VR-VF-03**: A `validationResponse()` caller has no active Identity Registry registration → the transaction reverts (non-compliant with FR-V13). Observable via failed transaction receipt.
- **VR-VF-04**: A validator's agentURI does not contain a `neuron-validator` service → suggests the agent is not a registered validator (non-compliant with FR-V11/V12). May be inconclusive if agentURI has not been updated since the validator role was added.

### Non-Observable Areas

- **Validator methodology**: How a validator gathers evidence, what tools they use, their network position, and their proprietary techniques are explicitly non-observable. This is by design (Constitution XI: Evidence Over Disclosure). Validators are evaluated by the quality of their evidence, not by their methods.
- **Off-chain evidence document content**: The protocol only enforces `keccak256(document) == evidenceHash`. The document's internal structure and format are validator-defined and opaque to the protocol.
- **Validator intent**: Whether a validator is acting in good faith cannot be directly observed. Credibility is inferred over time via the Reputation Registry and meta-validation (comparing a validator's verdicts against publicly observable state).

**Behavioral Inference Recipes**:

- If a validator's verdicts consistently contradict publicly observable state (e.g., declaring agents "compliant" when their heartbeats are observably missed), infer low credibility. The Reputation Registry provides the mechanism for recording this inference.

### Suggested Evidence Recipes

**Recipe: Validate the evidence chain integrity**

1. Discover the validator via Identity Registry `lookup(validatorAddress)` → agentURI
2. Extract the stdOut topic from the agentURI's `neuron-topic` services
3. Subscribe to the validator's stdOut topic
4. Read `validationEvidence` payloads; verify TopicMessage signature against validator's EVMAddress
5. For each evidence envelope: fetch content at `evidenceURI`, compute `keccak256(content)`, confirm it matches `evidenceHash`
6. Query the Validation Registry for the corresponding `validationResponse()` record; confirm `responseHash` matches `keccak256(canonicalJSON(envelope))`
7. If all hashes match → evidence chain is intact. If any mismatch → evidence chain is broken.
