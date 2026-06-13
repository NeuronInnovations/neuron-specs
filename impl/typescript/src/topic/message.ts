/**
 * TopicMessage -- signed, sequenced message published to a topic.
 *
 * Spec reference: 004 spec.md
 *   - FR-T02: TopicMessage fields (senderAddress, signature, timestamp, sequenceNumber, payload).
 *   - FR-T03: Signing chain: Keccak256(timestamp || sequenceNumber || payload) then ECDSA.
 *   - FR-T06: SequenceNumber MUST be monotonically increasing per sender per topic.
 *   - FR-T10: Signature verification and sender address recovery.
 *   - FR-T20: Payload is opaque bytes; the envelope does not interpret it.
 *   - FR-T21: Canonical JSON field order: senderAddress, signature, timestamp, sequenceNumber, payload.
 *
 * Spec reference: 006 algorithm-reference.md
 *   - FR-A07: RFC 6979 deterministic nonce generation.
 *   - FR-A08: Keccak256 pre-image for TopicMessage signing.
 *   - FR-A10: ECDSA R||S||V encoding (65 bytes).
 *
 * Spec reference: 006 wire-format.md
 *   - FR-W01: Compact JSON format.
 *   - FR-W02: UnsignedInt64 as JSON strings.
 *   - FR-W03: Binary fields as RFC 4648 base64.
 *   - FR-W05: Canonical field order.
 *   - FR-W06: EVM address in EIP-55 checksum.
 *
 * Immutable value type. Valid by construction.
 */

import { keccak_256 } from '@noble/hashes/sha3';
import type { NeuronPrivateKey } from '../keylib/private-key.js';
import { uint64ToBytesBE } from '../wire/uint64.js';
import { base64Encode } from '../wire/base64.js';
import { serializeCanonicalJson } from '../wire/canonical-json.js';
import type { CanonicalField } from '../wire/canonical-json.js';

/**
 * A signed, sequenced message published to a topic.
 *
 * FR-T02: Core message envelope with five required fields.
 * FR-T03: Signature covers Keccak256(timestamp || sequenceNumber || payload).
 * FR-T21: Canonical JSON uses struct declaration order.
 *
 * Construction is restricted to the static `create()` factory method, which
 * signs the message in one atomic step. This ensures every TopicMessage
 * instance has a valid signature at construction time.
 */
export class TopicMessage {
  /** EVM address of the sender (EIP-55 checksummed string). FR-T02 */
  private readonly _senderAddress: string;

  /** 65-byte ECDSA signature in R||S||V format. FR-T02, FR-T03 */
  private readonly _signature: Uint8Array;

  /** Unix timestamp in nanoseconds (sender-reported). FR-T02 */
  private readonly _timestamp: bigint;

  /** Monotonically increasing sequence number per sender per topic. FR-T02, FR-T06 */
  private readonly _sequenceNumber: bigint;

  /** Opaque application payload bytes. FR-T02, FR-T20 */
  private readonly _payload: Uint8Array;

  /** @internal -- use static factory methods instead. */
  private constructor(
    senderAddress: string,
    signature: Uint8Array,
    timestamp: bigint,
    sequenceNumber: bigint,
    payload: Uint8Array,
  ) {
    this._senderAddress = senderAddress;
    this._signature = signature;
    this._timestamp = timestamp;
    this._sequenceNumber = sequenceNumber;
    this._payload = payload;
  }

  // ---------------------------------------------------------------------------
  // Static factory methods
  // ---------------------------------------------------------------------------

  /**
   * Create a signed TopicMessage.
   *
   * FR-T03: Signing chain:
   *   1. preimage = uint64BE(timestamp) || uint64BE(sequenceNumber) || payload
   *   2. hash = Keccak256(preimage)
   *   3. signature = key.signHash(hash) -- ECDSA R||S||V (65 bytes)
   *   4. senderAddress = key.publicKey().evmAddress().toString() (EIP-55)
   *
   * FR-A07: RFC 6979 deterministic nonce generation (via keylib).
   * FR-A08: Keccak256 pre-image per algorithm reference Section 8.
   * FR-A10: Signature in R||S||V encoding (65 bytes).
   *
   * @param key - The sender's private key (from keylib 002)
   * @param timestamp - Unix timestamp in nanoseconds
   * @param sequenceNumber - Monotonically increasing sequence number
   * @param payload - Opaque application data
   * @returns A fully signed TopicMessage instance
   */
  static create(
    key: NeuronPrivateKey,
    timestamp: bigint,
    sequenceNumber: bigint,
    payload: Uint8Array,
  ): TopicMessage {
    // Step 1: Build pre-image
    const preimage = TopicMessage.buildPreimage(timestamp, sequenceNumber, payload);

    // Step 2: Hash pre-image with Keccak256
    const hash = keccak_256(preimage);

    // Step 3: Sign hash with ECDSA (RFC 6979, low-S normalization)
    const signature = key.signHash(hash);

    // Step 4: Derive sender address (EIP-55 checksummed)
    const senderAddress = key.publicKey().evmAddress().toString();

    return new TopicMessage(
      senderAddress,
      signature.toBytes(),
      timestamp,
      sequenceNumber,
      new Uint8Array(payload),
    );
  }

  /**
   * Reconstruct a TopicMessage from its constituent fields without re-signing.
   *
   * This is used for deserialization (e.g., from canonical JSON or from
   * a MessageDelivery received via Subscribe). The caller is responsible
   * for validating the signature via `validate()`.
   *
   * @param senderAddress - EIP-55 checksummed EVM address string
   * @param signature - 65-byte R||S||V signature
   * @param timestamp - Unix timestamp in nanoseconds
   * @param sequenceNumber - Sequence number
   * @param payload - Opaque payload bytes
   * @returns TopicMessage instance (signature not verified)
   */
  static fromFields(
    senderAddress: string,
    signature: Uint8Array,
    timestamp: bigint,
    sequenceNumber: bigint,
    payload: Uint8Array,
  ): TopicMessage {
    return new TopicMessage(
      senderAddress,
      new Uint8Array(signature),
      timestamp,
      sequenceNumber,
      new Uint8Array(payload),
    );
  }

  // ---------------------------------------------------------------------------
  // Static signing helpers
  // ---------------------------------------------------------------------------

  /**
   * Build the signing pre-image for a TopicMessage.
   *
   * FR-A08 / 006 algorithm-reference.md Section 8:
   *   preimage = uint64BE(timestamp) || uint64BE(sequenceNumber) || payload
   *
   * The pre-image is the byte concatenation of:
   *   - timestamp encoded as 8 bytes big-endian
   *   - sequenceNumber encoded as 8 bytes big-endian
   *   - raw payload bytes
   *
   * @param timestamp - Unix timestamp in nanoseconds
   * @param sequenceNumber - Sequence number
   * @param payload - Raw payload bytes
   * @returns Pre-image byte array (16 + payload.length bytes)
   */
  static buildPreimage(
    timestamp: bigint,
    sequenceNumber: bigint,
    payload: Uint8Array,
  ): Uint8Array {
    const tsBytes = uint64ToBytesBE(timestamp);
    const seqBytes = uint64ToBytesBE(sequenceNumber);

    // Concatenate: timestamp (8) || sequenceNumber (8) || payload (N)
    const result = new Uint8Array(8 + 8 + payload.length);
    result.set(tsBytes, 0);
    result.set(seqBytes, 8);
    result.set(payload, 16);
    return result;
  }

  /**
   * Hash the signing pre-image with Keccak256.
   *
   * FR-A08: Keccak256 hash of the pre-image is the message hash for signing.
   *
   * @param timestamp - Unix timestamp in nanoseconds
   * @param sequenceNumber - Sequence number
   * @param payload - Raw payload bytes
   * @returns 32-byte Keccak256 hash
   */
  static hashPreimage(
    timestamp: bigint,
    sequenceNumber: bigint,
    payload: Uint8Array,
  ): Uint8Array {
    const preimage = TopicMessage.buildPreimage(timestamp, sequenceNumber, payload);
    return keccak_256(preimage);
  }

  // ---------------------------------------------------------------------------
  // Instance accessors (read-only)
  // ---------------------------------------------------------------------------

  /** EVM address of the sender (EIP-55 checksummed string). FR-T02, FR-W06 */
  get senderAddress(): string {
    return this._senderAddress;
  }

  /** Unix timestamp in nanoseconds (sender-reported). FR-T02 */
  get timestamp(): bigint {
    return this._timestamp;
  }

  /** Monotonically increasing sequence number. FR-T02, FR-T06 */
  get sequenceNumber(): bigint {
    return this._sequenceNumber;
  }

  /** Defensive copy of the opaque application payload. FR-T02, FR-T20 */
  get payload(): Uint8Array {
    return new Uint8Array(this._payload);
  }

  // ---------------------------------------------------------------------------
  // Signature accessors
  // ---------------------------------------------------------------------------

  /**
   * Return a defensive copy of the 65-byte R||S||V signature.
   *
   * FR-A10: ECDSA signature in R (32) || S (32) || V (1) encoding.
   *
   * @returns New Uint8Array containing 65 signature bytes
   */
  signatureBytes(): Uint8Array {
    return new Uint8Array(this._signature);
  }

  /**
   * Return the signature encoded as RFC 4648 base64.
   *
   * FR-W03: Binary fields are encoded as standard base64 with = padding.
   *
   * @returns Base64-encoded signature string
   */
  signatureBase64(): string {
    return base64Encode(this._signature);
  }

  // ---------------------------------------------------------------------------
  // Payload accessors
  // ---------------------------------------------------------------------------

  /**
   * Return the payload encoded as RFC 4648 base64.
   *
   * FR-W03: Binary fields are encoded as standard base64 with = padding.
   *
   * @returns Base64-encoded payload string
   */
  payloadBase64(): string {
    return base64Encode(this._payload);
  }

  // ---------------------------------------------------------------------------
  // Serialization
  // ---------------------------------------------------------------------------

  /**
   * Serialize to canonical JSON.
   *
   * FR-T21 / FR-W05: Canonical field order is the struct declaration order:
   *   senderAddress, signature, timestamp, sequenceNumber, payload
   *
   * FR-W01: Compact format -- no whitespace between tokens.
   * FR-W02: UnsignedInt64 as JSON strings.
   * FR-W03: Binary fields as RFC 4648 base64 strings.
   * FR-W06: EVM address in EIP-55 checksum encoding.
   * FR-W08: String escaping per RFC 8259 Section 7.
   *
   * SC-T08: Deterministic serialization -- same inputs always produce same output.
   *
   * @returns Canonical JSON string
   */
  toCanonicalJson(): string {
    const fields: CanonicalField[] = [
      { key: 'senderAddress', type: 'string', value: this._senderAddress },
      { key: 'signature', type: 'string', value: base64Encode(this._signature) },
      { key: 'timestamp', type: 'uint64', value: this._timestamp },
      { key: 'sequenceNumber', type: 'uint64', value: this._sequenceNumber },
      { key: 'payload', type: 'string', value: base64Encode(this._payload) },
    ];
    return serializeCanonicalJson(fields);
  }
}
