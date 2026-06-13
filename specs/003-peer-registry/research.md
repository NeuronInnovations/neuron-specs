# Research: Peer Registry (EIP-8004 Registration)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `003-peer-registry` | **Date**: 2026-02-25 | **Source**: spec.md

---

## R1: Go Smart Contract Interaction Library

**Decision**: Use `github.com/ethereum/go-ethereum` (`ethclient` + `abigen` + `accounts/abi/bind`) for all EIP-8004 smart contract interactions.

**Rationale**: geth provides production-grade EVM smart contract interaction in Go. `ethclient` handles JSON-RPC communication with Ethereum and EVM-compatible chains (including Hedera EVM). `abigen` generates type-safe Go bindings from Solidity ABI, eliminating manual ABI encoding/decoding. `accounts/abi/bind` provides transaction management (signing, gas estimation, nonce management). This stack is the de facto standard for Go-based smart contract interaction and directly supports the operations required by FR-R01 (registry as smart contract), FR-R06 (proof-of-control via signed transactions), and FR-R10 (NFT ownership queries).

**Alternatives considered**:
- `github.com/umbracle/ethgo`: Lighter alternative, but less ecosystem support and no equivalent to `abigen`.
- Raw JSON-RPC via `net/http`: Maximum control but requires manual ABI encoding, nonce management, and receipt polling. Error-prone for contract-heavy workflows.
- Hedera SDK (`github.com/hashgraph/hedera-sdk-go`): Would be needed for Hedera-native HTS operations, but EIP-8004 on Hedera uses the EVM layer, so geth's `ethclient` connecting to Hedera's JSON-RPC relay is sufficient.

---

## R2: EIP-8004 Contract ABI Generation

**Decision**: Use `abigen` (part of go-ethereum tools) to generate Go bindings from the EIP-8004 Identity Registry Solidity ABI. Generated bindings live in `internal/registry/contract/`.

**Rationale**: `abigen` produces type-safe Go structs and methods for every contract function and event. This gives compile-time safety for `register()`, `updateAgentURI()`, `ownerOf()`, and other EIP-8004 + ERC-721 functions. The generated code handles ABI encoding/decoding, event log parsing, and transaction receipt processing. FR-R01 requires the registry to be a smart contract; `abigen` is the standard Go approach for interacting with smart contracts.

**Generation command**:
```bash
abigen --abi=identity_registry.abi --pkg=contract --out=identity_registry.go
```

**Key contract functions exposed** (from EIP-8004 + ERC-721):
- `register(agentURI string)` — mints NFT with auto-incrementing agentId; caller becomes owner (FR-R01, FR-R06, FR-R10)
- `updateAgentURI(tokenId uint256, agentURI string)` — update registration (owner or approved only)
- `agentURI(tokenId uint256) → string` — read agentURI for a token
- `ownerOf(tokenId uint256) → address` — ERC-721 ownership check (FR-R10)
- `approve(address, tokenId)` — ERC-721 operator approval (FR-R11c)
- `tokenOfOwnerByIndex(owner, index)` — enumerate a Child's tokens (for lookup by EVM address)

**Alternatives considered**:
- Hand-crafted ABI encoding: Fragile, no compile-time type safety. Rejected.
- `typechain` (TypeScript-based): Wrong language ecosystem for Go SDK.

---

## R3: ERC-721 NFT Integration

**Decision**: Use the ERC-721 interface methods already included in the EIP-8004 contract binding (generated via `abigen`). No separate ERC-721 library required.

**Rationale**: EIP-8004 extends ERC-721. The generated `abigen` bindings include all ERC-721 methods (`ownerOf`, `approve`, `setApprovalForAll`, `transferFrom`, `balanceOf`, `tokenOfOwnerByIndex`). FR-R10 requires the Child's EVMAddress to own the NFT. FR-R11 requires role boundaries (owner, approved operator, administrator). All of these are standard ERC-721 operations exposed through the generated binding.

**Key ERC-721 operations used**:
- `ownerOf(tokenId)` — verify NFT ownership is the Child's EVMAddress (FR-R10)
- `approve(to, tokenId)` — delegate operations to an operator (FR-R11c)
- `setApprovalForAll(operator, approved)` — blanket approval for all tokens
- `tokenOfOwnerByIndex(owner, 0)` — find the Child's token in a given registry (FR-R05: at most one per registry)

**Alternatives considered**:
- `github.com/ethereum/go-ethereum/common` + manual ERC-721 calls: Possible but redundant when `abigen` generates them.
- OpenZeppelin Go client: Does not exist; OpenZeppelin is Solidity/JS only.

---

## R4: Service Schema Validation

**Decision**: Use Go struct validation with field tags and custom validation functions. No external JSON Schema library.

**Rationale**: Service objects (`neuron-topic`, `neuron-p2p-exchange`, `DID`) have well-defined fields with MUST/SHOULD/MAY requirements (per 004 FR-T14, 004 FR-T17, FR-R13/R14). Go structs with validation methods provide compile-time type safety for field presence and type correctness. Custom validation functions check cross-field constraints (e.g. `topicRef` must reference an existing `neuron-topic` service name per 004 FR-T18). This approach is consistent with the validation patterns in `internal/account/` (001) and `internal/keylib/` (002).

**Validation rules**:
- `NeuronTopicService`: `type` MUST be `"neuron-topic"`, `name` MUST be a valid channel role, `version` MUST be semver, `channel` MUST match `name`, `transport` MUST be non-empty, `anchor` MUST be non-empty, `config` MUST be non-nil (FR-R02, 004 FR-T14)
- `NeuronP2PExchangeService`: `type` MUST be `"neuron-p2p-exchange"`, `peerID` MUST be non-empty, `protocol` MUST be non-empty, `topicRef` MUST reference an existing `neuron-topic` service name (FR-R03, 004 FR-T17, 004 FR-T18)
- `DIDService`: when present, DID value MUST be `did:key:zQ3s...` derived from the Child's NeuronPublicKey (FR-R14). At most one DID service per registration.
- `AgentURI`: MUST contain exactly three `neuron-topic` services (stdIn, stdOut, stdErr) and at least one `neuron-p2p-exchange` service (FR-R08, FR-R02, FR-R03)

**Alternatives considered**:
- `github.com/xeipuuv/gojsonschema`: Full JSON Schema validation. Overkill for well-known struct shapes. Adds external dependency.
- `github.com/go-playground/validator`: Tag-based validator. Useful for simple field checks but less natural for cross-field constraints like `topicRef` validation.

---

## R5: agentURI JSON Format

**Decision**: Represent the agentURI as a JSON document containing a `services` array. The SDK constructs this JSON from Go structs and serializes it for on-chain storage. The on-chain value is the JSON string (or a URI pointing to hosted JSON, depending on deployment).

**Rationale**: EIP-8004 defines `agentURI` as a URI resolving to a JSON registration file with a `services` array (per EIP-8004 spec and 003 appendix). The SDK builds the `AgentURI` struct in Go, validates completeness (FR-R08), serializes to JSON, and passes the JSON string to the `register()` or `updateAgentURI()` contract calls.

**JSON structure** (per spec appendix):
```json
{
  "services": [
    { "type": "neuron-topic", "name": "stdIn", ... },
    { "type": "neuron-topic", "name": "stdOut", ... },
    { "type": "neuron-topic", "name": "stdErr", ... },
    { "type": "neuron-p2p-exchange", "name": "p2p", ... },
    { "type": "DID", "name": "DID", "endpoint": "did:key:zQ3s...", "version": "v1" }
  ]
}
```

**Hosting modes** (deployment concern, not SDK concern):
- **Data URI**: agentURI is `data:application/json;base64,...` — JSON is stored directly on-chain
- **IPFS**: agentURI is `ipfs://Qm...` — JSON is stored on IPFS, URI on-chain
- **HTTPS**: agentURI is `https://...` — JSON is hosted on a web server, URI on-chain

The SDK produces the JSON content; how it is hosted and referenced is a deployment decision outside the scope of this implementation.

**Alternatives considered**:
- Protobuf/CBOR on-chain: More compact but loses EIP-8004 compatibility (which expects JSON).
- Individual service fields as contract storage: Would require custom contract extensions beyond EIP-8004.
