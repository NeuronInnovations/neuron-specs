# Contract: Reputation Registry (IReputationRegistry)

**Feature**: 007-identity-contract
**Date**: 2026-03-04
**Type**: Standalone contract linked to Identity Registry

---

## Overview

The Reputation Registry records feedback from clients about registered agents. It uses fixed-point arithmetic for rating values, supports categorical tags for filtering, and provides off-chain detail URIs with Keccak256 integrity hashes.

## Constructor

| Parameter | Type | Description |
|-----------|------|-------------|
| `identityRegistry` | `address` | Address of the linked Identity Registry contract |

## Interface

### Feedback Operations

#### `giveFeedback(uint256 agentId, int128 value, uint8 decimals, bytes32 tag1, bytes32 tag2, string feedbackURI, bytes32 feedbackHash) → uint256 feedbackIndex`

Record feedback about a registered agent.

- **Access**: Anyone (`msg.sender` becomes the client)
- **Preconditions**: `agentId` exists in Identity Registry; `decimals <= 18`
- **Effects**: Creates a new `FeedbackEntry` at the next `feedbackIndex` for `(agentId, msg.sender)`
- **Emits**: `FeedbackGiven(agentId, msg.sender, feedbackIndex, value, decimals, tag1, tag2)`
- **Reverts**: `AgentNotRegistered(agentId)`
- **FR**: FR-C-20, FR-C-21, FR-C-25, FR-C-26

#### `revokeFeedback(uint256 agentId, uint256 feedbackIndex)`

Revoke previously given feedback.

- **Access**: Original feedback giver only (`msg.sender` must match recorded client)
- **Preconditions**: Feedback entry exists and is not already revoked
- **Effects**: Sets `revoked = true` on the feedback entry
- **Emits**: `FeedbackRevoked(agentId, msg.sender, feedbackIndex)`
- **Reverts**: `NotFeedbackGiver(agentId, feedbackIndex, msg.sender)`
- **FR**: FR-C-22, FR-C-25

#### `appendResponse(uint256 agentId, address clientAddress, uint256 feedbackIndex, string responseURI, bytes32 responseHash)`

Agent owner responds to feedback.

- **Access**: Owner of `agentId` NFT in Identity Registry only
- **Preconditions**: `agentId` exists; `msg.sender` is `ownerOf(agentId)`; feedback entry exists
- **Effects**: Sets `responseURI` and `responseHash` on the feedback entry
- **Emits**: `ResponseAppended(agentId, clientAddress, feedbackIndex)`
- **Reverts**: `NotAgentOwner(agentId, msg.sender)`
- **FR**: FR-C-23, FR-C-25

### Queries

#### `getSummary(uint256 agentId, address[] clientAddresses, bytes32 tag1, bytes32 tag2) → (uint256 count, int256 totalValue, uint8 decimals)`

Return aggregated feedback statistics.

- **Access**: Anyone (view)
- **Filters**:
  - `clientAddresses`: Empty array = include all clients
  - `tag1`, `tag2`: `bytes32(0)` = wildcard (no filter on that dimension)
- **Returns**: Count of non-revoked matching feedback, total value sum, common decimals
- **FR**: FR-C-24

---

## Events

| Event | Signature | FR |
|-------|-----------|----|
| `FeedbackGiven` | `FeedbackGiven(uint256 indexed agentId, address indexed client, uint256 feedbackIndex, int128 value, uint8 decimals, bytes32 tag1, bytes32 tag2)` | FR-C-25 |
| `FeedbackRevoked` | `FeedbackRevoked(uint256 indexed agentId, address indexed client, uint256 feedbackIndex)` | FR-C-25 |
| `ResponseAppended` | `ResponseAppended(uint256 indexed agentId, address indexed clientAddress, uint256 feedbackIndex)` | FR-C-25 |

## Custom Errors

| Error | Signature | FR |
|-------|-----------|----|
| `AgentNotRegistered` | `AgentNotRegistered(uint256 agentId)` | FR-C-26 |
| `NotFeedbackGiver` | `NotFeedbackGiver(uint256 agentId, uint256 feedbackIndex, address caller)` | FR-C-22 |
| `NotAgentOwner` | `NotAgentOwner(uint256 agentId, address caller)` | FR-C-23 |
