// Spec 012 — wire/frame.ts unit tests.
// Covers: byte-layout, round-trip, keep-alive, partial-push reassembly,
// multi-frame-per-push, oversize rejection, encoder bound.

import { describe, expect, it } from 'vitest'
import { encodeFrame, FrameDecoder, FrameError } from '../../src/wire/frame.js'
import { MAX_FRAME_BYTES } from '../../src/browser-client/constants.js'

describe('encodeFrame', () => {
  it('produces [uint32-BE len][payload] for a simple payload', () => {
    const payload = new Uint8Array([0x01, 0x02, 0x03, 0x04])
    const frame = encodeFrame(payload)
    // 00 00 00 04 01 02 03 04
    expect(Array.from(frame)).toEqual([0, 0, 0, 4, 1, 2, 3, 4])
  })

  it('encodes an empty payload as four zero bytes (keep-alive)', () => {
    const frame = encodeFrame(new Uint8Array(0))
    expect(Array.from(frame)).toEqual([0, 0, 0, 0])
  })

  it('writes the length big-endian (not little-endian)', () => {
    // 0x0102 = 258 bytes — small enough to encode without allocating a huge buffer.
    const small = new Uint8Array(258)
    const frame = encodeFrame(small)
    expect(Array.from(frame.subarray(0, 4))).toEqual([0, 0, 1, 2])
    expect(frame.length).toBe(4 + 258)
  })

  it('throws FrameError on payloads larger than MAX_FRAME_BYTES', () => {
    // Use a typed-array view rather than allocate 4 MiB in the test
    const oversize = new Uint8Array(MAX_FRAME_BYTES + 1)
    expect(() => encodeFrame(oversize)).toThrowError(FrameError)
  })
})

describe('FrameDecoder', () => {
  it('round-trips a single frame', () => {
    const payload = new Uint8Array([0x10, 0x20, 0x30])
    const received: Uint8Array[] = []
    const dec = new FrameDecoder((p) => received.push(p))
    dec.push(encodeFrame(payload))
    expect(received).toHaveLength(1)
    expect(Array.from(received[0]!)).toEqual([0x10, 0x20, 0x30])
    expect(dec.pendingBytes).toBe(0)
  })

  it('yields a zero-length payload for a keep-alive frame', () => {
    const received: Uint8Array[] = []
    const dec = new FrameDecoder((p) => received.push(p))
    dec.push(encodeFrame(new Uint8Array(0)))
    expect(received).toHaveLength(1)
    expect(received[0]!.length).toBe(0)
  })

  it('reassembles a frame split across multiple pushes (length prefix halved)', () => {
    const payload = new Uint8Array([0xaa, 0xbb, 0xcc])
    const frame = encodeFrame(payload)
    const received: Uint8Array[] = []
    const dec = new FrameDecoder((p) => received.push(p))
    dec.push(frame.subarray(0, 2))
    expect(received).toHaveLength(0)
    expect(dec.pendingBytes).toBe(2)
    dec.push(frame.subarray(2, 3))
    expect(received).toHaveLength(0)
    dec.push(frame.subarray(3, 5))
    expect(received).toHaveLength(0) // has 4-byte len prefix but no payload yet
    dec.push(frame.subarray(5))
    expect(received).toHaveLength(1)
    expect(Array.from(received[0]!)).toEqual([0xaa, 0xbb, 0xcc])
  })

  it('yields multiple frames from a single push', () => {
    const a = encodeFrame(new Uint8Array([1]))
    const b = encodeFrame(new Uint8Array([2, 3]))
    const c = encodeFrame(new Uint8Array(0)) // keep-alive
    const d = encodeFrame(new Uint8Array([4, 5, 6]))
    const merged = new Uint8Array(a.length + b.length + c.length + d.length)
    merged.set(a, 0)
    merged.set(b, a.length)
    merged.set(c, a.length + b.length)
    merged.set(d, a.length + b.length + c.length)
    const received: Uint8Array[] = []
    const dec = new FrameDecoder((p) => received.push(p))
    dec.push(merged)
    expect(received).toHaveLength(4)
    expect(Array.from(received[0]!)).toEqual([1])
    expect(Array.from(received[1]!)).toEqual([2, 3])
    expect(received[2]!.length).toBe(0)
    expect(Array.from(received[3]!)).toEqual([4, 5, 6])
  })

  it('throws FrameError when a frame declares a length larger than MAX_FRAME_BYTES', () => {
    const bogus = new Uint8Array(4)
    const dv = new DataView(bogus.buffer)
    dv.setUint32(0, MAX_FRAME_BYTES + 1, false)
    const received: Uint8Array[] = []
    const dec = new FrameDecoder((p) => received.push(p))
    expect(() => dec.push(bogus)).toThrowError(FrameError)
    expect(received).toHaveLength(0)
  })

  it('hands owned copies to the callback (buffer mutation does not affect emitted frame)', () => {
    const payload = new Uint8Array([1, 2, 3])
    const received: Uint8Array[] = []
    const dec = new FrameDecoder((p) => received.push(p))
    const frame = encodeFrame(payload)
    const wireBuffer = new Uint8Array(frame) // own copy the caller might mutate
    dec.push(wireBuffer)
    wireBuffer.fill(0xff) // mutate the caller's buffer after the push
    expect(Array.from(received[0]!)).toEqual([1, 2, 3]) // emitted frame unaffected
  })

  it('byte-layout fixture — the encoded form of a small JSON payload matches the Go framer exactly', () => {
    // Canonical-JSON-style payload: `{"ok":true}` UTF-8 = 11 bytes
    // Go framer would produce: 00 00 00 0B 7B 22 6F 6B 22 3A 74 72 75 65 7D
    const payload = new TextEncoder().encode('{"ok":true}')
    const frame = encodeFrame(payload)
    expect(Array.from(frame)).toEqual([
      0x00, 0x00, 0x00, 0x0b,
      0x7b, 0x22, 0x6f, 0x6b, 0x22, 0x3a, 0x74, 0x72, 0x75, 0x65, 0x7d,
    ])
  })
})
