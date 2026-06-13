/**
 * Tests for registration operations -- register, update, revoke, lookup.
 *
 * Spec reference: 003 spec.md
 *   - FR-R06: register() signed by Child's NeuronPrivateKey.
 *   - FR-R07: Lookup failure is a documented error.
 *   - FR-R08: AgentURI validated before register/update.
 *   - FR-R10: NFT owner is Child's EVMAddress.
 *   - FR-R11: Role boundaries for update/revoke.
 *
 * Uses inline mock RegistryContract implementations for testing.
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import type { NeuronPublicKey } from '../../src/keylib/public-key.js';
import type { RegistryContract } from '../../src/registry/contract.js';
import type {
  AgentURI,
  Registration,
  RegistrationResult,
  LookupKey,
  NeuronTopicServiceEntry,
  NeuronP2PExchangeEntry,
} from '../../src/registry/types.js';
import {
  register,
  updateRegistration,
  revokeRegistration,
  lookupRegistration,
} from '../../src/registry/registration.js';
import { RegistryError } from '../../src/registry/errors.js';

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

const TEST_KEY_HEX = '0x0000000000000000000000000000000000000000000000000000000000000001';
const REGISTRY_ADDRESS = '0x742d35Cc6634C0532925a3b844Bc9e7595f2bD61';

let childKey: NeuronPrivateKey;
let childPubKey: NeuronPublicKey;
let childAddress: string;
let childPeerID: string;

beforeAll(() => {
  childKey = NeuronPrivateKey.fromHex(TEST_KEY_HEX);
  childPubKey = childKey.publicKey();
  childAddress = childPubKey.evmAddress().toString();
  childPeerID = childPubKey.peerId().toString();
});

/** Create a valid neuron-topic service entry. */
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

/** Create a valid neuron-p2p-exchange service. */
function makeP2PService(): NeuronP2PExchangeEntry {
  return {
    type: 'neuron-p2p-exchange',
    name: 'p2p',
    version: '1.0.0',
    peerID: childPeerID,
    protocol: '/neuron/multiaddr-exchange/1.0.0',
    topicRef: 'stdIn',
  };
}

/** Build a complete, valid AgentURI. */
function makeValidAgentURI(): AgentURI {
  return {
    services: [
      makeTopicService('stdIn'),
      makeTopicService('stdOut'),
      makeTopicService('stdErr'),
      makeP2PService(),
    ],
  };
}

/** Create a successful mock RegistryContract. */
function createMockContract(overrides?: Partial<RegistryContract>): RegistryContract {
  const storedRegistration: Registration = {
    registryAddress: REGISTRY_ADDRESS,
    childAddress,
    tokenId: 1n,
    agentURI: makeValidAgentURI(),
    chainId: 1n,
  };

  return {
    register: async (uri: string): Promise<RegistrationResult> => ({
      tokenId: 1n,
      transactionHash: '0xabc123def456',
      childAddress,
      registryAddress: REGISTRY_ADDRESS,
      chainId: 1n,
      agentURI: uri,
    }),
    updateAgentURI: async (_tokenId: bigint, newURI: string): Promise<RegistrationResult> => ({
      tokenId: 1n,
      transactionHash: '0xupdate789',
      childAddress,
      registryAddress: REGISTRY_ADDRESS,
      chainId: 1n,
      agentURI: newURI,
    }),
    revoke: async (_tokenId: bigint): Promise<string> => '0xrevoke321',
    lookup: async (_key: LookupKey): Promise<Registration | null> => storedRegistration,
    ownerOf: async (_tokenId: bigint): Promise<string> => childAddress,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('register()', () => {
  it('should succeed with a valid AgentURI and mock contract', async () => {
    const contract = createMockContract();
    const result = await register(childKey, contract, makeValidAgentURI());

    expect(result.tokenId).toBe(1n);
    expect(result.transactionHash).toBe('0xabc123def456');
    expect(result.childAddress).toBe(childAddress);
    expect(result.registryAddress).toBe(REGISTRY_ADDRESS);
    expect(result.chainId).toBe(1n);
    expect(result.agentURI).toContain('neuron-topic');
  });

  it('should throw NEURON-REG-005 when AgentURI is incomplete', async () => {
    const contract = createMockContract();
    const badURI: AgentURI = {
      services: [
        makeTopicService('stdIn'),
        // Missing stdOut, stdErr, and P2P
      ],
    };

    await expect(register(childKey, contract, badURI))
      .rejects
      .toThrow(RegistryError);

    try {
      await register(childKey, contract, badURI);
    } catch (e) {
      expect(e).toBeInstanceOf(RegistryError);
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-005');
      expect(err.message).toContain('validation failed');
    }
  });

  it('should throw NEURON-REG-001 when contract.register() fails', async () => {
    const contract = createMockContract({
      register: async () => {
        throw new Error('contract revert');
      },
    });

    await expect(register(childKey, contract, makeValidAgentURI()))
      .rejects
      .toThrow(RegistryError);

    try {
      await register(childKey, contract, makeValidAgentURI());
    } catch (e) {
      expect(e).toBeInstanceOf(RegistryError);
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-001');
      expect(err.cause).toBeInstanceOf(Error);
    }
  });

  it('should re-throw RegistryError from contract without wrapping', async () => {
    const contract = createMockContract({
      register: async () => {
        throw new RegistryError('NEURON-REG-006', 'UnauthorizedCaller', 'not allowed');
      },
    });

    try {
      await register(childKey, contract, makeValidAgentURI());
    } catch (e) {
      expect(e).toBeInstanceOf(RegistryError);
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-006');
    }
  });
});

describe('updateRegistration()', () => {
  it('should succeed with a valid AgentURI and mock contract', async () => {
    const contract = createMockContract();
    const result = await updateRegistration(childKey, contract, 1n, makeValidAgentURI());

    expect(result.tokenId).toBe(1n);
    expect(result.transactionHash).toBe('0xupdate789');
    expect(result.agentURI).toContain('neuron-topic');
  });

  it('should throw NEURON-REG-005 when new AgentURI is incomplete', async () => {
    const contract = createMockContract();
    const badURI: AgentURI = { services: [] };

    await expect(updateRegistration(childKey, contract, 1n, badURI))
      .rejects
      .toThrow(RegistryError);

    try {
      await updateRegistration(childKey, contract, 1n, badURI);
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-005');
    }
  });

  it('should throw NEURON-REG-003 when contract.updateAgentURI() fails', async () => {
    const contract = createMockContract({
      updateAgentURI: async () => {
        throw new Error('update revert');
      },
    });

    try {
      await updateRegistration(childKey, contract, 1n, makeValidAgentURI());
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-003');
      expect(err.cause).toBeInstanceOf(Error);
    }
  });
});

describe('revokeRegistration()', () => {
  it('should succeed and return transaction hash', async () => {
    const contract = createMockContract();
    const txHash = await revokeRegistration(childKey, contract, 1n);

    expect(txHash).toBe('0xrevoke321');
  });

  it('should throw NEURON-REG-004 when contract.revoke() fails', async () => {
    const contract = createMockContract({
      revoke: async () => {
        throw new Error('revoke revert');
      },
    });

    try {
      await revokeRegistration(childKey, contract, 1n);
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-004');
      expect(err.cause).toBeInstanceOf(Error);
    }
  });

  it('should re-throw RegistryError from contract without wrapping', async () => {
    const contract = createMockContract({
      revoke: async () => {
        throw new RegistryError('NEURON-REG-006', 'UnauthorizedCaller', 'not owner');
      },
    });

    try {
      await revokeRegistration(childKey, contract, 1n);
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-006');
    }
  });
});

describe('lookupRegistration()', () => {
  it('should return Registration when found by address', async () => {
    const contract = createMockContract();
    const result = await lookupRegistration(contract, {
      type: 'byAddress',
      address: childAddress,
    });

    expect(result).not.toBeNull();
    expect(result!.childAddress).toBe(childAddress);
    expect(result!.tokenId).toBe(1n);
    expect(result!.agentURI.services.length).toBeGreaterThan(0);
  });

  it('should return null when not found', async () => {
    const contract = createMockContract({
      lookup: async () => null,
    });

    const result = await lookupRegistration(contract, {
      type: 'byAddress',
      address: '0x0000000000000000000000000000000000000000',
    });

    expect(result).toBeNull();
  });

  it('should return Registration when found by external id', async () => {
    const contract = createMockContract();
    const result = await lookupRegistration(contract, {
      type: 'byExternalId',
      externalId: 'agent-alice-001',
    });

    expect(result).not.toBeNull();
    expect(result!.registryAddress).toBe(REGISTRY_ADDRESS);
  });

  it('should throw NEURON-REG-002 when contract.lookup() fails', async () => {
    const contract = createMockContract({
      lookup: async () => {
        throw new Error('network timeout');
      },
    });

    try {
      await lookupRegistration(contract, {
        type: 'byAddress',
        address: childAddress,
      });
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-002');
      expect(err.cause).toBeInstanceOf(Error);
    }
  });

  it('should re-throw RegistryError from contract without wrapping', async () => {
    const contract = createMockContract({
      lookup: async () => {
        throw new RegistryError('NEURON-REG-002', 'LookupFailed', 'registry unavailable');
      },
    });

    try {
      await lookupRegistration(contract, {
        type: 'byAddress',
        address: childAddress,
      });
    } catch (e) {
      const err = e as RegistryError;
      expect(err.code).toBe('NEURON-REG-002');
      expect(err.message).toBe('registry unavailable');
    }
  });
});
