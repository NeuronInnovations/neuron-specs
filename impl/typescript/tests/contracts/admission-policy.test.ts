/**
 * Tests for the IAdmissionPolicy interface and PERMISSIONLESS_POLICY constant.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-12: Pluggable admission via IAdmissionPolicy. address(0) = permissionless.
 *   - FR-C-13: setAdmissionPolicy() updates policy. Owner only.
 *   - FR-C-14: isAdmitted(childAddress, parentDIDProof) -> bool.
 *   - FR-C-15: Removing Parent DID does NOT auto-revoke existing registrations.
 *   - DD-02: Strategy pattern. Start permissionless, add restrictions later.
 *
 * Tests use mock implementations to verify the interface contract compiles
 * and behaves correctly at the type level.
 */

import { describe, it, expect } from 'vitest';
import type { IAdmissionPolicy } from '../../src/contracts/admission-policy.js';
import { PERMISSIONLESS_POLICY } from '../../src/contracts/admission-policy.js';
import type { Address } from '../../src/contracts/types.js';

// ---------------------------------------------------------------------------
// Mock Implementations
// ---------------------------------------------------------------------------

/**
 * Permissionless policy mock -- always admits. FR-C-12
 */
function createPermissionlessPolicy(): IAdmissionPolicy {
  return {
    async isAdmitted(_childAddress: Address, _parentDIDProof: Uint8Array): Promise<boolean> {
      return true;
    },
  };
}

/**
 * Allowlist policy mock -- admits only if parentDIDProof hash is in the set.
 * FR-C-14, FR-C-15, Appendix B AllowlistPolicy.
 */
function createAllowlistPolicy(allowedProofs: ReadonlyArray<string>): IAdmissionPolicy {
  const allowed = new Set(allowedProofs);

  return {
    async isAdmitted(_childAddress: Address, parentDIDProof: Uint8Array): Promise<boolean> {
      // Simple mock: use the proof as a UTF-8 string for lookup
      const proofStr = new TextDecoder().decode(parentDIDProof);
      return allowed.has(proofStr);
    },
  };
}

/**
 * Deny-all policy mock -- rejects everyone. Useful for testing restrictions.
 */
function createDenyAllPolicy(): IAdmissionPolicy {
  return {
    async isAdmitted(_childAddress: Address, _parentDIDProof: Uint8Array): Promise<boolean> {
      return false;
    },
  };
}

// ---------------------------------------------------------------------------
// PERMISSIONLESS_POLICY Constant Tests
// ---------------------------------------------------------------------------

describe('PERMISSIONLESS_POLICY', () => {
  it('should be the zero address (FR-C-12)', () => {
    expect(PERMISSIONLESS_POLICY).toBe('0x0000000000000000000000000000000000000000');
  });

  it('should be a 42-character hex string', () => {
    expect(PERMISSIONLESS_POLICY).toMatch(/^0x[0-9a-fA-F]{40}$/);
  });

  it('should equal address(0) -- the sentinel for permissionless mode', () => {
    // All characters after 0x should be zero
    const hexPart = PERMISSIONLESS_POLICY.slice(2);
    expect(hexPart).toBe('0'.repeat(40));
  });
});

// ---------------------------------------------------------------------------
// IAdmissionPolicy Interface Tests
// ---------------------------------------------------------------------------

describe('IAdmissionPolicy -- Permissionless', () => {
  it('should always admit any address with any proof (FR-C-12)', async () => {
    const policy = createPermissionlessPolicy();

    const result1 = await policy.isAdmitted(
      '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      new Uint8Array(0),
    );
    expect(result1).toBe(true);

    const result2 = await policy.isAdmitted(
      '0x0000000000000000000000000000000000000001',
      new Uint8Array([1, 2, 3]),
    );
    expect(result2).toBe(true);
  });

  it('should admit with empty parentDIDProof (FR-C-14)', async () => {
    const policy = createPermissionlessPolicy();

    const result = await policy.isAdmitted(
      '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      new Uint8Array(0),
    );
    expect(result).toBe(true);
  });
});

describe('IAdmissionPolicy -- Allowlist', () => {
  it('should admit a Child with allowed parentDIDProof (FR-C-14)', async () => {
    const policy = createAllowlistPolicy(['parent-did-alice', 'parent-did-bob']);

    const result = await policy.isAdmitted(
      '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      new TextEncoder().encode('parent-did-alice'),
    );
    expect(result).toBe(true);
  });

  it('should reject a Child with unlisted parentDIDProof (FR-C-14)', async () => {
    const policy = createAllowlistPolicy(['parent-did-alice']);

    const result = await policy.isAdmitted(
      '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      new TextEncoder().encode('parent-did-charlie'),
    );
    expect(result).toBe(false);
  });

  it('should reject with empty proof when proof is required (FR-C-14)', async () => {
    const policy = createAllowlistPolicy(['parent-did-alice']);

    const result = await policy.isAdmitted(
      '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      new Uint8Array(0),
    );
    expect(result).toBe(false);
  });
});

describe('IAdmissionPolicy -- DenyAll', () => {
  it('should reject all addresses', async () => {
    const policy = createDenyAllPolicy();

    const result = await policy.isAdmitted(
      '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      new TextEncoder().encode('valid-proof'),
    );
    expect(result).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Strategy Pattern Tests (DD-02)
// ---------------------------------------------------------------------------

describe('Admission Policy Strategy Pattern (DD-02)', () => {
  it('should allow switching policies at runtime', async () => {
    // Start with permissionless
    let policy: IAdmissionPolicy = createPermissionlessPolicy();
    const childAddr = '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B';
    const proof = new TextEncoder().encode('parent-did-alice');

    expect(await policy.isAdmitted(childAddr, proof)).toBe(true);

    // Switch to allowlist that does NOT include alice
    policy = createAllowlistPolicy(['parent-did-bob']);
    expect(await policy.isAdmitted(childAddr, proof)).toBe(false);

    // Switch to allowlist that includes alice
    policy = createAllowlistPolicy(['parent-did-alice']);
    expect(await policy.isAdmitted(childAddr, proof)).toBe(true);
  });

  it('should compare policy address to PERMISSIONLESS_POLICY sentinel', () => {
    // Simulate the Identity Registry's admission check logic
    const policyAddress: Address = PERMISSIONLESS_POLICY;
    const isPermissionless = policyAddress === PERMISSIONLESS_POLICY;
    expect(isPermissionless).toBe(true);

    const customPolicyAddress: Address = '0x1234567890123456789012345678901234567890';
    const isCustom = customPolicyAddress !== PERMISSIONLESS_POLICY;
    expect(isCustom).toBe(true);
  });
});
