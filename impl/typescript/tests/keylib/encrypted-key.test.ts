/**
 * T027: EncryptedPrivateKey tests — FR-015, 006 §11.
 *
 * Source: specs/002-key-library/spec.md, specs/006-protocol-determinism/spec.md §11
 * Phase: TDD Red — these tests MUST be written before implementation.
 *
 * Verifies:
 *   - Argon2id key derivation produces correct derived key (Chain 4 vector)
 *   - AES-256-GCM encryption produces correct ciphertext (Chain 4 vector)
 *   - Canonical JSON field order: version -> salt -> nonce -> ciphertext (FR-W05)
 *   - Decryption round-trip recovers original private key bytes
 *   - Parse from JSON (fromJson) and decrypt
 *   - Wrong password rejects with NEURON-KEY-009
 *
 * All tests are async because Argon2id and AES-GCM operations are async.
 *
 * SEC-003: No private key material appears in error messages.
 */

import { describe, it, expect } from 'vitest';
import { CHAIN4 } from '../conformance/vectors.js';
import { hexToBytes, bytesToHex } from '../../src/wire/index.js';
import { EncryptedPrivateKey } from '../../src/keylib/encrypted-key.js';
import { KeyError } from '../../src/keylib/errors.js';

describe('T027: EncryptedPrivateKey', () => {
  // Pre-compute shared byte arrays from Chain 4 test vectors
  const privateKeyBytes = hexToBytes(CHAIN4.privateKeyHex);
  const salt = hexToBytes(CHAIN4.saltHex);
  const nonce = hexToBytes(CHAIN4.nonceHex);

  // ---------------------------------------------------------------------------
  // Argon2id Key Derivation (006 §11, FR-A11)
  // ---------------------------------------------------------------------------

  describe('Argon2id Key Derivation', () => {
    it('should derive the correct key from password and salt (Chain 4 vector)', async () => {
      // FR-A11: Argon2id(password_utf8, salt, time=1, memory=65536, threads=4, tag_length=32)
      const derivedKey = await EncryptedPrivateKey.deriveKey(
        CHAIN4.password,
        salt,
        CHAIN4.argon2Params,
      );

      expect(derivedKey).toBeInstanceOf(Uint8Array);
      expect(derivedKey.length).toBe(32);
      expect(bytesToHex(derivedKey).toLowerCase()).toBe(
        CHAIN4.derivedKeyHex.toLowerCase(),
      );
    });
  });

  // ---------------------------------------------------------------------------
  // AES-256-GCM Encryption (006 §11, FR-A11)
  // ---------------------------------------------------------------------------

  describe('AES-256-GCM Encryption', () => {
    it('should produce correct ciphertext matching Chain 4 vector', async () => {
      // FR-A11: AES-256-GCM-Encrypt(derivedKey, nonce, privateKeyBytes) -> 48 bytes
      // Deterministic: uses explicit salt and nonce from test vectors (zero bytes)
      const encrypted = await EncryptedPrivateKey.encrypt(
        privateKeyBytes,
        CHAIN4.password,
        salt,
        nonce,
      );

      expect(encrypted.ciphertextBytes()).toBeInstanceOf(Uint8Array);
      expect(encrypted.ciphertextBytes().length).toBe(48); // 32 key + 16 GCM tag
      expect(bytesToHex(encrypted.ciphertextBytes()).toLowerCase()).toBe(
        CHAIN4.ciphertextHex.toLowerCase(),
      );
    });
  });

  // ---------------------------------------------------------------------------
  // EncryptedPrivateKey Canonical JSON (FR-W05)
  // ---------------------------------------------------------------------------

  describe('EncryptedPrivateKey JSON', () => {
    it('should produce canonical JSON matching Chain 4 vector (field order: version, salt, nonce, ciphertext)', async () => {
      // FR-W05: Field order is version -> salt -> nonce -> ciphertext
      // FR-W03: Binary fields encoded as base64
      const encrypted = await EncryptedPrivateKey.encrypt(
        privateKeyBytes,
        CHAIN4.password,
        salt,
        nonce,
      );

      const json = encrypted.toCanonicalJson();
      expect(json).toBe(CHAIN4.encryptedKeyJson);

      // Additionally verify field order by checking that "version" appears before "salt",
      // "salt" before "nonce", and "nonce" before "ciphertext"
      const versionIdx = json.indexOf('"version"');
      const saltIdx = json.indexOf('"salt"');
      const nonceIdx = json.indexOf('"nonce"');
      const ciphertextIdx = json.indexOf('"ciphertext"');

      expect(versionIdx).toBeLessThan(saltIdx);
      expect(saltIdx).toBeLessThan(nonceIdx);
      expect(nonceIdx).toBeLessThan(ciphertextIdx);
    });
  });

  // ---------------------------------------------------------------------------
  // Decryption Round-Trip
  // ---------------------------------------------------------------------------

  describe('Decryption Round-Trip', () => {
    it('should decrypt back to original private key bytes', async () => {
      // Encrypt then decrypt — must recover the original 32-byte private key
      const encrypted = await EncryptedPrivateKey.encrypt(
        privateKeyBytes,
        CHAIN4.password,
        salt,
        nonce,
      );

      const decrypted = await encrypted.decrypt(CHAIN4.password);

      expect(decrypted).toBeInstanceOf(Uint8Array);
      expect(decrypted.length).toBe(32);
      expect(bytesToHex(decrypted).toLowerCase()).toBe(
        CHAIN4.privateKeyHex.toLowerCase(),
      );
    });
  });

  // ---------------------------------------------------------------------------
  // Parse from JSON
  // ---------------------------------------------------------------------------

  describe('Parse from JSON', () => {
    it('should parse EncryptedPrivateKey from JSON string', () => {
      // fromJson should accept the canonical JSON and produce a valid object
      const encrypted = EncryptedPrivateKey.fromJson(CHAIN4.encryptedKeyJson);

      // The parsed object should have ciphertext matching the vector
      expect(encrypted.ciphertextBytes()).toBeInstanceOf(Uint8Array);
      expect(encrypted.ciphertextBytes().length).toBe(48);
      expect(bytesToHex(encrypted.ciphertextBytes()).toLowerCase()).toBe(
        CHAIN4.ciphertextHex.toLowerCase(),
      );
    });

    it('should decrypt a parsed EncryptedPrivateKey to recover original key', async () => {
      // Parse from canonical JSON, then decrypt with correct password
      const encrypted = EncryptedPrivateKey.fromJson(CHAIN4.encryptedKeyJson);
      const decrypted = await encrypted.decrypt(CHAIN4.password);

      expect(bytesToHex(decrypted).toLowerCase()).toBe(
        CHAIN4.privateKeyHex.toLowerCase(),
      );
    });
  });

  // ---------------------------------------------------------------------------
  // Wrong Password (NEURON-KEY-009)
  // ---------------------------------------------------------------------------

  describe('Wrong Password', () => {
    it('should reject decryption with wrong password (NEURON-KEY-009)', async () => {
      // NEURON-KEY-009: AES-GCM decryption failed (wrong password -> wrong derived key -> GCM tag mismatch)
      const encrypted = await EncryptedPrivateKey.encrypt(
        privateKeyBytes,
        CHAIN4.password,
        salt,
        nonce,
      );

      await expect(encrypted.decrypt('wrong-password-123')).rejects.toThrow(KeyError);

      try {
        await encrypted.decrypt('wrong-password-123');
      } catch (e) {
        expect(e).toBeInstanceOf(KeyError);
        const err = e as KeyError;
        expect(err.code).toBe('NEURON-KEY-009');
      }
    });

    it('should reject decryption of parsed JSON with wrong password (NEURON-KEY-009)', async () => {
      // Same test but starting from fromJson instead of encrypt
      const encrypted = EncryptedPrivateKey.fromJson(CHAIN4.encryptedKeyJson);

      await expect(encrypted.decrypt('wrong-password-123')).rejects.toThrow(KeyError);

      try {
        await encrypted.decrypt('wrong-password-123');
      } catch (e) {
        expect(e).toBeInstanceOf(KeyError);
        const err = e as KeyError;
        expect(err.code).toBe('NEURON-KEY-009');
      }
    });
  });
});
