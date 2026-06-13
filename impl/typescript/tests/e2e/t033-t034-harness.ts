// One-off Playwright harness that completes T033 (storage audit) + T034
// (per-Buy identity rotation) autonomously against a running `pnpm run demo`.
//
// This script is the evidence source for the manual steps in quickstart.md.
// It is NOT part of the canonical Phase 2 H3/H4 test suite — those remain
// pending. Use: `pnpm tsx tests/e2e/t033-t034-harness.ts` after `pnpm run demo`
// is healthy at http://127.0.0.1:5173/.

import { chromium } from 'playwright'

const ORIGIN = 'http://127.0.0.1:5173'

interface BuyResult {
  n: number
  statusLine: string
  buyerAddressFromLedger: string | null
  ledgerRowsForThisBuy: string[]
}

interface StorageDump {
  localStorage: Record<string, string>
  sessionStorage: Record<string, string>
  cookies: string
  indexedDb: string[]
}

async function main(): Promise<void> {
  const browser = await chromium.launch({ headless: true })
  const ctx = await browser.newContext()
  const page = await ctx.newPage()

  // Capture all browser-side logs so we can debug if anything fails.
  page.on('console', (msg) => {
    console.log(`[browser] ${msg.type()}: ${msg.text()}`)
  })
  page.on('pageerror', (err) => {
    console.error(`[browser-error] ${err.name}: ${err.message}`)
  })

  console.log(`[harness] goto ${ORIGIN}`)
  await page.goto(ORIGIN, { waitUntil: 'domcontentloaded' })

  // Wait until the module has loaded and install() has wired the Buy button.
  await page.waitForFunction(
    () => {
      const status = document.getElementById('status')
      return !!status && status.textContent?.includes('Ready.')
    },
    { timeout: 10_000 },
  )
  console.log('[harness] page is Ready')

  // T033 part 1 — storage audit BEFORE any Buy.
  const readStorage = async (): Promise<StorageDump> => {
    return await page.evaluate(async () => {
      const dbs = (await (indexedDB.databases?.() ?? Promise.resolve([]))) as Array<{ name?: string }>
      const ls: Record<string, string> = {}
      for (let i = 0; i < localStorage.length; i++) {
        const k = localStorage.key(i)
        if (k !== null) ls[k] = localStorage.getItem(k) ?? ''
      }
      const ss: Record<string, string> = {}
      for (let i = 0; i < sessionStorage.length; i++) {
        const k = sessionStorage.key(i)
        if (k !== null) ss[k] = sessionStorage.getItem(k) ?? ''
      }
      return {
        localStorage: ls,
        sessionStorage: ss,
        cookies: document.cookie,
        indexedDb: dbs.map((db) => db.name ?? '(unnamed)'),
      }
    })
  }

  const storageBefore = await readStorage()
  console.log('[harness] storage BEFORE first Buy:', JSON.stringify(storageBefore))

  const results: BuyResult[] = []

  for (let n = 1; n <= 3; n++) {
    console.log(`[harness] --- Buy #${n} ---`)
    await page.click('#buy')
    // Wait for either the verified status or a failure.
    // Note: browser-client/index.ts calls resetLedger()+resetStatus() on every
    // Buy click, so after verification the ledger holds exactly this Buy's
    // 4 envelope rows (serviceRequest / paymentDetails / connectionSetup /
    // invoiceAck). Earlier transactions' rows are intentionally gone.
    await page.waitForFunction(
      () => {
        const status = document.getElementById('status')
        const txt = status?.textContent ?? ''
        return txt.includes('verified SHA-256') || txt.startsWith('✗')
      },
      { timeout: 30_000 },
    )
    const statusLine = (await page.locator('#status').textContent()) ?? ''
    const rows = await page.locator('#ledger li').allTextContents()
    // Extract the buyer address from the first outbound row (serviceRequest).
    // Row text format: "· → serviceRequest       0xABCD…EFGH  payload-hash …"
    const serviceRequestRow = rows.find((r) => r.includes('serviceRequest'))
    const match = serviceRequestRow?.match(/0x[0-9a-fA-F]{4,6}…[0-9a-fA-F]{4,6}/)
    results.push({
      n,
      statusLine,
      buyerAddressFromLedger: match ? match[0] : null,
      ledgerRowsForThisBuy: rows,
    })
    console.log(`[harness] Buy #${n} status: ${statusLine}`)
    console.log(`[harness] Buy #${n} buyer addr: ${match ? match[0] : '(not found)'}`)
  }

  const storageAfter = await readStorage()
  console.log('[harness] storage AFTER all 3 Buys:', JSON.stringify(storageAfter))

  // Evaluate T033 pass/fail.
  const t033Pass =
    Object.keys(storageBefore.localStorage).length === 0 &&
    Object.keys(storageBefore.sessionStorage).length === 0 &&
    storageBefore.indexedDb.length === 0 &&
    storageBefore.cookies === '' &&
    Object.keys(storageAfter.localStorage).length === 0 &&
    Object.keys(storageAfter.sessionStorage).length === 0 &&
    storageAfter.indexedDb.length === 0 &&
    storageAfter.cookies === ''

  // Evaluate T034 pass/fail.
  const buyerAddrs = results.map((r) => r.buyerAddressFromLedger).filter((a): a is string => a !== null)
  const distinctAddrs = new Set(buyerAddrs)
  const t034Pass = buyerAddrs.length === 3 && distinctAddrs.size === 3

  console.log('')
  console.log('=== T033 RESULT ===')
  console.log(t033Pass ? 'PASS' : 'FAIL')
  console.log('storageBefore:', JSON.stringify(storageBefore))
  console.log('storageAfter:', JSON.stringify(storageAfter))

  console.log('')
  console.log('=== T034 RESULT ===')
  console.log(t034Pass ? 'PASS' : 'FAIL')
  for (const r of results) {
    console.log(`  Buy #${r.n}: ${r.buyerAddressFromLedger}  status="${r.statusLine.slice(0, 80)}…"`)
  }

  console.log('')
  console.log('=== OVERALL ===')
  console.log(`T033=${t033Pass ? 'PASS' : 'FAIL'}  T034=${t034Pass ? 'PASS' : 'FAIL'}`)

  await browser.close()
  process.exit(t033Pass && t034Pass ? 0 : 1)
}

main().catch((err) => {
  console.error('[harness] fatal:', err)
  process.exit(2)
})
