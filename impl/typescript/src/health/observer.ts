/**
 * Health observer validation.
 *
 * Spec reference: 005 contracts/health-observer.md
 *   - V-OBS-01: Signature verified (delegated to TopicMessage layer)
 *   - V-OBS-02: payload.type === "heartbeat"
 *   - V-OBS-03: payload.version major === 1
 *   - V-OBS-04: If deadline === 0n, return valid (caller handles OFFLINE)
 *   - V-OBS-05: deadline > consensusTimestamp
 *   - V-OBS-06: MIN_DEADLINE_DELTA <= delta <= MAX_DEADLINE_DELTA
 *
 * FR-H16: ALL deadline arithmetic uses consensusTimestamp.
 *
 * The function accepts a raw JSON string and validates structurally.
 * Signature verification (V-OBS-01) is handled at the TopicMessage level.
 */

import { MIN_DEADLINE_DELTA, MAX_DEADLINE_DELTA } from './constants.js';

/** One second in nanoseconds. */
const NANOS_PER_SECOND = 1_000_000_000n;

/** Result of inbound heartbeat validation. */
export interface InboundValidationResult {
  readonly valid: boolean;
  readonly error?: string;
}

/**
 * Observer-side validation of an inbound heartbeat.
 *
 * FR-H15: All V-OBS checks MUST pass before updating liveness state.
 * FR-H16: Clock authority is consensusTimestamp. NEVER local clock.
 *
 * V-OBS-01 (signature) is delegated to the TopicMessage layer -- the caller
 * must ensure signature verification before calling this function.
 *
 * @param payloadJson - Raw canonical JSON string of the HeartbeatPayload
 * @param consensusTimestamp - Consensus timestamp in nanoseconds (from MessageDelivery)
 * @returns Validation result: { valid, error? }
 */
export function validateInboundHeartbeat(
  payloadJson: string,
  consensusTimestamp: bigint,
): InboundValidationResult {
  let parsed: Record<string, unknown>;
  try {
    parsed = JSON.parse(payloadJson) as Record<string, unknown>;
  } catch {
    return { valid: false, error: 'invalid JSON' };
  }

  // V-OBS-02: type === "heartbeat"
  if (parsed['type'] !== 'heartbeat') {
    return { valid: false, error: 'not a heartbeat message' };
  }

  // V-OBS-03: version major === 1
  const version = parsed['version'];
  if (typeof version !== 'string') {
    return { valid: false, error: 'incompatible heartbeat version' };
  }
  const majorStr = version.split('.')[0];
  if (majorStr === undefined || majorStr !== '1') {
    return { valid: false, error: 'incompatible heartbeat version' };
  }

  // Parse deadline
  const deadlineStr = parsed['nextHeartbeatDeadline'];
  if (typeof deadlineStr !== 'string') {
    return { valid: false, error: 'missing or invalid nextHeartbeatDeadline' };
  }
  let deadline: bigint;
  try {
    deadline = BigInt(deadlineStr);
  } catch {
    return { valid: false, error: 'invalid nextHeartbeatDeadline format' };
  }

  // V-OBS-04: If deadline === 0n → valid (caller MUST transition to OFFLINE)
  if (deadline === 0n) {
    return { valid: true };
  }

  // V-OBS-05: deadline > consensusTimestamp
  if (deadline <= consensusTimestamp) {
    return { valid: false, error: 'deadline is in the past relative to consensus time' };
  }

  // V-OBS-06: MIN_DEADLINE_DELTA <= delta <= MAX_DEADLINE_DELTA
  const deltaNanos = deadline - consensusTimestamp;
  const deltaSeconds = deltaNanos / NANOS_PER_SECOND;

  if (deltaSeconds < MIN_DEADLINE_DELTA) {
    return { valid: false, error: 'deadline delta below minimum' };
  }
  if (deltaSeconds > MAX_DEADLINE_DELTA) {
    return { valid: false, error: 'deadline delta exceeds maximum' };
  }

  return { valid: true };
}
