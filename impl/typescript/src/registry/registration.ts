/**
 * Registration operations -- register, update, revoke, lookup.
 *
 * Spec reference: 003 spec.md
 *   - FR-R01: Registry is EIP-8004 smart contract.
 *   - FR-R06: register() signed by Child's NeuronPrivateKey (proof-of-control).
 *   - FR-R07: Lookup failure is a documented error.
 *   - FR-R08: AgentURI validated for completeness before register/update.
 *   - FR-R10: NFT owner is Child's EVMAddress after mint.
 *   - FR-R11: Role boundaries for operations.
 *
 * Spec reference: 003-to-007 mapping
 *   - register() -> 007 register()
 *   - updateRegistration() -> 007 updateAgentURI()
 *   - revokeRegistration() -> 007 revoke()
 *   - lookupRegistration() -> 007 lookup()
 */

import type { NeuronPrivateKey } from '../keylib/private-key.js';
import type { RegistryContract } from './contract.js';
import type { AgentURI, RegistrationResult, Registration, LookupKey } from './types.js';
import { validateRegistrationCompleteness } from './validation.js';
import { serializeAgentURI } from './agent-uri.js';
import {
  registrationFailed,
  updateFailed,
  revocationFailed,
  lookupFailed,
  invalidAgentURI,
} from './errors.js';
import { RegistryError } from './errors.js';

/**
 * Register a Child agent in an EIP-8004 registry.
 *
 * FR-R06: The Child's NeuronPrivateKey provides proof-of-control.
 * FR-R08: Validates AgentURI completeness before submitting.
 * FR-R10: The resulting NFT is owned by the Child's EVMAddress.
 *
 * Workflow:
 * 1. Validate AgentURI (V-REG-01..07, V-REG-11, V-REG-12)
 * 2. Serialize AgentURI to JSON
 * 3. Call contract.register() (on-chain transaction)
 *
 * @param childKey - The Child's NeuronPrivateKey (for proof-of-control and validation)
 * @param contract - RegistryContract adapter for the target chain
 * @param agentURI - The AgentURI containing all required services
 * @returns RegistrationResult with tokenId, transactionHash, and metadata
 * @throws RegistryError NEURON-REG-005 if AgentURI validation fails
 * @throws RegistryError NEURON-REG-001 if on-chain registration fails
 */
export async function register(
  childKey: NeuronPrivateKey,
  contract: RegistryContract,
  agentURI: AgentURI,
): Promise<RegistrationResult> {
  // FR-R08: Validate completeness before submission
  const childPubKey = childKey.publicKey();
  const validation = validateRegistrationCompleteness(agentURI, childPubKey);

  if (!validation.valid) {
    const messages = validation.errors.map(e => `[${e.rule}] ${e.message}`);
    throw invalidAgentURI(
      `AgentURI validation failed: ${messages.join('; ')}`,
    );
  }

  // Serialize and submit
  const uriJson = serializeAgentURI(agentURI);

  try {
    return await contract.register(uriJson);
  } catch (e) {
    if (e instanceof RegistryError) {
      throw e;
    }
    const cause = e instanceof Error ? e : new Error(String(e));
    throw registrationFailed('On-chain register() call failed', cause);
  }
}

/**
 * Update the AgentURI of an existing registration.
 *
 * FR-R06: Post-mint, updateAgentURI is governed by ERC-721 token ownership.
 * FR-R08: Validates the new AgentURI for completeness.
 * FR-R11b: Only the registered agent or approved operator may update.
 *
 * @param callerKey - The caller's NeuronPrivateKey (Child or delegated operator)
 * @param contract - RegistryContract adapter for the target chain
 * @param tokenId - The ERC-721 token ID of the registration to update
 * @param newAgentURI - The new AgentURI containing all required services
 * @returns RegistrationResult with the updated metadata
 * @throws RegistryError NEURON-REG-005 if AgentURI validation fails
 * @throws RegistryError NEURON-REG-003 if on-chain update fails
 * @throws RegistryError NEURON-REG-006 if caller is not authorized
 */
export async function updateRegistration(
  callerKey: NeuronPrivateKey,
  contract: RegistryContract,
  tokenId: bigint,
  newAgentURI: AgentURI,
): Promise<RegistrationResult> {
  // FR-R08: Validate completeness before submission
  const callerPubKey = callerKey.publicKey();
  const validation = validateRegistrationCompleteness(newAgentURI, callerPubKey);

  if (!validation.valid) {
    const messages = validation.errors.map(e => `[${e.rule}] ${e.message}`);
    throw invalidAgentURI(
      `AgentURI validation failed: ${messages.join('; ')}`,
    );
  }

  const uriJson = serializeAgentURI(newAgentURI);

  try {
    return await contract.updateAgentURI(tokenId, uriJson);
  } catch (e) {
    if (e instanceof RegistryError) {
      throw e;
    }
    const cause = e instanceof Error ? e : new Error(String(e));
    throw updateFailed('On-chain updateAgentURI() call failed', cause);
  }
}

/**
 * Revoke (burn) a registration in the registry.
 *
 * FR-R11b: Only the registered agent may revoke its own registration.
 *
 * @param childKey - The Child's NeuronPrivateKey
 * @param contract - RegistryContract adapter for the target chain
 * @param tokenId - The ERC-721 token ID of the registration to revoke
 * @returns The on-chain transaction hash
 * @throws RegistryError NEURON-REG-004 if on-chain revocation fails
 * @throws RegistryError NEURON-REG-006 if caller is not authorized
 */
export async function revokeRegistration(
  _childKey: NeuronPrivateKey,
  contract: RegistryContract,
  tokenId: bigint,
): Promise<string> {
  try {
    return await contract.revoke(tokenId);
  } catch (e) {
    if (e instanceof RegistryError) {
      throw e;
    }
    const cause = e instanceof Error ? e : new Error(String(e));
    throw revocationFailed('On-chain revoke() call failed', cause);
  }
}

/**
 * Look up a registration in the registry.
 *
 * FR-R04: Lookup by (registry + Child EVM address) or (registry + external id).
 * FR-R07: Returns null when not found; throws on network errors.
 *
 * @param contract - RegistryContract adapter for the target chain
 * @param key - Lookup discriminant (byAddress or byExternalId)
 * @returns Registration if found, null if not found
 * @throws RegistryError NEURON-REG-002 if lookup fails (network error, timeout)
 */
export async function lookupRegistration(
  contract: RegistryContract,
  key: LookupKey,
): Promise<Registration | null> {
  try {
    return await contract.lookup(key);
  } catch (e) {
    if (e instanceof RegistryError) {
      throw e;
    }
    const cause = e instanceof Error ? e : new Error(String(e));
    throw lookupFailed('Registry lookup failed', cause);
  }
}
