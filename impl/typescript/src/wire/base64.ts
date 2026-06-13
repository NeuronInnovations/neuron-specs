/**
 * Base64 encoding/decoding utilities.
 *
 * Spec reference: 006 wire-format.md Section 4 (FR-W03)
 * - All binary fields MUST be encoded using RFC 4648 Section 4 standard base64.
 * - Alphabet: ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/
 * - Padding: `=` MUST be used.
 * - Empty input → empty string "".
 */

import { NeuronError, WIRE_INVALID_BASE64 } from '../errors.js';

const BASE64_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';

/**
 * Encode bytes to RFC 4648 §4 standard base64 with `=` padding.
 * FR-W03: Standard alphabet with `=` padding.
 */
export function base64Encode(bytes: Uint8Array): string {
  if (bytes.length === 0) {
    return '';
  }

  let result = '';
  let i = 0;

  // Process 3-byte groups
  for (; i + 2 < bytes.length; i += 3) {
    const b0 = bytes[i]!;
    const b1 = bytes[i + 1]!;
    const b2 = bytes[i + 2]!;
    result += BASE64_CHARS[(b0 >> 2)!]!;
    result += BASE64_CHARS[((b0 & 0x03) << 4) | (b1 >> 4)]!;
    result += BASE64_CHARS[((b1 & 0x0f) << 2) | (b2 >> 6)]!;
    result += BASE64_CHARS[b2 & 0x3f]!;
  }

  // Handle remaining bytes with padding
  const remaining = bytes.length - i;
  if (remaining === 1) {
    const b0 = bytes[i]!;
    result += BASE64_CHARS[b0 >> 2]!;
    result += BASE64_CHARS[(b0 & 0x03) << 4]!;
    result += '==';
  } else if (remaining === 2) {
    const b0 = bytes[i]!;
    const b1 = bytes[i + 1]!;
    result += BASE64_CHARS[b0 >> 2]!;
    result += BASE64_CHARS[((b0 & 0x03) << 4) | (b1 >> 4)]!;
    result += BASE64_CHARS[(b1 & 0x0f) << 2]!;
    result += '=';
  }

  return result;
}

// Lookup table for base64 decoding
const BASE64_LOOKUP = new Uint8Array(128);
BASE64_LOOKUP.fill(0xff);
for (let i = 0; i < BASE64_CHARS.length; i++) {
  BASE64_LOOKUP[BASE64_CHARS.charCodeAt(i)] = i;
}

/**
 * Decode RFC 4648 §4 standard base64 string to bytes.
 * FR-W03: Decoders MUST reject non-base64 characters.
 * Decoders SHOULD accept inputs with or without padding for robustness.
 */
export function base64Decode(str: string): Uint8Array {
  if (str.length === 0) {
    return new Uint8Array(0);
  }

  // Strip padding for length calculation
  let strNoPad = str;
  while (strNoPad.endsWith('=')) {
    strNoPad = strNoPad.slice(0, -1);
  }

  // Validate all characters
  for (let i = 0; i < strNoPad.length; i++) {
    const code = strNoPad.charCodeAt(i);
    if (code >= 128 || BASE64_LOOKUP[code] === 0xff) {
      throw new NeuronError(
        WIRE_INVALID_BASE64,
        'InvalidBase64',
        `Invalid base64 character at position ${i.toString()}: '${strNoPad[i]!}'`,
      );
    }
  }

  // Calculate output length
  const outputLength = Math.floor((strNoPad.length * 3) / 4);
  const result = new Uint8Array(outputLength);

  let outIndex = 0;
  let i = 0;

  for (; i + 3 < strNoPad.length; i += 4) {
    const a = BASE64_LOOKUP[strNoPad.charCodeAt(i)]!;
    const b = BASE64_LOOKUP[strNoPad.charCodeAt(i + 1)]!;
    const c = BASE64_LOOKUP[strNoPad.charCodeAt(i + 2)]!;
    const d = BASE64_LOOKUP[strNoPad.charCodeAt(i + 3)]!;
    result[outIndex++] = (a << 2) | (b >> 4);
    result[outIndex++] = ((b & 0x0f) << 4) | (c >> 2);
    result[outIndex++] = ((c & 0x03) << 6) | d;
  }

  // Handle remaining characters
  const rem = strNoPad.length - i;
  if (rem >= 2) {
    const a = BASE64_LOOKUP[strNoPad.charCodeAt(i)]!;
    const b = BASE64_LOOKUP[strNoPad.charCodeAt(i + 1)]!;
    result[outIndex++] = (a << 2) | (b >> 4);
    if (rem >= 3) {
      const c = BASE64_LOOKUP[strNoPad.charCodeAt(i + 2)]!;
      result[outIndex++] = ((b & 0x0f) << 4) | (c >> 2);
    }
  }

  return result;
}
