#!/usr/bin/env node
// Stage 1/2 Tier B smoke test for the 2a-wt WebTransport spike.
//
// Drives a headless Chromium against the Vite dev server and clicks the
// Buy button. Asserts that:
//   (a) the status line turns green with `✓ verified SHA-256:`
//   (b) the #image element contains an <img src="blob:…">
//
// Assumes:
//  - Go seller is running with --jpeg set and --bootstrap-out pointing at
//    examples/browser-demo-wt/public/bootstrap-wt.json
//  - Vite dev server is running on http://127.0.0.1:5174
//
// Exits 0 on success, 1 on any failure.

import { chromium } from 'playwright'

const VITE_URL = process.env.VITE_URL ?? 'http://127.0.0.1:5174/'
const WAIT_MS = Number(process.env.WAIT_MS ?? 30000)

const browser = await chromium.launch({ headless: true })
const page = await browser.newPage()

const logs = []
page.on('console', (msg) => {
  logs.push(`[console:${msg.type()}] ${msg.text()}`)
})
page.on('pageerror', (err) => {
  logs.push(`[pageerror] ${err.message}`)
})

console.log(`[smoke-tierb] loading ${VITE_URL}`)
await page.goto(VITE_URL, { waitUntil: 'domcontentloaded', timeout: 15000 })

await page.waitForSelector('#buy', { timeout: 5000 })
console.log('[smoke-tierb] clicking #buy')
const clickStart = Date.now()
await page.click('#buy')

let status = ''
const deadline = Date.now() + WAIT_MS
while (Date.now() < deadline) {
  const info = await page.$eval('#status', (el) => ({
    text: el.textContent ?? '',
  }))
  status = info.text
  if (status.startsWith('✓') || status.startsWith('✗')) break
  await new Promise((r) => setTimeout(r, 200))
}
const totalMs = Date.now() - clickStart

// Collect ledger entries + image presence.
const ledger = await page.$$eval('#ledger li', (lis) => lis.map((li) => li.textContent ?? ''))
const imageInfo = await page.$eval('#image', (el) => {
  const img = el.querySelector('img')
  return {
    hasImg: img !== null,
    srcPrefix: img ? (img.getAttribute('src') ?? '').slice(0, 40) : '',
  }
})

console.log('--- status ---')
console.log(status)
console.log('--- ledger ---')
for (const line of ledger) console.log(line)
console.log('--- image ---')
console.log(JSON.stringify(imageInfo))
console.log('--- console logs ---')
for (const line of logs) console.log(line)
console.log(`--- total ${totalMs}ms ---`)

await browser.close()

const statusOk = status.startsWith('✓ verified SHA-256:')
const imageOk = imageInfo.hasImg && imageInfo.srcPrefix.startsWith('blob:')
const ledgerOk = ledger.length >= 4 // 4 control messages minimum

if (!statusOk) console.error(`[smoke-tierb] status check FAILED: ${status}`)
if (!imageOk) console.error(`[smoke-tierb] image check FAILED: ${JSON.stringify(imageInfo)}`)
if (!ledgerOk) console.error(`[smoke-tierb] ledger check FAILED: ${ledger.length} entries`)

const ok = statusOk && imageOk && ledgerOk
console.log(ok ? '[smoke-tierb] PASS' : '[smoke-tierb] FAIL')
process.exit(ok ? 0 : 1)
