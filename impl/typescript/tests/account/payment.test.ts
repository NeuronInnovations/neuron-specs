/**
 * T014: Payment address tests -- paymentAddress() resolution.
 *
 * Source: 001 spec.md
 * Verifies: FR-023 (Parent returns own EVM address),
 *           FR-024 (Child returns Parent's EVM address),
 *           SC-011 (deterministic resolution).
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import type { NeuronPublicKey } from '../../src/keylib/public-key.js';
import { MultisigKey } from '../../src/keylib/multisig-key.js';
import { AccountError } from '../../src/account/errors.js';
import { AccountType, createNeuronDID } from '../../src/account/types.js';
import type { NeuronDID } from '../../src/account/types.js';
import {
  ParentAccountBuilder,
  ChildAccountBuilder,
  SharedAccountBuilder,
} from '../../src/account/builder.js';
import { paymentAddress } from '../../src/account/payment.js';
import { isParent, isChild, isShared, evmAddress, peerId } from '../../src/account/account.js';

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

describe('paymentAddress (FR-023, FR-024)', () => {
  it('FR-023: Parent returns own EVM address', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    const addr = paymentAddress(parent);
    const expected = testPubKey.evmAddress();
    expect(addr.toString()).toBe(expected.toString());
  });

  it('FR-024: Child returns Parent EVM address (from parentPubKey)', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();

    const addr = paymentAddress(child);
    const expected = testParentPubKey.evmAddress();
    expect(addr.toString()).toBe(expected.toString());
  });

  it('FR-024: Child payment address differs from Child own EVM address', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();

    const payment = paymentAddress(child);
    const ownAddress = testPubKey.evmAddress();
    // Parent key != Child key, so addresses differ
    expect(payment.toString()).not.toBe(ownAddress.toString());
  });

  it('Shared throws NEURON-ACCT-001', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .build();

    expect(() => paymentAddress(shared)).toThrow(AccountError);

    try {
      paymentAddress(shared);
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-001');
      expect((e as AccountError).message).toContain('Shared');
    }
  });

  it('SC-011: payment address is deterministic (same input produces same output)', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    const addr1 = paymentAddress(parent);
    const addr2 = paymentAddress(parent);
    expect(addr1.toString()).toBe(addr2.toString());
  });
});

describe('evmAddress utility function (FR-008)', () => {
  it('should return EVM address for Parent', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    const addr = evmAddress(parent);
    expect(addr.toString()).toBe(testPubKey.evmAddress().toString());
  });

  it('should return EVM address for Child', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();

    const addr = evmAddress(child);
    expect(addr.toString()).toBe(testPubKey.evmAddress().toString());
  });

  it('should throw NEURON-ACCT-001 for Shared', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .build();

    expect(() => evmAddress(shared)).toThrow(AccountError);

    try {
      evmAddress(shared);
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-001');
    }
  });
});

describe('peerId utility function (FR-008)', () => {
  it('should return PeerID for Parent', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    const pid = peerId(parent);
    expect(pid.toString()).toBe(testPubKey.peerId().toString());
  });

  it('should return PeerID for Child', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();

    const pid = peerId(child);
    expect(pid.toString()).toBe(testPubKey.peerId().toString());
  });

  it('should throw NEURON-ACCT-001 for Shared', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .build();

    expect(() => peerId(shared)).toThrow(AccountError);

    try {
      peerId(shared);
    } catch (e) {
      expect((e as AccountError).code).toBe('NEURON-ACCT-001');
    }
  });
});

describe('Type guards (isParent, isChild, isShared)', () => {
  it('isParent returns true for Parent account', () => {
    const parent = new ParentAccountBuilder()
      .withPublicKey(testPubKey)
      .withDID(testDID)
      .withCurrency('HBAR')
      .build();

    expect(isParent(parent)).toBe(true);
    expect(isChild(parent)).toBe(false);
    expect(isShared(parent)).toBe(false);
  });

  it('isChild returns true for Child account', () => {
    const child = new ChildAccountBuilder()
      .withPublicKey(testPubKey)
      .withParentPublicKey(testParentPubKey)
      .withCurrency('HBAR')
      .build();

    expect(isParent(child)).toBe(false);
    expect(isChild(child)).toBe(true);
    expect(isShared(child)).toBe(false);
  });

  it('isShared returns true for Shared account', () => {
    const shared = new SharedAccountBuilder()
      .withMultisigKey(testMultisigKey)
      .withCurrency('HBAR')
      .build();

    expect(isParent(shared)).toBe(false);
    expect(isChild(shared)).toBe(false);
    expect(isShared(shared)).toBe(true);
  });
});
