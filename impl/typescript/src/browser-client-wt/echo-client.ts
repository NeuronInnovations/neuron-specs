// 2a-wt — Tier A echo client.
//
// Sends "ping:<timestamp>\n" on the echo stream, reads back
// "pong:<timestamp>\n", asserts the payload is preserved, and returns
// the measured round-trip time.
//
// Uses the libp2p@3 message-based stream API (send + addEventListener),
// matching the existing Tier 1 helper at src/browser-client/in-stream-channel.ts.

import { ECHO_MAX_LINE_BYTES, ECHO_PROTOCOL_ID } from './constants.js'
import { makeNeuronError, NeuronBrowserCode } from '../browser-client/errors.js'

// Narrow subset of the libp2p@3 stream shape that we actually need.
export interface EchoStreamLike {
  send(data: Uint8Array): boolean
  close(): Promise<void>
  addEventListener(
    type: 'message',
    handler: (evt: { data: Uint8Array | { subarray: () => Uint8Array } }) => void,
    opts?: { once?: boolean },
  ): void
  addEventListener(
    type: string,
    handler: (evt?: unknown) => void,
    opts?: { once?: boolean },
  ): void
}

export interface EchoResult {
  readonly rttMs: number
  readonly payload: string
  readonly remotePeerId: string
}

/**
 * Perform one ping -> pong exchange over an already-opened protocol
 * stream. The caller owns opening the stream (typical: via dialWtProtocol).
 * The stream is closed after a successful exchange.
 *
 * Wire contract (must match impl/golang/internal/browserprofile/echo_handler.go):
 *   client  -> "ping:<payload>\n"
 *   server  -> "pong:<payload>\n"
 */
export function performEcho(
  stream: EchoStreamLike,
  remotePeerId: string,
  timeoutMs = 10_000,
): Promise<EchoResult> {
  return new Promise<EchoResult>((resolve, reject) => {
    const payload = Date.now().toString(10)
    const expectedPrefix = `pong:${payload}`

    const encoder = new TextEncoder()
    const decoder = new TextDecoder()
    let buf = ''
    let settled = false
    const startedAt = performance.now()

    const done = (fn: () => void): void => {
      if (settled) return
      settled = true
      fn()
      stream.close().catch(() => { /* ignore */ })
    }

    const timer = setTimeout(() => {
      done(() =>
        reject(
          makeNeuronError(
            NeuronBrowserCode.READ_IDLE_TIMEOUT,
            `echo timed out after ${timeoutMs} ms`,
          ),
        ),
      )
    }, timeoutMs)

    const onMessage = (evt: { data: Uint8Array | { subarray: () => Uint8Array } }): void => {
      if (settled) return
      const raw = evt.data
      const bytes =
        raw && typeof (raw as { subarray?: () => Uint8Array }).subarray === 'function'
          ? (raw as { subarray: () => Uint8Array }).subarray()
          : (raw as Uint8Array)
      buf += decoder.decode(bytes, { stream: true })

      if (buf.length > ECHO_MAX_LINE_BYTES) {
        clearTimeout(timer)
        done(() =>
          reject(
            makeNeuronError(
              NeuronBrowserCode.FRAME_SIZE_EXCEEDED,
              `echo response exceeded ${ECHO_MAX_LINE_BYTES} bytes before newline`,
            ),
          ),
        )
        return
      }

      const nl = buf.indexOf('\n')
      if (nl === -1) return

      const firstLine = buf.slice(0, nl)
      clearTimeout(timer)

      if (!firstLine.startsWith(expectedPrefix)) {
        done(() =>
          reject(
            makeNeuronError(
              NeuronBrowserCode.UNEXPECTED_ENVELOPE_TYPE,
              `echo response malformed; expected ${expectedPrefix}, got ${firstLine}`,
            ),
          ),
        )
        return
      }

      const rttMs = Math.round(performance.now() - startedAt)
      done(() => resolve({ rttMs, payload, remotePeerId }))
    }

    const onClose = (): void => {
      if (settled) return
      clearTimeout(timer)
      done(() =>
        reject(
          makeNeuronError(
            NeuronBrowserCode.TRANSPORT_STREAM_CLOSED_UNEXPECTEDLY,
            `echo stream closed before pong received (buf=${JSON.stringify(buf)})`,
          ),
        ),
      )
    }

    stream.addEventListener('message', onMessage)
    stream.addEventListener('close', onClose, { once: true })

    // Write the single ping line. In libp2p@3 send() throws on a real
    // write failure; a false return value is only a backpressure signal
    // ("wait for drain before sending more"). For this single-shot
    // handshake the signal is safe to ignore.
    try {
      stream.send(encoder.encode(`ping:${payload}\n`))
    } catch (err) {
      clearTimeout(timer)
      done(() =>
        reject(
          makeNeuronError(
            NeuronBrowserCode.TRANSPORT_STREAM_CLOSED_UNEXPECTEDLY,
            `stream.send threw: ${(err as Error).message ?? String(err)}`,
            err,
          ),
        ),
      )
    }
  })
}

export { ECHO_PROTOCOL_ID }
