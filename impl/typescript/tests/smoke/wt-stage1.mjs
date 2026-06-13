#!/usr/bin/env node
// Stage 1 smoke test for the 2a-wt WebTransport spike.
//
// Assumes:
//  - Go seller is running on 127.0.0.1:4443 with bootstrap-out pointing at
//    examples/browser-demo-wt/public/bootstrap-wt.json
//  - Vite dev server is running on http://127.0.0.1:5174
//
// Drives a headless Chromium via Playwright to navigate, click Ping,
// and captures the resulting status + ledger. Exits 0 if status turns
// green within 15s, 1 otherwise.

import { chromium } from 'playwright'

const VITE_URL = process.env.VITE_URL ?? 'http://127.0.0.1:5174/'
const WAIT_MS = Number(process.env.WAIT_MS ?? 15000)

const browser = await chromium.launch({
  headless: true,
  // Chromium's WebTransport requires trustedCertsDomainString for certhash on
  // non-HTTPS origins; but for loopback it works out-of-the-box. No extra
  // flags needed for 127.0.0.1.
})
const page = await browser.newPage()

const logs = []
page.on('console', (msg) => {
  logs.push(`[console:${msg.type()}] ${msg.text()}`)
})
page.on('pageerror', (err) => {
  logs.push(`[pageerror] ${err.message}`)
})

console.log(`[smoke] loading ${VITE_URL}`)
await page.goto(VITE_URL, { waitUntil: 'domcontentloaded', timeout: 15000 })

await page.waitForSelector('#ping', { timeout: 5000 })
console.log('[smoke] clicking #ping')
await page.click('#ping')

let status = ''
let color = ''
const deadline = Date.now() + WAIT_MS
while (Date.now() < deadline) {
  const info = await page.$eval('#status', (el) => ({
    text: el.textContent ?? '',
    color: el.style.color,
  }))
  status = info.text
  color = info.color
  if (status.startsWith('✓') || status.startsWith('✗')) break
  await new Promise((r) => setTimeout(r, 200))
}

const ledger = await page.$$eval('#ledger li', (lis) => lis.map((li) => li.textContent ?? ''))

console.log('--- status ---')
console.log(status)
console.log('--- ledger ---')
for (const line of ledger) console.log(line)
console.log('--- console logs ---')
for (const line of logs) console.log(line)

await browser.close()

const ok = status.startsWith('✓')
console.log(ok ? '[smoke] PASS' : '[smoke] FAIL')
process.exit(ok ? 0 : 1)
