// Spec 012 — orchestrator. `pnpm run demo` entry.
// 1. Spawn the Node seller (src/server-demo/index.ts)
// 2. Wait for it to print "[seller] ready"
// 3. Spawn Vite dev server for examples/browser-demo/
// 4. Propagate Ctrl-C to both children.
//
// Traces: plan.md Manual test approach, quickstart.md.

import { spawn, type ChildProcess } from 'node:child_process'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = dirname(fileURLToPath(import.meta.url))
const TS_ROOT = resolve(HERE, '..')

function spawnChild(name: string, cmd: string, args: string[]): ChildProcess {
  const child = spawn(cmd, args, {
    cwd: TS_ROOT,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: process.env,
  })
  child.stdout?.on('data', (chunk: Buffer) => {
    process.stdout.write(`[${name}] ${chunk.toString()}`)
  })
  child.stderr?.on('data', (chunk: Buffer) => {
    process.stderr.write(`[${name}] ${chunk.toString()}`)
  })
  child.on('exit', (code, signal) => {
    if (signal) console.log(`[${name}] exited on ${signal}`)
    else console.log(`[${name}] exited with code ${code ?? 'null'}`)
  })
  return child
}

function waitForReady(child: ChildProcess, marker: string, timeoutMs = 10_000): Promise<void> {
  return new Promise((resolveP, rejectP) => {
    const t = setTimeout(() => rejectP(new Error(`timed out waiting for "${marker}"`)), timeoutMs)
    const onChunk = (chunk: Buffer): void => {
      if (chunk.toString().includes(marker)) {
        clearTimeout(t)
        child.stdout?.off('data', onChunk)
        resolveP()
      }
    }
    child.stdout?.on('data', onChunk)
  })
}

async function main(): Promise<void> {
  const seller = spawnChild('seller', 'pnpm', ['run', '-s', 'demo:server'])
  await waitForReady(seller, '[seller] ready')

  // VPS deployment shape: seller-only (no local Vite). Operator drives Vite
  // from a separate machine or terminal. Keeps the seller's stdout streaming.
  if (process.env.SELLER_ONLY === '1' || process.env.SELLER_ONLY === 'true') {
    console.log('[orchestrator] SELLER_ONLY set — skipping Vite; bootstrap.json is on the seller\'s disk')
    const shutdown = (sig: NodeJS.Signals): void => {
      console.log(`\n[orchestrator] received ${sig}; stopping seller`)
      seller.kill(sig)
      setTimeout(() => process.exit(0), 500)
    }
    process.on('SIGINT', () => shutdown('SIGINT'))
    process.on('SIGTERM', () => shutdown('SIGTERM'))
    // Keep the orchestrator alive so it can propagate SIGINT to the child.
    return
  }

  // VITE_HOST is honoured by examples/browser-demo/vite.config.ts; pass
  // through so `VITE_HOST=0.0.0.0 pnpm run demo` binds externally.
  const viteHost = process.env.VITE_HOST ?? '127.0.0.1'
  const vite = spawnChild('vite', 'pnpm', ['run', '-s', 'demo:browser', '--', `--host=${viteHost}`])

  const shutdown = (sig: NodeJS.Signals): void => {
    console.log(`\n[orchestrator] received ${sig}; stopping children`)
    seller.kill(sig)
    vite.kill(sig)
    setTimeout(() => process.exit(0), 500)
  }
  process.on('SIGINT', () => shutdown('SIGINT'))
  process.on('SIGTERM', () => shutdown('SIGTERM'))

  const pageUrl = viteHost === '0.0.0.0'
    ? '(bind on all interfaces — open http://<your-public-ip>:5173/ or http://localhost:5173/)'
    : `http://${viteHost}:5173/`
  console.log(`[orchestrator] both children up — open ${pageUrl} in a browser`)
}

main().catch((err) => {
  console.error('[orchestrator] fatal:', err)
  process.exit(1)
})
