/// <reference lib="dom" />
// 2a-wt Tier B — thin shim that reuses the Tier 1 transport-agnostic
// runBuyerFlow against a WebTransport-dialled control stream.
//
// Tier 1's runBuyerFlow is already transport-agnostic (it takes a pre-dialed
// libp2p Stream + libp2p instance + session + onLedger callback). This shim
// just adapts the WebTransport bootstrap shape (BootstrapWtJSON) into the
// Tier 1 BootstrapJSON type the Tier 1 module expects.
//
// The seller's 2a-wt runbook (and our Go server) populates the decrypted
// multiaddr list with the seller's own WT multiaddr, so the fallback path in
// buyer-flow.ts:106 (`addrs[0] ?? bootstrap.sellerWSSMultiaddr`) does NOT
// trigger — the `sellerWSSMultiaddr` value below is type-satisfaction only.
//
// Imports are intentionally 1:1 with src/browser-client/index.ts so any
// future change to the Buy flow in Tier 1 is inherited automatically.
//
// Traces: FR-B14, FR-B15, FR-B17 (reused verbatim).

import type { Libp2p } from 'libp2p'
import type { Stream } from '@libp2p/interface'
import { runBuyerFlow, type LedgerEntry } from '../browser-client/buyer-flow.js'
import type { BrowserSession } from '../browser-client/session.js'
import type { ReceivedFile } from '../browser-client/file-receive.js'
import type { BootstrapJSON } from '../browser-client/bootstrap-schema.js'
import type { BootstrapWtJSON } from './bootstrap-wt-schema.js'

/**
 * Execute the Tier 1 4-message Buy flow over a WebTransport-dialled control
 * stream. Returns the received file once SHA-256 verification passes.
 */
export async function runBuyerFlowWt(opts: {
  readonly session: BrowserSession
  readonly bootstrap: BootstrapWtJSON
  readonly libp2p: Libp2p
  readonly controlStream: Stream
  readonly onLedger: (entry: LedgerEntry) => void
}): Promise<ReceivedFile> {
  const tier1Bootstrap: BootstrapJSON = Object.freeze({
    version: 1,
    sellerEVMAddress: opts.bootstrap.sellerEVMAddress,
    sellerPeerID: opts.bootstrap.sellerPeerID,
    // NOTE: Tier 1's BootstrapJSON schema uses `sellerWSSMultiaddr`. For the
    // WT path there IS no WSS multiaddr; we supply the WT multiaddr so the
    // TS type narrows, and rely on the buyer-flow using `addrs[0]` (the
    // decrypted list from connectionSetup) rather than this fallback.
    sellerWSSMultiaddr: opts.bootstrap.sellerWTMultiaddr,
    controlStreamProtocolID: opts.bootstrap.controlStreamProtocolID,
    dataStreamProtocolID: opts.bootstrap.dataStreamProtocolID,
  })

  return runBuyerFlow({
    session: opts.session,
    bootstrap: tier1Bootstrap,
    libp2p: opts.libp2p,
    controlStream: opts.controlStream,
    onLedger: opts.onLedger,
  })
}
