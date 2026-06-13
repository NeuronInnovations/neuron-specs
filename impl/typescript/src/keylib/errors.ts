/**
 * KeyError — structured error type for the KEY domain.
 *
 * Spec reference: 006 error-taxonomy.md, KEY Domain (NEURON-KEY-001..014)
 * FR-008: Structured error types with specific error kinds and descriptive messages.
 * FR-008a: SDKError wraps underlying blockchain SDK errors.
 * SEC-003, SEC-005: Error messages MUST NOT contain private key material.
 */

import {
  NeuronError,
  KEY_INVALID_FORMAT,
  KEY_UNSUPPORTED_KEY_TYPE,
  KEY_INVALID_LENGTH,
  KEY_INVALID_HEX,
  KEY_INVALID_KEY,
  KEY_ZERO_VALUE,
  KEY_KEY_MISMATCH,
  KEY_ENCRYPTION_FAILED,
  KEY_DECRYPTION_FAILED,
  KEY_INVALID_MNEMONIC,
  KEY_DERIVATION_FAILED,
  KEY_SDK_ERROR,
  KEY_SIGNING_FAILED,
  KEY_VERIFICATION_FAILED,
} from '../errors.js';

export class KeyError extends NeuronError {}

// --- Factory functions for each error code ---

/** NEURON-KEY-001: Input format not recognized. FR-008 */
export function invalidFormat(message: string): KeyError {
  return new KeyError(KEY_INVALID_FORMAT, 'InvalidFormat', message);
}

/** NEURON-KEY-002: Ed25519 or non-secp256k1 key detected. FR-004, FR-A14 */
export function unsupportedKeyType(message: string): KeyError {
  return new KeyError(KEY_UNSUPPORTED_KEY_TYPE, 'UnsupportedKeyType', message);
}

/** NEURON-KEY-003: Input has wrong byte length. FR-008 */
export function invalidLength(expected: number, actual: number): KeyError {
  return new KeyError(
    KEY_INVALID_LENGTH,
    'InvalidLength',
    `Expected ${expected.toString()} bytes, got ${actual.toString()}`,
  );
}

/** NEURON-KEY-004: Non-hex characters in hex string input. FR-008 */
export function invalidHex(message: string): KeyError {
  return new KeyError(KEY_INVALID_HEX, 'InvalidHex', message);
}

/** NEURON-KEY-005: Key bytes fail secp256k1 curve validation. FR-008 */
export function invalidKey(message: string): KeyError {
  return new KeyError(KEY_INVALID_KEY, 'InvalidKey', message);
}

/** NEURON-KEY-006: All-zero key material provided. FR-008 */
export function zeroValue(): KeyError {
  return new KeyError(KEY_ZERO_VALUE, 'ZeroValue', 'All-zero key material is not a valid private key');
}

/** NEURON-KEY-007: Key relationship verification failed. FR-008 */
export function keyMismatch(message: string): KeyError {
  return new KeyError(KEY_KEY_MISMATCH, 'KeyMismatch', message);
}

/** NEURON-KEY-008: Argon2id or AES-GCM encryption failed. FR-008 */
export function encryptionFailed(message: string, cause?: Error): KeyError {
  return new KeyError(KEY_ENCRYPTION_FAILED, 'EncryptionFailed', message, cause);
}

/** NEURON-KEY-009: AES-GCM decryption failed. FR-008 */
export function decryptionFailed(message: string, cause?: Error): KeyError {
  return new KeyError(KEY_DECRYPTION_FAILED, 'DecryptionFailed', message, cause);
}

/** NEURON-KEY-010: Bad mnemonic. FR-008 */
export function invalidMnemonic(message: string): KeyError {
  return new KeyError(KEY_INVALID_MNEMONIC, 'InvalidMnemonic', message);
}

/** NEURON-KEY-011: BIP-44 HD key derivation failed. FR-008 */
export function derivationFailed(message: string, cause?: Error): KeyError {
  return new KeyError(KEY_DERIVATION_FAILED, 'DerivationFailed', message, cause);
}

/** NEURON-KEY-012: Wrapped underlying blockchain SDK error. FR-008a */
export function sdkError(message: string, cause: Error): KeyError {
  return new KeyError(KEY_SDK_ERROR, 'SDKError', message, cause);
}

/** NEURON-KEY-013: ECDSA signing operation failed. FR-008 */
export function signingFailed(message: string, cause?: Error): KeyError {
  return new KeyError(KEY_SIGNING_FAILED, 'SigningFailed', message, cause);
}

/** NEURON-KEY-014: Signature verification failed. FR-008 */
export function verificationFailed(message: string, cause?: Error): KeyError {
  return new KeyError(KEY_VERIFICATION_FAILED, 'VerificationFailed', message, cause);
}
