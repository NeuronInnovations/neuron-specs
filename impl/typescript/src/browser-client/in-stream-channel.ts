// Spec 012 — shared helper that wraps a libp2p@3 Stream with the length-prefix
// framing from src/wire/frame.ts to provide a send/receive API for canonical-
// JSON TopicMessage envelopes. Used by both the browser side (in a real
// buyer's session) and the Node server side (seller host).
//
// Framing is identical to contracts/stream-protocols.md §"Framing" — one
// length-prefixed frame per envelope.
//
// Traces: FR-B10, FR-B12.

import { encodeFrame, FrameDecoder } from '../wire/frame.js'
import { makeNeuronError, NeuronBrowserCode } from './errors.js'

// Minimal subset of libp2p@3's Stream we actually use. Keeping this narrow
// means the helper is trivially unit-testable with a fake stream.
export interface LibP2PStreamLike {
  send(data: Uint8Array): boolean
  close(): Promise<void>
  addEventListener(
    type: 'message',
    handler: (evt: { data: Uint8Array | { subarray: () => Uint8Array } }) => void,
    opts?: AddEventListenerOptions,
  ): void
  addEventListener(type: string, handler: (evt?: unknown) => void, opts?: AddEventListenerOptions): void
}

interface AddEventListenerOptions {
  once?: boolean
  passive?: boolean
}

/**
 * Frame-aware sender over a libp2p stream. One envelope per `sendEnvelope()`.
 */
export class FramedSender {
  constructor(private readonly stream: LibP2PStreamLike) {}

  /** Send one canonical-JSON envelope (already a Uint8Array) as a length-prefixed frame. */
  sendEnvelope(envelopeBytes: Uint8Array): void {
    this.stream.send(encodeFrame(envelopeBytes))
  }

  /** Emit a zero-length keep-alive frame. */
  sendKeepAlive(): void {
    this.stream.send(encodeFrame(new Uint8Array(0)))
  }
}

/**
 * Frame-aware receiver that yields one envelope per completed frame via the
 * supplied callback. Zero-length frames are silently consumed. Close +
 * abnormal-close map to NEURON-BROWSER-021.
 */
export class FramedReceiver {
  private readonly decoder: FrameDecoder
  private closed = false

  constructor(
    private readonly stream: LibP2PStreamLike,
    onEnvelope: (envelopeBytes: Uint8Array) => void,
    onAbort: (err: Error) => void,
  ) {
    this.decoder = new FrameDecoder((payload) => {
      if (payload.length === 0) return // keep-alive
      onEnvelope(payload)
    })
    stream.addEventListener('message', (evt) => {
      const raw = evt.data
      const bytes =
        raw && typeof (raw as { subarray?: () => Uint8Array }).subarray === 'function'
          ? (raw as { subarray: () => Uint8Array }).subarray()
          : (raw as Uint8Array)
      try {
        this.decoder.push(bytes)
      } catch (err) {
        onAbort(
          err instanceof Error
            ? err
            : makeNeuronError(NeuronBrowserCode.FRAME_SIZE_EXCEEDED, String(err)),
        )
      }
    })
    stream.addEventListener('close', () => {
      if (this.closed) return
      this.closed = true
      onAbort(
        makeNeuronError(
          NeuronBrowserCode.TRANSPORT_STREAM_CLOSED_UNEXPECTEDLY,
          'libp2p stream closed unexpectedly',
        ),
      )
    })
  }

  markClosedGracefully(): void {
    this.closed = true
  }
}
