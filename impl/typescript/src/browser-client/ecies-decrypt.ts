// Spec 012 / Spec 009 — ECIES decrypt (browser side).
//
// Wire format (must match impl/golang/internal/delivery/ecies.go byte-for-byte):
//   raw = [33 bytes ephPub compressed] || [12 bytes nonce] || [ciphertext + 16-byte GCM tag]
//   wire = base64(raw)
//
// Protocol:
//   sharedX = ECDH(recipientPrivKey, ephPub).x     // 32 bytes, zero-padded
//   aesKey  = HKDF-SHA256(ikm = sharedX, salt = ∅, info = "neuron-multiaddr-v1", L = 32)
//   plain   = AES-256-GCM-decrypt(aesKey, nonce, ciphertext+tag)
//
// Traces: FR-B16 (spec 012), FR-D11–D14 (spec 009).

import { getSharedSecret } from '@noble/secp256k1'
import { hkdf } from '@noble/hashes/hkdf'
import { sha256 } from '@noble/hashes/sha256'
import { gcm } from '@noble/ciphers/aes.js'
import { ECIES_INFO } from './constants.js'

export class ECIESError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ECIESError'
  }
}

const EPH_PUB_LEN = 33
const NONCE_LEN = 12
const TAG_LEN = 16
const MIN_RAW_LEN = EPH_PUB_LEN + NONCE_LEN + TAG_LEN

/**
 * Decrypt an ECIES-encrypted payload produced by either the Go or the TS
 * encrypt-side. Returns raw plaintext bytes; interpretation (e.g., JSON parse
 * to `string[]` of multiaddrs) is the caller's responsibility — this function
 * is transport-shape-agnostic.
 */
export function eciesDecrypt(
  encryptedBase64: string,
  recipientPrivKey: Uint8Array,
): Uint8Array {
  if (recipientPrivKey.length !== 32) {
    throw new ECIESError(`recipient private key must be 32 bytes, got ${recipientPrivKey.length}`)
  }
  const raw = base64Decode(encryptedBase64)
  if (raw.length < MIN_RAW_LEN) {
    throw new ECIESError(`ciphertext too short: ${raw.length} < ${MIN_RAW_LEN}`)
  }
  const ephPub = raw.subarray(0, EPH_PUB_LEN)
  const nonce = raw.subarray(EPH_PUB_LEN, EPH_PUB_LEN + NONCE_LEN)
  const ciphertextWithTag = raw.subarray(EPH_PUB_LEN + NONCE_LEN)

  // noble's getSharedSecret returns the compressed shared point (33 bytes:
  // parity byte + 32-byte X). Go's ecies.go uses only the X coordinate
  // ("sharedX, _ = ...ScalarMult(...)"), so drop byte 0.
  const shared = getSharedSecret(recipientPrivKey, ephPub, /* isCompressed */ true)
  const sharedX = shared.subarray(1) // 32 bytes

  const aesKey = hkdf(sha256, sharedX, undefined, new TextEncoder().encode(ECIES_INFO), 32)

  const cipher = gcm(aesKey, nonce)
  try {
    return cipher.decrypt(ciphertextWithTag)
  } catch (err) {
    throw new ECIESError(
      `AES-GCM decryption failed: ${err instanceof Error ? err.message : String(err)}`,
    )
  }
}

function base64Decode(b64: string): Uint8Array {
  // Prefer atob() in browsers and Node ≥ 16.
  const binary = atob(b64)
  const out = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) out[i] = binary.charCodeAt(i)
  return out
}
