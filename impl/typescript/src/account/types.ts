/**
 * Core types for the NeuronAccount module.
 *
 * Spec reference: 001 spec.md
 *   - FR-013: AccountType enum with numeric discriminators (Parent=1, Child=2, Shared=3).
 *   - FR-012: NeuronDID identifier derived from Parent's public key (did:key: format).
 *   - FR-018: LedgerAttachment tracking with state and verification status.
 *   - FR-019: VerificationStatus and VerificationResult from injected verifiers.
 *   - FR-020: currencySymbol (string) on every account.
 *   - FR-022: RegistryBinding for Child accounts (mandatory for completeness).
 *   - FR-026: LedgerAccountId for fee payer on Child accounts.
 *
 * All types are immutable (readonly properties). Invalid inputs are rejected
 * at construction time via builder validation.
 */

import type { EVMAddress } from '../keylib/evm-address.js';
import { invalidDID } from './errors.js';

/**
 * FR-013: Three account types with numeric discriminators.
 *
 * Parent (1): Root identity with DID and credit balance.
 * Child (2): Agent identity with parent reference and registry binding.
 * Shared (3): Multi-signature account with threshold scheme.
 */
export enum AccountType {
  Parent = 1,
  Child = 2,
  Shared = 3,
}

/** The did:key: prefix that all Neuron DIDs must start with. */
const DID_KEY_PREFIX = 'did:key:';

/**
 * FR-012: DID:key identifier for Parent accounts.
 *
 * Generated from the Parent's NeuronPublicKey via spec 002 FR-006a.
 * The identifier MUST start with "did:key:" and contain a valid
 * base58btc-encoded multicodec secp256k1 public key.
 *
 * Immutable value type. Valid by construction.
 */
export interface NeuronDID {
  /** did:key:zQ3s... identifier. Validated to start with "did:key:". */
  readonly identifier: string;
}

/**
 * Create a NeuronDID from a did:key: string with format validation.
 *
 * FR-012: Validates the identifier starts with "did:key:" prefix.
 *
 * @param identifier - DID string in did:key:zQ3s... format
 * @returns NeuronDID instance
 * @throws AccountError NEURON-ACCT-004 if identifier does not start with "did:key:"
 */
export function createNeuronDID(identifier: string): NeuronDID {
  if (!identifier.startsWith(DID_KEY_PREFIX)) {
    throw invalidDID(
      `DID identifier must start with "${DID_KEY_PREFIX}", got: "${identifier.slice(0, 20)}"`,
    );
  }
  return { identifier };
}

/**
 * FR-018: Ledger attachment state.
 *
 * An account is either attached to a ledger (linked to an on-chain address)
 * or detached (not yet linked or intentionally unlinked).
 */
export type LedgerState = 'attached' | 'detached';

/**
 * FR-019: Verification status from ledger verifier.
 *
 * - verified: The attachment has been confirmed by the verifier.
 * - unverified: The attachment has not yet been verified.
 * - failed: Verification was attempted but failed.
 */
export type VerificationStatus = 'verified' | 'unverified' | 'failed';

/**
 * FR-018: Ledger attachment tracking.
 *
 * Represents the binding between a NeuronAccount and a ledger address.
 * Tracks the current state and verification status.
 *
 * All fields are readonly. Immutable after construction.
 */
export interface LedgerAttachment {
  /** Ledger identifier (e.g., "ethereum-mainnet", "hedera-mainnet"). */
  readonly ledgerIdentifier: string;
  /** The EVM address linking Neuron identity to the ledger. */
  readonly attachedAddress: EVMAddress;
  /** Current attachment state: attached or detached. */
  readonly state: LedgerState;
  /** Verification status from the ledger verifier. */
  readonly verificationStatus: VerificationStatus;
  /** Timestamp of last balance/state sync, or null if never synced. */
  readonly lastSyncedAt: Date | null;
}

/**
 * FR-022: Child registry binding (mandatory for completeness).
 *
 * Binds a Child account to an external registry entry. The registryIdentifier
 * identifies the registry, and externalId is the opaque ID within that registry.
 */
export interface RegistryBinding {
  /** Registry URI or identifier (e.g., EIP-8004 registry address). */
  readonly registryIdentifier: string;
  /** Opaque ID from the registry (e.g., agent ID or token ID). */
  readonly externalId: string;
}

/**
 * FR-026: Ledger-agnostic account ID for fee payer.
 *
 * An opaque string identifying the account that pays transaction fees
 * on behalf of a Child agent. Format is ledger-specific.
 */
export type LedgerAccountId = string;

/**
 * FR-017, FR-019: Verification result from injected verifiers.
 *
 * Discriminated union on the status field:
 * - verified: Verification succeeded.
 * - unverified: Verification could not be completed (with optional reason).
 * - failed: Verification was attempted and failed (with error description).
 */
export type VerificationResult =
  | { readonly status: 'verified' }
  | { readonly status: 'unverified'; readonly reason?: string }
  | { readonly status: 'failed'; readonly error: string };
