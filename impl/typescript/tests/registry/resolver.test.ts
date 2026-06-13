/**
 * Tests for registration resolution -- resolveTopics and resolveP2PExchange.
 *
 * Spec reference: 003 spec.md
 *   - FR-R02: Three neuron-topic services resolvable by channel.
 *   - FR-R03: neuron-p2p-exchange service resolvable.
 *   - SC-R02: Public comms resolvable from registration.
 *   - SC-R03: Multiaddress discoverable from registration.
 */

import { describe, it, expect } from 'vitest';
import { resolveTopics, resolveP2PExchange } from '../../src/registry/resolver.js';
import { RegistryError } from '../../src/registry/errors.js';
import type {
  Registration,
  NeuronTopicServiceEntry,
  NeuronP2PExchangeEntry,
} from '../../src/registry/types.js';

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

/** Create a neuron-topic service entry. */
function makeTopicService(channel: string, transport?: string): NeuronTopicServiceEntry {
  return {
    type: 'neuron-topic',
    name: channel,
    version: '1.0.0',
    channel,
    transport: transport ?? 'hcs',
    anchor: 'hedera-mainnet',
    config: { network: 'hedera-mainnet', topicId: `0.0.${channel}` },
  };
}

/** Create a neuron-p2p-exchange service entry. */
function makeP2PService(): NeuronP2PExchangeEntry {
  return {
    type: 'neuron-p2p-exchange',
    name: 'p2p',
    version: '1.0.0',
    peerID: '12D3KooWTestPeerID',
    protocol: '/neuron/multiaddr-exchange/1.0.0',
    topicRef: 'stdIn',
  };
}

/** Build a complete Registration. */
function makeRegistration(
  services?: (NeuronTopicServiceEntry | NeuronP2PExchangeEntry)[],
): Registration {
  return {
    registryAddress: '0x742d35Cc6634C0532925a3b844Bc9e7595f2bD61',
    childAddress: '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
    tokenId: 42n,
    agentURI: {
      services: services ?? [
        makeTopicService('stdIn'),
        makeTopicService('stdOut'),
        makeTopicService('stdErr'),
        makeP2PService(),
      ],
    },
    chainId: 1n,
  };
}

// ---------------------------------------------------------------------------
// resolveTopics
// ---------------------------------------------------------------------------

describe('resolveTopics()', () => {
  it('should resolve stdIn, stdOut, stdErr from a complete registration', () => {
    const registration = makeRegistration();
    const topics = resolveTopics(registration);

    expect(topics.stdIn.channel).toBe('stdIn');
    expect(topics.stdOut.channel).toBe('stdOut');
    expect(topics.stdErr.channel).toBe('stdErr');
  });

  it('should return correct service details for each channel', () => {
    const registration = makeRegistration([
      makeTopicService('stdIn', 'hcs'),
      makeTopicService('stdOut', 'kafka'),
      makeTopicService('stdErr', 'hcs'),
      makeP2PService(),
    ]);

    const topics = resolveTopics(registration);

    expect(topics.stdIn.transport).toBe('hcs');
    expect(topics.stdOut.transport).toBe('kafka');
    expect(topics.stdErr.transport).toBe('hcs');
    expect(topics.stdIn.version).toBe('1.0.0');
    expect(topics.stdOut.anchor).toBe('hedera-mainnet');
  });

  it('should throw NEURON-REG-005 when stdIn is missing', () => {
    const registration = makeRegistration([
      // No stdIn
      makeTopicService('stdOut'),
      makeTopicService('stdErr'),
      makeP2PService(),
    ]);

    expect(() => resolveTopics(registration)).toThrow(RegistryError);

    try {
      resolveTopics(registration);
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-005');
      expect(err.message).toContain('stdIn');
    }
  });

  it('should throw NEURON-REG-005 when stdOut is missing', () => {
    const registration = makeRegistration([
      makeTopicService('stdIn'),
      // No stdOut
      makeTopicService('stdErr'),
      makeP2PService(),
    ]);

    expect(() => resolveTopics(registration)).toThrow(RegistryError);

    try {
      resolveTopics(registration);
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-005');
      expect(err.message).toContain('stdOut');
    }
  });

  it('should throw NEURON-REG-005 when stdErr is missing', () => {
    const registration = makeRegistration([
      makeTopicService('stdIn'),
      makeTopicService('stdOut'),
      // No stdErr
      makeP2PService(),
    ]);

    expect(() => resolveTopics(registration)).toThrow(RegistryError);

    try {
      resolveTopics(registration);
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-005');
      expect(err.message).toContain('stdErr');
    }
  });

  it('should throw NEURON-REG-005 when no topic services exist', () => {
    const registration = makeRegistration([makeP2PService()]);

    expect(() => resolveTopics(registration)).toThrow(RegistryError);
  });
});

// ---------------------------------------------------------------------------
// resolveP2PExchange
// ---------------------------------------------------------------------------

describe('resolveP2PExchange()', () => {
  it('should resolve the first neuron-p2p-exchange service', () => {
    const registration = makeRegistration();
    const p2p = resolveP2PExchange(registration);

    expect(p2p.type).toBe('neuron-p2p-exchange');
    expect(p2p.peerID).toBe('12D3KooWTestPeerID');
    expect(p2p.protocol).toBe('/neuron/multiaddr-exchange/1.0.0');
    expect(p2p.topicRef).toBe('stdIn');
  });

  it('should return correct service details', () => {
    const registration = makeRegistration();
    const p2p = resolveP2PExchange(registration);

    expect(p2p.name).toBe('p2p');
    expect(p2p.version).toBe('1.0.0');
  });

  it('should throw NEURON-REG-005 when no p2p service exists', () => {
    const registration = makeRegistration([
      makeTopicService('stdIn'),
      makeTopicService('stdOut'),
      makeTopicService('stdErr'),
      // No P2P service
    ]);

    expect(() => resolveP2PExchange(registration)).toThrow(RegistryError);

    try {
      resolveP2PExchange(registration);
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-005');
      expect(err.message).toContain('neuron-p2p-exchange');
    }
  });

  it('should return the first p2p service when multiple exist', () => {
    const secondP2P: NeuronP2PExchangeEntry = {
      type: 'neuron-p2p-exchange',
      name: 'p2p-alt',
      version: '2.0.0',
      peerID: '12D3KooWOtherPeerID',
      protocol: '/neuron/multiaddr-exchange/2.0.0',
      topicRef: 'stdOut',
    };

    const registration = makeRegistration([
      makeTopicService('stdIn'),
      makeTopicService('stdOut'),
      makeTopicService('stdErr'),
      makeP2PService(),
      secondP2P,
    ]);

    const p2p = resolveP2PExchange(registration);

    // Should return the first one
    expect(p2p.name).toBe('p2p');
    expect(p2p.version).toBe('1.0.0');
  });
});

// ---------------------------------------------------------------------------
// Integration: resolveTopics + resolveP2PExchange
// ---------------------------------------------------------------------------

describe('resolveTopics + resolveP2PExchange integration', () => {
  it('should resolve both topics and P2P from the same registration', () => {
    const registration = makeRegistration();

    const topics = resolveTopics(registration);
    const p2p = resolveP2PExchange(registration);

    // P2P topicRef should reference an existing topic
    expect(p2p.topicRef).toBe('stdIn');
    expect(topics.stdIn.name).toBe('stdIn');

    // All three channels resolved
    expect(topics.stdIn).toBeDefined();
    expect(topics.stdOut).toBeDefined();
    expect(topics.stdErr).toBeDefined();
  });
});
