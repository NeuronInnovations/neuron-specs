/**
 * Tests for liveness state machine.
 *
 * Spec reference: 005 spec.md, data-model.md, contracts/health-observer.md
 *   - FR-H18: LivenessRecord per-peer tracking
 *   - FR-H19: 5 states
 *   - FR-H20: State evaluation algorithm
 *   - FR-H21: Recovery invariant (any state + valid HB -> ALIVE)
 *   - FR-H17: Sequence ordering
 *
 * All 5 states and 8 transitions are tested.
 */

import { describe, it, expect } from 'vitest';
import { evaluateLiveness, updateLivenessRecord } from '../../src/health/liveness.js';
import { LivenessState } from '../../src/health/types.js';
import type { LivenessRecord } from '../../src/health/types.js';

/** Helper: nanoseconds per second */
const NANOS = 1_000_000_000n;

/** Helper: a base time in nanoseconds */
const BASE_TIME = 1700000000n * NANOS;

/** Helper: create a record for testing */
function makeRecord(overrides?: Partial<LivenessRecord>): LivenessRecord {
  return {
    senderAddress: '0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf',
    currentState: LivenessState.ALIVE,
    lastDeadline: BASE_TIME + 60n * NANOS,
    lastSequence: 1n,
    lastConsensusTimestamp: BASE_TIME,
    ...overrides,
  };
}

describe('evaluateLiveness', () => {
  // -------------------------------------------------------------------------
  // 5 states
  // -------------------------------------------------------------------------

  // State: UNKNOWN -- no record
  it('returns UNKNOWN when record is null', () => {
    expect(evaluateLiveness(null, BASE_TIME)).toBe(LivenessState.UNKNOWN);
  });

  // State: OFFLINE -- shutdown sentinel
  it('returns OFFLINE when lastDeadline is 0n (shutdown sentinel)', () => {
    const record = makeRecord({ lastDeadline: 0n });
    expect(evaluateLiveness(record, BASE_TIME)).toBe(LivenessState.OFFLINE);
  });

  // State: ALIVE -- within deadline + grace period (30s)
  it('returns ALIVE when currentTime <= deadline + GRACE_PERIOD', () => {
    const deadline = BASE_TIME + 60n * NANOS;
    const record = makeRecord({ lastDeadline: deadline });

    // Exactly at deadline
    expect(evaluateLiveness(record, deadline)).toBe(LivenessState.ALIVE);
    // 1 second after deadline (within 30s grace)
    expect(evaluateLiveness(record, deadline + 1n * NANOS)).toBe(LivenessState.ALIVE);
    // Exactly at deadline + GRACE_PERIOD (30s)
    expect(evaluateLiveness(record, deadline + 30n * NANOS)).toBe(LivenessState.ALIVE);
  });

  // State: SUSPECT -- past grace, within suspect window
  it('returns SUSPECT when currentTime > deadline + GRACE and <= deadline + GRACE + S2D', () => {
    const deadline = BASE_TIME + 60n * NANOS;
    const record = makeRecord({ lastDeadline: deadline });

    // 1 nanosecond past grace period
    expect(evaluateLiveness(record, deadline + 30n * NANOS + 1n)).toBe(LivenessState.SUSPECT);
    // Midway through suspect window
    expect(evaluateLiveness(record, deadline + 90n * NANOS)).toBe(LivenessState.SUSPECT);
    // Exactly at deadline + GRACE + S2D (30 + 120 = 150s)
    expect(evaluateLiveness(record, deadline + 150n * NANOS)).toBe(LivenessState.SUSPECT);
  });

  // State: DEAD -- past all tolerance windows
  it('returns DEAD when currentTime > deadline + GRACE + S2D', () => {
    const deadline = BASE_TIME + 60n * NANOS;
    const record = makeRecord({ lastDeadline: deadline });

    // 1 nanosecond past the full tolerance window
    expect(evaluateLiveness(record, deadline + 150n * NANOS + 1n)).toBe(LivenessState.DEAD);
    // Well past
    expect(evaluateLiveness(record, deadline + 300n * NANOS)).toBe(LivenessState.DEAD);
  });

  // -------------------------------------------------------------------------
  // 8 transitions
  // -------------------------------------------------------------------------

  // Transition 1: UNKNOWN -> ALIVE (first valid HB)
  // Tested via updateLivenessRecord below

  // Transition 2: ALIVE -> ALIVE (new valid HB resets deadline)
  // Tested via updateLivenessRecord below

  // Transition 3: ALIVE -> SUSPECT (time passes past deadline + grace)
  it('transitions ALIVE -> SUSPECT as time passes', () => {
    const deadline = BASE_TIME + 60n * NANOS;
    const record = makeRecord({ lastDeadline: deadline, currentState: LivenessState.ALIVE });

    // Still alive
    expect(evaluateLiveness(record, deadline + 29n * NANOS)).toBe(LivenessState.ALIVE);
    // Now suspect
    expect(evaluateLiveness(record, deadline + 31n * NANOS)).toBe(LivenessState.SUSPECT);
  });

  // Transition 4: ALIVE -> OFFLINE (deadline = 0n)
  // Tested by updateLivenessRecord setting deadline to 0n

  // Transition 5: SUSPECT -> ALIVE (new valid HB)
  // Tested via updateLivenessRecord below

  // Transition 6: SUSPECT -> DEAD (time passes past suspect window)
  it('transitions SUSPECT -> DEAD as time passes', () => {
    const deadline = BASE_TIME + 60n * NANOS;
    const record = makeRecord({ lastDeadline: deadline, currentState: LivenessState.SUSPECT });

    // Still suspect at boundary
    expect(evaluateLiveness(record, deadline + 150n * NANOS)).toBe(LivenessState.SUSPECT);
    // Now dead
    expect(evaluateLiveness(record, deadline + 151n * NANOS)).toBe(LivenessState.DEAD);
  });

  // Transition 7: DEAD -> ALIVE (new valid HB — recovery)
  // Tested via updateLivenessRecord below

  // Transition 8: OFFLINE -> ALIVE (new valid HB with deadline > 0)
  // Tested via updateLivenessRecord below
});

describe('updateLivenessRecord', () => {
  const SENDER = '0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf';

  // Transition 1: null -> ALIVE (first heartbeat)
  it('creates ALIVE record from null on first valid heartbeat', () => {
    const deadline = BASE_TIME + 60n * NANOS;
    const result = updateLivenessRecord(null, deadline, BASE_TIME, 1n, SENDER);

    expect(result.senderAddress).toBe(SENDER);
    expect(result.currentState).toBe(LivenessState.ALIVE);
    expect(result.lastDeadline).toBe(deadline);
    expect(result.lastSequence).toBe(1n);
    expect(result.lastConsensusTimestamp).toBe(BASE_TIME);
  });

  // Transition 1 variant: null -> OFFLINE (first heartbeat is shutdown)
  it('creates OFFLINE record from null on shutdown heartbeat', () => {
    const result = updateLivenessRecord(null, 0n, BASE_TIME, 1n, SENDER);
    expect(result.currentState).toBe(LivenessState.OFFLINE);
    expect(result.lastDeadline).toBe(0n);
  });

  // Transition 2: ALIVE -> ALIVE (new valid HB, resets deadline)
  it('updates deadline on new valid heartbeat (ALIVE -> ALIVE)', () => {
    const record = makeRecord({ lastSequence: 1n, lastDeadline: BASE_TIME + 60n * NANOS });
    const newDeadline = BASE_TIME + 120n * NANOS;
    const result = updateLivenessRecord(record, newDeadline, BASE_TIME + 50n * NANOS, 2n, SENDER);

    expect(result.currentState).toBe(LivenessState.ALIVE);
    expect(result.lastDeadline).toBe(newDeadline);
    expect(result.lastSequence).toBe(2n);
  });

  // Transition 4: ALIVE -> OFFLINE (shutdown heartbeat)
  it('transitions to OFFLINE on shutdown heartbeat', () => {
    const record = makeRecord({ lastSequence: 1n, currentState: LivenessState.ALIVE });
    const result = updateLivenessRecord(record, 0n, BASE_TIME + 50n * NANOS, 2n, SENDER);

    expect(result.currentState).toBe(LivenessState.OFFLINE);
    expect(result.lastDeadline).toBe(0n);
  });

  // Transition 5: SUSPECT -> ALIVE (recovery)
  it('transitions SUSPECT -> ALIVE on new valid heartbeat', () => {
    const record = makeRecord({ lastSequence: 1n, currentState: LivenessState.SUSPECT });
    const newDeadline = BASE_TIME + 120n * NANOS;
    const result = updateLivenessRecord(record, newDeadline, BASE_TIME + 100n * NANOS, 2n, SENDER);

    expect(result.currentState).toBe(LivenessState.ALIVE);
    expect(result.lastDeadline).toBe(newDeadline);
  });

  // Transition 7: DEAD -> ALIVE (recovery) — FR-H21
  it('transitions DEAD -> ALIVE on new valid heartbeat (recovery invariant FR-H21)', () => {
    const record = makeRecord({ lastSequence: 1n, currentState: LivenessState.DEAD });
    const newDeadline = BASE_TIME + 300n * NANOS;
    const result = updateLivenessRecord(record, newDeadline, BASE_TIME + 250n * NANOS, 2n, SENDER);

    expect(result.currentState).toBe(LivenessState.ALIVE);
    expect(result.lastDeadline).toBe(newDeadline);
  });

  // Transition 8: OFFLINE -> ALIVE (recovery with deadline > 0)
  it('transitions OFFLINE -> ALIVE on new valid heartbeat with deadline > 0', () => {
    const record = makeRecord({ lastSequence: 1n, lastDeadline: 0n, currentState: LivenessState.OFFLINE });
    const newDeadline = BASE_TIME + 120n * NANOS;
    const result = updateLivenessRecord(record, newDeadline, BASE_TIME + 60n * NANOS, 2n, SENDER);

    expect(result.currentState).toBe(LivenessState.ALIVE);
    expect(result.lastDeadline).toBe(newDeadline);
  });

  // FR-H17: Sequence ordering — ignore lower/equal sequence numbers
  it('ignores heartbeat with lower sequence number (FR-H17)', () => {
    const record = makeRecord({ lastSequence: 5n, lastDeadline: BASE_TIME + 60n * NANOS });
    const result = updateLivenessRecord(record, BASE_TIME + 120n * NANOS, BASE_TIME + 50n * NANOS, 3n, SENDER);

    // Record unchanged
    expect(result.lastSequence).toBe(5n);
    expect(result.lastDeadline).toBe(BASE_TIME + 60n * NANOS);
    expect(result).toBe(record); // Same reference
  });

  it('ignores heartbeat with equal sequence number (FR-H17)', () => {
    const record = makeRecord({ lastSequence: 5n, lastDeadline: BASE_TIME + 60n * NANOS });
    const result = updateLivenessRecord(record, BASE_TIME + 120n * NANOS, BASE_TIME + 50n * NANOS, 5n, SENDER);

    expect(result.lastSequence).toBe(5n);
    expect(result).toBe(record);
  });

  it('accepts heartbeat with higher sequence number', () => {
    const record = makeRecord({ lastSequence: 5n, lastDeadline: BASE_TIME + 60n * NANOS });
    const newDeadline = BASE_TIME + 120n * NANOS;
    const result = updateLivenessRecord(record, newDeadline, BASE_TIME + 50n * NANOS, 6n, SENDER);

    expect(result.lastSequence).toBe(6n);
    expect(result.lastDeadline).toBe(newDeadline);
    expect(result).not.toBe(record);
  });

  // Recovery invariant (FR-H21): Any state can recover
  it('FR-H21: recovery from UNKNOWN state (null record)', () => {
    const deadline = BASE_TIME + 60n * NANOS;
    const result = updateLivenessRecord(null, deadline, BASE_TIME, 1n, SENDER);
    expect(result.currentState).toBe(LivenessState.ALIVE);
  });

  it('FR-H21: recovery possible from every non-null state', () => {
    const states = [LivenessState.ALIVE, LivenessState.SUSPECT, LivenessState.DEAD, LivenessState.OFFLINE];
    for (const state of states) {
      const record = makeRecord({
        lastSequence: 1n,
        currentState: state,
        lastDeadline: state === LivenessState.OFFLINE ? 0n : BASE_TIME + 60n * NANOS,
      });
      const newDeadline = BASE_TIME + 120n * NANOS;
      const result = updateLivenessRecord(record, newDeadline, BASE_TIME + 100n * NANOS, 2n, SENDER);
      expect(result.currentState).toBe(LivenessState.ALIVE);
    }
  });
});
