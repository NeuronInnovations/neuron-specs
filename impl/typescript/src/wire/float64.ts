/**
 * Float64 serialization utilities.
 *
 * Spec reference: 006 wire-format.md Section 8 (FR-W10)
 * - Float64 values MUST be serialized as JSON numbers with up to 15 significant digits.
 * - No trailing zeros after decimal point.
 * - Integer-valued floats MUST include `.0` (e.g., 37.0 not 37).
 * - No exponential/scientific notation for values representable in fixed-point.
 * - NaN and Infinity are invalid — MUST reject.
 */

import { NeuronError, WIRE_INVALID_FIELD_ORDER } from '../errors.js';

/**
 * Serialize a Float64 value to a JSON-compatible string per FR-W10.
 *
 * Examples:
 *   37.7749  → "37.7749"
 *   -122.4194 → "-122.4194"
 *   10.0     → "10.0"     (not "10")
 *   0.0      → "0.0"      (not "0")
 */
export function float64ToJsonFragment(value: number): string {
  // FR-W10: NaN and Infinity are invalid
  if (!Number.isFinite(value)) {
    throw new NeuronError(
      WIRE_INVALID_FIELD_ORDER, // Closest applicable wire error
      'InvalidFloat64',
      `Float64 must be finite. Got: ${String(value)}`,
    );
  }

  // FR-W10: Integer-valued floats MUST include .0
  if (Number.isInteger(value)) {
    // Handles both positive and negative integers, and 0
    const str = value.toString();
    return str + '.0';
  }

  // For non-integer values, toString() produces the standard representation.
  // FR-W10: No exponential notation for representable values.
  const str = value.toString();

  // Check if JS produced exponential notation (e.g., 1e-7)
  if (str.includes('e') || str.includes('E')) {
    // Convert from exponential to fixed-point notation
    // Use toPrecision with 15 significant digits, then strip trailing zeros
    let fixed = value.toPrecision(15);
    // Strip trailing zeros after decimal point, but keep at least one digit after '.'
    if (fixed.includes('.')) {
      fixed = fixed.replace(/0+$/, '');
      if (fixed.endsWith('.')) {
        fixed += '0';
      }
    }
    return fixed;
  }

  return str;
}
