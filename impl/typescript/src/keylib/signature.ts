/**
 * Signature — 65-byte ECDSA signature in R||S||V format.
 *
 * Spec reference: 006 algorithm-reference.md Section 10 (FR-A10)
 *
 * Encoding:
 *   - 65 bytes total: R (32 bytes big-endian) || S (32 bytes big-endian) || V (1 byte)
 *   - V = recovery identifier: 0x00 or 0x01 (NOT Ethereum's 27/28)
 *   - Low-S normalization is mandatory (enforced by @noble/secp256k1 with {lowS: true})
 *
 * Spec reference: 006 wire-format.md Section 4 (FR-W03)
 *   - Base64 encoding uses RFC 4648 Section 4 standard alphabet with = padding
 *
 * Immutable value type. Valid by construction.
 */

import * as secp from '@noble/secp256k1';
import { timingSafeEqual } from 'node:crypto';
import { SIGNATURE_LENGTH, COMPRESSED_PUBLIC_KEY_LENGTH } from './constants.js';
import { invalidLength, invalidFormat, verificationFailed } from './errors.js';
import { base64Encode, base64Decode } from '../wire/base64.js';

/** Valid recovery identifiers per FR-A10. */
const VALID_RECOVERY_IDS = new Set([0, 1]);

/**
 * A 65-byte ECDSA signature in R||S||V wire format.
 *
 * FR-A10: All Neuron signatures use deterministic ECDSA (RFC 6979)
 * with low-S normalization and recovery byte V in {0, 1}.
 */
export class Signature {
  /** Internal 65-byte signature. Never exposed directly. */
  private readonly _bytes: Uint8Array;

  private constructor(bytes: Uint8Array) {
    this._bytes = bytes;
  }

  /**
   * Construct a Signature from a 65-byte R||S||V array.
   *
   * FR-A10: Validates length is exactly 65 and V is 0 or 1.
   *
   * @param bytes - Exactly 65 bytes: R (32) || S (32) || V (1)
   * @returns Signature instance
   * @throws KeyError NEURON-KEY-003 if length is not 65
   * @throws KeyError NEURON-KEY-001 if V byte is not 0 or 1
   */
  static fromBytes(bytes: Uint8Array): Signature {
    if (bytes.length !== SIGNATURE_LENGTH) {
      throw invalidLength(SIGNATURE_LENGTH, bytes.length);
    }

    const v = bytes[64]!;
    if (!VALID_RECOVERY_IDS.has(v)) {
      throw invalidFormat(
        `Recovery identifier V must be 0 or 1, got ${v.toString()}`,
      );
    }

    // Defensive copy
    return new Signature(new Uint8Array(bytes));
  }

  /**
   * Construct a Signature from a base64-encoded string.
   *
   * FR-W03: Uses RFC 4648 Section 4 standard base64 with = padding.
   *
   * @param str - Base64-encoded 65-byte signature
   * @returns Signature instance
   * @throws NeuronError NEURON-WIRE-003 if base64 is invalid
   * @throws KeyError NEURON-KEY-003 if decoded length is not 65
   * @throws KeyError NEURON-KEY-001 if V byte is not 0 or 1
   */
  static fromBase64(str: string): Signature {
    const bytes = base64Decode(str);
    return Signature.fromBytes(bytes);
  }

  /**
   * Construct a Signature from individual R, S, V components.
   *
   * FR-A10: R and S are 32 bytes big-endian, V is 0 or 1.
   *
   * @param r - 32-byte R component (big-endian)
   * @param s - 32-byte S component (big-endian)
   * @param v - Recovery identifier (0 or 1)
   * @returns Signature instance
   * @throws KeyError NEURON-KEY-003 if R or S is not 32 bytes
   * @throws KeyError NEURON-KEY-001 if V is not 0 or 1
   */
  static fromRSV(r: Uint8Array, s: Uint8Array, v: number): Signature {
    if (r.length !== 32) {
      throw invalidLength(32, r.length);
    }
    if (s.length !== 32) {
      throw invalidLength(32, s.length);
    }
    if (!VALID_RECOVERY_IDS.has(v)) {
      throw invalidFormat(
        `Recovery identifier V must be 0 or 1, got ${v.toString()}`,
      );
    }

    const bytes = new Uint8Array(SIGNATURE_LENGTH);
    bytes.set(r, 0);
    bytes.set(s, 32);
    bytes[64] = v;

    return new Signature(bytes);
  }

  /**
   * Return a defensive copy of the 65-byte R||S||V signature.
   *
   * @returns New Uint8Array containing the signature bytes
   */
  toBytes(): Uint8Array {
    return new Uint8Array(this._bytes);
  }

  /**
   * Return the 32-byte R component (big-endian).
   *
   * @returns New Uint8Array containing bytes[0..31]
   */
  r(): Uint8Array {
    return new Uint8Array(this._bytes.slice(0, 32));
  }

  /**
   * Return the 32-byte S component (big-endian).
   *
   * @returns New Uint8Array containing bytes[32..63]
   */
  s(): Uint8Array {
    return new Uint8Array(this._bytes.slice(32, 64));
  }

  /**
   * Return the recovery identifier V (0 or 1).
   *
   * FR-A10: V is 0x00 or 0x01, NOT Ethereum's 27/28.
   *
   * @returns Recovery identifier: 0 or 1
   */
  v(): number {
    return this._bytes[64]!;
  }

  /**
   * Encode the signature as RFC 4648 Section 4 standard base64.
   *
   * FR-W03: Binary fields use standard base64 with = padding.
   *
   * @returns Base64-encoded signature string
   */
  toBase64(): string {
    return base64Encode(this._bytes);
  }

  /**
   * Recover the compressed public key from this signature and a message hash.
   *
   * FR-A10: Uses the V recovery identifier to recover the signer's public key.
   * The caller provides a 32-byte Keccak-256 message hash.
   *
   * @param messageHash - 32-byte hash of the signed message
   * @returns 33-byte compressed public key of the signer
   * @throws KeyError NEURON-KEY-003 if messageHash is not 32 bytes
   * @throws KeyError NEURON-KEY-014 if recovery fails
   */
  recover(messageHash: Uint8Array): Uint8Array {
    if (messageHash.length !== 32) {
      throw invalidLength(32, messageHash.length);
    }

    try {
      // Reconstruct the @noble/secp256k1 Signature with recovery bit
      const compact = this._bytes.slice(0, 64);
      const recoveryBit = this._bytes[64]!;
      const sig = secp.Signature.fromCompact(compact).addRecoveryBit(recoveryBit);

      // Recover the public key point
      const recoveredPoint = sig.recoverPublicKey(messageHash);

      // Return compressed form (33 bytes)
      return recoveredPoint.toRawBytes(true);
    } catch (e) {
      const cause = e instanceof Error ? e : new Error(String(e));
      throw verificationFailed('Public key recovery failed', cause);
    }
  }

  /**
   * Verify this signature against a message hash and expected public key.
   *
   * FR-A10, SEC-004: Recovers the signer's public key and compares it
   * to the expected key using constant-time comparison to prevent
   * timing side-channel attacks.
   *
   * @param messageHash - 32-byte hash of the signed message
   * @param publicKey - 33-byte compressed public key of the expected signer
   * @returns `true` if the recovered key matches the expected key
   */
  verify(messageHash: Uint8Array, publicKey: Uint8Array): boolean {
    if (publicKey.length !== COMPRESSED_PUBLIC_KEY_LENGTH) {
      return false;
    }

    try {
      const recovered = this.recover(messageHash);

      // Constant-time comparison (SEC-004)
      if (recovered.length !== publicKey.length) {
        return false;
      }
      return timingSafeEqual(recovered, publicKey);
    } catch {
      return false;
    }
  }
}
