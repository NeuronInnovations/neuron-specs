/**
 * Canonical JSON serialization.
 *
 * Spec reference: 006 wire-format.md
 * - FR-W01: Compact format — no whitespace between tokens, no trailing commas, no BOM, no trailing newline.
 * - FR-W04: Optional fields that are absent MUST be omitted. MUST NOT be serialized as null.
 * - FR-W05: JSON objects MUST emit keys in the canonical order defined per entity.
 * - FR-W08: String escaping per RFC 8259 §7.
 *
 * Spec reference: 006 wire-format.md Section 9 (C-2)
 * - All string-to-bytes conversions MUST use UTF-8.
 * - JS uses UTF-16 internally — MUST explicitly encode to UTF-8 before hashing.
 *
 * This module provides a builder-pattern serializer that emits JSON fields in explicit order,
 * rather than relying on JSON.stringify key ordering.
 */

import { uint64ToJsonString } from './uint64.js';
import { base64Encode } from './base64.js';
import { float64ToJsonFragment } from './float64.js';

/** UTF-8 text encoder. C-2: All string-to-bytes must use UTF-8. */
const textEncoder = new TextEncoder();

/**
 * Encode a canonical JSON string to UTF-8 bytes.
 * C-2 / FR-W01: Wire format mandates UTF-8 encoding.
 */
export function canonicalJsonToBytes(json: string): Uint8Array {
  return textEncoder.encode(json);
}

/**
 * Escape a string per RFC 8259 §7.
 * FR-W08: Mandatory escapes: " → \", \ → \\, control U+0000–U+001F → \uXXXX.
 * Non-ASCII characters (U+0020+) SHOULD be emitted as literal UTF-8 bytes.
 */
function escapeJsonString(str: string): string {
  let result = '';
  for (let i = 0; i < str.length; i++) {
    const ch = str[i]!;
    const code = str.charCodeAt(i);

    if (ch === '"') {
      result += '\\"';
    } else if (ch === '\\') {
      result += '\\\\';
    } else if (code < 0x20) {
      // Control characters → \uXXXX
      result += '\\u' + code.toString(16).padStart(4, '0');
    } else {
      // U+0020 and above: emit literally (UTF-8 bytes in the output string)
      result += ch;
    }
  }
  return result;
}

/**
 * A field definition for canonical JSON serialization.
 * Each field specifies its key name and how to serialize its value.
 */
export type CanonicalField =
  | { key: string; type: 'string'; value: string | undefined }
  | { key: string; type: 'uint64'; value: bigint | undefined }
  | { key: string; type: 'base64'; value: Uint8Array | undefined }
  | { key: string; type: 'float64'; value: number | undefined }
  | { key: string; type: 'boolean'; value: boolean | undefined }
  | { key: string; type: 'number'; value: number | undefined }
  | { key: string; type: 'json'; value: string | undefined }
  | { key: string; type: 'array_string'; value: string[] | undefined }
  | { key: string; type: 'object'; value: string | undefined };

/**
 * Serialize an ordered list of fields into a canonical JSON string.
 *
 * FR-W05: Keys are emitted in the exact order provided.
 * FR-W04: Fields with undefined values are omitted entirely (not null).
 * FR-W01: Compact format — no whitespace.
 *
 * @param fields - Ordered list of field definitions. Order determines JSON key order.
 * @param required - If true, undefined fields throw an error instead of being omitted.
 */
export function serializeCanonicalJson(fields: CanonicalField[]): string {
  let result = '{';
  let first = true;

  for (const field of fields) {
    // FR-W04: Absent optional fields are omitted
    if (field.value === undefined) {
      continue;
    }

    if (!first) {
      result += ',';
    }
    first = false;

    // Key (always quoted string)
    result += '"' + escapeJsonString(field.key) + '":';

    // Value based on type
    switch (field.type) {
      case 'string':
        // FR-W08: String values are JSON strings
        result += '"' + escapeJsonString(field.value) + '"';
        break;
      case 'uint64':
        // FR-W02: UnsignedInt64 as JSON strings
        result += '"' + uint64ToJsonString(field.value) + '"';
        break;
      case 'base64':
        // FR-W03: Binary as RFC 4648 base64 strings
        result += '"' + base64Encode(field.value) + '"';
        break;
      case 'float64':
        // FR-W10: Float64 as JSON numbers with .0 for integers
        result += float64ToJsonFragment(field.value);
        break;
      case 'boolean':
        // FR-W01: JSON true/false
        result += field.value ? 'true' : 'false';
        break;
      case 'number':
        // FR-W01: JSON number (for UnsignedInt8, UnsignedInt32)
        result += field.value.toString();
        break;
      case 'json':
        // Pre-serialized JSON fragment (for nested objects)
        result += field.value;
        break;
      case 'array_string':
        // JSON array of strings
        result += '[' + field.value.map(s => '"' + escapeJsonString(s) + '"').join(',') + ']';
        break;
      case 'object':
        // Pre-serialized nested JSON object
        result += field.value;
        break;
    }
  }

  result += '}';
  return result;
}
