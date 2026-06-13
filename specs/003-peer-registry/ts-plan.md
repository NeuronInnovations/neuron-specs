# Implementation Plan: Peer Registry (TypeScript)

**Date**: 2026-03-17 | **Spec**: [spec.md](spec.md)
**Language**: TypeScript | **Derivation Source**: Language-neutral artifacts ONLY

## Summary

Spec 003 defines EIP-8004 registration lifecycle for Child agents. Registration binds a Child's EVMAddress to an NFT with an agentURI containing mandatory topic services (stdIn/stdOut/stdErr) and p2p exchange services. The SDK provides registration, lookup, update, revocation, and agentURI validation — delegating on-chain operations to a contract interface (implemented in spec 007).

## Technical Context

**Dependencies**: `@neuron-sdk/keylib` (signing, EVMAddress, PeerID, DIDKey), `@neuron-sdk/account` (Child accounts), `@neuron-sdk/topic` (service schemas, TopicRef)
**Key TS Adaptations**:
- Contract interface is abstract — actual EVM interaction deferred to spec 007
- AgentURI validation (12 rules V-REG-01..12) is the core SDK logic
- Error codes: NEURON-REG-001..006 per 006 error-taxonomy.md

## Constitution Check

All 11 gates PASS. Note: spec 003 has C-4 deferred (interface stability) — non-blocking for TS SDK types and validation.

## Source Structure

```text
impl/typescript/src/registry/
├── index.ts              # Public exports
├── types.ts              # Registration, AgentURI, RegistrationResult, LookupKey
├── agent-uri.ts          # AgentURI construction, parsing, service extraction
├── validation.ts         # V-REG-01..12 validation rules
├── registration.ts       # Register, Update, Revoke, Lookup operations (abstract contract)
├── resolver.ts           # ResolveTopics, ResolveP2PExchange
├── contract.ts           # RegistryContract interface (abstract — impl in spec 007)
└── errors.ts             # RegistryError, NEURON-REG-001..006
```

## Phases

### Phase 1: Foundational — Types + Errors
RegistryError (6 codes), Registration type, AgentURI type, LookupKey discriminated union.

### Phase 2: US1 — AgentURI Validation (P1) 🎯 MVP
12 validation rules (V-REG-01..12), service extraction, cross-reference checking.

### Phase 3: US2 — Registration Operations (P2)
Register, UpdateRegistration, RevokeRegistration, LookupRegistration against abstract RegistryContract interface.

### Phase 4: US3 — Resolution (P3)
ResolveTopics, ResolveP2PExchange, topicRef cross-validation.
