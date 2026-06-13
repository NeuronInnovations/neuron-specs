/**
 * RegistryContract -- abstract interface for EIP-8004 registry operations.
 *
 * Spec reference: 003 spec.md
 *   - FR-R01: Registry is a smart contract (EIP-8004) on Hedera and Ethereum.
 *   - FR-R04: Lookup by (registry + Child EVM address) or (registry + external id).
 *   - FR-R06: register() signed by Child's NeuronPrivateKey (proof-of-control).
 *   - FR-R10: NFT owner is Child's EVMAddress after mint.
 *   - FR-R11: Role boundaries for update/revoke.
 *
 * Spec reference: 003-to-007 mapping
 *   - register() -> 007 register()
 *   - updateAgentURI() -> 007 updateAgentURI()
 *   - revoke() -> 007 revoke()
 *   - lookup() -> 007 lookup()
 *
 * This interface is implemented by chain-specific adapters (e.g., Ethereum,
 * Hedera). The SDK provides the workflow logic; the adapter handles the
 * on-chain transaction details.
 */

import type { Registration, RegistrationResult, LookupKey } from './types.js';

/**
 * Abstract interface for interacting with an EIP-8004 Identity Registry contract.
 *
 * FR-R01: The same contract model applies on Hedera and Ethereum.
 * Implementations handle chain-specific RPC, signing, and transaction submission.
 *
 * All methods return Promises because on-chain operations are asynchronous.
 * Errors are reported via RegistryError (NEURON-REG-001..006).
 */
export interface RegistryContract {
  /**
   * Register a new agent in the EIP-8004 registry.
   *
   * FR-R06: The transaction MUST be signed by the Child's NeuronPrivateKey.
   * FR-R10: The resulting NFT MUST be owned by the Child's EVMAddress.
   *
   * @param agentURI - The serialized AgentURI JSON string
   * @returns RegistrationResult with tokenId, transactionHash, and metadata
   * @throws RegistryError NEURON-REG-001 if registration fails
   */
  register(agentURI: string): Promise<RegistrationResult>;

  /**
   * Update the agentURI of an existing registration.
   *
   * FR-R06: Post-mint, updateAgentURI is governed by ERC-721 token ownership.
   * FR-R11b: Only the registered agent or approved operator may update.
   *
   * @param tokenId - The ERC-721 token ID of the registration to update
   * @param newAgentURI - The new serialized AgentURI JSON string
   * @returns RegistrationResult with the updated metadata
   * @throws RegistryError NEURON-REG-003 if update fails
   * @throws RegistryError NEURON-REG-006 if caller is not authorized
   */
  updateAgentURI(tokenId: bigint, newAgentURI: string): Promise<RegistrationResult>;

  /**
   * Revoke (burn) a registration in the registry.
   *
   * FR-R11b: Only the registered agent may revoke its own registration.
   *
   * @param tokenId - The ERC-721 token ID of the registration to revoke
   * @returns The on-chain transaction hash
   * @throws RegistryError NEURON-REG-004 if revocation fails
   * @throws RegistryError NEURON-REG-006 if caller is not authorized
   */
  revoke(tokenId: bigint): Promise<string>;

  /**
   * Look up a registration in the registry.
   *
   * FR-R04: Lookup by Child EVM address or external id.
   * FR-R07: Returns null when not found; throws on network errors.
   *
   * @param key - Lookup discriminant (byAddress or byExternalId)
   * @returns Registration if found, null if not found
   * @throws RegistryError NEURON-REG-002 if lookup fails (network error, timeout)
   */
  lookup(key: LookupKey): Promise<Registration | null>;

  /**
   * Query the owner of an ERC-721 token in the registry.
   *
   * FR-R10: After mint, the owner MUST be the Child's EVMAddress.
   *
   * @param tokenId - The ERC-721 token ID
   * @returns The owner's EVM address as a hex string
   * @throws RegistryError NEURON-REG-002 if the query fails
   */
  ownerOf(tokenId: bigint): Promise<string>;
}
