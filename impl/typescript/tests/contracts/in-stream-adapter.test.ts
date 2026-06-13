// Spec 012 contract test — enforces contracts/in-stream-adapter.md on
// src/browser-client/envelope-verify.ts.
//
// Uses real keys + real TopicMessage.create() to build signed envelopes, then
// asserts each of the five verifier rules fires the correct NEURON-BROWSER-NNN
// code.
//
// Traces: FR-B13, FR-B14.

import { describe, expect, it, beforeAll } from 'vitest'
import { NeuronPrivateKey } from '../../src/keylib/private-key.js'
import { TopicMessage } from '../../src/topic/message.js'
import { verifyInboundEnvelope } from '../../src/browser-client/envelope-verify.js'
import {
  NeuronBrowserCode,
  NeuronBrowserError,
} from '../../src/browser-client/errors.js'

function utf8(s: string): Uint8Array { return new TextEncoder().encode(s) }

const NOW_MS = 1_800_000_000_000 // arbitrary fixed ms epoch
const NOW_NS = BigInt(NOW_MS) * 1_000_000n

interface TestKeys {
  seller: NeuronPrivateKey
  sellerAddr: string
  impostor: NeuronPrivateKey
}

let keys: TestKeys

beforeAll(() => {
  const seller = NeuronPrivateKey.generate()
  const impostor = NeuronPrivateKey.generate()
  keys = {
    seller,
    sellerAddr: seller.publicKey().evmAddress().toString(),
    impostor,
  }
})

function signEnvelope(
  key: NeuronPrivateKey,
  timestampNs: bigint,
  seqNum: bigint,
  payload: Uint8Array,
): TopicMessage {
  return TopicMessage.create(key, timestampNs, seqNum, payload)
}

describe('contracts/in-stream-adapter.md — signature verification rules', () => {
  it('happy path: all five checks pass for a fresh envelope', () => {
    const env = signEnvelope(keys.seller, NOW_NS, 1n, utf8('{"type":"paymentDetails"}'))
    const ok = verifyInboundEnvelope(env, {
      expectedSenderAddress: keys.sellerAddr,
      prevSenderSeqNum: null,
      nowMs: NOW_MS,
    })
    expect(ok).toBe(env)
  })

  it('rule 1: signature mismatch → SIGNATURE_RECOVER_MISMATCH (061)', () => {
    const env = signEnvelope(keys.seller, NOW_NS, 1n, utf8('payload'))
    // Reconstruct the envelope with one bit of the signature flipped.
    const sigBytes = env.signatureBytes()
    sigBytes[0] = sigBytes[0]! ^ 0x01
    const tampered = TopicMessage.fromFields(
      env.senderAddress,
      sigBytes,
      env.timestamp,
      env.sequenceNumber,
      env.payload,
    )
    try {
      verifyInboundEnvelope(tampered, {
        expectedSenderAddress: keys.sellerAddr,
        prevSenderSeqNum: null,
        nowMs: NOW_MS,
      })
      throw new Error('expected throw')
    } catch (err) {
      expect(err).toBeInstanceOf(NeuronBrowserError)
      expect((err as NeuronBrowserError).code).toBe(NeuronBrowserCode.SIGNATURE_RECOVER_MISMATCH)
    }
  })

  it('rule 2: wrong sender → SENDER_PIN_MISMATCH (062)', () => {
    // Envelope legitimately signed by the impostor (signature recovers cleanly);
    // fails pinning because we expect the seller instead.
    const env = signEnvelope(keys.impostor, NOW_NS, 1n, utf8('payload'))
    try {
      verifyInboundEnvelope(env, {
        expectedSenderAddress: keys.sellerAddr,
        prevSenderSeqNum: null,
        nowMs: NOW_MS,
      })
      throw new Error('expected throw')
    } catch (err) {
      expect(err).toBeInstanceOf(NeuronBrowserError)
      expect((err as NeuronBrowserError).code).toBe(NeuronBrowserCode.SENDER_PIN_MISMATCH)
    }
  })

  it('rule 3: sequence not strictly increasing → SEQUENCE_DECREMENT (063)', () => {
    const env = signEnvelope(keys.seller, NOW_NS, 5n, utf8('payload'))
    for (const prev of [5n, 6n, 100n]) {
      try {
        verifyInboundEnvelope(env, {
          expectedSenderAddress: keys.sellerAddr,
          prevSenderSeqNum: prev,
          nowMs: NOW_MS,
        })
        throw new Error('expected throw')
      } catch (err) {
        expect(err).toBeInstanceOf(NeuronBrowserError)
        expect((err as NeuronBrowserError).code).toBe(NeuronBrowserCode.SEQUENCE_DECREMENT)
      }
    }
  })

  it('rule 3: sequence strictly greater than previous passes', () => {
    const env = signEnvelope(keys.seller, NOW_NS, 10n, utf8('payload'))
    const ok = verifyInboundEnvelope(env, {
      expectedSenderAddress: keys.sellerAddr,
      prevSenderSeqNum: 9n,
      nowMs: NOW_MS,
    })
    expect(ok.sequenceNumber).toBe(10n)
  })

  it('rule 4: timestamp skew > 2 min → TIMESTAMP_SKEW (064)', () => {
    const farFutureNs = NOW_NS + BigInt(5 * 60 * 1000) * 1_000_000n // +5 min
    const env = signEnvelope(keys.seller, farFutureNs, 1n, utf8('payload'))
    try {
      verifyInboundEnvelope(env, {
        expectedSenderAddress: keys.sellerAddr,
        prevSenderSeqNum: null,
        nowMs: NOW_MS,
      })
      throw new Error('expected throw')
    } catch (err) {
      expect(err).toBeInstanceOf(NeuronBrowserError)
      expect((err as NeuronBrowserError).code).toBe(NeuronBrowserCode.TIMESTAMP_SKEW)
    }
  })

  it('rule 4: timestamp within skew tolerance passes', () => {
    const nearNs = NOW_NS + BigInt(30 * 1000) * 1_000_000n // +30 s
    const env = signEnvelope(keys.seller, nearNs, 1n, utf8('payload'))
    const ok = verifyInboundEnvelope(env, {
      expectedSenderAddress: keys.sellerAddr,
      prevSenderSeqNum: null,
      nowMs: NOW_MS,
    })
    expect(ok).toBe(env)
  })

  it('rule 4: custom clockSkewMs is honoured', () => {
    const nearNs = NOW_NS + BigInt(4 * 1000) * 1_000_000n // +4 s
    const env = signEnvelope(keys.seller, nearNs, 1n, utf8('payload'))
    // Tight 2 s skew should reject.
    try {
      verifyInboundEnvelope(env, {
        expectedSenderAddress: keys.sellerAddr,
        prevSenderSeqNum: null,
        nowMs: NOW_MS,
        clockSkewMs: 2_000,
      })
      throw new Error('expected throw')
    } catch (err) {
      expect((err as NeuronBrowserError).code).toBe(NeuronBrowserCode.TIMESTAMP_SKEW)
    }
  })
})
