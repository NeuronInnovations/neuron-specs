/**
 * T032: MultisigKey tests — FR-023, FR-024, GAP-005.
 *
 * Source: specs/002-key-library/spec.md
 * Phase: TDD Red — these tests MUST be written before implementation.
 *
 * T032 verifies:
 *   - Construction with "secp256k1-aggregated" protocol and m-of-n threshold
 *   - protocol() returns the correct protocol identifier
 *   - threshold() returns the correct m value
 *   - totalKeys() returns the correct n value
 *   - evmAddress() throws NEURON-KEY-002 for non-secp256k1 protocols (GAP-005)
 *   - peerId() throws NEURON-KEY-002 for non-secp256k1 protocols (GAP-005)
 *   - evmAddress() throws NEURON-KEY-002 even for "secp256k1-aggregated" (GAP-005)
 *   - peerId() throws NEURON-KEY-002 even for "secp256k1-aggregated" (GAP-005)
 *   - Construction with "hedera-threshold" protocol works
 *   - Construction rejects threshold < 1
 *   - Construction rejects threshold > totalKeys
 */

import { describe, it, expect } from 'vitest';
import { MultisigKey } from '../../src/keylib/multisig-key.js';
import { KeyError } from '../../src/keylib/errors.js';

describe('T032: MultisigKey', () => {
  // --- Construction (FR-023) ---

  it('should construct with "secp256k1-aggregated" protocol and m-of-n threshold', () => {
    // FR-023: MultisigKey declares protocol + m-of-n threshold
    const key = MultisigKey.fromConfig('secp256k1-aggregated', 2, 3);
    expect(key).toBeInstanceOf(MultisigKey);
  });

  it('should construct with "hedera-threshold" protocol', () => {
    // FR-023: Supports arbitrary protocol identifiers
    const key = MultisigKey.fromConfig('hedera-threshold', 3, 5);
    expect(key).toBeInstanceOf(MultisigKey);
  });

  it('should construct with threshold equal to totalKeys (n-of-n)', () => {
    const key = MultisigKey.fromConfig('secp256k1-aggregated', 3, 3);
    expect(key.threshold()).toBe(3);
    expect(key.totalKeys()).toBe(3);
  });

  it('should construct with threshold of 1 (1-of-n)', () => {
    const key = MultisigKey.fromConfig('secp256k1-aggregated', 1, 5);
    expect(key.threshold()).toBe(1);
    expect(key.totalKeys()).toBe(5);
  });

  // --- Accessors (FR-024) ---

  it('protocol() should return the correct protocol identifier', () => {
    // FR-024: protocol() accessor
    const key = MultisigKey.fromConfig('secp256k1-aggregated', 2, 3);
    expect(key.protocol()).toBe('secp256k1-aggregated');
  });

  it('threshold() should return the correct m value', () => {
    // FR-024: threshold() accessor
    const key = MultisigKey.fromConfig('secp256k1-aggregated', 2, 3);
    expect(key.threshold()).toBe(2);
  });

  it('totalKeys() should return the correct n value', () => {
    // FR-024: totalKeys() accessor
    const key = MultisigKey.fromConfig('secp256k1-aggregated', 2, 3);
    expect(key.totalKeys()).toBe(3);
  });

  it('protocol() should return "hedera-threshold" for hedera config', () => {
    const key = MultisigKey.fromConfig('hedera-threshold', 3, 5);
    expect(key.protocol()).toBe('hedera-threshold');
  });

  // --- EVM Address derivation (GAP-005) ---

  it('evmAddress() should throw NEURON-KEY-002 for "hedera-threshold" protocol', () => {
    // GAP-005: Aggregated key derivation not available
    const key = MultisigKey.fromConfig('hedera-threshold', 2, 3);

    expect(() => key.evmAddress()).toThrow(KeyError);
    try {
      key.evmAddress();
    } catch (e) {
      expect(e).toBeInstanceOf(KeyError);
      const err = e as KeyError;
      expect(err.code).toBe('NEURON-KEY-002');
    }
  });

  it('evmAddress() should throw NEURON-KEY-002 even for "secp256k1-aggregated" (GAP-005)', () => {
    // GAP-005: Even secp256k1-aggregated cannot derive EVM address yet
    const key = MultisigKey.fromConfig('secp256k1-aggregated', 2, 3);

    expect(() => key.evmAddress()).toThrow(KeyError);
    try {
      key.evmAddress();
    } catch (e) {
      expect(e).toBeInstanceOf(KeyError);
      const err = e as KeyError;
      expect(err.code).toBe('NEURON-KEY-002');
    }
  });

  // --- PeerID derivation (GAP-005) ---

  it('peerId() should throw NEURON-KEY-002 for "hedera-threshold" protocol', () => {
    // GAP-005: Aggregated key derivation not available
    const key = MultisigKey.fromConfig('hedera-threshold', 2, 3);

    expect(() => key.peerId()).toThrow(KeyError);
    try {
      key.peerId();
    } catch (e) {
      expect(e).toBeInstanceOf(KeyError);
      const err = e as KeyError;
      expect(err.code).toBe('NEURON-KEY-002');
    }
  });

  it('peerId() should throw NEURON-KEY-002 even for "secp256k1-aggregated" (GAP-005)', () => {
    // GAP-005: Even secp256k1-aggregated cannot derive PeerID yet
    const key = MultisigKey.fromConfig('secp256k1-aggregated', 2, 3);

    expect(() => key.peerId()).toThrow(KeyError);
    try {
      key.peerId();
    } catch (e) {
      expect(e).toBeInstanceOf(KeyError);
      const err = e as KeyError;
      expect(err.code).toBe('NEURON-KEY-002');
    }
  });

  // --- Validation ---

  it('should reject threshold < 1', () => {
    expect(() => MultisigKey.fromConfig('secp256k1-aggregated', 0, 3)).toThrow(KeyError);
    try {
      MultisigKey.fromConfig('secp256k1-aggregated', 0, 3);
    } catch (e) {
      expect(e).toBeInstanceOf(KeyError);
      const err = e as KeyError;
      expect(err.code).toBe('NEURON-KEY-001');
    }
  });

  it('should reject threshold > totalKeys', () => {
    expect(() => MultisigKey.fromConfig('secp256k1-aggregated', 5, 3)).toThrow(KeyError);
    try {
      MultisigKey.fromConfig('secp256k1-aggregated', 5, 3);
    } catch (e) {
      expect(e).toBeInstanceOf(KeyError);
      const err = e as KeyError;
      expect(err.code).toBe('NEURON-KEY-001');
    }
  });
});
