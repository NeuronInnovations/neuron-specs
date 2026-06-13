/**
 * Chain 4: Key Encryption Round-Trip — Golden test vector conformance.
 *
 * Source: specs/006-protocol-determinism/contracts/test-vectors.md §Chain 4
 * Verifies Argon2id key encryption and decryption round-trip.
 * Uses zero salt/nonce for deterministic test vector generation only.
 */

import { describe, it, expect } from 'vitest';
import { CHAIN4 } from './vectors.js';
import { hexToBytes, bytesToHex } from '../../src/wire/index.js';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import { EncryptedPrivateKey } from '../../src/keylib/encrypted-key.js';

describe('Chain 4: Key Encryption Round-Trip', () => {
  const privateKeyBytes = hexToBytes(CHAIN4.privateKeyHex);
  const salt = hexToBytes(CHAIN4.saltHex);
  const nonce = hexToBytes(CHAIN4.nonceHex);

  // §4.1 Argon2id Key Derivation
  // FR-A11: Argon2id(password_utf8, salt, time=1, memory=65536, threads=4, tag_length=32)
  it('should derive correct Argon2id key from password and salt', async () => {
    const derivedKey = await EncryptedPrivateKey.deriveKey(
      CHAIN4.password,
      salt,
      CHAIN4.argon2Params,
    );
    expect(bytesToHex(derivedKey).toLowerCase()).toBe(CHAIN4.derivedKeyHex.toLowerCase());
  });

  // §4.2 AES-256-GCM Encryption
  // FR-A11: AES-256-GCM-Encrypt(key, nonce, private_key_bytes) → 48 bytes (32 + 16 tag)
  it('should produce correct AES-GCM ciphertext', async () => {
    const encrypted = await EncryptedPrivateKey.encrypt(
      privateKeyBytes,
      CHAIN4.password,
      salt,
      nonce,
    );
    expect(bytesToHex(encrypted.ciphertextBytes()).toLowerCase()).toBe(
      CHAIN4.ciphertextHex.toLowerCase(),
    );
  });

  // §4.3 EncryptedPrivateKey JSON
  // FR-W05: Field order: version → salt → nonce → ciphertext
  // FR-W03: Binary fields as base64
  it('should produce correct EncryptedPrivateKey JSON', async () => {
    const encrypted = await EncryptedPrivateKey.encrypt(
      privateKeyBytes,
      CHAIN4.password,
      salt,
      nonce,
    );
    expect(encrypted.toCanonicalJson()).toBe(CHAIN4.encryptedKeyJson);
  });

  // §4.4 Decryption Verification
  // Round-trip: encrypt → decrypt → verify original key recovered
  it('should decrypt back to original private key', async () => {
    const encrypted = await EncryptedPrivateKey.encrypt(
      privateKeyBytes,
      CHAIN4.password,
      salt,
      nonce,
    );
    const decrypted = await encrypted.decrypt(CHAIN4.password);
    expect(bytesToHex(decrypted).toLowerCase()).toBe(CHAIN4.privateKeyHex.toLowerCase());
  });

  // Parse from JSON and decrypt
  it('should parse EncryptedPrivateKey from JSON and decrypt', async () => {
    const encrypted = EncryptedPrivateKey.fromJson(CHAIN4.encryptedKeyJson);
    const decrypted = await encrypted.decrypt(CHAIN4.password);
    expect(bytesToHex(decrypted).toLowerCase()).toBe(CHAIN4.privateKeyHex.toLowerCase());
  });
});
