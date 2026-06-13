/**
 * Health publisher validation.
 *
 * Spec reference: 005 contracts/health-publisher.md
 *   - V-PUB-01: payload.type === "heartbeat"
 *   - V-PUB-02: payload.version is recognized (major === 1)
 *   - V-PUB-03: If deadline === 0n, return (shutdown bypass)
 *   - V-PUB-04: deadline > senderClock
 *   - V-PUB-05: delta >= MIN_DEADLINE_DELTA
 *   - V-PUB-06: delta <= MAX_DEADLINE_DELTA
 *   - V-PUB-07: role in VALID_ROLES
 *
 * senderClock is in seconds (same unit as deadline delta constants).
 * The HeartbeatPayload.nextHeartbeatDeadline is in nanoseconds,
 * so we convert to seconds before comparing with constants.
 */

import type { HeartbeatPayload } from './payload.js';
import { VALID_ROLES } from './types.js';
import { MIN_DEADLINE_DELTA, MAX_DEADLINE_DELTA } from './constants.js';
import {
  invalidPayloadType,
  invalidVersion,
  invalidDeadline,
  invalidRole,
} from './errors.js';

/** One second in nanoseconds. */
const NANOS_PER_SECOND = 1_000_000_000n;

/**
 * Publisher-side validation before signing a heartbeat.
 *
 * FR-H13: All V-PUB checks MUST pass before the payload is signed and published.
 * Throws a NeuronError on the first failing check.
 *
 * @param payload - The HeartbeatPayload to validate
 * @param senderClock - Publisher's current wall clock in nanoseconds
 * @throws NeuronError with HEALTH_INVALID_PAYLOAD_TYPE (V-PUB-01)
 * @throws NeuronError with HEALTH_INVALID_VERSION (V-PUB-02)
 * @throws NeuronError with HEALTH_INVALID_DEADLINE (V-PUB-04..06)
 * @throws NeuronError with HEALTH_INVALID_ROLE (V-PUB-07)
 */
export function validateOutboundHeartbeat(
  payload: HeartbeatPayload,
  senderClock: bigint,
): void {
  // V-PUB-01: type === "heartbeat"
  if (payload.type !== 'heartbeat') {
    throw invalidPayloadType(payload.type);
  }

  // V-PUB-02: version recognized (major === 1)
  const majorStr = payload.version.split('.')[0];
  if (majorStr === undefined || majorStr !== '1') {
    throw invalidVersion(payload.version);
  }

  // V-PUB-03: If deadline === 0n → bypass (shutdown sentinel)
  if (payload.nextHeartbeatDeadline === 0n) {
    return;
  }

  // V-PUB-04: deadline > senderClock
  if (payload.nextHeartbeatDeadline <= senderClock) {
    throw invalidDeadline('deadline must be in the future');
  }

  // Compute delta in seconds
  const deltaNanos = payload.nextHeartbeatDeadline - senderClock;
  const deltaSeconds = deltaNanos / NANOS_PER_SECOND;

  // V-PUB-05: delta >= MIN_DEADLINE_DELTA (10 seconds)
  if (deltaSeconds < MIN_DEADLINE_DELTA) {
    throw invalidDeadline(`deadline too soon: minimum delta is ${MIN_DEADLINE_DELTA.toString()} seconds`);
  }

  // V-PUB-06: delta <= MAX_DEADLINE_DELTA (86400 seconds)
  if (deltaSeconds > MAX_DEADLINE_DELTA) {
    throw invalidDeadline(`deadline too far: maximum delta is ${MAX_DEADLINE_DELTA.toString()} seconds`);
  }

  // V-PUB-07: role in VALID_ROLES
  if (!VALID_ROLES.includes(payload.role)) {
    throw invalidRole(payload.role);
  }
}
