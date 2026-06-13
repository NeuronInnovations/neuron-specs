/**
 * Health module barrel exports.
 *
 * Spec reference: 005 spec.md
 * Exports all public types, constants, and functions for the health module.
 */

// Constants (FR-H06..H09, FR-H12, FR-H29)
export {
  MIN_DEADLINE_DELTA,
  MAX_DEADLINE_DELTA,
  GRACE_PERIOD,
  SUSPECT_TO_DEAD,
  SHUTDOWN_SENTINEL,
  MAX_MANDATORY_PAYLOAD_SIZE,
} from './constants.js';

// Types (FR-H02, FR-H03, FR-H05, FR-H18)
export type { NodeRole, NATType, GPSFixQuality, Capabilities, Location, LivenessRecord } from './types.js';
export { VALID_ROLES, VALID_NAT_TYPES, VALID_GPS_FIX_QUALITIES, LivenessState } from './types.js';

// HeartbeatPayload (FR-H01, FR-H02, FR-H04)
export { HeartbeatPayload } from './payload.js';
export type { HeartbeatPayloadOpts } from './payload.js';

// Publisher validation (FR-H13, V-PUB-01..07)
export { validateOutboundHeartbeat } from './publisher.js';

// Observer validation (FR-H15, V-OBS-01..06)
export { validateInboundHeartbeat } from './observer.js';
export type { InboundValidationResult } from './observer.js';

// Liveness state machine (FR-H18..H21)
export { evaluateLiveness, updateLivenessRecord } from './liveness.js';

// Errors (NEURON-HEALTH-001..007)
export {
  invalidPayloadType,
  invalidVersion,
  invalidDeadline,
  invalidRole,
  payloadTooLarge,
  invalidLocation,
  invalidCapabilities,
} from './errors.js';
