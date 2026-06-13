/**
 * EVMAddress — Ethereum Virtual Machine address derived from a secp256k1 public key.
 *
 * Spec reference: 006 algorithm-reference.md
 *   - Section 3 (FR-A03): EVM Address Derivation
 *   - Section 4 (FR-A04): EIP-55 Mixed-Case Checksum Encoding
 *
 * Immutable value type. Valid by construction — invalid inputs are rejected
 * at factory method boundaries.
 */

import { keccak_256 } from '@noble/hashes/sha3';
import { EVM_ADDRESS_LENGTH, UNCOMPRESSED_PUBLIC_KEY_LENGTH } from './constants.js';
import { invalidFormat, invalidLength } from './errors.js';
import { bytesToHex, hexToBytes } from '../wire/hex.js';
import { constantTimeEqual } from './matching.js';

/**
 * A 20-byte Ethereum address with EIP-55 checksum display.
 *
 * FR-A03: Derived from keccak256 of the uncompressed public key (sans 0x04 prefix).
 * FR-A04: String representation uses EIP-55 mixed-case checksum encoding.
 */
export class EVMAddress {
  /** Internal 20-byte address. Never exposed directly. */
  private readonly _bytes: Uint8Array;

  private constructor(bytes: Uint8Array) {
    this._bytes = bytes;
  }

  /**
   * Derive an EVM address from an uncompressed secp256k1 public key.
   *
   * FR-A03 Algorithm:
   * 1. Input: 65-byte uncompressed public key (0x04 || X || Y)
   * 2. Strip the 0x04 prefix -> 64 bytes (X || Y)
   * 3. hash = keccak256(64 bytes) -> 32 bytes
   * 4. address = last 20 bytes (hash[12..31])
   *
   * @param uncompressedPubKey - 65-byte uncompressed public key with 0x04 prefix
   * @returns EVMAddress instance
   * @throws KeyError NEURON-KEY-003 if length is not 65
   * @throws KeyError NEURON-KEY-001 if missing 0x04 prefix
   */
  static fromPublicKeyBytes(uncompressedPubKey: Uint8Array): EVMAddress {
    if (uncompressedPubKey.length !== UNCOMPRESSED_PUBLIC_KEY_LENGTH) {
      throw invalidLength(UNCOMPRESSED_PUBLIC_KEY_LENGTH, uncompressedPubKey.length);
    }

    if (uncompressedPubKey[0] !== 0x04) {
      throw invalidFormat(
        'Uncompressed public key must start with 0x04 prefix',
      );
    }

    // Step 2: Strip 0x04 prefix -> 64 bytes
    const keyBody = uncompressedPubKey.slice(1);

    // Step 3: Keccak-256 hash of the 64-byte key body
    const hash = keccak_256(keyBody);

    // Step 4: Take last 20 bytes
    const addressBytes = hash.slice(12);

    return new EVMAddress(addressBytes);
  }

  /**
   * Parse an EVM address from a hex string (with or without 0x prefix).
   *
   * Accepts both lowercase and EIP-55 checksummed hex strings.
   * If the input contains mixed case, EIP-55 checksum is validated.
   *
   * @param hex - Hex string representing a 20-byte address
   * @returns EVMAddress instance
   * @throws KeyError NEURON-KEY-004 if hex is invalid
   * @throws KeyError NEURON-KEY-003 if decoded length is not 20
   * @throws KeyError NEURON-KEY-001 if EIP-55 checksum is invalid
   */
  static fromHex(hex: string): EVMAddress {
    const bytes = hexToBytes(hex);
    if (bytes.length !== EVM_ADDRESS_LENGTH) {
      throw invalidLength(EVM_ADDRESS_LENGTH, bytes.length);
    }

    // If the input has mixed case (not all-lowercase, not all-uppercase),
    // validate EIP-55 checksum
    const hexBody = hex.startsWith('0x') || hex.startsWith('0X')
      ? hex.slice(2)
      : hex;

    const hasLower = /[a-f]/.test(hexBody);
    const hasUpper = /[A-F]/.test(hexBody);

    if (hasLower && hasUpper) {
      // Mixed case: validate EIP-55 checksum
      const address = new EVMAddress(bytes);
      const expected = address.toString().slice(2); // strip 0x
      if (hexBody !== expected) {
        throw invalidFormat(
          'EIP-55 checksum mismatch: address has invalid mixed-case encoding',
        );
      }
    }

    return new EVMAddress(bytes);
  }

  /**
   * Construct an EVMAddress from raw 20-byte address bytes.
   *
   * @param bytes - Exactly 20 bytes
   * @returns EVMAddress instance
   * @throws KeyError NEURON-KEY-003 if length is not 20
   */
  static fromBytes(bytes: Uint8Array): EVMAddress {
    if (bytes.length !== EVM_ADDRESS_LENGTH) {
      throw invalidLength(EVM_ADDRESS_LENGTH, bytes.length);
    }
    // Defensive copy
    return new EVMAddress(new Uint8Array(bytes));
  }

  /**
   * Return the EIP-55 checksummed hex string representation.
   *
   * FR-A04 Algorithm:
   * 1. Convert 20-byte address to lowercase hex (40 chars, no 0x prefix)
   * 2. hash = keccak256(ASCII bytes of the lowercase hex string)
   *    CRITICAL: Hash the ASCII character bytes, NOT the raw address bytes
   * 3. For each character position i (0..39):
   *    - Compute hash nibble: even i -> high nibble (byte >> 4); odd i -> low nibble (byte & 0x0F)
   *    - If nibble >= 8 AND character is a-f: uppercase it
   * 4. Prepend "0x"
   *
   * @returns EIP-55 checksummed address string, e.g. "0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"
   */
  toString(): string {
    return eip55Checksum(this._bytes);
  }

  /**
   * Return the lowercase hex representation with 0x prefix.
   *
   * @returns Lowercase address string, e.g. "0xab5801a7d398351b8be11c439e05c5b3259aec9b"
   */
  toLowercaseHex(): string {
    return bytesToHex(this._bytes);
  }

  /**
   * Return a defensive copy of the raw 20-byte address.
   *
   * @returns New Uint8Array containing the address bytes
   */
  toBytes(): Uint8Array {
    return new Uint8Array(this._bytes);
  }

  /**
   * Check equality with another EVMAddress using constant-time comparison.
   *
   * SEC-004: Timing side-channel mitigation.
   *
   * @param other - Another EVMAddress to compare
   * @returns `true` if addresses are identical
   */
  equals(other: EVMAddress): boolean {
    return constantTimeEqual(this._bytes, other._bytes);
  }
}

/**
 * Compute the EIP-55 mixed-case checksum encoding for a 20-byte address.
 *
 * FR-A04: The keccak256 input MUST be the ASCII bytes of the lowercase
 * hex string (40 characters), NOT the raw address bytes.
 *
 * @param addressBytes - 20-byte raw address
 * @returns EIP-55 checksummed hex string with 0x prefix
 */
function eip55Checksum(addressBytes: Uint8Array): string {
  // Step 1: Convert to lowercase hex (no 0x prefix, 40 chars)
  const lowercaseHex = bytesToHex(addressBytes).slice(2); // remove "0x"

  // Step 2: Hash the ASCII bytes of the lowercase hex string
  // CRITICAL: Use TextEncoder to get ASCII bytes of the hex string characters
  const encoder = new TextEncoder();
  const asciiBytes = encoder.encode(lowercaseHex);
  const hash = keccak_256(asciiBytes);

  // Step 3: Apply checksum — uppercase hex chars where hash nibble >= 8
  const chars: string[] = new Array<string>(40);
  for (let i = 0; i < 40; i++) {
    const char = lowercaseHex[i]!;
    // Get hash nibble at position i
    const byteIndex = Math.floor(i / 2);
    const byte = hash[byteIndex]!;
    const nibble = i % 2 === 0
      ? (byte >> 4) & 0x0f  // high nibble for even positions
      : byte & 0x0f;        // low nibble for odd positions

    // Uppercase if nibble >= 8 AND char is a hex letter (a-f)
    if (nibble >= 8 && char >= 'a' && char <= 'f') {
      chars[i] = char.toUpperCase();
    } else {
      chars[i] = char;
    }
  }

  // Step 4: Prepend "0x"
  return '0x' + chars.join('');
}
