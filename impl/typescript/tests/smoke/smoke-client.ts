/// <reference lib="dom" />
// T001 smoke test — browser libp2p client.
// Fetches the fixture server's multiaddr from /smoke-addr.txt, dials it over
// ws://, opens a single /test/echo/1.0.0 stream, sends "ping", and expects
// to read back "ping" (echo handler mirrors inbound bytes).

import { createLibp2p } from 'libp2p'
import { webSockets } from '@libp2p/websockets'
import { noise } from '@chainsafe/libp2p-noise'
import { yamux } from '@chainsafe/libp2p-yamux'
import { identify } from '@libp2p/identify'
import { multiaddr } from '@multiformats/multiaddr'

const ECHO_PROTOCOL = '/test/echo/1.0.0'

function log(msg: string, kind: 'info' | 'ok' | 'err' = 'info'): void {
  const el = document.getElementById('log')
  if (!el) return
  const line = document.createElement('div')
  line.textContent = `[${kind}] ${msg}`
  line.style.color = kind === 'ok' ? '#2e7d32' : kind === 'err' ? '#c62828' : '#333'
  el.appendChild(line)
}

async function runSmoke(): Promise<void> {
  const button = document.getElementById('run') as HTMLButtonElement | null
  if (button) button.disabled = true
  try {
    log('fetching /smoke-addr.txt …')
    const resp = await fetch('/smoke-addr.txt', { cache: 'no-store' })
    if (!resp.ok) throw new Error(`smoke-addr.txt fetch status ${resp.status}`)
    const serverAddr = (await resp.text()).trim()
    log(`server multiaddr: ${serverAddr}`)

    log('creating libp2p browser node …')
    const node = await createLibp2p({
      transports: [webSockets()],
      connectionEncrypters: [noise()],
      streamMuxers: [yamux()],
      services: { identify: identify() },
      // The default browser gater denies loopback + insecure /ws dials.
      // Phase 1 spike runs on http://127.0.0.1:5174 → ws://127.0.0.1:8081 by
      // design (see research.md R0.3). Phase 2 H2 moves to wss:// + a public
      // host, at which point this override should be removed.
      connectionGater: { denyDialMultiaddr: async () => false },
    })
    await node.start()
    log(`browser peer id: ${node.peerId.toString()}`, 'ok')

    log('dialing …')
    const started = performance.now()
    const stream = await node.dialProtocol(multiaddr(serverAddr), ECHO_PROTOCOL)
    log(`dialed in ${Math.round(performance.now() - started)}ms; stream opened`, 'ok')

    const enc = new TextEncoder()
    const dec = new TextDecoder()
    const sent = `ping-${Date.now()}`
    log(`sending: ${sent}`)

    const roundStart = performance.now()
    const got = await new Promise<string>((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error('echo timed out after 5s')), 5000)
      stream.addEventListener('message', (evt) => {
        clearTimeout(timeout)
        const msg = evt.data
        // evt.data may be Uint8Array OR Uint8ArrayList — both expose `.subarray()`.
        const bytes = 'subarray' in msg ? msg.subarray() : msg
        resolve(dec.decode(bytes))
      }, { once: true })
      stream.addEventListener('close', () => {
        clearTimeout(timeout)
        reject(new Error('stream closed before echo'))
      }, { once: true })
      stream.send(enc.encode(sent))
    })
    const rtt = Math.round(performance.now() - roundStart)

    if (got === sent) {
      log(`HANDSHAKE OK — round-trip = ${rtt}ms — received "${got}"`, 'ok')
    } else {
      log(`echo mismatch: sent "${sent}", got "${got}"`, 'err')
    }

    await stream.close()
    await node.stop()
    log('browser node stopped', 'info')
  } catch (err) {
    const msg = err instanceof Error ? `${err.name}: ${err.message}` : String(err)
    log(`SMOKE FAILED — ${msg}`, 'err')
    throw err
  } finally {
    if (button) button.disabled = false
  }
}

document.getElementById('run')?.addEventListener('click', () => {
  void runSmoke()
})
