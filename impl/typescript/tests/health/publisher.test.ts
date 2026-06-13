/**
 * Tests for publisher-side heartbeat validation.
 *
 * Spec reference: 005 contracts/health-publisher.md
 * Covers V-PUB-01 through V-PUB-07.
 */

import { describe, it, expect } from 'vitest';
import { HeartbeatPayload } from '../../src/health/payload.js';
import { validateOutboundHeartbeat } from '../../src/health/publisher.js';
import { NeuronError } from '../../src/errors.js';
import {
  HEALTH_INVALID_PAYLOAD_TYPE,
  HEALTH_INVALID_VERSION,
  HEALTH_INVALID_DEADLINE,
  HEALTH_INVALID_ROLE,
} from '../../src/errors.js';

/** Helper: nanoseconds per second */
const NANOS = 1_000_000_000n;

/** Helper: a base sender clock in nanoseconds */
const NOW = 1700000000n * NANOS; // 1700000000 seconds

/** Helper: build a valid payload for testing */
function validPayload(overrides?: {
  deadline?: bigint;
  role?: 'buyer' | 'seller' | 'relay' | 'validator';
}): HeartbeatPayload {
  return HeartbeatPayload.build({
    nextHeartbeatDeadline: overrides?.deadline ?? (NOW + 60n * NANOS),
    role: overrides?.role ?? 'seller',
  });
}

describe('validateOutboundHeartbeat', () => {
  // V-PUB-01: payload.type === "heartbeat"
  it('V-PUB-01: accepts type "heartbeat"', () => {
    const p = validPayload();
    expect(() => validateOutboundHeartbeat(p, NOW)).not.toThrow();
  });

  // V-PUB-01: Note: HeartbeatPayload.build always sets type to "heartbeat",
  // so this check can never fail in normal use, but the validator still checks.
  // The type field is readonly 'heartbeat' as const, so we test that the check passes.

  // V-PUB-02: version recognized (major === 1)
  it('V-PUB-02: accepts version "1.0.0"', () => {
    const p = validPayload();
    expect(() => validateOutboundHeartbeat(p, NOW)).not.toThrow();
  });

  // V-PUB-03: shutdown sentinel bypass
  it('V-PUB-03: accepts deadline 0n (shutdown sentinel) without other checks', () => {
    const p = HeartbeatPayload.build({
      nextHeartbeatDeadline: 0n,
      role: 'seller',
    });
    // Even though deadline is 0n (not in the future), shutdown bypasses all deadline checks
    expect(() => validateOutboundHeartbeat(p, NOW)).not.toThrow();
  });

  // V-PUB-04: deadline > senderClock
  it('V-PUB-04: rejects deadline in the past', () => {
    const p = HeartbeatPayload.build({
      nextHeartbeatDeadline: NOW - 1n * NANOS,
      role: 'seller',
    });
    try {
      validateOutboundHeartbeat(p, NOW);
      expect.unreachable('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(NeuronError);
      expect((e as NeuronError).code).toBe(HEALTH_INVALID_DEADLINE);
      expect((e as NeuronError).message).toContain('future');
    }
  });

  it('V-PUB-04: rejects deadline equal to senderClock', () => {
    const p = HeartbeatPayload.build({
      nextHeartbeatDeadline: NOW,
      role: 'seller',
    });
    try {
      validateOutboundHeartbeat(p, NOW);
      expect.unreachable('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(NeuronError);
      expect((e as NeuronError).code).toBe(HEALTH_INVALID_DEADLINE);
    }
  });

  // V-PUB-05: delta >= MIN_DEADLINE_DELTA (10 seconds)
  it('V-PUB-05: rejects deadline too soon (delta < 10 seconds)', () => {
    const p = HeartbeatPayload.build({
      nextHeartbeatDeadline: NOW + 5n * NANOS, // 5 seconds < 10 min
      role: 'seller',
    });
    try {
      validateOutboundHeartbeat(p, NOW);
      expect.unreachable('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(NeuronError);
      expect((e as NeuronError).code).toBe(HEALTH_INVALID_DEADLINE);
      expect((e as NeuronError).message).toContain('too soon');
    }
  });

  it('V-PUB-05: accepts deadline at exact minimum (delta = 10 seconds)', () => {
    const p = HeartbeatPayload.build({
      nextHeartbeatDeadline: NOW + 10n * NANOS,
      role: 'seller',
    });
    expect(() => validateOutboundHeartbeat(p, NOW)).not.toThrow();
  });

  // V-PUB-06: delta <= MAX_DEADLINE_DELTA (86400 seconds)
  it('V-PUB-06: rejects deadline too far (delta > 86400 seconds)', () => {
    const p = HeartbeatPayload.build({
      nextHeartbeatDeadline: NOW + 86401n * NANOS,
      role: 'seller',
    });
    try {
      validateOutboundHeartbeat(p, NOW);
      expect.unreachable('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(NeuronError);
      expect((e as NeuronError).code).toBe(HEALTH_INVALID_DEADLINE);
      expect((e as NeuronError).message).toContain('too far');
    }
  });

  it('V-PUB-06: accepts deadline at exact maximum (delta = 86400 seconds)', () => {
    const p = HeartbeatPayload.build({
      nextHeartbeatDeadline: NOW + 86400n * NANOS,
      role: 'seller',
    });
    expect(() => validateOutboundHeartbeat(p, NOW)).not.toThrow();
  });

  // V-PUB-07: role in VALID_ROLES
  it('V-PUB-07: accepts all valid roles', () => {
    const roles = ['buyer', 'seller', 'relay', 'validator'] as const;
    for (const role of roles) {
      const p = validPayload({ role });
      expect(() => validateOutboundHeartbeat(p, NOW)).not.toThrow();
    }
  });

  // Combined: valid payload passes all checks
  it('valid payload passes all checks', () => {
    const p = HeartbeatPayload.build({
      nextHeartbeatDeadline: NOW + 60n * NANOS,
      role: 'seller',
      capabilities: { natReachability: true, protocols: ['/adsb/v1'] },
    });
    expect(() => validateOutboundHeartbeat(p, NOW)).not.toThrow();
  });
});
