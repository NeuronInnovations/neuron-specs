/**
 * account -- Neuron SDK Account Module barrel exports.
 *
 * Spec reference: 001 spec.md
 *
 * Re-exports all public types, builders, validation, and utilities
 * from the account module. Consumers should import from this module
 * rather than individual files.
 *
 * Usage:
 *   import { ParentAccountBuilder, NeuronAccount, AccountType } from '../account/index.js';
 */

// Error types
export { AccountError } from './errors.js';
export {
  invalidAccountType,
  missingRequiredField,
  forbiddenField,
  invalidDID,
  parentKeyMismatch,
  invalidLedgerAttachment,
  accountIncomplete,
  invalidCurrencySymbol,
} from './errors.js';

// Core types
export { AccountType } from './types.js';
export { createNeuronDID } from './types.js';
export type {
  NeuronDID,
  LedgerState,
  VerificationStatus,
  LedgerAttachment,
  RegistryBinding,
  LedgerAccountId,
  VerificationResult,
} from './types.js';

// Account types and type guards
export type {
  BaseAccount,
  ParentAccount,
  ChildAccount,
  SharedAccount,
  NeuronAccount,
} from './account.js';
export { isParent, isChild, isShared, evmAddress, peerId } from './account.js';

// Builders
export { ParentAccountBuilder, ChildAccountBuilder, SharedAccountBuilder } from './builder.js';

// Validation
export type { ValidationError } from './validation.js';
export { validateAccount } from './validation.js';

// Payment
export { paymentAddress } from './payment.js';
