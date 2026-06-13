/**
 * Wire Format Conformance Tests.
 *
 * Source: specs/006-protocol-determinism/contracts/wire-format.md
 * Tests all 9 FR-W rules that govern JSON serialization.
 */

import { describe, it, expect } from 'vitest';
import {
  uint64ToJsonString,
  uint64FromJsonString,
  uint64ToBytesBE,
  base64Encode,
  base64Decode,
  float64ToJsonFragment,
  serializeCanonicalJson,
} from '../../src/wire/index.js';
import type { CanonicalField } from '../../src/wire/index.js';

describe('Wire Format Rules', () => {
  // FR-W01: Compact JSON — no whitespace between tokens
  describe('FR-W01: Compact JSON', () => {
    it('should produce JSON with no whitespace between tokens', () => {
      const fields: CanonicalField[] = [
        { key: 'a', type: 'string', value: 'hello' },
        { key: 'b', type: 'number', value: 42 },
      ];
      const json = serializeCanonicalJson(fields);
      expect(json).toBe('{"a":"hello","b":42}');
      // No spaces after : or ,
      expect(json).not.toMatch(/: /);
      expect(json).not.toMatch(/, /);
    });
  });

  // FR-W02: UnsignedInt64 as JSON strings
  describe('FR-W02: UnsignedInt64 Encoding', () => {
    it('should encode 0 as "0"', () => {
      expect(uint64ToJsonString(0n)).toBe('0');
    });

    it('should encode 42 as "42"', () => {
      expect(uint64ToJsonString(42n)).toBe('42');
    });

    it('should encode timestamp-scale value as string', () => {
      expect(uint64ToJsonString(1700000000000000000n)).toBe('1700000000000000000');
    });

    it('should encode max uint64', () => {
      expect(uint64ToJsonString(18446744073709551615n)).toBe('18446744073709551615');
    });

    it('should reject negative values', () => {
      expect(() => uint64ToJsonString(-1n)).toThrow();
    });

    it('should parse "0" to 0n', () => {
      expect(uint64FromJsonString('0')).toBe(0n);
    });

    it('should reject leading zeros ("042")', () => {
      expect(() => uint64FromJsonString('042')).toThrow();
    });

    it('should reject empty string', () => {
      expect(() => uint64FromJsonString('')).toThrow();
    });

    // Verify uint64 in canonical JSON is a string, not a number
    it('should serialize uint64 as JSON string in canonical JSON', () => {
      const fields: CanonicalField[] = [
        { key: 'timestamp', type: 'uint64', value: 1700000000000000000n },
      ];
      const json = serializeCanonicalJson(fields);
      expect(json).toBe('{"timestamp":"1700000000000000000"}');
    });
  });

  // FR-W02 byte encoding: 8 bytes big-endian
  describe('FR-W02: UnsignedInt64 Byte Encoding', () => {
    it('should encode timestamp to 8 bytes BE', () => {
      const bytes = uint64ToBytesBE(1700000000000000000n);
      const hex = Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('');
      expect(hex).toBe('17979cfe362a0000');
    });

    it('should encode 1 to 8 bytes BE', () => {
      const bytes = uint64ToBytesBE(1n);
      const hex = Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('');
      expect(hex).toBe('0000000000000001');
    });
  });

  // FR-W03: Binary as RFC 4648 §4 standard base64
  describe('FR-W03: Base64 Encoding', () => {
    it('should encode "Hello" to "SGVsbG8="', () => {
      const bytes = new TextEncoder().encode('Hello');
      expect(base64Encode(bytes)).toBe('SGVsbG8=');
    });

    it('should encode empty to ""', () => {
      expect(base64Encode(new Uint8Array(0))).toBe('');
    });

    it('should use = padding', () => {
      // 1 byte → 4 chars with ==
      const one = new Uint8Array([0xff]);
      const encoded = base64Encode(one);
      expect(encoded.endsWith('==')).toBe(true);
    });

    it('should round-trip decode', () => {
      const original = new Uint8Array([1, 2, 3, 4, 5]);
      const encoded = base64Encode(original);
      const decoded = base64Decode(encoded);
      expect(decoded).toEqual(original);
    });

    it('should reject non-base64 characters', () => {
      expect(() => base64Decode('SGVs$G8=')).toThrow();
    });
  });

  // FR-W04: Optional fields omitted (not null)
  describe('FR-W04: Optional Field Handling', () => {
    it('should omit undefined fields entirely', () => {
      const fields: CanonicalField[] = [
        { key: 'required', type: 'string', value: 'yes' },
        { key: 'optional', type: 'string', value: undefined },
      ];
      const json = serializeCanonicalJson(fields);
      expect(json).toBe('{"required":"yes"}');
      expect(json).not.toContain('null');
      expect(json).not.toContain('optional');
    });
  });

  // FR-W05: Canonical field order
  describe('FR-W05: Canonical Field Order', () => {
    it('should emit fields in the exact order provided', () => {
      const fields: CanonicalField[] = [
        { key: 'z', type: 'string', value: 'last' },
        { key: 'a', type: 'string', value: 'first' },
      ];
      const json = serializeCanonicalJson(fields);
      // z before a — order follows the array, not alphabetical
      expect(json).toBe('{"z":"last","a":"first"}');
    });
  });

  // FR-W07: BigInteger (arbitrary-precision) as JSON strings
  describe('FR-W07: BigInteger Encoding', () => {
    it('should serialize BigInteger as JSON string via uint64', () => {
      // BigInteger uses same encoding as uint64 for positive values
      const fields: CanonicalField[] = [
        { key: 'balance', type: 'uint64', value: 1000000000000000000n },
      ];
      const json = serializeCanonicalJson(fields);
      expect(json).toBe('{"balance":"1000000000000000000"}');
    });
  });

  // FR-W08: String escaping per RFC 8259 §7
  describe('FR-W08: String Escaping', () => {
    it('should escape double quotes', () => {
      const fields: CanonicalField[] = [
        { key: 'msg', type: 'string', value: 'say "hello"' },
      ];
      const json = serializeCanonicalJson(fields);
      expect(json).toBe('{"msg":"say \\"hello\\""}');
    });

    it('should escape backslashes', () => {
      const fields: CanonicalField[] = [
        { key: 'path', type: 'string', value: 'a\\b' },
      ];
      const json = serializeCanonicalJson(fields);
      expect(json).toBe('{"path":"a\\\\b"}');
    });

    it('should escape control characters as \\uXXXX', () => {
      const fields: CanonicalField[] = [
        { key: 'ctrl', type: 'string', value: '\t\n' },
      ];
      const json = serializeCanonicalJson(fields);
      expect(json).toBe('{"ctrl":"\\u0009\\u000a"}');
    });
  });

  // FR-W10: Float64 with .0 for integer values
  describe('FR-W10: Float64 Encoding', () => {
    it('should serialize 37.7749 as "37.7749"', () => {
      expect(float64ToJsonFragment(37.7749)).toBe('37.7749');
    });

    it('should serialize -122.4194 as "-122.4194"', () => {
      expect(float64ToJsonFragment(-122.4194)).toBe('-122.4194');
    });

    it('should serialize 10.0 as "10.0" (not "10")', () => {
      expect(float64ToJsonFragment(10.0)).toBe('10.0');
    });

    it('should serialize 0.0 as "0.0" (not "0")', () => {
      expect(float64ToJsonFragment(0.0)).toBe('0.0');
    });

    it('should serialize 37.0 as "37.0"', () => {
      expect(float64ToJsonFragment(37.0)).toBe('37.0');
    });

    it('should reject NaN', () => {
      expect(() => float64ToJsonFragment(NaN)).toThrow();
    });

    it('should reject Infinity', () => {
      expect(() => float64ToJsonFragment(Infinity)).toThrow();
    });

    it('should reject -Infinity', () => {
      expect(() => float64ToJsonFragment(-Infinity)).toThrow();
    });

    it('should serialize float64 in canonical JSON as number', () => {
      const fields: CanonicalField[] = [
        { key: 'lat', type: 'float64', value: 37.0 },
      ];
      const json = serializeCanonicalJson(fields);
      expect(json).toBe('{"lat":37.0}');
    });
  });
});
