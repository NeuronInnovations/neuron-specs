# Quickstart: Identity Registry Smart Contract (EIP-8004)

**Feature**: 007-identity-contract
**Date**: 2026-03-04
**Spec**: [spec.md](spec.md)

---

## Developer Prerequisites

Before working with the Identity Registry contracts, ensure you have:

1. **An EVM-compatible development environment** — Foundry (forge, cast, anvil), Hardhat, or Remix for Solidity development and testing.
2. **A Child account with an EVMAddress** — Derived from a secp256k1 private key (see Spec 002 Key Library). The Child's address will be `msg.sender` during registration.
3. **A funded account on the target chain** — ETH (Ethereum/Base) or HBAR (Hedera EVM) for gas fees.
4. **An agentURI** — A valid URI string pointing to or embedding the agent's JSON services document (see Spec 003 for agentURI schema). The contract stores this as an opaque string.
5. **(Optional) A Parent DID proof** — Required only for permissioned registries with an `AllowlistPolicy`. The proof format is defined by the admission policy implementation.

---

## Scenario 1: Register a Child Identity (US1 — P1)

### Step 1: Deploy the Identity Registry

Deploy the Identity Registry contract (ERC-721 + ERC721Enumerable). Set the initial admission policy to `address(0)` for permissionless registration.

### Step 2: Register

The Child calls `register(agentURI)` with its agentURI string. The contract:
- Checks `msg.sender` is not already registered
- Checks agentURI is not empty
- Checks admission policy (skipped when `address(0)`)
- Mints a new ERC-721 token to `msg.sender`
- Stores the agentURI
- Emits `IdentityRegistered(tokenId, owner, agentURI)`

**Verify**: Call `lookup(childAddress)` — returns `(tokenId, agentURI)`.

### Step 3: Update agentURI

The Child (or an approved operator) calls `updateAgentURI(tokenId, newAgentURI)`.

**Verify**: Call `agentURI(tokenId)` — returns the updated URI. Event `IdentityUpdated` emitted.

### Step 4: Revoke

The Child (owner only, not operators) calls `revoke(tokenId)`. The token is burned and the address mapping is cleared.

**Verify**: `lookup(childAddress)` returns `(0, "")`. The Child can call `register()` again to get a new tokenId.

---

## Scenario 2: Permissioned Registration with Allowlist (US2 — P2)

### Step 1: Deploy AllowlistPolicy

Deploy an `AllowlistPolicy` contract. The policy owner adds Parent DID hashes to the allowlist via `addParentDID(keccak256(parentDIDBytes))`.

### Step 2: Set Admission Policy

The registry owner calls `setAdmissionPolicy(allowlistPolicyAddress)`. The event `AdmissionPolicyUpdated` is emitted.

### Step 3: Register with Proof

The Child calls `register(agentURI, parentDIDProof)` where `parentDIDProof` is the raw Parent DID bytes. The contract calls `admissionPolicy.isAdmitted(msg.sender, parentDIDProof)`. If the Parent DID hash is in the allowlist, registration proceeds.

### Step 4: Verify Allowlist Removal Semantics

Remove a Parent DID from the allowlist. Existing registrations by that Parent's Children are NOT auto-revoked — only new registration attempts are rejected.

---

## Scenario 3: Give Reputation Feedback (US3 — P2)

### Step 1: Deploy Reputation Registry

Deploy the Reputation Registry with the Identity Registry address as a constructor parameter.

### Step 2: Give Feedback

A client calls `giveFeedback(agentId, 450, 2, "quality", "speed", feedbackURI, feedbackHash)`. This records a rating of 4.50 (450 / 10^2) with tags "quality" and "speed".

**Verify**: `FeedbackGiven` event emitted with correct values. Call `getSummary(agentId, [], "quality", bytes32(0))` to see aggregated results filtered by "quality" tag.

### Step 3: Agent Responds

The agent owner calls `appendResponse(agentId, clientAddress, feedbackIndex, responseURI, responseHash)`.

### Step 4: Revoke Feedback

The original client calls `revokeFeedback(agentId, feedbackIndex)`. Revoked feedback is excluded from future `getSummary()` results.

---

## Scenario 4: Request Validation (US4 — P2)

### Step 1: Deploy Validation Registry

Deploy the Validation Registry with the Identity Registry address as a constructor parameter.

### Step 2: Create Validation Request

The agent owner calls `validationRequest(validatorAddress, agentId, requestURI, requestHash)`.

**Verify**: `ValidationRequested` event emitted. `getValidationStatus(requestHash)` returns `response = 0` (pending).

### Step 3: Validator Responds

The addressed validator calls `validationResponse(requestHash, 1, responseURI, responseHash, "security")` for a pass.

**Verify**: `ValidationResponded` event emitted. `getValidationStatus(requestHash)` returns `response = 1` (pass).

### Step 4: Query Summary

Call `getSummary(agentId, [], "security")` to see aggregated pass/fail counts filtered by "security" tag.

---

## Scenario 5: Proxy Deployment (US5 — P2)

### Step 1: Deploy Implementation Contracts

Deploy `IdentityRegistryV1`, `ReputationRegistryV1`, `ValidationRegistryV1` as implementation contracts (not directly used).

### Step 2: Deploy Proxies

Deploy `TransparentUpgradeableProxy` for each, pointing to the implementation. Use a `ProxyAdmin` contract to manage upgrades. The ProxyAdmin address MUST be different from the registry admin.

### Step 3: Interact Through Proxies

All user interactions (register, feedback, validation) go through the proxy addresses. State is stored in proxy storage.

### Step 4: Upgrade

Deploy `IdentityRegistryV2`. ProxyAdmin calls `upgrade(identityProxy, registryV2Address)`. All existing registrations are preserved.

---

## Scenario 6: Observer Trust Root (US6 — P3)

### Step 1: Observer Receives Heartbeat

A health observer (Spec 005) receives a heartbeat message on an agent's stdOut topic.

### Step 2: Verify Registration

The observer calls `lookup(senderAddress)` on the Identity Registry. If registered, it returns `(tokenId, agentURI)`.

### Step 3: Verify Ownership

The observer calls `ownerOf(tokenId)` (standard ERC-721). If the result matches `senderAddress`, the heartbeat sender is a verified registered agent.

### Step 4: Trust Decision

- Match → trust the heartbeat, update liveness state
- No match or unregistered → treat sender as untrusted
