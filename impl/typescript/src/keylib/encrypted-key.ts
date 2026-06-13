/**
 * EncryptedPrivateKey — Argon2id KDF + AES-256-GCM encryption for private keys.
 *
 * Spec reference: 002 spec.md FR-015
 * Algorithm: 006 algorithm-reference.md §11 (FR-A11)
 *
 * Version 1 parameters (hardcoded defaults):
 *   - Time iterations: 1
 *   - Memory: 65536 KiB (64 MiB)
 *   - Parallelism: 4 threads
 *   - Salt: 16 bytes (cryptographically random)
 *   - Tag length: 32 bytes
 *   - Argon2 variant: Argon2id (hybrid)
 *
 * Encryption:
 *   1. Generate 16 random bytes for salt (or use provided for testing)
 *   2. Generate 12 random bytes for AES-GCM nonce (or use provided)
 *   3. Derive key: Argon2id(password_utf8, salt, params) → 32 bytes
 *   4. Encrypt: AES-256-GCM(key, nonce, privkey_bytes) → 48 bytes (32 ciphertext + 16 tag)
 *
 * Wire format: 006 wire-format.md §2 (FR-W05)
 *   Field order: version → salt → nonce → ciphertext
 *   Binary fields as base64 (FR-W03)
 *
 * SEC-006: Use industry-standard algorithms only.
 * SEC-003: Error messages MUST NOT reveal whether password or key was wrong.
 *
 * Immutable value type. JSON-serializable (the ONLY Neuron key type that supports JSON).
 */

import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto';
import { argon2id } from 'hash-wasm';
import { base64Encode, base64Decode } from '../wire/base64.js';
import { serializeCanonicalJson } from '../wire/canonical-json.js';
import type { CanonicalField } from '../wire/canonical-json.js';
import {
  ARGON2_V1_TIME,
  ARGON2_V1_MEMORY,
  ARGON2_V1_THREADS,
  SALT_LENGTH,
  NONCE_LENGTH,
  CIPHERTEXT_LENGTH,
} from './constants.js';
import { encryptionFailed, decryptionFailed } from './errors.js';

/** Argon2id parameters for key derivation. FR-A11 */
export interface Argon2Params {
  readonly time: number;
  readonly memory: number;
  readonly threads: number;
  readonly tagLength: number;
}

/** Default Argon2id v1 parameters. FR-A11 */
const ARGON2_V1_PARAMS: Argon2Params = {
  time: ARGON2_V1_TIME,
  memory: ARGON2_V1_MEMORY,
  threads: ARGON2_V1_THREADS,
  tagLength: 32,
};

/**
 * An encrypted private key using Argon2id KDF + AES-256-GCM.
 *
 * FR-015: JSON-serializable structure with version, salt, nonce, ciphertext.
 * The ONLY Neuron key type that supports JSON serialization.
 */
export class EncryptedPrivateKey {
  private readonly _version: number;
  private readonly _salt: Uint8Array;
  private readonly _nonce: Uint8Array;
  private readonly _ciphertext: Uint8Array;

  private constructor(
    version: number,
    salt: Uint8Array,
    nonce: Uint8Array,
    ciphertext: Uint8Array,
  ) {
    this._version = version;
    this._salt = salt;
    this._nonce = nonce;
    this._ciphertext = ciphertext;
  }

  /**
   * Derive a 32-byte encryption key from password and salt using Argon2id.
   *
   * FR-A11: Argon2id(password_utf8_bytes, salt, time, memory, threads, tag_length=32)
   *
   * @param password - User password (UTF-8 string)
   * @param salt - 16-byte salt
   * @param params - Argon2id parameters
   * @returns 32-byte derived key
   */
  static async deriveKey(
    password: string,
    salt: Uint8Array,
    params: Argon2Params,
  ): Promise<Uint8Array> {
    try {
      // FR-A11: password as UTF-8 bytes
      const passwordBytes = new TextEncoder().encode(password);

      const result = await argon2id({
        password: passwordBytes,
        salt,
        parallelism: params.threads,
        iterations: params.time,
        memorySize: params.memory,
        hashLength: params.tagLength,
        outputType: 'binary',
      });
      return new Uint8Array(result);
    } catch (err) {
      throw encryptionFailed(
        'Argon2id key derivation failed',
        err instanceof Error ? err : new Error(String(err)),
      );
    }
  }

  /**
   * Encrypt a private key with a password.
   *
   * FR-015, FR-A11: Argon2id KDF + AES-256-GCM authenticated encryption.
   *
   * @param keyBytes - 32-byte private key to encrypt
   * @param password - User password
   * @param salt - Optional 16-byte salt (random if omitted; fixed for testing)
   * @param nonce - Optional 12-byte nonce (random if omitted; fixed for testing)
   * @returns EncryptedPrivateKey instance
   */
  static async encrypt(
    keyBytes: Uint8Array,
    password: string,
    salt?: Uint8Array,
    nonce?: Uint8Array,
  ): Promise<EncryptedPrivateKey> {
    // Use provided or generate random salt/nonce
    const actualSalt = salt ?? randomBytes(SALT_LENGTH);
    const actualNonce = nonce ?? randomBytes(NONCE_LENGTH);

    if (actualSalt.length !== SALT_LENGTH) {
      throw encryptionFailed(`Salt must be ${SALT_LENGTH.toString()} bytes, got ${actualSalt.length.toString()}`);
    }
    if (actualNonce.length !== NONCE_LENGTH) {
      throw encryptionFailed(`Nonce must be ${NONCE_LENGTH.toString()} bytes, got ${actualNonce.length.toString()}`);
    }

    // FR-A11 Step 3: Derive encryption key
    const derivedKey = await EncryptedPrivateKey.deriveKey(password, actualSalt, ARGON2_V1_PARAMS);

    // FR-A11 Step 4: AES-256-GCM encrypt
    try {
      const cipher = createCipheriv('aes-256-gcm', derivedKey, actualNonce);
      const encrypted = cipher.update(keyBytes);
      const final = cipher.final();
      const authTag = cipher.getAuthTag(); // 16 bytes

      // Ciphertext = encrypted data + GCM auth tag (48 bytes total)
      const ciphertext = new Uint8Array(encrypted.length + final.length + authTag.length);
      ciphertext.set(encrypted, 0);
      ciphertext.set(final, encrypted.length);
      ciphertext.set(authTag, encrypted.length + final.length);

      return new EncryptedPrivateKey(1, actualSalt, actualNonce, ciphertext);
    } catch (err) {
      throw encryptionFailed(
        'AES-256-GCM encryption failed',
        err instanceof Error ? err : new Error(String(err)),
      );
    }
  }

  /**
   * Decrypt the encrypted private key with a password.
   *
   * FR-015, FR-A11: Derive key with Argon2id, decrypt with AES-256-GCM.
   * SEC-003: Error does NOT reveal whether password or key was wrong.
   *
   * @param password - User password
   * @returns 32-byte decrypted private key
   */
  async decrypt(password: string): Promise<Uint8Array> {
    // Determine Argon2 params based on version
    const params = this._version === 1 ? ARGON2_V1_PARAMS : ARGON2_V1_PARAMS; // v2 TODO: read from fields

    // FR-A11 Step 3: Derive key
    const derivedKey = await EncryptedPrivateKey.deriveKey(password, this._salt, params);

    // FR-A11 Step 4: AES-256-GCM decrypt
    try {
      // Separate ciphertext body from auth tag (last 16 bytes)
      const ciphertextBody = this._ciphertext.slice(0, this._ciphertext.length - 16);
      const authTag = this._ciphertext.slice(this._ciphertext.length - 16);

      const decipher = createDecipheriv('aes-256-gcm', derivedKey, this._nonce);
      decipher.setAuthTag(authTag);
      const decrypted = decipher.update(ciphertextBody);
      const final = decipher.final();

      const result = new Uint8Array(decrypted.length + final.length);
      result.set(decrypted, 0);
      result.set(final, decrypted.length);
      return result;
    } catch (err) {
      // SEC-003: Do not reveal whether password or key was wrong
      throw decryptionFailed(
        'Decryption failed — verify password and encrypted key data',
        err instanceof Error ? err : new Error(String(err)),
      );
    }
  }

  /**
   * Return a defensive copy of the ciphertext bytes.
   *
   * @returns 48-byte ciphertext (32 encrypted key + 16 GCM tag)
   */
  ciphertextBytes(): Uint8Array {
    return new Uint8Array(this._ciphertext);
  }

  /**
   * Serialize to canonical JSON per wire format.
   *
   * FR-W05 field order: version → salt → nonce → ciphertext
   * FR-W03: Binary fields as base64
   */
  toCanonicalJson(): string {
    const fields: CanonicalField[] = [
      { key: 'version', type: 'number', value: this._version },
      { key: 'salt', type: 'string', value: base64Encode(this._salt) },
      { key: 'nonce', type: 'string', value: base64Encode(this._nonce) },
      { key: 'ciphertext', type: 'string', value: base64Encode(this._ciphertext) },
    ];
    return serializeCanonicalJson(fields);
  }

  /**
   * Parse an EncryptedPrivateKey from canonical JSON.
   *
   * @param json - JSON string with version, salt, nonce, ciphertext fields
   * @returns EncryptedPrivateKey instance
   */
  static fromJson(json: string): EncryptedPrivateKey {
    try {
      const parsed = JSON.parse(json) as {
        version?: number;
        salt?: string;
        nonce?: string;
        ciphertext?: string;
      };

      if (parsed.version === undefined || parsed.salt === undefined ||
          parsed.nonce === undefined || parsed.ciphertext === undefined) {
        throw new Error('Missing required fields');
      }

      const salt = base64Decode(parsed.salt);
      const nonce = base64Decode(parsed.nonce);
      const ciphertext = base64Decode(parsed.ciphertext);

      return new EncryptedPrivateKey(parsed.version, salt, nonce, ciphertext);
    } catch (err) {
      if (err instanceof Error && err.message.includes('Missing required fields')) {
        throw encryptionFailed('Invalid EncryptedPrivateKey JSON: missing required fields');
      }
      throw encryptionFailed(
        'Failed to parse EncryptedPrivateKey JSON',
        err instanceof Error ? err : new Error(String(err)),
      );
    }
  }
}
