/**
 * Tests for the IIdentityRegistry interface and event types.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-02: register() mints NFT. registerWithProof() for permissioned.
 *   - FR-C-03: agentURI(tokenId) reads stored URI.
 *   - FR-C-04: IdentityRegistered event.
 *   - FR-C-05: IdentityUpdated, IdentityRevoked events.
 *   - FR-C-07: updateAgentURI() by owner or approved operator.
 *   - FR-C-08: revoke() burns token, owner only.
 *   - FR-C-10: lookup(address) returns (tokenId, agentURI) or (0, "").
 *   - FR-C-12: admissionPolicy() returns current policy address.
 *   - FR-C-13: setAdmissionPolicy() admin only. AdmissionPolicyUpdated event.
 *   - FR-C-16: Re-registration after revocation with new tokenId.
 *
 * Tests use a mock implementation to verify the interface contract compiles
 * and behaves correctly at the type level.
 */

import { describe, it, expect } from 'vitest';
import type {
  IIdentityRegistry,
  IdentityRegisteredEvent,
  IdentityUpdatedEvent,
  IdentityRevokedEvent,
  AdmissionPolicyUpdatedEvent,
} from '../../src/contracts/identity-registry.js';
import type { Address, Uint256 } from '../../src/contracts/types.js';

// ---------------------------------------------------------------------------
// Mock Implementation
// ---------------------------------------------------------------------------

/**
 * Minimal mock that satisfies IIdentityRegistry.
 * Simulates a single-agent registry for testing interface contracts.
 */
function createMockIdentityRegistry(): IIdentityRegistry {
  let nextTokenId = 1n;
  const registrations = new Map<Address, { tokenId: Uint256; agentURI: string }>();
  const tokenOwners = new Map<Uint256, Address>();
  let currentPolicy: Address = '0x0000000000000000000000000000000000000000';

  return {
    async register(agentURI: string): Promise<Uint256> {
      const tokenId = nextTokenId++;
      const owner = '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B';
      registrations.set(owner, { tokenId, agentURI });
      tokenOwners.set(tokenId, owner);
      return tokenId;
    },

    async registerWithProof(agentURI: string, _parentDIDProof: Uint8Array): Promise<Uint256> {
      const tokenId = nextTokenId++;
      const owner = '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B';
      registrations.set(owner, { tokenId, agentURI });
      tokenOwners.set(tokenId, owner);
      return tokenId;
    },

    async updateAgentURI(tokenId: Uint256, newAgentURI: string): Promise<void> {
      const owner = tokenOwners.get(tokenId);
      if (owner !== undefined) {
        registrations.set(owner, { tokenId, agentURI: newAgentURI });
      }
    },

    async revoke(tokenId: Uint256): Promise<void> {
      const owner = tokenOwners.get(tokenId);
      if (owner !== undefined) {
        registrations.delete(owner);
        tokenOwners.delete(tokenId);
      }
    },

    async agentURI(tokenId: Uint256): Promise<string> {
      const owner = tokenOwners.get(tokenId);
      if (owner !== undefined) {
        const reg = registrations.get(owner);
        if (reg !== undefined) {
          return reg.agentURI;
        }
      }
      return '';
    },

    async lookup(address: Address): Promise<{ readonly tokenId: Uint256; readonly agentURI: string }> {
      const reg = registrations.get(address);
      if (reg !== undefined) {
        return { tokenId: reg.tokenId, agentURI: reg.agentURI };
      }
      return { tokenId: 0n, agentURI: '' };
    },

    async ownerOf(tokenId: Uint256): Promise<Address> {
      return tokenOwners.get(tokenId) ?? '0x0000000000000000000000000000000000000000';
    },

    async setAdmissionPolicy(newPolicy: Address): Promise<void> {
      currentPolicy = newPolicy;
    },

    async admissionPolicy(): Promise<Address> {
      return currentPolicy;
    },
  };
}

// ---------------------------------------------------------------------------
// Interface Contract Tests
// ---------------------------------------------------------------------------

describe('IIdentityRegistry', () => {
  it('should register and return a tokenId (FR-C-02)', async () => {
    const registry = createMockIdentityRegistry();
    const tokenId = await registry.register('https://example.com/agent.json');

    expect(tokenId).toBe(1n);
    expect(typeof tokenId).toBe('bigint');
  });

  it('should register with proof and return a tokenId (FR-C-02, FR-C-12)', async () => {
    const registry = createMockIdentityRegistry();
    const proof = new Uint8Array([1, 2, 3, 4]);
    const tokenId = await registry.registerWithProof('https://example.com/agent.json', proof);

    expect(tokenId).toBe(1n);
  });

  it('should increment tokenId for successive registrations (FR-C-02)', async () => {
    const registry = createMockIdentityRegistry();
    const id1 = await registry.register('uri-1');
    const id2 = await registry.register('uri-2');

    expect(id2).toBeGreaterThan(id1);
  });

  it('should read agentURI after registration (FR-C-03)', async () => {
    const registry = createMockIdentityRegistry();
    const tokenId = await registry.register('https://example.com/agent.json');
    const uri = await registry.agentURI(tokenId);

    expect(uri).toBe('https://example.com/agent.json');
  });

  it('should update agentURI (FR-C-05, FR-C-07)', async () => {
    const registry = createMockIdentityRegistry();
    const tokenId = await registry.register('https://example.com/v1.json');

    await registry.updateAgentURI(tokenId, 'https://example.com/v2.json');
    const uri = await registry.agentURI(tokenId);

    expect(uri).toBe('https://example.com/v2.json');
  });

  it('should revoke registration (FR-C-05, FR-C-08)', async () => {
    const registry = createMockIdentityRegistry();
    const tokenId = await registry.register('https://example.com/agent.json');

    await registry.revoke(tokenId);

    const owner = await registry.ownerOf(tokenId);
    expect(owner).toBe('0x0000000000000000000000000000000000000000');
  });

  it('should lookup by address (FR-C-10)', async () => {
    const registry = createMockIdentityRegistry();
    await registry.register('https://example.com/agent.json');

    const result = await registry.lookup('0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B');
    expect(result.tokenId).toBe(1n);
    expect(result.agentURI).toBe('https://example.com/agent.json');
  });

  it('should return (0, "") for unregistered address (FR-C-10)', async () => {
    const registry = createMockIdentityRegistry();

    const result = await registry.lookup('0x0000000000000000000000000000000000000099');
    expect(result.tokenId).toBe(0n);
    expect(result.agentURI).toBe('');
  });

  it('should return owner address for valid token (ERC-721 ownerOf)', async () => {
    const registry = createMockIdentityRegistry();
    const tokenId = await registry.register('uri');

    const owner = await registry.ownerOf(tokenId);
    expect(owner).toBe('0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B');
  });

  it('should set and read admission policy (FR-C-12, FR-C-13)', async () => {
    const registry = createMockIdentityRegistry();

    // Default is permissionless (address(0))
    const defaultPolicy = await registry.admissionPolicy();
    expect(defaultPolicy).toBe('0x0000000000000000000000000000000000000000');

    // Set custom policy
    await registry.setAdmissionPolicy('0x1234567890123456789012345678901234567890');
    const newPolicy = await registry.admissionPolicy();
    expect(newPolicy).toBe('0x1234567890123456789012345678901234567890');
  });

  it('should re-register after revocation with new tokenId (FR-C-16)', async () => {
    const registry = createMockIdentityRegistry();
    const id1 = await registry.register('uri-v1');
    await registry.revoke(id1);

    const id2 = await registry.register('uri-v2');
    expect(id2).toBeGreaterThan(id1);
  });
});

// ---------------------------------------------------------------------------
// Event Type Shape Tests
// ---------------------------------------------------------------------------

describe('IdentityRegisteredEvent', () => {
  it('should have correct shape (FR-C-04)', () => {
    const event: IdentityRegisteredEvent = {
      tokenId: 1n,
      owner: '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      agentURI: 'https://example.com/agent.json',
    };

    expect(event.tokenId).toBe(1n);
    expect(event.owner).toMatch(/^0x/);
    expect(event.agentURI).toContain('agent.json');
  });
});

describe('IdentityUpdatedEvent', () => {
  it('should have correct shape (FR-C-05)', () => {
    const event: IdentityUpdatedEvent = {
      tokenId: 1n,
      newAgentURI: 'https://example.com/agent-v2.json',
    };

    expect(event.tokenId).toBe(1n);
    expect(event.newAgentURI).toContain('v2');
  });
});

describe('IdentityRevokedEvent', () => {
  it('should have correct shape (FR-C-05)', () => {
    const event: IdentityRevokedEvent = {
      tokenId: 1n,
      owner: '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
    };

    expect(event.tokenId).toBe(1n);
    expect(event.owner).toMatch(/^0x/);
  });
});

describe('AdmissionPolicyUpdatedEvent', () => {
  it('should have correct shape (FR-C-13)', () => {
    const event: AdmissionPolicyUpdatedEvent = {
      oldPolicy: '0x0000000000000000000000000000000000000000',
      newPolicy: '0x1234567890123456789012345678901234567890',
    };

    expect(event.oldPolicy).toBe('0x0000000000000000000000000000000000000000');
    expect(event.newPolicy).toMatch(/^0x/);
    expect(event.newPolicy).not.toBe(event.oldPolicy);
  });
});
