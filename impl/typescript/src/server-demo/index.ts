// Spec 012 — Node.js seller main entry.
// Orchestrates: libp2p host start → stream handlers → bootstrap JSON write.
//
// Invoked by `pnpm run demo:server` or by scripts/run-demo.ts.
//
// Traces: FR-B08 (JS side), FR-B23 (bootstrap writer), FR-B18 (mock escrow).

import { writeFileSync, mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  BOOTSTRAP_VERSION,
  CONTROL_PROTOCOL_ID,
  DATA_PROTOCOL_ID,
} from '../browser-client/constants.js'
import { startSellerHost } from './transport.js'
import { loadAsset } from './file-send.js'
import { MockEscrow } from './mock-escrow.js'
import { handleControlStream } from './seller-flow.js'

const HERE = dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = resolve(HERE, '..', '..', '..', '..') // impl/typescript/src/server-demo/ → repo root
const BOOTSTRAP_PATH = resolve(
  REPO_ROOT,
  'impl/typescript/examples/browser-demo/public/bootstrap.json',
)
const ASSET_PATH = resolve(HERE, 'assets/demo.jpg')

async function main(): Promise<void> {
  const host = await startSellerHost()
  const asset = loadAsset(ASSET_PATH, 'image/jpeg')
  const escrow = new MockEscrow()

  // Register data-stream handler FIRST so when the seller-flow opens it, the
  // buyer is already ready to accept. Actually, the data stream is opened by
  // the SELLER to the BUYER (reverse direction), so the BROWSER registers the
  // data handler and seller dials. For the spike we keep it simple: after the
  // buyer connects via control-stream, the seller uses that same libp2p
  // connection to open a NEW stream on DATA_PROTOCOL_ID back to the buyer.

  host.libp2p.handle(CONTROL_PROTOCOL_ID, (stream, connection) => {
    const remotePeer = connection.remotePeer
    handleControlStream(stream, {
      sellerPrivateKey: host.privateKey,
      sellerAddress: host.evmAddress,
      asset,
      escrow,
      ownWSSMultiaddr: host.wssMultiaddr,
      openDataStream: async () => {
        const dataStream = await host.libp2p.dialProtocol(remotePeer, DATA_PROTOCOL_ID)
        return {
          send: (b: Uint8Array): boolean => dataStream.send(b),
          close: (): Promise<void> => dataStream.close(),
        }
      },
    })
  })

  // Write the bootstrap JSON so the browser page can fetch it same-origin.
  mkdirSync(dirname(BOOTSTRAP_PATH), { recursive: true })
  const bootstrap = {
    version: BOOTSTRAP_VERSION,
    sellerEVMAddress: host.evmAddress,
    sellerPeerID: host.peerId,
    sellerWSSMultiaddr: host.wssMultiaddr,
    controlStreamProtocolID: CONTROL_PROTOCOL_ID,
    dataStreamProtocolID: DATA_PROTOCOL_ID,
  }
  writeFileSync(BOOTSTRAP_PATH, JSON.stringify(bootstrap, null, 2) + '\n', 'utf8')

  console.log(`[seller] peer id: ${host.peerId}`)
  console.log(`[seller] EVM address: ${host.evmAddress}`)
  console.log(`[seller] listening: ${host.wssMultiaddr}`)
  console.log(`[seller] wrote bootstrap: ${BOOTSTRAP_PATH}`)
  console.log(`[seller] asset: ${asset.metadata.filename} (${asset.metadata.sizeBytes} bytes, sha256 ${asset.metadata.sha256Hex.slice(0, 12)}…)`)
  console.log(`[seller] escrow: in-memory mock (FR-B18)`)
  console.log(`[seller] ready — press ctrl-c to stop`)

  const shutdown = async (): Promise<void> => {
    console.log('[seller] shutting down')
    await host.libp2p.stop()
    process.exit(0)
  }
  process.on('SIGINT', shutdown)
  process.on('SIGTERM', shutdown)
}

main().catch((err) => {
  console.error('[seller] fatal:', err)
  process.exit(1)
})
