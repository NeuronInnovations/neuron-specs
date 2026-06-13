// Spec 012 — frame-0 metadata parser for the data-plane stream.
//
// Wire format per contracts/stream-protocols.md §"Data stream frame 0":
//   Canonical-JSON object with required fields:
//     filename      — string, ≤ 255 bytes, no path separators
//     sizeBytes     — integer, ≥ 1, ≤ MAX_TOTAL_BYTES (1 MiB in v1)
//     contentType   — allowlist: "image/jpeg" | "image/png"
//     sha256Hex     — 64-char lowercase hex
//
// Unknown top-level keys → NEURON-BROWSER-103 (FR-B21 strict).
//
// Traces: FR-B19, FR-B21, contracts/stream-protocols.md.

import { MAX_TOTAL_BYTES } from './constants.js'
import { makeNeuronError, NeuronBrowserCode } from './errors.js'

export interface FileMetadata {
  readonly filename: string
  readonly sizeBytes: number
  readonly contentType: string
  readonly sha256Hex: string
}

export const ALLOWED_CONTENT_TYPES = ['image/jpeg', 'image/png'] as const
const REQUIRED_KEYS = new Set(['filename', 'sizeBytes', 'contentType', 'sha256Hex'])
const SHA256_HEX_RE = /^[0-9a-f]{64}$/

/**
 * Parse the frame-0 payload bytes into a validated FileMetadata object.
 * Throws NeuronBrowserError with the appropriate profile-local code on any
 * rule violation; never returns partial data.
 */
export function parseFrameZero(bytes: Uint8Array): FileMetadata {
  let doc: unknown
  try {
    doc = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes))
  } catch (err) {
    throw makeNeuronError(
      NeuronBrowserCode.ENVELOPE_MALFORMED,
      'frame 0 is not valid UTF-8 JSON',
      err,
    )
  }
  if (doc === null || typeof doc !== 'object' || Array.isArray(doc)) {
    throw makeNeuronError(
      NeuronBrowserCode.ENVELOPE_MALFORMED,
      'frame 0 must be a JSON object',
    )
  }
  const obj = doc as Record<string, unknown>

  for (const key of Object.keys(obj)) {
    if (!REQUIRED_KEYS.has(key)) {
      throw makeNeuronError(
        NeuronBrowserCode.METADATA_UNKNOWN_FIELD,
        `unknown frame-0 field: ${key}`,
      )
    }
  }
  for (const key of REQUIRED_KEYS) {
    if (!(key in obj)) {
      throw makeNeuronError(
        NeuronBrowserCode.BOOTSTRAP_MISSING_FIELD,
        `frame 0 missing required field: ${key}`,
      )
    }
  }

  const { filename, sizeBytes, contentType, sha256Hex } = obj
  if (typeof filename !== 'string' || filename.length === 0 || filename.length > 255) {
    throw makeNeuronError(
      NeuronBrowserCode.ENVELOPE_MALFORMED,
      `filename must be a non-empty string ≤ 255 bytes`,
    )
  }
  if (filename.includes('/') || filename.includes('\\')) {
    throw makeNeuronError(
      NeuronBrowserCode.ENVELOPE_MALFORMED,
      `filename must not contain path separators`,
    )
  }
  if (typeof sizeBytes !== 'number' || !Number.isInteger(sizeBytes) || sizeBytes < 1) {
    throw makeNeuronError(
      NeuronBrowserCode.ENVELOPE_MALFORMED,
      `sizeBytes must be a positive integer`,
    )
  }
  if (sizeBytes > MAX_TOTAL_BYTES) {
    throw makeNeuronError(
      NeuronBrowserCode.FRAME_SIZE_EXCEEDED,
      `sizeBytes ${sizeBytes} exceeds v1 cap ${MAX_TOTAL_BYTES}`,
    )
  }
  if (typeof contentType !== 'string' || !(ALLOWED_CONTENT_TYPES as readonly string[]).includes(contentType)) {
    throw makeNeuronError(
      NeuronBrowserCode.ENVELOPE_MALFORMED,
      `contentType must be one of ${ALLOWED_CONTENT_TYPES.join(', ')}; got ${String(contentType)}`,
    )
  }
  if (typeof sha256Hex !== 'string' || !SHA256_HEX_RE.test(sha256Hex)) {
    throw makeNeuronError(
      NeuronBrowserCode.ENVELOPE_MALFORMED,
      `sha256Hex must be 64 lowercase hex characters`,
    )
  }

  return Object.freeze({ filename, sizeBytes, contentType, sha256Hex })
}
