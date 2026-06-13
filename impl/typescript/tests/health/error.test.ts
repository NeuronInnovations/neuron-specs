/**
 * Tests for health module error factories.
 *
 * Spec reference: 005 spec.md, 006 error-taxonomy.md
 * Verifies each NEURON-HEALTH-001..007 error factory produces
 * the correct code, name, and message format.
 */

import { describe, it, expect } from 'vitest';
import { NeuronError } from '../../src/errors.js';
import {
  HEALTH_INVALID_PAYLOAD_TYPE,
  HEALTH_INVALID_VERSION,
  HEALTH_INVALID_DEADLINE,
  HEALTH_INVALID_ROLE,
  HEALTH_PAYLOAD_TOO_LARGE,
  HEALTH_INVALID_LOCATION,
  HEALTH_INVALID_CAPABILITIES,
} from '../../src/errors.js';
import {
  invalidPayloadType,
  invalidVersion,
  invalidDeadline,
  invalidRole,
  payloadTooLarge,
  invalidLocation,
  invalidCapabilities,
} from '../../src/health/errors.js';

describe('Health Errors', () => {
  it('invalidPayloadType returns NEURON-HEALTH-001', () => {
    const err = invalidPayloadType('status');
    expect(err).toBeInstanceOf(NeuronError);
    expect(err.code).toBe(HEALTH_INVALID_PAYLOAD_TYPE);
    expect(err.code).toBe('NEURON-HEALTH-001');
    expect(err.name).toBe('InvalidPayloadType');
    expect(err.message).toContain('heartbeat');
    expect(err.message).toContain('status');
  });

  it('invalidVersion returns NEURON-HEALTH-002', () => {
    const err = invalidVersion('2.0.0');
    expect(err).toBeInstanceOf(NeuronError);
    expect(err.code).toBe(HEALTH_INVALID_VERSION);
    expect(err.code).toBe('NEURON-HEALTH-002');
    expect(err.name).toBe('InvalidVersion');
    expect(err.message).toContain('2.0.0');
  });

  it('invalidDeadline returns NEURON-HEALTH-003', () => {
    const err = invalidDeadline('deadline must be in the future');
    expect(err).toBeInstanceOf(NeuronError);
    expect(err.code).toBe(HEALTH_INVALID_DEADLINE);
    expect(err.code).toBe('NEURON-HEALTH-003');
    expect(err.name).toBe('InvalidDeadline');
    expect(err.message).toBe('deadline must be in the future');
  });

  it('invalidRole returns NEURON-HEALTH-004', () => {
    const err = invalidRole('miner');
    expect(err).toBeInstanceOf(NeuronError);
    expect(err.code).toBe(HEALTH_INVALID_ROLE);
    expect(err.code).toBe('NEURON-HEALTH-004');
    expect(err.name).toBe('InvalidRole');
    expect(err.message).toContain('miner');
  });

  it('payloadTooLarge returns NEURON-HEALTH-005', () => {
    const err = payloadTooLarge(512, 256);
    expect(err).toBeInstanceOf(NeuronError);
    expect(err.code).toBe(HEALTH_PAYLOAD_TOO_LARGE);
    expect(err.code).toBe('NEURON-HEALTH-005');
    expect(err.name).toBe('PayloadTooLarge');
    expect(err.message).toContain('512');
    expect(err.message).toContain('256');
  });

  it('invalidLocation returns NEURON-HEALTH-006', () => {
    const err = invalidLocation('lat is required when location is present');
    expect(err).toBeInstanceOf(NeuronError);
    expect(err.code).toBe(HEALTH_INVALID_LOCATION);
    expect(err.code).toBe('NEURON-HEALTH-006');
    expect(err.name).toBe('InvalidLocation');
    expect(err.message).toContain('lat');
  });

  it('invalidCapabilities returns NEURON-HEALTH-007', () => {
    const err = invalidCapabilities('natType must be a valid NATType');
    expect(err).toBeInstanceOf(NeuronError);
    expect(err.code).toBe(HEALTH_INVALID_CAPABILITIES);
    expect(err.code).toBe('NEURON-HEALTH-007');
    expect(err.name).toBe('InvalidCapabilities');
    expect(err.message).toContain('natType');
  });

  it('all errors extend Error', () => {
    const errors = [
      invalidPayloadType('x'),
      invalidVersion('x'),
      invalidDeadline('x'),
      invalidRole('x'),
      payloadTooLarge(1, 0),
      invalidLocation('x'),
      invalidCapabilities('x'),
    ];
    for (const err of errors) {
      expect(err).toBeInstanceOf(Error);
    }
  });
});
