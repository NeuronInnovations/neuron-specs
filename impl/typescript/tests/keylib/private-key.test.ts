/**
 * T025: Key Generation tests — FR-012, FR-A01.
 * T026: Mnemonic Restoration tests — FR-013, FR-A12, FR-A13.
 *
 * Source: specs/002-key-library/spec.md, specs/006-protocol-determinism/spec.md
 * Phase: TDD Red — these tests MUST be written before implementation.
 *
 * T025 verifies:
 *   - generate() produces a valid 32-byte non-zero key
 *   - Generated key can derive a public key
 *   - Generated key can sign a message
 *   - Two generate() calls produce different keys (overwhelming probability)
 *
 * T026 verifies:
 *   - fromMnemonic(validMnemonic) returns a valid key
 *   - Same mnemonic produces same key (deterministic, BIP-39/44)
 *   - Default derivation path is m/44'/60'/0'/0/0
 *   - Custom path produces a different key than default
 *   - Invalid mnemonic rejects with NEURON-KEY-010
 *   - Invalid checksum rejects with NEURON-KEY-010
 *
 * SEC-003: No private key material appears in error messages.
 */

import { describe, it, expect } from 'vitest';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import { KeyError } from '../../src/keylib/errors.js';
import { PRIVATE_KEY_LENGTH, BIP44_DEFAULT_PATH } from '../../src/keylib/constants.js';
import { bytesToHex } from '../../src/wire/index.js';

/**
 * Standard BIP-39 test vector mnemonic (12 words, all-zeros entropy).
 * This is the canonical test mnemonic used across Bitcoin/Ethereum tooling.
 */
const TEST_MNEMONIC =
  'abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about';

// ---------------------------------------------------------------------------
// T025 — Key Generation (FR-012)
// ---------------------------------------------------------------------------

describe('T025: NeuronPrivateKey.generate()', () => {
  it('should return a valid key with 32 bytes (non-zero)', () => {
    // FR-012: Cryptographically secure key generation
    // FR-A01: 1 <= k < n
    const key = NeuronPrivateKey.generate();
    const bytes = key.toBytes();

    expect(bytes).toBeInstanceOf(Uint8Array);
    expect(bytes.length).toBe(PRIVATE_KEY_LENGTH);

    // Must not be all zeros
    const allZero = bytes.every((b) => b === 0);
    expect(allZero).toBe(false);
  });

  it('should produce a key that can derive a public key', () => {
    // FR-001: NeuronPrivateKey derives NeuronPublicKey
    const key = NeuronPrivateKey.generate();
    const pubKey = key.publicKey();

    // Compressed public key is 33 bytes (0x02 or 0x03 prefix + 32 bytes X)
    const compressed = pubKey.toCompressedBytes();
    expect(compressed.length).toBe(33);
    expect(compressed[0] === 0x02 || compressed[0] === 0x03).toBe(true);
  });

  it('should produce a key that can sign a message', () => {
    // FR-014: ECDSA signing with R||S||V format (65 bytes)
    const key = NeuronPrivateKey.generate();
    const message = new Uint8Array([0x48, 0x65, 0x6c, 0x6c, 0x6f]); // "Hello"

    const sig = key.sign(message);
    expect(sig.toBytes().length).toBe(65);
  });

  it('should produce different keys on separate calls (overwhelming probability)', () => {
    // Two independently generated keys should differ.
    // The probability of collision is 1/2^256 — effectively impossible.
    const key1 = NeuronPrivateKey.generate();
    const key2 = NeuronPrivateKey.generate();

    const hex1 = bytesToHex(key1.toBytes());
    const hex2 = bytesToHex(key2.toBytes());

    expect(hex1).not.toBe(hex2);
  });
});

// ---------------------------------------------------------------------------
// T026 — Mnemonic Restoration (FR-013, 006 §12/§13)
// ---------------------------------------------------------------------------

describe('T026: NeuronPrivateKey.fromMnemonic()', () => {
  it('should return a valid key from a known BIP-39 mnemonic', () => {
    // FR-013: BIP-39 mnemonic validation + BIP-44 HD derivation
    // FR-A12: Mnemonic-to-seed via PBKDF2(HMAC-SHA512)
    const key = NeuronPrivateKey.fromMnemonic(TEST_MNEMONIC);
    const bytes = key.toBytes();

    expect(bytes).toBeInstanceOf(Uint8Array);
    expect(bytes.length).toBe(PRIVATE_KEY_LENGTH);

    // Must not be all zeros
    const allZero = bytes.every((b) => b === 0);
    expect(allZero).toBe(false);
  });

  it('should produce the same key from the same mnemonic (deterministic)', () => {
    // BIP-39/BIP-44 derivation is fully deterministic.
    // Same mnemonic + same path = same key, always.
    const key1 = NeuronPrivateKey.fromMnemonic(TEST_MNEMONIC);
    const key2 = NeuronPrivateKey.fromMnemonic(TEST_MNEMONIC);

    expect(bytesToHex(key1.toBytes())).toBe(bytesToHex(key2.toBytes()));
  });

  it('should use default path m/44\'/60\'/0\'/0/0', () => {
    // FR-A13: Default BIP-44 Ethereum path
    // Calling without explicit path should produce the same result
    // as calling with the default path explicitly.
    const keyDefault = NeuronPrivateKey.fromMnemonic(TEST_MNEMONIC);
    const keyExplicit = NeuronPrivateKey.fromMnemonic(TEST_MNEMONIC, BIP44_DEFAULT_PATH);

    expect(bytesToHex(keyDefault.toBytes())).toBe(bytesToHex(keyExplicit.toBytes()));
  });

  it('should produce a different key with a custom derivation path', () => {
    // A different BIP-44 path yields a different derived key.
    const keyDefault = NeuronPrivateKey.fromMnemonic(TEST_MNEMONIC);
    const keyCustom = NeuronPrivateKey.fromMnemonic(TEST_MNEMONIC, "m/44'/60'/0'/0/1");

    expect(bytesToHex(keyDefault.toBytes())).not.toBe(bytesToHex(keyCustom.toBytes()));
  });

  it('should reject an invalid mnemonic (wrong words) with NEURON-KEY-010', () => {
    // FR-008: Structured errors with specific error kinds
    // NEURON-KEY-010: Invalid mnemonic
    const invalidMnemonic = 'not a valid mnemonic phrase at all these words are wrong foo bar';

    expect(() => NeuronPrivateKey.fromMnemonic(invalidMnemonic)).toThrow(KeyError);
    try {
      NeuronPrivateKey.fromMnemonic(invalidMnemonic);
    } catch (e) {
      expect(e).toBeInstanceOf(KeyError);
      const err = e as KeyError;
      expect(err.code).toBe('NEURON-KEY-010');
    }
  });

  it('should reject a mnemonic with invalid checksum with NEURON-KEY-010', () => {
    // BIP-39 mnemonics include a checksum. Swapping the last word
    // with a different valid BIP-39 word creates a checksum mismatch.
    const badChecksum =
      'abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon';

    expect(() => NeuronPrivateKey.fromMnemonic(badChecksum)).toThrow(KeyError);
    try {
      NeuronPrivateKey.fromMnemonic(badChecksum);
    } catch (e) {
      expect(e).toBeInstanceOf(KeyError);
      const err = e as KeyError;
      expect(err.code).toBe('NEURON-KEY-010');
    }
  });
});
