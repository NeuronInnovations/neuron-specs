// T001 smoke test — Node.js libp2p fixture server.
// No Neuron-level code. Only proves that js-libp2p on Node speaks ws:// +
// Noise XX + yamux + identify well enough to round-trip a single echo stream
// to a browser. Once this passes, the full 012 stack can be built on top.

// Polyfill Promise.withResolvers for Node.js < 21.12 / < 20.12.
// libp2p@3 uses it internally in its upgrader during the Noise handshake.
// Without this, the handshake throws EncryptionFailedError and the remote
// peer sees "EOF while reading 0/1 bytes". Safe no-op on modern Node/browsers.
type WithResolversShim = {
  withResolvers?: <T>() => { promise: Promise<T>; resolve: (v: T | PromiseLike<T>) => void; reject: (e?: unknown) => void }
}
if (typeof (Promise as unknown as WithResolversShim).withResolvers !== 'function') {
  ;(Promise as unknown as WithResolversShim).withResolvers = function <T>() {
    let resolve!: (v: T | PromiseLike<T>) => void
    let reject!: (e?: unknown) => void
    const promise = new Promise<T>((res, rej) => {
      resolve = res
      reject = rej
    })
    return { promise, resolve, reject }
  }
}

import { createLibp2p } from 'libp2p'
import { webSockets } from '@libp2p/websockets'
import { noise } from '@chainsafe/libp2p-noise'
import { yamux } from '@chainsafe/libp2p-yamux'
import { identify } from '@libp2p/identify'
import { writeFileSync, mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const ECHO_PROTOCOL = '/test/echo/1.0.0'
const LISTEN_PORT = Number(process.env.SMOKE_PORT ?? 8081)
const HERE = dirname(fileURLToPath(import.meta.url))
const ADDR_FILE = resolve(HERE, 'public', 'smoke-addr.txt')

const node = await createLibp2p({
  addresses: { listen: [`/ip4/127.0.0.1/tcp/${LISTEN_PORT}/ws`] },
  transports: [webSockets()],
  connectionEncrypters: [noise()],
  streamMuxers: [yamux()],
  services: { identify: identify() },
})

// libp2p v3 stream handler: (stream, connection) => void.
// Echo: on every inbound message, send the exact same bytes back.
await node.handle(ECHO_PROTOCOL, (stream) => {
  stream.addEventListener('message', (evt) => {
    stream.send(evt.data)
  })
})

await node.start()

const addrs = node.getMultiaddrs().map((a) => a.toString())
const dialable = addrs.find((a) => a.includes('/ws/'))
if (!dialable) {
  console.error('fixture-server: no /ws/ multiaddr advertised, cannot proceed')
  process.exit(1)
}

mkdirSync(dirname(ADDR_FILE), { recursive: true })
writeFileSync(ADDR_FILE, dialable + '\n', 'utf8')

console.log(`fixture-server: peer id ${node.peerId.toString()}`)
console.log(`fixture-server: listening ${dialable}`)
console.log(`fixture-server: wrote ${ADDR_FILE}`)
console.log(`fixture-server: echo protocol ${ECHO_PROTOCOL}`)
console.log('fixture-server: ready — press ctrl-c to stop')

const shutdown = async (): Promise<void> => {
  console.log('\nfixture-server: shutting down')
  await node.stop()
  process.exit(0)
}
process.on('SIGINT', shutdown)
process.on('SIGTERM', shutdown)
