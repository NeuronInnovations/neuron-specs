# Tasks: Payment (Spec 008)

**Input**: Design documents from `specs/008-payment/`
**Prerequisites**: plan.md (complete), spec.md (complete), data-model.md (complete), contracts/ (complete)

**Tests**: Included per Constitution Principle IX (Test-First Development). Test tasks precede implementation tasks.

**Organization**: Tasks grouped by user story. Each story is independently testable after its phase completes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Exact file paths included in descriptions

## Path Conventions

- Go SDK layout per Constitution VI: `internal/payment/`, `internal/registry/`
- Tests colocated as `*_test.go`
- All paths relative to `impl/golang/`

## Constitution Compliance Notes

- **Principle VII**: Every task references FR-* or SC-* it satisfies
- **Principle IX**: Test tasks (Red) precede implementation tasks (Green) in each phase
- **Principle X**: Signing tasks include determinism verification (sign twice, assert byte-equal)
- **Principle XI**: Integration tests include validator-perspective scenarios

---

## Phase 1: Setup

**Purpose**: Create payment package structure and shared infrastructure

- [x] T001 Create `internal/payment/doc.go` with package-level godoc documenting Spec 008 scope, dependency chain (keylib→account→topic→registry→payment), and architecture decisions DD-P01–P04
- [x] T002 [P] Create `internal/payment/errors.go` with `PaymentError` structured error type, `PaymentErrorKind` enum (16 kinds per FR-P32), and `NEURON-PAYMENT-*` error code constants. FR-P32
- [x] T003 [P] Add `ServiceTypeNeuronCommerce = "neuron-commerce"` constant to `internal/registry/service.go`. FR-P01
- [x] T004 [P] Add `InvalidDeliveryRef` error type to `internal/registry/errors.go` with `ServiceName`, `DeliveryRef`, `Mode` fields. FR-P01b, FR-P32

**Checkpoint**: Package structure exists, error types defined, no logic yet.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types that ALL user stories depend on. MUST complete before any story phase.

### Tests (Red)

- [x] T005 [P] Write tests for `NeuronCommerceService` construction and validation in `internal/payment/commerce_service_test.go`: valid P2P config, valid topic config, valid custom config, missing name, missing delivery mode, P2P without serviceRef, topic without channelRef, empty settlement binding, empty pricing fields. FR-P01, FR-P01a
- [x] T006 [P] Write tests for `DeliveryDescriptor`, `SettlementDescriptor`, `PricingDescriptor` construction in `internal/payment/commerce_service_test.go`: valid values, invalid delivery mode, custom mode pattern validation. FR-P01a, FR-P02, FR-P03
- [x] T007 [P] Write tests for `NeuronCommerceService` canonical JSON round-trip in `internal/payment/commerce_service_test.go`: serialize→deserialize byte-identical, field ordering matches FR-P01 (`type→name→version→delivery→settlement→pricing→termsRef*`), optional termsRef omitted when empty. SC-P02, SC-P10
- [x] T008 [P] Write tests for `AgreementState` enum and state machine transitions in `internal/payment/agreement_test.go`: all 13 valid transitions from FR-P14, reject invalid transitions, IDLE initial state. FR-P13, FR-P14, SC-P04
- [x] T009 [P] Write tests for `EscrowAdapter` interface types (`EscrowRef`, `ReleaseRequestRef`, `Balance`, `DepositResult`, `ReleaseResult`) in `internal/payment/escrow_test.go`: construction, opaque locator. FR-P16–P23

### Implementation (Green)

- [x] T010 [P] Implement `DeliveryMode` type, `DeliveryDescriptor`, `SettlementDescriptor`, `PricingDescriptor` in `internal/payment/commerce_service.go` with validation and canonical JSON `MarshalJSON()`. FR-P01a, FR-P02, FR-P03
- [x] T011 Implement `NeuronCommerceService` struct with constructor `NewNeuronCommerceService(name, version, delivery, settlement, pricing, opts...)` in `internal/payment/commerce_service.go`. Hardcode `type = "neuron-commerce"`. Validate all MUST fields. Support `WithTermsRef(uri)` option. Canonical JSON: `type→name→version→delivery→settlement→pricing→termsRef*`. FR-P01, FR-P04
- [x] T012 Implement `AgreementState` enum (10 states) and `AgreementStateMachine` with `Transition(event)` method in `internal/payment/agreement.go`. Track per `requestId`. All 13 transitions from FR-P14. Return `NegotiationFailed` on invalid transition. FR-P13, FR-P13a, FR-P14, FR-P14a
- [x] T013 [P] Implement `EscrowAdapter` interface, `EscrowRef`, `ReleaseRequestRef`, `Balance`, `DepositResult`, `ReleaseResult` value types in `internal/payment/escrow.go`. FR-P16–P23

**Checkpoint**: Foundation types exist, state machine works, tests pass. Ready for user story phases.

---

## Phase 3: User Story 2 — Publish a Commercial Service Offering (Priority: P1)

**Goal**: Seller can create a `neuron-commerce` entry and include it in agentURI with canonical JSON round-trip.

**Independent Test**: Create `neuron-commerce` entry, add to agentURI, serialize to JSON, deserialize, verify byte-identical.

### Tests (Red)

- [x] T014 [P] [US2] Write tests for AgentURI with commerce services in `internal/registry/agent_uri_test.go`: NewAgentURI accepts commerce services, CommerceServices() accessor returns defensive copy, MarshalJSON ordering (topic→p2p→commerce→DID), UnmarshalJSON routes neuron-commerce type. FR-P05, SC-P02, SC-P10
- [x] T015 [P] [US2] Write tests for delivery cross-reference validation in `internal/registry/agent_uri_test.go`: P2P serviceRef matches existing p2p service name, topic channelRef matches existing topic service name, broken serviceRef → InvalidDeliveryRef error, broken channelRef → InvalidDeliveryRef error. FR-P01b

### Implementation

- [x] T016 [US2] Add `commerceServices []NeuronCommerceService` field to `AgentURI` struct in `internal/registry/agent_uri.go`. Add `CommerceServices()` accessor (defensive copy). Update `NewAgentURI()` signature to accept optional `[]NeuronCommerceService`. FR-P05
- [x] T017 [US2] Update `AgentURI.MarshalJSON()` in `internal/registry/agent_uri.go` to emit commerce services after p2p, before DID (ordering: topic→p2p→commerce→DID per 003 data-model amendment). FR-P01, SC-P02
- [x] T018 [US2] Update `AgentURI.UnmarshalJSON()` in `internal/registry/agent_uri.go` to route `"neuron-commerce"` type discriminator to `commerceServices` slice. Silently ignore unknown fields for forward compatibility (FR-P12a). FR-P01
- [x] T019 [US2] Add delivery cross-reference validation to `ValidateRegistrationCompleteness()` in `internal/registry/validation.go`: for each commerce service, validate delivery.serviceRef or delivery.channelRef against existing services. Produce InvalidDeliveryRef on broken ref. FR-P01b

**Checkpoint**: Seller can publish commerce offerings in agentURI. Round-trip JSON verified.

---

## Phase 4: User Story 1 — Discover and Purchase a Service (Priority: P1)

**Goal**: Full buyer→seller commerce flow: discover offering, negotiate (accept), create escrow, deliver, invoice, settle.

**Independent Test**: Two agents exchange serviceRequest→serviceResponse(accept)→escrowCreated→invoice→invoiceAck(approved) on same binding.

### Tests (Red)

- [x] T020 [P] [US1] Write tests for all 6 negotiation payload types (serviceRequest, serviceResponse, connectionSetup, escrowCreated, invoice, invoiceAck) canonical JSON construction and round-trip in `internal/payment/negotiation_test.go` and `internal/payment/escrow_payloads_test.go`. Verify field ordering per FR-P12. SC-P03
- [x] T021 [P] [US1] Write determinism test: sign same payload twice via `topic.NewTopicMessage()`, assert byte-equal signatures in `internal/payment/negotiation_test.go`. SC-P03, Constitution X
- [x] T022 [P] [US1] Write agreement lifecycle integration test in `internal/payment/agreement_test.go`: full IDLE→REQUESTED→AGREED→FUNDED→ACTIVE→INVOICED→ACTIVE→INVOICED→ACTIVE→COMPLETED cycle — must exercise at least two ACTIVE⇄INVOICED iterations to verify FR-P15 continuous streaming (buyer deposits more after first invoiceAck, seller issues second invoice). SC-P01, SC-P04, FR-P15
- [x] T023 [P] [US1] Write `agreementHash` verification test: `keccak256(canonicalJSON(acceptedServiceResponse))` matches stored hash. SC-P07

### Implementation

- [x] T024 [P] [US1] Implement `ServiceRequest` payload type with constructor, validation, and canonical JSON `MarshalJSON()` in `internal/payment/negotiation.go`. Fields per FR-P07. Include `serviceParams` with lexicographic key ordering. FR-P07, FR-P12
- [x] T025 [P] [US1] Implement `ServiceResponse` payload type with constructor and canonical JSON in `internal/payment/negotiation.go`. Handle `accept`/`reject`/`counter` actions. Conditional fields `counterAmount`/`counterInterval`. FR-P08, FR-P12
- [x] T026 [P] [US1] Implement `ConnectionSetup` payload type with constructor and canonical JSON in `internal/payment/negotiation.go`. `encryptedMultiaddrs` as base64 string per FR-W03. Optional `natStatus`. FR-P33, FR-P34, FR-P12
- [x] T027 [P] [US1] Implement `EscrowCreated`, `Invoice`, `InvoiceAck` payload types with constructors and canonical JSON in `internal/payment/escrow_payloads.go`. FR-P09, FR-P10, FR-P11, FR-P12
- [x] T028 [US1] Implement `ComputeAgreementHash(acceptedResponse ServiceResponse) [32]byte` utility: `keccak256(canonicalJSON(serviceResponse))` in `internal/payment/agreement.go`. FR-P17, SC-P07
- [x] T029 [US1] Implement `ValidateVersion(version string) error` utility enforcing FR-P12a rule (accept 1.x.y, reject 2+) in `internal/payment/negotiation.go`. FR-P12a

**Checkpoint**: Full negotiation→escrow→invoice→settlement flow works with in-memory state.

---

## Phase 5: User Story 3 — Negotiate Terms Before Commitment (Priority: P2)

**Goal**: Buyer and seller can counter-offer before committing. State machine handles NEGOTIATING state.

**Independent Test**: Exchange serviceRequest→serviceResponse(counter)→serviceRequest(revised)→serviceResponse(accept). Verify IDLE→REQUESTED→NEGOTIATING→AGREED transitions.

### Tests (Red)

- [x] T030 [P] [US3] Write counter-offer negotiation test in `internal/payment/agreement_test.go`: REQUESTED→NEGOTIATING on counter, NEGOTIATING→AGREED on accept, NEGOTIATING→REJECTED on withdraw. FR-P14
- [x] T031 [P] [US3] Write negotiation deadline expiry test in `internal/payment/agreement_test.go`: auto-transition to REJECTED when deadline elapses in REQUESTED or NEGOTIATING state. FR-P07a

### Implementation

- [x] T032 [US3] Add `negotiationDeadline` expiry logic to `AgreementStateMachine` in `internal/payment/agreement.go`: `CheckDeadline(now uint64)` method auto-transitions to REJECTED if `now > negotiationDeadline`. FR-P07a, FR-P14
- [x] T033 [US3] Add counter-offer handling: when `ServiceResponse.Action == "counter"`, extract `counterAmount`/`counterInterval` and keep machine in NEGOTIATING. FR-P08, FR-P14

**Checkpoint**: Negotiation works with counter-offers and deadline enforcement.

---

## Phase 6: User Story 4 — Refund on Timeout (Priority: P2)

**Goal**: Buyer can claim refund after escrow timeout elapses.

**Independent Test**: Create escrow with short timeout, wait for expiry, call claimRefund, verify funds return.

### Tests (Red)

- [x] T034 [P] [US4] Write escrow precondition tests in `internal/payment/escrow_test.go`: claimRefund succeeds after timeout, claimRefund fails before timeout with `RefundNotEligible`/`TimeoutNotElapsed` error, requestRelease fails when amount exceeds available balance with `InsufficientBalance` error. FR-P22, FR-P25a, FR-P25b, SC-P06, SC-P09

### Implementation

- [x] T035 [US4] Add timeout validation to mock EscrowAdapter `claimRefund()` in `internal/payment/escrow.go`: check `currentTime >= timeout`, return `RefundNotEligible` or `TimeoutNotElapsed` error if not. FR-P22, FR-P25

**Checkpoint**: Refund mechanism works with timeout enforcement.

---

## Phase 7: User Story 5 — Multi-Binding Settlement Discovery (Priority: P3)

**Goal**: Buyer discovers seller with multiple settlement bindings and selects preferred one.

**Independent Test**: Create seller with two neuron-commerce entries (same service, different bindings), verify buyer can filter by binding.

### Tests (Red)

- [x] T036 [P] [US5] Write multi-binding discovery test in `internal/payment/commerce_service_test.go`: agentURI with two neuron-commerce entries (hedera-native + evm-escrow), filter by binding, both visible with distinct settlement objects. FR-P05, SC-P01

### Implementation

- [x] T037 [US5] Implement `FilterCommerceByBinding(services []NeuronCommerceService, binding string) []NeuronCommerceService` utility in `internal/payment/commerce_service.go`. FR-P05
- [x] T038 [US5] Implement `FilterCommerceByName(services []NeuronCommerceService, name string) []NeuronCommerceService` utility in `internal/payment/commerce_service.go`. FR-P05

**Checkpoint**: Multi-binding discovery and selection works.

---

## Phase 8: User Story 6 — Trust-Gated Engagement (Priority: P3)

**Goal**: Buyer performs SHOULD-level trust checks before funding escrow.

**Independent Test**: Query seller's liveness, registration, reputation, validation status and verify pass/fail.

### Tests (Red)

- [x] T039 [P] [US6] Write trust gating tests in `internal/payment/trust_test.go`: all checks pass → proceed, seller DEAD → warning, missing registration → warning. FR-P30

### Implementation

- [x] T040 [US6] Implement `PreEngagementCheck(sellerAddress, healthObserver, registryContract, reputationContract, validationContract) (TrustResult, error)` in `internal/payment/trust.go`. SHOULD-level: returns warnings, not errors. FR-P30, FR-P31
- [x] T041 [US6] Implement `PostSettlementFeedback(buyerKey, reputationContract, agentId, rating, comment)` in `internal/payment/trust.go`. Direct call preserving msg.sender. FR-P31

**Checkpoint**: Trust gating works as SHOULD-level recommendation.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Integration, determinism, and validator-perspective tests

- [x] T042 [P] Write full end-to-end integration test in `internal/payment/integration_test.go`: US1 complete flow (discover→negotiate→escrow→deliver→invoice→settle) with mock TopicAdapter and mock EscrowAdapter. Verify `requestRelease` recipient defaults to seller's `PaymentAddress()` (resolved via 001 FR-023/024). SC-P01, FR-P28
- [x] T043 [P] Write determinism verification test in `internal/payment/integration_test.go`: same message sequence on two independent state machines produces same AgreementState. SC-P04
- [x] T044 [P] Write `evidenceHash` verification test in `internal/payment/integration_test.go`: `keccak256(canonicalJSON(deliveryProofTopicMessage))` matches stored evidenceHash. SC-P08
- [x] T045 [P] Write validator-perspective test in `internal/payment/integration_test.go`: simulate third-party observing stdIn/stdOut messages, verify VR-PAY-01 through VR-PAY-07 pass conditions. VR-PAY-01–07
- [x] T046 Run `go vet ./internal/payment/...` and `go vet ./internal/registry/...` — zero warnings
- [x] T047 Verify all existing tests still pass: `go test ./internal/...`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1. BLOCKS all user stories.
- **Phase 3 (US2)**: Depends on Phase 2. Can start immediately after foundational. (**Note**: US2 before US1 because US1 needs commerce types from US2)
- **Phase 4 (US1)**: Depends on Phase 2 + Phase 3 (needs commerce types and agentURI integration)
- **Phase 5 (US3)**: Depends on Phase 4 (extends negotiation from US1)
- **Phase 6 (US4)**: Depends on Phase 2 only (escrow types independent of negotiation)
- **Phase 7 (US5)**: Depends on Phase 3 (needs commerce types)
- **Phase 8 (US6)**: Depends on Phase 2 only (trust checks independent of commerce)
- **Phase 9 (Polish)**: Depends on Phases 3–8

### User Story Dependencies

```
Phase 2 (Foundational) ──┬──> Phase 3 (US2: Publish Offering) ──> Phase 4 (US1: Discover & Purchase)
                         │                                         │
                         │                                         └──> Phase 5 (US3: Negotiate)
                         │
                         ├──> Phase 6 (US4: Refund) [independent]
                         ├──> Phase 7 (US5: Multi-Binding) [depends on US2]
                         └──> Phase 8 (US6: Trust) [independent]
```

### Parallel Opportunities

Within Phase 2: T005–T009 (all tests) can run in parallel, then T010–T013 (all implementations) in parallel.
Phase 6 (US4) and Phase 8 (US6) can run in parallel with Phase 3 (US2).
Phase 7 (US5) can start as soon as Phase 3 completes.

---

## Implementation Strategy

### MVP First (User Stories 2 + 1 Only)

1. Complete Phase 1 (Setup) + Phase 2 (Foundational)
2. Complete Phase 3 (US2: Publish Offering) — seller can advertise
3. Complete Phase 4 (US1: Discover & Purchase) — full commerce flow
4. **STOP and VALIDATE**: Test end-to-end flow independently
5. Deploy/demo: agents can discover, negotiate, and settle payments

### Incremental Delivery

1. Setup + Foundational → Types and state machine ready
2. US2 → Sellers can publish offerings (agentURI integration verified)
3. US1 → Full commerce flow (negotiate + escrow + invoice + settle)
4. US3 → Counter-offers and deadline enforcement
5. US4 → Refund safety net
6. US5 → Multi-chain selection
7. US6 → Trust gating (SHOULD-level)

---

## Summary

| Metric | Value |
|--------|-------|
| Total tasks | 47 |
| Phase 1 (Setup) | 4 tasks |
| Phase 2 (Foundational) | 9 tasks |
| Phase 3 (US2) | 6 tasks |
| Phase 4 (US1) | 10 tasks |
| Phase 5 (US3) | 4 tasks |
| Phase 6 (US4) | 2 tasks |
| Phase 7 (US5) | 3 tasks |
| Phase 8 (US6) | 3 tasks |
| Phase 9 (Polish) | 6 tasks |
| Parallel opportunities | 31 tasks marked [P] |
| MVP scope | Phases 1–4 (US2 + US1): 29 tasks |

---

## Phase 10 — 2026-05-08 Amendment

Tasks added by the spec amendment ratified in `spec.md` Clarifications Session 2026-05-08. These are appended after Phase 9 (Polish) of the original plan; they do NOT block any of the original 47 tasks but DO depend on Phase 2 (Foundational) being complete.

All tasks below are **TEST-FIRST** per Constitution Principle IX. Each implementation task is preceded by its corresponding test task.

### Lifecycle messages (FR-P36, FR-P37, FR-P38)

- **T-NA-001 [test]** — Add canonical-JSON round-trip tests for `serviceStop`, `serviceCancel`, `serviceRenew` payloads in `internal/payment/lifecycle_test.go`. Verify field order per FR-P12 amendment. Tests MUST FAIL initially. Trace: SC-P12.
- **T-NA-002** — Implement `ServiceStopPayload`, `ServiceCancelPayload`, `ServiceRenewPayload` types in `internal/payment/lifecycle.go` with `MarshalJSON`/`UnmarshalJSON` honoring canonical field order. Trace: FR-P36, FR-P37, FR-P38.
- **T-NA-003 [test]** — State-machine transition tests in `internal/payment/agreement_lifecycle_test.go`: ACTIVE+EventStop → TERMINATED, REQUESTED+EventCancel → TERMINATED, ACTIVE+EventRenew → ACTIVE (no transition, expiresAt updated). Trace: FR-P14 amendment.
- **T-NA-004** — Extend `AgreementStateMachine` event enum in `internal/payment/agreement.go` with `EventStop`, `EventCancel`, `EventRenew` and the corresponding transition rules. Trace: FR-P14, FR-P36, FR-P37, FR-P38.
- **T-NA-005 [test]** — RenewExpiryNotMonotonic error test: send `serviceRenew` with `extendUntil` ≤ current `expiresAt`; expect ignore + error surface. Trace: FR-P38.
- **T-NA-006** — Wire `serviceStop` / `serviceCancel` handlers into the seller's commerce loop in `internal/edgeapp/commerce.go` and the buyer's commerce loop in `internal/edgeapp/buyer.go`. Trace: FR-P36, FR-P37.

### streams[] catalog (FR-P33a)

- **T-NA-010 [test]** — Canonical-JSON round-trip test for `connectionSetup` with `streams[]` catalog containing literal, wildcard, and buyer-initiated entries. Trace: SC-P14.
- **T-NA-011** — Add `Streams []StreamCatalogEntry` to `ConnectionSetup` in `internal/payment/negotiation.go`. Define `StreamCatalogEntry` type with `Name`, `ProtocolID`, `Direction`, optional `Schema`. Trace: FR-P33a.
- **T-NA-012 [test]** — Validate cross-field constraint: at least one of `Protocol` or `Streams` MUST be present. Trace: FR-P33.
- **T-NA-013** — Implement validation in `ConnectionSetup.UnmarshalJSON` rejecting payloads with neither `Protocol` nor `Streams`. Trace: FR-P33.

### Active-service persistence (FR-P40–P44)

- **T-NA-020 [test]** — Define `ActiveServiceStore` interface contract test in `internal/payment/persistence/store_test.go`: Save, Load, Replay, Evict operations. Run against both Memory and JSONFile implementations. Trace: FR-P40, FR-P41, FR-P42.
- **T-NA-021** — Implement `ActiveServiceStore` interface and `MemoryStore` in `internal/payment/persistence/`. Trace: FR-P40.
- **T-NA-022** — Implement `JSONFileStore` (atomic temp+rename write per existing `state.go` pattern) in `internal/payment/persistence/json_file_store.go`. Trace: FR-P40, FR-P43.
- **T-NA-023 [test]** — Restart-and-resume integration test: start seller in goroutine, establish ACTIVE agreement, kill seller goroutine, restart with same JSONFileStore, observe data-plane resume within 30 seconds without re-negotiation. Trace: SC-P13.
- **T-NA-024** — Wire `ActiveServiceStore` into `internal/edgeapp/seller.go` startup: replay all entries with state ∈ {AGREED, FUNDED, ACTIVE, INVOICED} whose `expiresAt` is in the future. Trace: FR-P41.
- **T-NA-025** — Add configurable `ActiveServiceCutoff` to `internal/edgeapp/config.go` with default 86400s. Wire eviction sweep on a periodic timer. Trace: FR-P42.
- **T-NA-026 [test]** — Eviction test: an entry past `expiresAt` is removed on the next sweep; an entry with valid `serviceRenew` extending `expiresAt` survives. Trace: FR-P42.

### Long-lived service discipline (FR-P45–P47)

- **T-NA-030** — Audit `internal/edgeapp/seller.go` for any code path that closes the data-plane stream on resource pressure, transient transport faults, or control-plane unavailability. Document each occurrence; remove or downgrade to logging-only. Trace: FR-P46.
- **T-NA-031 [test]** — Slow-buyer back-pressure test: simulate a buyer that stops reading; verify the seller drops frames in FIFO order rather than closing the stream. Trace: FR-P46 (resource pressure case), 016 SC-A07 (DApp-side equivalent).

### Degraded mode (FR-P50–P53)

- **T-NA-040 [test]** — Topic-backend-unavailable integration test: force `bus.Publish` failures for 60 seconds during an ACTIVE session; assert the data-plane stream remains open and frames continue to flow. Trace: SC-P15.
- **T-NA-041** — Implement degraded-mode error handling: failed publishes surface as warnings, not stream-closure triggers. Trace: FR-P51.

### AdmissionPolicy interface (FR-P55–P57)

- **T-NA-050 [test]** — `AllowAll.Decide(...)` always returns `"allow-direct"`. Trace: SC-P16.
- **T-NA-051** — Implement `AdmissionPolicy` interface and `AllowAll` default in `internal/payment/admission.go`. Trace: FR-P55, FR-P56.
- **T-NA-052** — Add policy injection point to `internal/edgeapp/seller.go` constructor; default to `AllowAll` when not provided. Trace: FR-P55.
- **T-NA-053 [test]** — Constitution Principle XII compliance check: assert that `internal/payment/admission.go` exports only the interface and `AllowAll`; no priority list, allowlist, denylist, or domain-specific implementations live in this file. Trace: FR-P57, Principle XII.

### Phase 10 totals

- Test tasks: 12
- Implementation tasks: 17
- Total Phase 10: 29 tasks
- Cumulative project total: 47 (original) + 29 (Phase 10) = 76 tasks
