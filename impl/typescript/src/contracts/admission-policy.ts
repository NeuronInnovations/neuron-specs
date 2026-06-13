/**
 * IAdmissionPolicy -- pluggable admission policy for Identity Registry.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-12: Pluggable admission via IAdmissionPolicy interface.
 *              address(0) = permissionless mode (no admission check).
 *   - FR-C-13: setAdmissionPolicy() updates the policy. Owner only.
 *   - FR-C-14: isAdmitted(childAddress, parentDIDProof) -> bool.
 *              parentDIDProof is opaque bytes; format defined by implementation.
 *   - FR-C-15: Removing a Parent DID from an allowlist does NOT auto-revoke
 *              existing registrations.
 *   - DD-02: Strategy pattern for admission control.
 *
 * Two informative examples from spec appendix:
 *   - PermissionlessPolicy: always returns true (address(0) sentinel).
 *   - AllowlistPolicy: admits Children whose Parent DID hash is on the allowlist.
 */

import type { Address } from './types.js';

// ---------------------------------------------------------------------------
// Interface
// ---------------------------------------------------------------------------

/**
 * Pluggable admission policy interface.
 *
 * FR-C-14: Defines whether a Child address is admitted to register in a
 * permissioned Identity Registry. The parentDIDProof format is opaque --
 * each policy implementation defines its own proof structure.
 */
export interface IAdmissionPolicy {
  /**
   * Check whether a Child address is admitted to register.
   *
   * FR-C-14: parentDIDProof is opaque bytes. For permissionless policies,
   *          the proof may be empty (new Uint8Array(0)).
   * FR-C-12: When the Identity Registry's admission policy is address(0),
   *          this method is never called (permissionless mode).
   *
   * @param childAddress - The Child's EVM address attempting to register.
   * @param parentDIDProof - Opaque proof bytes for admission verification.
   * @returns true if the Child is admitted, false otherwise.
   */
  isAdmitted(childAddress: Address, parentDIDProof: Uint8Array): Promise<boolean>;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/**
 * Sentinel address for permissionless mode.
 *
 * FR-C-12: When the Identity Registry's admission policy address is set to
 * address(0), the registry operates in permissionless mode -- any address
 * satisfying proof-of-control may register without an admission check.
 *
 * DD-02: Platforms can start permissionless and later set a custom policy
 * without redeploying the registry contract.
 */
export const PERMISSIONLESS_POLICY: Address = '0x0000000000000000000000000000000000000000';
