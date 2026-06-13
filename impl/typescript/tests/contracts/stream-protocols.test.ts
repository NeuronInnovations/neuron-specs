// Spec 012 contract test — enforces contracts/stream-protocols.md rules on
// the shared framing (src/wire/frame.ts) and the data-plane frame-0 parser
// (src/browser-client/file-metadata.ts).
//
// Traces: FR-B11, FR-B12, FR-B19, FR-B20, FR-B21.

import { describe, expect, it } from 'vitest'
import {
  CONTROL_PROTOCOL_ID,
  DATA_PROTOCOL_ID,
  MAX_FRAME_BYTES,
  MAX_TOTAL_BYTES,
} from '../../src/browser-client/constants.js'
import { encodeFrame, FrameDecoder, FrameError } from '../../src/wire/frame.js'
import { parseFrameZero } from '../../src/browser-client/file-metadata.js'
import {
  NeuronBrowserCode,
  NeuronBrowserError,
} from '../../src/browser-client/errors.js'

const utf8 = (s: string): Uint8Array => new TextEncoder().encode(s)

describe('contracts/stream-protocols.md — protocol identifiers', () => {
  it('control stream protocol id matches the contract', () => {
    expect(CONTROL_PROTOCOL_ID).toBe('/neuron/browser-profile/control/1.0.0')
  })
  it('data stream protocol id matches the contract', () => {
    expect(DATA_PROTOCOL_ID).toBe('/neuron/browser-profile/data/1.0.0')
  })
  it('control and data protocol ids are distinct (FR-B11)', () => {
    expect(CONTROL_PROTOCOL_ID).not.toBe(DATA_PROTOCOL_ID)
  })
})

describe('contracts/stream-protocols.md — framing bounds', () => {
  it('MAX_FRAME_BYTES is exactly 4 MiB', () => {
    expect(MAX_FRAME_BYTES).toBe(4 * 1024 * 1024)
  })

  it('accepts a frame at the MAX_FRAME_BYTES boundary', () => {
    const atLimit = new Uint8Array(MAX_FRAME_BYTES) // all zeros is fine; only length matters
    const frame = encodeFrame(atLimit)
    const received: Uint8Array[] = []
    const dec = new FrameDecoder((p) => received.push(p))
    dec.push(frame)
    expect(received).toHaveLength(1)
    expect(received[0]!.length).toBe(MAX_FRAME_BYTES)
  })

  it('rejects a declared frame length of MAX_FRAME_BYTES + 1', () => {
    const bogus = new Uint8Array(4)
    new DataView(bogus.buffer).setUint32(0, MAX_FRAME_BYTES + 1, false)
    const dec = new FrameDecoder(() => { /* unused */ })
    expect(() => dec.push(bogus)).toThrowError(FrameError)
  })

  it('silently consumes keep-alive (zero-length) frames', () => {
    const received: Uint8Array[] = []
    const dec = new FrameDecoder((p) => received.push(p))
    dec.push(encodeFrame(new Uint8Array(0)))
    dec.push(encodeFrame(new Uint8Array(0)))
    dec.push(encodeFrame(utf8('data')))
    expect(received).toHaveLength(3)
    expect(received[0]!.length).toBe(0)
    expect(received[1]!.length).toBe(0)
    expect(received[2]!.length).toBe(4)
  })
})

describe('contracts/stream-protocols.md — data stream frame 0 (metadata)', () => {
  const validMeta = () => ({
    filename: 'demo.jpg',
    sizeBytes: 100_000,
    contentType: 'image/jpeg',
    sha256Hex: 'a'.repeat(64),
  })

  it('accepts a well-formed metadata frame', () => {
    const parsed = parseFrameZero(utf8(JSON.stringify(validMeta())))
    expect(parsed.filename).toBe('demo.jpg')
    expect(parsed.sizeBytes).toBe(100_000)
    expect(parsed.contentType).toBe('image/jpeg')
    expect(parsed.sha256Hex).toBe('a'.repeat(64))
  })

  it('rejects any unknown top-level field with METADATA_UNKNOWN_FIELD', () => {
    const meta = { ...validMeta(), extra: 'nope' }
    try {
      parseFrameZero(utf8(JSON.stringify(meta)))
      throw new Error('expected throw')
    } catch (err) {
      expect(err).toBeInstanceOf(NeuronBrowserError)
      expect((err as NeuronBrowserError).code).toBe(NeuronBrowserCode.METADATA_UNKNOWN_FIELD)
    }
  })

  it('rejects sizeBytes larger than MAX_TOTAL_BYTES with FRAME_SIZE_EXCEEDED (FR-B21)', () => {
    const meta = { ...validMeta(), sizeBytes: MAX_TOTAL_BYTES + 1 }
    try {
      parseFrameZero(utf8(JSON.stringify(meta)))
      throw new Error('expected throw')
    } catch (err) {
      expect(err).toBeInstanceOf(NeuronBrowserError)
      expect((err as NeuronBrowserError).code).toBe(NeuronBrowserCode.FRAME_SIZE_EXCEEDED)
    }
  })

  it('accepts sizeBytes exactly at MAX_TOTAL_BYTES', () => {
    const meta = { ...validMeta(), sizeBytes: MAX_TOTAL_BYTES }
    const parsed = parseFrameZero(utf8(JSON.stringify(meta)))
    expect(parsed.sizeBytes).toBe(MAX_TOTAL_BYTES)
  })

  it('rejects a content-type outside the allowlist', () => {
    const meta = { ...validMeta(), contentType: 'application/octet-stream' }
    expect(() => parseFrameZero(utf8(JSON.stringify(meta)))).toThrowError(NeuronBrowserError)
  })

  it('rejects a non-lowercase or wrong-length sha256Hex', () => {
    for (const bad of ['A'.repeat(64), 'a'.repeat(63), 'a'.repeat(65), 'zz' + 'a'.repeat(62)]) {
      const meta = { ...validMeta(), sha256Hex: bad }
      expect(() => parseFrameZero(utf8(JSON.stringify(meta)))).toThrowError(NeuronBrowserError)
    }
  })

  it('rejects a filename with a path separator', () => {
    for (const bad of ['../evil.jpg', 'a/b.jpg', 'a\\b.jpg']) {
      const meta = { ...validMeta(), filename: bad }
      expect(() => parseFrameZero(utf8(JSON.stringify(meta)))).toThrowError(NeuronBrowserError)
    }
  })

  it('rejects non-JSON-object inputs', () => {
    for (const bad of [utf8('null'), utf8('"str"'), utf8('[1,2]'), utf8('not json')]) {
      expect(() => parseFrameZero(bad)).toThrowError(NeuronBrowserError)
    }
  })

  it('rejects missing required fields with BOOTSTRAP_MISSING_FIELD (shared error code)', () => {
    const { filename: _omit, ...rest } = validMeta()
    try {
      parseFrameZero(utf8(JSON.stringify(rest)))
      throw new Error('expected throw')
    } catch (err) {
      expect(err).toBeInstanceOf(NeuronBrowserError)
      expect((err as NeuronBrowserError).code).toBe(NeuronBrowserCode.BOOTSTRAP_MISSING_FIELD)
    }
  })
})
