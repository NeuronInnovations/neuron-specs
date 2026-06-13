/**
 * T005-T007: Builder tests -- ParentAccountBuilder, ChildAccountBuilder, SharedAccountBuilder.
 *
 * Source: 001 spec.md, contracts/account-builder.md
 * Verifies: FR-001, FR-002, FR-006, FR-007, FR-007a, FR-011, FR-011a,
 *           FR-020, FR-022, FR-026, SC-001, SC-002, SC-005, SC-012.
 *
 * Test key fixtures:
 *   Key 1: 0x0000...0001 (used as Child/Parent publicKey)
 *   Key 2: 0x0000...0002 (used as Parent publicKey in Child builder)
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import type { NeuronPublicKey } from '../../src/keylib/public-key.js';
import { MultisigKey } from '../../src/keylib/multisig-key.js';
import { AccountError } from '../../src/account/errors.js';
import { AccountType, createNeuronDID } from '../../src/account/types.js';
import type { LedgerAttachment, RegistryBinding, NeuronDID } from '../../src/account/types.js';
import {
  ParentAccountBuilder,
  ChildAccountBuilder,
  SharedAccountBuilder,
} from '../../src/account/builder.js';

// --- Test fixtures ---

let testPubKey: NeuronPublicKey;
let testParentPubKey: NeuronPublicKey;
let testDID: NeuronDID;
let testLedgerAttachment: LedgerAttachment;
let testRegistryBinding: RegistryBinding;
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

  // DID derived from key1's public key via DIDKey
  testDID = createNeuronDID(testPubKey.didKey().toString());

  testLedgerAttachment = {
    ledgerIdentifier: 'ethereum-mainnet',
    attachedAddress: testPubKey.evmAddress(),
    state: 'attached',
    verificationStatus: 'verified',
    lastSyncedAt: new Date('2026-01-01T00:00:00Z'),
  };

  testRegistryBinding = {
    registryIdentifier: 'eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18',
    externalId: '42',
  };

  testMultisigKey = MultisigKey.fromConfig('secp256k1-aggregated', 2, 3);
});

// === T005: ParentAccountBuilder ===

describe('ParentAccountBuilder (T005)', () => {
  it('should build a valid Parent account with all required fields', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    expect(parent.accountType).toBe(AccountType.Parent);
    expect(parent.publicKey).toBe(testPubKey);
    expect(parent.did.identifier).toBe(testDID.identifier);
    expect(parent.currencySymbol).toBe('HBAR');
  });

  it('should build Parent with optional ledger attachment', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('ETH')
      .withLedgerAttachment(testLedgerAttachment)
      .build();

    expect(parent.ledgerAttachment).toBeDefined();
    expect(parent.ledgerAttachment?.ledgerIdentifier).toBe('ethereum-mainnet');
  });

  it('should have undefined ledgerAttachment when not provided', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    expect(parent.ledgerAttachment).toBeUndefined();
  });

  it('should have undefined creditBalance by default', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    expect(parent.creditBalance).toBeUndefined();
  });

  // Missing required fields
  it('should throw NEURON-ACCT-002 when publicKey is missing', () => {
    expect(() => {
      new ParentAccountBuilder()
        .withDID(testDID)
        .withCurrency('HBAR')
        .build();
    }).toThrow(AccountError);

    try {
      new ParentAccountBuilder()
        .withDID(testDID)
        .withCurrency('HBAR')
        .build();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-002');
    }
  });

  it('should throw NEURON-ACCT-002 when DID is missing', () => {
    expect(() => {
      new ParentAccountBuilder()
        .withPublicKey(testPubKey)
        .withCurrency('HBAR')
        .build();
    }).toThrow(AccountError);

    try {
      new ParentAccountBuilder()
        .withPublicKey(testPubKey)
        .withCurrency('HBAR')
        .build();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-002');
    }
  });

  it('should throw NEURON-ACCT-008 when currency is missing', () => {
    expect(() => {
      new ParentAccountBuilder()
        .withPublicKey(testPubKey)
        .withDID(testDID)
        .build();
    }).toThrow(AccountError);

    try {
      new ParentAccountBuilder()
        .withPublicKey(testPubKey)
        .withDID(testDID)
        .build();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-008');
    }
  });

  it('should throw NEURON-ACCT-008 when currency is empty string', () => {
    expect(() => {
      new ParentAccountBuilder()
        .withPublicKey(testPubKey)
        .withDID(testDID)
        .withCurrency('')
        .build();
    }).toThrow(AccountError);

    try {
      new ParentAccountBuilder()
        .withPublicKey(testPubKey)
        .withDID(testDID)
        .withCurrency('')
        .build();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-008');
    }
  });

  // buildComplete()
  it('should succeed in buildComplete when ledger attachment is provided', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .withLedgerAttachment(testLedgerAttachment)
      .buildComplete();

    expect(parent.ledgerAttachment).toBeDefined();
  });

  it('should throw NEURON-ACCT-007 in buildComplete without ledger attachment', () => {
    expect(() => {
      new ParentAccountBuilder()
        .withPublicKey(testPubKey)
        .withDID(testDID)
        .withCurrency('HBAR')
        .buildComplete();
    }).toThrow(AccountError);

    try {
      new ParentAccountBuilder()
        .withPublicKey(testPubKey)
        .withDID(testDID)
        .withCurrency('HBAR')
        .buildComplete();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-007');
    }
  });

  // Fluent chaining returns same builder
  it('should support fluent chaining (returns this)', () => {
    const builder = new ParentAccountBuilder();
    const returned = builder.withPublicKey(testPubKey);
    expect(returned).toBe(builder);
  });
});

// === T006: ChildAccountBuilder ===

describe('ChildAccountBuilder (T006)', () => {
  it('should build a valid Child account with all required fields', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();

    expect(child.accountType).toBe(AccountType.Child);
    expect(child.publicKey).toBe(testPubKey);
    expect(child.parentPubKey).toBe(testParentPubKey);
    expect(child.currencySymbol).toBe('HBAR');
  });

  it('should build Child with optional registry binding', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .withRegistryBinding(testRegistryBinding)
      .build();

    expect(child.registryBinding).toBeDefined();
    expect(child.registryBinding?.externalId).toBe('42');
  });

  it('should build Child with optional fee payer', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .withFeePayer('0.0.12345')
      .build();

    expect(child.feePayer).toBe('0.0.12345');
  });

  it('should build Child with optional ledger attachment', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .withLedgerAttachment(testLedgerAttachment)
      .build();

    expect(child.ledgerAttachment).toBeDefined();
  });

  it('should have undefined optional fields when not provided', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();

    expect(child.registryBinding).toBeUndefined();
    expect(child.feePayer).toBeUndefined();
    expect(child.ledgerAttachment).toBeUndefined();
    expect(child.balanceAllocation).toBeUndefined();
  });

  // Missing required fields
  it('should throw NEURON-ACCT-002 when publicKey is missing', () => {
    expect(() => {
      new ChildAccountBuilder()
        .withParentPublicKey(testParentPubKey)
        .withCurrency('HBAR')
        .build();
    }).toThrow(AccountError);

    try {
      new ChildAccountBuilder()
        .withParentPublicKey(testParentPubKey)
        .withCurrency('HBAR')
        .build();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-002');
    }
  });

  it('should throw NEURON-ACCT-002 when parentPubKey is missing', () => {
    expect(() => {
      new ChildAccountBuilder()
        .withPublicKey(testPubKey)
        .withCurrency('HBAR')
        .build();
    }).toThrow(AccountError);

    try {
      new ChildAccountBuilder()
        .withPublicKey(testPubKey)
        .withCurrency('HBAR')
        .build();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-002');
    }
  });

  it('should throw NEURON-ACCT-008 when currency is missing', () => {
    expect(() => {
      new ChildAccountBuilder()
        .withPublicKey(testPubKey)
        .withParentPublicKey(testParentPubKey)
        .build();
    }).toThrow(AccountError);

    try {
      new ChildAccountBuilder()
        .withPublicKey(testPubKey)
        .withParentPublicKey(testParentPubKey)
        .build();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-008');
    }
  });

  // buildComplete()
  it('should succeed in buildComplete when registry binding is provided', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .withRegistryBinding(testRegistryBinding)
      .buildComplete();

    expect(child.registryBinding).toBeDefined();
  });

  it('should throw NEURON-ACCT-007 in buildComplete without registry binding', () => {
    expect(() => {
      new ChildAccountBuilder()
        .withPublicKey(testPubKey)
        .withParentPublicKey(testParentPubKey)
        .withCurrency('HBAR')
        .buildComplete();
    }).toThrow(AccountError);

    try {
      new ChildAccountBuilder()
        .withPublicKey(testPubKey)
        .withParentPublicKey(testParentPubKey)
        .withCurrency('HBAR')
        .buildComplete();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-007');
    }
  });

  // Fluent chaining
  it('should support fluent chaining (returns this)', () => {
    const builder = new ChildAccountBuilder();
    const returned = builder.withPublicKey(testPubKey);
    expect(returned).toBe(builder);
  });
});

// === T007: SharedAccountBuilder ===

describe('SharedAccountBuilder (T007)', () => {
  it('should build a valid Shared account with all required fields', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .build();

    expect(shared.accountType).toBe(AccountType.Shared);
    expect(shared.multisigKey).toBe(testMultisigKey);
    expect(shared.currencySymbol).toBe('HBAR');
  });

  it('should build Shared with optional ledger attachment', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('ETH')
      .withLedgerAttachment(testLedgerAttachment)
      .build();

    expect(shared.ledgerAttachment).toBeDefined();
  });

  it('should have undefined optional fields when not provided', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .build();

    expect(shared.ledgerAttachment).toBeUndefined();
    expect(shared.balance).toBeUndefined();
  });

  // Missing required fields
  it('should throw NEURON-ACCT-002 when multisigKey is missing', () => {
    expect(() => {
      new SharedAccountBuilder()
        .withCurrency('HBAR')
        .build();
    }).toThrow(AccountError);

    try {
      new SharedAccountBuilder()
        .withCurrency('HBAR')
        .build();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-002');
    }
  });

  it('should throw NEURON-ACCT-008 when currency is missing', () => {
    expect(() => {
      new SharedAccountBuilder()
        .withMultisigKey(testMultisigKey)
        .build();
    }).toThrow(AccountError);

    try {
      new SharedAccountBuilder()
        .withMultisigKey(testMultisigKey)
        .build();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-008');
    }
  });

  // buildComplete()
  it('should succeed in buildComplete when ledger attachment is provided', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .withLedgerAttachment(testLedgerAttachment)
      .buildComplete();

    expect(shared.ledgerAttachment).toBeDefined();
  });

  it('should throw NEURON-ACCT-007 in buildComplete without ledger attachment', () => {
    expect(() => {
      new SharedAccountBuilder()
        .withMultisigKey(testMultisigKey)
        .withCurrency('HBAR')
        .buildComplete();
    }).toThrow(AccountError);

    try {
      new SharedAccountBuilder()
        .withMultisigKey(testMultisigKey)
        .withCurrency('HBAR')
        .buildComplete();
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-007');
    }
  });

  // MultisigKey properties accessible
  it('should preserve MultisigKey properties', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .build();

    expect(shared.multisigKey.protocol()).toBe('secp256k1-aggregated');
    expect(shared.multisigKey.threshold()).toBe(2);
    expect(shared.multisigKey.totalKeys()).toBe(3);
  });

  // Fluent chaining
  it('should support fluent chaining (returns this)', () => {
    const builder = new SharedAccountBuilder();
    const returned = builder.withMultisigKey(testMultisigKey);
    expect(returned).toBe(builder);
  });
});

// === Cross-builder tests ===

describe('Builder cross-type isolation', () => {
  it('ParentAccountBuilder produces accountType = Parent (1)', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();
    expect(parent.accountType).toBe(1);
  });

  it('ChildAccountBuilder produces accountType = Child (2)', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();
    expect(child.accountType).toBe(2);
  });

  it('SharedAccountBuilder produces accountType = Shared (3)', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .build();
    expect(shared.accountType).toBe(3);
  });
});
