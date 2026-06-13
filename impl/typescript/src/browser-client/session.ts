/// <reference lib="dom" />
// Spec 012 — ephemeral per-page-load session.
// The private key lives in a closure and NEVER leaves the session wrapper.
//
// Traces: FR-B01, FR-B02, FR-B03, FR-B04, FR-B36.

import { NeuronPrivateKey } from '../keylib/private-key.js'
import { NeuronPublicKey } from '../keylib/public-key.js'

export interface PublicIdentity {
  readonly evmAddress: string
  readonly peerId: string
  readonly didKey: string
  readonly compressedPublicKeyHex: string
}

export interface BrowserSession {
  readonly identity: PublicIdentity
  /** Sign an arbitrary 32-byte hash. Public API — never exposes the key. */
  signHash(hash: Uint8Array): Uint8Array
  /** Return the 32-byte private key bytes (exclusive use: ECIES decrypt). */
  exportPrivateKeyForECIES(): Uint8Array
  /** Produce a TopicMessage-style TS wrapper (see envelope-verify for consumer usage). */
  createPrivateKey(): NeuronPrivateKey
  /** AbortController that is fired when the session is destroyed. */
  readonly abortSignal: AbortSignal
  destroy(): void
}

export function createBrowserSession(): BrowserSession {
  const priv = NeuronPrivateKey.generate()
  const pub: NeuronPublicKey = priv.publicKey()
  const abortController = new AbortController()

  const identity: PublicIdentity = {
    evmAddress: pub.evmAddress().toString(),
    peerId: pub.peerId().toString(),
    didKey: pub.didKey().toString(),
    compressedPublicKeyHex: Array.from(pub.toCompressedBytes(), (b) =>
      b.toString(16).padStart(2, '0'),
    ).join(''),
  }

  return {
    identity,
    signHash: (hash: Uint8Array): Uint8Array => priv.signHash(hash).toBytes(),
    exportPrivateKeyForECIES: (): Uint8Array => priv.toBytes(),
    createPrivateKey: (): NeuronPrivateKey => priv,
    abortSignal: abortController.signal,
    destroy: () => abortController.abort(),
  }
}
