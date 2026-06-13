/**
 * IIdentityRegistry -- EIP-8004 Identity Registry interface.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-01: ERC-721 contract with ERC721Enumerable support.
 *   - FR-C-02: register() mints NFT with agentURI. Returns tokenId.
 *   - FR-C-03: agentURI(tokenId) retrieves stored URI string.
 *   - FR-C-04: IdentityRegistered event on successful registration.
 *   - FR-C-05: IdentityUpdated / IdentityRevoked events.
 *   - FR-C-06: One registration per address per registry instance.
 *   - FR-C-07: updateAgentURI() callable by owner or approved operator.
 *   - FR-C-08: revoke() burns token. Owner only.
 *   - FR-C-09: Admin cannot mint, modify, or revoke on behalf of agents.
 *   - FR-C-10: lookup(address) returns (tokenId, agentURI) or (0, "").
 *   - FR-C-11: Contract does not parse agentURI content (opaque string).
 *   - FR-C-12: Pluggable admission policy via IAdmissionPolicy.
 *   - FR-C-13: setAdmissionPolicy() owner only. Emits AdmissionPolicyUpdated.
 *   - FR-C-16: After revoke(), same address may re-register with new tokenId.
 *   - FR-C-18: Reentrancy protection on state-modifying functions.
 *   - FR-C-19: Empty agentURI rejected with EmptyAgentURI revert.
 *
 * This interface is chain-agnostic. Implementations handle RPC, signing, and
 * transaction submission for the target EVM chain (Ethereum, Base, Hedera EVM).
 */

import type { Uint256, Address } from './types.js';

// ---------------------------------------------------------------------------
// Interface
// ---------------------------------------------------------------------------

/**
 * EIP-8004 Identity Registry -- ERC-721 + URIStorage.
 *
 * Maps a Child's EVMAddress to a registration NFT containing an agentURI.
 * Enforces proof-of-control (msg.sender == Child), uniqueness (one registration
 * per address), pluggable admission policy, and role boundaries.
 */
export interface IIdentityRegistry {
  /**
   * Register a new agent identity.
   *
   * FR-C-02: Mints an ERC-721 token to msg.sender with auto-incrementing tokenId.
   * FR-C-06: Reverts with AlreadyRegistered if caller already holds a token.
   * FR-C-19: Reverts with EmptyAgentURI if agentURI is empty.
   * FR-C-04: Emits IdentityRegistered(tokenId, owner, agentURI).
   *
   * @param agentURI - The serialized AgentURI JSON string (opaque to contract).
   * @returns The newly minted tokenId.
   */
  register(agentURI: string): Promise<Uint256>;

  /**
   * Register with admission proof for permissioned registries.
   *
   * FR-C-02: Same as register() but includes parentDIDProof for admission check.
   * FR-C-12: When admission policy is set, calls isAdmitted(childAddress, parentDIDProof).
   * FR-C-14: parentDIDProof format is defined by the admission policy implementation.
   *
   * @param agentURI - The serialized AgentURI JSON string.
   * @param parentDIDProof - Opaque bytes for admission policy verification.
   * @returns The newly minted tokenId.
   */
  registerWithProof(agentURI: string, parentDIDProof: Uint8Array): Promise<Uint256>;

  /**
   * Update the agentURI of an existing registration.
   *
   * FR-C-05: Emits IdentityUpdated(tokenId, newAgentURI).
   * FR-C-07: Callable by token owner or approved operator only.
   * FR-C-19: Reverts with EmptyAgentURI if newAgentURI is empty.
   *
   * @param tokenId - The ERC-721 token ID of the registration to update.
   * @param newAgentURI - The new serialized AgentURI JSON string.
   */
  updateAgentURI(tokenId: Uint256, newAgentURI: string): Promise<void>;

  /**
   * Revoke (burn) a registration.
   *
   * FR-C-05: Emits IdentityRevoked(tokenId, owner).
   * FR-C-08: Burns the token and clears the address-to-token mapping.
   *          Callable only by the token owner (not approved operators, not admin).
   * FR-C-16: After revocation, the same address may re-register with a new tokenId.
   *
   * @param tokenId - The ERC-721 token ID of the registration to revoke.
   */
  revoke(tokenId: Uint256): Promise<void>;

  /**
   * Read the agentURI for a given token.
   *
   * FR-C-03: Returns the stored agentURI string for valid tokens.
   *          Reverts for non-existent tokens.
   *
   * @param tokenId - The ERC-721 token ID to query.
   * @returns The stored agentURI string.
   */
  agentURI(tokenId: Uint256): Promise<string>;

  /**
   * Look up a registration by address.
   *
   * FR-C-10: Returns (tokenId, agentURI) if registered.
   *          Returns (0n, "") if the address is not registered.
   *
   * @param address - The EVM address to look up (EIP-55 checksummed).
   * @returns Object with tokenId and agentURI.
   */
  lookup(address: Address): Promise<{ readonly tokenId: Uint256; readonly agentURI: string }>;

  /**
   * Query the owner of an ERC-721 token.
   *
   * Standard ERC-721 ownerOf. Used by observers (005) to verify proof-of-control.
   *
   * @param tokenId - The ERC-721 token ID.
   * @returns The owner's EVM address.
   */
  ownerOf(tokenId: Uint256): Promise<Address>;

  /**
   * Set the admission policy contract address.
   *
   * FR-C-13: Registry admin (owner) only. Emits AdmissionPolicyUpdated.
   * FR-C-12: address(0) means permissionless mode (no admission check).
   *
   * @param newPolicy - The new admission policy contract address.
   */
  setAdmissionPolicy(newPolicy: Address): Promise<void>;

  /**
   * Read the current admission policy address.
   *
   * FR-C-12: Returns address(0) for permissionless registries.
   *
   * @returns The current admission policy contract address.
   */
  admissionPolicy(): Promise<Address>;
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

/**
 * Emitted when a new agent identity is registered.
 *
 * FR-C-04: IdentityRegistered(uint256 indexed tokenId, address indexed owner, string agentURI).
 */
export interface IdentityRegisteredEvent {
  /** The newly minted ERC-721 token ID. FR-C-02 */
  readonly tokenId: Uint256;

  /** The address that called register() (msg.sender). FR-C-02 */
  readonly owner: Address;

  /** The stored agentURI string. FR-C-03 */
  readonly agentURI: string;
}

/**
 * Emitted when an agent's agentURI is updated.
 *
 * FR-C-05: IdentityUpdated(uint256 indexed tokenId, string newAgentURI).
 */
export interface IdentityUpdatedEvent {
  /** The ERC-721 token ID. FR-C-05 */
  readonly tokenId: Uint256;

  /** The new agentURI string. FR-C-05 */
  readonly newAgentURI: string;
}

/**
 * Emitted when an agent's registration is revoked (token burned).
 *
 * FR-C-05: IdentityRevoked(uint256 indexed tokenId, address indexed owner).
 */
export interface IdentityRevokedEvent {
  /** The burned ERC-721 token ID. FR-C-08 */
  readonly tokenId: Uint256;

  /** The address that owned the token. FR-C-08 */
  readonly owner: Address;
}

/**
 * Emitted when the admission policy is updated.
 *
 * FR-C-13: AdmissionPolicyUpdated(address indexed oldPolicy, address indexed newPolicy).
 */
export interface AdmissionPolicyUpdatedEvent {
  /** The previous admission policy address. FR-C-13 */
  readonly oldPolicy: Address;

  /** The new admission policy address. FR-C-13 */
  readonly newPolicy: Address;
}
