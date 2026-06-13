# Tasks: Identity Contract — TypeScript (Spec 007)

**Date**: 2026-03-18 | **Spec**: [spec.md](spec.md) | **Plan**: [ts-plan.md](ts-plan.md)

---

## Phase 1: Identity Registry

- [ ] T001 [P] Write tests + implement contract types: uint256 as bigint, address as string, FeedbackEntry, ValidationRecord. Files: `src/contracts/types.ts`, `tests/contracts/types.test.ts` (FR-C-01)
- [ ] T002 [P] Write tests + implement IIdentityRegistry interface: register(agentURI) → tokenId, updateAgentURI(tokenId, newURI), revoke(tokenId), agentURI(tokenId), lookup(address) → (tokenId, agentURI), setAdmissionPolicy(address), admissionPolicy(). Events: IdentityRegistered, IdentityUpdated, IdentityRevoked, AdmissionPolicyUpdated. Files: `src/contracts/identity-registry.ts`, `tests/contracts/identity-registry.test.ts` (FR-C-02..FR-C-13)

---

## Phase 2: Reputation + Validation Registries

- [ ] T003 [P] Write tests + implement IReputationRegistry interface: giveFeedback(agentId, value, decimals, tag1, tag2, feedbackURI, feedbackHash), revokeFeedback(agentId, feedbackIndex), appendResponse(agentId, feedbackIndex, responseURI, responseHash), getSummary(agentId, tags). Events: FeedbackGiven, FeedbackRevoked, ResponseAppended. Files: `src/contracts/reputation-registry.ts`, `tests/contracts/reputation-registry.test.ts` (FR-C-20..FR-C-25)
- [ ] T004 [P] Write tests + implement IValidationRegistry interface: validationRequest(agentId, validator, requestURI, tag), validationResponse(requestHash, response, responseURI, responseHash, tag), getValidationStatus(requestHash), getSummary(agentId, tags). Events: ValidationRequested, ValidationResponded. Files: `src/contracts/validation-registry.ts`, `tests/contracts/validation-registry.test.ts` (FR-C-27..FR-C-32)

---

## Phase 3: Admission Policy + Polish

- [ ] T005 Write tests + implement IAdmissionPolicy interface: isAdmitted(childAddress, parentDIDProof) → boolean. AllowlistPolicy type. Files: `src/contracts/admission-policy.ts`, `tests/contracts/admission-policy.test.ts` (FR-C-13, FR-DD-02)
- [ ] T006 Create barrel exports. File: `src/contracts/index.ts`
- [ ] T007 Run all contract tests. All pass.

---

## Summary

| Metric | Value |
|--------|-------|
| Total tasks | 7 |
| FR coverage | FR-C-01..FR-C-32 (3 registries) |
| Note | Type definitions only — no ethers.js dependency. ABI bindings in future phase. |
