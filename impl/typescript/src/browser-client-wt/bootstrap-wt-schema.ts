// 2a-wt — bootstrap validator for the WebTransport variant.
//
// Parallel of src/browser-client/bootstrap-schema.ts for Tier 1. Kept
// intentionally separate so the Tier 1 schema stays unchanged and the
// two profiles cannot cross-consume bootstraps.

import { EVMAddress } from '../keylib/evm-address.js'
import { makeNeuronError, NeuronBrowserCode } from '../browser-client/errors.js'
import {
  BOOTSTRAP_VERSION_WT,
  CONTROL_PROTOCOL_ID,
  DATA_PROTOCOL_ID,
  ECHO_PROTOCOL_ID,
} from './constants.js'

export interface BootstrapWtJSON {
  readonly version: typeof BOOTSTRAP_VERSION_WT
  readonly sellerEVMAddress: string
  readonly sellerPeerID: string
  readonly sellerWTMultiaddr: string
  readonly controlStreamProtocolID: typeof CONTROL_PROTOCOL_ID
  readonly dataStreamProtocolID: typeof DATA_PROTOCOL_ID
  readonly echoProtocolID: typeof ECHO_PROTOCOL_ID
}

const REQUIRED_KEYS = [
  'version',
  'sellerEVMAddress',
  'sellerPeerID',
  'sellerWTMultiaddr',
  'controlStreamProtocolID',
  'dataStreamProtocolID',
  'echoProtocolID',
] as const

// base58btc PeerID pattern — same shape as Tier 1. go-libp2p secp256k1
// emits `16Uiu2H...`, js-libp2p Ed25519 emits `12D3KooW...`.
const PEER_ID_BASE58_RE = /^[1-9A-HJ-NP-Za-km-z]{32,128}$/

/**
 * Validate a 2a-wt bootstrap document. Throws NeuronBrowserError on any
 * violation, returns a frozen BootstrapWtJSON on success.
 *
 * Rules (stricter than Tier 1):
 *  - version MUST be the string "2a-wt"
 *  - multiaddr MUST contain "/webtransport/" AND at least one "/certhash/"
 *    component — this is the whole point of the profile
 *  - all three protocol IDs must match the WT constants exactly
 */
export function validateBootstrapWt(doc: unknown): BootstrapWtJSON {
  if (doc === null || typeof doc !== 'object' || Array.isArray(doc)) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_PARSE_FAILURE,
      'bootstrap must be a JSON object',
    )
  }
  const obj = doc as Record<string, unknown>

  for (const key of Object.keys(obj)) {
    if (!(REQUIRED_KEYS as readonly string[]).includes(key)) {
      throw makeNeuronError(
        NeuronBrowserCode.BOOTSTRAP_UNKNOWN_KEY,
        `unknown bootstrap field: ${key}`,
      )
    }
  }
  for (const key of REQUIRED_KEYS) {
    if (!(key in obj)) {
      throw makeNeuronError(
        NeuronBrowserCode.BOOTSTRAP_MISSING_FIELD,
        `bootstrap missing required field: ${key}`,
      )
    }
  }

  if (typeof obj.version !== 'string') {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
      `bootstrap.version must be a string, got ${typeof obj.version}`,
    )
  }
  if (obj.version !== BOOTSTRAP_VERSION_WT) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_VERSION_MISMATCH,
      `bootstrap.version must be "${BOOTSTRAP_VERSION_WT}", got ${String(obj.version)}`,
    )
  }

  for (const key of [
    'sellerEVMAddress',
    'sellerPeerID',
    'sellerWTMultiaddr',
    'controlStreamProtocolID',
    'dataStreamProtocolID',
    'echoProtocolID',
  ] as const) {
    if (typeof obj[key] !== 'string') {
      throw makeNeuronError(
        NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
        `bootstrap.${key} must be a string, got ${typeof obj[key]}`,
      )
    }
  }

  try {
    EVMAddress.fromHex(obj.sellerEVMAddress as string)
  } catch (err) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_BAD_EVM_ADDRESS,
      `bootstrap.sellerEVMAddress fails EIP-55 checksum: ${obj.sellerEVMAddress as string}`,
      err,
    )
  }

  if (!PEER_ID_BASE58_RE.test(obj.sellerPeerID as string)) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_BAD_PEER_ID,
      `bootstrap.sellerPeerID is not a valid base58btc PeerID: ${obj.sellerPeerID as string}`,
    )
  }

  const ma = obj.sellerWTMultiaddr as string
  if (!ma.includes('/webtransport')) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_BAD_MULTIADDR_SCHEME,
      `bootstrap.sellerWTMultiaddr must contain /webtransport; got ${ma}`,
    )
  }
  if (!ma.includes('/certhash/')) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_BAD_MULTIADDR_SCHEME,
      `bootstrap.sellerWTMultiaddr must contain at least one /certhash/ component; got ${ma}`,
    )
  }
  // /quic-v1 transport is required for WebTransport per libp2p spec.
  if (!ma.includes('/quic-v1')) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_BAD_MULTIADDR_SCHEME,
      `bootstrap.sellerWTMultiaddr must contain /quic-v1; got ${ma}`,
    )
  }
  // The /p2p/<PEERID> suffix must match sellerPeerID exactly.
  const p2pMatch = ma.match(/\/p2p\/([^/]+)$/)
  const p2pSuffix = p2pMatch && p2pMatch[1] !== undefined ? p2pMatch[1] : null
  if (p2pSuffix !== obj.sellerPeerID) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_BAD_PEER_ID,
      `bootstrap.sellerWTMultiaddr /p2p/ suffix ${String(p2pSuffix)} must match sellerPeerID ${obj.sellerPeerID as string}`,
    )
  }

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
  if (obj.echoProtocolID !== ECHO_PROTOCOL_ID) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
      `bootstrap.echoProtocolID must be ${ECHO_PROTOCOL_ID}, got ${String(obj.echoProtocolID)}`,
    )
  }

  return Object.freeze({
    version: BOOTSTRAP_VERSION_WT,
    sellerEVMAddress: obj.sellerEVMAddress as string,
    sellerPeerID: obj.sellerPeerID as string,
    sellerWTMultiaddr: obj.sellerWTMultiaddr as string,
    controlStreamProtocolID: CONTROL_PROTOCOL_ID,
    dataStreamProtocolID: DATA_PROTOCOL_ID,
    echoProtocolID: ECHO_PROTOCOL_ID,
  })
}
