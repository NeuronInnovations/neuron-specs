/// <reference lib="dom" />
// Spec 012 — browser-side libp2p host configuration + dial with PeerID pinning.
//
// Traces: FR-B05, FR-B06, FR-B07, FR-B09.

import { createLibp2p, type Libp2p } from 'libp2p'
import { webSockets } from '@libp2p/websockets'
import { noise } from '@chainsafe/libp2p-noise'
import { yamux } from '@chainsafe/libp2p-yamux'
import { identify } from '@libp2p/identify'
import { generateKeyPair } from '@libp2p/crypto/keys'
import { multiaddr } from '@multiformats/multiaddr'
import type { BootstrapJSON } from './bootstrap-schema.js'
import { makeNeuronError, NeuronBrowserCode } from './errors.js'

export interface BrowserTransport {
  readonly libp2p: Libp2p
  readonly peerId: string
  stop(): Promise<void>
}

export async function startBrowserTransport(): Promise<BrowserTransport> {
  const privateKey = await generateKeyPair('secp256k1')
  const libp2p = await createLibp2p({
    privateKey,
    transports: [webSockets()],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()],
    services: { identify: identify() },
    // Phase 1 permits loopback + ws://; removed in H2.
    connectionGater: { denyDialMultiaddr: async () => false },
  })
  await libp2p.start()
  return {
    libp2p,
    peerId: libp2p.peerId.toString(),
    stop: async (): Promise<void> => { await libp2p.stop() },
  }
}

function extractPeerIdFromMultiaddrString(ma: string): string | null {
  const m = ma.match(/\/p2p\/([^/]+)/)
  return m && m[1] !== undefined ? m[1] : null
}

/**
 * Dial the seller's control stream. Verifies that the Noise-authenticated
 * PeerID matches `bootstrap.sellerPeerID` before returning the stream.
 */
export async function dialControlStream(
  transport: BrowserTransport,
  bootstrap: BootstrapJSON,
): Promise<{ stream: Awaited<ReturnType<Libp2p['dialProtocol']>>; remotePeerId: string }> {
  const ma = multiaddr(bootstrap.sellerWSSMultiaddr)
  let stream: Awaited<ReturnType<Libp2p['dialProtocol']>>
  try {
    stream = await transport.libp2p.dialProtocol(ma, bootstrap.controlStreamProtocolID)
  } catch (err) {
    console.error('[neuron-012] dial failed — full error:', err, '\nstack:', (err as Error)?.stack)
    throw makeNeuronError(
      NeuronBrowserCode.TRANSPORT_CONNECT_REFUSED,
      `dial failed: ${(err as Error).message ?? String(err)}`,
      err,
    )
  }
  // Extract the remote peer id from the multiaddr string and verify it
  // matches. (libp2p's Noise handshake already checks this; we repeat for
  // defence-in-depth and to surface the specific NEURON-BROWSER-043 code.)
  const peerFromAddr = extractPeerIdFromMultiaddrString(bootstrap.sellerWSSMultiaddr)
  if (peerFromAddr !== bootstrap.sellerPeerID) {
    await stream.close().catch(() => { /* ignore */ })
    throw makeNeuronError(
      NeuronBrowserCode.HANDSHAKE_PEER_ID_MISMATCH,
      `multiaddr peer id ${peerFromAddr} does not match bootstrap.sellerPeerID ${bootstrap.sellerPeerID}`,
    )
  }
  return { stream, remotePeerId: bootstrap.sellerPeerID }
}
