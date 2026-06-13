/**
 * Tests for health module constants.
 *
 * Spec reference: 005 data-model.md, ProtocolConstants table
 * Verifies each constant matches the spec-defined value.
 */

import { describe, it, expect } from 'vitest';
import {
  MIN_DEADLINE_DELTA,
  MAX_DEADLINE_DELTA,
  GRACE_PERIOD,
  SUSPECT_TO_DEAD,
  SHUTDOWN_SENTINEL,
  MAX_MANDATORY_PAYLOAD_SIZE,
} from '../../src/health/constants.js';

describe('Health Constants', () => {
  // FR-H06: MIN_DEADLINE_DELTA = 10 seconds
  it('MIN_DEADLINE_DELTA is 10n (FR-H06)', () => {
    expect(MIN_DEADLINE_DELTA).toBe(10n);
    expect(typeof MIN_DEADLINE_DELTA).toBe('bigint');
  });

  // FR-H07: MAX_DEADLINE_DELTA = 86400 seconds (24 hours)
  it('MAX_DEADLINE_DELTA is 86400n (FR-H07)', () => {
    expect(MAX_DEADLINE_DELTA).toBe(86400n);
    expect(MAX_DEADLINE_DELTA).toBe(BigInt(24 * 60 * 60));
  });

  // FR-H08: GRACE_PERIOD = 30 seconds
  it('GRACE_PERIOD is 30n (FR-H08)', () => {
    expect(GRACE_PERIOD).toBe(30n);
  });

  // FR-H09: SUSPECT_TO_DEAD = 120 seconds
  it('SUSPECT_TO_DEAD is 120n (FR-H09)', () => {
    expect(SUSPECT_TO_DEAD).toBe(120n);
  });

  // FR-H12: SHUTDOWN_SENTINEL = 0
  it('SHUTDOWN_SENTINEL is 0n (FR-H12)', () => {
    expect(SHUTDOWN_SENTINEL).toBe(0n);
  });

  // FR-H29: MAX_MANDATORY_PAYLOAD_SIZE = 256 bytes
  it('MAX_MANDATORY_PAYLOAD_SIZE is 256 (FR-H29)', () => {
    expect(MAX_MANDATORY_PAYLOAD_SIZE).toBe(256);
    expect(typeof MAX_MANDATORY_PAYLOAD_SIZE).toBe('number');
  });

  // Invariant: MIN < MAX
  it('MIN_DEADLINE_DELTA < MAX_DEADLINE_DELTA', () => {
    expect(MIN_DEADLINE_DELTA < MAX_DEADLINE_DELTA).toBe(true);
  });

  // Invariant: GRACE + S2D defines the full tolerance window
  it('GRACE_PERIOD + SUSPECT_TO_DEAD = 150n (total tolerance window)', () => {
    expect(GRACE_PERIOD + SUSPECT_TO_DEAD).toBe(150n);
  });
});
