<!--
  Sync Impact Report
  Version change: 1.6.0 → 1.7.0
  Bump rationale: MINOR — Adds Principle XII (Layered Architecture: SDK Core
    vs DApp). New principle; no existing principle modified; no in-flight
    plan becomes non-compliant. Codifies the SDK/DApp boundary established
    by architecture decision 2026-05-07: core SDK specs MUST NOT encode
    domain-specific topology, application admission policy, or app fan-out
    semantics; those belong in DApp specs. Resolves the recurring
    "is this core or DApp?" ambiguity that surfaced when Spec 014 (Fan-Out)
    and Spec 015 (Admission Policy) were proposed and then rejected as core.
    Establishes a normative test future spec authors can apply.
  Modified principles:
    - XII (NEW): Layered Architecture (SDK Core vs DApp) — codifies the
      separation between core SDK specs (001–013) and DApp specs (016+).
      Core SDK exposes primitives (libp2p pubsub, wildcard stream
      registration, AdmissionPolicy interface, multiaddr filtering);
      DApps compose them into domain-specific workflows (ADS-B, Remote ID,
      future sensor profiles). Frozen relay charter (011) is named as
      Core SDK and out-of-bounds for DApp redefinition.
  Unchanged principles: I–XI
  Renamed sections: None
  Updated sections:
    - Implementation Standards: new bullet "Layering check" enforces
      Principle XII at plan and tasks generation time.
    - Development Workflow: Constitution Check requirements list extended
      to include Principle XII.
  Removed sections: None
  Templates (⚠ partial — to be reconciled in follow-up):
    - .specify/templates/tasks-template.md — no change required; tasks
      already reference principles by number.
  Commands: No changes required. /speckit.plan regenerations will pick up
    Principle XII automatically once the plan template is updated.
  Backward compatibility:
    - All existing specs/*/plan.md Constitution Check rows continue to PASS
      (none of 001–013 violate Principle XII; the principle codifies what
      they already do).
    - Specs 014 and 015 are explicitly DEPRECATED as core-spec proposals.
      Fan-out semantics (originally proposed as core 014) and admission
      policy semantics (originally proposed as core 015) live in DApp
      specs per Principle XII (e.g., 016 ADS-B, 017 Remote ID). Future
      authors MUST NOT resurrect 014/015 as core specs without amending
      Principle XII first.
    - Spec 011 remains relay-only per its frozen charter; XII names this
      explicitly.
-->

# Neuron SDK Spec Constitution

## Core Principles

### I. Specification-First

Every feature MUST be defined by a feature specification under `specs/` before an
implementation plan or task list is produced. Specs MUST include the mandatory
sections: Purpose (or equivalent), User Scenarios & Testing, Requirements
(Functional Requirements, Key Entities), and Success Criteria. Out-of-scope
MUST be stated explicitly.

**Rationale**: Prevents implementation drift and ensures shared understanding
before design or code.

### II. Independently Testable User Stories

User stories MUST be prioritized (e.g. P1, P2, P3) and each story MUST be
independently testable. Delivering one story MUST yield a verifiable increment.
Acceptance scenarios MUST use Given/When/Then and be testable without
implementing later stories.

**Rationale**: Enables incremental delivery and clear acceptance.

### III. Clarification Before Plan

Underspecified areas that affect architecture, data model, or test design MUST
be resolved (e.g. via clarification workflow) before generating the
implementation plan. Specs MUST NOT retain unresolved [NEEDS CLARIFICATION]
markers or placeholder decisions that materially impact implementation.

**Rationale**: Reduces rework and misaligned acceptance tests.

### IV. High-Level Types and Terminology

Data models and diagrams (e.g. ER diagrams) MUST use high-level semantic types
(e.g. AccountType, PeerID, EVMAddress) rather than low-level primitives
(e.g. string, object) except where no richer type applies. Canonical terminology
MUST be used consistently; when introduced or renamed, it MUST be recorded in
the spec's Clarifications.

**Rationale**: Improves readability and prevents ambiguous or brittle contracts.

### V. Traceability and Governance

Functional requirements, success criteria, and key entities MUST be present and
aligned in the spec. Constitution amendments MUST be documented with a version
bump and Last Amended date. Reviews that produce or change specs or plans
SHOULD verify compliance with this constitution.

**Rationale**: Ensures accountability and a single source of truth.

### VI. Language-Neutral Protocol, Reference Implementations

Neuron is a language-agnostic protocol. The specifications under `specs/` —
together with Spec 006 (Protocol Determinism) and its golden test vectors
under `specs/006-protocol-determinism/contracts/` — are the sole source of
truth for protocol semantics: wire format, canonical serialization, signing,
message envelopes, and on-chain interactions. No implementation, in any
language, may redefine protocol behavior; behavior observed in any one SDK
is descriptive, never normative.

For practical reasons, this repository currently uses Go as the **primary
reference implementation path**: it is the first SDK to land each spec, the
first transport adapter to integrate against Hedera testnet, and the default
target for `/speckit.plan` when no other language is requested. This is a
repository strategy, not a protocol property. Other language SDKs
(TypeScript today; others later) are equal in standing once they pass the
Spec 006 conformance suite. Cross-language consistency is enforced by the
specs, the Spec 006 wire format and algorithm references, and the golden
test vectors — never by deferring to any one SDK. Repository-specific
implementation conventions (module layout, tooling, file naming) live in
`CLAUDE.md` and the per-language READMEs, not in this principle.

**Rationale**: Treating a single implementation as normative would let
language-specific quirks silently rewrite the protocol. Anchoring authority
in the spec corpus and conformance vectors — while keeping a practical
reference implementation path that ships first — gives us unambiguous,
byte-level interoperability without granting any one language ecosystem
veto power over protocol design.

### VII. Strict Spec Compliance

SDK implementations MUST strictly comply with all MUST, MUST NOT, SHOULD,
and SHOULD NOT requirements in the feature specifications. Any deviation from
a spec requirement MUST be preceded by a spec amendment — implementations
MUST NOT silently diverge. When a spec uses SHOULD, the implementation MAY
omit the behavior but MUST document the omission and rationale. When a spec
uses MUST, the implementation has no discretion — the behavior is mandatory.
Compliance MUST be verifiable via the spec's success criteria (SC-*) and
functional requirements (FR-*).

**Rationale**: The specification-first model only works if implementations
faithfully execute the spec. Silent divergence creates cross-implementation
incompatibility and undermines the protocol's interoperability guarantees.

### VIII. Hedera Topic Transport Binding

Hedera Consensus Service (HCS) is the **primary transport binding** for the
Neuron topic system (Spec 004). The first TopicAdapter implementation MUST
target HCS. HCS-specific behaviors (consensus timestamps, topic IDs, message
ordering guarantees) MUST be documented as the reference binding. Other
transport bindings (EVM event logs, Kafka with HCS anchoring) MAY follow
but MUST NOT contradict the HCS reference. The health heartbeat system
(Spec 005) MUST be validated end-to-end on HCS before other backends.

**Rationale**: HCS provides native consensus timestamps and guaranteed
ordering — properties the protocol depends on for liveness deadlines and
message integrity. Validating on HCS first ensures the core protocol works
on its strongest backend before adapting to weaker-guarantee transports.

### IX. Test-First Development

Implementation MUST follow **test-first development** (TDD). For each
feature task:

1. Write tests that exercise the spec's acceptance scenarios — tests MUST
   fail before implementation (Red).
2. Implement the minimum code to make tests pass (Green).
3. Refactor while keeping tests green (Refactor).

Test files MUST be colocated with source or kept in a parallel test tree,
following the conventions of the implementation language. Integration tests
that exercise cross-spec contracts (e.g. 002 signing → 004 TopicMessage →
005 heartbeat) MUST exist before a feature is marked complete. Test coverage
for MUST-level requirements MUST be 100%.

**Rationale**: Test-first development catches spec misinterpretation early,
produces executable documentation of spec requirements, and prevents
regression during cross-spec integration.

### X. Deterministic Message Signing

All cryptographic signing operations MUST be **deterministic** — the same
private key and message MUST produce the same signature bytes on every
invocation. Specifically:

- Message hashing MUST use Keccak256 (per Spec 002 FR-017).
- ECDSA signing MUST use deterministic nonce generation per RFC 6979.
- Signature output MUST be in R||S||V format (65 bytes, per Spec 002 FR-014).
- TopicMessage signing (Spec 004 FR-T03) MUST produce identical signatures
  for identical `(timestamp, sequenceNumber, payload)` tuples when signed
  by the same key.
- Pre-image construction, canonical JSON, and algorithm choices MUST follow
  Spec 006 (Protocol Determinism) — specifically
  `specs/006-protocol-determinism/contracts/wire-format.md`,
  `algorithm-reference.md`, and the golden `test-vectors.md`. Conforming
  implementations in any language MUST reproduce those vectors byte-for-byte.

Implementations MUST NOT use random nonce generation for ECDSA. Test suites
MUST include determinism verification: sign the same message twice and assert
byte-equal signatures.

**Rationale**: Deterministic signing enables reproducible test vectors,
cross-implementation compatibility verification, and eliminates a class of
nonce-reuse vulnerabilities. The Neuron protocol's observer validation model
(Spec 005) depends on signature reproducibility for independent verification.

### XI. Verifiable Execution (Evidence-Based Validation)

Every spec MUST define the observable evidence by which a third-party validator —
with no access to the agent's internals — can independently assess whether a
deployed agent adheres to the spec at runtime.

#### Validator Autonomy

Validators are autonomous agents, not implementation followers. Specs define
observable signals and suggested evidence recipes, but validators choose their
own methods. A competent validator MAY use alternative methods as long as they
produce valid, defensible evidence. Specs MUST NOT prescribe mandatory
validation procedures — they define what CAN be observed, not what validators
MUST do.

#### Evidence Over Disclosure

Validators MUST provide evidence and a verdict. They are NOT required to reveal
tooling, infrastructure, network position, or proprietary techniques. The
protocol evaluates the quality of the evidence and the defensibility of the
judgment, not the method used to obtain it.

#### Three-Outcome Verdicts

Validator output MUST support three outcomes:

- **Compliant** (Validation Registry response code `1`): Evidence supports
  spec adherence.
- **Non-compliant** (Validation Registry response code `2`): Evidence
  indicates spec violation.
- **Inconclusive** (Validation Registry response code `3`): Insufficient
  evidence to determine compliance.

#### Evidenceability Principle

If behaviour cannot be evidenced, it cannot be enforced. Every MUST-level
requirement SHOULD have either observable evidence or a defined inference
method. Requirements with neither MUST be explicitly acknowledged in the
spec's Non-Observable Areas subsection.

#### Validators Are Agents

Validators MUST register as standard agents with a `neuron-validator` service
type in their agentURI. They are discoverable, ratable, and validatable
through the same Identity, Reputation, and Validation Registries as any other
agent.

#### Verification Tiers

Each spec MUST classify its **verification tier** based on what observable
signals exist:

- **`on-chain-only`**: Observable signals are limited to blockchain state
  (contract storage, events). Example: Identity Registry (007) — a validator
  reads `ownerOf()`, `lookup()`, and `agentURI()`.
- **`topic-observable`**: Observable signals include public topic channel
  messages. Example: Health (005) — a validator subscribes to an agent's
  stdOut topic and observes heartbeat messages.
- **`proof-required`**: Agent MUST publish cryptographic proofs beyond normal
  operational messages (e.g., Merkle inclusion proofs, commitment openings,
  signed execution traces). These proofs are intentional "information leaking"
  that makes black-box verification feasible.

When the verification tier is `topic-observable` or `proof-required`, the spec
MUST define the observation schema: which topics, which message types, which
fields are observable.

#### Mandatory Evidence & Validation Section

Each spec MUST include an **"Evidence & Validation"** section with four
mandatory subsections:

1. **Observable Signals**: What can be externally observed — blockchain
   transactions, topic messages, emitted events, public state transitions.
2. **Evidence Rules** (`VR-*`): How observable signals map to compliance
   verdicts. These are SUGGESTED interpretations, not mandated procedures.
   Each rule specifies an observable signal and the verdict it suggests.
3. **Non-Observable Areas**: What CANNOT be directly verified externally.
   Specs MUST explicitly acknowledge the limits of external verification.
   When non-observable areas exist, the section SHOULD include **Behavioral
   Inference Recipes**: practical methods for inferring non-observable
   behaviour from observable proxies.
4. **Suggested Evidence Recipes**: Informative step-by-step guidance for
   common validation scenarios. These are reference approaches for
   non-expert validators, not mandatory procedures.

**Rationale**: Spec 006 guarantees observable output determinism — conforming
implementations produce identical wire formats and signatures for identical
inputs. However, output determinism is a build-time property verified by test
vectors. Once deployed, an agent is a black box. Without runtime verification
rules, the spec-driven model degrades to "trust the deployer" — the very
problem that on-chain registries and public topic channels exist to solve.
Third-party validators need agents to intentionally "leak" verifiable state to
public channels so they can assess execution properties and settle disputes.
This principle ensures every spec defines what observable evidence exists and
how it can be interpreted, while respecting validator autonomy — validators
produce defensible judgments using methods of their choosing. The three-outcome
model acknowledges that some validations are inherently uncertain, and the
evidenceability principle forces specs to be honest about their verification
limits.

### XII. Layered Architecture (SDK Core vs DApp)

The Neuron specification corpus is organized into two strictly separated
layers. Specs MUST declare which layer they belong to, and MUST NOT cross the
boundary defined below.

#### Core SDK layer (specs 001–013)

The Core SDK layer covers identity, keys, registry, topics, health, protocol
determinism, identity contract, payment, P2P data delivery, validation
framework, relay, browser-client profile, and connectivity profiles. Core SDK
specs:

- MUST define **primitives, interfaces, and substrate semantics** that are
  domain-agnostic — usable by any application.
- MAY expose **pluggable interfaces** (e.g., `AdmissionPolicy`, `EscrowAdapter`,
  `TopicAdapter`, `DeliveryAdapter`) for application-specific behavior to be
  injected, without prescribing what that behavior is.
- MAY expose **substrate primitives** that DApps compose (e.g., libp2p
  pubsub/floodsub/gossipsub, wildcard stream protocol-ID registration,
  multiaddr advertisement filtering rules) without prescribing topology,
  copy semantics, mesh parameters, or application policy.
- MUST NOT encode **domain-specific topology** (e.g., ADS-B fan-out mesh
  shape, Remote ID overlay structure).
- MUST NOT encode **application admission policy** (e.g., coalition-partner
  whitelist, sensor-class priority ordering, per-tenant rate limits).
- MUST NOT encode **application fan-out semantics** (e.g., "ADS-B raw frames
  fan out via gossipsub mesh of 8 with no copy on lag").
- MUST NOT encode **sensor- or domain-specific frame formats, decoders, or
  schemas** (e.g., BEAST 0x1A framing internals beyond what is necessary as
  an opaque payload, ASTM F3411-22a Remote ID parsing).

The frozen scope of Spec 011 (Relay) is part of the Core SDK layer and
governs **single-hop relay-assisted connectivity only**. DApp specs MUST NOT
redefine or broaden this charter.

#### DApp layer (specs 016+)

The DApp layer covers application-specific compositions of Core SDK
primitives. DApp specs:

- MUST cite the Core SDK primitives they consume (e.g., "this DApp uses
  Spec 009 wildcard handler registration to expose `/adsb/filtered/*`").
- MAY define **fan-out topology** (gossipsub mesh parameters, copy/drop
  policy, distribution-relay roles) tailored to the DApp's traffic shape.
- MAY define **AdmissionPolicy implementations** (priority lists,
  whitelists, blacklists, per-tenant rules) tailored to the DApp's
  participants.
- MAY define **frame formats, decoders, and stream catalog paths** specific
  to the DApp's data type.
- MUST NOT modify Core SDK primitives, interfaces, or wire formats.
  Required Core SDK changes MUST be requested as Core SDK amendments;
  the DApp MUST NOT implement workarounds that diverge from Core SDK
  behavior.
- MUST include a **fused-buyer section** when relevant — describing how a
  downstream buyer can aggregate this DApp's streams with streams from
  other DApps. Fused-buyer behavior is not a separate DApp spec; it is a
  composable section of each producer DApp.

#### Boundary test

When deciding whether a normative requirement belongs in Core SDK or DApp:

- If the requirement names a specific application, sensor, frame format,
  partner organization, or business policy → DApp.
- If the requirement defines an interface, primitive, or substrate that
  multiple unrelated applications would consume identically → Core SDK.
- If unclear, the author MUST ask a CLARIFICATION question and record the
  decision in the spec's Clarifications, citing this principle.

#### Cross-references and additivity

DApp specs MAY reference Core SDK FR identifiers freely (e.g.,
"FR-D-wildcard-handler" from 009). Core SDK specs MUST NOT reference DApp
FR identifiers, FR groups, or DApp-specific terminology in normative
requirements. Adding a new DApp MUST NOT require Core SDK amendment.

**Rationale**: Without an explicit boundary, every cross-cutting feature
(fan-out, admission, sensor framing) creates a "is this core or DApp?"
debate that delays spec work and risks core-spec contamination with
domain-specific decisions. The architecture decision of 2026-05-07 explicitly
re-scoped fan-out and admission policy from proposed core specs (014, 015)
into the DApp layer, with the SDK exposing libp2p pubsub primitives and a
pluggable `AdmissionPolicy` interface. This principle codifies that
direction so the same re-scoping does not need to be re-litigated for
future domain features (e.g., radiation sensors, drone command/control,
maritime AIS).

**Companion guidance documents**: Two normative-informative documents
support DApp authors in applying Principle XII consistently:

- `docs/dapp-frame-format-precedent.md` — rules R-FF-01 / R-FF-02 / R-FF-03
  for choosing between opaque pass-through and normalized canonical-JSON
  frame formats. ADS-B (016) follows R-FF-01; Remote ID (017) follows
  R-FF-02. Future DApp authors MUST consult this document before designing
  a new data-plane payload format.
- `docs/dapp-admission-anchor-pattern.md` — anchors A1 (deployment-config),
  A2 (EIP-8004 service metadata), A3 (future on-chain admission registry)
  for the AdmissionPolicy backend. Core SDK (008 FR-P55–P57) provides only
  the helper interface; the data anchor is a DApp choice. June 2026 reference demo
  uses A1; production deployments are encouraged to migrate toward A2 or
  A3 as the deployment matures.

These companion documents are informative — they MAY be updated without
amending this principle — but their existence is normatively required
to keep the boundary test in this principle actionable for new DApp
authors.

## Specification Standards

- Specs live under `specs/<###-feature-name>/` with `spec.md` as the primary
  artifact. Checklists (e.g. requirements) MAY live under `checklists/`.
- Dependencies on other specs (e.g. Key Library) MUST be linked by path or
  reference. Out-of-scope MUST list registry/chain interaction, key storage,
  and transport where applicable.
- Entity-relationship or high-level diagrams SHOULD use semantic types and be
  kept in sync with Key Entities.

### Related specs section

Whenever a **new spec** is created, it MUST include a **section that lists all
relevant specs** for that document. This section MUST list:

- **Specs in this repo**: Any spec under `specs/` (in this repository) that this
  document depends on, extends, or references (e.g. Key Library, Peer Registry).
- **External standards**: Any EIP, HIP, or other external standard (by identifier
  and link) that the document normatively references (e.g. ERC-4337, EIP-8004,
  HIP-xxx).

The section MAY be titled "Related specs", "Referenced specs", or equivalent.
Implementations and tooling (e.g. speckit.specify) MUST ensure this section is
present when creating or materially updating a spec.

**Rationale**: Ensures every spec documents its dependency graph and external
standards in one place, improving discoverability and consistency across the
repo and with EIP/HIP ecosystems.

### Mermaid diagrams in specs

Whenever a spec is written or materially modified, it MUST include Mermaid
diagrams in an **Appendix** when applicable (e.g. entity relationships, flows,
sequences, state). When it is unclear which Mermaid diagram type is most
appropriate (e.g. `flowchart`, `sequenceDiagram`, `erDiagram`, `stateDiagram`),
the author MUST ask a CLARIFICATION question, choose the best fit, and record
the decision in the spec's Clarifications.

**Rationale**: Diagrams improve shared understanding and catch inconsistencies
early; clarifying diagram type avoids wrong or redundant visuals.

### Blockchain and ledger compatibility

Specs MUST remain **blockchain-agnostic** in wording and structure (no
chain-specific requirements unless the feature is explicitly chain-specific).
At the same time, specs MUST **aim for Ethereum compatibility**: where the
feature touches ledgers, tokens, or identity, design and requirements SHOULD
allow a straightforward Ethereum (or EVM) implementation without
contradiction. Specs MUST include **hints or a short section** on how **Hedera**
and Hedera-based implementations can adhere to the spec (e.g. Hedera Hashgraph
Consensus Service, HTS, HIPs, or SDK mappings). Where the author does not know
how Hedera can adhere, the spec MUST pose a CLARIFICATION question and resolve
it (e.g. in Clarifications or a dedicated "Hedera adherence" note) before the
implementation plan is generated.

**Rationale**: Ethereum compatibility maximizes reuse and tooling; Hedera
guidance supports a key target ledger without hard-coding it into core
principles.

### Implementation standards

Implementation plans and task lists MUST reflect the following constraints
derived from Principles VI–XII:

- **Reference implementation default**: For this repository, Go is the
  current primary reference implementation path (Principle VI). Plans
  SHOULD target Go unless they explicitly justify another language, and
  MUST NOT leave language choice as "NEEDS CLARIFICATION". Targeting any
  language is permitted; the protocol does not privilege one.
- **Spec traceability**: Every implementation task MUST reference the FR-*
  or SC-* requirement it satisfies (Principle VII).
- **Transport priority**: The first adapter integration test MUST use HCS
  (Principle VIII).
- **Test ordering**: Test tasks MUST appear before implementation tasks in
  each phase (Principle IX).
- **Signing tests**: Any task involving message signing MUST include a
  determinism test (Principle X).
- **Evidence & validation**: Every spec MUST include an Evidence & Validation
  section with observable signals, evidence rules, non-observable areas, and
  suggested evidence recipes (Principle XI). Test suites SHOULD include at
  least one evidence-grounded scenario: a test that asserts the agent produces
  the observable signals defined in the spec's Evidence Rules, verifiable from
  a third-party validator perspective using only publicly observable state.
- **Layering check**: Every plan MUST identify whether the spec belongs to
  the Core SDK layer (001–013) or the DApp layer (016+) and verify against
  Principle XII's boundary test. Plans for Core SDK specs MUST flag any
  normative requirement that names a specific application, sensor, partner
  organization, or business policy as a layering violation. Plans for DApp
  specs MUST cite which Core SDK primitives they consume.

**Rationale**: Codifies the implementation principles into actionable
constraints for plan and task generation.

### Evidence & Validation section

Every spec MUST include an **"Evidence & Validation"** section as a mandatory
section. This section MUST contain four subsections:

- **Verification Tier**: One of `on-chain-only`, `topic-observable`, or
  `proof-required` (Principle XI). Describes what observable signals exist for
  this spec's requirements, not what validators must check.
- **Observable Signals**: What can be externally observed without access to
  agent internals. Each signal MUST specify: what is observable (transaction,
  event, topic message, public state), and where (contract address, topic ID,
  event signature).
- **Evidence Rules** (`VR-*`): How observable signals map to compliance
  verdicts. Each rule specifies: an observable signal, the verdict it suggests
  (compliant, non-compliant, or inconclusive), and the reasoning. These are
  SUGGESTED interpretations — validators MAY reach the same conclusion through
  alternative methods. Evidence Rule identifiers (`VR-*`) are scoped per spec
  (e.g., `VR-I-01` for Identity, `VR-H-01` for Health). Existing `VR-*`
  identifiers in specs 008 and 009 remain valid under the new framework.
- **Non-Observable Areas**: What CANNOT be directly verified externally. Specs
  MUST explicitly state what falls outside the reach of external observation
  (e.g., private P2P streams, encrypted payloads, internal agent state). When
  non-observable areas exist, the section SHOULD include **Behavioral Inference
  Recipes**: practical methods for inferring non-observable behaviour from
  observable proxies (e.g., "if Agent B connects back to Agent A within timeout,
  infer receipt").
- **Suggested Evidence Recipes** (informative): Step-by-step guidance for
  common validation scenarios. These are reference approaches useful for
  non-expert validators. They are not mandatory procedures — a competent
  validator MAY use alternative methods as long as they produce valid,
  defensible evidence.

Implementations MUST publish the observable state referenced in the Evidence
Rules so that any third party can assess compliance. Validator autonomy is
protocol-guaranteed: the spec defines what CAN be observed, not what validators
MUST do. Validators produce defensible judgments using methods of their choosing.

When it is unclear which verification tier applies (e.g., a spec with both
on-chain and topic-based requirements), the author MUST ask a CLARIFICATION
question, choose the highest applicable tier, and document the decision.

**Rationale**: Formalizes the "information leaking" pattern already present
ad-hoc in specs 005 and 007 into a mandatory, auditable section. Ensures the
Validation Registry (007 US4) has concrete evidence rules to evaluate, not
just a mechanism for recording pass/fail. The four-subsection structure
enforces honest disclosure of verification limits (Non-Observable Areas) and
provides actionable guidance (Suggested Evidence Recipes) without constraining
validator methodology.

## Development Workflow

- Intended order: **Specify** (or load spec) → **Clarify** (resolve ambiguity) →
  **Plan** (implementation plan from spec) → **Tasks** (task list from plan).
- The implementation plan MUST include a **Constitution Check** section. Gates
  are derived from `.specify/memory/constitution.md` and MUST be re-checked
  after Phase 1 design. Gates MUST include checks for Principles VI–XII
  (language-neutral protocol, spec compliance, HCS binding, test-first,
  deterministic signing, verifiable execution, layered architecture).
- No implementation work SHOULD start until the plan's Constitution Check passes
  and foundational phase is defined.

## Governance

- This constitution supersedes ad-hoc practice for the Neuron SDK Spec
  repository. Conflicts between a spec/plan and a principle MUST be resolved by
  updating the spec or plan, or by amending the constitution (with version and
  date).
- **Amendments**: Update `.specify/memory/constitution.md`; increment version
  per semantic versioning (MAJOR: incompatible principle change; MINOR: new
  principle or section; PATCH: wording/clarification only); set Last Amended to
  the change date; prepend a Sync Impact Report (HTML comment) listing version
  change, modified principles, and template updates.
- **Compliance**: Analyze and plan commands MUST load the constitution and
  enforce principle checks. Violations MUST be flagged and resolved before
  proceeding.

**Version**: 1.7.0 | **Ratified**: 2026-01-23 | **Last Amended**: 2026-05-08
