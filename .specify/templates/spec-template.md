# Feature Specification: [FEATURE NAME]

**Feature Branch**: `[###-feature-name]`  
**Created**: [DATE]  
**Status**: Draft  
**Input**: User description: "$ARGUMENTS"

## Layer *(mandatory; Constitution Principle XII)*

<!--
  IMPORTANT (Constitution Principle XII — Layered Architecture): Every spec MUST
  declare whether it belongs to the Core SDK layer (001–013) or the DApp layer
  (016+). Core SDK specs MUST NOT encode domain-specific topology, application
  admission policy, sensor frame formats, or app fan-out semantics. DApp specs
  MAY define those, MUST cite the Core SDK primitives they consume by FR-*, and
  MUST NOT redefine the relay charter (011) or other Core SDK interfaces.

  If a draft requirement names a specific application, sensor, partner
  organization, or business policy → DApp. If it defines an interface or
  primitive consumed identically by multiple unrelated apps → Core SDK. When
  unclear, ask a CLARIFICATION question and record the decision.
-->

[**Core SDK** | **DApp**] — [one-sentence layer-test justification].

[For DApp specs, list the Core SDK primitives consumed, e.g.: "Consumes 008
FR-P06 commerce taxonomy, 008 FR-P33a streams[] catalog, 009 FR-D-wildcard-handler."]

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.
  
  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - [Brief Title] (Priority: P1)

[Describe this user journey in plain language]

**Why this priority**: [Explain the value and why it has this priority level]

**Independent Test**: [Describe how this can be tested independently - e.g., "Can be fully tested by [specific action] and delivers [specific value]"]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]
2. **Given** [initial state], **When** [action], **Then** [expected outcome]

---

### User Story 2 - [Brief Title] (Priority: P2)

[Describe this user journey in plain language]

**Why this priority**: [Explain the value and why it has this priority level]

**Independent Test**: [Describe how this can be tested independently]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]

---

### User Story 3 - [Brief Title] (Priority: P3)

[Describe this user journey in plain language]

**Why this priority**: [Explain the value and why it has this priority level]

**Independent Test**: [Describe how this can be tested independently]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]

---

[Add more user stories as needed, each with an assigned priority]

### Edge Cases

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right edge cases.
-->

- What happens when [boundary condition]?
- How does system handle [error scenario]?

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements

- **FR-001**: System MUST [specific capability, e.g., "allow users to create accounts"]
- **FR-002**: System MUST [specific capability, e.g., "validate email addresses"]  
- **FR-003**: Users MUST be able to [key interaction, e.g., "reset their password"]
- **FR-004**: System MUST [data requirement, e.g., "persist user preferences"]
- **FR-005**: System MUST [behavior, e.g., "log all security events"]

*Example of marking unclear requirements:*

- **FR-006**: System MUST authenticate users via [NEEDS CLARIFICATION: auth method not specified - email/password, SSO, OAuth?]
- **FR-007**: System MUST retain user data for [NEEDS CLARIFICATION: retention period not specified]

### Key Entities *(include if feature involves data)*

- **[Entity 1]**: [What it represents, key attributes without implementation]
- **[Entity 2]**: [What it represents, relationships to other entities]

## Success Criteria *(mandatory)*

<!--
  ACTION REQUIRED: Define measurable success criteria.
  These must be technology-agnostic and measurable.
-->

### Measurable Outcomes

- **SC-001**: [Measurable metric, e.g., "Users can complete account creation in under 2 minutes"]
- **SC-002**: [Measurable metric, e.g., "System handles 1000 concurrent users without degradation"]
- **SC-003**: [User satisfaction metric, e.g., "90% of users successfully complete primary task on first attempt"]
- **SC-004**: [Business metric, e.g., "Reduce support tickets related to [X] by 50%"]

## Evidence & Validation *(mandatory)*

<!--
  IMPORTANT (Constitution Principle XI — Evidence-Based Validation): Every spec
  MUST define the observable evidence by which a third-party validator — with no
  access to agent internals — can independently assess runtime compliance.
  Validators are autonomous: specs define what CAN be observed, not what
  validators MUST do. See .specify/memory/constitution.md Principle XI for full
  details.
-->

### Verification Tier

<!-- One of: `on-chain-only`, `topic-observable`, `proof-required` -->

[Chosen tier and justification]

### Observable Signals

<!--
  What can be externally observed without access to agent internals.
  Each signal: what is observable (transaction, event, topic message, public
  state), and where (contract address, topic ID, event signature).
-->

- [Signal 1]: [What can be observed] at [where]
- [Signal 2]: [What can be observed] at [where]

### Evidence Rules

<!--
  How observable signals map to compliance verdicts. These are SUGGESTED
  interpretations, not mandated procedures. Validators MAY use alternative
  methods as long as they produce valid, defensible evidence.
  VR-* identifiers are scoped per spec (e.g., VR-I-01 for Identity,
  VR-H-01 for Health).
-->

- **VR-[PREFIX]-01**: [Observable signal] suggests [compliant/non-compliant/inconclusive] because [reasoning].
- **VR-[PREFIX]-02**: [Observable signal] suggests [compliant/non-compliant/inconclusive] because [reasoning].

### Non-Observable Areas

<!--
  What CANNOT be directly verified externally. Specs MUST explicitly
  acknowledge the limits of external verification. When non-observable
  areas exist, include Behavioral Inference Recipes: practical methods
  for inferring non-observable behaviour from observable proxies.
-->

- [Non-observable behaviour 1]: [Why it cannot be directly verified]
- [Non-observable behaviour 2]: [Why it cannot be directly verified]

**Behavioral Inference Recipes** *(include if non-observable areas exist)*:

- If [observable proxy], then infer [non-observable behaviour]

### Suggested Evidence Recipes

<!--
  Informative step-by-step guidance for common validation scenarios.
  These are reference approaches for non-expert validators, not mandatory
  procedures. A competent validator MAY use alternative methods.
-->

1. [Step 1: e.g., Discover agent via Identity Registry lookup]
2. [Step 2: e.g., Subscribe to stdOut topic]
3. [Step 3: e.g., Observe signal and evaluate]
4. [Step 4: e.g., Publish evidence envelope with verdict]
