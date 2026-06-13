// Spec 012 — profile-local error taxonomy.
//
// Traces: FR-B31 (failure-category surfacing), FR-B35 (profile-local IDs in v1).
// Phase 2 H5 maps these to Spec 006's NEURON-{DOMAIN}-{NNN} — pre-merge obligation.
//
// Numbering ranges per research.md R0.9:
//   001–019  Configuration    (bootstrap JSON, unsupported browser)
//   020–039  Transport        (TLS, WS dial, port blocked, stream close)
//   040–059  Handshake        (Noise, PeerID mismatch)
//   060–079  Signature        (envelope verification)
//   080–099  Hash mismatch    (data integrity, SHA-256 comparison)
//   100–119  Size cap / framing
//   120–139  Timeout
//   140–159  State-machine protocol fault

export type NeuronErrorCategory =
  | 'configuration'
  | 'transport'
  | 'handshake'
  | 'signature'
  | 'hash-mismatch'
  | 'size-cap'
  | 'timeout'
  | 'state-machine'

/**
 * Canonical profile-local error codes. Add new entries in the appropriate
 * numeric range. Every entry MUST be referenced by at least one throw site
 * or test so unused codes are visible.
 */
export const NeuronBrowserCode = {
  // Configuration 001–019
  BOOTSTRAP_FETCH_FAILED: 'NEURON-BROWSER-001',
  BOOTSTRAP_VERSION_MISMATCH: 'NEURON-BROWSER-002',
  BOOTSTRAP_UNKNOWN_KEY: 'NEURON-BROWSER-003',
  BOOTSTRAP_MISSING_FIELD: 'NEURON-BROWSER-004',
  BOOTSTRAP_BAD_EVM_ADDRESS: 'NEURON-BROWSER-005',
  BOOTSTRAP_BAD_PEER_ID: 'NEURON-BROWSER-006',
  BOOTSTRAP_BAD_MULTIADDR_SCHEME: 'NEURON-BROWSER-007',
  BOOTSTRAP_WS_ON_NON_LOCALHOST: 'NEURON-BROWSER-008',
  BOOTSTRAP_TYPE_MISMATCH: 'NEURON-BROWSER-009',
  BOOTSTRAP_WRONG_CONTENT_TYPE: 'NEURON-BROWSER-010',
  BOOTSTRAP_PARSE_FAILURE: 'NEURON-BROWSER-011',
  UNSUPPORTED_BROWSER: 'NEURON-BROWSER-012',

  // Transport 020–039
  TRANSPORT_CONNECT_REFUSED: 'NEURON-BROWSER-020',
  TRANSPORT_STREAM_CLOSED_UNEXPECTEDLY: 'NEURON-BROWSER-021',
  TRANSPORT_TLS_FAILURE: 'NEURON-BROWSER-022',

  // Handshake 040–059
  HANDSHAKE_NOISE_FAILED: 'NEURON-BROWSER-041',
  HANDSHAKE_YAMUX_FAILED: 'NEURON-BROWSER-042',
  HANDSHAKE_PEER_ID_MISMATCH: 'NEURON-BROWSER-043',

  // Signature 060–079
  ENVELOPE_MALFORMED: 'NEURON-BROWSER-060',
  SIGNATURE_RECOVER_MISMATCH: 'NEURON-BROWSER-061',
  SENDER_PIN_MISMATCH: 'NEURON-BROWSER-062',
  SEQUENCE_DECREMENT: 'NEURON-BROWSER-063',
  TIMESTAMP_SKEW: 'NEURON-BROWSER-064',

  // Hash mismatch 080–099
  FILE_SHA256_MISMATCH: 'NEURON-BROWSER-082',

  // Size cap / framing 100–119
  FRAME_SIZE_EXCEEDED: 'NEURON-BROWSER-101',
  CHUNK_TOTAL_MISMATCH: 'NEURON-BROWSER-102',
  METADATA_UNKNOWN_FIELD: 'NEURON-BROWSER-103',

  // Timeout 120–139
  READ_IDLE_TIMEOUT: 'NEURON-BROWSER-121',

  // State-machine 140–159
  UNEXPECTED_ENVELOPE_TYPE: 'NEURON-BROWSER-140',
  CONCURRENT_BUY_REJECTED: 'NEURON-BROWSER-141',
} as const

export type NeuronBrowserCode = typeof NeuronBrowserCode[keyof typeof NeuronBrowserCode]

const CATEGORY_BY_CODE_RANGE: Array<{ max: number; category: NeuronErrorCategory }> = [
  { max: 19, category: 'configuration' },
  { max: 39, category: 'transport' },
  { max: 59, category: 'handshake' },
  { max: 79, category: 'signature' },
  { max: 99, category: 'hash-mismatch' },
  { max: 119, category: 'size-cap' },
  { max: 139, category: 'timeout' },
  { max: 159, category: 'state-machine' },
]

function categoryOf(code: string): NeuronErrorCategory {
  const num = Number(code.split('-')[2] ?? NaN)
  if (!Number.isFinite(num)) return 'configuration'
  for (const row of CATEGORY_BY_CODE_RANGE) {
    if (num <= row.max) return row.category
  }
  return 'configuration'
}

export class NeuronBrowserError extends Error {
  readonly code: string
  readonly category: NeuronErrorCategory
  override readonly cause?: unknown

  constructor(code: string, message: string, cause?: unknown) {
    super(`${code}: ${message}`)
    this.name = 'NeuronBrowserError'
    this.code = code
    this.category = categoryOf(code)
    if (cause !== undefined) this.cause = cause
  }
}

/**
 * Factory: produce a structured error the browser UI can render.
 * Example: `throw makeNeuronError(NeuronBrowserCode.SENDER_PIN_MISMATCH, 'expected 0x..., got 0x...')`.
 */
export function makeNeuronError(
  code: string,
  message: string,
  cause?: unknown,
): NeuronBrowserError {
  return new NeuronBrowserError(code, message, cause)
}
