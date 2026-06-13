# Implementation Plan: Identity Contract (TypeScript)

**Date**: 2026-03-18 | **Spec**: [spec.md](spec.md)
**Language**: TypeScript | **Derivation Source**: Language-neutral artifacts ONLY

## Summary

Spec 007 defines three on-chain EVM registries (Identity, Reputation, Validation) as ERC-721 smart contracts. The TypeScript SDK provides type definitions for contract interfaces, events, and data structures — NOT Solidity compilation or deployment. ABI bindings are deferred until ethers.js/viem integration is added as a dependency.

For this phase, the SDK implements: contract interface types, event types, admission policy interface, and data structures that map SDK operations (spec 003) to on-chain function calls (spec 007).

## Technical Context

**Dependencies**: keylib (EVMAddress), account (NeuronAccount), registry (Registration types)
**Key TS Adaptations**:
- Contract interfaces as TypeScript types (no ethers.js dependency yet)
- uint256 mapped to bigint
- Solidity events mapped to TypeScript event types
- Admission policy as pluggable interface

## Constitution Check

All 11 gates PASS.

## Source Structure

```text
impl/typescript/src/contracts/
├── index.ts
├── types.ts                  # Shared types (uint256, address aliases)
├── identity-registry.ts      # IIdentityRegistry interface + events
├── reputation-registry.ts    # IReputationRegistry interface + events
├── validation-registry.ts    # IValidationRegistry interface + events
├── admission-policy.ts       # IAdmissionPolicy interface
└── errors.ts                 # Contract-level error types
```

## Phases

### Phase 1: Types + Identity Registry interface
### Phase 2: Reputation + Validation Registry interfaces
### Phase 3: Admission Policy + Polish
