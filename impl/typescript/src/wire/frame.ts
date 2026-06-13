// Spec 012 / Spec 009 — length-prefix framing.
//
// Wire format per contracts/stream-protocols.md:
//   ┌────────────────────┬────────────────────────────────┐
//   │ 4 bytes BE uint32  │ N bytes payload                 │
//   │ payload length = N │                                 │
//   └────────────────────┴────────────────────────────────┘
//
// - N = 0 is a keep-alive; the decoder silently yields an empty Uint8Array.
// - N > MAX_FRAME_BYTES (4 MiB) is a protocol fault; throws FrameError.
// - This TypeScript implementation MUST produce byte-for-byte identical output
//   to impl/golang/internal/delivery/framing.go for the same payload.
// - Traces: FR-B12, FR-B19, FR-B20 (spec 012); FR-D22 (spec 009).

import { MAX_FRAME_BYTES } from '../browser-client/constants.js'

export class FrameError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'FrameError'
  }
}

/**
 * Encode a single frame.
 * `payload` may be empty; an empty payload encodes to 4 zero bytes (keep-alive).
 * Throws FrameError if payload exceeds MAX_FRAME_BYTES.
 */
export function encodeFrame(payload: Uint8Array): Uint8Array {
  if (payload.length > MAX_FRAME_BYTES) {
    throw new FrameError(`frame payload ${payload.length} bytes exceeds max ${MAX_FRAME_BYTES}`)
  }
  const out = new Uint8Array(4 + payload.length)
  const dv = new DataView(out.buffer)
  dv.setUint32(0, payload.length, /* littleEndian */ false)
  out.set(payload, 4)
  return out
}

/**
 * Streaming decoder for length-prefixed frames.
 *
 * Feed arbitrary byte chunks via push(); receive complete frame payloads via
 * the `frame` callback. Safe under any split (partial length prefix, partial
 * payload, multiple frames in one push, frame spanning multiple pushes).
 *
 * Not safe for concurrent pushes; callers serialise.
 */
export class FrameDecoder {
  private readonly onFrame: (payload: Uint8Array) => void
  private buffer: Uint8Array = new Uint8Array(0)

  constructor(onFrame: (payload: Uint8Array) => void) {
    this.onFrame = onFrame
  }

  /**
   * Feed inbound bytes. Emits any complete frames to the `onFrame` callback
   * (possibly 0 or >1 per call). Throws FrameError on an oversize frame.
   */
  push(chunk: Uint8Array): void {
    if (chunk.length === 0) return
    this.buffer = concat(this.buffer, chunk)
    for (;;) {
      if (this.buffer.length < 4) return
      const len = new DataView(
        this.buffer.buffer,
        this.buffer.byteOffset,
        4,
      ).getUint32(0, /* littleEndian */ false)
      if (len > MAX_FRAME_BYTES) {
        throw new FrameError(`frame length ${len} exceeds max ${MAX_FRAME_BYTES}`)
      }
      if (this.buffer.length < 4 + len) return
      const payload = this.buffer.subarray(4, 4 + len)
      // Copy so downstream consumers can't observe future buffer mutations.
      const owned = new Uint8Array(payload.length)
      owned.set(payload)
      this.buffer = this.buffer.subarray(4 + len)
      this.onFrame(owned)
    }
  }

  /** Bytes already seen but not yet resolved into a complete frame. */
  get pendingBytes(): number {
    return this.buffer.length
  }
}

function concat(a: Uint8Array, b: Uint8Array): Uint8Array {
  if (a.length === 0) return b
  if (b.length === 0) return a
  const out = new Uint8Array(a.length + b.length)
  out.set(a, 0)
  out.set(b, a.length)
  return out
}
