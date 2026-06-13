/// <reference lib="dom" />
// Spec 012 — DOM-based signature-chain ledger renderer.
// textContent only — no innerHTML anywhere in this module (FR-B29).

import type { LedgerEntry } from '../buyer-flow.js'

export function resetLedger(): void {
  const host = document.querySelector('#ledger ul')
  if (host) host.textContent = ''
}

export function appendLedgerEntry(entry: LedgerEntry): void {
  const host = document.querySelector('#ledger ul')
  if (!host) return
  const li = document.createElement('li')
  const shortAddr = entry.senderAddress.slice(0, 8) + '…' + entry.senderAddress.slice(-4)
  const shortHash = entry.payloadHashHex.slice(0, 10) + '…'
  const arrow = entry.direction === 'outbound' ? '→' : '←'
  const statusGlyph =
    entry.signatureStatus === 'verified' ? '✓'
    : entry.signatureStatus === 'self-signed' ? '·'
    : '✗'
  const color =
    entry.signatureStatus === 'verified' ? '#2e7d32'
    : entry.signatureStatus === 'failed' ? '#c62828'
    : '#555'
  li.style.color = color
  li.textContent = `${statusGlyph} ${arrow} ${entry.messageType.padEnd(18)} ${shortAddr}  payload-hash ${shortHash}`
  host.appendChild(li)
}
