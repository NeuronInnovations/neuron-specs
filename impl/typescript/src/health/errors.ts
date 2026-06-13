/**
 * Health module error factories.
 *
 * Spec reference: 005 spec.md, 006 error-taxonomy.md
 * Domain: NEURON-HEALTH-001..007
 *
 * Each factory function constructs a NeuronError with the appropriate code,
 * name, and descriptive message. Error codes are imported from the central
 * error code registry in src/errors.ts.
 */

import { NeuronError } from '../errors.js';
import {
  HEALTH_INVALID_PAYLOAD_TYPE,
  HEALTH_INVALID_VERSION,
  HEALTH_INVALID_DEADLINE,
  HEALTH_INVALID_ROLE,
  HEALTH_PAYLOAD_TOO_LARGE,
  HEALTH_INVALID_LOCATION,
  HEALTH_INVALID_CAPABILITIES,
} from '../errors.js';

/** NEURON-HEALTH-001: Payload type is not "heartbeat". V-PUB-01, V-OBS-02 */
export function invalidPayloadType(got: string): NeuronError {
  return new NeuronError(
    HEALTH_INVALID_PAYLOAD_TYPE,
    'InvalidPayloadType',
    `Expected payload type "heartbeat", got "${got}"`,
  );
}

/** NEURON-HEALTH-002: Heartbeat version not recognized. V-PUB-02, V-OBS-03 */
export function invalidVersion(got: string): NeuronError {
  return new NeuronError(
    HEALTH_INVALID_VERSION,
    'InvalidVersion',
    `Unsupported heartbeat version: "${got}"`,
  );
}

/** NEURON-HEALTH-003: Deadline validation failed. V-PUB-04..06, V-OBS-05..06 */
export function invalidDeadline(reason: string): NeuronError {
  return new NeuronError(
    HEALTH_INVALID_DEADLINE,
    'InvalidDeadline',
    reason,
  );
}

/** NEURON-HEALTH-004: Role not in allowed set. V-PUB-07 */
export function invalidRole(got: string): NeuronError {
  return new NeuronError(
    HEALTH_INVALID_ROLE,
    'InvalidRole',
    `Unrecognized role: "${got}"`,
  );
}

/** NEURON-HEALTH-005: Payload exceeds size budget. FR-H29 */
export function payloadTooLarge(size: number, max: number): NeuronError {
  return new NeuronError(
    HEALTH_PAYLOAD_TOO_LARGE,
    'PayloadTooLarge',
    `Payload size ${size.toString()} bytes exceeds maximum ${max.toString()} bytes`,
  );
}

/** NEURON-HEALTH-006: Location object invalid. FR-H03 */
export function invalidLocation(reason: string): NeuronError {
  return new NeuronError(
    HEALTH_INVALID_LOCATION,
    'InvalidLocation',
    reason,
  );
}

/** NEURON-HEALTH-007: Capabilities object invalid. FR-H03 */
export function invalidCapabilities(reason: string): NeuronError {
  return new NeuronError(
    HEALTH_INVALID_CAPABILITIES,
    'InvalidCapabilities',
    reason,
  );
}
