# Architecture Quality Checklist: Spec 008 — Payment

**Purpose**: Validate spec quality against 7 architecture-level concerns before proceeding to `/speckit.plan`
**Created**: 2026-03-19
**Feature**: [specs/008-payment/spec.md](../spec.md)
**Focus**: Chain-agnostic boundaries, binding separation, normative wording, determinism, error handling, out-of-scope discipline, multi-language SDK suitability

---

## Chain-Agnostic Protocol Boundaries

- [ ] CHK001 — Are all protocol-layer requirements (Sections A–C, E, F, G) free of Hedera-specific primitives (SharedAccount, ScheduleCreate, MultisigKey, HBAR, CryptoTransfer)? [Consistency, Spec §FR-P01–P15, §FR-P24–P31]
- [ ] CHK002 — Are all protocol-layer requirements free of EVM-specific primitives (ERC-20, Permit2, SafeERC20, Solidity, block.timestamp, msg.sender)? [Consistency, Spec §FR-P01–P15, §FR-P24–P31]
- [ ] CHK003 — Is the `settlement` object in the `neuron-commerce` schema defined as a generic descriptor with `binding` field, rather than assuming a specific chain's data shape? [Clarity, Spec §FR-P02]
- [ ] CHK004 — Are all escrow operation parameters (`agreementHash`, `evidenceHash`, `timeout`, `currency`, `threshold`) defined in chain-neutral types (bytes32, uint64, string), not chain-specific types? [Consistency, Spec §FR-P16–P23]
- [ ] CHK005 — Is `EscrowRef.locator` defined as an opaque string with no protocol-level parsing rules, leaving format to bindings? [Clarity, Spec §FR-P23]
- [ ] CHK006 — Does the agreement lifecycle state machine (FR-P13/P14) reference only TopicMessage payloads and abstract escrow operations — never binding-specific transactions? [Consistency, Spec §FR-P13–P14]

## Hedera vs EVM Binding Separation

- [ ] CHK007 — Are binding-specific details (SharedAccount, ScheduleCreate/Sign, threshold keys, CryptoTransfer) confined exclusively to a clearly labeled Hedera binding section or the architecture recommendation — not in the protocol-layer FRs? [Completeness, Gap]
- [ ] CHK008 — Are binding-specific details (NeuronEscrow.sol, ERC-20, Permit2, SafeERC20, pull payments, EIP-712 domain, withdraw()) confined exclusively to a clearly labeled EVM binding section or the architecture recommendation — not in the protocol-layer FRs? [Completeness, Gap]
- [ ] CHK009 — Does FR-P18 (deposit) explicitly state that deposit authorization method is binding-specific without naming any specific method in the protocol layer? [Clarity, Spec §FR-P18]
- [ ] CHK010 — Does FR-P21 (approveRelease) explicitly state that push vs pull fund transfer is binding-internal, and that `withdraw` is NOT an abstract operation? [Clarity, Spec §FR-P21]
- [ ] CHK011 — Is the race condition edge case (simultaneous buyer/seller operations) described without assuming either EVM or Hedera semantics as the default? [Consistency, Spec §Edge Cases]

## Normative vs Binding-Specific Wording

- [ ] CHK012 — Does every FR use RFC 2119 language (MUST, SHOULD, MAY) consistently, with MUST for protocol-level invariants and SHOULD for SDK-level recommendations? [Clarity, Spec §FR-P01–P32]
- [ ] CHK013 — Are trust gating checks (FR-P30/P31) explicitly marked as SHOULD-level, not MUST? [Clarity, Spec §FR-P30–P31]
- [ ] CHK014 — Is `termsRef` explicitly marked as SHOULD (not MUST)? [Clarity, Spec §FR-P04]
- [ ] CHK015 — Are balance validation rules using appropriate normative levels — MUST for zero-balance refusal (FR-P27), SHOULD for stop-delivery and notification (FR-P25/P26)? [Consistency, Spec §FR-P24–P27]
- [ ] CHK016 — Are binding-specific details in the architecture recommendation clearly labeled as non-normative design guidance, not protocol requirements? [Clarity, Gap]
- [ ] CHK017 — Does the spec avoid using "the SDK" or "the implementation" as a subject in MUST-level requirements, keeping language implementation-neutral? [Consistency, Spec §FR-P07a]

## Determinism / Validator Observability

- [x] CHK018 — Is canonical JSON field ordering specified for ALL five negotiation payload types, not just referenced generically? [Completeness, Spec §FR-P12] ✅ Fixed: field orders now listed inline in FR-P12
- [ ] CHK019 — Does FR-P12 explicitly reference 006 FR-W05 (canonical field order), FR-W02 (UnsignedInt64 as string decimal), and FR-W04 (omit absent optional fields)? [Traceability, Spec §FR-P12]
- [ ] CHK020 — Is the `agreementHash` computation formula specified unambiguously: `keccak256(canonicalJSON(acceptedServiceResponse))`? [Clarity, Spec §FR-P17, §SC-P07]
- [ ] CHK021 — Is the `evidenceHash` computation formula specified unambiguously: `keccak256(canonicalJSON(deliveryProofTopicMessage))`? [Clarity, Spec §FR-P20, §SC-P08]
- [ ] CHK022 — Does each VR-PAY rule (VR-PAY-01 through VR-PAY-06) specify: what to observe, where to observe it, pass condition, and fail condition? [Completeness, Spec §VR-PAY-01–06]
- [ ] CHK023 — Are the Observable State Commitments sufficient for all VR-PAY rules — i.e., does every validator check have a corresponding published artifact (topic message or on-chain state)? [Coverage, Spec §Observable State Commitments]
- [ ] CHK024 — Is the dual verification tier (`topic-observable` for protocol, `on-chain-only` for bindings) explicitly documented with justification? [Clarity, Spec §Verification Tier]
- [ ] CHK025 — Does the version compatibility rule (FR-P12a) explicitly define behavior for unknown fields in minor versions (ignore) to ensure deterministic parsing across SDK versions? [Clarity, Spec §FR-P12a]

## Error Handling Completeness

- [ ] CHK026 — Does FR-P32 enumerate at least one error kind for each escrow operation (createEscrow, deposit, getBalance, requestRelease, approveRelease, claimRefund)? [Coverage, Spec §FR-P32]
- [x] CHK027 — Is there an error kind for version mismatch (receiver gets a `version: "2.0.0"` payload)? [Gap, Spec §FR-P12a, §FR-P32] ✅ Fixed: `VersionMismatch` added to FR-P32 with FR-P12a cross-ref
- [x] CHK028 — Is there an error kind for negotiation deadline expiry? [Gap, Spec §FR-P07a, §FR-P32] ✅ Fixed: `NegotiationExpired` added to FR-P32 with FR-P07a cross-ref
- [ ] CHK029 — Is there an error kind for binding mismatch (buyer requests a binding the seller doesn't support)? [Coverage, Spec §FR-P32 — `BindingUnavailable` exists]
- [ ] CHK030 — Does the error taxonomy follow the `NEURON-PAYMENT-{NNN}` format with explicit numeric codes, or is it name-only? [Clarity, Spec §FR-P32]
- [ ] CHK031 — Are error kinds mapped to specific FRs (e.g., `RefundNotEligible` → FR-P22, `InsufficientBalance` → FR-P24)? [Traceability, Spec §FR-P32]

## Out-of-Scope Discipline

- [ ] CHK032 — Does the Out of Scope section explicitly list all deferred payment models (x402, streaming, ERC-8183) without suggesting partial support? [Completeness, Spec §Out of Scope]
- [ ] CHK033 — Does the spec avoid defining SLA content, format, or validation — limiting itself to the `termsRef` URI pointer? [Consistency, Spec §FR-P04, §Out of Scope]
- [ ] CHK034 — Does the spec avoid specifying revenue split ratios, delegating this to product configuration? [Consistency, Spec §FR-P29, §Out of Scope]
- [ ] CHK035 — Does the spec avoid on-chain agentURI parsing, consistent with 007 DD-01? [Consistency, Spec §Out of Scope]
- [ ] CHK036 — Are dispute resolution mechanisms limited to `claimRefund` (timeout-based) and arbiter intervention (2-of-3 threshold), without introducing new dispute resolution frameworks? [Consistency, Spec §Out of Scope, §FR-P22]
- [ ] CHK037 — Does the spec avoid marketplace/discovery UI concerns, limiting itself to machine-readable `neuron-commerce` schema? [Consistency, Spec §Out of Scope]

## Multi-Language SDK Generation Suitability

- [ ] CHK038 — Are all data types in the escrow interface (EscrowRef, DepositResult, Balance, ReleaseRequestRef, ReleaseResult, RefundResult) defined with language-neutral field types (string, uint64, bytes32, timestamp), not language-specific types? [Clarity, Spec §FR-P16–P23]
- [ ] CHK039 — Are payload field types compatible with 006's language-neutral type mapping (string for UnsignedInt64, string for BigInteger, string for EVMAddress)? [Consistency, Spec §FR-P07–P11, 006 FR-X02]
- [ ] CHK040 — Is the agreement state machine (FR-P13/P14) defined as a pure function of message inputs — deterministic without runtime-specific state? [Clarity, Spec §FR-P13–P14, §SC-P04]
- [ ] CHK041 — Are all enum values (AgreementState states, negotiation actions, invoiceAck actions) defined as string constants, not numeric codes? [Clarity, Spec §FR-P13, §FR-P08, §FR-P11]
- [ ] CHK042 — Does the spec avoid Go-specific types, runtime details, or framework references per Constitution Principle VI? [Consistency, Spec §all sections]
- [ ] CHK043 — Is the `EscrowAdapter` interface defined with operation signatures that map cleanly to method/function signatures in Go, TypeScript, and Rust (no language-specific generics or trait assumptions)? [Clarity, Spec §FR-P16]
- [x] CHK044 — Are canonical JSON field orderings for all five payload types explicitly listed (not just referenced), so any language can implement a custom serializer? [Completeness, Spec §FR-P12] ✅ Fixed: field orders now listed inline in FR-P12
