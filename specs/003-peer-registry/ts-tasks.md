# Tasks: Peer Registry — TypeScript (Spec 003)

**Date**: 2026-03-17 | **Spec**: [spec.md](spec.md) | **Plan**: [ts-plan.md](ts-plan.md)

---

## Phase 1: Foundational

- [ ] T001 [P] Write tests + implement RegistryError with NEURON-REG-001..006 codes. File: `impl/typescript/src/registry/errors.ts`, `impl/typescript/tests/registry/error.test.ts` (FR-R07, 006 error-taxonomy.md)
- [ ] T002 [P] Write tests + implement types: Registration (registryAddress, childAddress, tokenId, agentURI, chainId), AgentURI (services array), RegistrationResult, LookupKey (ByAddress | ByExternalId). File: `impl/typescript/src/registry/types.ts`, `impl/typescript/tests/registry/types.test.ts` (FR-R01, FR-R05, FR-R08)

---

## Phase 2: US1 — AgentURI Validation (P1) 🎯 MVP

- [ ] T003 [P] [US1] Write failing tests for V-REG-01..12: exactly 3 neuron-topic (stdIn/stdOut/stdErr), ≥1 p2p-exchange, required fields, topicRef cross-reference, DID service validation, peerID match, channel uniqueness. File: `impl/typescript/tests/registry/validation.test.ts` (FR-R02, FR-R03, FR-R08, FR-R13, FR-R14)
- [ ] T004 [US1] Implement validateRegistrationCompleteness(agentURI, childPublicKey): applies all 12 V-REG rules, returns (valid, errors[]). File: `impl/typescript/src/registry/validation.ts` (FR-R02, FR-R03, FR-R08, FR-R13, FR-R14)
- [ ] T005 [US1] Implement AgentURI construction and parsing: buildAgentURI(topics, p2p, did?), parseAgentURI(json). File: `impl/typescript/src/registry/agent-uri.ts` (FR-R08)

---

## Phase 3: US2 — Registration Operations (P2)

- [ ] T006 [P] [US2] Write failing tests for registration ops: register(), updateRegistration(), revokeRegistration(), lookupRegistration() against mock RegistryContract. File: `impl/typescript/tests/registry/registration.test.ts` (FR-R06, FR-R10, FR-R11)
- [ ] T007 [US2] Implement RegistryContract interface (abstract) with register, updateAgentURI, revoke, lookup, ownerOf methods. File: `impl/typescript/src/registry/contract.ts` (FR-R01, FR-R06)
- [ ] T008 [US2] Implement registration operations delegating to RegistryContract: validation before call, proof-of-control check, result construction. File: `impl/typescript/src/registry/registration.ts` (FR-R06, FR-R10, FR-R11)

---

## Phase 4: US3 — Resolution (P3)

- [ ] T009 [P] [US3] Write failing tests for resolvers: resolveTopics extracts 3 channel TopicRefs, resolveP2PExchange validates topicRef cross-reference. File: `impl/typescript/tests/registry/resolver.test.ts` (FR-R02, FR-R03, 004 FR-T18)
- [ ] T010 [US3] Implement resolveTopics and resolveP2PExchange. File: `impl/typescript/src/registry/resolver.ts` (FR-R02, FR-R03)

---

## Phase 5: Polish

- [ ] T011 Create barrel exports. File: `impl/typescript/src/registry/index.ts`
- [ ] T012 Run all registry tests. All pass.

---

## Summary

| Metric | Value |
|--------|-------|
| Total tasks | 12 |
| FR coverage | 14/14 FR-R* traced |
| Validation rules | 12 V-REG rules |
