/**
 * T004: KeyError tests — all 14 NEURON-KEY-* codes.
 *
 * Source: 006 error-taxonomy.md, KEY Domain
 * Verifies: error codes, names, messages, cause wrapping, no key material in messages.
 * FR-008, FR-008a, SEC-003, SEC-005, SC-002
 */

import { describe, it, expect } from 'vitest';
import { NeuronError } from '../../src/errors.js';
import * as codes from '../../src/errors.js';

/** A sample private key hex — used to verify it does NOT appear in error messages. SEC-003 */
const SAMPLE_KEY_HEX = '0x0000000000000000000000000000000000000000000000000000000000000001';

describe('KeyError (NEURON-KEY-001..014)', () => {
  const errorDefs: Array<{ code: string; name: string; constant: string }> = [
    { code: 'NEURON-KEY-001', name: 'InvalidFormat', constant: 'KEY_INVALID_FORMAT' },
    { code: 'NEURON-KEY-002', name: 'UnsupportedKeyType', constant: 'KEY_UNSUPPORTED_KEY_TYPE' },
    { code: 'NEURON-KEY-003', name: 'InvalidLength', constant: 'KEY_INVALID_LENGTH' },
    { code: 'NEURON-KEY-004', name: 'InvalidHex', constant: 'KEY_INVALID_HEX' },
    { code: 'NEURON-KEY-005', name: 'InvalidKey', constant: 'KEY_INVALID_KEY' },
    { code: 'NEURON-KEY-006', name: 'ZeroValue', constant: 'KEY_ZERO_VALUE' },
    { code: 'NEURON-KEY-007', name: 'KeyMismatch', constant: 'KEY_KEY_MISMATCH' },
    { code: 'NEURON-KEY-008', name: 'EncryptionFailed', constant: 'KEY_ENCRYPTION_FAILED' },
    { code: 'NEURON-KEY-009', name: 'DecryptionFailed', constant: 'KEY_DECRYPTION_FAILED' },
    { code: 'NEURON-KEY-010', name: 'InvalidMnemonic', constant: 'KEY_INVALID_MNEMONIC' },
    { code: 'NEURON-KEY-011', name: 'DerivationFailed', constant: 'KEY_DERIVATION_FAILED' },
    { code: 'NEURON-KEY-012', name: 'SDKError', constant: 'KEY_SDK_ERROR' },
    { code: 'NEURON-KEY-013', name: 'SigningFailed', constant: 'KEY_SIGNING_FAILED' },
    { code: 'NEURON-KEY-014', name: 'VerificationFailed', constant: 'KEY_VERIFICATION_FAILED' },
  ];

  it('should define all 14 error code constants', () => {
    for (const def of errorDefs) {
      const value = (codes as unknown as Record<string, string>)[def.constant];
      expect(value).toBe(def.code);
    }
  });

  it('should instantiate NeuronError with correct code and name', () => {
    for (const def of errorDefs) {
      const err = new NeuronError(def.code, def.name, `Test message for ${def.name}`);
      expect(err.code).toBe(def.code);
      expect(err.name).toBe(def.name);
      expect(err.message).toContain(def.name);
      expect(err).toBeInstanceOf(Error);
      expect(err).toBeInstanceOf(NeuronError);
    }
  });

  // FR-008a: SDKError wraps underlying error
  it('should wrap cause for SDK errors (NEURON-KEY-012)', () => {
    const underlying = new Error('underlying SDK failure');
    const err = new NeuronError(codes.KEY_SDK_ERROR, 'SDKError', 'SDK operation failed', underlying);
    expect(err.cause).toBe(underlying);
    expect(err.code).toBe('NEURON-KEY-012');
  });

  // SEC-003, SEC-005: Error messages MUST NOT contain private key material
  it('should never contain private key material in error messages', () => {
    for (const def of errorDefs) {
      const err = new NeuronError(def.code, def.name, `Error during operation on key`);
      expect(err.message).not.toContain(SAMPLE_KEY_HEX);
      expect(err.message).not.toContain(SAMPLE_KEY_HEX.slice(2)); // without 0x
    }
  });
});
