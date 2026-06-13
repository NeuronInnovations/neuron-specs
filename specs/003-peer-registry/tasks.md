# Tasks: Peer Registry (EIP-8004 Registration)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `003-peer-registry` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)
**Input**: Design documents from `/specs/003-peer-registry/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/registration.md, quickstart.md

**Tests**: Constitution IX mandates test-first development. Within each phase, test tasks (`*_test.go`) appear BEFORE implementation tasks. Constitution X applies: FR-R06 requires NeuronPrivateKey signing of registration transactions; `signing_test.go` verifies determinism.

**Organization**: Tasks are grouped by phase (setup, foundational, user stories, polish). User stories map to spec.md scenarios. Every task traces to FR-R* or SC-R* per Constitution VII.

## Format: `- [x] T### [P?] [US#?] Description with exact Go file path (FR-XXX, SC-XXX)`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[US#]**: Which user story this task belongs to (US1, US2, US3)
- Every task references the FR-R* or SC-R* it satisfies (Constitution VII)
- Every task includes the exact Go file path in `internal/registry/`

## Path Conventions

- **SDK module**: `internal/registry/` within the neuron-go-sdk module (Constitution VI: Go-first)
- **Contract bindings**: `internal/registry/contract/` (abigen-generated, not hand-maintained)
- **Tests**: Colocated as `*_test.go` per Go conventions

## Dependencies (Cross-Spec)

- **Spec 002** (`internal/keylib`): NeuronPrivateKey (signing), NeuronPublicKey, EVMAddress(), PeerID(), DIDKey()
- **Spec 001** (`internal/account`): Child identity, RegistryBinding, PaymentAddress, parent-child link
- **Spec 004** (`internal/topic`): neuron-topic service schema (FR-T14), neuron-p2p-exchange schema (FR-T17), topicRef validation (FR-T18), TopicAdapter interface
- **External**: `github.com/ethereum/go-ethereum` (ethclient, abigen, accounts/abi/bind, common)

---

## Phase 1: Setup (Go Module Structure, ABI Generation)

**Purpose**: Initialize the Go module structure, install dependencies, generate abigen contract bindings, and create the project skeleton that all subsequent phases depend on.

- [x] T001 Create `internal/registry/` package directory structure per plan.md: `registration.go`, `registration_test.go`, `service.go`, `service_test.go`, `agent_uri.go`, `agent_uri_test.go`, `validation.go`, `validation_test.go`, `resolver.go`, `resolver_test.go`, `errors.go`, `errors_test.go`, `signing_test.go`, `integration_test.go` (FR-R01, FR-R02, FR-R03, FR-R04, FR-R05, FR-R06, FR-R07, FR-R08)
- [x] T002 [P] Create `internal/registry/contract/` directory with EIP-8004 Identity Registry ABI file at `internal/registry/contract/identity_registry.abi` sourced from EIP-8004 Solidity compilation (FR-R01)
- [x] T003 [P] Generate abigen Go bindings at `internal/registry/contract/identity_registry.go` from `identity_registry.abi` using `abigen --abi=identity_registry.abi --pkg=contract --out=identity_registry.go`; verify bindings expose `register()`, `updateAgentURI()`, `ownerOf()`, `tokenOfOwnerByIndex()`, `approve()` (FR-R01, FR-R06, FR-R10, FR-R11c)
- [x] T004 [P] Add Go module dependencies: `github.com/ethereum/go-ethereum` (ethclient, abigen, accounts/abi/bind, common), `github.com/stretchr/testify` (assertions); verify `go mod tidy` succeeds (FR-R01)

---

## Phase 2: Foundational (EIP-8004 Types, Base Interfaces, Error Types)

**Purpose**: Define all shared types, error kinds, role enum, and the contract abstraction layer that every user story depends on. MUST complete before any user story work begins.

**CRITICAL**: No user story work can begin until this phase is complete.

### Tests (Constitution IX)

- [x] T005 Write error type tests in `internal/registry/errors_test.go`: verify each error kind (`RegistryUnavailable`, `RegistrationNotFound`, `IncompleteRegistration`, `ProofOfControlFailed`, `AdmissionDenied`, `DuplicateRegistration`, `InvalidDIDService`, `BrokenTopicRef`, `InvalidServiceSchema`, `UnauthorizedOperation`, `AllowlistRejection`) has correct string representation, implements the `error` interface, and carries detail fields (FR-R07, FR-R05, FR-R06, FR-R08, FR-R09, FR-R14, FR-R02, FR-R03, FR-R11, FR-R12d)
- [x] T006 [P] Write service type tests in `internal/registry/service_test.go`: verify `NeuronTopicService`, `NeuronP2PExchangeService`, and `DIDService` struct construction with all MUST fields; verify zero-value structs are invalid; verify JSON tags match 004 FR-T14 and 004 FR-T17 field names (FR-R02, FR-R03, FR-R13, FR-R14)
- [x] T007 [P] Write AgentURI type tests in `internal/registry/agent_uri_test.go`: verify `AgentURI` struct construction with empty services array; verify JSON serialization produces `{"services":[...]}` structure; verify round-trip JSON marshal/unmarshal (FR-R08)

### Implementation

- [x] T008 Implement structured error types in `internal/registry/errors.go`: `RegistryUnavailable`, `RegistrationNotFound`, `IncompleteRegistration`, `ProofOfControlFailed`, `AdmissionDenied`, `DuplicateRegistration`, `InvalidDIDService`, `BrokenTopicRef`, `InvalidServiceSchema`, `UnauthorizedOperation`, `AllowlistRejection`; each with `Error() string` method and optional detail fields (FR-R07, FR-R05, FR-R06, FR-R08, FR-R09, FR-R14, FR-R02, FR-R03, FR-R11, FR-R12d)
- [x] T009 [P] Implement `RegistryRole` enum in `internal/registry/roles.go`: `REGISTRY_ADMIN`, `REGISTERED_AGENT`, `DELEGATED_OPERATOR`, `PARENT` with string representation and documentation per FR-R11a/b/c/d (FR-R11)
- [x] T010 [P] Implement `NeuronTopicService` struct in `internal/registry/service.go` with fields: `Type` (string), `Name` (string), `Version` (string), `Channel` (string), `Transport` (string), `Anchor` (string), `Config` (map[string]any), `Endpoint` (string, optional); JSON tags per 004 FR-T14 field names (FR-R02)
- [x] T011 [P] Implement `NeuronP2PExchangeService` struct in `internal/registry/service.go` with fields: `Type` (string), `Name` (string), `Version` (string), `PeerID` (string), `Protocol` (string), `TopicRef` (string); JSON tags per 004 FR-T17 field names (FR-R03)
- [x] T012 [P] Implement `DIDService` struct in `internal/registry/service.go` with fields: `Name` (string), `Endpoint` (string), `Version` (string); JSON tags per FR-R13/FR-R14 (FR-R13, FR-R14)
- [x] T013 Implement `AgentURI` struct in `internal/registry/agent_uri.go` with `Services` field (slice of service objects); implement JSON marshaling/unmarshaling that handles heterogeneous service types (neuron-topic, neuron-p2p-exchange, DID) via `type` discriminator (FR-R08, FR-R02, FR-R03)
- [x] T014 [P] Implement `Registration` struct in `internal/registry/registration.go` with fields: `RegistryAddress` (EVMAddress), `ChildAddress` (EVMAddress), `TokenId` (*big.Int), `AgentURI` (AgentURI), `ChainId` (uint64) per data-model.md (FR-R01, FR-R05, FR-R06, FR-R10)
- [x] T015 [P] Implement `RegistrationResult` struct in `internal/registry/registration.go` with fields: `TokenId` (*big.Int), `TransactionHash` (string), `ChildAddress` (EVMAddress), `RegistryAddress` (EVMAddress), `ChainId` (uint64), `AgentURI` (string) per data-model.md (FR-R01, FR-R06, FR-R10)
- [x] T016 [P] Implement `AdmissionPolicy` struct in `internal/registry/validation.go` with fields: `PolicyType` (string: "permissionless" | "permissioned"), `TrustAnchor` (string), `Allowlist` ([]string) per data-model.md (FR-R09, FR-R12)
- [x] T017 [P] Implement `LookupKey` sum type in `internal/registry/resolver.go`: `ByEVMAddress(EVMAddress)` and `ByExternalID(string)` variants for registration lookup (FR-R04, SC-R01)
- [x] T018 Define `RegistryContract` interface in `internal/registry/contract_iface.go` abstracting the abigen-generated bindings: `Register(opts, agentURI) (*types.Transaction, error)`, `UpdateAgentURI(opts, tokenId, agentURI) (*types.Transaction, error)`, `OwnerOf(opts, tokenId) (common.Address, error)`, `TokenOfOwnerByIndex(opts, owner, index) (*big.Int, error)`, `AgentURI(opts, tokenId) (string, error)` for testability (FR-R01, FR-R06)

**Checkpoint**: Foundation ready -- all shared types, errors, role enum, and contract interface defined. User story implementation can now begin.

---

## Phase 3: US1 -- Register Child as EIP-8004 Extended NFT (Priority: P1)

**Goal**: A developer can register a Child account in an EIP-8004 registry, mint an extended NFT (ERC-721), and look up the registration by EVM address. Registration operations (create, update, revoke) are signed by the Child's NeuronPrivateKey.

**Independent Test**: Create a Child identity (from 002/001), build an AgentURI with mandatory services, call `Register()`, verify NFT minted with Child as owner, verify `LookupRegistration()` resolves the registration. Covers SC-R01.

**Acceptance Scenarios**: US1-S1 (register and resolve by EVM address), US1-S2 (lookup returns NFT and services).

### Tests (Constitution IX)

- [x] T019 [US1] Write registration lifecycle tests in `internal/registry/registration_test.go`: test `Register()` with valid AgentURI creates registration and returns `RegistrationResult` with correct `tokenId`, `childAddress`, `registryAddress`, `chainId`; test `Register()` rejects incomplete AgentURI with `IncompleteRegistration` error; test `Register()` rejects duplicate registration with `DuplicateRegistration` error; test ownership verification (`ownerOf` == `childAddress`); test registry unavailable returns `RegistryUnavailable` error (FR-R01, FR-R05, FR-R06, FR-R07, FR-R08, FR-R10, SC-R01)
- [x] T020 [US1] Write update registration tests in `internal/registry/registration_test.go`: test `UpdateRegistration()` with valid new AgentURI succeeds for owner; test `UpdateRegistration()` rejects non-owner with `UnauthorizedOperation`; test `UpdateRegistration()` rejects invalid AgentURI with `IncompleteRegistration` (FR-R06, FR-R08, FR-R11b)
- [x] T021 [US1] Write revoke registration tests in `internal/registry/registration_test.go`: test `RevokeRegistration()` succeeds for owner and returns transaction hash; test `RevokeRegistration()` rejects non-owner with `UnauthorizedOperation`; test `RevokeRegistration()` on non-existent token returns `RegistrationNotFound` (FR-R06, FR-R11a, FR-R11b)
- [x] T022 [US1] Write proof-of-control tests in `internal/registry/registration_test.go`: verify registration transaction is signed by Child's NeuronPrivateKey (signer address matches `childAddress`); verify non-matching signer produces `ProofOfControlFailed` error (FR-R06)

### Implementation

- [x] T023 [US1] Implement `Register(childKey NeuronPrivateKey, registryAddress EVMAddress, chainId uint64, agentURI AgentURI, client EthClient) (RegistrationResult, error)` in `internal/registry/registration.go`: derive `childAddress` from `childKey`, validate agentURI completeness, check no duplicate registration, serialize agentURI to JSON, build and sign `register()` transaction, submit to chain, verify `ownerOf(tokenId)` == `childAddress`, return `RegistrationResult` per contracts/registration.md (FR-R01, FR-R05, FR-R06, FR-R08, FR-R10, SC-R01)
- [x] T024 [US1] Implement `UpdateRegistration(childKey NeuronPrivateKey, registryAddress EVMAddress, chainId uint64, tokenId *big.Int, newAgentURI AgentURI, client EthClient) (RegistrationResult, error)` in `internal/registry/registration.go`: verify caller is owner or approved operator, validate new agentURI completeness, build and sign `updateAgentURI()` transaction, return updated `RegistrationResult` per contracts/registration.md (FR-R06, FR-R08, FR-R11b, FR-R11c)
- [x] T025 [US1] Implement `RevokeRegistration(childKey NeuronPrivateKey, registryAddress EVMAddress, chainId uint64, tokenId *big.Int, client EthClient) (string, error)` in `internal/registry/registration.go`: verify caller is owner, build and sign revocation transaction (burn/deregister), return transaction hash per contracts/registration.md (FR-R06, FR-R11b)

**Checkpoint**: US1 complete -- register, update, and revoke lifecycle works end-to-end. SC-R01 is verifiable.

---

## Phase 4: US2 -- Public Comms (Unified Topic System) as EIP-8004 Services (Priority: P1)

**Goal**: A developer can attach three mandatory `neuron-topic` service objects (stdIn, stdOut, stdErr) to a registration's AgentURI, construct a validated AgentURI, resolve topic services from a looked-up registration, and confirm service completeness. Different channels MAY use different transports.

**Independent Test**: Build three `NeuronTopicService` objects (one per channel, with different transports -- e.g. stdIn on HCS, stdOut on Kafka, stdErr on HCS), construct AgentURI, validate completeness, register, lookup, resolve topics. Covers SC-R02.

**Acceptance Scenarios**: US2-S1 (attach neuron-topic services, resolvable from NFT), US2-S2 (different backends via unified topic adapter).

### Tests (Constitution IX)

- [x] T026 [US2] Write `NeuronTopicService` builder tests in `internal/registry/service_test.go`: test construction with all MUST fields (`type`, `name`, `version`, `channel`, `transport`, `anchor`, `config`); test optional `endpoint` field; test each standard channel role (`stdIn`, `stdOut`, `stdErr`); test invalid channel name rejected; test missing MUST field rejected with `InvalidServiceSchema` (FR-R02)
- [x] T027 [US2] Write AgentURI construction tests in `internal/registry/agent_uri_test.go`: test construction with three neuron-topic services (stdIn, stdOut, stdErr) + one neuron-p2p-exchange; test JSON serialization with correct service ordering (neuron-topic first, then neuron-p2p-exchange, then optional); test round-trip JSON marshal/unmarshal preserves all fields (FR-R08, FR-R02, FR-R03)
- [x] T028 [US2] Write AgentURI completeness validation tests in `internal/registry/validation_test.go`: test V-REG-01 (exactly 3 neuron-topic), V-REG-02 (at least 1 neuron-p2p-exchange), V-REG-11 (each standard channel appears exactly once); test missing `stdOut` returns `IncompleteRegistration`; test duplicate `stdIn` returns `IncompleteRegistration`; test mixed transports (stdIn on HCS, stdOut on Kafka, stdErr on HCS) is valid (FR-R08, FR-R02)
- [x] T029 [US2] Write service schema validation tests in `internal/registry/validation_test.go`: test V-REG-03 (neuron-topic MUST fields), V-REG-04 (neuron-p2p-exchange MUST fields); test each missing field produces specific `InvalidServiceSchema` error message (FR-R02, FR-R03)
- [x] T030 [US2] Write topic resolution tests in `internal/registry/resolver_test.go`: test `ResolveTopics()` extracts stdIn, stdOut, stdErr from a valid registration; test `ResolveTopics()` returns `IncompleteRegistration` when a channel is missing; test channels with different transports are independently resolved (FR-R02, FR-R08, SC-R02)
- [x] T031 [US2] Write lookup tests in `internal/registry/resolver_test.go`: test `LookupRegistration()` by EVM address returns Registration with correct fields; test `LookupRegistration()` by external ID (registry binding) resolves correctly; test lookup on non-existent address returns `RegistrationNotFound`; test lookup on unavailable registry returns `RegistryUnavailable` (FR-R04, FR-R07, SC-R01)

### Implementation

- [x] T032 [US2] Implement `NewNeuronTopicService(opts ...TopicServiceOption) (NeuronTopicService, error)` builder function in `internal/registry/service.go`: set `type` to `"neuron-topic"`, validate all MUST fields (`name`, `version`, `channel`, `transport`, `anchor`, `config`), validate `channel` is a valid channel role, return constructed service or `InvalidServiceSchema` error (FR-R02)
- [x] T033 [US2] Implement `NewAgentURI(topicServices []NeuronTopicService, p2pService NeuronP2PExchangeService, didService *DIDService) (AgentURI, error)` constructor in `internal/registry/agent_uri.go`: accept three `NeuronTopicService` objects + one `NeuronP2PExchangeService` + optional `DIDService`, assemble into services array with canonical ordering (FR-R08)
- [x] T034 [US2] Implement `ValidateRegistrationCompleteness(agentURI AgentURI, childPublicKey NeuronPublicKey) (bool, []ValidationError)` in `internal/registry/validation.go`: execute validation rules V-REG-01 through V-REG-12 per contracts/registration.md; return `(valid bool, errors []ValidationError)` (FR-R08, FR-R02, FR-R03, FR-R14)
- [x] T035 [US2] Implement `LookupRegistration(registryAddress EVMAddress, chainId uint64, lookupKey LookupKey, client EthClient) (Registration, error)` in `internal/registry/resolver.go`: handle `ByEVMAddress` (call `tokenOfOwnerByIndex` then `agentURI`) and `ByExternalID` (resolve to EVMAddress then proceed); parse agentURI JSON into `AgentURI` struct; return `Registration` or error per contracts/registration.md (FR-R04, FR-R07, SC-R01)
- [x] T036 [US2] Implement `ResolveTopics(registration Registration) (stdIn NeuronTopicService, stdOut NeuronTopicService, stdErr NeuronTopicService, error)` in `internal/registry/resolver.go`: filter services for `type=="neuron-topic"`, identify stdIn/stdOut/stdErr by `channel` field, return three `NeuronTopicService` objects or `IncompleteRegistration` error (FR-R02, FR-R08, SC-R02)

**Checkpoint**: US2 complete -- topic services attach, validate, and resolve correctly with mixed transports. SC-R02 is verifiable.

---

## Phase 5: US3 -- Private Comms (Topic-Based Multiaddress Exchange) (Priority: P2)

**Goal**: A developer can attach a mandatory `neuron-p2p-exchange` service to a registration, resolve the peer's PeerID/protocol/topicRef, and validate the topicRef cross-reference against existing `neuron-topic` service names. The actual multiaddress is NOT stored in the registry; it is exchanged at runtime over the referenced topic.

**Independent Test**: Build `NeuronP2PExchangeService` with peerID/protocol/topicRef, validate topicRef resolves to an existing neuron-topic service name, register, lookup, resolve P2P exchange info. Covers SC-R03.

**Acceptance Scenarios**: US3-S1 (read peerID, protocol, topicRef), US3-S2 (topicRef validation -- broken reference produces `BrokenTopicRef` error), US3-S3 (multiaddress exchange over referenced topic).

### Tests (Constitution IX)

- [x] T037 [US3] Write `NeuronP2PExchangeService` builder tests in `internal/registry/service_test.go`: test construction with all MUST fields (`type`, `name`, `version`, `peerID`, `protocol`, `topicRef`); test missing `peerID` rejected with `InvalidServiceSchema`; test missing `protocol` rejected; test `peerID` matches Child's PeerID (V-REG-12) (FR-R03)
- [x] T038 [US3] Write topicRef cross-validation tests in `internal/registry/validation_test.go`: test V-REG-05 (topicRef `"stdIn"` matches existing neuron-topic service name); test broken topicRef `"nonExistent"` produces `BrokenTopicRef` error; test topicRef is case-sensitive (FR-R03)
- [x] T039 [US3] Write P2P exchange resolution tests in `internal/registry/resolver_test.go`: test `ResolveP2PExchange()` returns `NeuronP2PExchangeService` with correct `peerID`, `protocol`, `topicRef`; test `ResolveP2PExchange()` returns `IncompleteRegistration` when no neuron-p2p-exchange service exists; test `ResolveP2PExchange()` validates topicRef against neuron-topic services and returns `BrokenTopicRef` if invalid (FR-R03, SC-R03)

### Implementation

- [x] T040 [US3] Implement `NewNeuronP2PExchangeService(opts ...P2PServiceOption) (NeuronP2PExchangeService, error)` builder function in `internal/registry/service.go`: set `type` to `"neuron-p2p-exchange"`, validate all MUST fields (`name`, `version`, `peerID`, `protocol`, `topicRef`), return constructed service or `InvalidServiceSchema` error (FR-R03)
- [x] T041 [US3] Implement topicRef cross-validation in `internal/registry/validation.go` (within `ValidateRegistrationCompleteness`): verify `topicRef` value matches the `name` field of an existing `neuron-topic` service in the same AgentURI; produce `BrokenTopicRef` error on mismatch (FR-R03)
- [x] T042 [US3] Implement `ResolveP2PExchange(registration Registration) (NeuronP2PExchangeService, error)` in `internal/registry/resolver.go`: filter services for `type=="neuron-p2p-exchange"`, return first matching service, validate topicRef references existing neuron-topic service name, return `IncompleteRegistration` if not found or `BrokenTopicRef` if invalid (FR-R03, SC-R03)
- [x] T043 [US3] Implement peerID cross-validation in `internal/registry/validation.go`: verify neuron-p2p-exchange `peerID` matches `childPublicKey.PeerID()` (V-REG-12) (FR-R03)

**Checkpoint**: US3 complete -- P2P exchange service attaches, validates (including topicRef cross-reference), and resolves correctly. SC-R03 is verifiable.

---

## Phase 6: Polish (Admission Policy, DID Service, Deterministic Signing, Role Boundaries, Integration)

**Purpose**: Complete remaining cross-cutting concerns: admission policy interface, DID service support (FR-R13, FR-R14), deterministic signing verification (Constitution X), role boundary enforcement (FR-R11), allowlist behavior (FR-R12), and full end-to-end integration test with EIP-8004 contract. Validates all three success criteria (SC-R01, SC-R02, SC-R03).

### Tests (Constitution IX)

- [x] T044 Write DID service tests in `internal/registry/service_test.go`: test `NewDIDService()` with valid `did:key:zQ3s...` endpoint derived from Child's NeuronPublicKey; test DID not matching Child's key produces `InvalidDIDService`; test DID using Parent's key rejected; test multiple DID services rejected (V-REG-06, V-REG-07); test DID is optional -- registration is complete without it per FR-R08 (FR-R13, FR-R14)
- [x] T045 [P] Write admission policy tests in `internal/registry/validation_test.go`: test permissionless policy allows any entity with proof-of-control; test permissioned policy with allowlist rejects Child whose Parent DID is not listed (`AllowlistRejection`); test adding Parent DID to allowlist makes Children eligible; test removing Parent DID does NOT auto-revoke existing registrations; test only Registry Administrator can modify allowlist (FR-R09, FR-R12)
- [x] T046 [P] Write role boundary enforcement tests in `internal/registry/validation_test.go`: test Registry Administrator MUST NOT mint/modify/transfer (FR-R11a); test Registered Agent can update own agentURI but MUST NOT modify another's (FR-R11b); test Delegated Operator can update on approved tokens but MUST NOT call `register()` (FR-R11c); test Parent MUST NOT call `register()` on behalf of Child (FR-R11d); test NFT ownership at mint time is Child's EVMAddress not Parent (FR-R10)
- [x] T047 Write deterministic signing verification tests in `internal/registry/signing_test.go`: given the same Child NeuronPrivateKey and the same registration payload, verify two independent calls to sign the `register()` transaction produce identical signatures (RFC 6979 determinism per Constitution X); verify signing of `updateAgentURI()` is also deterministic; verify signing of revocation is deterministic (FR-R06)
- [x] T048 Write full end-to-end integration test in `internal/registry/integration_test.go`: test complete lifecycle -- (1) create Child identity (from 002), (2) build AgentURI with 3 neuron-topic services (stdIn/HCS, stdOut/Kafka, stdErr/HCS) + 1 neuron-p2p-exchange + optional DID service, (3) call `Register()`, (4) `LookupRegistration()` by EVM address, (5) `ResolveTopics()` returns stdIn/stdOut/stdErr with correct transports, (6) `ResolveP2PExchange()` returns peerID/protocol/topicRef, (7) `UpdateRegistration()` with new agentURI, (8) verify updated services, (9) `RevokeRegistration()`, (10) verify `RegistrationNotFound` after revocation (SC-R01, SC-R02, SC-R03)

### Implementation

- [x] T049 Implement `NewDIDService(childPublicKey NeuronPublicKey) (DIDService, error)` builder function in `internal/registry/service.go`: derive `did:key` from Child's NeuronPublicKey using secp256k1 multicodec `0xe7` (base58btc prefix `zQ3s`), set `name` to `"DID"`, return constructed `DIDService` (FR-R13, FR-R14)
- [x] T050 Implement DID validation in `internal/registry/validation.go` (within `ValidateRegistrationCompleteness`): check V-REG-06 (DID matches Child's NeuronPublicKey), V-REG-07 (at most one DID service); reject with `InvalidDIDService` on failure (FR-R14)
- [x] T051 [P] Implement `AdmissionPolicy` interface in `internal/registry/validation.go`: define `IsAdmitted(childAddress EVMAddress, parentDID string) (bool, error)` method; implement `PermissionlessPolicy` (always admits) and `AllowlistPolicy` (checks Parent DID against allowlist) (FR-R09, FR-R12)
- [x] T052 [P] Implement role boundary checks in `internal/registry/validation.go`: `ValidateRoleBoundary(callerRole RegistryRole, operation string) error` that enforces FR-R11a/b/c/d constraints; return `UnauthorizedOperation` on violation (FR-R11)
- [x] T053 Implement allowlist behavior in `internal/registry/validation.go`: `AllowlistPolicy.AddParentDID(did string)`, `AllowlistPolicy.RemoveParentDID(did string)` with semantics per FR-R12 -- adding makes Children eligible, removing rejects new registrations but does NOT revoke existing (FR-R12)

**Checkpoint**: All phases complete -- full registration lifecycle with validation, resolution, admission policy, DID service, role boundaries, and deterministic signing verified. SC-R01, SC-R02, SC-R03 all pass.

---

## Dependencies & Execution Order

### Phase Graph

```
Phase 1: Setup (T001-T004)
  |
  v
Phase 2: Foundational (T005-T018) ---- BLOCKS all user stories
  |
  +--> Phase 3: US1 (T019-T025) -- Register/Update/Revoke lifecycle
  |      |
  |      +--> Phase 4: US2 (T026-T036) -- Topic services, AgentURI, resolution
  |             |                          [depends on US1 for Register()]
  |             |
  |             +--> Phase 5: US3 (T037-T043) -- P2P exchange, topicRef validation
  |                    |                          [depends on US2 for AgentURI + validation]
  |                    |
  v                    v
Phase 6: Polish (T044-T053) ---- depends on US1 + US2 + US3
```

### Cross-Spec Blocking Dependencies

| Dependency | Source | Required For | Status |
|-----------|--------|-------------|--------|
| `NeuronPrivateKey`, `NeuronPublicKey`, `EVMAddress()`, `PeerID()`, `DIDKey()` | Spec 002 (`internal/keylib`) | All phases (signing, identity derivation) | Required |
| Child identity, RegistryBinding | Spec 001 (`internal/account`) | US1 (register), US2 (lookup by binding) | Required |
| `neuron-topic` schema (FR-T14), `neuron-p2p-exchange` schema (FR-T17), topicRef validation (FR-T18) | Spec 004 (`internal/topic`) | US2 (topic services), US3 (P2P exchange) | Required |
| TopicAdapter interface | Spec 004 | Integration test (end-to-end resolution) | Optional for unit tests |

### Parallel Opportunities

| Tasks | Reason |
|-------|--------|
| T002, T003, T004 | Phase 1 Setup: different concerns (ABI file, abigen generation, go mod) |
| T005, T006, T007 | Phase 2 tests: different test files (errors_test, service_test, agent_uri_test) |
| T009, T010, T011, T012, T014, T015, T016, T017 | Phase 2 impl: different files, no cross-deps |
| T026, T027 (US2 tests) | Different test files (service_test vs agent_uri_test) |
| T037, T038, T039 (US3 tests) | Different test files (service_test vs validation_test vs resolver_test) |
| T044, T045, T046 (Polish tests) | Different concerns (DID, admission, roles) |
| T051, T052 (Polish impl) | Different concerns (admission policy vs role boundaries) |

### Within Each Phase

1. Test tasks (`*_test.go`) BEFORE implementation tasks (Constitution IX)
2. Shared types/structs before functions that use them
3. Validation before orchestration
4. Core implementation before integration verification

### Implementation Strategy

#### MVP First (US1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T018)
3. Complete Phase 3: US1 -- Register Child (T019-T025)
4. **STOP and VALIDATE**: Register a Child, verify NFT minted, verify lookup by EVM address. SC-R01 passes.

#### Incremental Delivery

1. Setup + Foundational --> Foundation ready
2. Add US1 (Register) --> Validate SC-R01 --> **MVP!**
3. Add US2 (Topics) --> Validate SC-R02 --> Public comms resolvable
4. Add US3 (P2P Exchange) --> Validate SC-R03 --> Private comms discoverable
5. Add Polish (DID, admission, signing, integration) --> All SCs pass --> Production-ready registry
6. **STOP and VALIDATE**: Full integration test passes end-to-end

#### Parallel Team Strategy

With 2 developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1 (registration lifecycle) --> then US3 (P2P exchange)
   - Developer B: US2 (topic services + resolution) --> then Polish (DID, admission)
3. Integration test after US1 + US2 + US3 converge
4. Signing test + role boundaries as final tasks

---

## Task Count Summary

| Phase | Tasks | Test Tasks | Impl Tasks |
|-------|-------|-----------|-----------|
| Phase 1: Setup | T001-T004 (4) | 0 | 4 |
| Phase 2: Foundational | T005-T018 (14) | 3 | 11 |
| Phase 3: US1 | T019-T025 (7) | 4 | 3 |
| Phase 4: US2 | T026-T036 (11) | 6 | 5 |
| Phase 5: US3 | T037-T043 (7) | 3 | 4 |
| Phase 6: Polish | T044-T053 (10) | 5 | 5 |
| **Total** | **53** | **21** | **32** |

---

## FR/SC Traceability

| Requirement | Tasks |
|-------------|-------|
| FR-R01 (Registry as smart contract) | T001, T002, T003, T008, T014, T015, T018, T023 |
| FR-R02 (neuron-topic mandatory, per-channel) | T006, T008, T010, T026, T027, T028, T029, T032, T034, T036 |
| FR-R03 (neuron-p2p-exchange mandatory) | T006, T008, T011, T029, T037, T038, T040, T041, T042, T043 |
| FR-R04 (Lookup by EVM address or binding) | T017, T031, T035 |
| FR-R05 (One per registry) | T005, T008, T014, T019, T023 |
| FR-R06 (Proof-of-control, NeuronPrivateKey signing) | T005, T008, T019, T020, T021, T022, T023, T024, T025, T047 |
| FR-R07 (Documented error on failure) | T005, T008, T031, T035 |
| FR-R08 (Complete registration) | T007, T013, T027, T028, T029, T033, T034 |
| FR-R09 (Admission policy) | T005, T008, T016, T045, T051 |
| FR-R10 (Child owns NFT) | T003, T014, T015, T019, T023, T046 |
| FR-R11 (Role boundaries) | T005, T008, T009, T020, T021, T024, T025, T046, T052 |
| FR-R12 (Allowlist behavior) | T005, T008, T016, T045, T051, T053 |
| FR-R13 (DID service optional) | T006, T012, T044, T049 |
| FR-R14 (DID validation) | T005, T006, T008, T012, T034, T044, T049, T050 |
| SC-R01 (Register + lookup) | T019, T023, T031, T035, T048 |
| SC-R02 (Topic resolution) | T030, T036, T048 |
| SC-R03 (P2P exchange resolution) | T039, T042, T048 |

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [US#] label maps task to specific user story for traceability
- Constitution IX: test tasks appear BEFORE implementation in every phase
- Constitution X: T047 (`signing_test.go`) verifies deterministic signing for registration transactions
- Constitution VII: every task traces to at least one FR-R* or SC-R* requirement
- Constitution VI: all file paths use Go layout with `internal/registry/` and colocated `*_test.go`
- Constitution VIII: HCS is a primary transport for neuron-topic services; integration test (T048) validates HCS-backed topic channels
- abigen-generated code in `internal/registry/contract/` is NOT hand-maintained (T002, T003)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
