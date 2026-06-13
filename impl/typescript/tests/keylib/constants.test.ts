/**
 * T005: Constants tests — named constants from 006 algorithm-reference.md §1.
 *
 * FR-001, FR-005, FR-006a, FR-013, FR-014, FR-015
 */

import { describe, it, expect } from 'vitest';
import {
  PRIVATE_KEY_LENGTH,
  COMPRESSED_PUBLIC_KEY_LENGTH,
  UNCOMPRESSED_PUBLIC_KEY_LENGTH,
  SIGNATURE_LENGTH,
  EVM_ADDRESS_LENGTH,
  SECP256K1_ORDER,
  MULTICODEC_SECP256K1_PUB,
  DID_KEY_PREFIX,
  BIP44_DEFAULT_PATH,
  ARGON2_V1_TIME,
  ARGON2_V1_MEMORY,
  ARGON2_V1_THREADS,
  SALT_LENGTH,
  NONCE_LENGTH,
  CIPHERTEXT_LENGTH,
} from '../../src/keylib/constants.js';

describe('Constants', () => {
  // Key lengths from 006 algorithm-reference.md §1
  it('PRIVATE_KEY_LENGTH = 32', () => {
    expect(PRIVATE_KEY_LENGTH).toBe(32);
  });

  it('COMPRESSED_PUBLIC_KEY_LENGTH = 33', () => {
    expect(COMPRESSED_PUBLIC_KEY_LENGTH).toBe(33);
  });

  it('UNCOMPRESSED_PUBLIC_KEY_LENGTH = 65', () => {
    expect(UNCOMPRESSED_PUBLIC_KEY_LENGTH).toBe(65);
  });

  it('SIGNATURE_LENGTH = 65 (R||S||V)', () => {
    expect(SIGNATURE_LENGTH).toBe(65);
  });

  it('EVM_ADDRESS_LENGTH = 20', () => {
    expect(EVM_ADDRESS_LENGTH).toBe(20);
  });

  // secp256k1 curve order from 006 algorithm-reference.md §1
  it('SECP256K1_ORDER matches spec hex value', () => {
    expect(SECP256K1_ORDER).toBe(
      0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141n,
    );
  });

  // Multicodec varint for secp256k1-pub (006 algorithm-reference.md §6)
  it('MULTICODEC_SECP256K1_PUB = [0xe7, 0x01]', () => {
    expect(MULTICODEC_SECP256K1_PUB).toEqual(new Uint8Array([0xe7, 0x01]));
  });

  // DID:key prefix (006 algorithm-reference.md §6)
  it('DID_KEY_PREFIX = "did:key:z"', () => {
    expect(DID_KEY_PREFIX).toBe('did:key:z');
  });

  // BIP-44 default path (006 algorithm-reference.md §13)
  it('BIP44_DEFAULT_PATH = "m/44\'/60\'/0\'/0/0"', () => {
    expect(BIP44_DEFAULT_PATH).toBe("m/44'/60'/0'/0/0");
  });

  // Argon2id v1 defaults (006 algorithm-reference.md §11)
  it('ARGON2_V1_TIME = 1', () => {
    expect(ARGON2_V1_TIME).toBe(1);
  });

  it('ARGON2_V1_MEMORY = 65536 (KiB)', () => {
    expect(ARGON2_V1_MEMORY).toBe(65536);
  });

  it('ARGON2_V1_THREADS = 4', () => {
    expect(ARGON2_V1_THREADS).toBe(4);
  });

  // Encryption field sizes (006 algorithm-reference.md §11)
  it('SALT_LENGTH = 16', () => {
    expect(SALT_LENGTH).toBe(16);
  });

  it('NONCE_LENGTH = 12', () => {
    expect(NONCE_LENGTH).toBe(12);
  });

  it('CIPHERTEXT_LENGTH = 48 (32 key + 16 GCM tag)', () => {
    expect(CIPHERTEXT_LENGTH).toBe(48);
  });
});
