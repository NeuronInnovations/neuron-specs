# Contract: Validation Registry (IValidationRegistry)

**Feature**: 007-identity-contract
**Date**: 2026-03-04
**Type**: Standalone contract linked to Identity Registry

---

## Overview

The Validation Registry enables third-party validators to verify agent capabilities. Agents create validation requests addressed to specific validators; validators respond with pass/fail/pending status codes.

## Constructor

| Parameter | Type | Description |
|-----------|------|-------------|
| `identityRegistry` | `address` | Address of the linked Identity Registry contract |

## Interface

### Validation Operations

#### `validationRequest(address validatorAddress, uint256 agentId, string requestURI, bytes32 requestHash)`

Create a validation request addressed to a specific validator.

- **Access**: Owner of `agentId` NFT in Identity Registry only
- **Preconditions**: `agentId` exists in Identity Registry; `msg.sender` is `ownerOf(agentId)`; `validatorAddress` has an active Identity Registry registration; `requestHash` not already used
- **Effects**: Creates a new `ValidationRecord` with `response = 0` (pending)
- **Emits**: `ValidationRequested(requestHash, agentId, validatorAddress)`
- **Reverts**: `AgentNotRegistered(agentId)`, `NotAgentOwner(agentId, msg.sender)`, `ValidatorNotRegistered(validatorAddress)`, `RequestAlreadyExists(requestHash)`
- **FR**: FR-C-27, FR-C-32, FR-C-33, Spec 010 FR-V14

#### `validationResponse(bytes32 requestHash, uint8 response, string responseURI, bytes32 responseHash, bytes32 tag)`

Respond to a validation request.

- **Access**: Addressed validator only (`msg.sender` must match `validator` in the request); caller MUST have an active Identity Registry registration
- **Preconditions**: Request exists; caller is the addressed validator; caller is registered in Identity Registry; `response` is 1 (pass), 2 (fail), or 3 (inconclusive)
- **Effects**: Updates `response`, `responseURI`, `responseHash`, `tag`, and `lastUpdate` in the record
- **Emits**: `ValidationResponded(requestHash, response, tag)`
- **Reverts**: `NotAddressedValidator(requestHash, msg.sender)`, `ValidatorNotRegistered(msg.sender)`
- **FR**: FR-C-28, FR-C-29, FR-C-32, Spec 010 FR-V13

### Queries

#### `getValidationStatus(bytes32 requestHash) → (address validator, uint256 agentId, uint8 response, bytes32 responseHash, bytes32 tag, uint256 lastUpdate)`

- **Access**: Anyone (view)
- **Returns**: Complete validation record for the given request hash
- **FR**: FR-C-30

#### `getSummary(uint256 agentId, address[] validatorAddresses, bytes32 tag) → (uint256 count, uint256 passCount, uint256 failCount, uint256 inconclusiveCount)`

Return aggregated validation statistics.

- **Access**: Anyone (view)
- **Filters**:
  - `validatorAddresses`: Empty array = include all validators
  - `tag`: `bytes32(0)` = wildcard (no filter)
- **Returns**: Total count of responded validations, pass count, fail count, inconclusive count
- **FR**: FR-C-31, Spec 010 FR-V10

---

## Events

| Event | Signature | FR |
|-------|-----------|----|
| `ValidationRequested` | `ValidationRequested(bytes32 indexed requestHash, uint256 indexed agentId, address indexed validatorAddress)` | FR-C-32 |
| `ValidationResponded` | `ValidationResponded(bytes32 indexed requestHash, uint8 response, bytes32 tag)` | FR-C-32 |

## Custom Errors

| Error | Signature | FR |
|-------|-----------|----|
| `AgentNotRegistered` | `AgentNotRegistered(uint256 agentId)` | FR-C-33 |
| `NotAgentOwner` | `NotAgentOwner(uint256 agentId, address caller)` | FR-C-27 |
| `NotAddressedValidator` | `NotAddressedValidator(bytes32 requestHash, address caller)` | FR-C-28 |
| `ValidatorNotRegistered` | `ValidatorNotRegistered(address caller)` | FR-C-27, FR-C-28 (Spec 010 FR-V13/V14) |
| `RequestAlreadyExists` | `RequestAlreadyExists(bytes32 requestHash)` | FR-C-27 |

## Response Codes

| Code | Name | Description |
|------|------|-------------|
| 0 | `PENDING` | Initial state (set on request creation) |
| 1 | `PASS` | Validator confirms agent capability (maps to Spec 010 verdict `"compliant"`) |
| 2 | `FAIL` | Validator denies agent capability (maps to Spec 010 verdict `"non-compliant"`) |
| 3 | `INCONCLUSIVE` | Insufficient evidence to determine compliance (maps to Spec 010 verdict `"inconclusive"`) |
