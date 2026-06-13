/**
 * Tests for RegistryError -- REG domain error factories.
 *
 * Spec reference: 006 error-taxonomy.md, REG Domain (NEURON-REG-001..006)
 * FR-R07: All 6 error kinds must be covered.
 */

import { describe, it, expect } from 'vitest';
import {
  RegistryError,
  registrationFailed,
  lookupFailed,
  updateFailed,
  revocationFailed,
  invalidAgentURI,
  unauthorizedCaller,
} from '../../src/registry/errors.js';
import { NeuronError } from '../../src/errors.js';

describe('RegistryError', () => {
  it('should extend NeuronError', () => {
    const err = registrationFailed('test');
    expect(err).toBeInstanceOf(RegistryError);
    expect(err).toBeInstanceOf(NeuronError);
    expect(err).toBeInstanceOf(Error);
  });

  it('should preserve message, name, and code', () => {
    const err = registrationFailed('register failed');
    expect(err.code).toBe('NEURON-REG-001');
    expect(err.name).toBe('RegistrationFailed');
    expect(err.message).toBe('register failed');
  });

  it('should preserve optional cause', () => {
    const cause = new Error('underlying');
    const err = lookupFailed('lookup failed', cause);
    expect(err.cause).toBe(cause);
  });

  it('should have undefined cause when not provided', () => {
    const err = updateFailed('update failed');
    expect(err.cause).toBeUndefined();
  });
});

describe('RegistryError factory functions', () => {
  const factories = [
    { fn: registrationFailed, code: 'NEURON-REG-001', name: 'RegistrationFailed' },
    { fn: lookupFailed, code: 'NEURON-REG-002', name: 'LookupFailed' },
    { fn: updateFailed, code: 'NEURON-REG-003', name: 'UpdateFailed' },
    { fn: revocationFailed, code: 'NEURON-REG-004', name: 'RevocationFailed' },
    { fn: invalidAgentURI, code: 'NEURON-REG-005', name: 'InvalidAgentURI' },
    { fn: unauthorizedCaller, code: 'NEURON-REG-006', name: 'UnauthorizedCaller' },
  ];

  for (const { fn, code, name } of factories) {
    it(`${name} (${code}) should produce correct error`, () => {
      const err = fn('test message');
      expect(err.code).toBe(code);
      expect(err.name).toBe(name);
      expect(err.message).toBe('test message');
      expect(err).toBeInstanceOf(RegistryError);
    });
  }

  it('should cover all 6 REG error codes', () => {
    expect(factories).toHaveLength(6);
  });
});
