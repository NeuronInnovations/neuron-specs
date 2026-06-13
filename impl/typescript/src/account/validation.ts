/**
 * Account validation -- type-specific rule enforcement.
 *
 * Spec reference: 001 spec.md, data-model.md
 *   - FR-006: Parent validation rules V-PARENT-01..05.
 *   - FR-007: Child validation rules V-CHILD-01..05.
 *   - FR-007a: Shared validation rules V-SHARED-01..04.
 *   - FR-014: Structured error reporting with all violations collected.
 *   - SC-005: All violations reported, not just the first.
 *
 * The validateAccount function checks all applicable rules for the given
 * account type and returns an array of ALL validation errors found.
 * An empty array indicates a valid account.
 *
 * Note: Several rules (V-PARENT-04, V-PARENT-05, V-CHILD-05, V-SHARED-02,
 * V-SHARED-03, V-SHARED-04) are enforced at compile time by the TypeScript
 * type system (discriminated union on accountType). These rules cannot be
 * violated at runtime if the account was constructed via the builder.
 * The runtime validation focuses on rules that could be violated if
 * accounts are constructed directly (e.g., from deserialization).
 */

import type { NeuronAccount } from './account.js';
import { isParent, isChild, isShared } from './account.js';
import {
  ACCT_MISSING_REQUIRED_FIELD,
  ACCT_INVALID_ACCOUNT_TYPE,
} from '../errors.js';

/**
 * A single validation error describing a rule violation.
 *
 * FR-014: Structured error reporting with field name, rule ID, and message.
 */
export interface ValidationError {
  /** The field that caused the validation failure. */
  readonly field: string;
  /** The validation rule ID (e.g., "V-PARENT-01"). */
  readonly rule: string;
  /** The error code from the NEURON-ACCT-* taxonomy. */
  readonly code: string;
  /** Human-readable description of the violation. */
  readonly message: string;
}

/**
 * Validate a NeuronAccount against its type-specific rules.
 *
 * FR-014, SC-005: Collects ALL violations -- does not stop at the first error.
 * Returns an empty array for a structurally valid account.
 *
 * Rules checked at runtime:
 * - V-PARENT-01: Parent must have DID (NEURON-ACCT-002)
 * - V-PARENT-02: Parent must have publicKey (NEURON-ACCT-002)
 * - V-CHILD-01: Child must have parentPubKey (NEURON-ACCT-002)
 * - V-CHILD-02: Child must have publicKey (NEURON-ACCT-002)
 * - V-SHARED-01: Shared must have multisigKey (NEURON-ACCT-002)
 *
 * Rules enforced by the type system (not checked at runtime):
 * - V-PARENT-03: creditBalance capability (field exists on ParentAccount)
 * - V-PARENT-04: No parentPubKey (not on ParentAccount interface)
 * - V-PARENT-05: No multisigKey (not on ParentAccount interface)
 * - V-CHILD-04: DID is optional (no error for absence)
 * - V-CHILD-05: No multisigKey (not on ChildAccount interface)
 * - V-SHARED-02: No DID (not on SharedAccount interface)
 * - V-SHARED-03: No parentPubKey (not on SharedAccount interface)
 * - V-SHARED-04: No publicKey (not on SharedAccount interface)
 *
 * @param account - NeuronAccount to validate
 * @returns Array of validation errors (empty if valid)
 */
export function validateAccount(account: NeuronAccount): ValidationError[] {
  const errors: ValidationError[] = [];

  if (isParent(account)) {
    validateParent(account, errors);
  } else if (isChild(account)) {
    validateChild(account, errors);
  } else if (isShared(account)) {
    validateShared(account, errors);
  } else {
    // Exhaustive check: should never reach here with valid discriminated union
    errors.push({
      field: 'accountType',
      rule: 'V-TYPE-01',
      code: ACCT_INVALID_ACCOUNT_TYPE,
      message: `Unknown account type: ${String((account as NeuronAccount).accountType)}`,
    });
  }

  return errors;
}

/**
 * Validate Parent-specific rules.
 *
 * V-PARENT-01: Must have DID.
 * V-PARENT-02: Must have publicKey.
 */
function validateParent(
  account: { readonly did?: unknown; readonly publicKey?: unknown },
  errors: ValidationError[],
): void {
  // V-PARENT-01: Must have DID
  if (account.did == null) {
    errors.push({
      field: 'did',
      rule: 'V-PARENT-01',
      code: ACCT_MISSING_REQUIRED_FIELD,
      message: 'Parent account must have a DID',
    });
  }

  // V-PARENT-02: Must have publicKey
  if (account.publicKey == null) {
    errors.push({
      field: 'publicKey',
      rule: 'V-PARENT-02',
      code: ACCT_MISSING_REQUIRED_FIELD,
      message: 'Parent account must have a publicKey',
    });
  }
}

/**
 * Validate Child-specific rules.
 *
 * V-CHILD-01: Must have parentPubKey.
 * V-CHILD-02: Must have publicKey.
 * V-CHILD-04: DID is optional (not an error if absent).
 */
function validateChild(
  account: { readonly parentPubKey?: unknown; readonly publicKey?: unknown },
  errors: ValidationError[],
): void {
  // V-CHILD-01: Must have parentPubKey
  if (account.parentPubKey == null) {
    errors.push({
      field: 'parentPubKey',
      rule: 'V-CHILD-01',
      code: ACCT_MISSING_REQUIRED_FIELD,
      message: 'Child account must have a parentPubKey',
    });
  }

  // V-CHILD-02: Must have publicKey
  if (account.publicKey == null) {
    errors.push({
      field: 'publicKey',
      rule: 'V-CHILD-02',
      code: ACCT_MISSING_REQUIRED_FIELD,
      message: 'Child account must have a publicKey',
    });
  }
}

/**
 * Validate Shared-specific rules.
 *
 * V-SHARED-01: Must have multisigKey.
 */
function validateShared(
  account: { readonly multisigKey?: unknown },
  errors: ValidationError[],
): void {
  // V-SHARED-01: Must have multisigKey
  if (account.multisigKey == null) {
    errors.push({
      field: 'multisigKey',
      rule: 'V-SHARED-01',
      code: ACCT_MISSING_REQUIRED_FIELD,
      message: 'Shared account must have a multisigKey',
    });
  }
}
