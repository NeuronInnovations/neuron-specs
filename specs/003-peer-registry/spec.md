# Feature Specification: Peer Registry (EIP-8004 Registration)

**Feature Branch**: `003-peer-registry`  
**Created**: 2026-02-05  
**Status**: Draft  

## Related specs

- **Specs in this repo**:
  - [001 NeuronAccount Module](../001-neuron-account-module/spec.md) — upstream dependency; provides Child identity (EVM address), Parent payment address, registry binding, and proof of control.
  - [002 Key Library](../002-key-library/spec.md) — NeuronPrivateKey / NeuronPublicKey; signing for registration proof-of-control.
  - [004 Topic System](../004-topic-system/spec.md) — defines topic abstractions, TopicMessage envelope, TopicAdapter interface, and agentURI service schemas (`neuron-topic`, `neuron-p2p-exchange`) that this spec requires as EIP-8004 services (FR-R02).
  - [007 Identity Registry Smart Contract](../007-identity-contract/spec.md) — on-chain contract layer implementing the EIP-8004 registries. This spec (003) defines the SDK-level registration workflow; Spec 007 defines the on-chain contract that the SDK's `RegistryContract` interface calls. Mapping: 003 `Register()` → 007 `register()`, 003 `UpdateAgentURI()` → 007 `updateAgentURI()`, 003 `Revoke()` → 007 `revoke()`, 003 `Lookup()` → 007 `lookup()`.

> **Schema Authority**: Spec 004 (Topic System) is the authoritative specification for `neuron-topic` and `neuron-p2p-exchange` service schemas. This spec (003) defines which services MUST be present and registration completeness rules; the internal structure and field requirements are governed by 004.

- **External standards**:
  - [EIP-8004](https://eips.ethereum.org/EIPS/eip-8004) — Trustless Agents / Identity Registry; the registry contract model used for peer registration.

## Purpose

This specification defines **peer registration** for the Neuron SDK: how a **Child account** (identified by EVM address from the [NeuronAccount module](../001-neuron-account-module/spec.md)) is published and discovered via an **EIP-8004**-compatible registry. Registration is the **single place** where communication endpoints (public topics and private multiaddress) are defined; the account module does not store them. The account is the upstream identity and ledger link; the registry holds the extended peer profile, including an **extended NFT** (EIP-721) that the Child is linked to, and **services** for public comms (per-channel topic system per 004) and private comms (topic-based multiaddress exchange per 004 FR-T17).

**Relationship to Account (001)**

- The [NeuronAccount module](../001-neuron-account-module/spec.md) provides: Child/Parent identity, EVM address (child), payment address (Parent's evmAddress), proof of control of the ledger address, and optional registry binding (registry identifier + external id).
- This (registration) spec defines: how that Child **has a registration** in an EIP-8004 registry; the registration is represented by an **extended NFT** (EIP-721); the NFT and EIP-8004 **services** carry identity elements (DID, MCP, Email, Trust Registry), **public communication** (unified topic system: stdIn, stdOut, stdErr), and **private communication** (topic-based multiaddress exchange via `peerID`/`protocol`/`topicRef` per 004 FR-T17).

**Registry is fully smart contract**  
The registry is implemented as a **smart contract** (EIP-8004 Identity Registry). This exists for both **Hedera** and **Ethereum**; the same EIP-8004 contract model applies on both chains (implementation details per chain may be documented elsewhere).

**EIP-8004 base**  
Registration is fully based on [EIP-8004](https://eips.ethereum.org/EIPS/eip-8004) (Identity Registry). The Child's EVM address is linked to an **extended NFT** (EIP-721); that NFT and the registry's **service** model carry:

- **Public comms (topics)**: Public, DLT-traceable communication endpoints. The **topic system** (unified topics) is defined in a **separate spec**; whatever that spec defines MUST be **provided as EIP-8004 services** (e.g. stdIn, stdOut, stdErr) in the registry's service fields.
- **Private comms (p2pconns)**: Peer-to-peer connectivity (e.g. **multiaddress** / libp2p). The **p2p/multiaddress** model (direct vs indirect, protocols) is defined in a **separate spec**; whatever that spec defines MUST be **provided as EIP-8004 services** in the registry's service fields.

**Topics and p2pconns: defined elsewhere, mandatory here**

- **Topics** (public DLT-traceable comms): Full definition (unified topic system, backends, adapters) lives in a **dedicated topics spec** (004). This (registry) spec requires that topic services are **mandatory** and **per-channel** — every registration MUST include three `neuron-topic` service objects (one per standard channel: stdIn, stdOut, stdErr). Each channel is **independently backed** and MAY use a different transport (per 004 FR-T13); the per-channel schema is defined in 004 FR-T14.
- **P2P connections** (multiaddress): The p2p service describes **how to discover this peer's multiaddress** via a topic-based signaling protocol. Full protocol details live in a **dedicated spec** (004). This (registry) spec requires that a **neuron-p2p-exchange** service is **mandatory** and carries `peerID`, `protocol`, and `topicRef` fields (per 004 FR-T17). The actual multiaddress is NOT stored in the registry; it is exchanged over the referenced topic channel.

## Clarifications

### Session 2026-02-05

- Q: Where do PublicCommsAddress and P2PCommAddress (topics, multiaddress) live? → A: They move **out of the account spec** into this **registration spec**. The account (001) does not store communication endpoints; the account identifies the entity (e.g. Child by EVM address). Communication endpoints are **discovered via registration** (this spec): the Child is linked to an EIP-8004 extended NFT, and topics + multiaddress are part of the registry's services (EIP-8004++).
- Q: Can a Child have multiple registrations (e.g. in different registries) or at most one? → A: One registration per Child **per registry instance**. The same Child can be registered in multiple registries.
- Q: Who can create, update, or revoke a registration? → A: The **Child itself**, using its own NeuronPrivateKey (proof-of-control). The Child's EVMAddress is the registering entity and NFT owner. See FR-R06.
- Q: When registry is unavailable or lookup returns not found, what should clients expect? → A: **Fail clearly**: documented error/absence; retry and caching are client-defined (out of scope here).
- Q: What does "registry binding" mean for resolution? → A: **Registry instance + optional alternate key**: "Registry binding" = (registry identifier, optional external id). Lookup by (registry + Child EVM address) or (registry + external id) when account has that binding.
- Q: Any scale or latency targets for registration lookup? → A: **No targets in this spec**; latency and scale are implementation/deployment concerns.
- Q: Are topic and p2p services required in every registration? → A: **Yes.** Topic services (neuron-topic) and p2p services (neuron-p2p-exchange) are **mandatory** in the registry's `services` array for every registration.
- Q: How does the p2p exchange describe how to get the peer's multiaddress? → A: The **neuron-p2p-exchange** service carries three required fields (per 004 FR-T17): **`peerID`** (the peer's libp2p PeerID from 002), **`protocol`** (the exchange protocol ID, e.g. `/neuron/multiaddr-exchange/1.0.0`), and **`topicRef`** (a cross-reference to a `neuron-topic` service name in the same agentURI, validated per 004 FR-T18). The actual multiaddress is NOT stored in the registry; it is exchanged over the referenced topic channel using the specified protocol. _(Note: prior drafts described a `multiAddressDiscovery` array with multiple methods — registry, topic, dnsTxt, dht. That model has been superseded by 004's topic-based signaling approach; multi-method discovery may be reintroduced as a future extension.)_

### Session 2026-02-11

- Q: Who actually calls `register()` — the Parent or the Child? → A: The **Child**. The Child's EVMAddress is the registering entity and becomes the NFT owner (FR-R06, FR-R10). The Parent's role is upstream: it creates the Child (001) and its DID serves as the trust anchor for admission policies (FR-R09). The Parent does not sign registration transactions.
- Q: How does a permissioned registry decide who can register? → A: Via an **admission policy** anchored to the Parent's DID (FR-R09). The platform trusts a Parent DID; any Child provably linked to that Parent is admitted. The admission mechanism is platform-defined and out of scope for this spec.
- Q: Can a Child include a DID in its registration? → A: Yes, as a **service** (like MCP or email). It is the Child's NeuronPublicKey in `did:key` format — an interoperability identifier, not a new identity root (FR-R13, FR-R14). See [Neuron Identity Model](../../docs/neuron-identity-model.md).
- Q: What happens to existing registrations when a Parent DID is removed from an allowlist? → A: **Nothing.** Existing registrations are retained. Only new registration attempts by Children of the removed Parent are rejected (FR-R12d). Revoking an existing registration is a separate operation.
- Q: Can the Registry Administrator modify a Child's registration? → A: **No.** The Registry Administrator manages the admission policy only. Only the Child (as NFT owner) or its approved operators may modify its registration (FR-R11a, FR-R11b, FR-R11c).
- Q: Is the Parent's DID the only identity root, or can a Registration introduce a separate identity root? → A: The Parent's DID (`did:key` from the Parent's NeuronPublicKey) is the **sole identity root** for the hierarchy. The Registration MAY include a DID service (`did:key` from the Child's NeuronPublicKey) as an operational identifier for interoperability, but it is NOT an identity root. Trust verification follows the parent-child chain to the Parent's DID. See [Neuron Identity Model](../../docs/neuron-identity-model.md).

### Session 2026-05-08

- Q: Spec 008 FR-P05 already permits multiple `neuron-commerce` entries per registration, and 008 FR-P33a (added 2026-05-08) introduces a `streams[]` catalog where each entry carries a libp2p protocol ID that MAY be a wildcard pattern (e.g., `/jetvision/filtered/*`). Does 003 need a normative change to support this? → A: No. 003 already defers `services[]` schema authority to the per-service spec (FR-R02 to 004 for `neuron-topic`, FR-R03 to 004 for `neuron-p2p-exchange`, FR-P01 in 008 for `neuron-commerce`, FR-V12 in 010 for `neuron-validator`, and now also 016/017 for DApp-specific service entries). The on-chain `agentURI` JSON is opaque to 003 (DD-01); content validation is service-spec authority. Adding a documentation example in this spec's appendix to make path-based protocol IDs visible to readers; no normative requirement changes. Per Constitution Principle XII, the *content* of any specific service entry (frame format, stream catalog, fan-out topology) belongs to the consuming spec, not 003.

## Out of Scope

- Account creation, parent-child relationship, ledger attachment, and balance remain in the [NeuronAccount module](../001-neuron-account-module/spec.md).
- **Topics** (public DLT-traceable comms) and **p2pconns** (multiaddress) are fully defined in **separate specs**; this spec requires that **both** are **mandatory** as EIP-8004 services (neuron-topic and neuron-p2p-exchange) in every registration.
- Exact on-chain contract ABI, RPC, and transaction flows are defined in [Spec 007 (Identity Registry Smart Contract)](../007-identity-contract/spec.md). This spec defines the SDK-level workflow; Spec 007 defines the on-chain contract behavior.
- Message transport or routing semantics (how bytes are sent on topics or over multiaddress) are not defined here; only the registration shape and how endpoints are published/discovered.
- Retry, backoff, and caching behavior for registration lookups are client-defined; this spec only requires that failure is reported as a documented error or absence (FR-R07).
- Performance and capacity (lookup latency, max registrations per registry) are not defined in this spec; they are implementation and deployment concerns.
- **Admission policy mechanism**: How a registry verifies Parent-Child lineage on-chain (mapping, Merkle tree, oracle) is not defined; this spec requires only that the trust anchor is the Parent's DID (FR-R09).
- **Revocation semantics**: Revoking an existing registration is a registry governance concern, out of scope for this spec (FR-R12e).
- **Emergency key recovery**: Recovery when a Child's NeuronPrivateKey is compromised is a known limitation, not addressed in this spec. A future access-control revision will address recovery.

## Identity Model

The registration follows the **single-root hierarchical identity model** defined in the [Neuron Identity Model](../../docs/neuron-identity-model.md):

1. **Identity root**: The Parent Account's DID (`did:key`, derived from the Parent's NeuronPublicKey per 001 FR-012) is the sole identity root for the Parent-Child hierarchy.

2. **Child identity**: The Child's identity is its EVMAddress (derived from its own NeuronPublicKey per 002). The Child does NOT have an authoritative DID on the Account entity. Identity resolution proceeds via the parent-child relationship (001 FR-017) to the Parent's DID.

3. **DID service on registration**: The registration MAY include a DID service (`name: "DID"`) in the EIP-8004 services array. When present:
   - The DID value MUST be a `did:key` derived from the registered Child's NeuronPublicKey (secp256k1).
   - The `did:key` MUST use the secp256k1 multicodec encoding (`0xe7`, base58btc prefix `zQ3s`), consistent with 002.
   - This DID is an **operational identifier** for DID-based discovery. It is NOT an identity root.
   - Trust verification follows: Child DID:key -> Child NeuronPublicKey -> Child EVMAddress -> parent-child link (001 FR-017) -> Parent DID (identity root).

4. **Prohibited patterns**: The registration MUST NOT include a DID that is unrelated to the Child's NeuronPublicKey. The registration MUST NOT use the Parent's DID as its DID service value. At most one DID service is permitted per registration.

## Terminology Mapping (003 ↔ 007)

This spec (003) and [Spec 007 (Identity Registry Smart Contract)](../007-identity-contract/spec.md) describe the same system from different perspectives — SDK workflow vs. on-chain contract behavior. The following terms are synonymous:

| This spec (003) | Spec 007 | EIP-8004 | Description |
|-----------------|----------|----------|-------------|
| extended NFT | registration NFT / ERC-721 token | — | The on-chain token representing a registered agent |
| registration | identity (registry context) | identity | The record linking an address to an agentURI |
| token id | tokenId / agentId | agentId | The `uint256` identifier assigned on `register()` |
| registry (EIP-8004) | Identity Registry contract | Identity Registry | The smart contract holding registrations |

## Access Control Model

Registration uses a **two-layer access control model**:

1. **Layer 1 — Proof-of-Control** (MUST, universal): The Child's EVMAddress calls `register()`. The transaction signature proves control of the Child's NeuronPrivateKey. See FR-R06.

2. **Layer 2 — Admission Policy** (MAY, platform-defined): A registry MAY enforce an admission policy that restricts which entities can register. When the policy references identity lineage, it MUST anchor to the Parent's DID. See FR-R09.

3. **Ownership**: Upon successful registration, the Child's EVMAddress owns the resulting NFT. See FR-R10.

4. **Roles**: Registry Administrator (manages admission policy), Registered Agent (Child with NFT), Delegated Operator (ERC-721 approved), Parent (structural authority, not transactional). See FR-R11.

5. **Allowlist**: When a registry uses an allowlist, it operates at the Parent DID level. Removing a Parent DID does NOT auto-revoke existing registrations. See FR-R12.

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Register Child as EIP-8004 extended NFT (Priority: P1)

A developer has a Child account (from the account module) and wants to register it in an EIP-8004 registry so that the Child is linked to an extended NFT (EIP-721). The registration exposes the Child's identity (EVM address) and allows attaching services (public topics, private multiaddress) to that NFT.

**Acceptance Scenarios**:

1. **Given** a Child account with an EVM address and optional registry binding, **When** the developer (or registry client) registers the Child in an EIP-8004 registry, **Then** the Child is associated with an extended NFT and the registry entry can be resolved by EVM address (or token id).
2. **Given** a registered Child, **When** a client looks up the registration by Child EVM address, **Then** the extended NFT and its services (topics, multiaddress if present) can be retrieved.

---

### User Story 2 - Public comms (unified topic system) as EIP-8004 services (Priority: P1)

Public communication channels (stdIn, stdOut, stdErr) are part of the registration's services, using a **unified topic system** that can be backed by Hedera HCS, Ethereum events, Kafka, or other transports via adapters.

**Acceptance Scenarios**:

1. **Given** a registered Child (extended NFT), **When** the developer attaches public comms services (e.g. topic kind + locator for stdIn, stdOut, stdErr), **Then** those are stored as EIP-8004 services using service type `neuron-topic` (as defined in the [Topic System spec](../004-topic-system/spec.md)) and resolvable from the NFT.
2. **Given** different backends (HCS, Ethereum events, Kafka), **When** a unified topic adapter is used, **Then** the same topic abstraction (kind + locator) works and the registration does not depend on a single transport.

---

### User Story 3 - Private comms (topic-based multiaddress exchange) (Priority: P2)

Private peer-to-peer connectivity is provided via multiaddress (libp2p). The **neuron-p2p-exchange** service declares the peer's **peerID**, an exchange **protocol**, and a **topicRef** pointing to a `neuron-topic` channel in the same agentURI (per 004 FR-T17). The actual multiaddress is NOT stored in the registry; it is exchanged over the referenced topic using the specified protocol.

**Acceptance Scenarios**:

1. **Given** a registered Child with a `neuron-p2p-exchange` service, **When** a client reads the `peerID`, `protocol`, and `topicRef` fields, **Then** the client knows the peer's libp2p PeerID, the exchange protocol to use, and which topic channel to contact the peer on.
2. **Given** a valid `topicRef` (e.g. `"stdIn"`), **When** the client resolves it against the agentURI's `neuron-topic` services, **Then** the reference MUST match an existing `neuron-topic` service `name` (per 004 FR-T18); an invalid reference produces a `BrokenTopicRef` error.
3. **Given** the resolved topic channel and protocol, **When** the requester initiates a multiaddress exchange over that topic, **Then** the peer responds with their current multiaddress so the requester can dial.

---

## Requirements _(mandatory)_

### Functional Requirements

- **FR-R01**: The **registry** MUST be implemented as a **smart contract** (EIP-8004). The same contract model applies on **Hedera** and **Ethereum**. The Child (identified by EVM address from the account module) MUST be linkable to an **extended NFT** (EIP-721) in the registry.
- **FR-R02**: **Topic services** (neuron-topic; public DLT-traceable comms) are **mandatory** and **per-channel**. Every registration MUST include three `neuron-topic` service objects — one for each standard channel role: `stdIn`, `stdOut`, and `stdErr`. Each channel is **independently backed**: different channels for the same peer MAY use different transports (per 004 FR-T13). The per-channel service schema (fields: `type`, `name`, `version`, `channel`, `transport`, `anchor`, `config`, and optionally `endpoint`) is defined in 004 FR-T14; this spec defers to 004 for the internal structure of each service object.
- **FR-R03**: **P2P services** (neuron-p2p-exchange; multiaddress / libp2p) are **mandatory**. Every registration MUST include a **neuron-p2p-exchange** service with the following required fields (per 004 FR-T17): **`peerID`** (MUST — the peer's libp2p PeerID, derived from NeuronPublicKey per 002), **`protocol`** (MUST — the exchange protocol ID, e.g. `/neuron/multiaddr-exchange/1.0.0`), and **`topicRef`** (MUST — a cross-reference to a `neuron-topic` service `name` in the same agentURI; validated per 004 FR-T18). The actual multiaddress is NOT stored in the registry; it is exchanged over the referenced topic channel using the specified protocol. _(Multi-method discovery — e.g. registry-direct, DNS TXT, DHT — is noted as a potential future extension; this spec defers to 004 for the current schema.)_
- **FR-R04**: The account module (001) MUST NOT store PublicCommsAddress or P2PCommAddress; the account provides identity (EVM address, parent, payment address, proof of control). Resolution of topics and multiaddress MUST be via this registration spec. Lookup is by (registry instance + Child EVM address) or by (registry instance + external id) when the account has a registry binding (registry identifier + optional external id).
- **FR-R05**: Uniqueness of registration is **per registry instance**: a Child MAY have at most one registration per EIP-8004 registry; the same Child MAY be registered in multiple different registries.
- **FR-R06**: The **registering entity** for a given Child MUST be the Child's own EVMAddress (i.e., the address derived from the Child's NeuronPrivateKey per 002). The initial `register()` call MUST be signed by the Child's NeuronPrivateKey — this constitutes **proof-of-control** at mint time. Post-mint, `updateAgentURI()` and `revoke()` are governed by ERC-721 token ownership: the **current token owner** (which may differ from the original registering Child after an ERC-721 transfer) controls these operations. Standard ERC-721 transfer semantics apply per [Spec 007 DD-03](../007-identity-contract/spec.md). No registry administrator or third party MAY mint on behalf of a Child.
- **FR-R07**: When a registration lookup fails (registry unavailable, timeout, or not found), the outcome MUST be a **documented error or absence** (no silent failure). Retry, backoff, and caching are client-defined and out of scope for this spec.
- **FR-R08**: A **complete registration** MUST include three `neuron-topic` service objects (as defined in the [Topic System spec](../004-topic-system/spec.md)) — one for each standard channel role: `stdIn`, `stdOut`, and `stdErr`. Each channel is independently backed and MAY use a different transport (per 004 FR-T13). The per-channel service schema is defined in 004 FR-T14. A registration without all three topic services is considered **incomplete**. Channels are NOT required to share the same transport; a valid registration MAY have stdIn on HCS, stdOut on Kafka, and stdErr on HCS, for example.
- **FR-R09**: A registry instance MAY enforce an **admission policy** that restricts which entities are permitted to register. When an admission policy references identity lineage, it MUST anchor to the **Parent's DID** (the single identity root per 001) — not to individual Child EVMAddresses. The mechanism by which a registry verifies Parent-Child lineage is out of scope for this spec; the requirement is that the trust anchor is the Parent's DID. A registry with no admission policy is considered **permissionless** (any entity satisfying proof-of-control per FR-R06 MAY register).
- **FR-R10**: Upon successful registration, the Child's EVMAddress MUST be the **owner** of the resulting NFT (EIP-721 token) in that registry. The Child, as NFT owner, MAY update its `agentURI`, set metadata, and approve operators per the EIP-8004 standard. Ownership of the registration NFT MUST NOT be assigned to the Parent, the platform operator, or any other party at mint time.
- **FR-R11**: The following role boundaries MUST be observed for registry operations:
  (a) **Registry Administrator** (the platform operator) MAY manage the admission policy (e.g. add or remove Parent DIDs from an allowlist). The Registry Administrator MUST NOT mint NFTs on behalf of a Child, modify a Child's registration, or transfer or revoke a Child's NFT.
  (b) **Registered Agent** (a Child holding an NFT in the registry) MAY update its own `agentURI`, set its own metadata, approve operators for its own NFT, and revoke its own registration. A Registered Agent MUST NOT modify another Child's registration.
  (c) **Delegated Operator** (an address approved by a Registered Agent per ERC-721 `approve()` or `setApprovalForAll()`) MAY update `agentURI` and metadata on the approved token(s). Per standard ERC-721 semantics, an approved operator MAY also transfer the approved token(s); this spec does not restrict standard ERC-721 transfer capabilities. A Delegated Operator MUST NOT call `register()` to create new registrations.
  (d) **Parent** MAY create Child accounts (per 001) and MAY be listed on a platform's allowlist via its DID. The Parent MUST NOT call `register()` on behalf of a Child or update a Child's registration. At mint time, the registration NFT MUST be owned by the Child's EVMAddress (per FR-R10); post-mint transfer is governed by standard ERC-721 semantics and platform governance, not by this spec.
- **FR-R12**: When a registry enforces a permissioned admission policy via an allowlist:
  (a) The allowlist MUST operate at the **Parent DID** level. A single allowlist entry covers all current and future Children of that Parent.
  (b) Only the **Registry Administrator** MAY add or remove Parent DIDs from the allowlist.
  (c) When a Parent DID is added, all Children of that Parent become **eligible** to register (subject to proof-of-control per FR-R06).
  (d) When a Parent DID is removed, **existing registrations** by Children of that Parent MUST NOT be automatically revoked. Only **new** registration attempts by Children of the removed Parent MUST be rejected.
  (e) Explicit revocation of an existing registration is a separate operation from allowlist removal. Revocation semantics are out of scope for this spec.
- **FR-R13**: A registration's agentURI registration file MAY include a `DID` service entry. When present, this MUST be the Child's own NeuronPublicKey expressed in `did:key` format. This DID is an **interoperability identifier**. External systems MAY use it for verification or discovery of the Child agent. However, it MUST NOT be treated as an **identity root** — authority over the registration and trust delegation trace exclusively to the Parent's DID (the single identity root per 001) via the on-ledger Parent-Child link.
- **FR-R14**: When a registration includes a DID service (service name `"DID"`) in the EIP-8004 services array, the DID value MUST be a `did:key` identifier deterministically derived from the registered Child's NeuronPublicKey (secp256k1, per 002). The `did:key` MUST use multicodec `0xe7` (secp256k1-pub, base58btc prefix `zQ3s`). The DID service is an operational identifier for DID-based discovery and MUST NOT introduce an identity root independent of the Parent-Child hierarchy defined in 001. The DID service is OPTIONAL; a registration is complete without it (per FR-R08). See [Neuron Identity Model](../../docs/neuron-identity-model.md).
- **FR-R15**: A registration's agentURI `services[]` array MAY include a `neuron-validator` service object. When present, it declares the agent as a validator capable of producing evidence-based compliance assessments per [Spec 010 (Validation Framework)](../010-validation-framework/spec.md). The `neuron-validator` service MUST include: `type` (always `"neuron-validator"`), `name` (string), `version` (semver string), `domains` (array of strings — spec references or domain tags the validator covers, e.g. `["005-health"]`), `verdictDelivery` (string — `"topic"` for stdOut publication). The `neuron-validator` service is OPTIONAL; a registration is complete without it (per FR-R08). Schema authority: Spec 010 FR-V12.
- **FR-R16** *(added 2026-05-13 — Stage 3B 2026-05-08 amendment)*: A `neuron-p2p-exchange` service entry MAY carry an optional `multiaddrs` array of strings advertising the producer's current libp2p listen multiaddrs at the time of registration. Producers SHOULD filter the list to publicly-reachable interfaces (loopback / link-local / virtual-bridge addresses MUST be stripped) before publishing. Consumers MUST treat the field as **advisory**: the producer's listen address may have changed since registration, and consumers MUST still verify the dial target's libp2p PeerID against the registered `peerID` field (defence-in-depth — FR-R03). DApp specs MAY require this field as a normative DApp-level FR (see Spec 017 FR-R20 for the Remote ID example); Core SDK consumers MUST tolerate its absence. The `multiaddrs` field is intentionally **registration-time-only**; long-lived state changes (port re-binding, IP rotation) are signalled via `updateAgentURI()` rather than out-of-band heartbeat capabilities.

### Key Entities _(include if feature involves data)_

- **Registry (EIP-8004)**: Implemented as a **smart contract** on Hedera and Ethereum. The registry entry for a peer (Child) is linked to the Child's EVM address and contains or references the extended NFT (EIP-721). Uniqueness: one registration per (Child, registry instance).
- **Extended NFT (EIP-721)**: The on-chain token representing the registered peer. Carries a **`services`** array: identity (e.g. DID), **neuron-topic** (one service object per channel — stdIn, stdOut, stdErr — each independently backed per 004 FR-T13, schema per 004 FR-T14), and **neuron-p2p-exchange** (p2p multiaddress exchange with `peerID`, `protocol`, `topicRef` per 004 FR-T17). See appendix _EIP-8004 service shape (registry)_ for the normative structure.
- **Topics (public comms)**: Defined in 004 (Topic System). In this spec they are **mandatory** and **per-channel**: every registration MUST include three `neuron-topic` service objects (one per standard channel: stdIn, stdOut, stdErr). Each channel is independently backed and MAY use a different transport (per 004 FR-T13).
- **P2P connections (p2pconns / multiaddress)**: The **neuron-p2p-exchange** service is **mandatory** and carries `peerID` (libp2p PeerID from 002), `protocol` (exchange protocol ID), and `topicRef` (cross-reference to a `neuron-topic` service name, validated per 004 FR-T18). The actual multiaddress is NOT in the registry; it is exchanged over the referenced topic. Schema per 004 FR-T17.
- **Registry binding** (from account module 001): The pair (registry identifier, optional external id). Resolution of a Child's registration can be by (registry instance + Child EVM address) or by (registry instance + external id) when the account holds that binding.
- **NeuronValidatorService**: An optional EIP-8004 service object in agentURI `services[]`. Declares the agent as a third-party validator. Carries: `type` (`"neuron-validator"`), `name`, `version`, `domains` (spec refs or domain tags), `verdictDelivery` (`"topic"`). Follows the same service pattern as `neuron-topic`, `neuron-p2p-exchange`, and `neuron-commerce` (008). Schema defined in Spec 010 FR-V12. A registration is complete without this service (FR-R08).

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-R01**: A Child account (from 001) can be registered in an EIP-8004 registry and linked to an extended NFT; the registration is resolvable by Child EVM address or registry binding.
- **SC-R02**: Public comms (stdIn, stdOut, stdErr) are resolvable from the registration as EIP-8004 services using the unified topic system; at least one backend (e.g. HCS or Ethereum events) works via an adapter.
- **SC-R03**: Multiaddress can be discovered directly from the registration when registered, or indirectly via a defined protocol over public topics.

---

## Evidence & Validation *(mandatory)*

### Verification Tier

**`on-chain-only`**

A third-party validator can assess peer registry compliance by reading on-chain state from the Identity Registry (007). Registration existence, agentURI content, and service presence are all queryable via `lookup()` and `agentURI()` contract calls. No topic subscription is required for registry-level validation.

### Observable Signals

- **Registration existence**: `lookup(childAddress)` on the Identity Registry returns a valid `(tokenId, agentURI)` pair, confirming the agent is registered (007 FR-C-10)
- **agentURI content**: The agentURI string is stored on-chain (opaque) and retrievable via `agentURI(tokenId)`. Parsing the JSON reveals the `services[]` array with all declared service types
- **Mandatory service presence**: The `services[]` array contains three `neuron-topic` entries (stdIn, stdOut, stdErr) and one `neuron-p2p-exchange` entry — verifiable by parsing the agentURI JSON
- **Validator service presence**: When an agent claims to be a validator, the `services[]` array contains a `neuron-validator` entry with `domains` and `verdictDelivery` fields (FR-R15)

### Evidence Rules

- **VR-REG-01**: `lookup(childAddress)` returns a valid tokenId and non-empty agentURI → suggests the agent has a valid registration (compliant with FR-R01). If `lookup()` returns no result → the agent is not registered (non-compliant or not applicable).
- **VR-REG-02**: Parsing the agentURI's `services[]` array reveals three `neuron-topic` entries with distinct `channel` values (`stdIn`, `stdOut`, `stdErr`) and one `neuron-p2p-exchange` entry → suggests registration completeness (compliant with FR-R08). Missing mandatory services → non-compliant.
- **VR-REG-03**: A `neuron-p2p-exchange` entry's `topicRef` matches the `name` of an existing `neuron-topic` service in the same agentURI → suggests valid cross-reference (compliant with 004 FR-T18). Broken reference → non-compliant.
- **VR-REG-04**: An agent claiming validator status has a `neuron-validator` service with non-empty `domains` array and `verdictDelivery: "topic"` → suggests valid validator declaration (compliant with FR-R15). Missing required fields → non-compliant. Absence of `neuron-validator` service is not non-compliant (it is optional) — but if the agent is referenced as a validator in a `validationRequest()`, absence suggests the agent is not a registered validator (inconclusive).

### Non-Observable Areas

- **agentURI content accuracy**: The agentURI is an opaque string stored on-chain. The Identity Registry does not validate its internal JSON structure. An agent could store malformed JSON or a services array that does not match its actual capabilities. Validators must parse and verify the content off-chain.
- **Validator capability truth**: A `neuron-validator` service declaring `domains: ["005-health"]` is a self-declaration. The registry does not verify that the validator is actually capable of health validation. Validator credibility is assessed over time via the Reputation Registry (007) and evidence quality, not at registration time.

**Behavioral Inference Recipes**:

- If an agent's `neuron-validator` service declares domains but the agent never publishes evidence envelopes to its stdOut topic, infer the validator is inactive or non-operational. The Reputation Registry provides the mechanism for recording this observation.

### Suggested Evidence Recipes

**Recipe: Verify validator registration completeness**

1. Query Identity Registry `lookup(validatorAddress)` → confirm registration exists, extract agentURI
2. Parse agentURI JSON → extract `services[]` array
3. Verify mandatory services: three `neuron-topic` entries (stdIn, stdOut, stdErr) + one `neuron-p2p-exchange`
4. Verify `neuron-p2p-exchange` topicRef resolves to an existing `neuron-topic` service name
5. Check for `neuron-validator` service → extract `domains`, `verdictDelivery`, `version`
6. If all mandatory services present AND neuron-validator well-formed → validator registration is compliant
7. Construct evidence envelope (Spec 010 FR-V01–V07) with `specRef: "003-peer-registry"` and publish to validator's stdOut

---

## Appendix: High-level view

### Registration and account (conceptual)

- **Account (001)**: Parent/Child/Shared; EVM address (child); parent–child traceable on ledger; payment address; ledger attachment; optional registry binding. No communication endpoints stored.
- **Registration (003)**: Child EVM address **has a registration** → extended **NFT (EIP-721)**. NFT links to identity (DID, MCP, Email, Trust Registry), **PublicComms** (three per-channel `neuron-topic` service objects — stdIn, stdOut, stdErr — each independently backed per 004 FR-T13), and **PrivateComms** (`neuron-p2p-exchange` with `peerID`/`protocol`/`topicRef` per 004 FR-T17 — how to exchange multiaddress via a topic channel).

### Blockchain and ledger compatibility

- **Ethereum / EVM**: The registry is a **smart contract** (EIP-8004); extended NFT (EIP-721) and registry live on-chain. Topics and p2pconns are provided as EIP-8004 services (their full definitions are in separate specs).
- **Hedera**: The registry is a **smart contract** (EIP-8004 model); same as Ethereum. Registry and NFT semantics exist for both Hedera and Ethereum. Topics and p2pconns are provided as EIP-8004 services; chain-specific details (e.g. HCS for topics) are in the dedicated topics/p2p specs.

### EIP-8004 service shape (registry)

The registry's extended NFT (or equivalent EIP-8004 payload) carries a **`services`** array. Each entry is a service object. **Mandatory services:** every registration MUST include three **`neuron-topic`** service objects (one per standard channel: stdIn, stdOut, stdErr — each independently backed, per 004 FR-T13) and one **`neuron-p2p-exchange`** service; other types (e.g. DID) are optional. **Spec 004 is the authoritative source** for the internal schema of `neuron-topic` and `neuron-p2p-exchange` services; this appendix summarizes the structure for registry integration context.

- **`neuron-topic`**: Public, DLT-traceable channels — **one service object per channel** (per 004 FR-T14). Each registration includes three `neuron-topic` entries: one for `stdIn`, one for `stdOut`, one for `stdErr`. Each channel is independently backed and MAY use a different transport (per 004 FR-T13). Fields (per 004 FR-T14): `type` (MUST, always `"neuron-topic"`), `name` (MUST, channel role), `version` (MUST, semver), `channel` (MUST, channel role), `transport` (MUST, backend kind), `anchor` (MUST, anchoring ledger), `config` (MUST, transport-specific), `endpoint` (SHOULD, compact Topic URI for backward compatibility).
- **`neuron-p2p-exchange`**: P2P connectivity; declares **how to exchange multiaddress** via a topic-based signaling protocol (per 004 FR-T17). Fields: `type` (MUST, always `"neuron-p2p-exchange"`), `name` (MUST, e.g. `"p2p"`), `version` (MUST, semver), `peerID` (MUST, libp2p PeerID from 002), `protocol` (MUST, exchange protocol ID), `topicRef` (MUST, cross-reference to a `neuron-topic` service `name` in the same agentURI — validated per 004 FR-T18). The actual multiaddress is NOT stored in the registry.
- **`DID`** (and other identity types): Identity endpoint; e.g. `type`, `name`, `endpoint`.
- **`neuron-validator`**: Validator attestation; optional. Declares the agent as a third-party validator per [Spec 010](../010-validation-framework/spec.md). Fields: `type` (MUST, always `"neuron-validator"`), `name` (MUST, service name), `version` (MUST, semver), `domains` (MUST, array of spec refs or domain tags), `verdictDelivery` (MUST, `"topic"`). Schema authority: Spec 010 FR-V12.

Example **`services`** array (informative):

```json
"services": [
  {
    "type": "neuron-topic",
    "name": "stdIn",
    "endpoint": "hcs://0.0.4515382",
    "version": "1.0.0",
    "channel": "stdIn",
    "transport": "hcs",
    "anchor": "hedera-mainnet",
    "config": {
      "network": "hedera-mainnet",
      "topicId": "0.0.4515382"
    }
  },
  {
    "type": "neuron-topic",
    "name": "stdOut",
    "endpoint": "kafka+ledger://kafka1.neuron.network:9092/neuron.agent.alice.stdout",
    "version": "1.0.0",
    "channel": "stdOut",
    "transport": "kafka",
    "anchor": "hedera-mainnet",
    "config": {
      "bootstrapServers": ["kafka1.neuron.network:9092"],
      "topicName": "neuron.agent.alice.stdout",
      "saslMechanism": "SCRAM-SHA-512",
      "anchoring": {
        "method": "hcs-hash-chain",
        "anchorTopicId": "0.0.9999999",
        "anchorNetwork": "hedera-mainnet",
        "interval": "every-batch"
      }
    }
  },
  {
    "type": "neuron-topic",
    "name": "stdErr",
    "endpoint": "hcs://0.0.4515383",
    "version": "1.0.0",
    "channel": "stdErr",
    "transport": "hcs",
    "anchor": "hedera-mainnet",
    "config": {
      "network": "hedera-mainnet",
      "topicId": "0.0.4515383"
    }
  },
  {
    "type": "neuron-p2p-exchange",
    "name": "p2p",
    "version": "1.0.0",
    "peerID": "12D3KooWA1b2c3D4e5F6g7H8i9J0kLmNoPqRsTuVwXyZ",
    "protocol": "/neuron/multiaddr-exchange/1.0.0",
    "topicRef": "stdIn"
  },
  {
    "name": "DID",
    "endpoint": "did:key:zQ3shP2mWsZYWgpKDXRRx8rBe6UaDQY4mJgZrm5KywKgjqiU9",
    "version": "v1"
  },
  {
    "type": "neuron-validator",
    "name": "validation",
    "version": "1.0.0",
    "domains": ["005-health"],
    "verdictDelivery": "topic"
  }
]
```

- **neuron-topic**: Structure is **one service object per channel** (per 004 FR-T14). Each `neuron-topic` entry represents one channel (e.g. `stdIn`, `stdOut`, `stdErr`) with its own transport. Different channels MAY use different transports (per 004 FR-T13) — for example, stdIn on HCS, stdOut on Kafka, stdErr on HCS. `config` per entry is transport-specific. Optional `anchoring` within `config` for DLT traceability when the transport is not itself on-chain.
- **neuron-p2p-exchange**: Carries **`peerID`** (the peer's libp2p PeerID), **`protocol`** (exchange protocol ID, e.g. `/neuron/multiaddr-exchange/1.0.0`), and **`topicRef`** (cross-reference to a `neuron-topic` service `name` in the same agentURI — validated per 004 FR-T18). The actual multiaddress is not stored in the registry; the requester contacts the peer on the referenced topic using the specified protocol to obtain it. Schema per 004 FR-T17.
- **Spec 004 (Topic System) is authoritative** for the internal schema of `neuron-topic` and `neuron-p2p-exchange` services. This appendix summarizes the registry integration shape; for the complete field definitions and validation rules, see 004 FR-T14 (topic schema), 004 FR-T17 (p2p exchange schema), and 004 FR-T18 (topicRef validation).

### Path-based protocol IDs and multi-service catalogs *(informative — added 2026-05-08)*

EIP-8004's `services[]` array is open: the `neuron-p2p-exchange` service carries one `protocol` (the multiaddr-exchange handshake), but a single registration MAY include any number of additional service entries each declaring their own protocol ID for different stream variants. Spec 008 FR-P05 already permits multiple `neuron-commerce` entries per agent. With Spec 008 FR-P33a (added 2026-05-08), each `neuron-commerce` entry's `connectionSetup` MAY advertise a `streams[]` catalog whose individual entries declare libp2p stream protocol IDs (literal or wildcard pattern, e.g. `/jetvision/filtered/*`).

The registry's role here is unchanged: 003 stores the agentURI JSON opaquely (DD-01) and provides `lookup()`. Validation of `streams[]` content is delegated to 008 (envelope) and to the consuming DApp spec (semantics) per Constitution Principle XII.

Below is an informative example of an agentURI that declares two DApp-level commerce services (ADS-B and Remote ID) on the same registration. Note the multiple distinct protocol IDs and the wildcard pattern in the ADS-B service — this is allowed even though the registry itself sees the entries as opaque JSON.

```json
"services": [
  { "type": "neuron-topic", "name": "stdIn", /* ... */ },
  { "type": "neuron-topic", "name": "stdOut", /* ... */ },
  { "type": "neuron-topic", "name": "stdErr", /* ... */ },
  {
    "type": "neuron-p2p-exchange",
    "name": "p2p",
    "version": "1.0.0",
    "peerID": "12D3KooW...",
    "protocol": "/neuron/multiaddr-exchange/1.0.0",
    "topicRef": "stdIn"
  },
  {
    "type": "neuron-commerce",
    "name": "adsb",
    "version": "1.0.0",
    "delivery": { "mode": "p2p", "serviceRef": "p2p" },
    "settlement": { "binding": "evm-escrow", "chainId": 296, "contract": "0x...", "token": "0x..." },
    "pricing": { "amount": "1", "currency": "USDC", "unit": "frame", "interval": "0" },
    "termsRef": "https://dapp.adsb.example/terms.json"
  },
  {
    "type": "neuron-commerce",
    "name": "remote-id",
    "version": "1.0.0",
    "delivery": { "mode": "p2p", "serviceRef": "p2p" },
    "settlement": { "binding": "hedera-native" },
    "pricing": { "amount": "1", "currency": "HBAR", "unit": "frame", "interval": "0" },
    "termsRef": "https://dapp.remoteid.example/terms.json"
  }
]
```

When the buyer negotiates `adsb`, the resulting `connectionSetup` (008 FR-P33a) MAY carry a stream catalog like:

```json
"streams": [
  { "name": "raw",      "protocolID": "/jetvision/raw/1.0.0",       "direction": "seller-initiates" },
  { "name": "filtered", "protocolID": "/jetvision/filtered/*",      "direction": "seller-initiates" },
  { "name": "status",   "protocolID": "/jetvision/status/1.0.0",    "direction": "buyer-initiates" }
]
```

The registry sees only the agentURI JSON. The wildcard `/jetvision/filtered/*` and the buyer-initiated `/jetvision/status/1.0.0` are interpreted by 008 (envelope) and 009 (libp2p binding) at runtime; their semantics — what altitude bands the wildcard parameter encodes, what queries `/jetvision/status` answers — are defined by the ADS-B DApp spec (016).

Frame format choice for the bytes carried inside each stream (opaque BEAST 0x1A for ADS-B; normalized canonical-JSON `RemoteIdFrame` for Remote ID) is a DApp decision per `docs/dapp-frame-format-precedent.md` (Constitution Principle XII). The registry treats stream payloads as opaque regardless of frame format.
