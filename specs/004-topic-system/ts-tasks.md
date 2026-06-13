# Tasks: Topic System — TypeScript (Spec 004)

**Date**: 2026-03-17 | **Spec**: [spec.md](spec.md) | **Plan**: [ts-plan.md](ts-plan.md)

---

## Phase 1: Foundational

### Tests (Red)

- [ ] T001 [P] Write failing tests for TopicError: all 10 NEURON-TOPIC-* codes, factory functions, descriptive messages. File: `impl/typescript/tests/topic/error.test.ts` (FR-T11, 006 error-taxonomy.md)
- [ ] T002 [P] Write failing tests for enums/types: BackendKind (hcs, erc-log, kafka), ChannelRole (stdIn, stdOut, stdErr, custom:*), ConfirmationMode (FIRE_AND_FORGET, WAIT_FOR_CONSENSUS), reserved channel name validation. File: `impl/typescript/tests/topic/types.test.ts` (FR-T05, FR-T07, FR-T08, FR-T23)

### Implementation (Green)

- [ ] T003 [P] Implement TopicError extending NeuronError with 10 NEURON-TOPIC-* codes. File: `impl/typescript/src/topic/errors.ts` (FR-T11)
- [ ] T004 [P] Implement enums: BackendKind, ChannelRole with validation (reserved names, custom: prefix), ConfirmationMode. File: `impl/typescript/src/topic/types.ts` (FR-T05, FR-T07, FR-T08, FR-T23)

**Checkpoint**: Error and type tests pass.

---

## Phase 2: US1 — TopicMessage Core (Priority: P1) 🎯 MVP

**Goal**: Create, sign, serialize, and verify TopicMessage envelopes.
**Conformance Gate**: Chain 2 test vectors MUST pass.

### Tests (Red)

- [ ] T005 [P] [US1] Write failing tests for TopicMessage: static `buildPreimage(timestamp, sequenceNumber, payload)` returns correct byte concatenation per 006 §8 (ts_be8 || seq_be8 || payload). Static `hashPreimage(ts, seq, payload)` returns correct Keccak256 hash. Verify against Chain 2 test vector hex values. File: `impl/typescript/tests/topic/message.test.ts` (FR-T03, FR-T21, FR-A08)
- [ ] T006 [P] [US1] Write failing tests for TopicMessage.create(key, timestamp, sequenceNumber, payload): creates signed message, signatureBytes() matches Chain 2 hex, signatureBase64() matches Chain 2 base64, payloadBase64() matches Chain 2 base64, toCanonicalJson() matches Chain 2 canonical JSON byte-for-byte (field order: senderAddress→signature→timestamp→sequenceNumber→payload). File: `impl/typescript/tests/topic/message.test.ts` (append) (FR-T02, FR-T03, FR-T21, FR-W01, FR-W02, FR-W03, FR-W05, FR-W06)
- [ ] T007 [P] [US1] Write failing tests for TopicMessage.verify(message): valid message returns true, tampered payload returns false, wrong sender returns SenderMismatch error, malformed signature returns InvalidSignature error. File: `impl/typescript/tests/topic/message.test.ts` (append) (FR-T10, SC-T03, SC-T06)

### Implementation (Green)

- [ ] T008 [US1] Implement TopicMessage class: fields (senderAddress: string EIP-55, signatureBytes: Uint8Array, timestamp: bigint, sequenceNumber: bigint, payload: Uint8Array). Static `buildPreimage(ts, seq, payload)`: uint64ToBytesBE(ts) || uint64ToBytesBE(seq) || payload per 006 §8. Static `hashPreimage(ts, seq, payload)`: keccak_256(buildPreimage(...)). Static `create(key, ts, seq, payload)`: hash preimage → key.signHash(hash) → construct message with key.publicKey().evmAddress().toString() as senderAddress. `signatureBytes()`, `signatureBase64()`, `payloadBase64()`, `toCanonicalJson()` using serializeCanonicalJson with field order [senderAddress, signature, timestamp, sequenceNumber, payload]. Static `verify(message)`: recover public key from signature + hash → derive EVMAddress → constant-time compare with senderAddress. File: `impl/typescript/src/topic/message.ts` (FR-T02, FR-T03, FR-T10, FR-T21, FR-A07, FR-A08, FR-A10)

#### Conformance Verification

- [ ] T009 [US1] Run Chain 2 conformance tests: `npx vitest run tests/conformance/chain2-topic-signing.test.ts`. All intermediate hex values MUST match. File: `tests/conformance/chain2-topic-signing.test.ts` (FR-A07, FR-A08, FR-A10)

**Checkpoint**: Chain 2 golden test vectors pass. TopicMessage signing, serialization, and verification work.

---

## Phase 3: US2 — TopicRef + Adapter Interface (Priority: P2)

**Goal**: TopicRef type, TopicAdapter interface, adapter registry.

### Tests (Red)

- [ ] T010 [P] [US2] Write failing tests for TopicRef: construction from (transport, locator), validation (transport registered, locator non-empty), URI parsing (hcs://0.0.123, erc-log://1:0xABC, kafka+ledger://broker/topic), equality check. File: `impl/typescript/tests/topic/topic-ref.test.ts` (FR-T01, FR-T12, SC-T01)
- [ ] T011 [P] [US2] Write failing tests for TopicAdapter interface: interface has required methods (createTopic, publish, subscribe, resolve, maxMessageSize, supportedTransport), PublishResult type fields, MessageDelivery type fields. File: `impl/typescript/tests/topic/adapter.test.ts` (FR-T04, FR-T22, FR-T24)

### Implementation (Green)

- [ ] T012 [US2] Implement TopicRef class: immutable (transport: BackendKind, locator: string), validation per FR-T12, URI parsing/serialization, fromService(NeuronTopicService) extraction. File: `impl/typescript/src/topic/topic-ref.ts` (FR-T01, FR-T12)
- [ ] T013 [US2] Implement TopicAdapter interface, adapter registry (Map<BackendKind, TopicAdapter>), PublishResult type, MessageDelivery type, SubscribeOpts/PublishOpts/CreateTopicOpts types. File: `impl/typescript/src/topic/adapter.ts`, `impl/typescript/src/topic/publish-result.ts`, `impl/typescript/src/topic/message-delivery.ts` (FR-T04, FR-T05, FR-T22, FR-T24, FR-T25)

**Checkpoint**: TopicRef validates and parses. Adapter interface defined.

---

## Phase 4: US3 — Service Schemas (Priority: P3)

**Goal**: Parse NeuronTopicService and NeuronP2PExchangeService from agentURI JSON.

### Tests (Red)

- [ ] T014 [P] [US3] Write failing tests for service parsing: parseAgentURIServices extracts topic and p2p services from JSON, validates required fields, rejects malformed, round-trip serialize/parse produces identical JSON. Cross-reference validation: p2p topicRef resolves to existing topic service name. File: `impl/typescript/tests/topic/service.test.ts` (FR-T09, FR-T14, FR-T17, FR-T18, SC-T09, SC-T10)

### Implementation (Green)

- [ ] T015 [US3] Implement service parsing: NeuronTopicService and NeuronP2PExchangeService types, parseAgentURIServices(), validateCrossReferences(), extractTopicRef(), serializeTopicService(). Transport config discriminated union (HCSConfig, ERCLogConfig, KafkaConfig). File: `impl/typescript/src/topic/service.ts` (FR-T09, FR-T14, FR-T15, FR-T16, FR-T17, FR-T18)

**Checkpoint**: Service parsing and cross-reference validation work.

---

## Phase 5: Polish

- [ ] T016 Create public exports barrel file. File: `impl/typescript/src/topic/index.ts`
- [ ] T017 Run all topic tests: `npx vitest run tests/topic/`. All pass. (SC-T01..SC-T15)

---

## Summary

| Metric | Value |
|--------|-------|
| Total tasks | 17 |
| FR coverage | 29/29 FR-T* traced |
| Conformance gates | Chain 2 (TopicMessage signing) |
| Parallel opportunities | 8 tasks marked [P] |
