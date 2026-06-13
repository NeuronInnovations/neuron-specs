// Spec 012 — bootstrap JSON validator.
//
// Pure synchronous validator; the network-fetch wrapper lives in bootstrap.ts.
// Rules enforced per contracts/bootstrap-json.md; errors use the
// NEURON-BROWSER-NNN taxonomy from errors.ts.
//
// Traces: FR-B23, FR-B24, FR-B25.

import {
  BOOTSTRAP_VERSION,
  CONTROL_PROTOCOL_ID,
  DATA_PROTOCOL_ID,
} from './constants.js'
import { makeNeuronError, NeuronBrowserCode } from './errors.js'
import { EVMAddress } from '../keylib/evm-address.js'

export interface BootstrapJSON {
  readonly version: 1
  readonly sellerEVMAddress: string
  readonly sellerPeerID: string
  readonly sellerWSSMultiaddr: string
  readonly controlStreamProtocolID: string
  readonly dataStreamProtocolID: string
}

const REQUIRED_KEYS = [
  'version',
  'sellerEVMAddress',
  'sellerPeerID',
  'sellerWSSMultiaddr',
  'controlStreamProtocolID',
  'dataStreamProtocolID',
] as const

// libp2p PeerIDs are base58btc multihashes. Their leading characters vary by
// key type (Ed25519 → 12D3KooW…, secp256k1 → 16Uiu2H…, RSA → Qm… etc). v1
// accepts any syntactically valid base58btc PeerID of plausible length; the
// real cryptographic check is the Noise handshake's identity verification.
const PEER_ID_BASE58_RE = /^[1-9A-HJ-NP-Za-km-z]{32,128}$/

/**
 * Validate a bootstrap document. `pageOrigin` is the browser's current origin
 * (e.g., `"http://127.0.0.1:5174"`); used to enforce the `/ws`-on-localhost
 * rule per contracts/bootstrap-json.md §"Scheme-specific validation".
 *
 * Returns a frozen BootstrapJSON on success; throws NeuronBrowserError on any
 * violation.
 */
export function validateBootstrap(doc: unknown, pageOrigin: string): BootstrapJSON {
  if (doc === null || typeof doc !== 'object' || Array.isArray(doc)) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_PARSE_FAILURE,
      'bootstrap must be a JSON object',
    )
  }
  const obj = doc as Record<string, unknown>
  // Reject unknown top-level keys (strict).
  for (const key of Object.keys(obj)) {
    if (!(REQUIRED_KEYS as readonly string[]).includes(key)) {
      throw makeNeuronError(
        NeuronBrowserCode.BOOTSTRAP_UNKNOWN_KEY,
        `unknown bootstrap field: ${key}`,
      )
    }
  }
  // Required presence.
  for (const key of REQUIRED_KEYS) {
    if (!(key in obj)) {
      throw makeNeuronError(
        NeuronBrowserCode.BOOTSTRAP_MISSING_FIELD,
        `bootstrap missing required field: ${key}`,
      )
    }
  }
  // version
  if (typeof obj.version !== 'number') {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
      `bootstrap.version must be a number, got ${typeof obj.version}`,
    )
  }
  if (obj.version !== BOOTSTRAP_VERSION) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_VERSION_MISMATCH,
      `bootstrap.version must be ${BOOTSTRAP_VERSION}, got ${obj.version}`,
    )
  }
  // String-typed fields
  for (const key of [
    'sellerEVMAddress',
    'sellerPeerID',
    'sellerWSSMultiaddr',
    'controlStreamProtocolID',
    'dataStreamProtocolID',
  ] as const) {
    if (typeof obj[key] !== 'string') {
      throw makeNeuronError(
        NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
        `bootstrap.${key} must be a string, got ${typeof obj[key]}`,
      )
    }
  }
  // EIP-55 checksum
  try {
    EVMAddress.fromHex(obj.sellerEVMAddress as string)
  } catch (err) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_BAD_EVM_ADDRESS,
      `bootstrap.sellerEVMAddress fails EIP-55 checksum: ${obj.sellerEVMAddress as string}`,
      err,
    )
  }
  // PeerID syntactic check (full libp2p decode is deferred — v1 accepts `12D3KooW…` base58btc patterns)
  if (!PEER_ID_BASE58_RE.test(obj.sellerPeerID as string)) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_BAD_PEER_ID,
      `bootstrap.sellerPeerID is not a valid base58btc PeerID: ${obj.sellerPeerID as string}`,
    )
  }
  // Multiaddr scheme check (syntactic)
  const ma = obj.sellerWSSMultiaddr as string
  const isWs = ma.includes('/ws/') || ma.endsWith('/ws')
  const isWss = ma.includes('/wss/') || ma.endsWith('/wss') || ma.includes('/tls/ws')
  if (!isWs && !isWss) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_BAD_MULTIADDR_SCHEME,
      `bootstrap.sellerWSSMultiaddr must use /ws or /wss; got ${ma}`,
    )
  }
  // ws:// allowed only when one of:
  //   (a) page origin is http://localhost or http://127.0.0.1 (the Phase 1 local-dev case)
  //   (b) page origin host exactly matches the multiaddr's host — i.e. page
  //       and seller are served from the same machine (the VPS-demo case).
  //       Keeps the cross-origin-insecure-ws attack rejected (an HTTPS or
  //       other-origin page cannot dial ws:// at a victim host), while
  //       allowing the single legitimate same-host deploy shape.
  // Both paths are replaced by /wss in Phase 2 H2.
  if (isWs && !isWss) {
    const allowed = isLocalhostOrigin(pageOrigin) || isSameHostOrigin(pageOrigin, ma)
    if (!allowed) {
      throw makeNeuronError(
        NeuronBrowserCode.BOOTSTRAP_WS_ON_NON_LOCALHOST,
        `insecure /ws multiaddr not permitted from origin ${pageOrigin}; use /wss, SSH-tunnel to localhost, or serve the page from the same host as ${ma}`,
      )
    }
  }
  // Stream protocol IDs — v1 expects exact match
  if (obj.controlStreamProtocolID !== CONTROL_PROTOCOL_ID) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
      `bootstrap.controlStreamProtocolID must be ${CONTROL_PROTOCOL_ID}, got ${String(obj.controlStreamProtocolID)}`,
    )
  }
  if (obj.dataStreamProtocolID !== DATA_PROTOCOL_ID) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
      `bootstrap.dataStreamProtocolID must be ${DATA_PROTOCOL_ID}, got ${String(obj.dataStreamProtocolID)}`,
    )
  }

  return Object.freeze({
    version: BOOTSTRAP_VERSION,
    sellerEVMAddress: obj.sellerEVMAddress as string,
    sellerPeerID: obj.sellerPeerID as string,
    sellerWSSMultiaddr: obj.sellerWSSMultiaddr as string,
    controlStreamProtocolID: obj.controlStreamProtocolID,
    dataStreamProtocolID: obj.dataStreamProtocolID,
  })
}

function isLocalhostOrigin(origin: string): boolean {
  // http://localhost[:PORT] or http://127.0.0.1[:PORT]
  try {
    const u = new URL(origin)
    if (u.protocol !== 'http:') return false
    return u.hostname === 'localhost' || u.hostname === '127.0.0.1'
  } catch {
    return false
  }
}

/**
 * True iff the multiaddr's host (the token following `/ip4/`, `/ip6/`, or
 * `/dns4/`/`/dns6/`/`/dns/`) matches the page origin's hostname. Used to
 * permit `ws://` when the page and the seller are served from the same host
 * — the VPS-demo shape where Vite on 203.0.113.42 serves a page that dials
 * `ws://203.0.113.42:8080`. Cross-origin insecure-ws remains rejected.
 */
function isSameHostOrigin(origin: string, multiaddr: string): boolean {
  try {
    const pageHost = new URL(origin).hostname
    if (!pageHost) return false
    const maHost = extractMultiaddrHost(multiaddr)
    if (maHost === null) return false
    return maHost === pageHost
  } catch {
    return false
  }
}

function extractMultiaddrHost(multiaddr: string): string | null {
  // Matches the first /ip4/, /ip6/, /dns4/, /dns6/, or /dns/ tuple.
  const m = multiaddr.match(/^\/(?:ip4|ip6|dns4|dns6|dns)\/([^/]+)/)
  return m && m[1] !== undefined ? m[1] : null
}
