/**
 * Tests for registry types -- type construction and structural validation.
 *
 * Spec reference: 003 data-model.md
 *   - Registration, AgentURI, NeuronTopicServiceEntry, NeuronP2PExchangeEntry,
 *     DIDServiceEntry, RegistrationResult, LookupKey.
 *
 * These tests verify that the type interfaces accept well-formed data
 * and that the discriminated union LookupKey works correctly.
 */

import { describe, it, expect } from 'vitest';
import type {
  Registration,
  AgentURI,
  NeuronTopicServiceEntry,
  NeuronP2PExchangeEntry,
  DIDServiceEntry,
  AgentURIService,
  RegistrationResult,
  LookupKey,
} from '../../src/registry/types.js';

describe('NeuronTopicServiceEntry', () => {
  it('should construct a valid neuron-topic service', () => {
    const svc: NeuronTopicServiceEntry = {
      type: 'neuron-topic',
      name: 'stdIn',
      version: '1.0.0',
      channel: 'stdIn',
      transport: 'hcs',
      anchor: 'hedera-mainnet',
      config: { network: 'hedera-mainnet', topicId: '0.0.4515382' },
    };

    expect(svc.type).toBe('neuron-topic');
    expect(svc.name).toBe('stdIn');
    expect(svc.channel).toBe('stdIn');
    expect(svc.transport).toBe('hcs');
    expect(svc.endpoint).toBeUndefined();
  });

  it('should accept optional endpoint field', () => {
    const svc: NeuronTopicServiceEntry = {
      type: 'neuron-topic',
      name: 'stdOut',
      version: '1.0.0',
      channel: 'stdOut',
      transport: 'hcs',
      anchor: 'hedera-mainnet',
      config: { network: 'hedera-mainnet', topicId: '0.0.4515383' },
      endpoint: 'hcs://0.0.4515383',
    };

    expect(svc.endpoint).toBe('hcs://0.0.4515383');
  });
});

describe('NeuronP2PExchangeEntry', () => {
  it('should construct a valid neuron-p2p-exchange service', () => {
    const svc: NeuronP2PExchangeEntry = {
      type: 'neuron-p2p-exchange',
      name: 'p2p',
      version: '1.0.0',
      peerID: '12D3KooWTestPeerID',
      protocol: '/neuron/multiaddr-exchange/1.0.0',
      topicRef: 'stdIn',
    };

    expect(svc.type).toBe('neuron-p2p-exchange');
    expect(svc.peerID).toBe('12D3KooWTestPeerID');
    expect(svc.topicRef).toBe('stdIn');
  });
});

describe('DIDServiceEntry', () => {
  it('should construct a valid DID service', () => {
    const svc: DIDServiceEntry = {
      type: 'DID',
      name: 'DID',
      endpoint: 'did:key:zQ3shP2mWsZYWgpKDXRRx8rBe6UaDQY4mJgZrm5KywKgjqiU9',
      version: 'v1',
    };

    expect(svc.type).toBe('DID');
    expect(svc.name).toBe('DID');
    expect(svc.endpoint).toContain('did:key:zQ3s');
  });
});

describe('AgentURIService discriminated union', () => {
  it('should discriminate neuron-topic by type field', () => {
    const svc: AgentURIService = {
      type: 'neuron-topic',
      name: 'stdIn',
      version: '1.0.0',
      channel: 'stdIn',
      transport: 'hcs',
      anchor: 'hedera-mainnet',
      config: {},
    };

    if (svc.type === 'neuron-topic') {
      // Type narrowing should give us NeuronTopicServiceEntry
      expect(svc.channel).toBe('stdIn');
      expect(svc.transport).toBe('hcs');
    }
  });

  it('should discriminate neuron-p2p-exchange by type field', () => {
    const svc: AgentURIService = {
      type: 'neuron-p2p-exchange',
      name: 'p2p',
      version: '1.0.0',
      peerID: '12D3KooWTest',
      protocol: '/neuron/multiaddr-exchange/1.0.0',
      topicRef: 'stdIn',
    };

    if (svc.type === 'neuron-p2p-exchange') {
      expect(svc.peerID).toBe('12D3KooWTest');
      expect(svc.topicRef).toBe('stdIn');
    }
  });

  it('should discriminate DID by type field', () => {
    const svc: AgentURIService = {
      type: 'DID',
      name: 'DID',
      endpoint: 'did:key:zQ3sTest',
      version: 'v1',
    };

    if (svc.type === 'DID') {
      expect(svc.endpoint).toBe('did:key:zQ3sTest');
    }
  });
});

describe('AgentURI', () => {
  it('should construct with a services array', () => {
    const uri: AgentURI = {
      services: [
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
    };

    expect(uri.services).toHaveLength(1);
    expect(uri.services[0]!.type).toBe('neuron-topic');
  });
});

describe('Registration', () => {
  it('should construct with all required fields', () => {
    const reg: Registration = {
      registryAddress: '0x742d35Cc6634C0532925a3b844Bc9e7595f2bD61',
      childAddress: '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      tokenId: 42n,
      agentURI: { services: [] },
      chainId: 1n,
    };

    expect(reg.tokenId).toBe(42n);
    expect(reg.chainId).toBe(1n);
    expect(typeof reg.registryAddress).toBe('string');
  });
});

describe('RegistrationResult', () => {
  it('should construct with all required fields', () => {
    const result: RegistrationResult = {
      tokenId: 1n,
      transactionHash: '0xabc123',
      childAddress: '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      registryAddress: '0x742d35Cc6634C0532925a3b844Bc9e7595f2bD61',
      chainId: 1n,
      agentURI: '{"services":[]}',
    };

    expect(result.tokenId).toBe(1n);
    expect(result.transactionHash).toBe('0xabc123');
    expect(result.agentURI).toBe('{"services":[]}');
  });
});

describe('LookupKey discriminated union', () => {
  it('should support byAddress lookup', () => {
    const key: LookupKey = {
      type: 'byAddress',
      address: '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
    };

    expect(key.type).toBe('byAddress');
    if (key.type === 'byAddress') {
      expect(key.address).toContain('0x');
    }
  });

  it('should support byExternalId lookup', () => {
    const key: LookupKey = {
      type: 'byExternalId',
      externalId: 'agent-alice-001',
    };

    expect(key.type).toBe('byExternalId');
    if (key.type === 'byExternalId') {
      expect(key.externalId).toBe('agent-alice-001');
    }
  });
});
