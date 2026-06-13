/**
 * NeuronAccount -- discriminated union type for agent identity.
 *
 * Spec reference: 001 spec.md
 *   - FR-001: Parent account with DID and single NeuronPublicKey.
 *   - FR-002: Child account with parent reference and own NeuronPublicKey.
 *   - FR-007a: Shared account with MultisigKey and threshold scheme.
 *   - FR-008: Derived fields evmAddress() and peerId() delegate to publicKey.
 *   - FR-013: AccountType discriminator (Parent=1, Child=2, Shared=3).
 *   - FR-016: Balance fields are optional (undefined until synced).
 *   - FR-018: LedgerAttachment is optional on all account types.
 *   - FR-022: All fields are readonly. Immutable after construction.
 *   - FR-026: FeePayer is optional on Child accounts.
 *
 * The NeuronAccount type is a discriminated union on accountType.
 * Type guards (isParent, isChild, isShared) narrow the union to
 * the specific variant for type-safe field access.
 */

import type { NeuronPublicKey } from '../keylib/public-key.js';
import type { EVMAddress } from '../keylib/evm-address.js';
import type { PeerID } from '../keylib/peer-id.js';
import type { MultisigKey } from '../keylib/multisig-key.js';
import type {
  NeuronDID,
  LedgerAttachment,
  RegistryBinding,
  LedgerAccountId,
} from './types.js';
import { AccountType } from './types.js';
import { invalidAccountType } from './errors.js';

/**
 * Base fields shared by all account types.
 *
 * FR-013: accountType is the discriminator.
 * FR-020: currencySymbol is required on all accounts.
 * FR-018: ledgerAttachment is optional on all accounts.
 */
export interface BaseAccount {
  /** FR-013: Account type discriminator. */
  readonly accountType: AccountType;
  /** FR-020: Currency symbol (e.g., "HBAR", "ETH"). */
  readonly currencySymbol: string;
  /** FR-018: Optional ledger attachment. */
  readonly ledgerAttachment?: LedgerAttachment | undefined;
}

/**
 * Parent account -- root identity with DID and credit balance.
 *
 * FR-001: Has a single NeuronPublicKey and a DID.
 * FR-006: Validation rules V-PARENT-01..05 apply.
 * FR-016: creditBalance is optional (undefined until synced from ledger).
 */
export interface ParentAccount extends BaseAccount {
  readonly accountType: AccountType.Parent;
  /** FR-001: Single secp256k1 public key. */
  readonly publicKey: NeuronPublicKey;
  /** FR-012: DID:key identifier derived from publicKey. */
  readonly did: NeuronDID;
  /** FR-016: Credit balance. Undefined until synced from ledger. */
  readonly creditBalance?: bigint | undefined;
}

/**
 * Child account -- agent identity with parent reference.
 *
 * FR-002: Has own NeuronPublicKey and a reference to Parent's publicKey.
 * FR-007: Validation rules V-CHILD-01..05 apply.
 * FR-016: balanceAllocation is optional (undefined until synced).
 * FR-022: registryBinding is mandatory for completeness.
 * FR-026: feePayer is optional.
 */
export interface ChildAccount extends BaseAccount {
  readonly accountType: AccountType.Child;
  /** FR-002: Single secp256k1 public key for this Child agent. */
  readonly publicKey: NeuronPublicKey;
  /** FR-007: Reference to the Parent's public key. */
  readonly parentPubKey: NeuronPublicKey;
  /** FR-016: Balance allocation from Parent. Undefined until synced. */
  readonly balanceAllocation?: bigint | undefined;
  /** FR-022: Registry binding. Mandatory for completeness (FR-011a). */
  readonly registryBinding?: RegistryBinding | undefined;
  /** FR-026: Fee payer account ID. Optional. */
  readonly feePayer?: LedgerAccountId | undefined;
}

/**
 * Shared account -- multi-signature account with threshold scheme.
 *
 * FR-007a: Has MultisigKey, no DID, no publicKey, no parent reference.
 * FR-021: balance is optional (undefined until synced).
 */
export interface SharedAccount extends BaseAccount {
  readonly accountType: AccountType.Shared;
  /** FR-007a: Multi-signature key configuration (m-of-n). */
  readonly multisigKey: MultisigKey;
  /** FR-021: Shared balance. Undefined until synced. */
  readonly balance?: bigint | undefined;
}

/**
 * Discriminated union of all account types.
 *
 * FR-013: Discriminated on accountType field.
 * Use type guards (isParent, isChild, isShared) to narrow.
 */
export type NeuronAccount = ParentAccount | ChildAccount | SharedAccount;

/**
 * Type guard: narrow NeuronAccount to ParentAccount.
 *
 * @param account - Account to check
 * @returns true if account is a Parent account
 */
export function isParent(account: NeuronAccount): account is ParentAccount {
  return account.accountType === AccountType.Parent;
}

/**
 * Type guard: narrow NeuronAccount to ChildAccount.
 *
 * @param account - Account to check
 * @returns true if account is a Child account
 */
export function isChild(account: NeuronAccount): account is ChildAccount {
  return account.accountType === AccountType.Child;
}

/**
 * Type guard: narrow NeuronAccount to SharedAccount.
 *
 * @param account - Account to check
 * @returns true if account is a Shared account
 */
export function isShared(account: NeuronAccount): account is SharedAccount {
  return account.accountType === AccountType.Shared;
}

/**
 * Derive the EVM address from an account's public key.
 *
 * FR-008: Delegates to publicKey.evmAddress() for Parent and Child accounts.
 * Shared accounts do not have a single public key and cannot derive an EVM address.
 *
 * @param account - NeuronAccount to derive EVM address from
 * @returns EVMAddress derived from the account's public key
 * @throws AccountError NEURON-ACCT-001 if account is Shared
 */
export function evmAddress(account: NeuronAccount): EVMAddress {
  if (isParent(account) || isChild(account)) {
    return account.publicKey.evmAddress();
  }
  throw invalidAccountType('Shared accounts do not have a single public key for EVM address derivation');
}

/**
 * Derive the libp2p PeerID from an account's public key.
 *
 * FR-008: Delegates to publicKey.peerId() for Parent and Child accounts.
 * Shared accounts do not have a single public key and cannot derive a PeerID.
 *
 * @param account - NeuronAccount to derive PeerID from
 * @returns PeerID derived from the account's public key
 * @throws AccountError NEURON-ACCT-001 if account is Shared
 */
export function peerId(account: NeuronAccount): PeerID {
  if (isParent(account) || isChild(account)) {
    return account.publicKey.peerId();
  }
  throw invalidAccountType('Shared accounts do not have a single public key for PeerID derivation');
}
