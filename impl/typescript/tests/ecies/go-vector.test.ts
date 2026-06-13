// Spec 012 / 009 — TS decrypt of Go-produced ECIES ciphertext.
// Fixture is regeneratable via:
//   go run ./cmd/ecies-fixture-gen --out ../typescript/tests/ecies/fixtures/go-vector.json
//
// This is the gating test for FR-B16 cross-language compliance — if it passes,
// TS and Go ECIES implementations agree byte-for-byte on the wire contract.

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { eciesDecrypt } from '../../src/browser-client/ecies-decrypt.js'

const HERE = dirname(fileURLToPath(import.meta.url))
const FIXTURE_PATH = resolve(HERE, 'fixtures', 'go-vector.json')

interface GoFixture {
  note: string
  recipientPrivKeyHex: string
  recipientPubKeyHexCompressed: string
  plaintext: string[]
  ciphertextBase64: string
}

function hexToBytes(hex: string): Uint8Array {
  const out = new Uint8Array(hex.length / 2)
  for (let i = 0; i < out.length; i++) out[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16)
  return out
}

describe('ECIES Go → TS interop', () => {
  it('decrypts a Go-produced ciphertext with the matching private key', () => {
    const fx = JSON.parse(readFileSync(FIXTURE_PATH, 'utf8')) as GoFixture
    const priv = hexToBytes(fx.recipientPrivKeyHex)
    const plainBytes = eciesDecrypt(fx.ciphertextBase64, priv)
    const plainStr = new TextDecoder().decode(plainBytes)
    const got = JSON.parse(plainStr) as string[]
    expect(got).toEqual(fx.plaintext)
  })
})
