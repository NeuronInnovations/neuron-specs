/**
 * NeuronPrivateKey — immutable, type-safe secp256k1 private key.
 *
 * Spec reference: 002 spec.md
 *   - FR-001: NeuronPrivateKey produces NeuronPublicKey via derivation.
 *   - FR-002: Elevation from raw bytes/hex/mnemonic with validation at construction.
 *   - FR-003: Defaults to ECDSA secp256k1.
 *   - FR-008: Structured error types with specific error kinds.
 *   - FR-009: Type-safe function signatures (not strings).
 *   - FR-010: Factory methods accept hex strings, raw bytes.
 *   - FR-012: Cryptographically secure key generation.
 *   - FR-013: BIP-39 mnemonic restoration with BIP-44 derivation paths.
 *   - FR-014: ECDSA signing with R||S||V format (65 bytes).
 *   - FR-017: Keccak256 hashing before ECDSA signing.
 *   - FR-018: If a NeuronPrivateKey instance exists, it is guaranteed valid.
 *   - FR-020: Clear API naming (generate, fromHex, fromBytes, fromMnemonic).
 *   - FR-021: Zeroize private key material from memory.
 *   - FR-022: Immutable after construction, except zeroize().
 *
 * Algorithm reference: 006 spec.md
 *   - FR-A01 (Key Generation): 1 <= k < n (secp256k1 order).
 *   - FR-A07 (RFC 6979): Deterministic nonce generation.
 *   - FR-A08 (Keccak256 Pre-Image): Hash before signing.
 *   - FR-A10 (Signature Encoding): R||S||V with low-S normalization.
 *   - FR-A12 (BIP-39): Mnemonic to seed via PBKDF2.
 *   - FR-A13 (BIP-44): Derivation path m/44'/60'/0'/0/0.
 *
 * SEC-003: Private key material MUST NOT appear in error messages or logs.
 * SEC-005: All inputs validated before processing.
 *
 * Immutable value type (except zeroize). Valid by construction.
 */

import * as secp from '@noble/secp256k1';
import { hmac } from '@noble/hashes/hmac';
import { sha256 } from '@noble/hashes/sha256';
import { keccak_256 } from '@noble/hashes/sha3';

// Configure @noble/secp256k1 v2 with HMAC-SHA256 for synchronous RFC 6979 signing.
// FR-A07: RFC 6979 deterministic nonce generation requires HMAC-SHA256.
secp.etc.hmacSha256Sync = (k: Uint8Array, ...m: Uint8Array[]): Uint8Array => {
  const h = hmac.create(sha256, k);
  for (const msg of m) h.update(msg);
  return h.digest();
};
import * as bip39 from '@scure/bip39';
import { wordlist } from '@scure/bip39/wordlists/english';
import { HDKey } from '@scure/bip32';
import {
  PRIVATE_KEY_LENGTH,
  SECP256K1_ORDER,
  BIP44_DEFAULT_PATH,
} from './constants.js';
import {
  KeyError,
  invalidHex,
  invalidLength,
  invalidKey,
  zeroValue,
  invalidMnemonic,
  derivationFailed,
  signingFailed,
} from './errors.js';
import { bytesToHex, hexToBytes } from '../wire/hex.js';
import { constantTimeEqual } from './matching.js';
import { NeuronPublicKey } from './public-key.js';
import { Signature } from './signature.js';
import type { EVMAddress } from './evm-address.js';

/**
 * A secp256k1 private key (32 bytes).
 *
 * FR-003: Uses ECDSA secp256k1 curve.
 * FR-018: Guaranteed valid — construction rejects out-of-range values.
 * FR-022: Immutable after construction, except {@link zeroize}.
 *
 * SEC-003: Key material never appears in error messages or logs.
 */
export class NeuronPrivateKey {
  /** Internal 32-byte private key scalar. Mutable only by zeroize(). */
  private _bytes: Uint8Array;

  /** Tracks whether key material has been zeroed. FR-021 */
  private _zeroized: boolean;

  /** @internal — use factory methods instead. */
  private constructor(bytes: Uint8Array) {
    this._bytes = bytes;
    this._zeroized = false;
  }

  /**
   * Parse a private key from a hex string (with or without 0x/0X prefix).
   *
   * FR-002, FR-010: Accepts hex strings with or without 0x prefix.
   * FR-008: Validates hex characters with position info.
   * SEC-003: Error messages never contain key material.
   *
   * @param hex - 64-character hex string representing 32 bytes
   * @returns NeuronPrivateKey instance
   * @throws KeyError NEURON-KEY-004 if hex contains non-hex characters
   * @throws KeyError NEURON-KEY-003 if decoded length is not 32
   * @throws KeyError NEURON-KEY-006 if all-zero
   * @throws KeyError NEURON-KEY-005 if out of secp256k1 range
   */
  static fromHex(hex: string): NeuronPrivateKey {
    // Strip 0x/0X prefix
    let stripped = hex;
    if (stripped.startsWith('0x') || stripped.startsWith('0X')) {
      stripped = stripped.slice(2);
    }

    // FR-008: Validate hex characters with position info
    for (let i = 0; i < stripped.length; i++) {
      const c = stripped[i]!;
      if (!/[0-9a-fA-F]/.test(c)) {
        throw invalidHex(
          `Invalid hex character '${c}' at position ${i.toString()}`,
        );
      }
    }

    // Validate length: must be exactly 64 hex chars = 32 bytes
    if (stripped.length !== PRIVATE_KEY_LENGTH * 2) {
      throw invalidLength(
        PRIVATE_KEY_LENGTH,
        Math.floor(stripped.length / 2),
      );
    }

    // Convert to bytes using the wire hex utility
    const bytes = hexToBytes(stripped);
    return NeuronPrivateKey.fromBytes(bytes);
  }

  /**
   * Construct a NeuronPrivateKey from raw 32-byte key material.
   *
   * FR-002: Validates at construction time.
   * FR-A01: Validates 1 <= k < n (secp256k1 curve order).
   * FR-018: Only valid keys can be constructed.
   * FR-022: Defensive copy — callers cannot mutate internal state.
   * SEC-003: Error messages never reveal key material.
   *
   * @param bytes - Exactly 32 bytes of private key material
   * @returns NeuronPrivateKey instance
   * @throws KeyError NEURON-KEY-003 if length is not 32
   * @throws KeyError NEURON-KEY-006 if all-zero
   * @throws KeyError NEURON-KEY-005 if out of secp256k1 range (k >= n)
   */
  static fromBytes(bytes: Uint8Array): NeuronPrivateKey {
    // Validate length
    if (bytes.length !== PRIVATE_KEY_LENGTH) {
      throw invalidLength(PRIVATE_KEY_LENGTH, bytes.length);
    }

    // Defensive copy
    const keyBytes = new Uint8Array(bytes);

    // Check for all-zero key
    let allZero = true;
    for (let i = 0; i < keyBytes.length; i++) {
      if (keyBytes[i] !== 0) {
        allZero = false;
        break;
      }
    }
    if (allZero) {
      throw zeroValue();
    }

    // FR-A01: Interpret as big-endian unsigned integer, check 1 <= k < n
    const k = bytesToBigInt(keyBytes);
    if (k >= SECP256K1_ORDER) {
      throw invalidKey(
        'Private key scalar must be less than the secp256k1 curve order',
      );
    }

    return new NeuronPrivateKey(keyBytes);
  }

  /**
   * Generate a new cryptographically secure private key.
   *
   * FR-012: Uses cryptographically secure randomness.
   * FR-A01: The generated key satisfies 1 <= k < n.
   *
   * @returns NeuronPrivateKey instance with a fresh random key
   */
  static generate(): NeuronPrivateKey {
    const raw = secp.utils.randomPrivateKey();
    // noble/secp256k1 already ensures the key is in valid range [1, n-1]
    return new NeuronPrivateKey(raw);
  }

  /**
   * Restore a private key from a BIP-39 mnemonic phrase.
   *
   * FR-013: Supports BIP-39 mnemonic validation and BIP-44 HD derivation.
   * FR-A12: Mnemonic-to-seed via PBKDF2(HMAC-SHA512).
   * FR-A13: Default path m/44'/60'/0'/0/0 (Ethereum standard).
   *
   * @param mnemonic - BIP-39 mnemonic phrase (space-separated words)
   * @param path - BIP-44 derivation path (defaults to m/44'/60'/0'/0/0)
   * @returns NeuronPrivateKey instance derived from the mnemonic
   * @throws KeyError NEURON-KEY-010 if mnemonic is invalid
   * @throws KeyError NEURON-KEY-011 if HD key derivation fails
   */
  static fromMnemonic(mnemonic: string, path?: string): NeuronPrivateKey {
    // Validate mnemonic
    if (!bip39.validateMnemonic(mnemonic, wordlist)) {
      throw invalidMnemonic('Invalid BIP-39 mnemonic phrase');
    }

    try {
      // FR-A12: Mnemonic to seed (64 bytes)
      const seed = bip39.mnemonicToSeedSync(mnemonic);

      // FR-A13: HD key derivation
      const derivationPath = path ?? BIP44_DEFAULT_PATH;
      const hdkey = HDKey.fromMasterSeed(seed).derive(derivationPath);

      if (hdkey.privateKey == null) {
        throw derivationFailed(
          `HD key derivation produced no private key for path: ${derivationPath}`,
        );
      }

      return NeuronPrivateKey.fromBytes(hdkey.privateKey);
    } catch (e) {
      // Re-throw our own errors
      if (e instanceof KeyError) {
        throw e;
      }
      const cause = e instanceof Error ? e : new Error(String(e));
      throw derivationFailed(
        'BIP-44 HD key derivation failed',
        cause,
      );
    }
  }

  /**
   * Derive the corresponding public key.
   *
   * FR-001: NeuronPrivateKey derives NeuronPublicKey deterministically.
   * FR-A01: Q = k * G (secp256k1 scalar multiplication).
   *
   * @returns NeuronPublicKey — the corresponding public key
   * @throws KeyError NEURON-KEY-005 if the key has been zeroized
   */
  publicKey(): NeuronPublicKey {
    this.guardZeroized();
    const compressed = secp.getPublicKey(this._bytes, true);
    return NeuronPublicKey.fromCompressedBytes(compressed);
  }

  /**
   * Sign a message using ECDSA with Keccak256 pre-hashing.
   *
   * FR-014: Produces 65-byte R||S||V signature.
   * FR-017: Message is Keccak256-hashed before ECDSA signing.
   * FR-A07: Uses RFC 6979 deterministic nonce generation.
   * FR-A08: Pre-image is Keccak256(message).
   * FR-A10: Low-S normalization is applied; V is 0 or 1.
   *
   * @param message - Raw message bytes to sign
   * @returns Signature in R||S||V format (65 bytes)
   * @throws KeyError NEURON-KEY-013 if the key has been zeroized
   * @throws KeyError NEURON-KEY-013 if signing fails
   */
  sign(message: Uint8Array): Signature {
    this.guardZeroized();

    try {
      // FR-A08: Keccak256 pre-image
      const hash = keccak_256(message);
      return this.signRaw(hash);
    } catch (e) {
      // Re-throw our own errors
      if (e instanceof KeyError) {
        throw e;
      }
      const cause = e instanceof Error ? e : new Error(String(e));
      throw signingFailed('ECDSA signing failed', cause);
    }
  }

  /**
   * Sign a pre-hashed message (32-byte hash) using ECDSA.
   *
   * FR-014: Produces 65-byte R||S||V signature.
   * FR-A07: Uses RFC 6979 deterministic nonce generation.
   * FR-A10: Low-S normalization; V is 0 or 1.
   *
   * Use this when the caller has already computed the Keccak256 hash
   * (e.g., for TopicMessage pre-image signing per FR-A08).
   *
   * @param messageHash - 32-byte Keccak256 hash of the message
   * @returns Signature in R||S||V format (65 bytes)
   * @throws KeyError NEURON-KEY-013 if the key has been zeroized
   * @throws KeyError NEURON-KEY-013 if signing fails
   */
  signHash(messageHash: Uint8Array): Signature {
    this.guardZeroized();

    try {
      return this.signRaw(messageHash);
    } catch (e) {
      // Re-throw our own errors
      if (e instanceof KeyError) {
        throw e;
      }
      const cause = e instanceof Error ? e : new Error(String(e));
      throw signingFailed('ECDSA signing failed', cause);
    }
  }

  /**
   * Return a defensive copy of the 32-byte private key material.
   *
   * FR-022: Defensive copy — callers cannot mutate internal state.
   * SEC-003: Caller is responsible for zeroizing the returned bytes.
   *
   * @returns New Uint8Array containing the private key bytes
   * @throws KeyError NEURON-KEY-005 if the key has been zeroized
   */
  toBytes(): Uint8Array {
    this.guardZeroized();
    return new Uint8Array(this._bytes);
  }

  /**
   * Return the private key as a lowercase hex string with 0x prefix.
   *
   * FR-010: Hex serialization format.
   * SEC-003: Caller is responsible for handling the returned string securely.
   *
   * @returns Hex string (e.g., "0x...")
   * @throws KeyError NEURON-KEY-005 if the key has been zeroized
   */
  toHex(): string {
    this.guardZeroized();
    return bytesToHex(this._bytes);
  }

  /**
   * Zeroize private key material from memory.
   *
   * FR-021: Overwrites the internal byte array with zeros.
   * FR-022: This is the only mutation allowed after construction.
   *
   * After calling this method, all operations that require the private key
   * will throw. This operation is irreversible.
   */
  zeroize(): void {
    this._bytes.fill(0);
    this._zeroized = true;
  }

  /**
   * Whether this key has been zeroized.
   *
   * FR-021: Once zeroized, the key is permanently unusable.
   *
   * @returns `true` if {@link zeroize} has been called
   */
  get isZeroized(): boolean {
    return this._zeroized;
  }

  /**
   * Check whether this private key corresponds to the given public key.
   *
   * FR-016, SEC-004: Derives the public key and uses constant-time
   * comparison on the compressed bytes to prevent timing side-channel attacks.
   *
   * @param pubkey - NeuronPublicKey to compare against
   * @returns `true` if this private key derives to the given public key
   * @throws KeyError NEURON-KEY-005 if the key has been zeroized
   */
  matchesPublicKey(pubkey: NeuronPublicKey): boolean {
    this.guardZeroized();
    const derived = this.publicKey();
    return constantTimeEqual(
      derived.toCompressedBytes(),
      pubkey.toCompressedBytes(),
    );
  }

  /**
   * Check whether this private key corresponds to the given EVM address.
   *
   * FR-016, SEC-004: Derives the EVM address and uses constant-time
   * comparison to prevent timing side-channel attacks.
   *
   * @param address - EVMAddress to compare against
   * @returns `true` if this private key derives to the given address
   * @throws KeyError NEURON-KEY-005 if the key has been zeroized
   */
  matchesEvmAddress(address: EVMAddress): boolean {
    this.guardZeroized();
    const derived = this.publicKey().evmAddress();
    return constantTimeEqual(derived.toBytes(), address.toBytes());
  }

  // --- Private helpers ---

  /**
   * Guard against operations on a zeroized key.
   * @throws KeyError NEURON-KEY-005 / NEURON-KEY-013 depending on context
   */
  private guardZeroized(): void {
    if (this._zeroized) {
      throw invalidKey('Key has been zeroized');
    }
  }

  /**
   * Sign a raw 32-byte hash with ECDSA (RFC 6979, low-S).
   *
   * FR-A07: Deterministic nonce via RFC 6979.
   * FR-A10: R||S||V encoding with low-S normalization.
   *
   * @param hash - 32-byte hash to sign
   * @returns Signature in R||S||V format
   */
  private signRaw(hash: Uint8Array): Signature {
    // FR-A07: RFC 6979 deterministic signing with low-S normalization
    const sig = secp.sign(hash, this._bytes, { lowS: true });

    // FR-A10: Extract R (32 bytes), S (32 bytes), V (0 or 1)
    const compact = sig.toCompactRawBytes(); // 64 bytes: R || S
    const r = compact.slice(0, 32);
    const s = compact.slice(32, 64);
    const v = sig.recovery;

    return Signature.fromRSV(r, s, v);
  }
}

/**
 * Interpret a byte array as a big-endian unsigned integer.
 *
 * Used for secp256k1 scalar range validation (FR-A01).
 *
 * @param bytes - Byte array in big-endian order
 * @returns BigInt value
 */
function bytesToBigInt(bytes: Uint8Array): bigint {
  let result = 0n;
  for (let i = 0; i < bytes.length; i++) {
    result = (result << 8n) | BigInt(bytes[i]!);
  }
  return result;
}
