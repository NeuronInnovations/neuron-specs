# Data Model: Peer Registry (EIP-8004 Registration)

**Branch**: `003-peer-registry` | **Date**: 2026-02-25 | **Source**: spec.md FR-R01..FR-R14, Key Entities

---

## Entities

### Registration

The primary entity — represents a Child's registration in an EIP-8004 registry, linked to an extended NFT (ERC-721).

| Field | Type | Required | Description | Source FR |
|-------|------|----------|-------------|----------|
| `registryAddress` | EVMAddress | MUST | Address of the EIP-8004 registry smart contract | FR-R01 |
| `childAddress` | EVMAddress | MUST | The Child's EVM address (NFT owner) | FR-R06, FR-R10 |
| `tokenId` | uint256 | MUST | ERC-721 token ID (auto-incrementing agentId from `register()`) | FR-R01 |
| `agentURI` | AgentURI | MUST | The registration's agent URI (services array) | FR-R02, FR-R03, FR-R08 |
| `chainId` | UnsignedInt64 | MUST | The chain ID where the registry is deployed (e.g. 1 for Ethereum mainnet, 295 for Hedera mainnet) | FR-R01 |

**Uniqueness** (FR-R05): One registration per (childAddress, registryAddress). The same Child MAY be registered in multiple different registries.

**Ownership** (FR-R10): `childAddress` MUST be the `ownerOf(tokenId)` on the ERC-721 contract. Ownership MUST NOT be assigned to the Parent, platform operator, or any other party at mint time.

**Proof-of-control** (FR-R06): All registration operations (create, update, revoke) MUST be signed by the NeuronPrivateKey corresponding to `childAddress`.

---

### AgentURI

The JSON registration file referenced by the EIP-8004 agentURI. Contains the services array.

| Field | Type | Required | Description | Source FR |
|-------|------|----------|-------------|----------|
| `services` | Array\<Service\> | MUST | Array of EIP-8004 service objects | FR-R02, FR-R03 |

**Completeness** (FR-R08): A complete AgentURI MUST contain:
- Exactly three `neuron-topic` service objects (one for each of: `stdIn`, `stdOut`, `stdErr`)
- At least one `neuron-p2p-exchange` service object
- Zero or one `DID` service object (OPTIONAL, per FR-R13)

**Serialization**: JSON. Field order: `services`. Services within the `services[]` array MUST be ordered deterministically for canonical JSON compliance (006 FR-W05): `neuron-topic` entries first (ordered by channel role: stdIn, stdOut, stdErr, then custom channels sorted alphabetically), followed by `neuron-p2p-exchange` entries (sorted alphabetically by `name`), followed by `neuron-commerce` entries (sorted alphabetically by `name`, defined by Spec 008), followed by optional entries (DID, etc.). This ordering ensures byte-identical serialization across SDK implementations.

---

### NeuronTopicService

An EIP-8004 service object representing a public communication channel. Schema is authoritative per 004 FR-T14; this entity describes the registry integration shape.

| Field | Type | Required | Validation | Source FR |
|-------|------|----------|------------|----------|
| `type` | string | MUST | Exact match: `"neuron-topic"` | FR-R02, 004 FR-T14 |
| `name` | string | MUST | Channel role: `"stdIn"`, `"stdOut"`, `"stdErr"`, or `"custom:<name>"` | FR-R02, 004 FR-T14 |
| `version` | string | MUST | Semantic version (e.g. `"1.0.0"`) | 004 FR-T14 |
| `channel` | string | MUST | Channel role, MUST match `name` for standard channels | 004 FR-T14 |
| `transport` | string | MUST | Backend kind: `"hcs"`, `"evm"`, `"kafka"`, etc. | FR-R02, 004 FR-T13, 004 FR-T14 |
| `anchor` | string | MUST | Anchoring ledger (e.g. `"hedera-mainnet"`, `"ethereum-mainnet"`) | 004 FR-T14 |
| `config` | JSONObject | MUST | Transport-specific configuration | 004 FR-T14 |
| `endpoint` | string | SHOULD | Compact Topic URI for backward-compatible EIP-8004 consumers | 004 FR-T14 |

**Per-channel independence** (004 FR-T13): Different channels for the same peer MAY use different transports. A valid registration MAY have stdIn on HCS, stdOut on Kafka, and stdErr on HCS.

**Config by transport**:
- `hcs`: `{ "network": string, "topicId": string }`
- `kafka`: `{ "bootstrapServers": StringArray, "topicName": string, "saslMechanism": string, "anchoring": { "method": string, "anchorTopicId": string, "anchorNetwork": string, "interval": string } }`
- `evm`: `{ "contractAddress": string, "eventSignature": string, "chainId": UnsignedInt64 }`

---

### NeuronP2PExchangeService

An EIP-8004 service object describing how to discover the peer's multiaddress via a topic-based signaling protocol. Schema is authoritative per 004 FR-T17.

| Field | Type | Required | Validation | Source FR |
|-------|------|----------|------------|----------|
| `type` | string | MUST | Exact match: `"neuron-p2p-exchange"` | FR-R03, 004 FR-T17 |
| `name` | string | MUST | Service name (e.g. `"p2p"`) | 004 FR-T17 |
| `version` | string | MUST | Semantic version (e.g. `"1.0.0"`) | 004 FR-T17 |
| `peerID` | string | MUST | libp2p PeerID derived from Child's NeuronPublicKey (per 002 FR-006) | FR-R03, 004 FR-T17 |
| `protocol` | string | MUST | Exchange protocol ID (e.g. `"/neuron/multiaddr-exchange/1.0.0"`) | FR-R03, 004 FR-T17 |
| `topicRef` | string | MUST | Cross-reference to a `neuron-topic` service `name` in the same agentURI | FR-R03, 004 FR-T17 |

**topicRef validation** (004 FR-T18): The `topicRef` value MUST match the `name` of an existing `neuron-topic` service in the same AgentURI. A broken reference produces a `BrokenTopicRef` error.

**Multiaddress NOT stored** (FR-R03): The actual multiaddress is NOT stored in the registry. It is exchanged over the referenced topic channel using the specified protocol.

---

### DIDService

An optional EIP-8004 service object providing the Child's operational DID identifier.

| Field | Type | Required | Validation | Source FR |
|-------|------|----------|------------|----------|
| `name` | string | MUST (when present) | Exact match: `"DID"` | FR-R13, FR-R14 |
| `endpoint` | string | MUST (when present) | `did:key:zQ3s...` — deterministically derived from Child's NeuronPublicKey (secp256k1, multicodec `0xe7`) | FR-R14 |
| `version` | string | MUST (when present) | Version string (e.g. `"v1"`) | FR-R13 |

**Identity model** (FR-R13, FR-R14):
- The DID MUST be derived from the registered Child's NeuronPublicKey (not the Parent's)
- The DID is an **operational identifier** for interoperability, NOT an identity root
- Trust verification: Child DID:key -> Child NeuronPublicKey -> Child EVMAddress -> parent-child link (001 FR-017) -> Parent DID (identity root)
- At most one DID service per registration

**Prohibited patterns** (Identity Model section):
- MUST NOT include a DID unrelated to the Child's NeuronPublicKey
- MUST NOT use the Parent's DID as the DID service value

---

### AdmissionPolicy

Describes a registry's admission policy. The mechanism is platform-defined and out of scope for this spec; this entity represents the policy's observable properties.

| Field | Type | Required | Description | Source FR |
|-------|------|----------|-------------|----------|
| `policyType` | string | MUST | `"permissionless"` or `"permissioned"` | FR-R09 |
| `trustAnchor` | string | MUST (permissioned) | `"parent-did"` — identity lineage MUST anchor to Parent's DID | FR-R09 |
| `allowlist` | StringArray | MAY (permissioned) | List of Parent DIDs permitted to register Children | FR-R12 |

**Allowlist behavior** (FR-R12):
- (a) Operates at the Parent DID level — one entry covers all Children of that Parent
- (b) Only the Registry Administrator may modify the allowlist
- (c) Adding a Parent DID makes all its Children eligible (subject to FR-R06 proof-of-control)
- (d) Removing a Parent DID does NOT auto-revoke existing registrations; only new registrations by Children of the removed Parent are rejected
- (e) Explicit revocation of existing registrations is out of scope

---

### RegistrationResult

The result of a registration operation (create, update, revoke).

| Field | Type | Description | Source FR |
|-------|------|-------------|----------|
| `tokenId` | uint256 | The ERC-721 token ID of the registered NFT | FR-R01 |
| `transactionHash` | string | The on-chain transaction hash | FR-R06 |
| `childAddress` | EVMAddress | The Child's EVM address (NFT owner) | FR-R10 |
| `registryAddress` | EVMAddress | The registry contract address | FR-R01 |
| `chainId` | UnsignedInt64 | The chain ID | FR-R01 |
| `agentURI` | string | The agentURI string stored on-chain | FR-R08 |

---

### RegistryRole (Enum)

| Value | Name | Capabilities | Source FR |
|-------|------|--------------|----------|
| `REGISTRY_ADMIN` | Registry Administrator | Manage admission policy (add/remove Parent DIDs). MUST NOT mint, modify registrations, or transfer NFTs. | FR-R11a |
| `REGISTERED_AGENT` | Registered Agent (Child) | Update own agentURI, set own metadata, approve operators, revoke own registration. MUST NOT modify other Children's registrations. | FR-R11b |
| `DELEGATED_OPERATOR` | Delegated Operator | Update agentURI and metadata on approved tokens. MAY transfer approved tokens (ERC-721 semantics). MUST NOT call `register()`. | FR-R11c |
| `PARENT` | Parent | Create Child accounts (per 001). MAY be listed on allowlist via DID. MUST NOT call `register()` or update Child's registration. | FR-R11d |

---

### Error Types

| Error Kind | Description | Source FR |
|------------|-------------|----------|
| `RegistryUnavailable` | Registry contract unreachable or RPC error | FR-R07 |
| `RegistrationNotFound` | No registration found for the given lookup key | FR-R07 |
| `IncompleteRegistration` | AgentURI missing required services (three neuron-topic + one neuron-p2p-exchange) | FR-R08 |
| `ProofOfControlFailed` | Transaction signer does not match the Child's EVMAddress | FR-R06 |
| `AdmissionDenied` | Registry admission policy rejected the registration | FR-R09 |
| `DuplicateRegistration` | Child already has a registration in this registry | FR-R05 |
| `InvalidDIDService` | DID service value does not match Child's NeuronPublicKey, or multiple DID services | FR-R14 |
| `BrokenTopicRef` | `topicRef` in neuron-p2p-exchange does not match any neuron-topic service name | 004 FR-T18 |
| `InvalidServiceSchema` | Service object missing required fields or has invalid field values | FR-R02, FR-R03 |
| `UnauthorizedOperation` | Caller does not have permission for the requested operation (role boundary violation) | FR-R11 |
| `AllowlistRejection` | Child's Parent DID is not on the registry's allowlist | FR-R12d |

---

## Validation Rules

| Rule | Check | Error on Failure | Source FR |
|------|-------|-----------------|----------|
| V-REG-01 | AgentURI contains exactly 3 `neuron-topic` services (stdIn, stdOut, stdErr) | `IncompleteRegistration` | FR-R08, FR-R02 |
| V-REG-02 | AgentURI contains at least 1 `neuron-p2p-exchange` service | `IncompleteRegistration` | FR-R08, FR-R03 |
| V-REG-03 | Each `neuron-topic` has all MUST fields (`type`, `name`, `version`, `channel`, `transport`, `anchor`, `config`) | `InvalidServiceSchema` | FR-R02, 004 FR-T14 |
| V-REG-04 | Each `neuron-p2p-exchange` has all MUST fields (`type`, `name`, `version`, `peerID`, `protocol`, `topicRef`) | `InvalidServiceSchema` | FR-R03, 004 FR-T17 |
| V-REG-05 | `topicRef` in neuron-p2p-exchange matches a `neuron-topic` service `name` | `BrokenTopicRef` | 004 FR-T18 |
| V-REG-06 | DID service (when present) value is `did:key:zQ3s...` derived from Child's NeuronPublicKey | `InvalidDIDService` | FR-R14 |
| V-REG-07 | At most one DID service per registration | `InvalidDIDService` | FR-R14 |
| V-REG-08 | Registration transaction is signed by Child's NeuronPrivateKey (signer matches `childAddress`) | `ProofOfControlFailed` | FR-R06 |
| V-REG-09 | Child does not already have a registration in this registry | `DuplicateRegistration` | FR-R05 |
| V-REG-10 | NFT owner after mint is the Child's EVMAddress | `UnauthorizedOperation` | FR-R10 |
| V-REG-11 | Standard channel names (`stdIn`, `stdOut`, `stdErr`) each appear exactly once in neuron-topic services | `IncompleteRegistration` | FR-R02, FR-R08 |
| V-REG-12 | neuron-p2p-exchange `peerID` matches Child's PeerID (derived from NeuronPublicKey per 002 FR-006) | `InvalidServiceSchema` | FR-R03 |

---

## Relationships

```
Registration ──[lives in]──► EIP-8004 Registry (smart contract, FR-R01)
Registration ──[linked to]──► Extended NFT (ERC-721 token, FR-R01)
Registration ──[owned by]──► Child EVMAddress (FR-R10)
Registration ──[signed by]──► Child NeuronPrivateKey (proof-of-control, FR-R06)
Registration ──[contains]──► AgentURI (FR-R08)

AgentURI ──[contains 3x]──► NeuronTopicService (stdIn, stdOut, stdErr; FR-R02)
AgentURI ──[contains 1x]──► NeuronP2PExchangeService (FR-R03)
AgentURI ──[contains 0..1x]──► DIDService (FR-R13)

NeuronTopicService.channel ──[defined by]──► 004 FR-T14 (schema authority)
NeuronTopicService.transport ──[independently backed]──► 004 FR-T13

NeuronP2PExchangeService.peerID ──[derived from]──► NeuronPublicKey.PeerID() (002 FR-006)
NeuronP2PExchangeService.topicRef ──[references]──► NeuronTopicService.name (004 FR-T18)
NeuronP2PExchangeService.protocol ──[defines exchange over]──► Referenced topic channel

DIDService.endpoint ──[derived from]──► NeuronPublicKey.DIDKey() (002 FR-006a)
DIDService ──[NOT identity root]──► Trust chain → Parent DID (001 FR-012)

Child EVMAddress ──[from]──► NeuronAccount.evmAddress (001 FR-008)
Child EVMAddress ──[per registry]──► At most one Registration (FR-R05)

AdmissionPolicy ──[anchored to]──► Parent DID (FR-R09)
AdmissionPolicy.allowlist ──[lists]──► Parent DIDs (FR-R12)
```
