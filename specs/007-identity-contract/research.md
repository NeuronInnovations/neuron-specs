# Research: Identity Registry Smart Contract (EIP-8004)

**Feature**: 007-identity-contract
**Date**: 2026-03-04
**Purpose**: Resolve technical unknowns and document design decisions before implementation planning.

---

## R1: EIP-8004 Compliance Analysis

### Decision
The three-registry model (Identity, Reputation, Validation) follows the EIP-8004 specification for Trustless Agents. The Identity Registry is an ERC-721 contract; Reputation and Validation are standalone contracts linked by `agentId`.

### Rationale
EIP-8004 defines a modular registry architecture. The Identity Registry maps agents to on-chain identities via NFTs. Reputation and Validation registries extend the identity with trust signals. This modularity allows independent deployment and upgrade cycles.

### Key EIP-8004 Findings
- **Identity Registry**: ERC-721 + URIStorage pattern. Auto-incrementing `tokenId` as `agentId`. `agentURI` stored as opaque string.
- **Reputation Registry**: Feedback model with fixed-point values, tags, and off-chain detail URIs. Supports revocation and agent responses.
- **Validation Registry**: Request/response model between agents and validators. Status codes: pending (0), pass (1), fail (2).
- **Cross-registry key**: `agentId` (tokenId from Identity Registry) is the shared identifier.
- **agentWallet**: EIP-712 signed metadata for payment addresses — noted but not implemented in this spec iteration.

### Alternatives Considered
- **Single monolithic contract**: Rejected — too large for EVM 24KB limit, reduces upgradeability granularity.
- **ERC-1155 multi-token**: Rejected — ERC-721 aligns with one-NFT-per-agent identity model and provides better tooling compatibility.

---

## R2: ERC-721 Extension Pattern

### Decision
Use ERC-721 with ERC721Enumerable extension. Maintain a custom `address → tokenId` reverse mapping for `lookup()`.

### Rationale
ERC721Enumerable enables on-chain enumeration of all registered agents. The reverse mapping (`addressToTokenId`) is necessary for O(1) `lookup()` by address — standard ERC-721 only provides `ownerOf(tokenId)`.

### Implementation Notes
- OpenZeppelin Contracts v5 provides `ERC721Enumerable` with gas-optimized storage.
- Reverse mapping must be maintained during `register()` (set) and `revoke()` (clear).
- Standard `transferFrom()` does NOT update the reverse mapping — this is a known limitation documented in Appendix D of the spec.

### Alternatives Considered
- **ERC721URIStorage only**: Rejected — no enumeration support for registry-wide queries.
- **Custom enumeration**: Rejected — OpenZeppelin's battle-tested implementation preferred.

---

## R3: Admission Policy Architecture

### Decision
Strategy pattern via `IAdmissionPolicy` interface. `address(0)` means permissionless. Two `register()` overloads: `register(agentURI)` and `register(agentURI, parentDIDProof)`.

### Rationale
The strategy pattern decouples admission logic from the registry contract. Platform operators can swap policies without redeploying the registry. The `parentDIDProof` is opaque `bytes` — each policy implementation defines its own proof format (e.g., Merkle proof, signature, oracle attestation).

### Implementation Notes
- `isAdmitted(address childAddress, bytes parentDIDProof)` returns `bool`.
- When policy is `address(0)`, skip admission check entirely.
- When policy is non-zero and `register(agentURI)` overload is used, call `isAdmitted(msg.sender, "")` with empty proof.
- Removing a Parent DID from an allowlist does NOT auto-revoke existing registrations (003 FR-R12d).

### Alternatives Considered
- **Hardcoded allowlist in registry**: Rejected — inflexible, can't swap policies.
- **Off-chain admission with on-chain signature verification**: More complex — deferred to future policy implementations.

---

## R4: Proxy Upgrade Pattern

### Decision
Recommend EIP-1967 Transparent Proxy for all three registries. Proxy admin MUST be separate from registry admin.

### Rationale
EIP-1967 is the most widely adopted proxy pattern. Transparent proxies avoid function selector clashes between admin and user functions. Separate admin addresses prevent privilege escalation.

### Implementation Notes
- OpenZeppelin `TransparentUpgradeableProxy` + `ProxyAdmin` contract.
- Registry contracts must use `initializer` instead of `constructor` for proxy compatibility.
- Storage layout must follow EIP-1967 slot conventions to avoid collisions.

### Alternatives Considered
- **UUPS (EIP-1822)**: Viable alternative — upgrade logic in implementation rather than proxy. Slightly lower gas but harder to audit. Not recommended as default.
- **Immutable deployment**: Valid for test/demo environments. Document as alternative with tradeoff: no upgrades but simpler security model.
- **Diamond pattern (EIP-2535)**: Over-engineered for this use case.

---

## R5: Hedera EVM Compatibility

### Decision
All contracts MUST use standard EVM opcodes only. No Hedera-specific precompiles (e.g., HTS at `0x167`).

### Rationale
Cross-chain compatibility (FR-C-17) requires standard EVM bytecode. Hedera EVM supports standard Solidity contracts; the differences are operational (gas model, account model, finality) rather than functional.

### Key Hedera EVM Differences (FR-C-38)
1. **Gas model**: Gas units → HBAR conversion via dynamic exchange rate. Deployment may cost more/less than Ethereum depending on rate.
2. **Account model**: Addresses require explicit creation. Auto-account creation from ECDSA keys supported.
3. **Finality**: Immediate finality (no reorgs). Simplifies confirmation logic.
4. **Contract size**: Same 24KB limit as Ethereum.
5. **Block timestamps**: Hedera consensus timestamps have ~3-5 second granularity.

### Alternatives Considered
- **Hedera Token Service (HTS)**: Could use HTS for NFTs instead of ERC-721. Rejected — breaks Ethereum compatibility and requires Hedera-specific SDK calls.

---

## R6: Go SDK Integration Layer

### Decision
The Go SDK's existing `RegistryContract` interface (from Spec 003, `internal/registry/`) maps to the deployed contract ABI via `go-ethereum`'s `abigen` tool or manual ABI binding.

### Rationale
Constitution Principle VI requires Go as the primary SDK language. The existing `RegistryContract` interface in `internal/registry/` defines the Go-side abstraction. The contract ABI generated from this spec's interface definitions provides the bridge.

### Integration Notes
- `go-ethereum/accounts/abi/bind` for contract interaction.
- SDK calls `register()`, `updateAgentURI()`, `revoke()`, `lookup()` on the Identity Registry.
- SDK validates agentURI (V-REG-01 through V-REG-12) before calling the contract.
- Contract ABI stub already exists in `internal/registry/`; update to match Spec 007 interfaces.

### Alternatives Considered
- **Direct RPC calls**: Lower-level but less type-safe. `abigen` preferred for type safety and code generation.

---

## R7: Fixed-Point Arithmetic for Reputation

### Decision
Reputation values use `int128 value` with `uint8 decimals` (0–18). Actual rating = `value / 10^decimals`.

### Rationale
Fixed-point avoids floating-point precision issues on-chain. `int128` allows negative ratings (penalties). `uint8 decimals` with max 18 matches Ethereum's standard decimal precision (wei/ether uses 18 decimals).

### Implementation Notes
- Summation: aggregate `totalValue` as `int256` to avoid overflow.
- Division for average: done off-chain or in view function with proper rounding.
- `decimals > 18` MUST be rejected to prevent `10^decimals` overflow.

### Alternatives Considered
- **uint256 only (positive ratings)**: Rejected — doesn't support penalties or negative feedback.
- **Basis points (uint16, 0-10000)**: Too restrictive for general-purpose ratings.
