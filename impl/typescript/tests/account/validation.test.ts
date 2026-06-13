/**
 * T010: Validation tests -- type-specific rule enforcement.
 *
 * Source: 001 spec.md, data-model.md
 * Verifies: FR-006, FR-007, FR-007a, FR-014, SC-005.
 *
 * SC-005: All violations reported, not just the first.
 * Tests V-PARENT-01..05, V-CHILD-01..05, V-SHARED-01..04.
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import type { NeuronPublicKey } from '../../src/keylib/public-key.js';
import { MultisigKey } from '../../src/keylib/multisig-key.js';
import { AccountType, createNeuronDID } from '../../src/account/types.js';
import type { NeuronDID } from '../../src/account/types.js';
import {
  ParentAccountBuilder,
  ChildAccountBuilder,
  SharedAccountBuilder,
} from '../../src/account/builder.js';
import { validateAccount } from '../../src/account/validation.js';
import type { ValidationError } from '../../src/account/validation.js';
import {
  ACCT_MISSING_REQUIRED_FIELD,
} from '../../src/errors.js';

// --- Test fixtures ---

let testPubKey: NeuronPublicKey;
let testParentPubKey: NeuronPublicKey;
let testDID: NeuronDID;
let testMultisigKey: MultisigKey;

beforeAll(() => {
  const key1 = NeuronPrivateKey.fromHex(
    '0x0000000000000000000000000000000000000000000000000000000000000001',
  );
  const key2 = NeuronPrivateKey.fromHex(
    '0x0000000000000000000000000000000000000000000000000000000000000002',
  );

  testPubKey = key1.publicKey();
  testParentPubKey = key2.publicKey();
  testDID = createNeuronDID(testPubKey.didKey().toString());
  testMultisigKey = MultisigKey.fromConfig('secp256k1-aggregated', 2, 3);
});

describe('validateAccount -- Parent rules', () => {
  it('should return empty array for valid Parent account', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    const errors = validateAccount(parent);
    expect(errors).toHaveLength(0);
  });

  it('V-PARENT-01: reports missing DID', () => {
    // Construct a structurally invalid Parent account by bypassing the builder
    const invalid = {
      accountType: AccountType.Parent as const,
      publicKey: testPubKey,
      currencySymbol: 'HBAR',
      // did is missing
    };

    const errors = validateAccount(invalid as any);
    const didError = errors.find((e: ValidationError) => e.rule === 'V-PARENT-01');
    expect(didError).toBeDefined();
    expect(didError?.code).toBe(ACCT_MISSING_REQUIRED_FIELD);
    expect(didError?.field).toBe('did');
  });

  it('V-PARENT-02: reports missing publicKey', () => {
    const invalid = {
      accountType: AccountType.Parent as const,
      did: testDID,
      currencySymbol: 'HBAR',
      // publicKey is missing
    };

    const errors = validateAccount(invalid as any);
    const pkError = errors.find((e: ValidationError) => e.rule === 'V-PARENT-02');
    expect(pkError).toBeDefined();
    expect(pkError?.code).toBe(ACCT_MISSING_REQUIRED_FIELD);
    expect(pkError?.field).toBe('publicKey');
  });

  it('SC-005: reports ALL violations, not just the first', () => {
    // Both did and publicKey are missing
    const invalid = {
      accountType: AccountType.Parent as const,
      currencySymbol: 'HBAR',
    };

    const errors = validateAccount(invalid as any);
    expect(errors.length).toBeGreaterThanOrEqual(2);

    const ruleIds = errors.map((e: ValidationError) => e.rule);
    expect(ruleIds).toContain('V-PARENT-01');
    expect(ruleIds).toContain('V-PARENT-02');
  });
});

describe('validateAccount -- Child rules', () => {
  it('should return empty array for valid Child account', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();

    const errors = validateAccount(child);
    expect(errors).toHaveLength(0);
  });

  it('V-CHILD-01: reports missing parentPubKey', () => {
    const invalid = {
      accountType: AccountType.Child as const,
      publicKey: testPubKey,
      currencySymbol: 'HBAR',
      // parentPubKey is missing
    };

    const errors = validateAccount(invalid as any);
    const parentKeyError = errors.find((e: ValidationError) => e.rule === 'V-CHILD-01');
    expect(parentKeyError).toBeDefined();
    expect(parentKeyError?.code).toBe(ACCT_MISSING_REQUIRED_FIELD);
    expect(parentKeyError?.field).toBe('parentPubKey');
  });

  it('V-CHILD-02: reports missing publicKey', () => {
    const invalid = {
      accountType: AccountType.Child as const,
      parentPubKey: testParentPubKey,
      currencySymbol: 'HBAR',
      // publicKey is missing
    };

    const errors = validateAccount(invalid as any);
    const pkError = errors.find((e: ValidationError) => e.rule === 'V-CHILD-02');
    expect(pkError).toBeDefined();
    expect(pkError?.code).toBe(ACCT_MISSING_REQUIRED_FIELD);
    expect(pkError?.field).toBe('publicKey');
  });

  it('SC-005: reports ALL Child violations', () => {
    const invalid = {
      accountType: AccountType.Child as const,
      currencySymbol: 'HBAR',
    };

    const errors = validateAccount(invalid as any);
    expect(errors.length).toBeGreaterThanOrEqual(2);

    const ruleIds = errors.map((e: ValidationError) => e.rule);
    expect(ruleIds).toContain('V-CHILD-01');
    expect(ruleIds).toContain('V-CHILD-02');
  });

  it('V-CHILD-04: DID absence is NOT an error for Child', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();

    const errors = validateAccount(child);
    // No error about missing DID
    const didError = errors.find((e: ValidationError) => e.field === 'did');
    expect(didError).toBeUndefined();
  });
});

describe('validateAccount -- Shared rules', () => {
  it('should return empty array for valid Shared account', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .build();

    const errors = validateAccount(shared);
    expect(errors).toHaveLength(0);
  });

  it('V-SHARED-01: reports missing multisigKey', () => {
    const invalid = {
      accountType: AccountType.Shared as const,
      currencySymbol: 'HBAR',
      // multisigKey is missing
    };

    const errors = validateAccount(invalid as any);
    const msError = errors.find((e: ValidationError) => e.rule === 'V-SHARED-01');
    expect(msError).toBeDefined();
    expect(msError?.code).toBe(ACCT_MISSING_REQUIRED_FIELD);
    expect(msError?.field).toBe('multisigKey');
  });
});

describe('validateAccount -- return type', () => {
  it('should return an array (possibly empty)', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    const errors = validateAccount(parent);
    expect(Array.isArray(errors)).toBe(true);
  });

  it('each error has field, rule, code, and message', () => {
    const invalid = {
      accountType: AccountType.Parent as const,
      currencySymbol: 'HBAR',
    };

    const errors = validateAccount(invalid as any);
    for (const err of errors) {
      expect(typeof err.field).toBe('string');
      expect(typeof err.rule).toBe('string');
      expect(typeof err.code).toBe('string');
      expect(typeof err.message).toBe('string');
      expect(err.code).toMatch(/^NEURON-ACCT-\d{3}$/);
    }
  });
});
