# API Contract: Registration (Peer Registry)

**Source**: spec.md FR-R01..FR-R14, SC-R01..SC-R03

---

## Register

Registers a Child in an EIP-8004 registry, minting an extended NFT (ERC-721) and storing the agentURI on-chain.

**Input**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `childKey` | NeuronPrivateKey | MUST | The Child's private key (for proof-of-control and transaction signing) |
| `registryAddress` | EVMAddress | MUST | Address of the EIP-8004 registry contract |
| `chainId` | UnsignedInt64 | MUST | Chain ID where the registry is deployed |
| `agentURI` | AgentURI | MUST | The agent URI containing services (must pass completeness validation) |
| `client` | Blockchain RPC client | MUST | Ethereum-compatible JSON-RPC client connected to the target chain |

**Output**: Returns RegistrationResult. Raises Error if registration fails.

**Behavior** (FR-R01, FR-R06, FR-R08, FR-R10):
1. Derive `childAddress` from `childKey.PublicKey().EVMAddress()`
2. Validate agentURI completeness via `ValidateRegistrationCompleteness(agentURI, childKey.PublicKey())` — reject with `IncompleteRegistration` or `InvalidServiceSchema` if validation fails
3. Check no existing registration for `childAddress` in `registryAddress` — reject with `DuplicateRegistration` if found (FR-R05)
4. Serialize `agentURI` to JSON string
5. Build transaction: call `register(agentURIJson)` on the registry contract
6. Sign transaction with `childKey` (proof-of-control, FR-R06) — uses deterministic signing (RFC 6979) per Constitution X
7. Submit transaction to the chain via `client`
8. Wait for transaction receipt
9. Verify `ownerOf(tokenId) == childAddress` (FR-R10) — reject with `UnauthorizedOperation` if mismatch
10. Return `RegistrationResult{tokenId, transactionHash, childAddress, registryAddress, chainId, agentURIJson}`

**Error conditions**:
- `IncompleteRegistration` — agentURI missing required services (V-REG-01, V-REG-02)
- `InvalidServiceSchema` — service object has invalid fields (V-REG-03, V-REG-04)
- `BrokenTopicRef` — topicRef does not match a neuron-topic name (V-REG-05)
- `InvalidDIDService` — DID service fails validation (V-REG-06, V-REG-07)
- `DuplicateRegistration` — Child already registered in this registry (FR-R05)
- `AdmissionDenied` — registry admission policy rejected the Child (FR-R09)
- `RegistryUnavailable` — contract call failed (FR-R07)

---

## UpdateRegistration

Updates the agentURI of an existing registration. Only the Child (NFT owner) or an approved operator may call this.

**Input**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `childKey` | NeuronPrivateKey | MUST | The Child's private key (or delegated operator's key) |
| `registryAddress` | EVMAddress | MUST | Address of the EIP-8004 registry contract |
| `chainId` | UnsignedInt64 | MUST | Chain ID |
| `tokenId` | uint256 | MUST | The token ID of the existing registration |
| `newAgentURI` | AgentURI | MUST | The updated agent URI |
| `client` | Blockchain RPC client | MUST | Ethereum-compatible JSON-RPC client |

**Output**: Returns RegistrationResult. Raises Error if update fails.

**Behavior** (FR-R06, FR-R08, FR-R11b, FR-R11c):
1. Derive caller address from `childKey`
2. Verify caller is `ownerOf(tokenId)` or is an approved operator — reject with `UnauthorizedOperation` if neither (FR-R11b, FR-R11c)
3. Validate `newAgentURI` completeness — reject if invalid
4. Serialize `newAgentURI` to JSON
5. Build transaction: call `updateAgentURI(tokenId, agentURIJson)` on the contract
6. Sign and submit transaction
7. Return updated `RegistrationResult`

**Error conditions**:
- `UnauthorizedOperation` — caller is not owner or approved operator
- `RegistrationNotFound` — tokenId does not exist
- `IncompleteRegistration` / `InvalidServiceSchema` — new agentURI fails validation
- `RegistryUnavailable` — contract call failed

---

## RevokeRegistration

Revokes the Child's registration by burning the NFT. Only the Child (NFT owner) may revoke.

**Input**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `childKey` | NeuronPrivateKey | MUST | The Child's private key |
| `registryAddress` | EVMAddress | MUST | Address of the EIP-8004 registry contract |
| `chainId` | UnsignedInt64 | MUST | Chain ID |
| `tokenId` | uint256 | MUST | The token ID to revoke |
| `client` | Blockchain RPC client | MUST | Ethereum-compatible JSON-RPC client |

**Output**: Returns transactionHash (string). Raises Error if revocation fails.

**Behavior** (FR-R06, FR-R11b):
1. Derive caller address from `childKey`
2. Verify caller is `ownerOf(tokenId)` — reject with `UnauthorizedOperation` if not
3. Build transaction: call the revocation method on the contract (burn or deregister)
4. Sign with `childKey` (proof-of-control, FR-R06)
5. Submit and wait for receipt
6. Return transaction hash

**Error conditions**:
- `UnauthorizedOperation` — caller is not the NFT owner (FR-R11b)
- `RegistrationNotFound` — tokenId does not exist
- `RegistryUnavailable` — contract call failed

**Note**: The Registry Administrator MUST NOT be able to revoke a Child's registration (FR-R11a). Revocation semantics beyond owner-initiated revocation are out of scope (FR-R12e).

---

## LookupRegistration

Looks up a Child's registration by EVM address or registry binding.

**Input**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `registryAddress` | EVMAddress | MUST | Address of the EIP-8004 registry contract |
| `chainId` | UnsignedInt64 | MUST | Chain ID |
| `lookupKey` | LookupKey | MUST | Either `ByEVMAddress(EVMAddress)` or `ByExternalID(string)` |
| `client` | Blockchain RPC client | MUST | Ethereum-compatible JSON-RPC client |

**Output**: Returns Registration. Raises Error if lookup fails.

**Behavior** (FR-R04, SC-R01):
1. **ByEVMAddress**: Call `tokenOfOwnerByIndex(childAddress, 0)` to get the token ID (FR-R05: at most one per registry). Then call `agentURI(tokenId)` to get the agentURI JSON.
2. **ByExternalID**: Resolve the external ID to a Child EVMAddress via registry-specific lookup, then proceed as ByEVMAddress (FR-R04).
3. Parse the agentURI JSON into an `AgentURI` struct
4. Construct and return `Registration{registryAddress, childAddress, tokenId, agentURI, chainId}`

**Error conditions** (FR-R07):
- `RegistrationNotFound` — no token found for the address, or external ID not found
- `RegistryUnavailable` — RPC or contract call failed
- `InvalidServiceSchema` — agentURI JSON is malformed or has invalid service objects

**Failure semantics** (FR-R07): When lookup fails, the outcome MUST be a documented error. No silent failure. Retry, backoff, and caching are client-defined.

---

## ResolveTopics

Extracts the three mandatory `neuron-topic` services from a registration's agentURI.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `registration` | Registration | A previously looked-up registration |

**Output**: Returns (stdIn: NeuronTopicService, stdOut: NeuronTopicService, stdErr: NeuronTopicService). Raises Error if incomplete.

**Behavior** (FR-R02, FR-R08, SC-R02):
1. Filter `registration.agentURI.services` for entries with `type == "neuron-topic"`
2. Identify the service with `channel == "stdIn"` → `stdIn`
3. Identify the service with `channel == "stdOut"` → `stdOut`
4. Identify the service with `channel == "stdErr"` → `stdErr`
5. If any of the three is missing: return `IncompleteRegistration` error
6. Return the three services

**Note**: Each channel is independently backed (004 FR-T13). The caller MAY receive stdIn on HCS, stdOut on Kafka, and stdErr on HCS, for example. Transport-specific handling is the caller's responsibility via the appropriate TopicAdapter.

---

## ResolveP2PExchange

Extracts the `neuron-p2p-exchange` service from a registration's agentURI.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `registration` | Registration | A previously looked-up registration |

**Output**: Returns NeuronP2PExchangeService. Raises Error if not found or invalid.

**Behavior** (FR-R03, SC-R03):
1. Filter `registration.agentURI.services` for entries with `type == "neuron-p2p-exchange"`
2. Return the first matching service
3. If not found: return `IncompleteRegistration` error
4. Validate `topicRef` references an existing `neuron-topic` service name (004 FR-T18) — return `BrokenTopicRef` if invalid

**Usage**: The caller uses the returned `peerID`, `protocol`, and `topicRef` to initiate a multiaddress exchange. The caller:
1. Resolves `topicRef` to a `NeuronTopicService` (via `ResolveTopics`)
2. Connects to the referenced topic using the appropriate TopicAdapter
3. Initiates the exchange protocol to obtain the peer's current multiaddress
4. Dials the peer at the received multiaddress

---

## ValidateRegistrationCompleteness

Validates that an AgentURI meets all registration completeness requirements.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `agentURI` | AgentURI | The agent URI to validate |
| `childPublicKey` | NeuronPublicKey | The Child's public key (for DID and PeerID validation) |

**Output**: Returns (valid: Boolean, errors: Array\<ValidationError\>)

**Validation steps**:

| Step | Check | Error on Failure | Source |
|------|-------|-----------------|--------|
| V-REG-01 | Exactly 3 `neuron-topic` services (stdIn, stdOut, stdErr) | `IncompleteRegistration: missing neuron-topic services` | FR-R08, FR-R02 |
| V-REG-02 | At least 1 `neuron-p2p-exchange` service | `IncompleteRegistration: missing neuron-p2p-exchange service` | FR-R08, FR-R03 |
| V-REG-03 | Each neuron-topic has MUST fields (type, name, version, channel, transport, anchor, config) | `InvalidServiceSchema: neuron-topic missing {field}` | FR-R02, 004 FR-T14 |
| V-REG-04 | Each neuron-p2p-exchange has MUST fields (type, name, version, peerID, protocol, topicRef) | `InvalidServiceSchema: neuron-p2p-exchange missing {field}` | FR-R03, 004 FR-T17 |
| V-REG-05 | topicRef matches an existing neuron-topic service name | `BrokenTopicRef: "{topicRef}" does not match any neuron-topic service` | 004 FR-T18 |
| V-REG-06 | DID service (if present) is `did:key:zQ3s...` derived from childPublicKey | `InvalidDIDService: DID does not match Child's NeuronPublicKey` | FR-R14 |
| V-REG-07 | At most one DID service | `InvalidDIDService: multiple DID services` | FR-R14 |
| V-REG-11 | Each standard channel (stdIn, stdOut, stdErr) appears exactly once | `IncompleteRegistration: duplicate or missing channel {name}` | FR-R02, FR-R08 |
| V-REG-12 | peerID matches Child's PeerID (childPublicKey.PeerID()) | `InvalidServiceSchema: peerID does not match Child's PeerID` | FR-R03 |

**Returns**: `valid = true` if all checks pass; otherwise `valid = false` with the list of validation errors.
