/**
 * T001: AccountError tests -- all 8 NEURON-ACCT-* codes.
 *
 * Source: 006 error-taxonomy.md, ACCT Domain
 * Verifies: error codes, names, messages, cause wrapping, inheritance.
 * FR-014, SC-001
 */

import { describe, it, expect } from 'vitest';
import { NeuronError } from '../../src/errors.js';
import * as codes from '../../src/errors.js';
import {
  AccountError,
  invalidAccountType,
  missingRequiredField,
  forbiddenField,
  invalidDID,
  parentKeyMismatch,
  invalidLedgerAttachment,
  accountIncomplete,
  invalidCurrencySymbol,
} from '../../src/account/errors.js';

describe('AccountError (NEURON-ACCT-001..008)', () => {
  const errorDefs: Array<{
    code: string;
    name: string;
    constant: string;
    factory: (msg: string) => AccountError;
  }> = [
    {
      code: 'NEURON-ACCT-001',
      name: 'InvalidAccountType',
      constant: 'ACCT_INVALID_ACCOUNT_TYPE',
      factory: invalidAccountType,
    },
    {
      code: 'NEURON-ACCT-002',
      name: 'MissingRequiredField',
      constant: 'ACCT_MISSING_REQUIRED_FIELD',
      factory: missingRequiredField,
    },
    {
      code: 'NEURON-ACCT-003',
      name: 'ForbiddenField',
      constant: 'ACCT_FORBIDDEN_FIELD',
      factory: forbiddenField,
    },
    {
      code: 'NEURON-ACCT-004',
      name: 'InvalidDID',
      constant: 'ACCT_INVALID_DID',
      factory: invalidDID,
    },
    {
      code: 'NEURON-ACCT-005',
      name: 'ParentKeyMismatch',
      constant: 'ACCT_PARENT_KEY_MISMATCH',
      factory: parentKeyMismatch,
    },
    {
      code: 'NEURON-ACCT-006',
      name: 'InvalidLedgerAttachment',
      constant: 'ACCT_INVALID_LEDGER_ATTACHMENT',
      factory: invalidLedgerAttachment,
    },
    {
      code: 'NEURON-ACCT-007',
      name: 'AccountIncomplete',
      constant: 'ACCT_ACCOUNT_INCOMPLETE',
      factory: accountIncomplete,
    },
    {
      code: 'NEURON-ACCT-008',
      name: 'InvalidCurrencySymbol',
      constant: 'ACCT_INVALID_CURRENCY_SYMBOL',
      factory: invalidCurrencySymbol,
    },
  ];

  it('should define all 8 error code constants', () => {
    for (const def of errorDefs) {
      const value = (codes as unknown as Record<string, string>)[def.constant];
      expect(value).toBe(def.code);
    }
  });

  it('should be an instance of Error and NeuronError', () => {
    for (const def of errorDefs) {
      const err = def.factory(`Test message for ${def.name}`);
      expect(err).toBeInstanceOf(Error);
      expect(err).toBeInstanceOf(NeuronError);
      expect(err).toBeInstanceOf(AccountError);
    }
  });

  it('should set code and name correctly via factory functions', () => {
    for (const def of errorDefs) {
      const err = def.factory(`Test: ${def.name}`);
      expect(err.code).toBe(def.code);
      expect(err.name).toBe(def.name);
    }
  });

  it('should include the descriptive message', () => {
    for (const def of errorDefs) {
      const msg = `Descriptive message for ${def.name}`;
      const err = def.factory(msg);
      expect(err.message).toBe(msg);
    }
  });

  it('should have undefined cause when none provided', () => {
    for (const def of errorDefs) {
      const err = def.factory('no cause');
      expect(err.cause).toBeUndefined();
    }
  });

  it('should support cause wrapping via AccountError constructor', () => {
    const underlying = new Error('underlying failure');
    const err = new AccountError(
      codes.ACCT_INVALID_ACCOUNT_TYPE,
      'InvalidAccountType',
      'Wrapping test',
      underlying,
    );
    expect(err.cause).toBe(underlying);
    expect(err.code).toBe('NEURON-ACCT-001');
  });

  it('factory functions should return distinct instances', () => {
    const a = invalidAccountType('first');
    const b = invalidAccountType('second');
    expect(a).not.toBe(b);
    expect(a.message).toBe('first');
    expect(b.message).toBe('second');
  });
});
