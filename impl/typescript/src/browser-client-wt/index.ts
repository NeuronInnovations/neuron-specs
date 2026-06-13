/// <reference lib="dom" />
// 2a-wt — Tier A (Ping) + Tier B (Buy) demo page wiring.
//
// Tier A Ping path:
//   fetch bootstrap-wt.json -> start WebTransport node -> dial echo protocol
//   -> perform ping/pong -> render RTT.
//
// Tier B Buy path:
//   fetch bootstrap-wt.json -> start WebTransport node -> dial control stream
//   -> runBuyerFlowWt (4-message flow + file receive + SHA-256) -> render
//   #status + #ledger + #image.
//
// Tier 1 UI helpers (status.ts, ledger.ts) are reused verbatim.

if (typeof document !== 'undefined') {
  console.log(
    '[neuron-012-wt] module loaded; origin =',
    window.location.origin,
    'href =',
    window.location.href,
  )
  window.addEventListener('error', (evt) => {
    const host = document.getElementById('status')
    if (host) {
      host.style.color = '#c62828'
      host.textContent = `✗ module error: ${evt.message}  (${evt.filename}:${evt.lineno})`
    }
  })
  window.addEventListener('unhandledrejection', (evt) => {
    const host = document.getElementById('status')
    if (host) {
      host.style.color = '#c62828'
      host.textContent = `✗ unhandled rejection: ${String((evt as PromiseRejectionEvent).reason)}`
    }
  })
}

import { loadBootstrapWt } from './bootstrap-wt.js'
import { startBrowserWtTransport, dialWtProtocol } from './transport.js'
import { performEcho, ECHO_PROTOCOL_ID } from './echo-client.js'
import { runBuyerFlowWt } from './buyer-flow-wt.js'
import { CONTROL_PROTOCOL_ID } from './constants.js'
import { createBrowserSession, type BrowserSession } from '../browser-client/session.js'
import { appendLedgerEntry, resetLedger } from '../browser-client/ui/ledger.js'
import { renderFailure, renderImage, renderVerified, resetStatus } from '../browser-client/ui/status.js'
import type { NeuronBrowserError } from '../browser-client/errors.js'

let running = false
let activeSession: BrowserSession | null = null

function setStatus(text: string, color: string): void {
  const el = document.getElementById('status')
  if (!el) return
  el.style.color = color
  el.textContent = text
}

function logTrace(line: string): void {
  const el = document.getElementById('ledger')
  if (!el) return
  let ul = el.querySelector('ul')
  if (!ul) {
    ul = document.createElement('ul')
    el.appendChild(ul)
  }
  const li = document.createElement('li')
  li.textContent = line
  ul.appendChild(li)
}

function resetTrace(): void {
  const ul = document.querySelector('#ledger ul')
  if (ul) ul.innerHTML = ''
}

function resetImage(): void {
  const el = document.getElementById('image')
  if (el) el.textContent = ''
}

async function onPing(): Promise<void> {
  if (running) return
  running = true
  const pingBtn = document.getElementById('ping') as HTMLButtonElement | null
  const buyBtn = document.getElementById('buy') as HTMLButtonElement | null
  if (pingBtn) pingBtn.disabled = true
  if (buyBtn) buyBtn.disabled = true

  resetTrace()
  resetImage()
  setStatus('Starting WebTransport browser node…', '#555')

  let transport: Awaited<ReturnType<typeof startBrowserWtTransport>> | null = null
  try {
    logTrace('1. fetch /bootstrap-wt.json')
    const bootstrap = await loadBootstrapWt(window.location.origin)
    logTrace(`   sellerPeerID = ${bootstrap.sellerPeerID}`)
    logTrace(`   sellerWTMultiaddr = ${bootstrap.sellerWTMultiaddr}`)

    logTrace('2. start libp2p browser node (WebTransport only)')
    transport = await startBrowserWtTransport()
    logTrace(`   buyerPeerID = ${transport.peerId}`)

    logTrace(`3. dial ${ECHO_PROTOCOL_ID}`)
    const stream = await dialWtProtocol(transport, bootstrap, ECHO_PROTOCOL_ID)

    logTrace('4. perform ping → pong')
    const result = await performEcho(stream, bootstrap.sellerPeerID)

    logTrace(`   RTT = ${result.rttMs} ms, payload = ${result.payload}`)
    setStatus(
      `✓ WebTransport direct dial OK — RTT ${result.rttMs}ms, peer ${result.remotePeerId}`,
      '#2e7d32',
    )
  } catch (err) {
    const nerr = err as NeuronBrowserError | Error
    const code = (nerr as NeuronBrowserError).code ?? 'ERROR'
    setStatus(`✗ ${code}: ${nerr.message}`, '#c62828')
    console.error('[ping] failed:', err)
  } finally {
    try { await transport?.stop() } catch { /* ignore */ }
    if (pingBtn) pingBtn.disabled = false
    if (buyBtn) buyBtn.disabled = false
    running = false
  }
}

async function onBuy(): Promise<void> {
  if (running) return
  running = true
  const pingBtn = document.getElementById('ping') as HTMLButtonElement | null
  const buyBtn = document.getElementById('buy') as HTMLButtonElement | null
  if (pingBtn) pingBtn.disabled = true
  if (buyBtn) buyBtn.disabled = true

  // Rotate session per FR-B36 parity.
  activeSession?.destroy()
  activeSession = createBrowserSession()

  resetLedger()
  resetStatus()
  resetImage()
  const status = document.getElementById('status')
  if (status) status.textContent = `Session ${activeSession.identity.peerId.slice(0, 12)}… starting…`

  let transport: Awaited<ReturnType<typeof startBrowserWtTransport>> | null = null
  try {
    const bootstrap = await loadBootstrapWt(window.location.origin)
    transport = await startBrowserWtTransport()
    const controlStream = await dialWtProtocol(transport, bootstrap, CONTROL_PROTOCOL_ID)

    const file = await runBuyerFlowWt({
      session: activeSession,
      bootstrap,
      libp2p: transport.libp2p,
      controlStream,
      onLedger: appendLedgerEntry,
    })

    renderImage(file.bytes, file.metadata.contentType)
    renderVerified(file.metadata.sha256Hex, bootstrap.sellerEVMAddress)

    // runBuyerFlow fire-and-forgets the invoiceAck — it returns as soon as
    // sendEnvelope queues the frame into the libp2p send buffer, without
    // waiting for the seller to acknowledge. If we immediately tear down the
    // transport, the QUIC stream closes before the invoiceAck frame flushes
    // and the seller never observes the ack (so the mock escrow never
    // transitions to `released`). Closing the control stream explicitly is
    // libp2p's way to flush pending writes before the stream goes away.
    try {
      await (controlStream as unknown as { close(): Promise<void> }).close()
    } catch { /* already closed is fine */ }
    // Extra belt-and-suspenders: a short wait so the seller's async reader
    // has time to pull the final frame off the wire before libp2p.stop().
    await new Promise((resolve) => setTimeout(resolve, 500))
  } catch (err) {
    renderFailure(err as NeuronBrowserError | Error)
    console.error('[buy-wt] failed:', err)
  } finally {
    try { await transport?.stop() } catch { /* ignore */ }
    if (pingBtn) pingBtn.disabled = false
    if (buyBtn) buyBtn.disabled = false
    running = false
  }
}

function install(): void {
  const pingBtn = document.getElementById('ping') as HTMLButtonElement | null
  const buyBtn = document.getElementById('buy') as HTMLButtonElement | null
  const status = document.getElementById('status')

  if (!pingBtn) {
    console.error('[neuron-012-wt] no #ping button in DOM')
  } else {
    pingBtn.addEventListener('click', () => { void onPing() })
    pingBtn.disabled = false
  }

  if (!buyBtn) {
    console.error('[neuron-012-wt] no #buy button in DOM')
  } else {
    buyBtn.addEventListener('click', () => { void onBuy() })
    buyBtn.disabled = false
  }

  if (status && pingBtn && buyBtn) {
    status.textContent = 'Ready. (Click Ping for Tier A echo, or Buy for the full 4-message flow.)'
  }

  window.addEventListener('beforeunload', () => {
    activeSession?.destroy()
  })

  console.log('[neuron-012-wt] Ping + Buy buttons wired')
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', install, { once: true })
} else {
  install()
}
