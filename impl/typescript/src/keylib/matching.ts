/**
 * Constant-time byte comparison utilities.
 *
 * Spec reference: FR-016, SEC-004
 * Prevents timing side-channel attacks when comparing secret material
 * (private keys, signatures, HMAC tags).
 *
 * Uses Node.js `crypto.timingSafeEqual` which guarantees constant-time
 * comparison regardless of where the first difference occurs.
 */

import { timingSafeEqual } from 'node:crypto';

/**
 * Compare two byte arrays in constant time.
 *
 * FR-016: All equality checks on key material MUST use constant-time comparison.
 * SEC-004: Timing side-channels MUST be mitigated.
 *
 * @param a - First byte array
 * @param b - Second byte array
 * @returns `true` if both arrays have identical length and contents
 */
export function constantTimeEqual(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  return timingSafeEqual(a, b);
}
