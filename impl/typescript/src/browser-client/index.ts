/// <reference lib="dom" />
// Spec 012 — browser demo entry. Wires the Buy button to a full session +
// transaction. Each click rotates the ephemeral session (FR-B36).
//
// Traces: FR-B26, FR-B27, FR-B28, FR-B32, FR-B33, FR-B36.

// Surface any module-load fault on the page itself so a reviewer without
// DevTools still sees the error. Runs immediately on import.
if (typeof document !== 'undefined') {
  console.log(
    '[neuron-012] browser-client module loaded; origin =',
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

import { loadBootstrap } from './bootstrap.js'
import { createBrowserSession, type BrowserSession } from './session.js'
import { startBrowserTransport, dialControlStream } from './transport.js'
import { runBuyerFlow } from './buyer-flow.js'
import { appendLedgerEntry, resetLedger } from './ui/ledger.js'
import { renderFailure, renderImage, renderVerified, resetStatus } from './ui/status.js'
import type { NeuronBrowserError } from './errors.js'

let activeSession: BrowserSession | null = null
let running = false

async function onBuy(): Promise<void> {
  if (running) {
    console.warn('Buy already in progress; ignoring click (FR-B28)')
    return
  }
  running = true
  const button = document.getElementById('buy') as HTMLButtonElement | null
  if (button) button.disabled = true

  // Rotate session per FR-B36.
  activeSession?.destroy()
  activeSession = createBrowserSession()

  resetLedger()
  resetStatus()
  const status = document.getElementById('status')
  if (status) status.textContent = `Session ${activeSession.identity.peerId.slice(0, 12)}… starting…`

  let transport: Awaited<ReturnType<typeof startBrowserTransport>> | null = null
  try {
    const bootstrap = await loadBootstrap(window.location.origin)
    transport = await startBrowserTransport()
    const { stream: controlStream } = await dialControlStream(transport, bootstrap)
    const file = await runBuyerFlow({
      session: activeSession,
      bootstrap,
      libp2p: transport.libp2p,
      controlStream,
      onLedger: appendLedgerEntry,
    })
    renderImage(file.bytes, file.metadata.contentType)
    renderVerified(file.metadata.sha256Hex, bootstrap.sellerEVMAddress)
  } catch (err) {
    renderFailure(err as NeuronBrowserError | Error)
    console.error('[buy] failed:', err)
  } finally {
    try { await transport?.stop() } catch { /* ignore */ }
    if (button) button.disabled = false
    running = false
  }
}

function install(): void {
  const button = document.getElementById('buy') as HTMLButtonElement | null
  const status = document.getElementById('status')
  if (!button) {
    console.error('No #buy button in page')
    if (status) {
      status.style.color = '#c62828'
      status.textContent = '✗ No #buy button found in DOM'
    }
    return
  }
  button.addEventListener('click', () => { void onBuy() })
  // Set a visible "wired" marker so we know the handler is attached.
  if (status) status.textContent = 'Ready. (Click Buy to start a session.)'
  button.disabled = false
  console.log('[neuron-012] Buy button wired')
  window.addEventListener('beforeunload', () => {
    activeSession?.destroy()
  })
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', install, { once: true })
} else {
  install()
}
