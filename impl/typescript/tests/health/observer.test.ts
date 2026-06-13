/**
 * Tests for observer-side heartbeat validation.
 *
 * Spec reference: 005 contracts/health-observer.md
 * Covers V-OBS-02 through V-OBS-06.
 * V-OBS-01 (signature) is handled at the TopicMessage level, not tested here.
 */

import { describe, it, expect } from 'vitest';
import { validateInboundHeartbeat } from '../../src/health/observer.js';

/** Helper: nanoseconds per second */
const NANOS = 1_000_000_000n;

/** Helper: a base consensus timestamp in nanoseconds */
const CONSENSUS_TS = 1700000000n * NANOS;

/** Helper: build a valid heartbeat JSON string */
function validHeartbeatJson(overrides?: {
  type?: string;
  version?: string;
  deadline?: bigint;
  role?: string;
}): string {
  return JSON.stringify({
    type: overrides?.type ?? 'heartbeat',
    version: overrides?.version ?? '1.0.0',
    nextHeartbeatDeadline: (overrides?.deadline ?? (CONSENSUS_TS + 60n * NANOS)).toString(),
    role: overrides?.role ?? 'seller',
  });
}

describe('validateInboundHeartbeat', () => {
  // V-OBS-02: type === "heartbeat"
  it('V-OBS-02: rejects non-heartbeat type', () => {
    const json = validHeartbeatJson({ type: 'status' });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('not a heartbeat message');
  });

  it('V-OBS-02: accepts heartbeat type', () => {
    const json = validHeartbeatJson();
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(true);
  });

  // V-OBS-03: version major === 1
  it('V-OBS-03: rejects version 2.0.0', () => {
    const json = validHeartbeatJson({ version: '2.0.0' });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('incompatible heartbeat version');
  });

  it('V-OBS-03: accepts version 1.1.0', () => {
    const json = validHeartbeatJson({ version: '1.1.0' });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(true);
  });

  it('V-OBS-03: accepts version 1.99.0', () => {
    const json = validHeartbeatJson({ version: '1.99.0' });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(true);
  });

  it('V-OBS-03: rejects version 0.1.0', () => {
    const json = validHeartbeatJson({ version: '0.1.0' });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('incompatible heartbeat version');
  });

  // V-OBS-04: deadline === 0n → valid (caller handles OFFLINE)
  it('V-OBS-04: accepts shutdown sentinel (deadline = 0)', () => {
    const json = validHeartbeatJson({ deadline: 0n });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(true);
  });

  // V-OBS-05: deadline > consensusTimestamp
  it('V-OBS-05: rejects deadline in the past relative to consensus', () => {
    const json = validHeartbeatJson({ deadline: CONSENSUS_TS - 1n * NANOS });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('deadline is in the past relative to consensus time');
  });

  it('V-OBS-05: rejects deadline equal to consensusTimestamp', () => {
    const json = validHeartbeatJson({ deadline: CONSENSUS_TS });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('deadline is in the past relative to consensus time');
  });

  // V-OBS-06: MIN_DEADLINE_DELTA <= delta <= MAX_DEADLINE_DELTA
  it('V-OBS-06: rejects delta below minimum (< 10 seconds)', () => {
    const json = validHeartbeatJson({ deadline: CONSENSUS_TS + 5n * NANOS });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('deadline delta below minimum');
  });

  it('V-OBS-06: accepts delta at exact minimum (10 seconds)', () => {
    const json = validHeartbeatJson({ deadline: CONSENSUS_TS + 10n * NANOS });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(true);
  });

  it('V-OBS-06: rejects delta above maximum (> 86400 seconds)', () => {
    const json = validHeartbeatJson({ deadline: CONSENSUS_TS + 86401n * NANOS });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('deadline delta exceeds maximum');
  });

  it('V-OBS-06: accepts delta at exact maximum (86400 seconds)', () => {
    const json = validHeartbeatJson({ deadline: CONSENSUS_TS + 86400n * NANOS });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(true);
  });

  // Edge: invalid JSON
  it('rejects invalid JSON', () => {
    const result = validateInboundHeartbeat('not json', CONSENSUS_TS);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('invalid JSON');
  });

  // Edge: missing deadline field
  it('rejects missing nextHeartbeatDeadline field', () => {
    const json = JSON.stringify({ type: 'heartbeat', version: '1.0.0', role: 'seller' });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(false);
    expect(result.error).toContain('nextHeartbeatDeadline');
  });

  // Valid payload with capabilities
  it('accepts valid payload with capabilities', () => {
    const json = JSON.stringify({
      type: 'heartbeat',
      version: '1.0.0',
      nextHeartbeatDeadline: (CONSENSUS_TS + 60n * NANOS).toString(),
      role: 'seller',
      capabilities: { natReachability: true, protocols: ['/adsb/v1'] },
    });
    const result = validateInboundHeartbeat(json, CONSENSUS_TS);
    expect(result.valid).toBe(true);
  });
});
