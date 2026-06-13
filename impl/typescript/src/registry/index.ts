/**
 * registry -- Neuron SDK Peer Registry barrel exports.
 *
 * Spec reference: 003 spec.md
 *
 * Re-exports all public types, factories, and utilities from the registry module.
 * Consumers should import from this module rather than individual files.
 *
 * Usage:
 *   import { register, lookupRegistration, resolveTopics } from '@neuron-sdk/registry';
 */

// --- Errors ---
export { RegistryError } from './errors.js';
export {
  registrationFailed,
  lookupFailed,
  updateFailed,
  revocationFailed,
  invalidAgentURI,
  unauthorizedCaller,
} from './errors.js';

// --- Types ---
export type {
  NeuronTopicServiceEntry,
  NeuronP2PExchangeEntry,
  DIDServiceEntry,
  AgentURIService,
  AgentURI,
  Registration,
  RegistrationResult,
  LookupKey,
} from './types.js';

// --- Contract Interface ---
export type { RegistryContract } from './contract.js';

// --- Validation ---
export type { ValidationError, ValidationResult } from './validation.js';
export { validateRegistrationCompleteness } from './validation.js';

// --- Registration Operations ---
export {
  register,
  updateRegistration,
  revokeRegistration,
  lookupRegistration,
} from './registration.js';

// --- Resolution ---
export type { ResolvedTopics } from './resolver.js';
export { resolveTopics, resolveP2PExchange } from './resolver.js';

// --- AgentURI Construction/Parsing ---
export {
  buildAgentURI,
  parseAgentURI,
  serializeAgentURI,
} from './agent-uri.js';
