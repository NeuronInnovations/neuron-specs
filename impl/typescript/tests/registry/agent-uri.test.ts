/**
 * Tests for AgentURI construction, parsing, and serialization.
 *
 * Spec reference: 003 data-model.md
 *   - AgentURI entity: JSON with services array.
 *   - Canonical ordering: neuron-topic (stdIn, stdOut, stdErr),
 *     then neuron-p2p-exchange, then DID.
 *
 * SC-R01: Round-trip guarantee (serialize then parse produces identical result).
 */

import { describe, it, expect } from 'vitest';
import {
  buildAgentURI,
  parseAgentURI,
  serializeAgentURI,
} from '../../src/registry/agent-uri.js';
import { RegistryError } from '../../src/registry/errors.js';
import type {
  NeuronTopicServiceEntry,
  NeuronP2PExchangeEntry,
  DIDServiceEntry,
} from '../../src/registry/types.js';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

function makeTopicService(channel: string): NeuronTopicServiceEntry {
  return {
    type: 'neuron-topic',
    name: channel,
    version: '1.0.0',
    channel,
    transport: 'hcs',
    anchor: 'hedera-mainnet',
    config: { network: 'hedera-mainnet', topicId: `0.0.${channel}` },
  };
}

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

function makeDIDService(): DIDServiceEntry {
  return {
    type: 'DID',
    name: 'DID',
    endpoint: 'did:key:zQ3shP2mWsZYWgpKDXRRx8rBe6UaDQY4mJgZrm5KywKgjqiU9',
    version: 'v1',
  };
}

// ---------------------------------------------------------------------------
// buildAgentURI
// ---------------------------------------------------------------------------

describe('buildAgentURI()', () => {
  it('should build an AgentURI with topics and P2P services', () => {
    const topics = [
      makeTopicService('stdIn'),
      makeTopicService('stdOut'),
      makeTopicService('stdErr'),
    ];
    const p2p = [makeP2PService()];

    const uri = buildAgentURI(topics, p2p);

    expect(uri.services).toHaveLength(4);
    expect(uri.services[0]!.type).toBe('neuron-topic');
    expect(uri.services[3]!.type).toBe('neuron-p2p-exchange');
  });

  it('should place topics in canonical channel order (stdIn, stdOut, stdErr)', () => {
    // Deliberately pass in wrong order
    const topics = [
      makeTopicService('stdErr'),
      makeTopicService('stdIn'),
      makeTopicService('stdOut'),
    ];

    const uri = buildAgentURI(topics, [makeP2PService()]);

    const channels = uri.services
      .filter(s => s.type === 'neuron-topic')
      .map(s => (s as NeuronTopicServiceEntry).channel);

    expect(channels).toEqual(['stdIn', 'stdOut', 'stdErr']);
  });

  it('should include DID service at the end when provided', () => {
    const topics = [
      makeTopicService('stdIn'),
      makeTopicService('stdOut'),
      makeTopicService('stdErr'),
    ];
    const p2p = [makeP2PService()];
    const did = makeDIDService();

    const uri = buildAgentURI(topics, p2p, did);

    expect(uri.services).toHaveLength(5);
    expect(uri.services[4]!.type).toBe('DID');
  });

  it('should not include DID service when not provided', () => {
    const topics = [
      makeTopicService('stdIn'),
      makeTopicService('stdOut'),
      makeTopicService('stdErr'),
    ];

    const uri = buildAgentURI(topics, [makeP2PService()]);

    expect(uri.services).toHaveLength(4);
    const types = uri.services.map(s => s.type);
    expect(types).not.toContain('DID');
  });
});

// ---------------------------------------------------------------------------
// parseAgentURI
// ---------------------------------------------------------------------------

describe('parseAgentURI()', () => {
  it('should parse a valid JSON AgentURI', () => {
    const json = JSON.stringify({
      services: [
        {
          type: 'neuron-topic',
          name: 'stdIn',
          version: '1.0.0',
          channel: 'stdIn',
          transport: 'hcs',
          anchor: 'hedera-mainnet',
          config: { network: 'hedera-mainnet', topicId: '0.0.123' },
        },
        {
          type: 'neuron-p2p-exchange',
          name: 'p2p',
          version: '1.0.0',
          peerID: '12D3KooWTest',
          protocol: '/neuron/multiaddr-exchange/1.0.0',
          topicRef: 'stdIn',
        },
      ],
    });

    const uri = parseAgentURI(json);

    expect(uri.services).toHaveLength(2);
    expect(uri.services[0]!.type).toBe('neuron-topic');
    expect(uri.services[1]!.type).toBe('neuron-p2p-exchange');
  });

  it('should parse DID services', () => {
    const json = JSON.stringify({
      services: [
        {
          type: 'DID',
          name: 'DID',
          endpoint: 'did:key:zQ3sTest',
          version: 'v1',
        },
      ],
    });

    const uri = parseAgentURI(json);

    expect(uri.services).toHaveLength(1);
    expect(uri.services[0]!.type).toBe('DID');
    if (uri.services[0]!.type === 'DID') {
      expect(uri.services[0]!.endpoint).toBe('did:key:zQ3sTest');
    }
  });

  it('should parse neuron-topic with optional endpoint field', () => {
    const json = JSON.stringify({
      services: [
        {
          type: 'neuron-topic',
          name: 'stdIn',
          version: '1.0.0',
          channel: 'stdIn',
          transport: 'hcs',
          anchor: 'hedera-mainnet',
          config: { topicId: '0.0.123' },
          endpoint: 'hcs://0.0.123',
        },
      ],
    });

    const uri = parseAgentURI(json);

    const svc = uri.services[0] as NeuronTopicServiceEntry;
    expect(svc.endpoint).toBe('hcs://0.0.123');
  });

  it('should ignore unrecognized service types', () => {
    const json = JSON.stringify({
      services: [
        { type: 'unknown-service', name: 'something', version: '1.0' },
        {
          type: 'neuron-topic',
          name: 'stdIn',
          version: '1.0.0',
          channel: 'stdIn',
          transport: 'hcs',
          anchor: 'hedera-mainnet',
          config: {},
        },
      ],
    });

    const uri = parseAgentURI(json);

    // Only the recognized neuron-topic is included
    expect(uri.services).toHaveLength(1);
    expect(uri.services[0]!.type).toBe('neuron-topic');
  });

  it('should throw NEURON-REG-005 on malformed JSON', () => {
    expect(() => parseAgentURI('not-json')).toThrow(RegistryError);

    try {
      parseAgentURI('not-json');
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-005');
      expect(err.message).toContain('parse');
    }
  });

  it('should throw NEURON-REG-005 when document is not an object', () => {
    expect(() => parseAgentURI('"a string"')).toThrow(RegistryError);
  });

  it('should throw NEURON-REG-005 when services array is missing', () => {
    expect(() => parseAgentURI('{}')).toThrow(RegistryError);

    try {
      parseAgentURI('{}');
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-005');
      expect(err.message).toContain('services');
    }
  });

  it('should throw NEURON-REG-005 when a neuron-topic is missing required fields', () => {
    const json = JSON.stringify({
      services: [
        {
          type: 'neuron-topic',
          name: 'stdIn',
          // Missing version, channel, transport, anchor, config
        },
      ],
    });

    expect(() => parseAgentURI(json)).toThrow(RegistryError);
  });

  it('should throw NEURON-REG-005 when a neuron-p2p-exchange is missing required fields', () => {
    const json = JSON.stringify({
      services: [
        {
          type: 'neuron-p2p-exchange',
          name: 'p2p',
          // Missing version, peerID, protocol, topicRef
        },
      ],
    });

    expect(() => parseAgentURI(json)).toThrow(RegistryError);
  });

  it('should throw NEURON-REG-005 when a service entry is not an object', () => {
    const json = JSON.stringify({ services: [42] });

    expect(() => parseAgentURI(json)).toThrow(RegistryError);
  });
});

// ---------------------------------------------------------------------------
// serializeAgentURI
// ---------------------------------------------------------------------------

describe('serializeAgentURI()', () => {
  it('should produce valid JSON', () => {
    const uri = buildAgentURI(
      [makeTopicService('stdIn'), makeTopicService('stdOut'), makeTopicService('stdErr')],
      [makeP2PService()],
    );

    const json = serializeAgentURI(uri);

    const parsed = JSON.parse(json) as Record<string, unknown>;
    expect(parsed['services']).toBeDefined();
    expect(Array.isArray(parsed['services'])).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Round-trip guarantee (SC-R01)
// ---------------------------------------------------------------------------

describe('round-trip: serialize then parse', () => {
  it('should produce identical AgentURI after round-trip', () => {
    const original = buildAgentURI(
      [makeTopicService('stdIn'), makeTopicService('stdOut'), makeTopicService('stdErr')],
      [makeP2PService()],
      makeDIDService(),
    );

    const json = serializeAgentURI(original);
    const parsed = parseAgentURI(json);

    expect(parsed.services).toHaveLength(original.services.length);

    // Verify each service matches
    for (let i = 0; i < original.services.length; i++) {
      expect(parsed.services[i]!.type).toBe(original.services[i]!.type);
      expect(parsed.services[i]!.name).toBe(original.services[i]!.name);
    }
  });

  it('should preserve neuron-topic config through round-trip', () => {
    const topic = makeTopicService('stdIn');
    const uri = buildAgentURI([topic, makeTopicService('stdOut'), makeTopicService('stdErr')], [makeP2PService()]);

    const json = serializeAgentURI(uri);
    const parsed = parseAgentURI(json);

    const parsedTopic = parsed.services[0] as NeuronTopicServiceEntry;
    expect(parsedTopic.config).toEqual(topic.config);
  });

  it('should preserve P2P service fields through round-trip', () => {
    const p2p = makeP2PService();
    const uri = buildAgentURI(
      [makeTopicService('stdIn'), makeTopicService('stdOut'), makeTopicService('stdErr')],
      [p2p],
    );

    const json = serializeAgentURI(uri);
    const parsed = parseAgentURI(json);

    const parsedP2P = parsed.services.find(s => s.type === 'neuron-p2p-exchange') as NeuronP2PExchangeEntry;
    expect(parsedP2P.peerID).toBe(p2p.peerID);
    expect(parsedP2P.protocol).toBe(p2p.protocol);
    expect(parsedP2P.topicRef).toBe(p2p.topicRef);
  });

  it('should preserve DID service through round-trip', () => {
    const did = makeDIDService();
    const uri = buildAgentURI(
      [makeTopicService('stdIn'), makeTopicService('stdOut'), makeTopicService('stdErr')],
      [makeP2PService()],
      did,
    );

    const json = serializeAgentURI(uri);
    const parsed = parseAgentURI(json);

    const parsedDID = parsed.services.find(s => s.type === 'DID') as DIDServiceEntry;
    expect(parsedDID.endpoint).toBe(did.endpoint);
    expect(parsedDID.version).toBe(did.version);
  });
});
