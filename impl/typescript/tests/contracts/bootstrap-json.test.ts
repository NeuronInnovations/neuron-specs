// Spec 012 contract test — enforces contracts/bootstrap-json.md rules on
// src/browser-client/bootstrap-schema.ts.
//
// Traces: FR-B23, FR-B24, FR-B25.

import { describe, expect, it } from 'vitest'
import { validateBootstrap } from '../../src/browser-client/bootstrap-schema.js'
import {
  NeuronBrowserCode,
  NeuronBrowserError,
} from '../../src/browser-client/errors.js'

const LOCALHOST_ORIGIN = 'http://127.0.0.1:5173'
const HTTPS_ORIGIN = 'https://demo.example.com'

// A checksum-valid EIP-55 address + a well-formed base58btc PeerID for use in
// the happy-path fixture. These strings were produced during T001 runs.
const VALID_BOOTSTRAP = {
  version: 1,
  sellerEVMAddress: '0x5533527cF40444AC0c7e26490C6e02Fbddb97B21',
  sellerPeerID: '12D3KooWEShe5uWFxUyoL89fyVajW8aUQBGYRFT9JL19GpUy1H3M',
  sellerWSSMultiaddr: '/ip4/127.0.0.1/tcp/8080/ws/p2p/12D3KooWEShe5uWFxUyoL89fyVajW8aUQBGYRFT9JL19GpUy1H3M',
  controlStreamProtocolID: '/neuron/browser-profile/control/1.0.0',
  dataStreamProtocolID: '/neuron/browser-profile/data/1.0.0',
}

function expectCode(fn: () => unknown, code: string): void {
  try {
    fn()
    throw new Error('expected throw')
  } catch (err) {
    expect(err, `threw ${(err as Error).message}`).toBeInstanceOf(NeuronBrowserError)
    expect((err as NeuronBrowserError).code, `wrong code for ${(err as NeuronBrowserError).message}`).toBe(code)
  }
}

describe('contracts/bootstrap-json.md — happy path', () => {
  it('accepts a well-formed bootstrap with ws:// on localhost origin', () => {
    const b = validateBootstrap(VALID_BOOTSTRAP, LOCALHOST_ORIGIN)
    expect(b.version).toBe(1)
    expect(b.sellerEVMAddress).toBe(VALID_BOOTSTRAP.sellerEVMAddress)
    expect(b.controlStreamProtocolID).toBe('/neuron/browser-profile/control/1.0.0')
    expect(b.dataStreamProtocolID).toBe('/neuron/browser-profile/data/1.0.0')
  })
})

describe('contracts/bootstrap-json.md — strict schema', () => {
  it('rejects unknown top-level key with BOOTSTRAP_UNKNOWN_KEY (003)', () => {
    expectCode(
      () => validateBootstrap({ ...VALID_BOOTSTRAP, extra: 'x' }, LOCALHOST_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_UNKNOWN_KEY,
    )
  })

  it('rejects missing required field with BOOTSTRAP_MISSING_FIELD (004)', () => {
    const { sellerPeerID: _omit, ...rest } = VALID_BOOTSTRAP
    expectCode(
      () => validateBootstrap(rest, LOCALHOST_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_MISSING_FIELD,
    )
  })

  it('rejects version 2 with BOOTSTRAP_VERSION_MISMATCH (002)', () => {
    expectCode(
      () => validateBootstrap({ ...VALID_BOOTSTRAP, version: 2 }, LOCALHOST_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_VERSION_MISMATCH,
    )
  })

  it('rejects version as string with BOOTSTRAP_TYPE_MISMATCH (009)', () => {
    expectCode(
      () => validateBootstrap({ ...VALID_BOOTSTRAP, version: '1' }, LOCALHOST_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
    )
  })

  it('rejects non-object bootstrap with BOOTSTRAP_PARSE_FAILURE (011)', () => {
    for (const bad of [null, [], 'str', 42]) {
      expectCode(
        () => validateBootstrap(bad, LOCALHOST_ORIGIN),
        NeuronBrowserCode.BOOTSTRAP_PARSE_FAILURE,
      )
    }
  })
})

describe('contracts/bootstrap-json.md — EIP-55 and PeerID', () => {
  it('rejects a wrong-checksum EVM address with BOOTSTRAP_BAD_EVM_ADDRESS (005)', () => {
    // Mixed-case with one case flipped fails EIP-55.
    const bad = { ...VALID_BOOTSTRAP, sellerEVMAddress: '0x5533527CF40444AC0c7e26490C6e02Fbddb97B21' }
    // Uppercased 'C' at position 7 should be lowercase per EIP-55 for this address.
    expectCode(
      () => validateBootstrap(bad, LOCALHOST_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_BAD_EVM_ADDRESS,
    )
  })

  it('rejects a malformed PeerID with BOOTSTRAP_BAD_PEER_ID (006)', () => {
    expectCode(
      () => validateBootstrap({ ...VALID_BOOTSTRAP, sellerPeerID: 'not-a-peer-id' }, LOCALHOST_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_BAD_PEER_ID,
    )
  })
})

describe('contracts/bootstrap-json.md — multiaddr scheme', () => {
  it('rejects a /tcp-only multiaddr (no ws/wss) with BOOTSTRAP_BAD_MULTIADDR_SCHEME (007)', () => {
    expectCode(
      () => validateBootstrap({ ...VALID_BOOTSTRAP, sellerWSSMultiaddr: '/ip4/1.2.3.4/tcp/4001' }, LOCALHOST_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_BAD_MULTIADDR_SCHEME,
    )
  })

  it('rejects ws:// multiaddr when page origin is https:// (008)', () => {
    expectCode(
      () => validateBootstrap(VALID_BOOTSTRAP, HTTPS_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_WS_ON_NON_LOCALHOST,
    )
  })

  it('accepts /wss multiaddr from any origin', () => {
    const wssBootstrap = {
      ...VALID_BOOTSTRAP,
      sellerWSSMultiaddr: `/dns4/demo.example/tcp/443/tls/ws/p2p/${VALID_BOOTSTRAP.sellerPeerID}`,
    }
    const b = validateBootstrap(wssBootstrap, HTTPS_ORIGIN)
    expect(b.sellerWSSMultiaddr).toContain('/tls/ws')
  })

  it('accepts /ws when the page origin host matches the multiaddr host (VPS-demo case)', () => {
    // Same-host-origin relaxation: page served from http://203.0.113.42:5173
    // is allowed to dial ws://203.0.113.42:8080 because both are the same
    // host. Cross-origin insecure-ws remains rejected by the previous test.
    const vpsBootstrap = {
      ...VALID_BOOTSTRAP,
      sellerWSSMultiaddr: `/ip4/203.0.113.42/tcp/8080/ws/p2p/${VALID_BOOTSTRAP.sellerPeerID}`,
    }
    const b = validateBootstrap(vpsBootstrap, 'http://203.0.113.42:5173')
    expect(b.sellerWSSMultiaddr).toContain('/ip4/203.0.113.42/tcp/8080/ws')
  })

  it('rejects /ws when page origin host differs from multiaddr host (cross-origin-insecure-ws attack)', () => {
    // Attacker page on http://attacker.example tries to dial ws://victim.
    const victimAddr = {
      ...VALID_BOOTSTRAP,
      sellerWSSMultiaddr: `/ip4/203.0.113.42/tcp/8080/ws/p2p/${VALID_BOOTSTRAP.sellerPeerID}`,
    }
    expectCode(
      () => validateBootstrap(victimAddr, 'http://attacker.example:5173'),
      NeuronBrowserCode.BOOTSTRAP_WS_ON_NON_LOCALHOST,
    )
  })

  it('accepts /ws when a dns4 multiaddr host matches the page origin hostname', () => {
    const dnsBootstrap = {
      ...VALID_BOOTSTRAP,
      sellerWSSMultiaddr: `/dns4/demo.example/tcp/8080/ws/p2p/${VALID_BOOTSTRAP.sellerPeerID}`,
    }
    const b = validateBootstrap(dnsBootstrap, 'http://demo.example:5173')
    expect(b.sellerWSSMultiaddr).toContain('/dns4/demo.example')
  })
})

describe('contracts/bootstrap-json.md — stream-protocol pinning', () => {
  it('rejects a non-matching controlStreamProtocolID (strict v1)', () => {
    expectCode(
      () => validateBootstrap({ ...VALID_BOOTSTRAP, controlStreamProtocolID: '/x/y/1.0.0' }, LOCALHOST_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
    )
  })
  it('rejects a non-matching dataStreamProtocolID (strict v1)', () => {
    expectCode(
      () => validateBootstrap({ ...VALID_BOOTSTRAP, dataStreamProtocolID: '/x/y/1.0.0' }, LOCALHOST_ORIGIN),
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
    )
  })
})
