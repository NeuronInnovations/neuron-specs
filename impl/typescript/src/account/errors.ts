/**
 * AccountError -- structured error type for the ACCT domain.
 *
 * Spec reference: 006 error-taxonomy.md, ACCT Domain (NEURON-ACCT-001..008)
 * FR-014: Structured error types with specific codes, names, and descriptive messages.
 *
 * Each factory function returns an AccountError with the appropriate code and
 * human-readable name. The error extends NeuronError for consistent SDK error
 * handling across all domains.
 */

import {
  NeuronError,
  ACCT_INVALID_ACCOUNT_TYPE,
  ACCT_MISSING_REQUIRED_FIELD,
  ACCT_FORBIDDEN_FIELD,
  ACCT_INVALID_DID,
  ACCT_PARENT_KEY_MISMATCH,
  ACCT_INVALID_LEDGER_ATTACHMENT,
  ACCT_ACCOUNT_INCOMPLETE,
  ACCT_INVALID_CURRENCY_SYMBOL,
} from '../errors.js';

/**
 * Domain-specific error for NeuronAccount operations.
 *
 * All account-related errors use codes in the NEURON-ACCT-001..008 range.
 */
export class AccountError extends NeuronError {}

/**
 * NEURON-ACCT-001: Invalid account type value.
 *
 * Thrown when an account type discriminator is not one of the valid
 * AccountType enum values (Parent=1, Child=2, Shared=3).
 *
 * @param message - Descriptive message about the invalid type
 * @returns AccountError with code NEURON-ACCT-001
 */
export function invalidAccountType(message: string): AccountError {
  return new AccountError(ACCT_INVALID_ACCOUNT_TYPE, 'InvalidAccountType', message);
}

/**
 * NEURON-ACCT-002: Required field is missing.
 *
 * Thrown when a field that is mandatory for the account type is not provided.
 * Examples: Parent missing DID, Child missing parentPubKey.
 *
 * @param message - Descriptive message identifying the missing field
 * @returns AccountError with code NEURON-ACCT-002
 */
export function missingRequiredField(message: string): AccountError {
  return new AccountError(ACCT_MISSING_REQUIRED_FIELD, 'MissingRequiredField', message);
}

/**
 * NEURON-ACCT-003: Forbidden field is present.
 *
 * Thrown when a field that is prohibited for the account type is set.
 * Examples: Parent with parentPubKey, Shared with publicKey.
 *
 * @param message - Descriptive message identifying the forbidden field
 * @returns AccountError with code NEURON-ACCT-003
 */
export function forbiddenField(message: string): AccountError {
  return new AccountError(ACCT_FORBIDDEN_FIELD, 'ForbiddenField', message);
}

/**
 * NEURON-ACCT-004: Invalid DID format.
 *
 * Thrown when a DID identifier does not conform to the expected did:key: format.
 *
 * @param message - Descriptive message about the DID validation failure
 * @returns AccountError with code NEURON-ACCT-004
 */
export function invalidDID(message: string): AccountError {
  return new AccountError(ACCT_INVALID_DID, 'InvalidDID', message);
}

/**
 * NEURON-ACCT-005: Parent key mismatch.
 *
 * Thrown when the parent public key reference does not match the expected
 * parent account's public key.
 *
 * @param message - Descriptive message about the key mismatch
 * @returns AccountError with code NEURON-ACCT-005
 */
export function parentKeyMismatch(message: string): AccountError {
  return new AccountError(ACCT_PARENT_KEY_MISMATCH, 'ParentKeyMismatch', message);
}

/**
 * NEURON-ACCT-006: Invalid ledger attachment.
 *
 * Thrown when a ledger attachment has invalid state, missing fields,
 * or inconsistent data.
 *
 * @param message - Descriptive message about the attachment issue
 * @returns AccountError with code NEURON-ACCT-006
 */
export function invalidLedgerAttachment(message: string): AccountError {
  return new AccountError(ACCT_INVALID_LEDGER_ATTACHMENT, 'InvalidLedgerAttachment', message);
}

/**
 * NEURON-ACCT-007: Account is incomplete.
 *
 * Thrown by buildComplete() when an account is structurally valid but
 * missing fields required for completeness (FR-011a). Examples: Parent
 * without ledger attachment, Child without registry binding.
 *
 * @param message - Descriptive message about what is missing for completeness
 * @returns AccountError with code NEURON-ACCT-007
 */
export function accountIncomplete(message: string): AccountError {
  return new AccountError(ACCT_ACCOUNT_INCOMPLETE, 'AccountIncomplete', message);
}

/**
 * NEURON-ACCT-008: Invalid currency symbol.
 *
 * Thrown when a currency symbol is empty, missing, or otherwise invalid.
 *
 * @param message - Descriptive message about the currency symbol issue
 * @returns AccountError with code NEURON-ACCT-008
 */
export function invalidCurrencySymbol(message: string): AccountError {
  return new AccountError(ACCT_INVALID_CURRENCY_SYMBOL, 'InvalidCurrencySymbol', message);
}
