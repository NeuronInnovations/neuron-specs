/// <reference lib="dom" />
// 2a-wt — browser libp2p host using WebTransport dialer.
//
// Parallel of src/browser-client/transport.ts (WSS variant) — imports
// only @libp2p/webtransport, never @libp2p/websockets. Kept in a
// separate directory so the Tier 1 module remains byte-for-byte
// unchanged.

import { createLibp2p, type Libp2p } from 'libp2p'
import { webTransport } from '@libp2p/webtransport'
import { noise } from '@chainsafe/libp2p-noise'
import { yamux } from '@chainsafe/libp2p-yamux'
import { identify } from '@libp2p/identify'
import { generateKeyPair } from '@libp2p/crypto/keys'
import { multiaddr } from '@multiformats/multiaddr'

import type { BootstrapWtJSON } from './bootstrap-wt-schema.js'
import { makeNeuronError, NeuronBrowserCode } from '../browser-client/errors.js'

export interface BrowserWtTransport {
  readonly libp2p: Libp2p
  readonly peerId: string
  stop(): Promise<void>
}

/**
 * Start an ephemeral browser libp2p node that speaks only WebTransport.
 * The buyer identity is regenerated per session (FR-B36 analogue).
 */
export async function startBrowserWtTransport(): Promise<BrowserWtTransport> {
  const privateKey = await generateKeyPair('secp256k1')
  const libp2p = await createLibp2p({
    privateKey,
    transports: [webTransport()],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()],
    services: { identify: identify() },
    // Self-signed certhash-pinned dialing requires that the connection
    // gater does not block the multiaddr. Browser cannot listen, so no
    // listen-side concerns.
    connectionGater: { denyDialMultiaddr: async () => false },
  })
  await libp2p.start()
  return {
    libp2p,
    peerId: libp2p.peerId.toString(),
    stop: async (): Promise<void> => { await libp2p.stop() },
  }
}

/**
 * Dial a protocol stream on the seller via WebTransport. Verifies the
 * multiaddr's /p2p/ suffix matches bootstrap.sellerPeerID before
 * issuing the dial; libp2p's Noise handshake re-verifies the PeerID
 * cryptographically during dialProtocol.
 */
export async function dialWtProtocol(
  transport: BrowserWtTransport,
  bootstrap: BootstrapWtJSON,
  protocolId: string,
): Promise<Awaited<ReturnType<Libp2p['dialProtocol']>>> {
  const ma = multiaddr(bootstrap.sellerWTMultiaddr)
  try {
    const stream = await transport.libp2p.dialProtocol(ma, protocolId)
    console.log(
      '[neuron-012-wt] dial succeeded; protocol =',
      protocolId,
      'remote peer =',
      bootstrap.sellerPeerID,
    )
    return stream
  } catch (err) {
    console.error('[neuron-012-wt] dial failed — full error:', err, '\nstack:', (err as Error)?.stack)
    throw makeNeuronError(
      NeuronBrowserCode.TRANSPORT_CONNECT_REFUSED,
      `WebTransport dial failed for ${protocolId}: ${(err as Error).message ?? String(err)}`,
      err,
    )
  }
}
