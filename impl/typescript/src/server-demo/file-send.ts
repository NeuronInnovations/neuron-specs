// Spec 012 — server-side file delivery.
// Writes a frame-0 metadata envelope followed by the file bytes chunked into
// frames of ≤ MAX_FRAME_BYTES. Uses the shared length-prefix framing.
//
// Traces: FR-B19.

import { readFileSync } from 'node:fs'
import { basename } from 'node:path'
import { sha256 } from '@noble/hashes/sha256'
import { encodeFrame } from '../wire/frame.js'
import { MAX_FRAME_BYTES } from '../browser-client/constants.js'
import type { FileMetadata } from '../browser-client/file-metadata.js'

export interface LoadedAsset {
  readonly metadata: FileMetadata
  readonly bytes: Uint8Array
}

export function loadAsset(path: string, contentType: 'image/jpeg' | 'image/png'): LoadedAsset {
  const bytes = new Uint8Array(readFileSync(path))
  const hash = sha256(bytes)
  const sha256Hex = Array.from(hash, (b) => b.toString(16).padStart(2, '0')).join('')
  // TAMPER HOOK (US2 — T031). When TAMPER=hash is set, flip one byte of the
  // payload AFTER the SHA-256 metadata has been declared. The declared hash
  // stays, so the browser reassembles the payload, computes a different hash,
  // and aborts with NEURON-BROWSER-082.
  if (process.env.TAMPER === 'hash' && bytes.length > 0) {
    const idx = Math.floor(bytes.length / 2)
    bytes[idx] = (bytes[idx] ?? 0) ^ 0x01
    console.warn(
      `[file-send] TAMPER=hash: flipped payload byte at index ${idx}; declared sha256 left intact`,
    )
  }
  return {
    metadata: {
      filename: basename(path),
      sizeBytes: bytes.length,
      contentType,
      sha256Hex,
    },
    bytes,
  }
}

/** Send frame 0 (metadata) + frames 1..N (chunks up to MAX_FRAME_BYTES). */
export function sendAsset(
  asset: LoadedAsset,
  stream: { send: (b: Uint8Array) => boolean },
): void {
  const metaJson = new TextEncoder().encode(JSON.stringify(asset.metadata))
  stream.send(encodeFrame(metaJson))
  for (let offset = 0; offset < asset.bytes.length; offset += MAX_FRAME_BYTES) {
    const chunk = asset.bytes.subarray(offset, Math.min(offset + MAX_FRAME_BYTES, asset.bytes.length))
    stream.send(encodeFrame(chunk))
  }
}
