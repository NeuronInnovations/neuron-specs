/**
 * Hex encoding/decoding utilities.
 *
 * Used throughout the SDK for key material, addresses, and test vector verification.
 * Spec reference: 006 error-taxonomy.md — NEURON-KEY-004 (InvalidHex)
 */

import { NeuronError, KEY_INVALID_HEX } from '../errors.js';

/**
 * Encode bytes to lowercase hex string with 0x prefix.
 */
export function bytesToHex(bytes: Uint8Array): string {
  let hex = '0x';
  for (let i = 0; i < bytes.length; i++) {
    hex += bytes[i]!.toString(16).padStart(2, '0');
  }
  return hex;
}

/**
 * Decode hex string (with or without 0x prefix) to bytes.
 * Validates hex characters per NEURON-KEY-004.
 */
export function hexToBytes(hex: string): Uint8Array {
  let str = hex;
  if (str.startsWith('0x') || str.startsWith('0X')) {
    str = str.slice(2);
  }

  if (str.length % 2 !== 0) {
    throw new NeuronError(
      KEY_INVALID_HEX,
      'InvalidHex',
      `Hex string must have even length, got ${str.length.toString()}`,
    );
  }

  if (!/^[0-9a-fA-F]*$/.test(str)) {
    throw new NeuronError(
      KEY_INVALID_HEX,
      'InvalidHex',
      'Hex string contains non-hex characters. Valid: 0-9, a-f, A-F.',
    );
  }

  const bytes = new Uint8Array(str.length / 2);
  for (let i = 0; i < str.length; i += 2) {
    bytes[i / 2] = parseInt(str.substring(i, i + 2), 16);
  }
  return bytes;
}
