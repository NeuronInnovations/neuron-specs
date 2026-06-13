/**
 * UnsignedInt64 encoding utilities.
 *
 * Spec reference: 006 wire-format.md Section 3 (FR-W02)
 * - All UnsignedInt64 fields MUST be encoded as JSON strings containing the decimal representation.
 * - Decimal digits only (0-9), no leading zeros except for "0" itself, no sign prefix.
 *
 * Spec reference: 006 algorithm-reference.md §8 (FR-A08)
 * - UnsignedInt64 values in byte representations use 8 bytes big-endian.
 */

import { NeuronError, WIRE_INVALID_UINT64_ENCODING } from '../errors.js';

/** Maximum value for UnsignedInt64: 2^64 - 1 */
const UINT64_MAX = 18446744073709551615n;

/**
 * Encode a bigint as a JSON-safe decimal string.
 * FR-W02: UnsignedInt64 MUST be JSON strings containing decimal digits.
 */
export function uint64ToJsonString(value: bigint): string {
  if (value < 0n) {
    throw new NeuronError(
      WIRE_INVALID_UINT64_ENCODING,
      'InvalidUint64Encoding',
      `UnsignedInt64 cannot be negative: ${value.toString()}`,
    );
  }
  if (value > UINT64_MAX) {
    throw new NeuronError(
      WIRE_INVALID_UINT64_ENCODING,
      'InvalidUint64Encoding',
      `UnsignedInt64 exceeds maximum (2^64 - 1): ${value.toString()}`,
    );
  }
  return value.toString();
}

/**
 * Parse a JSON string representation into a bigint.
 * FR-W02: Decimal digits only, no leading zeros except "0".
 */
export function uint64FromJsonString(str: string): bigint {
  // Validate format: decimal digits only
  if (!/^(0|[1-9][0-9]*)$/.test(str)) {
    throw new NeuronError(
      WIRE_INVALID_UINT64_ENCODING,
      'InvalidUint64Encoding',
      `Invalid UnsignedInt64 string: "${str}". Must be decimal digits with no leading zeros.`,
    );
  }
  const value = BigInt(str);
  if (value > UINT64_MAX) {
    throw new NeuronError(
      WIRE_INVALID_UINT64_ENCODING,
      'InvalidUint64Encoding',
      `UnsignedInt64 exceeds maximum (2^64 - 1): ${str}`,
    );
  }
  return value;
}

/**
 * Encode a bigint as 8 bytes big-endian.
 * FR-A08: timestamp and sequenceNumber are encoded as 8 bytes big-endian for signing pre-image.
 */
export function uint64ToBytesBE(value: bigint): Uint8Array {
  if (value < 0n || value > UINT64_MAX) {
    throw new NeuronError(
      WIRE_INVALID_UINT64_ENCODING,
      'InvalidUint64Encoding',
      `UnsignedInt64 out of range: ${value.toString()}`,
    );
  }
  const bytes = new Uint8Array(8);
  let remaining = value;
  for (let i = 7; i >= 0; i--) {
    bytes[i] = Number(remaining & 0xffn);
    remaining >>= 8n;
  }
  return bytes;
}

/**
 * Decode 8 bytes big-endian to a bigint.
 */
export function uint64FromBytesBE(bytes: Uint8Array): bigint {
  if (bytes.length !== 8) {
    throw new NeuronError(
      WIRE_INVALID_UINT64_ENCODING,
      'InvalidUint64Encoding',
      `Expected 8 bytes for UnsignedInt64, got ${bytes.length.toString()}`,
    );
  }
  let value = 0n;
  for (let i = 0; i < 8; i++) {
    value = (value << 8n) | BigInt(bytes[i]!);
  }
  return value;
}
