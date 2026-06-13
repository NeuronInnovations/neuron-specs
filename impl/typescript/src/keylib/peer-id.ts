/**
 * PeerID — libp2p peer identifier derived from a secp256k1 compressed public key.
 *
 * Spec reference: 006 algorithm-reference.md Section 5 (FR-A05)
 *
 * Algorithm:
 *   Step 5.1: Construct Protobuf PublicKey message
 *     - Wire bytes: 0x08 0x02 0x12 0x21 + 33 compressed key bytes = 37 bytes total
 *       (0x08 = field 1 varint, 0x02 = Secp256k1 KeyType)
 *       (0x12 = field 2 length-delimited, 0x21 = length 33)
 *
 *   Step 5.2: Wrap in identity multihash (for keys <= 42 bytes)
 *     - 0x00 (identity hash code) + 0x25 (length 37) + 37 protobuf bytes = 39 bytes total
 *
 *   Step 5.3: Base58btc encode the 39 multihash bytes
 *     - Result starts with "12D3KooW" for all secp256k1 keys
 *
 * Immutable value type. Valid by construction.
 */

import bs58 from 'bs58';
import {
  COMPRESSED_PUBLIC_KEY_LENGTH,
  PEER_ID_PROTOBUF_HEADER,
  IDENTITY_MULTIHASH_HEADER,
} from './constants.js';
import { invalidLength, invalidFormat } from './errors.js';

/** Expected length of the Protobuf-encoded public key message. */
const PROTOBUF_MESSAGE_LENGTH = PEER_ID_PROTOBUF_HEADER.length + COMPRESSED_PUBLIC_KEY_LENGTH; // 4 + 33 = 37

/** Expected length of the identity multihash wrapping the protobuf message. */
const MULTIHASH_LENGTH = IDENTITY_MULTIHASH_HEADER.length + PROTOBUF_MESSAGE_LENGTH; // 2 + 37 = 39

/**
 * A libp2p PeerID derived from a secp256k1 compressed public key.
 *
 * FR-A05: PeerID uses the identity multihash of a Protobuf-encoded
 * secp256k1 public key, base58btc-encoded.
 */
export class PeerID {
  /** Internal 39-byte identity multihash. Never exposed directly. */
  private readonly _multihashBytes: Uint8Array;

  private constructor(multihashBytes: Uint8Array) {
    this._multihashBytes = multihashBytes;
  }

  /**
   * Derive a PeerID from a 33-byte compressed secp256k1 public key.
   *
   * FR-A05 Algorithm:
   * 1. Build Protobuf PublicKey message: header (4 bytes) + compressed key (33 bytes) = 37 bytes
   * 2. Wrap in identity multihash: header (2 bytes) + protobuf (37 bytes) = 39 bytes
   * 3. Base58btc encode for display (done in toString)
   *
   * @param compressed - 33-byte compressed public key (0x02 or 0x03 prefix)
   * @returns PeerID instance
   * @throws KeyError NEURON-KEY-003 if length is not 33
   * @throws KeyError NEURON-KEY-001 if prefix byte is not 0x02 or 0x03
   */
  static fromCompressedPublicKey(compressed: Uint8Array): PeerID {
    if (compressed.length !== COMPRESSED_PUBLIC_KEY_LENGTH) {
      throw invalidLength(COMPRESSED_PUBLIC_KEY_LENGTH, compressed.length);
    }

    const prefix = compressed[0];
    if (prefix !== 0x02 && prefix !== 0x03) {
      throw invalidFormat(
        'Compressed public key must start with 0x02 or 0x03 prefix',
      );
    }

    // Step 5.1: Build Protobuf PublicKey message (37 bytes)
    const protobuf = new Uint8Array(PROTOBUF_MESSAGE_LENGTH);
    protobuf.set(PEER_ID_PROTOBUF_HEADER, 0);
    protobuf.set(compressed, PEER_ID_PROTOBUF_HEADER.length);

    // Step 5.2: Wrap in identity multihash (39 bytes)
    const multihash = new Uint8Array(MULTIHASH_LENGTH);
    multihash.set(IDENTITY_MULTIHASH_HEADER, 0);
    multihash.set(protobuf, IDENTITY_MULTIHASH_HEADER.length);

    return new PeerID(multihash);
  }

  /**
   * Return the base58btc-encoded PeerID string.
   *
   * FR-A05 Step 5.3: The result always starts with "12D3KooW" for secp256k1 keys.
   *
   * @returns Base58btc-encoded PeerID string
   */
  toString(): string {
    return bs58.encode(this._multihashBytes);
  }

  /**
   * Return a defensive copy of the 37-byte Protobuf PublicKey message.
   *
   * Useful for test vector verification against the protobuf encoding.
   *
   * @returns New Uint8Array containing the protobuf bytes (37 bytes)
   */
  protobufBytes(): Uint8Array {
    // Protobuf bytes are the last 37 bytes of the multihash (after the 2-byte header)
    return new Uint8Array(
      this._multihashBytes.slice(IDENTITY_MULTIHASH_HEADER.length),
    );
  }

  /**
   * Return a defensive copy of the 39-byte identity multihash.
   *
   * Useful for test vector verification against the full multihash encoding.
   *
   * @returns New Uint8Array containing the multihash bytes (39 bytes)
   */
  multihashBytes(): Uint8Array {
    return new Uint8Array(this._multihashBytes);
  }

  /**
   * Check equality with another PeerID by comparing multihash bytes.
   *
   * @param other - Another PeerID to compare
   * @returns `true` if PeerIDs are identical
   */
  equals(other: PeerID): boolean {
    if (this._multihashBytes.length !== other._multihashBytes.length) {
      return false;
    }
    for (let i = 0; i < this._multihashBytes.length; i++) {
      if (this._multihashBytes[i] !== other._multihashBytes[i]) {
        return false;
      }
    }
    return true;
  }
}
