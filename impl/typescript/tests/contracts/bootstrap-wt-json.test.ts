// 2a-wt — contract test for bootstrap-wt-schema.ts.
//
// Parallel to tests/contracts/bootstrap-json.test.ts (Tier 1). Kept as
// a separate file; neither test file mutates the other's schema.

import { describe, expect, it } from 'vitest'
import { validateBootstrapWt } from '../../src/browser-client-wt/bootstrap-wt-schema.js'
import {
  NeuronBrowserCode,
  NeuronBrowserError,
} from '../../src/browser-client/errors.js'
import {
  BOOTSTRAP_VERSION_WT,
  CONTROL_PROTOCOL_ID,
  DATA_PROTOCOL_ID,
  ECHO_PROTOCOL_ID,
} from '../../src/browser-client-wt/constants.js'

const VALID_PEER_ID = '16Uiu2HAm7zG9AKs9JMbZigrTf8yw2pBDcm2bJghNWv5NG5u96Y9o'
const VALID_MULTIADDR =
  '/ip4/127.0.0.1/udp/4443/quic-v1/webtransport/certhash/uEiB9vrDDe0XUBV9jRTEdNSgDk0itbza-l3bILWI7q5kUHQ/certhash/uEiAqTtUN1SZJUUpnWW-GTxjkrClc7HzCLdIHFyT5JNrkvw/p2p/' +
  VALID_PEER_ID

const VALID_BOOTSTRAP = {
  version: BOOTSTRAP_VERSION_WT,
  sellerEVMAddress: '0x5533527cF40444AC0c7e26490C6e02Fbddb97B21',
  sellerPeerID: VALID_PEER_ID,
  sellerWTMultiaddr: VALID_MULTIADDR,
  controlStreamProtocolID: CONTROL_PROTOCOL_ID,
  dataStreamProtocolID: DATA_PROTOCOL_ID,
  echoProtocolID: ECHO_PROTOCOL_ID,
}

function expectCode(fn: () => unknown, code: string): void {
  try {
    fn()
    throw new Error('expected throw')
  } catch (err) {
    expect(err, `threw ${(err as Error).message}`).toBeInstanceOf(NeuronBrowserError)
    expect((err as NeuronBrowserError).code).toBe(code)
  }
}

describe('2a-wt bootstrap validator — happy path', () => {
  it('accepts a well-formed 2a-wt bootstrap', () => {
    const b = validateBootstrapWt(VALID_BOOTSTRAP)
    expect(b.version).toBe(BOOTSTRAP_VERSION_WT)
    expect(b.sellerPeerID).toBe(VALID_PEER_ID)
    expect(b.sellerWTMultiaddr).toBe(VALID_MULTIADDR)
    expect(b.echoProtocolID).toBe(ECHO_PROTOCOL_ID)
  })
})

describe('2a-wt bootstrap validator — strict schema', () => {
  it('rejects Tier-1 version number with BOOTSTRAP_TYPE_MISMATCH', () => {
    expectCode(
      () => validateBootstrapWt({ ...VALID_BOOTSTRAP, version: 1 }),
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
    )
  })

  it('rejects wrong version string with BOOTSTRAP_VERSION_MISMATCH', () => {
    expectCode(
      () => validateBootstrapWt({ ...VALID_BOOTSTRAP, version: '1' }),
      NeuronBrowserCode.BOOTSTRAP_VERSION_MISMATCH,
    )
  })

  it('rejects unknown field with BOOTSTRAP_UNKNOWN_KEY', () => {
    expectCode(
      () => validateBootstrapWt({ ...VALID_BOOTSTRAP, extra: 'x' }),
      NeuronBrowserCode.BOOTSTRAP_UNKNOWN_KEY,
    )
  })

  it('rejects missing echoProtocolID with BOOTSTRAP_MISSING_FIELD', () => {
    const { echoProtocolID: _omit, ...rest } = VALID_BOOTSTRAP
    expectCode(
      () => validateBootstrapWt(rest),
      NeuronBrowserCode.BOOTSTRAP_MISSING_FIELD,
    )
  })
})

describe('2a-wt bootstrap validator — multiaddr discipline', () => {
  it('rejects WSS multiaddr (Tier 1 shape) with BOOTSTRAP_BAD_MULTIADDR_SCHEME', () => {
    expectCode(
      () =>
        validateBootstrapWt({
          ...VALID_BOOTSTRAP,
          sellerWTMultiaddr: '/ip4/127.0.0.1/tcp/8080/ws/p2p/' + VALID_PEER_ID,
        }),
      NeuronBrowserCode.BOOTSTRAP_BAD_MULTIADDR_SCHEME,
    )
  })

  it('rejects /webtransport without /certhash with BOOTSTRAP_BAD_MULTIADDR_SCHEME', () => {
    expectCode(
      () =>
        validateBootstrapWt({
          ...VALID_BOOTSTRAP,
          sellerWTMultiaddr:
            '/ip4/127.0.0.1/udp/4443/quic-v1/webtransport/p2p/' + VALID_PEER_ID,
        }),
      NeuronBrowserCode.BOOTSTRAP_BAD_MULTIADDR_SCHEME,
    )
  })

  it('rejects missing /quic-v1 with BOOTSTRAP_BAD_MULTIADDR_SCHEME', () => {
    expectCode(
      () =>
        validateBootstrapWt({
          ...VALID_BOOTSTRAP,
          sellerWTMultiaddr:
            '/ip4/127.0.0.1/udp/4443/webtransport/certhash/uEiA/p2p/' + VALID_PEER_ID,
        }),
      NeuronBrowserCode.BOOTSTRAP_BAD_MULTIADDR_SCHEME,
    )
  })

  it('rejects /p2p/ suffix that mismatches sellerPeerID with BOOTSTRAP_BAD_PEER_ID', () => {
    expectCode(
      () =>
        validateBootstrapWt({
          ...VALID_BOOTSTRAP,
          sellerWTMultiaddr:
            '/ip4/127.0.0.1/udp/4443/quic-v1/webtransport/certhash/uEiA/p2p/16Uiu2HAmOther1234567890Other1234567890Other1234567890',
        }),
      NeuronBrowserCode.BOOTSTRAP_BAD_PEER_ID,
    )
  })
})

describe('2a-wt bootstrap validator — protocol ID discipline', () => {
  it('rejects wrong control protocol with BOOTSTRAP_TYPE_MISMATCH', () => {
    expectCode(
      () =>
        validateBootstrapWt({
          ...VALID_BOOTSTRAP,
          controlStreamProtocolID: '/wrong/1',
        }),
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
    )
  })

  it('rejects wrong echo protocol with BOOTSTRAP_TYPE_MISMATCH', () => {
    expectCode(
      () =>
        validateBootstrapWt({
          ...VALID_BOOTSTRAP,
          echoProtocolID: '/wrong/1',
        }),
      NeuronBrowserCode.BOOTSTRAP_TYPE_MISMATCH,
    )
  })
})

describe('2a-wt bootstrap validator — semantics', () => {
  it('rejects bad EIP-55 (mixed-case checksum) with BOOTSTRAP_BAD_EVM_ADDRESS', () => {
    // Valid hex, mixed-case, but EIP-55 checksum is wrong (flipped case).
    expectCode(
      () =>
        validateBootstrapWt({
          ...VALID_BOOTSTRAP,
          sellerEVMAddress: '0x5533527Cf40444aC0C7E26490c6e02fbDdB97b21',
        }),
      NeuronBrowserCode.BOOTSTRAP_BAD_EVM_ADDRESS,
    )
  })

  it('rejects non-hex EVM address with BOOTSTRAP_BAD_EVM_ADDRESS', () => {
    expectCode(
      () =>
        validateBootstrapWt({
          ...VALID_BOOTSTRAP,
          sellerEVMAddress: 'not-an-address',
        }),
      NeuronBrowserCode.BOOTSTRAP_BAD_EVM_ADDRESS,
    )
  })

  it('rejects non-object with BOOTSTRAP_PARSE_FAILURE', () => {
    for (const bad of [null, [], 'str', 42]) {
      expectCode(
        () => validateBootstrapWt(bad),
        NeuronBrowserCode.BOOTSTRAP_PARSE_FAILURE,
      )
    }
  })
})
