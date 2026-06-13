# Contract: Admission Policy (IAdmissionPolicy)

**Feature**: 007-identity-contract
**Date**: 2026-03-04
**Type**: Interface + Informative Implementations

---

## Overview

The `IAdmissionPolicy` interface defines the pluggable admission check for the Identity Registry. When the Identity Registry's admission policy address is set to a non-zero address, the registry calls `isAdmitted()` during `register()` to determine if the caller is permitted to register.

Setting the admission policy to `address(0)` bypasses admission entirely (permissionless mode).

## Interface

### `isAdmitted(address childAddress, bytes calldata parentDIDProof) → bool`

Determine if a Child address is admitted to register.

- **Parameters**:
  - `childAddress`: The `msg.sender` attempting to register (the Child's EVMAddress)
  - `parentDIDProof`: Opaque bytes — the proof format is defined by each policy implementation. For the permissionless `register(agentURI)` overload, this is empty bytes (`""`).
- **Returns**: `true` if admitted, `false` if rejected
- **Caller**: Identity Registry contract (during `register()`)
- **FR**: FR-C-14

## Informative Implementations

### PermissionlessPolicy

Always admits any address. Functionally equivalent to `address(0)` but as a deployed contract (useful for testing or explicit policy documentation).

```
isAdmitted(address, bytes) → true (always)
```

- **FR**: FR-C-12 (permissionless mode)

### AllowlistPolicy

Admits Children whose Parent DID hash is on the allowlist.

**State**:
- `owner`: Address authorized to manage the allowlist
- `allowedParentDIDs`: `mapping(bytes32 => bool)` — hash of Parent DID → admitted

**Functions**:
- `addParentDID(bytes32 parentDIDHash)` — Owner-only. Adds a Parent DID to the allowlist.
- `removeParentDID(bytes32 parentDIDHash)` — Owner-only. Removes a Parent DID from the allowlist. Does NOT affect existing registrations (FR-C-15).
- `isAdmitted(address, bytes parentDIDProof) → bool` — Returns `allowedParentDIDs[keccak256(parentDIDProof)]`.

**Semantics**:
- The `parentDIDProof` is the raw DID bytes (e.g., the `did:key:zQ3s...` string). The policy hashes it with `keccak256` and checks the allowlist.
- One allowlist entry covers all current and future Children of that Parent (per 003 FR-R12a).
- Removing a Parent DID only blocks NEW registrations — existing registrations persist (FR-C-15, per 003 FR-R12d).

**FR**: FR-C-12, FR-C-14, FR-C-15
