# Tasks: Validation Framework

**Input**: Design documents from `specs/010-validation-framework/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US4)

## Path Conventions

- **Go SDK (Constitution VI)**: `internal/validation/` at module root (`impl/golang/`); tests colocated as `*_test.go`

## Constitution Compliance Notes

- **Principle VII**: Every task description references the FR-* or SC-* it satisfies
- **Principle IX**: Test tasks appear before implementation tasks within each phase (Red-Green-Refactor)
- **Principle X**: Evidence envelopes are signed TopicMessages — determinism inherited from 004/006; SC-V01 requires byte-identical serialization across implementations
- **Principle XI**: Test tasks include evidence-grounded scenarios verifying observable signals from a validator perspective

---

## Phase 1: Setup

**Purpose**: Package initialization and shared infrastructure

- [ ] T001 Create `impl/golang/internal/validation/` directory and `doc.go` with package-level godoc describing the validation framework package (cross-cutting types for evidence-based validation per spec 010)
- [ ] T002 [P] Create error types in `impl/golang/internal/validation/errors.go` — `ValidationError` with `ErrorKind` enum: `InvalidVerdict`, `InvalidEnvelopeField`, `HashMismatch`, `MissingRequiredField`, `IncompatibleVersion` (follows keylib/account error pattern)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types that ALL user stories depend on

### Tests (Red — must fail before implementation)

- [ ] T003 [P] Write tests for OracleVerdict type in `impl/golang/internal/validation/verdict_test.go` — test verdict string validation (FR-V04: accept "compliant"/"non-compliant"/"inconclusive", reject others), verdict-to-registry-code mapping (FR-V08: compliant→1, non-compliant→2, inconclusive→3), code-to-verdict reverse mapping, PENDING code 0 handling
- [ ] T004 [P] Write tests for EvidenceEnvelope construction in `impl/golang/internal/validation/evidence_envelope_test.go` — test NewEvidenceEnvelope() with valid fields (FR-V02), rejection of invalid verdict (FR-V04), rejection of zero agentIds, rejection of empty specRef/evidenceHash/evidenceURI, version compatibility (FR-V07: accept 1.x.y, reject 2.x.y)

### Implementation (Green)

- [ ] T005 [P] Implement OracleVerdict type in `impl/golang/internal/validation/verdict.go` — Verdict type (string enum), VerdictToCode()/CodeToVerdict() mapping functions, IsValidVerdict() validation (FR-V04, FR-V08)
- [ ] T006 Implement EvidenceEnvelope type in `impl/golang/internal/validation/evidence_envelope.go` — immutable struct with mandatory fields (FR-V02), NewEvidenceEnvelope() factory with validation, optional fields (ObservationWindow, CompositeRefs per FR-V06), field accessors, version compatibility check (FR-V07)

**Checkpoint**: Verdict and EvidenceEnvelope types exist and pass construction/validation tests

---

## Phase 3: User Story 1 — Publish a Validation Verdict (Priority: P1)

**Goal**: A validator can construct, serialize, and publish an evidence envelope as a TopicMessage on stdOut

**Independent Test**: Build an envelope with known fields, serialize to canonical JSON, verify byte-identical output and field ordering

### Tests (Red)

- [ ] T007 [P] [US1] Write serialization tests in `impl/golang/internal/validation/evidence_envelope_test.go` — test canonical JSON field ordering (FR-V03: type→version→validatorAgentId→...→evidenceURI), optional field alphabetical ordering (compositeRefs before observationWindow), observationWindow internal ordering (end before start per 006), omission of absent optional fields (FR-V06/006 FR-W04), numeric encoding (UnsignedInt256 as decimal string per 006 FR-W02), hex encoding (evidenceHash as 0x-prefixed lowercase per 006 FR-W06) (SC-V01)
- [ ] T008 [P] [US1] Write envelope hash computation tests in `impl/golang/internal/validation/evidence_envelope_test.go` — test EnvelopeHash() returns keccak256(canonicalJSON(envelope)) (FR-V09), test that hash is deterministic (sign twice, assert byte-equal per Principle X), test evidenceHash field independently matches keccak256 of mock off-chain document (FR-V05) (SC-V07)
- [ ] T009 [P] [US1] Write conformance test vector in `impl/golang/internal/validation/conformance_test.go` — golden test: construct envelope with predetermined field values, serialize, assert exact JSON bytes match expected string, assert EnvelopeHash() matches expected hex (SC-V01)

### Implementation (Green)

- [ ] T010 [US1] Implement canonical JSON serialization in `impl/golang/internal/validation/evidence_envelope.go` — MarshalJSON() method following FR-V03 field order, 006 wire format rules (FR-W01–W10), optional field handling (FR-V06) (SC-V01)
- [ ] T011 [US1] Implement EnvelopeHash() in `impl/golang/internal/validation/evidence_envelope.go` — returns keccak256(canonicalJSON(envelope)) using keylib.Keccak256 (FR-V09)
- [ ] T012 [US1] Implement EvidenceHashVerify() utility in `impl/golang/internal/validation/evidence_envelope.go` — given raw bytes and an envelope, verify keccak256(bytes) == envelope.EvidenceHash() (FR-V05, SC-V07)

**Checkpoint**: Evidence envelope serializes to canonical JSON, hashes are deterministic, conformance vector passes

---

## Phase 4: User Story 2 — Register as a Validator Agent (Priority: P1)

**Goal**: A validator registers with a `neuron-validator` service discoverable via Identity Registry lookup

**Independent Test**: Construct a ValidatorService, embed in agentURI, verify parseable after round-trip

### Tests (Red)

- [ ] T013 [P] [US2] Write ValidatorService construction tests in `impl/golang/internal/validation/validator_service_test.go` — test NewValidatorService() with valid domains and verdictDelivery (FR-V12), rejection of empty domains array, rejection of empty verdictDelivery, version field presence (SC-V04)
- [ ] T014 [P] [US2] Write ValidatorService parsing tests in `impl/golang/internal/validation/validator_service_test.go` — test ParseValidatorService() from agentURI service JSON object, verify type=="neuron-validator" check, extract domains/verdictDelivery/version fields, reject service objects with wrong type (FR-V11, FR-V12)
- [ ] T015 [P] [US2] Write specRef/domain format validation tests in `impl/golang/internal/validation/validator_service_test.go` — test IsValidSpecRef() accepts "005-health", "008-payment", custom domains "aviation", rejects empty strings (FR-V24)

### Implementation (Green)

- [ ] T016 [US2] Implement ValidatorService type in `impl/golang/internal/validation/validator_service.go` — immutable struct with type/name/version/domains/verdictDelivery fields, NewValidatorService() factory with validation (FR-V11, FR-V12, FR-V15)
- [ ] T017 [US2] Implement ParseValidatorService() in `impl/golang/internal/validation/validator_service.go` — parse neuron-validator service from agentURI JSON service object, extract and validate all fields (FR-V12, SC-V04)
- [ ] T018 [US2] Implement IsValidSpecRef() in `impl/golang/internal/validation/validator_service.go` — validate specRef format: "NNN-short-name" pattern or custom domain string (FR-V24)

**Checkpoint**: ValidatorService constructs, serializes to JSON, round-trips through parse, specRef validation works

---

## Phase 5: User Story 3 — Compose a Cross-Spec Validation Scenario (Priority: P2)

**Goal**: A validator composes individual evidence envelopes into a composite validation linked via compositeRefs

**Independent Test**: Build individual envelopes, compute composite refs, verify linkage integrity

### Tests (Red)

- [ ] T019 [P] [US3] Write CompositeValidation tests in `impl/golang/internal/validation/composite_test.go` — test ComputeCompositeRefs() computes keccak256(canonicalJSON(envelope)) for each individual envelope (FR-V06 clarification: payload hashes), test that compositeRefs array in composite envelope matches computed hashes, test individual envelopes are independently verifiable (FR-V21)
- [ ] T020 [P] [US3] Write Zero-to-Heartbeat scenario test in `impl/golang/internal/validation/composite_test.go` — test construction of 4 evidence envelopes (identity check, service check, heartbeat presence, deadline compliance per FR-V22), test composite envelope with compositeRefs linking all 4, test specRef values ("003-peer-registry", "004-topic-system", "005-health") (FR-V20, SC-V06)

### Implementation (Green)

- [ ] T021 [US3] Implement ComputeCompositeRefs() in `impl/golang/internal/validation/composite.go` — accepts slice of EvidenceEnvelopes, returns HexBytes slice of keccak256(canonicalJSON(envelope)) for each (FR-V06, FR-V21)
- [ ] T022 [US3] Implement NewCompositeEnvelope() in `impl/golang/internal/validation/composite.go` — factory that builds an EvidenceEnvelope with compositeRefs populated from individual envelopes, validates all individuals are well-formed (FR-V21)

**Checkpoint**: Composite envelopes link to individual envelopes via verifiable hashes

---

## Phase 6: User Story 4 — Read and Interpret Validation Evidence (Priority: P2)

**Goal**: A consumer can parse evidence envelopes from JSON, verify evidence chain integrity, and map verdicts to registry codes

**Independent Test**: Parse an envelope from JSON, verify evidenceHash against mock content, map verdict to registry code

### Tests (Red)

- [ ] T023 [P] [US4] Write envelope parsing tests in `impl/golang/internal/validation/evidence_envelope_test.go` — test UnmarshalJSON() from canonical JSON string, verify all fields parsed correctly, test version compatibility rejection (major 2 → reject per FR-V07), test unknown optional fields ignored (minor version tolerance per FR-V07) (SC-V02)
- [ ] T024 [P] [US4] Write evidence chain verification tests in `impl/golang/internal/validation/evidence_envelope_test.go` — test VerifyEvidenceChain(): given envelope + mock off-chain document bytes, verify keccak256(document) == evidenceHash (FR-V05, SC-V07), test VerifyResponseHash(): given envelope + responseHash from on-chain, verify keccak256(canonicalJSON(envelope)) == responseHash (FR-V09, VR-VF-02)
- [ ] T025 [P] [US4] Write divergent verdict acceptance test in `impl/golang/internal/validation/evidence_envelope_test.go` — construct two valid envelopes for same subjectAgentId/specRef with different verdicts ("compliant" and "inconclusive"), verify both pass validation independently (SC-V08)

### Implementation (Green)

- [ ] T026 [US4] Implement UnmarshalJSON() in `impl/golang/internal/validation/evidence_envelope.go` — parse evidence envelope from canonical JSON, validate all mandatory fields, apply version compatibility check (FR-V07), ignore unknown fields in minor versions (SC-V02)
- [ ] T027 [US4] Implement VerifyEvidenceChain() in `impl/golang/internal/validation/evidence_envelope.go` — given off-chain document bytes, verify keccak256(bytes) == evidenceHash (FR-V05, SC-V07)
- [ ] T028 [US4] Implement VerifyResponseHash() in `impl/golang/internal/validation/evidence_envelope.go` — given on-chain responseHash, verify keccak256(canonicalJSON(envelope)) == responseHash (FR-V09, VR-VF-02)

**Checkpoint**: Envelopes parse from JSON, evidence chain integrity verifiable, divergent verdicts both accepted

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Integration, documentation, and cross-spec validation

- [ ] T029 [P] Add evidence-grounded integration test in `impl/golang/internal/validation/integration_test.go` — full lifecycle: construct validator service → build envelope → serialize → compute envelope hash → verify evidence chain → verify response hash linkage (SC-V02, SC-V07, Principle XI)
- [ ] T030 [P] Add multi-validator divergent verdict integration test in `impl/golang/internal/validation/integration_test.go` — two validators produce envelopes for same subject with different verdicts, both valid, both compute distinct envelope hashes (SC-V08)
- [ ] T031 Update `impl/golang/internal/validation/doc.go` with complete package documentation including: package purpose (cross-cutting validation types per spec 010), key types (EvidenceEnvelope, OracleVerdict, ValidatorService, CompositeValidation), usage examples, FR traceability index

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 (EvidenceEnvelope type, OracleVerdict type)
- **US2 (Phase 4)**: Depends on Phase 2 only — can run in PARALLEL with US1
- **US3 (Phase 5)**: Depends on US1 (needs serialization + EnvelopeHash for compositeRefs)
- **US4 (Phase 6)**: Depends on US1 (needs serialization for UnmarshalJSON and hash verification)
- **Polish (Phase 7)**: Depends on all user stories complete

### User Story Dependencies

```text
Phase 1 (Setup)
    │
Phase 2 (Foundational)
    │
    ├──> Phase 3 (US1: Publish Verdict) ──> Phase 5 (US3: Cross-Spec)
    │                                    └──> Phase 6 (US4: Read Evidence)
    └──> Phase 4 (US2: Register Validator) [PARALLEL with US1]
    │
Phase 7 (Polish)
```

### Within Each User Story

- Tests (T0xx) MUST be written and FAIL before implementation (T0xx+)
- All tests within a phase marked [P] can run in parallel
- Implementation tasks within a phase are sequential (models → services → utilities)

### Parallel Opportunities

- **Phase 2**: T003 ∥ T004 (test files), T005 ∥ T006 (after tests pass — different files)
- **Phase 3 + 4**: US1 and US2 can proceed in parallel after Phase 2
- **Phase 3**: T007 ∥ T008 ∥ T009 (test files)
- **Phase 4**: T013 ∥ T014 ∥ T015 (test files)
- **Phase 5**: T019 ∥ T020 (test files)
- **Phase 6**: T023 ∥ T024 ∥ T025 (test files)
- **Phase 7**: T029 ∥ T030 ∥ T031 (independent files)

---

## Implementation Strategy

### MVP First (US1 + US2 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: US1 (Publish Verdict) + Phase 4: US2 (Register Validator) — in parallel
4. **STOP and VALIDATE**: Serialize an envelope, verify hashes, parse a validator service
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 (Publish) + US2 (Register) → Core validation types (MVP)
3. US3 (Cross-Spec) → Composite validation linking
4. US4 (Read Evidence) → Consumer-side parsing and verification
5. Polish → Integration tests, documentation

### Parallel Team Strategy

With multiple developers:
1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1 (Publish Verdict)
   - Developer B: US2 (Register Validator)
3. After US1 completes:
   - Developer A: US3 (Cross-Spec) or US4 (Read Evidence)
4. Polish phase together
