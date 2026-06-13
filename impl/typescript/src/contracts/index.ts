/**
 * Barrel exports for the Neuron Identity Contract module (Spec 007).
 *
 * Provides TypeScript type definitions for the three EVM registries
 * defined by EIP-8004:
 *   - Identity Registry (ERC-721 + URIStorage)
 *   - Reputation Registry (client feedback)
 *   - Validation Registry (third-party verification)
 *   - Admission Policy (pluggable registration control)
 *
 * No ethers.js dependency -- pure type definitions.
 * ABI JSON files are deferred to src/contracts/abi/.
 */

// --- Shared Types ---
export type {
  Uint256,
  Address,
  Bytes32,
  Int128,
  FeedbackEntry,
  FeedbackSummary,
  ValidationRecord,
  ValidationSummary,
} from './types.js';

export {
  ValidationResponse,
  BYTES32_LENGTH,
  MAX_FEEDBACK_DECIMALS,
} from './types.js';

// --- Identity Registry ---
export type {
  IIdentityRegistry,
  IdentityRegisteredEvent,
  IdentityUpdatedEvent,
  IdentityRevokedEvent,
  AdmissionPolicyUpdatedEvent,
} from './identity-registry.js';

// --- Reputation Registry ---
export type {
  IReputationRegistry,
  FeedbackGivenEvent,
  FeedbackRevokedEvent,
  ResponseAppendedEvent,
} from './reputation-registry.js';

// --- Validation Registry ---
export type {
  IValidationRegistry,
  ValidationRequestedEvent,
  ValidationRespondedEvent,
} from './validation-registry.js';

// --- Admission Policy ---
export type { IAdmissionPolicy } from './admission-policy.js';
export { PERMISSIONLESS_POLICY } from './admission-policy.js';
