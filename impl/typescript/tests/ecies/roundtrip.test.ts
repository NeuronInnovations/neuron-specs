// Spec 012 / 009 — ECIES TS round-trip.
// Go-vector cross-check lives in tests/ecies/go-vector.test.ts (gated on
// producing a Go-side fixture via impl/golang/cmd/ecies-fixture-gen/).
//
// Traces: FR-B16.

import { describe, expect, it } from 'vitest'
import { getPublicKey, utils as secpUtils } from '@noble/secp256k1'
import { eciesEncrypt } from '../../src/server-demo/ecies-encrypt.js'
import { eciesDecrypt, ECIESError } from '../../src/browser-client/ecies-decrypt.js'

function utf8(s: string): Uint8Array { return new TextEncoder().encode(s) }
function utf8Decode(b: Uint8Array): string { return new TextDecoder().decode(b) }

describe('ECIES round-trip (TS ↔ TS)', () => {
  it('encrypts and decrypts a simple string payload', () => {
    const recipientPriv = secpUtils.randomPrivateKey()
    const recipientPub = getPublicKey(recipientPriv, true) // 33 bytes compressed
    const plain = utf8('hello ECIES')
    const wire = eciesEncrypt(plain, recipientPub)
    const got = eciesDecrypt(wire, recipientPriv)
    expect(utf8Decode(got)).toBe('hello ECIES')
  })

  it('handles a realistic multiaddr JSON payload', () => {
    const recipientPriv = secpUtils.randomPrivateKey()
    const recipientPub = getPublicKey(recipientPriv, true)
    const addrs = [
      '/ip4/127.0.0.1/tcp/8080/ws/p2p/12D3KooWExample1234567890',
      '/dns4/example.com/tcp/443/wss/p2p/12D3KooWExample0987654321',
    ]
    const wire = eciesEncrypt(utf8(JSON.stringify(addrs)), recipientPub)
    const got = JSON.parse(utf8Decode(eciesDecrypt(wire, recipientPriv))) as string[]
    expect(got).toEqual(addrs)
  })

  it('produces different ciphertexts for the same plaintext (randomised per FR-D13)', () => {
    const recipientPriv = secpUtils.randomPrivateKey()
    const recipientPub = getPublicKey(recipientPriv, true)
    const plain = utf8('deterministic plaintext')
    const a = eciesEncrypt(plain, recipientPub)
    const b = eciesEncrypt(plain, recipientPub)
    expect(a).not.toBe(b)
    // Both still decrypt to the original.
    expect(utf8Decode(eciesDecrypt(a, recipientPriv))).toBe('deterministic plaintext')
    expect(utf8Decode(eciesDecrypt(b, recipientPriv))).toBe('deterministic plaintext')
  })

  it('rejects a wrong-key decrypt attempt', () => {
    const correctPriv = secpUtils.randomPrivateKey()
    const correctPub = getPublicKey(correctPriv, true)
    const wrongPriv = secpUtils.randomPrivateKey()
    const wire = eciesEncrypt(utf8('secret'), correctPub)
    expect(() => eciesDecrypt(wire, wrongPriv)).toThrowError(ECIESError)
  })

  it('rejects a tampered ciphertext (GCM tag check)', () => {
    const recipientPriv = secpUtils.randomPrivateKey()
    const recipientPub = getPublicKey(recipientPriv, true)
    const wire = eciesEncrypt(utf8('payload'), recipientPub)
    // Flip one byte in the base64 body while staying within valid base64.
    // Insert a different char near the end (before padding if any).
    const tampered = wire.slice(0, wire.length - 2) + 'AA' + wire.slice(wire.length)
    expect(() => eciesDecrypt(tampered, recipientPriv)).toThrowError(ECIESError)
  })

  it('rejects a ciphertext that is too short', () => {
    const recipientPriv = secpUtils.randomPrivateKey()
    expect(() => eciesDecrypt('YWJj', recipientPriv)).toThrowError(/too short/)
  })

  it('byte-layout: output starts with a 33-byte compressed ephPub and has 12-byte nonce next', () => {
    const recipientPriv = secpUtils.randomPrivateKey()
    const recipientPub = getPublicKey(recipientPriv, true)
    const fixedEph = secpUtils.randomPrivateKey()
    const fixedNonce = new Uint8Array(12).map((_, i) => i + 1)
    const wire = eciesEncrypt(utf8('x'), recipientPub, { ephemeralPrivKey: fixedEph, nonce: fixedNonce })
    const raw = Uint8Array.from(atob(wire), (c) => c.charCodeAt(0))
    expect(raw.length).toBeGreaterThanOrEqual(33 + 12 + 16)
    // First byte is 0x02 or 0x03 (compressed prefix).
    expect([0x02, 0x03]).toContain(raw[0])
    // Bytes 33..45 MUST be the injected nonce.
    for (let i = 0; i < 12; i++) expect(raw[33 + i]).toBe(i + 1)
  })
})
