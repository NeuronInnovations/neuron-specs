/// <reference lib="dom" />
// Spec 012 — status renderers for verified/failure states.
// textContent only — no innerHTML.
// Traces: FR-B30, FR-B31.

import type { NeuronBrowserError } from '../errors.js'

export function resetStatus(): void {
  const host = document.getElementById('status')
  if (host) {
    host.textContent = 'Running…'
    host.style.color = '#333'
  }
  const imgHost = document.getElementById('image')
  if (imgHost) imgHost.textContent = ''
}

export function renderVerified(sha256Hex: string, sellerAddress: string): void {
  const host = document.getElementById('status')
  if (!host) return
  host.style.color = '#2e7d32'
  host.textContent = `✓ verified SHA-256: ${sha256Hex}   seller: ${sellerAddress}`
}

export function renderFailure(err: NeuronBrowserError | Error): void {
  const host = document.getElementById('status')
  if (!host) return
  host.style.color = '#c62828'
  const code = (err as NeuronBrowserError).code ?? 'NEURON-BROWSER-UNKNOWN'
  const category = (err as NeuronBrowserError).category ?? 'unknown'
  host.textContent = `✗ ${code} [${category}]  ${err.message}`
}

export function renderImage(bytes: Uint8Array, contentType: string): void {
  const host = document.getElementById('image')
  if (!host) return
  host.textContent = ''
  const blob = new Blob([bytes as unknown as BlobPart], { type: contentType })
  const url = URL.createObjectURL(blob)
  const img = document.createElement('img')
  img.alt = 'Received Neuron asset'
  img.src = url
  host.appendChild(img)
}
