/**
 * DIDKey — W3C DID:key identifier derived from a secp256k1 compressed public key.
 *
 * Spec reference: 006 algorithm-reference.md Section 6 (FR-A06)
 *
 * Algorithm:
 *   1. Prepend multicodec varint 0xE7 0x01 to compressed public key (33 bytes) = 35 bytes
 *   2. Base58btc encode the 35 bytes
 *   3. Prepend "did:key:z"
 *
 * Result format: did:key:zQ3s...
 *
 * Immutable value type. Valid by construction.
 */

import bs58 from 'bs58';
import {
  COMPRESSED_PUBLIC_KEY_LENGTH,
  MULTICODEC_SECP256K1_PUB,
  DID_KEY_PREFIX,
} from './constants.js';
import { invalidLength, invalidFormat } from './errors.js';

/** Expected length of the multicodec-prefixed key bytes. */
const MULTICODEC_BYTES_LENGTH = MULTICODEC_SECP256K1_PUB.length + COMPRESSED_PUBLIC_KEY_LENGTH; // 2 + 33 = 35

/**
 * A W3C DID:key identifier for a secp256k1 public key.
 *
 * FR-A06: Uses multicodec secp256k1-pub (0xE7, 0x01) prefix,
 * base58btc encoding, and the "did:key:z" prefix.
 */
export class DIDKey {
  /** Internal 35-byte multicodec-prefixed key. Never exposed directly. */
  private readonly _multicodecBytes: Uint8Array;

  /** Cached string representation. Computed once at construction. */
  private readonly _string: string;

  private constructor(multicodecBytes: Uint8Array, str: string) {
    this._multicodecBytes = multicodecBytes;
    this._string = str;
  }

  /**
   * Derive a DID:key from a 33-byte compressed secp256k1 public key.
   *
   * FR-A06 Algorithm:
   * 1. Prepend multicodec varint [0xE7, 0x01] to compressed key = 35 bytes
   * 2. Base58btc encode the 35 bytes
   * 3. Prepend "did:key:z"
   *
   * @param compressed - 33-byte compressed public key (0x02 or 0x03 prefix)
   * @returns DIDKey instance
   * @throws KeyError NEURON-KEY-003 if length is not 33
   * @throws KeyError NEURON-KEY-001 if prefix byte is not 0x02 or 0x03
   */
  static fromCompressedPublicKey(compressed: Uint8Array): DIDKey {
    if (compressed.length !== COMPRESSED_PUBLIC_KEY_LENGTH) {
      throw invalidLength(COMPRESSED_PUBLIC_KEY_LENGTH, compressed.length);
    }

    const prefix = compressed[0];
    if (prefix !== 0x02 && prefix !== 0x03) {
      throw invalidFormat(
        'Compressed public key must start with 0x02 or 0x03 prefix',
      );
    }

    // Step 1: Prepend multicodec varint to compressed key
    const multicodecBytes = new Uint8Array(MULTICODEC_BYTES_LENGTH);
    multicodecBytes.set(MULTICODEC_SECP256K1_PUB, 0);
    multicodecBytes.set(compressed, MULTICODEC_SECP256K1_PUB.length);

    // Step 2: Base58btc encode
    const base58Encoded = bs58.encode(multicodecBytes);

    // Step 3: Prepend "did:key:z"
    const didString = DID_KEY_PREFIX + base58Encoded;

    return new DIDKey(multicodecBytes, didString);
  }

  /**
   * Return the full DID:key string representation.
   *
   * Format: "did:key:zQ3s..." (always starts with "did:key:z" per FR-A06)
   *
   * @returns DID:key string
   */
  toString(): string {
    return this._string;
  }

  /**
   * Return a defensive copy of the 35-byte multicodec-prefixed key bytes.
   *
   * Useful for test vector verification.
   * Format: [0xE7, 0x01] + 33-byte compressed public key
   *
   * @returns New Uint8Array containing the multicodec bytes (35 bytes)
   */
  multicodecBytes(): Uint8Array {
    return new Uint8Array(this._multicodecBytes);
  }

  /**
   * Check equality with another DIDKey.
   *
   * @param other - Another DIDKey to compare
   * @returns `true` if DID:key identifiers are identical
   */
  equals(other: DIDKey): boolean {
    return this._string === other._string;
  }
}
