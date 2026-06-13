/**
 * Liveness state machine.
 *
 * Spec reference: 005 spec.md, data-model.md, contracts/health-observer.md
 *   - FR-H18: LivenessRecord tracks per-peer state
 *   - FR-H19: 5-state model (UNKNOWN, ALIVE, SUSPECT, DEAD, OFFLINE)
 *   - FR-H20: State evaluation algorithm
 *   - FR-H21: Recovery invariant -- any valid heartbeat transitions to ALIVE from any state
 *   - FR-H17: Sequence ordering -- only highest sequence number is authoritative
 *
 * All time values (currentTime, deadlines, constants) are in nanoseconds.
 */

import { LivenessState } from './types.js';
import type { LivenessRecord } from './types.js';
import { GRACE_PERIOD, SUSPECT_TO_DEAD } from './constants.js';

/** One second in nanoseconds. */
const NANOS_PER_SECOND = 1_000_000_000n;

/** Grace period in nanoseconds. */
const GRACE_NANOS = GRACE_PERIOD * NANOS_PER_SECOND;

/** Suspect-to-dead window in nanoseconds. */
const S2D_NANOS = SUSPECT_TO_DEAD * NANOS_PER_SECOND;

/**
 * Evaluate the current liveness state for a peer.
 *
 * FR-H20: Pure function of the LivenessRecord and current time.
 *
 * Algorithm:
 *   - If record is null (no heartbeat ever observed): UNKNOWN
 *   - If lastDeadline === 0n (shutdown sentinel): OFFLINE
 *   - If currentTime <= deadline + GRACE_PERIOD: ALIVE
 *   - If currentTime <= deadline + GRACE_PERIOD + SUSPECT_TO_DEAD: SUSPECT
 *   - Else: DEAD
 *
 * @param record - Per-peer liveness record (null if no heartbeat observed)
 * @param currentTime - Current time in nanoseconds
 * @returns Computed LivenessState
 */
export function evaluateLiveness(
  record: LivenessRecord | null,
  currentTime: bigint,
): LivenessState {
  // No heartbeat ever observed
  if (record === null) {
    return LivenessState.UNKNOWN;
  }

  // Shutdown sentinel (FR-H12)
  if (record.lastDeadline === 0n) {
    return LivenessState.OFFLINE;
  }

  // ALIVE: within deadline + grace period
  if (currentTime <= record.lastDeadline + GRACE_NANOS) {
    return LivenessState.ALIVE;
  }

  // SUSPECT: within deadline + grace + suspect-to-dead
  if (currentTime <= record.lastDeadline + GRACE_NANOS + S2D_NANOS) {
    return LivenessState.SUSPECT;
  }

  // DEAD: exceeded all tolerance windows
  return LivenessState.DEAD;
}

/**
 * Update a LivenessRecord when a new valid heartbeat arrives.
 *
 * FR-H17: Only the highest sequence number is authoritative.
 * FR-H21: Recovery invariant -- any valid heartbeat from any state transitions to ALIVE
 *         (or OFFLINE if deadline === 0n).
 *
 * @param record - Existing record (null for first heartbeat from this peer)
 * @param deadline - nextHeartbeatDeadline from the heartbeat payload (nanoseconds)
 * @param consensusTimestamp - Consensus timestamp from MessageDelivery (nanoseconds)
 * @param sequenceNumber - Sequence number from TopicMessage envelope
 * @param senderAddress - EVM address of the sender
 * @returns Updated LivenessRecord
 */
export function updateLivenessRecord(
  record: LivenessRecord | null,
  deadline: bigint,
  consensusTimestamp: bigint,
  sequenceNumber: bigint,
  senderAddress: string,
): LivenessRecord {
  // FR-H17: Ignore if sequenceNumber <= record.lastSequence (out-of-order or duplicate)
  if (record !== null && sequenceNumber <= record.lastSequence) {
    return record;
  }

  // Determine new state based on deadline
  const newState = deadline === 0n ? LivenessState.OFFLINE : LivenessState.ALIVE;

  return {
    senderAddress,
    currentState: newState,
    lastDeadline: deadline,
    lastSequence: sequenceNumber,
    lastConsensusTimestamp: consensusTimestamp,
  };
}
