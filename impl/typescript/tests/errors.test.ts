// Spec 012 — errors.ts unit tests.
// Verify code ranges → category mapping, message formatting, cause chaining.

import { describe, expect, it } from 'vitest'
import {
  makeNeuronError,
  NeuronBrowserCode,
  NeuronBrowserError,
} from '../src/browser-client/errors.js'

describe('NeuronBrowserError', () => {
  it('formats the message with the code prefix', () => {
    const err = makeNeuronError(NeuronBrowserCode.SENDER_PIN_MISMATCH, 'got 0xFOO, expected 0xBAR')
    expect(err.message).toBe('NEURON-BROWSER-062: got 0xFOO, expected 0xBAR')
    expect(err.code).toBe('NEURON-BROWSER-062')
  })

  it('derives the correct category from the numeric range', () => {
    const cases: Array<[string, string]> = [
      [NeuronBrowserCode.BOOTSTRAP_UNKNOWN_KEY, 'configuration'],
      [NeuronBrowserCode.TRANSPORT_STREAM_CLOSED_UNEXPECTEDLY, 'transport'],
      [NeuronBrowserCode.HANDSHAKE_PEER_ID_MISMATCH, 'handshake'],
      [NeuronBrowserCode.SIGNATURE_RECOVER_MISMATCH, 'signature'],
      [NeuronBrowserCode.FILE_SHA256_MISMATCH, 'hash-mismatch'],
      [NeuronBrowserCode.FRAME_SIZE_EXCEEDED, 'size-cap'],
      [NeuronBrowserCode.READ_IDLE_TIMEOUT, 'timeout'],
      [NeuronBrowserCode.UNEXPECTED_ENVELOPE_TYPE, 'state-machine'],
    ]
    for (const [code, category] of cases) {
      const err = makeNeuronError(code, 'x')
      expect(err.category, `${code} should be ${category}`).toBe(category)
    }
  })

  it('carries a typed cause', () => {
    const inner = new TypeError('wrapped')
    const err = makeNeuronError(NeuronBrowserCode.BOOTSTRAP_PARSE_FAILURE, 'bad json', inner)
    expect(err.cause).toBe(inner)
  })

  it('is an Error and an instanceof NeuronBrowserError', () => {
    const err = makeNeuronError(NeuronBrowserCode.READ_IDLE_TIMEOUT, 'x')
    expect(err).toBeInstanceOf(Error)
    expect(err).toBeInstanceOf(NeuronBrowserError)
    expect(err.name).toBe('NeuronBrowserError')
  })

  it('codes form the NEURON-BROWSER-NNN pattern', () => {
    for (const code of Object.values(NeuronBrowserCode)) {
      expect(code).toMatch(/^NEURON-BROWSER-\d{3}$/)
    }
  })
})
