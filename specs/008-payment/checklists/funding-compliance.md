# Funding Compliance Requirements Quality Checklist: Payment

**Purpose**: Validate the quality, completeness, and clarity of the revised Section E (Settlement Preconditions and Funding Compliance) and its cross-references throughout spec 008, following the 2026-03-24 architectural revision that decoupled service-level balance rules from protocol-level settlement mechanics.
**Created**: 2026-03-24
**Feature**: [specs/008-payment/spec.md](../spec.md)
**Focus**: Settlement-vs-service layering clarity, pluggable funding policy hooks, normative wording consistency, cross-section coherence

## Settlement vs Service-Level Distinction

- [ ] CHK001 — Is the boundary between settlement preconditions and service-level funding compliance explicitly defined with clear criteria for each category? [Clarity, Spec §FR-P24]
- [ ] CHK002 — Are settlement preconditions (FR-P25) stated as MUST-level invariants enforceable by all bindings, independent of any agreement terms? [Consistency, Spec §FR-P25]
- [ ] CHK003 — Is FR-P25(a) (`requestRelease` amount vs available) consistent with FR-P20's description of `requestRelease`, which does not currently state a failure condition for insufficient balance? [Consistency, Spec §FR-P20 vs §FR-P25]
- [ ] CHK004 — Is FR-P25(b) (`claimRefund` before timeout) clearly marked as a restatement of FR-P22 rather than a new requirement, to avoid ambiguity about which is normative? [Clarity, Spec §FR-P25(b) vs §FR-P22]
- [ ] CHK005 — Does FR-P26 clearly state that service-level funding rules are NOT defined by this spec without leaving implementers uncertain about what they ARE responsible for? [Clarity, Spec §FR-P26]
- [ ] CHK006 — Is the delegation mechanism for service-level funding rules sufficiently structured — i.e., does the spec identify WHERE (termsRef, negotiation parameters) these rules come from? [Completeness, Spec §FR-P26]

## Non-Compliance Semantics

- [ ] CHK007 — Are the four implications of funding non-compliance (observable, evidence for dispute, grounds for refusal/continuation, not automatic failure) mutually consistent and non-overlapping? [Consistency, Spec §FR-P27(a–d)]
- [ ] CHK008 — Is the relationship between FR-P27(d) ("no automatic protocol failure") and FR-P14 state transitions clear — specifically, can an implementer determine exactly which transitions require explicit party actions vs which are system-driven? [Clarity, Spec §FR-P27(d) cross-ref §FR-P14]
- [ ] CHK009 — Does FR-P27(c) ("grounds for refusal or continuation") provide enough specificity for SDK implementers to know what actions/APIs to surface, or is it too open-ended? [Completeness, Spec §FR-P27(c)]
- [ ] CHK010 — Is the dispute evidence path (FR-P27(b)) traceable — i.e., does the spec define how a validator or arbiter would reconstruct the agreed funding requirements from observable artifacts (agreementHash → topic history → termsRef)? [Completeness, Spec §FR-P27(b)]

## FUNDED → ACTIVE Transition

- [ ] CHK011 — Does FR-P27a clearly specify that the FUNDED → ACTIVE transition is a seller decision without creating ambiguity about whether buyers have any role or visibility in this decision? [Clarity, Spec §FR-P27a]
- [ ] CHK012 — Is the FR-P14 transition description ("seller evaluates funding compliance per agreed terms and begins delivery") consistent with FR-P27a's more detailed statement? [Consistency, Spec §FR-P14 vs §FR-P27a]
- [ ] CHK013 — Does the spec define what observable event signals the FUNDED → ACTIVE transition — or is the transition purely internal to the seller's SDK with no protocol-level message? [Gap, Spec §FR-P14]

## termsRef and Funding Policy Hooks

- [ ] CHK014 — Does FR-P04's addendum ("terms document SHOULD include service-level funding requirements") create a normative expectation without defining what minimally constitutes "funding requirements"? [Clarity, Spec §FR-P04]
- [ ] CHK015 — Is the relationship between `termsRef` (FR-P04), `proposedAmount`/`proposedInterval` (FR-P07), and `agreementHash` (FR-P17) as the three sources of funding policy sufficiently documented for an implementer to understand which takes precedence? [Completeness, Gap]
- [ ] CHK016 — Does the spec clarify whether `termsRef` is the same URI in both the `neuron-commerce` service offering and the accepted `serviceResponse`, or can it change during negotiation? [Ambiguity, Spec §FR-P04 vs §FR-P07/P08]

## Cross-Section Coherence

- [ ] CHK017 — Is the User Story 1 Scenario 2 wording ("evaluate whether funding meets the requirements established in the agreed terms") consistent with Section E's normative language? [Consistency, Spec §US1.2 vs §FR-P27a]
- [ ] CHK018 — Does the edge case about "buyer deposits less than agreed" align with Section E — specifically, does it avoid re-introducing a universal threshold while still providing implementable guidance? [Consistency, Spec §Edge Cases vs §FR-P26]
- [ ] CHK019 — Is SC-P09's revised wording ("settlement preconditions are enforced at the escrow level") testable — i.e., can a conformance test suite verify this criterion? [Measurability, Spec §SC-P09]
- [ ] CHK020 — Does SC-P09 adequately replace the former zero-balance criterion, or is there a gap where no success criterion covers the service-level funding compliance concept? [Coverage, Spec §SC-P09]

## Normative Wording

- [ ] CHK021 — Do all settlement preconditions in FR-P25 use MUST consistently, and do all service-level delegations in FR-P26/P27 avoid MUST except for stating what this spec does NOT mandate? [Consistency, Spec §FR-P24–P27a]
- [ ] CHK022 — Is FR-P27a's use of MAY ("seller MAY query getBalance") intentional, and does it correctly signal that the query is optional rather than the evaluation itself? [Clarity, Spec §FR-P27a]
- [ ] CHK023 — Does the spec avoid using "the SDK" or "the implementation" as subjects in MUST-level requirements within Section E, keeping language implementation-neutral? [Consistency, Spec §FR-P24–P27a]

## Error Handling Alignment

- [ ] CHK024 — Is the `InsufficientBalance` error kind in FR-P32 still correctly mapped now that it represents a settlement precondition (requestRelease amount > available) rather than the former service-level balance check? [Consistency, Spec §FR-P32 vs §FR-P25]
- [ ] CHK025 — Does the error taxonomy cover the case where a seller refuses FUNDED → ACTIVE due to service-level funding non-compliance, or is this a gap? [Coverage, Spec §FR-P32, Gap]

## Validator Observability

- [ ] CHK026 — Is the funding compliance observation note (after VR-PAY-06) consistent with VR-PAY-01 through VR-PAY-06's pass/fail structure, or does its MAY-level framing create an inconsistency in the validator checklist? [Consistency, Spec §VR-PAY Funding Note]
- [ ] CHK027 — Does the observation note provide sufficient guidance for a validator to reconstruct funding compliance criteria from the `agreementHash` → topic history → `termsRef` chain? [Completeness, Spec §VR-PAY Funding Note]

## Out-of-Scope Discipline

- [ ] CHK028 — Does the new "Funding compliance formulas" out-of-scope entry align with FR-P26's delegation language without contradicting the structured hooks that ARE provided (getBalance, termsRef, negotiation parameters)? [Consistency, Spec §Out of Scope vs §FR-P26]
- [ ] CHK029 — Is the boundary between "funding compliance formulas" (out of scope) and "service-level funding requirements in termsRef" (in scope via FR-P04 addendum) clear enough to avoid confusion? [Clarity, Spec §Out of Scope vs §FR-P04]
- [ ] CHK030 — Does the existing "SLA content and format" out-of-scope entry remain consistent with the revised Section E, or does the termsRef addendum in FR-P04 partially contradict it by recommending specific content categories? [Consistency, Spec §Out of Scope vs §FR-P04]

## Constitution Alignment

- [ ] CHK031 — Does the revised Section E comply with Principle VII (Strict Spec Compliance) — specifically, are MUST/SHOULD distinctions used correctly for the two rule categories? [Constitution VII, Spec §FR-P24–P27a]
- [ ] CHK032 — Does the revised Section E comply with Principle XI (Verifiable Execution) — specifically, can all settlement preconditions be verified by a third party using the VR-PAY rules? [Constitution XI, Spec §FR-P25 vs §VR-PAY]
- [ ] CHK033 — Does the delegation of service-level rules to `termsRef` comply with Principle IV (Semantic Types) — are the referenced types (Balance, termsRef URI, negotiation parameters) semantic rather than primitive? [Constitution IV, Spec §FR-P26]

## Notes

- This checklist validates the 2026-03-24 revision that replaced hardcoded balance thresholds (old FR-P24–P27) with the settlement-vs-service-level distinction (new FR-P24–P27a).
- Items marked [Gap] identify requirements that may be intentionally omitted but should be explicitly acknowledged.
- Cross-reference with `architecture-quality.md` CHK015 (now stale — needs update in follow-up).
