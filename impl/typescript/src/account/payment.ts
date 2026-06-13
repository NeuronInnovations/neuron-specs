/**
 * Payment address resolution for NeuronAccount.
 *
 * Spec reference: 001 spec.md
 *   - FR-023: Parent's payment address is its own EVM address.
 *   - FR-024: Child's payment address resolves to Parent's EVM address
 *     via parentPubKey derivation.
 *   - SC-011: Payment address resolution is deterministic and verifiable.
 *
 * Shared accounts do not have a payment address (no single public key).
 */

import type { EVMAddress } from '../keylib/evm-address.js';
import type { NeuronAccount } from './account.js';
import { isParent, isChild } from './account.js';
import { invalidAccountType } from './errors.js';

/**
 * Resolve the payment address for a NeuronAccount.
 *
 * FR-023: Parent accounts return their own EVM address (derived from publicKey).
 * FR-024: Child accounts return the Parent's EVM address (derived from parentPubKey).
 * Shared accounts do not have a payment address and throw.
 *
 * @param account - NeuronAccount to resolve payment address for
 * @returns EVMAddress for payment purposes
 * @throws AccountError NEURON-ACCT-001 if account is Shared
 */
export function paymentAddress(account: NeuronAccount): EVMAddress {
  if (isParent(account)) {
    return account.publicKey.evmAddress();
  }
  if (isChild(account)) {
    return account.parentPubKey.evmAddress();
  }
  throw invalidAccountType('Shared accounts do not have a payment address');
}
