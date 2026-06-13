/**
 * RegistryError -- structured error type for the REG domain.
 *
 * Spec reference: 006 error-taxonomy.md, REG Domain (NEURON-REG-001..006)
 * Spec reference: 003 spec.md FR-R07 (documented errors for registration operations)
 *
 * Each factory function returns a RegistryError with the appropriate code and
 * human-readable name. The error extends NeuronError for consistent SDK error
 * handling across all domains.
 */

import {
  NeuronError,
  REG_REGISTRATION_FAILED,
  REG_LOOKUP_FAILED,
  REG_UPDATE_FAILED,
  REG_REVOCATION_FAILED,
  REG_INVALID_AGENT_URI,
  REG_UNAUTHORIZED_CALLER,
} from '../errors.js';

/**
 * Domain-specific error for Peer Registry operations.
 *
 * All registry-related errors use codes in the NEURON-REG-001..006 range.
 * FR-R07: Registration failures MUST be reported as documented errors.
 */
export class RegistryError extends NeuronError {}

/**
 * NEURON-REG-001: Registration failed.
 *
 * Thrown when an on-chain `register()` call fails (contract revert,
 * network error, or admission policy rejection).
 * FR-R06: Proof-of-control; FR-R09: Admission policy.
 *
 * @param message - Descriptive message about the registration failure
 * @param cause - Optional underlying error
 * @returns RegistryError with code NEURON-REG-001
 */
export function registrationFailed(message: string, cause?: Error): RegistryError {
  return new RegistryError(REG_REGISTRATION_FAILED, 'RegistrationFailed', message, cause);
}

/**
 * NEURON-REG-002: Lookup failed.
 *
 * Thrown when a registry lookup fails due to network error, timeout,
 * or the registration is not found.
 * FR-R07: Failure MUST be a documented error or absence.
 *
 * @param message - Descriptive message about the lookup failure
 * @param cause - Optional underlying error
 * @returns RegistryError with code NEURON-REG-002
 */
export function lookupFailed(message: string, cause?: Error): RegistryError {
  return new RegistryError(REG_LOOKUP_FAILED, 'LookupFailed', message, cause);
}

/**
 * NEURON-REG-003: Update failed.
 *
 * Thrown when an on-chain `updateAgentURI()` call fails.
 * FR-R06: Only the NFT owner or approved operator may update.
 *
 * @param message - Descriptive message about the update failure
 * @param cause - Optional underlying error
 * @returns RegistryError with code NEURON-REG-003
 */
export function updateFailed(message: string, cause?: Error): RegistryError {
  return new RegistryError(REG_UPDATE_FAILED, 'UpdateFailed', message, cause);
}

/**
 * NEURON-REG-004: Revocation failed.
 *
 * Thrown when an on-chain `revoke()` call fails.
 * FR-R11b: Only the registered agent may revoke its own registration.
 *
 * @param message - Descriptive message about the revocation failure
 * @param cause - Optional underlying error
 * @returns RegistryError with code NEURON-REG-004
 */
export function revocationFailed(message: string, cause?: Error): RegistryError {
  return new RegistryError(REG_REVOCATION_FAILED, 'RevocationFailed', message, cause);
}

/**
 * NEURON-REG-005: Invalid AgentURI.
 *
 * Thrown when the AgentURI fails validation (missing required services,
 * broken topicRef, invalid DID service, etc.).
 * FR-R08: A complete registration MUST include three neuron-topic services
 * and at least one neuron-p2p-exchange service.
 *
 * @param message - Descriptive message about the AgentURI validation failure
 * @param cause - Optional underlying error
 * @returns RegistryError with code NEURON-REG-005
 */
export function invalidAgentURI(message: string, cause?: Error): RegistryError {
  return new RegistryError(REG_INVALID_AGENT_URI, 'InvalidAgentURI', message, cause);
}

/**
 * NEURON-REG-006: Unauthorized caller.
 *
 * Thrown when the caller does not have permission for the requested
 * registry operation (role boundary violation).
 * FR-R11: Role boundaries for registry operations.
 *
 * @param message - Descriptive message about the authorization failure
 * @param cause - Optional underlying error
 * @returns RegistryError with code NEURON-REG-006
 */
export function unauthorizedCaller(message: string, cause?: Error): RegistryError {
  return new RegistryError(REG_UNAUTHORIZED_CALLER, 'UnauthorizedCaller', message, cause);
}
