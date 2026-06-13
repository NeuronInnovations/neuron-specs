# Tasks: Identity Registry Smart Contract (EIP-8004)

**Input**: Design documents from `/specs/007-identity-contract/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Test tasks included per Constitution Principle IX (Test-First Development). Tests MUST be written and FAIL before implementation (Red-Green-Refactor).

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go SDK**: `internal/registry/` at module root; tests colocated as `*_test.go`
- **Spec documents**: `specs/007-identity-contract/`

## Constitution Compliance Notes

- **Principle VII**: Every task description references the FR-C-* or SC-C-* it satisfies
- **Principle IX**: Test tasks appear before implementation tasks within each phase (Red-Green-Refactor)
- **Principle X**: N/A for smart contract specs (signing handled by EVM transaction model)

---

## Phase 1: Setup

**Purpose**: Project initialization and Go module structure for registry package

- [ ] T001 Create `internal/registry/` directory and package declaration in `internal/registry/doc.go` — package documentation referencing Spec 007
- [ ] T002 [P] Add ABI type definitions and constants in `internal/registry/abi.go` — define Go constants for all custom error selectors, event signatures, and function selectors from data-model.md (FR-C-04, FR-C-05, FR-C-25, FR-C-32)
- [ ] T003 [P] Define shared types in `internal/registry/types.go` — Go structs for `FeedbackEntry`, `ValidationRecord`, `RegistrationInfo` (agentId + agentURI), response code constants (FR-C-29), and revert reason types from data-model.md (FR-C-06 through FR-C-08, FR-C-19, FR-C-26, FR-C-28)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core interfaces that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Define `IIdentityRegistry` Go interface in `internal/registry/identity_registry.go` — methods: `Register(agentURI) → tokenId`, `RegisterWithProof(agentURI, parentDIDProof) → tokenId`, `UpdateAgentURI(tokenId, newAgentURI)`, `Revoke(tokenId)`, `AgentURI(tokenId) → string`, `Lookup(address) → (tokenId, agentURI)`, `SetAdmissionPolicy(address)`, `AdmissionPolicy() → address`, `OwnerOf(tokenId) → address` per contracts/registry-contract.md (FR-C-02, FR-C-03, FR-C-05, FR-C-07, FR-C-08, FR-C-10, FR-C-12, FR-C-13)
- [ ] T005 [P] Define `IReputationRegistry` Go interface in `internal/registry/reputation_registry.go` — methods: `GiveFeedback(...)`, `RevokeFeedback(...)`, `AppendResponse(...)`, `GetSummary(...)` per contracts/reputation-contract.md (FR-C-20, FR-C-22, FR-C-23, FR-C-24)
- [ ] T006 [P] Define `IValidationRegistry` Go interface in `internal/registry/validation_registry.go` — methods: `ValidationRequest(...)`, `ValidationResponse(...)`, `GetValidationStatus(...)`, `GetSummary(...)` per contracts/validation-contract.md (FR-C-27, FR-C-28, FR-C-30, FR-C-31)
- [ ] T007 [P] Define `IAdmissionPolicy` Go interface in `internal/registry/admission_policy.go` — method: `IsAdmitted(childAddress, parentDIDProof) → bool` per contracts/admission-policy.md (FR-C-14)

**Checkpoint**: Foundation ready — all interfaces defined, user story implementation can begin

---

## Phase 3: User Story 1 — Register and Manage a Child Identity On-Chain (Priority: P1)

**Goal**: Child can register, update agentURI, lookup, and revoke identity via the Identity Registry contract

**Independent Test**: Deploy Identity Registry on local testnet → register → update → lookup → revoke → re-register. Verify token ownership, events, and revert conditions.

### Tests for User Story 1

> **Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T008 [P] [US1] Test `Register` in `internal/registry/identity_test.go` — Given unregistered address, When register(agentURI), Then tokenId > 0 and agentURI queryable via Lookup(address) (SC-C-01, FR-C-02, FR-C-04, FR-C-10)
- [ ] T009 [P] [US1] Test `Register` duplicate revert in `internal/registry/identity_test.go` — Given registered address, When register() again, Then revert AlreadyRegistered (SC-C-02, FR-C-06)
- [ ] T010 [P] [US1] Test `Register` empty agentURI revert in `internal/registry/identity_test.go` — Given empty string, When register(""), Then revert EmptyAgentURI (SC-C-14, FR-C-19)
- [ ] T011 [P] [US1] Test `UpdateAgentURI` success and unauthorized revert in `internal/registry/identity_test.go` — Given registered Child, When owner calls updateAgentURI Then success; When non-owner calls Then revert NotOwnerOrApproved (SC-C-03, FR-C-07)
- [ ] T012 [P] [US1] Test `Revoke` and re-registration in `internal/registry/identity_test.go` — Given registered Child, When owner calls revoke() Then token burned and lookup returns (0,""); When register() again Then new higher tokenId (SC-C-04, FR-C-08, FR-C-16)
- [ ] T013 [P] [US1] Test admin cannot mint/modify/revoke in `internal/registry/identity_test.go` — Given admin address, When admin calls register/update/revoke on behalf of Child Then revert (SC-C-05, FR-C-09)
- [ ] T014 [P] [US1] Test event emissions in `internal/registry/identity_test.go` — Verify IdentityRegistered, IdentityUpdated, IdentityRevoked events with correct indexed fields (SC-C-07, FR-C-04, FR-C-05)

### Implementation for User Story 1

- [ ] T015 [US1] Implement `IdentityRegistryClient` struct in `internal/registry/identity.go` — Go client wrapping go-ethereum contract binding for Identity Registry; implements `IIdentityRegistry` interface. Methods: `Register`, `RegisterWithProof`, `UpdateAgentURI`, `Revoke`, `AgentURI`, `Lookup`, `SetAdmissionPolicy`, `AdmissionPolicy`, `OwnerOf` (FR-C-02, FR-C-03, FR-C-05, FR-C-07, FR-C-08, FR-C-10, FR-C-12, FR-C-13)
- [ ] T016 [US1] Implement event parsing helpers in `internal/registry/events.go` — Parse `IdentityRegistered`, `IdentityUpdated`, `IdentityRevoked` events from transaction receipts into Go structs (FR-C-04, FR-C-05)
- [ ] T017 [US1] Implement error mapping in `internal/registry/errors.go` — Map contract revert reasons (`AlreadyRegistered`, `NotOwnerOrApproved`, `NotTokenOwner`, `EmptyAgentURI`, `AdmissionDenied`) to typed Go errors (FR-C-06, FR-C-07, FR-C-08, FR-C-19)

**Checkpoint**: User Story 1 fully functional — Child can register, update, lookup, and revoke identity

---

## Phase 4: User Story 2 — Enforce Admission Policy for Permissioned Registries (Priority: P2)

**Goal**: Platform operator can set, update, and manage admission policies; permissionless and allowlist modes work correctly

**Independent Test**: Deploy Identity Registry with AllowlistPolicy → add Parent DID → Child registers (success) → unlisted Child registers (revert) → remove Parent DID → existing registration persists

### Tests for User Story 2

- [ ] T018 [P] [US2] Test permissionless mode in `internal/registry/admission_test.go` — Given policy=address(0), When any address calls register(), Then admission not checked and registration proceeds (SC-C-06, FR-C-12)
- [ ] T019 [P] [US2] Test admission denial in `internal/registry/admission_test.go` — Given non-zero policy set, When unadmitted address calls register(), Then revert AdmissionDenied (SC-C-06, FR-C-12, FR-C-14)
- [ ] T020 [P] [US2] Test allowlist removal does not auto-revoke in `internal/registry/admission_test.go` — Given registered Child via allowed Parent DID, When Parent DID removed from allowlist, Then existing registration persists; new registration attempt by same Parent's Children reverts (FR-C-15)
- [ ] T021 [P] [US2] Test AdmissionPolicyUpdated event in `internal/registry/admission_test.go` — When owner calls setAdmissionPolicy(), Then emit AdmissionPolicyUpdated(oldPolicy, newPolicy) (SC-C-07, FR-C-13)

### Implementation for User Story 2

- [ ] T022 [US2] Implement admission policy check in `IdentityRegistryClient.Register` and `RegisterWithProof` in `internal/registry/identity.go` — When policy != address(0), call isAdmitted(). Overload (a) passes empty proof; overload (b) passes parentDIDProof (FR-C-02, FR-C-12, FR-C-14)
- [ ] T023 [US2] Implement `SetAdmissionPolicy` and `AdmissionPolicy` in `internal/registry/identity.go` — Owner-only set; emit AdmissionPolicyUpdated event; read current policy address (FR-C-13)
- [ ] T024 [P] [US2] Add admission event parsing in `internal/registry/events.go` — Parse `AdmissionPolicyUpdated` event from transaction receipts (FR-C-13)
- [ ] T025 [P] [US2] Add `AdmissionDenied` error mapping in `internal/registry/errors.go` (FR-C-12)

**Checkpoint**: User Story 2 complete — permissionless and permissioned registries work correctly

---

## Phase 5: User Story 3 — Give and Query Reputation Feedback for an Agent (Priority: P2)

**Goal**: Clients can give, revoke, and respond to feedback; tag-filtered summaries work

**Independent Test**: Deploy Identity + Reputation registries → register agent → give feedback → revoke feedback → append response → query summary with tag filter

### Tests for User Story 3

- [ ] T026 [P] [US3] Test `GiveFeedback` in `internal/registry/reputation_test.go` — Given registered agent, When client gives feedback(agentId, 450, 2, "quality", "speed", uri, hash), Then feedbackIndex returned, FeedbackGiven event emitted (SC-C-09, FR-C-20, FR-C-21, FR-C-25)
- [ ] T027 [P] [US3] Test `GiveFeedback` with unregistered agentId in `internal/registry/reputation_test.go` — When agentId not in Identity Registry, Then revert AgentNotRegistered (FR-C-26)
- [ ] T028 [P] [US3] Test `RevokeFeedback` in `internal/registry/reputation_test.go` — Given existing feedback, When original giver revokes, Then FeedbackRevoked emitted; When non-giver revokes, Then revert NotFeedbackGiver (SC-C-09, FR-C-22, FR-C-25)
- [ ] T029 [P] [US3] Test `AppendResponse` in `internal/registry/reputation_test.go` — Given existing feedback, When agent owner responds, Then ResponseAppended emitted; When non-owner responds, Then revert NotAgentOwner (FR-C-23, FR-C-25)
- [ ] T030 [P] [US3] Test `GetSummary` with tag filtering in `internal/registry/reputation_test.go` — Given multiple feedback with different tags, When getSummary filtered by tag1="quality", Then only matching entries counted; revoked entries excluded (SC-C-09, FR-C-24)

### Implementation for User Story 3

- [ ] T031 [US3] Implement `ReputationRegistryClient` struct in `internal/registry/reputation.go` — Go client wrapping go-ethereum contract binding; implements `IReputationRegistry`. Methods: `GiveFeedback`, `RevokeFeedback`, `AppendResponse`, `GetSummary` (FR-C-20, FR-C-22, FR-C-23, FR-C-24)
- [ ] T032 [P] [US3] Add reputation event parsing in `internal/registry/events.go` — Parse `FeedbackGiven`, `FeedbackRevoked`, `ResponseAppended` events (FR-C-25)
- [ ] T033 [P] [US3] Add reputation error mapping in `internal/registry/errors.go` — `AgentNotRegistered`, `NotFeedbackGiver`, `NotAgentOwner` (FR-C-22, FR-C-23, FR-C-26)

**Checkpoint**: User Story 3 complete — reputation feedback lifecycle works end-to-end

---

## Phase 6: User Story 4 — Request and Provide Third-Party Validation (Priority: P2)

**Goal**: Agent owners can request validation; validators can respond; summaries work

**Independent Test**: Deploy Identity + Validation registries → register agent → create validation request → validator responds → query status → query summary with tag filter

### Tests for User Story 4

- [ ] T034 [P] [US4] Test `ValidationRequest` in `internal/registry/validation_test.go` — Given registered agent, When agent owner calls validationRequest(), Then ValidationRequested event emitted; When non-owner calls, Then revert NotAgentOwner (SC-C-10, FR-C-27, FR-C-32)
- [ ] T035 [P] [US4] Test `ValidationResponse` in `internal/registry/validation_test.go` — Given pending request, When addressed validator responds with pass(1), Then ValidationResponded emitted; When non-validator responds, Then revert NotAddressedValidator (SC-C-10, FR-C-28, FR-C-29, FR-C-32)
- [ ] T036 [P] [US4] Test `GetValidationStatus` in `internal/registry/validation_test.go` — Given responded validation, When getValidationStatus(requestHash), Then returns correct validator, agentId, response, responseHash, tag, lastUpdate (FR-C-30)
- [ ] T037 [P] [US4] Test `GetSummary` with tag filtering in `internal/registry/validation_test.go` — Given multiple validations with different tags, When getSummary filtered by tag="security", Then only matching entries counted (FR-C-31)
- [ ] T038 [P] [US4] Test duplicate requestHash revert in `internal/registry/validation_test.go` — When validationRequest called with existing requestHash, Then revert RequestAlreadyExists (FR-C-27)

### Implementation for User Story 4

- [ ] T039 [US4] Implement `ValidationRegistryClient` struct in `internal/registry/validation.go` — Go client wrapping go-ethereum contract binding; implements `IValidationRegistry`. Methods: `ValidationRequest`, `ValidationResponse`, `GetValidationStatus`, `GetSummary` (FR-C-27, FR-C-28, FR-C-30, FR-C-31)
- [ ] T040 [P] [US4] Add validation event parsing in `internal/registry/events.go` — Parse `ValidationRequested`, `ValidationResponded` events (FR-C-32)
- [ ] T041 [P] [US4] Add validation error mapping in `internal/registry/errors.go` — `NotAddressedValidator`, `RequestAlreadyExists`, `NotAgentOwner` for validation context (FR-C-27, FR-C-28)

**Checkpoint**: User Story 4 complete — validation request/response lifecycle works end-to-end

---

## Phase 7: User Story 5 — Deploy Registries with Proxy Upgradeability (Priority: P2)

**Goal**: Document and test EIP-1967 proxy deployment; verify Hedera EVM compatibility

**Independent Test**: Deploy proxied Identity Registry → register agents → upgrade implementation → verify state preserved → document Hedera differences

### Tests for User Story 5

- [ ] T042 [P] [US5] Test proxy deployment preserves behavior in `internal/registry/proxy_test.go` — Given proxied Identity Registry, When agents register through proxy, Then behavior identical to non-proxied (SC-C-12, FR-C-34)
- [ ] T043 [P] [US5] Test proxy upgrade preserves state in `internal/registry/proxy_test.go` — Given proxied registry with registrations, When proxy admin upgrades implementation, Then all registrations and agentURIs preserved (SC-C-12, FR-C-34)
- [ ] T044 [P] [US5] Test proxy admin separation in `internal/registry/proxy_test.go` — Given separate proxy admin and registry admin, When proxy admin calls upgrade Then success; When registry admin calls upgrade Then revert (FR-C-35)

### Implementation for User Story 5

- [ ] T045 [US5] Add proxy deployment helpers in `internal/registry/deploy.go` — Functions to deploy Identity/Reputation/Validation registries behind EIP-1967 proxies with separate ProxyAdmin (FR-C-34, FR-C-35)
- [ ] T046 [US5] Document Hedera EVM deployment differences in `specs/007-identity-contract/spec.md` Appendix C — Verify and update gas model, account model, finality, contract size, precompile notes (FR-C-38, SC-C-11)

**Checkpoint**: User Story 5 complete — proxy deployment works on Ethereum and Hedera EVM

---

## Phase 8: User Story 6 — Observe Agent Trust Root from Registry (Priority: P3)

**Goal**: Health observers can verify agent registration via Identity Registry lookup

**Independent Test**: Deploy Identity Registry → register agent → simulate observer calling lookup(senderAddress) and ownerOf(tokenId) → verify match

### Tests for User Story 6

- [ ] T047 [P] [US6] Test observer trust root verification in `internal/registry/observer_test.go` — Given registered agent, When observer calls Lookup(senderAddress), Then returns (tokenId, agentURI); When observer calls OwnerOf(tokenId), Then result matches senderAddress (SC-C-08, FR-C-10)
- [ ] T048 [P] [US6] Test unregistered sender returns empty in `internal/registry/observer_test.go` — Given unregistered address, When Lookup called, Then returns (0, "") (SC-C-08, FR-C-10)

### Implementation for User Story 6

- [ ] T049 [US6] Implement `VerifyAgentRegistration` helper in `internal/registry/observer.go` — Combines Lookup + OwnerOf verification into a single function that returns registration validity for health observer use case (SC-C-13, FR-C-10)
- [ ] T050 [US6] Document cross-spec integration in `specs/007-identity-contract/spec.md` — Verify Appendix E Mermaid diagram for observer trust root flow matches implementation (FR-C-10)

**Checkpoint**: User Story 6 complete — health observers can verify agent trust root

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Cross-spec references, SDK interface mapping, final documentation

- [ ] T051 [P] Add forward reference to Spec 007 in `specs/003-peer-registry/spec.md` — In Related Specs section, add link to 007 with note that 003's RegistryContract interface maps to 007's contract methods (SC-C-13)
- [ ] T052 [P] Update `CLAUDE.md` Project Structure — Add `internal/registry/` entry with 007 description; add Solidity ^0.8.20 (informative) to Active Technologies
- [ ] T053 Verify SDK interface mapping in `internal/registry/contract.go` — Ensure Go SDK `RegistryContract` interface methods map to deployed contract ABI: Register→register, UpdateAgentURI→updateAgentURI, Revoke→revoke, Lookup→lookup (SC-C-13)
- [ ] T054 [P] Verify ERC-165 `supportsInterface` support in `internal/registry/identity.go` — Add `SupportsInterface(interfaceId) → bool` method to `IIdentityRegistry` and test it returns true for ERC-721, ERC-721Enumerable, and IIdentityRegistry (FR-C-36)
- [ ] T055 [P] Verify reentrancy protection documentation in `specs/007-identity-contract/spec.md` — Ensure FR-C-18 is addressed in all state-modifying function descriptions in contracts/*.md (FR-C-18)
- [ ] T056 Run quickstart.md validation — Walk through all 6 scenarios in `specs/007-identity-contract/quickstart.md` against implemented code to verify accuracy

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup (T001-T003) — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational (T004-T007) — BLOCKS US2 (admission extends identity)
- **US2 (Phase 4)**: Depends on US1 (identity registration must work first)
- **US3 (Phase 5)**: Depends on Foundational (T004-T007) — can run in parallel with US1/US2
- **US4 (Phase 6)**: Depends on Foundational (T004-T007) — can run in parallel with US1/US2/US3
- **US5 (Phase 7)**: Depends on US1 (needs working identity registry to test proxy)
- **US6 (Phase 8)**: Depends on US1 (needs working identity registry for lookup)
- **Polish (Phase 9)**: Depends on all user stories being complete

### User Story Dependencies

```text
Phase 1 (Setup) → Phase 2 (Foundational)
                       ↓
              ┌────────┼────────┐
              ↓        ↓        ↓
           US1(P1)   US3(P2)  US4(P2)    ← US3/US4 can start after Foundational
              ↓
           US2(P2)   ← US2 depends on US1
              ↓
         ┌────┴────┐
         ↓         ↓
      US5(P2)   US6(P3)    ← Both depend on US1
              ↓
        Polish (Phase 9)
```

### Within Each User Story

1. Tests MUST be written and FAIL before implementation (Principle IX)
2. Interfaces before clients
3. Clients before event parsing and error mapping
4. Core implementation before integration helpers
5. Story complete before moving to next priority

### Parallel Opportunities

**Phase 2** (all [P]):
```
T004 (IIdentityRegistry) | T005 (IReputationRegistry) | T006 (IValidationRegistry) | T007 (IAdmissionPolicy)
```

**Phase 3 tests** (all [P]):
```
T008 | T009 | T010 | T011 | T012 | T013 | T014
```

**US3 and US4 in parallel** (after Foundational):
```
Developer A: US3 (T026-T033) — Reputation
Developer B: US4 (T034-T041) — Validation
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T007)
3. Complete Phase 3: US1 — Identity CRUD (T008-T017)
4. **STOP and VALIDATE**: Test register, update, lookup, revoke independently
5. Deploy/demo if ready — this is a functional identity registry

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 → Identity CRUD → Test independently → **MVP!**
3. US2 → Admission Policy → Test independently
4. US3 + US4 (parallel) → Reputation + Validation → Test independently
5. US5 → Proxy Deployment → Test independently
6. US6 → Observer Trust Root → Test independently
7. Polish → Cross-spec refs, docs, final validation

### Parallel Team Strategy

With multiple developers after Foundational:
- **Developer A**: US1 → US2 → US5 (Identity → Admission → Proxy)
- **Developer B**: US3 (Reputation) — independent after Foundational
- **Developer C**: US4 (Validation) — independent after Foundational
- **Together**: US6 → Polish

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing (Red-Green-Refactor)
- All FR-C-* and SC-C-* references enable Constitution Principle VII traceability
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
