/**
 * NeuronPublicKey — immutable, type-safe secp256k1 public key.
 *
 * Spec reference: 002 spec.md
 *   - FR-001: NeuronPublicKey provides both compressed (33 bytes) and
 *     uncompressed (65 bytes) representations.
 *   - FR-002: Elevation from raw bytes/hex with validation at construction.
 *   - FR-004: Ed25519 keys are detected and rejected (NEURON-KEY-002).
 *   - FR-004a: API surface is constrained to secp256k1-valid operations only.
 *   - FR-010: Factory methods accept hex strings and raw bytes.
 *   - FR-018: If a NeuronPublicKey instance exists, it is guaranteed valid.
 *   - FR-022: Immutable after construction. No setters or mutators.
 *
 * Algorithm reference: 006 spec.md
 *   - FR-A02 (Point Compression): 33-byte compressed, 65-byte uncompressed.
 *   - FR-A14 (Ed25519 Rejection): Reject non-secp256k1 prefix bytes.
 *
 * Immutable value type. Valid by construction — invalid inputs are rejected
 * at factory method boundaries.
 */

import * as secp from '@noble/secp256k1';
import {
  COMPRESSED_PUBLIC_KEY_LENGTH,
  UNCOMPRESSED_PUBLIC_KEY_LENGTH,
} from './constants.js';
import {
  unsupportedKeyType,
  invalidLength,
  invalidKey,
} from './errors.js';
import { hexToBytes } from '../wire/hex.js';
import { constantTimeEqual } from './matching.js';
import { EVMAddress } from './evm-address.js';
import { PeerID } from './peer-id.js';
import { DIDKey } from './did-key.js';

/**
 * A secp256k1 public key stored internally in compressed form (33 bytes).
 *
 * FR-001: Provides both compressed and uncompressed representations.
 * FR-018: Guaranteed valid — construction rejects invalid curve points.
 * FR-022: Immutable after construction.
 */
export class NeuronPublicKey {
  /** Internal 33-byte compressed public key (0x02/0x03 prefix + 32-byte X). */
  private readonly _compressed: Uint8Array;

  /** @internal — use factory methods instead. */
  private constructor(compressed: Uint8Array) {
    this._compressed = compressed;
  }

  /**
   * Parse a public key from a hex string (with or without 0x prefix).
   *
   * FR-002, FR-010: Accepts hex strings with or without 0x prefix.
   * Delegates to {@link fromBytes} after hex decoding.
   *
   * @param hex - Hex-encoded public key (33 or 65 bytes)
   * @returns NeuronPublicKey instance
   * @throws KeyError NEURON-KEY-004 if hex is invalid
   * @throws KeyError NEURON-KEY-002 if prefix byte indicates non-secp256k1 key
   * @throws KeyError NEURON-KEY-003 if decoded length is not 33 or 65
   * @throws KeyError NEURON-KEY-005 if point is not on the secp256k1 curve
   */
  static fromHex(hex: string): NeuronPublicKey {
    const bytes = hexToBytes(hex);
    return NeuronPublicKey.fromBytes(bytes);
  }

  /**
   * Construct a NeuronPublicKey from raw bytes (compressed or uncompressed).
   *
   * FR-001: Accepts both compressed (33 bytes) and uncompressed (65 bytes).
   * FR-004, FR-A14: Rejects Ed25519 and non-secp256k1 prefix bytes.
   * FR-A02: Uncompressed keys are compressed for internal storage.
   * FR-018: Validates point is on the secp256k1 curve.
   *
   * @param bytes - 33-byte compressed or 65-byte uncompressed public key
   * @returns NeuronPublicKey instance
   * @throws KeyError NEURON-KEY-002 if prefix byte indicates non-secp256k1 key
   * @throws KeyError NEURON-KEY-003 if length is not 33 or 65
   * @throws KeyError NEURON-KEY-005 if point is not on the secp256k1 curve
   */
  static fromBytes(bytes: Uint8Array): NeuronPublicKey {
    if (bytes.length === COMPRESSED_PUBLIC_KEY_LENGTH) {
      // FR-A14: Validate prefix is 0x02 or 0x03 (secp256k1 compressed)
      const prefix = bytes[0];
      if (prefix !== 0x02 && prefix !== 0x03) {
        throw unsupportedKeyType(
          `Compressed public key must have prefix 0x02 or 0x03, got 0x${(prefix ?? 0).toString(16).padStart(2, '0')}. ` +
          'Non-secp256k1 key types (e.g., Ed25519) are not supported',
        );
      }

      // Validate point is on the secp256k1 curve
      const compressed = validateOnCurve(bytes);
      return new NeuronPublicKey(compressed);
    }

    if (bytes.length === UNCOMPRESSED_PUBLIC_KEY_LENGTH) {
      // FR-A14: Validate prefix is 0x04 (uncompressed)
      const prefix = bytes[0];
      if (prefix !== 0x04) {
        throw unsupportedKeyType(
          `Uncompressed public key must have prefix 0x04, got 0x${(prefix ?? 0).toString(16).padStart(2, '0')}. ` +
          'Non-secp256k1 key types (e.g., Ed25519) are not supported',
        );
      }

      // FR-A02: Validate on curve and compress for internal storage
      const compressed = validateOnCurve(bytes);
      return new NeuronPublicKey(compressed);
    }

    // FR-008: Invalid length — report expected compressed length
    throw invalidLength(
      COMPRESSED_PUBLIC_KEY_LENGTH,
      bytes.length,
    );
  }

  /**
   * Construct a NeuronPublicKey from known-compressed bytes (33 bytes).
   *
   * Shortcut for callers that already hold a validated compressed key.
   * Still validates the point is on the secp256k1 curve.
   *
   * @param bytes - 33-byte compressed public key (0x02 or 0x03 prefix)
   * @returns NeuronPublicKey instance
   * @throws KeyError NEURON-KEY-002 if prefix byte is not 0x02 or 0x03
   * @throws KeyError NEURON-KEY-003 if length is not 33
   * @throws KeyError NEURON-KEY-005 if point is not on the secp256k1 curve
   */
  static fromCompressedBytes(bytes: Uint8Array): NeuronPublicKey {
    if (bytes.length !== COMPRESSED_PUBLIC_KEY_LENGTH) {
      throw invalidLength(COMPRESSED_PUBLIC_KEY_LENGTH, bytes.length);
    }

    const prefix = bytes[0];
    if (prefix !== 0x02 && prefix !== 0x03) {
      throw unsupportedKeyType(
        `Compressed public key must have prefix 0x02 or 0x03, got 0x${(prefix ?? 0).toString(16).padStart(2, '0')}. ` +
        'Non-secp256k1 key types (e.g., Ed25519) are not supported',
      );
    }

    const compressed = validateOnCurve(bytes);
    return new NeuronPublicKey(compressed);
  }

  /**
   * Return a defensive copy of the 33-byte compressed public key.
   *
   * FR-001: Compressed format (0x02/0x03 prefix + 32-byte X coordinate).
   * FR-022: Returns a copy — callers cannot mutate internal state.
   *
   * @returns New Uint8Array containing 33 compressed bytes
   */
  toCompressedBytes(): Uint8Array {
    return new Uint8Array(this._compressed);
  }

  /**
   * Return the 65-byte uncompressed public key.
   *
   * FR-001, FR-A02: Decompresses to 0x04 || X (32 bytes) || Y (32 bytes).
   *
   * @returns New Uint8Array containing 65 uncompressed bytes
   */
  toUncompressedBytes(): Uint8Array {
    const point = secp.ProjectivePoint.fromHex(this._compressed);
    return point.toRawBytes(false);
  }

  /**
   * Derive the EVM address from this public key.
   *
   * FR-005, FR-A03: Keccak256 of the uncompressed key body, last 20 bytes.
   *
   * @returns EVMAddress instance
   */
  evmAddress(): EVMAddress {
    return EVMAddress.fromPublicKeyBytes(this.toUncompressedBytes());
  }

  /**
   * Derive the libp2p PeerID from this public key.
   *
   * FR-006, FR-A05: Protobuf-wrapped identity multihash of compressed key.
   *
   * @returns PeerID instance
   */
  peerId(): PeerID {
    return PeerID.fromCompressedPublicKey(new Uint8Array(this._compressed));
  }

  /**
   * Derive the W3C DID:key identifier from this public key.
   *
   * FR-006a, FR-A06: Multicodec secp256k1-pub prefix + base58btc encoding.
   *
   * @returns DIDKey instance
   */
  didKey(): DIDKey {
    return DIDKey.fromCompressedPublicKey(new Uint8Array(this._compressed));
  }

  /**
   * Check whether this public key corresponds to the given EVM address.
   *
   * FR-016, SEC-004: Derives the EVM address and uses constant-time
   * comparison to prevent timing side-channel attacks.
   *
   * @param address - EVMAddress to compare against
   * @returns `true` if this key derives to the given address
   */
  matchesEvmAddress(address: EVMAddress): boolean {
    const derived = this.evmAddress();
    return constantTimeEqual(derived.toBytes(), address.toBytes());
  }

  /**
   * Check whether this public key corresponds to the given PeerID.
   *
   * FR-016, SEC-004: Derives the PeerID and uses constant-time
   * comparison on the multihash bytes to prevent timing side-channel attacks.
   *
   * @param peerId - PeerID to compare against
   * @returns `true` if this key derives to the given PeerID
   */
  matchesPeerId(peerId: PeerID): boolean {
    const derived = this.peerId();
    return constantTimeEqual(derived.multihashBytes(), peerId.multihashBytes());
  }
}

/**
 * Validate that the given bytes represent a point on the secp256k1 curve
 * and return the compressed form.
 *
 * @param bytes - 33-byte compressed or 65-byte uncompressed public key
 * @returns 33-byte compressed public key
 * @throws KeyError NEURON-KEY-005 if the point is not on the curve
 */
function validateOnCurve(bytes: Uint8Array): Uint8Array {
  try {
    const point = secp.ProjectivePoint.fromHex(bytes);
    // Return compressed form (33 bytes)
    return point.toRawBytes(true);
  } catch (e) {
    const cause = e instanceof Error ? e : new Error(String(e));
    throw invalidKey(
      `Public key bytes are not a valid secp256k1 curve point: ${cause.message}`,
    );
  }
}
