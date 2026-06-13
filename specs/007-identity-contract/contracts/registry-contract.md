# Contract: Identity Registry (IIdentityRegistry)

**Feature**: 007-identity-contract
**Date**: 2026-03-04
**Type**: ERC-721 + ERC721Enumerable + Custom Extensions

---

## Overview

The Identity Registry is the primary on-chain contract for the Neuron protocol. It maps a Child's EVMAddress to a registration NFT containing an `agentURI` (opaque string pointing to or embedding a JSON services document).

## Interface

### Registration

#### `register(string agentURI) → uint256 tokenId`

Mint a new registration token to `msg.sender` in permissionless mode.

- **Access**: Anyone (subject to admission policy)
- **Preconditions**: `msg.sender` not already registered; `agentURI` not empty; admission policy (if set) called with empty proof
- **Effects**: Mints token, stores agentURI, sets reverse mapping
- **Emits**: `IdentityRegistered(tokenId, owner, agentURI)`
- **Reverts**: `AlreadyRegistered(msg.sender)`, `EmptyAgentURI()`, `AdmissionDenied(msg.sender)`
- **FR**: FR-C-02, FR-C-04, FR-C-06, FR-C-12, FR-C-18, FR-C-19

#### `register(string agentURI, bytes parentDIDProof) → uint256 tokenId`

Mint a new registration token with admission proof for permissioned registries.

- **Access**: Anyone (subject to admission policy)
- **Preconditions**: Same as above; admission policy called with `parentDIDProof`
- **Effects**: Same as above
- **Emits**: `IdentityRegistered(tokenId, owner, agentURI)`
- **Reverts**: Same as above
- **FR**: FR-C-02, FR-C-04, FR-C-06, FR-C-12, FR-C-14, FR-C-18, FR-C-19

#### `updateAgentURI(uint256 tokenId, string newAgentURI)`

Update the agentURI for an existing registration.

- **Access**: Token owner or approved operator
- **Preconditions**: Token exists; caller is owner or approved; `newAgentURI` not empty
- **Effects**: Updates stored agentURI
- **Emits**: `IdentityUpdated(tokenId, newAgentURI)`
- **Reverts**: `NotOwnerOrApproved(tokenId, msg.sender)`, `EmptyAgentURI()`
- **FR**: FR-C-05, FR-C-07, FR-C-18, FR-C-19

#### `revoke(uint256 tokenId)`

Burn the registration token and clear the reverse mapping.

- **Access**: Token owner only (NOT operators, NOT admin)
- **Preconditions**: Token exists; caller is owner
- **Effects**: Burns token, clears `_addressToTokenId[owner]`
- **Emits**: `IdentityRevoked(tokenId, owner)`
- **Reverts**: `NotTokenOwner(tokenId, msg.sender)`
- **FR**: FR-C-05, FR-C-08, FR-C-16, FR-C-18

### Queries

#### `agentURI(uint256 tokenId) → string`

- **Access**: Anyone (view)
- **Returns**: The stored agentURI for the given tokenId
- **Reverts**: If token does not exist
- **FR**: FR-C-03

#### `lookup(address account) → (uint256 tokenId, string agentURI)`

- **Access**: Anyone (view)
- **Returns**: `(tokenId, agentURI)` if registered; `(0, "")` if not
- **FR**: FR-C-10

### Admin

#### `setAdmissionPolicy(address newPolicy)`

- **Access**: Contract owner only
- **Effects**: Updates admission policy address
- **Emits**: `AdmissionPolicyUpdated(oldPolicy, newPolicy)`
- **FR**: FR-C-13

#### `admissionPolicy() → address`

- **Access**: Anyone (view)
- **Returns**: Current admission policy address (`address(0)` = permissionless)
- **FR**: FR-C-12

### Inherited (ERC-721)

Standard ERC-721 functions: `ownerOf`, `balanceOf`, `transferFrom`, `safeTransferFrom`, `approve`, `setApprovalForAll`, `getApproved`, `isApprovedForAll`.

Standard ERC721Enumerable functions: `totalSupply`, `tokenByIndex`, `tokenOfOwnerByIndex`.

Standard ERC-165: `supportsInterface` (FR-C-36).

---

## Events

| Event | Signature | FR |
|-------|-----------|----|
| `IdentityRegistered` | `IdentityRegistered(uint256 indexed tokenId, address indexed owner, string agentURI)` | FR-C-04 |
| `IdentityUpdated` | `IdentityUpdated(uint256 indexed tokenId, string newAgentURI)` | FR-C-05 |
| `IdentityRevoked` | `IdentityRevoked(uint256 indexed tokenId, address indexed owner)` | FR-C-05 |
| `AdmissionPolicyUpdated` | `AdmissionPolicyUpdated(address indexed oldPolicy, address indexed newPolicy)` | FR-C-13 |

## Custom Errors

| Error | Signature | FR |
|-------|-----------|----|
| `AlreadyRegistered` | `AlreadyRegistered(address account)` | FR-C-06 |
| `NotOwnerOrApproved` | `NotOwnerOrApproved(uint256 tokenId, address caller)` | FR-C-07 |
| `NotTokenOwner` | `NotTokenOwner(uint256 tokenId, address caller)` | FR-C-08 |
| `EmptyAgentURI` | `EmptyAgentURI()` | FR-C-19 |
| `AdmissionDenied` | `AdmissionDenied(address account)` | FR-C-12 |
