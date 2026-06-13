// Spec 012 / Spec 009 — ECIES encrypt (server side).
// Byte-for-byte compatible with impl/golang/internal/delivery/ecies.go.
//
// Produces: base64( [33-byte ephPub compressed] || [12-byte nonce] || [ct+tag] )
// Traces: FR-B16 (spec 012), FR-D11–D14 (spec 009).

import { getPublicKey, getSharedSecret, utils as secpUtils } from '@noble/secp256k1'
import { hkdf } from '@noble/hashes/hkdf'
import { sha256 } from '@noble/hashes/sha256'
import { gcm } from '@noble/ciphers/aes.js'
import { randomBytes } from 'node:crypto'
import { ECIES_INFO } from '../browser-client/constants.js'

const EPH_PUB_LEN = 33
const NONCE_LEN = 12

export interface EncryptOptions {
  /** Override ephemeral private key (fixture tests only). 32 bytes. */
  ephemeralPrivKey?: Uint8Array
  /** Override nonce (fixture tests only). 12 bytes. */
  nonce?: Uint8Array
}

/**
 * Encrypt plaintext bytes for a recipient's compressed secp256k1 public key.
 * The recipient decrypts with `eciesDecrypt(..., recipientPrivKey)`.
 */
export function eciesEncrypt(
  plaintext: Uint8Array,
  recipientPubKeyCompressed: Uint8Array,
  opts: EncryptOptions = {},
): string {
  if (recipientPubKeyCompressed.length !== 33) {
    throw new Error(`recipient pubkey must be 33 bytes (compressed), got ${recipientPubKeyCompressed.length}`)
  }

  const ephPriv = opts.ephemeralPrivKey ?? secpUtils.randomPrivateKey()
  if (ephPriv.length !== 32) {
    throw new Error(`ephemeral private key must be 32 bytes, got ${ephPriv.length}`)
  }
  const ephPub = getPublicKey(ephPriv, /* isCompressed */ true) // 33 bytes

  // Shared secret: take only the X coordinate (drop noble's parity byte).
  const shared = getSharedSecret(ephPriv, recipientPubKeyCompressed, true)
  const sharedX = shared.subarray(1) // 32 bytes

  const aesKey = hkdf(sha256, sharedX, undefined, new TextEncoder().encode(ECIES_INFO), 32)

  const nonce = opts.nonce ?? new Uint8Array(randomBytes(NONCE_LEN))
  if (nonce.length !== NONCE_LEN) {
    throw new Error(`nonce must be ${NONCE_LEN} bytes, got ${nonce.length}`)
  }

  const cipher = gcm(aesKey, nonce)
  const ciphertextWithTag = cipher.encrypt(plaintext)

  const raw = new Uint8Array(EPH_PUB_LEN + NONCE_LEN + ciphertextWithTag.length)
  raw.set(ephPub, 0)
  raw.set(nonce, EPH_PUB_LEN)
  raw.set(ciphertextWithTag, EPH_PUB_LEN + NONCE_LEN)
  return base64Encode(raw)
}

function base64Encode(bytes: Uint8Array): string {
  let binary = ''
  for (let i = 0; i < bytes.length; i++) {
    const byte = bytes[i]
    if (byte === undefined) break
    binary += String.fromCharCode(byte)
  }
  return btoa(binary)
}
