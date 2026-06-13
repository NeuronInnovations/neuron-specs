/**
 * T034: Signing Determinism tests — Constitution X, FR-A07.
 *
 * Source: specs/002-key-library/spec.md, specs/006-protocol-determinism/spec.md
 *
 * Constitution X (Deterministic Signing): RFC 6979 + Keccak256 + R||S||V
 * FR-A07: Deterministic nonce generation via RFC 6979.
 *
 * T034 verifies:
 *   - Signing the same message twice with the same key produces byte-identical signatures
 *   - Signing the same message with different keys produces different signatures
 *   - signHash with the same hash twice produces byte-identical signatures
 */

import { describe, it, expect } from 'vitest';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import { bytesToHex } from '../../src/wire/index.js';

describe('Signing Determinism (Constitution X)', () => {
  it('sign same message twice → byte-identical R||S||V', () => {
    const key = NeuronPrivateKey.fromHex('0x0000000000000000000000000000000000000000000000000000000000000001');
    const message = new TextEncoder().encode('test message');
    const sig1 = key.sign(message);
    const sig2 = key.sign(message);
    expect(bytesToHex(sig1.toBytes())).toBe(bytesToHex(sig2.toBytes()));
  });

  it('sign same message with different key → different signature', () => {
    const key1 = NeuronPrivateKey.fromHex('0x0000000000000000000000000000000000000000000000000000000000000001');
    const key2 = NeuronPrivateKey.fromHex('0x0000000000000000000000000000000000000000000000000000000000000002');
    const message = new TextEncoder().encode('test message');
    const sig1 = key1.sign(message);
    const sig2 = key2.sign(message);
    expect(bytesToHex(sig1.toBytes())).not.toBe(bytesToHex(sig2.toBytes()));
  });

  it('signHash with same hash twice → byte-identical', () => {
    const key = NeuronPrivateKey.fromHex('0x0000000000000000000000000000000000000000000000000000000000000001');
    const hash = new Uint8Array(32).fill(0xab);
    const sig1 = key.signHash(hash);
    const sig2 = key.signHash(hash);
    expect(bytesToHex(sig1.toBytes())).toBe(bytesToHex(sig2.toBytes()));
  });
});
