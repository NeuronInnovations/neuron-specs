// 2a-wt orchestrator — optionally pulls bootstrap from VPS via scp,
// then spawns Vite on port 5174 serving the WebTransport demo page.
//
// Usage modes:
//   pnpm run demo:wt                     # laptop + loopback (Go seller runs locally on :4443)
//   pnpm run demo:wt -- --fetch          # scp bootstrap from VPS, then browser-only
//   WT_VITE_PORT=5174 pnpm run demo:wt   # override Vite port
//
// The seller binary itself is Go (`cmd/webtransport-seller`). This
// orchestrator does NOT spawn it — the operator starts it either
// locally for Stage 1 or on the VPS for Stage 2.

import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'

const cliArgs = process.argv.slice(2)
const shouldFetch = cliArgs.includes('--fetch')

async function run(): Promise<void> {
  if (shouldFetch) {
    console.log('[run-demo-wt] fetching bootstrap-wt.json from VPS…')
    await runOnce('bash', ['scripts/fetch-wt-bootstrap.sh'])
  } else {
    const localBootstrap = 'examples/browser-demo-wt/public/bootstrap-wt.json'
    if (!existsSync(localBootstrap)) {
      console.warn(
        `[run-demo-wt] no ${localBootstrap} found. Either:\n` +
          '  • Start the Go seller locally with --bootstrap-out ' +
          `pointing at ${localBootstrap}, or\n` +
          '  • Run `pnpm run demo:wt -- --fetch` to scp from the VPS.',
      )
    }
  }

  console.log('[run-demo-wt] starting Vite on port', process.env.WT_VITE_PORT ?? '5174')
  const viteHost = process.env.VITE_HOST ?? '127.0.0.1'
  const vite = spawn(
    'pnpm',
    ['run', '-s', 'demo:wt:browser', '--', `--host=${viteHost}`],
    { stdio: 'inherit' },
  )

  const shutdown = (sig: NodeJS.Signals): void => {
    vite.kill(sig)
  }
  process.on('SIGINT', shutdown)
  process.on('SIGTERM', shutdown)

  vite.on('exit', (code) => { process.exit(code ?? 0) })
}

function runOnce(cmd: string, args: readonly string[]): Promise<void> {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, [...args], { stdio: 'inherit' })
    child.on('exit', (code) => {
      if (code === 0) resolve()
      else reject(new Error(`${cmd} ${args.join(' ')} exited ${code}`))
    })
    child.on('error', reject)
  })
}

run().catch((err) => {
  console.error('[run-demo-wt] fatal:', err)
  process.exit(1)
})
