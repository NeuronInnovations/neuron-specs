/**
 * Fluent builders for NeuronAccount construction.
 *
 * Spec reference: 001 spec.md, contracts/account-builder.md
 *   - FR-001, FR-006: ParentAccountBuilder with publicKey + DID + currency.
 *   - FR-002, FR-007: ChildAccountBuilder with publicKey + parentPubKey + currency.
 *   - FR-007a: SharedAccountBuilder with multisigKey + currency.
 *   - FR-011: build() validates type-specific rules, throws on failure.
 *   - FR-011a: buildComplete() adds completeness checks (ledger attachment, registry binding).
 *   - FR-018: Optional ledger attachment on all builders.
 *   - FR-020: Currency symbol is mandatory on all accounts.
 *   - FR-022: RegistryBinding is mandatory for Child completeness.
 *   - FR-026: FeePayer is optional for Child accounts.
 *
 * Each builder follows the fluent pattern: `.withX().withY().build()`.
 * build() returns a readonly, immutable NeuronAccount instance.
 * buildComplete() enforces stricter completeness requirements.
 */

import type { NeuronPublicKey } from '../keylib/public-key.js';
import type { MultisigKey } from '../keylib/multisig-key.js';
import type { ParentAccount, ChildAccount, SharedAccount } from './account.js';
import type {
  NeuronDID,
  LedgerAttachment,
  RegistryBinding,
  LedgerAccountId,
} from './types.js';
import { AccountType } from './types.js';
import {
  missingRequiredField,
  invalidCurrencySymbol,
  accountIncomplete,
} from './errors.js';

/**
 * Fluent builder for Parent accounts.
 *
 * FR-001, FR-006: Requires publicKey, DID, and currencySymbol.
 * FR-018: Optional ledger attachment.
 *
 * Usage:
 *   const parent = new ParentAccountBuilder()
 *     .withPublicKey(pubKey)
 *     .withDID(did)
 *     .withCurrency('HBAR')
 *     .build();
 */
export class ParentAccountBuilder {
  private _publicKey?: NeuronPublicKey | undefined;
  private _did?: NeuronDID | undefined;
  private _currencySymbol?: string | undefined;
  private _ledgerAttachment?: LedgerAttachment | undefined;

  /**
   * Set the Parent's secp256k1 public key.
   * FR-006: MUST be a single NeuronPublicKey.
   */
  withPublicKey(pk: NeuronPublicKey): this {
    this._publicKey = pk;
    return this;
  }

  /**
   * Set the Parent's DID:key identifier.
   * FR-006, FR-012: MUST be present for Parent accounts.
   */
  withDID(did: NeuronDID): this {
    this._did = did;
    return this;
  }

  /**
   * Set the currency symbol.
   * FR-020: MUST be present on all account types.
   */
  withCurrency(symbol: string): this {
    this._currencySymbol = symbol;
    return this;
  }

  /**
   * Set the optional ledger attachment.
   * FR-018: MAY be present on any account type.
   */
  withLedgerAttachment(attachment: LedgerAttachment): this {
    this._ledgerAttachment = attachment;
    return this;
  }

  /**
   * Build a Parent account with basic validation.
   *
   * FR-011: Validates type-specific rules (V-PARENT-01..02).
   * Throws on the first missing required field.
   *
   * @returns Immutable ParentAccount instance
   * @throws AccountError NEURON-ACCT-002 if publicKey is missing
   * @throws AccountError NEURON-ACCT-002 if DID is missing
   * @throws AccountError NEURON-ACCT-008 if currencySymbol is missing or empty
   */
  build(): ParentAccount {
    if (this._publicKey == null) {
      throw missingRequiredField('publicKey is required for Parent account');
    }
    if (this._did == null) {
      throw missingRequiredField('did is required for Parent account');
    }
    if (this._currencySymbol == null || this._currencySymbol === '') {
      throw invalidCurrencySymbol('Currency symbol is required');
    }

    const account: ParentAccount = {
      accountType: AccountType.Parent,
      publicKey: this._publicKey,
      did: this._did,
      currencySymbol: this._currencySymbol,
      ...(this._ledgerAttachment != null
        ? { ledgerAttachment: this._ledgerAttachment }
        : {}),
    };

    return account;
  }

  /**
   * Build a complete Parent account with strict completeness validation.
   *
   * FR-011a: A Parent account is complete when it has all MUST fields
   * plus a ledger attachment.
   *
   * @returns Immutable ParentAccount instance with ledger attachment
   * @throws AccountError NEURON-ACCT-002 if required fields are missing
   * @throws AccountError NEURON-ACCT-008 if currencySymbol is missing
   * @throws AccountError NEURON-ACCT-007 if ledger attachment is missing
   */
  buildComplete(): ParentAccount {
    const account = this.build();
    if (account.ledgerAttachment == null) {
      throw accountIncomplete('Parent account requires ledger attachment for completeness');
    }
    return account;
  }
}

/**
 * Fluent builder for Child accounts.
 *
 * FR-002, FR-007: Requires publicKey, parentPubKey, and currencySymbol.
 * FR-022: RegistryBinding mandatory for completeness.
 * FR-026: FeePayer is optional.
 * FR-018: Optional ledger attachment.
 *
 * Usage:
 *   const child = new ChildAccountBuilder()
 *     .withPublicKey(childPubKey)
 *     .withParentPublicKey(parentPubKey)
 *     .withCurrency('HBAR')
 *     .withRegistryBinding({ registryIdentifier: '...', externalId: '...' })
 *     .build();
 */
export class ChildAccountBuilder {
  private _publicKey?: NeuronPublicKey | undefined;
  private _parentPubKey?: NeuronPublicKey | undefined;
  private _currencySymbol?: string | undefined;
  private _registryBinding?: RegistryBinding | undefined;
  private _feePayer?: LedgerAccountId | undefined;
  private _ledgerAttachment?: LedgerAttachment | undefined;

  /**
   * Set the Child's secp256k1 public key.
   * FR-007: MUST be a single NeuronPublicKey.
   */
  withPublicKey(pk: NeuronPublicKey): this {
    this._publicKey = pk;
    return this;
  }

  /**
   * Set the reference to the Parent's public key.
   * FR-007: MUST be present for Child accounts.
   */
  withParentPublicKey(pk: NeuronPublicKey): this {
    this._parentPubKey = pk;
    return this;
  }

  /**
   * Set the currency symbol.
   * FR-020: MUST be present on all account types.
   */
  withCurrency(symbol: string): this {
    this._currencySymbol = symbol;
    return this;
  }

  /**
   * Set the registry binding.
   * FR-022: MUST be present for completeness (FR-011a).
   */
  withRegistryBinding(binding: RegistryBinding): this {
    this._registryBinding = binding;
    return this;
  }

  /**
   * Set the fee payer account ID.
   * FR-026: MAY be present on Child accounts.
   */
  withFeePayer(feePayer: LedgerAccountId): this {
    this._feePayer = feePayer;
    return this;
  }

  /**
   * Set the optional ledger attachment.
   * FR-018: MAY be present on any account type.
   */
  withLedgerAttachment(attachment: LedgerAttachment): this {
    this._ledgerAttachment = attachment;
    return this;
  }

  /**
   * Build a Child account with basic validation.
   *
   * FR-011: Validates type-specific rules (V-CHILD-01..02).
   * Throws on the first missing required field.
   *
   * @returns Immutable ChildAccount instance
   * @throws AccountError NEURON-ACCT-002 if publicKey is missing
   * @throws AccountError NEURON-ACCT-002 if parentPubKey is missing
   * @throws AccountError NEURON-ACCT-008 if currencySymbol is missing or empty
   */
  build(): ChildAccount {
    if (this._publicKey == null) {
      throw missingRequiredField('publicKey is required for Child account');
    }
    if (this._parentPubKey == null) {
      throw missingRequiredField('parentPubKey is required for Child account');
    }
    if (this._currencySymbol == null || this._currencySymbol === '') {
      throw invalidCurrencySymbol('Currency symbol is required');
    }

    const account: ChildAccount = {
      accountType: AccountType.Child,
      publicKey: this._publicKey,
      parentPubKey: this._parentPubKey,
      currencySymbol: this._currencySymbol,
      ...(this._registryBinding != null
        ? { registryBinding: this._registryBinding }
        : {}),
      ...(this._feePayer != null
        ? { feePayer: this._feePayer }
        : {}),
      ...(this._ledgerAttachment != null
        ? { ledgerAttachment: this._ledgerAttachment }
        : {}),
    };

    return account;
  }

  /**
   * Build a complete Child account with strict completeness validation.
   *
   * FR-011a: A Child account is complete when it has all MUST fields
   * plus a registry binding.
   *
   * @returns Immutable ChildAccount instance with registry binding
   * @throws AccountError NEURON-ACCT-002 if required fields are missing
   * @throws AccountError NEURON-ACCT-008 if currencySymbol is missing
   * @throws AccountError NEURON-ACCT-007 if registry binding is missing
   */
  buildComplete(): ChildAccount {
    const account = this.build();
    if (account.registryBinding == null) {
      throw accountIncomplete('Child account requires registry binding for completeness');
    }
    return account;
  }
}

/**
 * Fluent builder for Shared accounts.
 *
 * FR-007a: Requires multisigKey and currencySymbol.
 * FR-018: Optional ledger attachment.
 *
 * Usage:
 *   const shared = new SharedAccountBuilder()
 *     .withMultisigKey(multisig)
 *     .withCurrency('HBAR')
 *     .build();
 */
export class SharedAccountBuilder {
  private _multisigKey?: MultisigKey | undefined;
  private _currencySymbol?: string | undefined;
  private _ledgerAttachment?: LedgerAttachment | undefined;

  /**
   * Set the multi-signature key configuration.
   * FR-007a: MUST be present for Shared accounts.
   */
  withMultisigKey(key: MultisigKey): this {
    this._multisigKey = key;
    return this;
  }

  /**
   * Set the currency symbol.
   * FR-020: MUST be present on all account types.
   */
  withCurrency(symbol: string): this {
    this._currencySymbol = symbol;
    return this;
  }

  /**
   * Set the optional ledger attachment.
   * FR-018: MAY be present on any account type.
   */
  withLedgerAttachment(attachment: LedgerAttachment): this {
    this._ledgerAttachment = attachment;
    return this;
  }

  /**
   * Build a Shared account with basic validation.
   *
   * FR-011: Validates type-specific rules (V-SHARED-01).
   * Throws on the first missing required field.
   *
   * @returns Immutable SharedAccount instance
   * @throws AccountError NEURON-ACCT-002 if multisigKey is missing
   * @throws AccountError NEURON-ACCT-008 if currencySymbol is missing or empty
   */
  build(): SharedAccount {
    if (this._multisigKey == null) {
      throw missingRequiredField('multisigKey is required for Shared account');
    }
    if (this._currencySymbol == null || this._currencySymbol === '') {
      throw invalidCurrencySymbol('Currency symbol is required');
    }

    const account: SharedAccount = {
      accountType: AccountType.Shared,
      multisigKey: this._multisigKey,
      currencySymbol: this._currencySymbol,
      ...(this._ledgerAttachment != null
        ? { ledgerAttachment: this._ledgerAttachment }
        : {}),
    };

    return account;
  }

  /**
   * Build a complete Shared account with strict completeness validation.
   *
   * FR-011a: A Shared account is complete when it has all MUST fields
   * plus a ledger attachment.
   *
   * @returns Immutable SharedAccount instance with ledger attachment
   * @throws AccountError NEURON-ACCT-002 if required fields are missing
   * @throws AccountError NEURON-ACCT-008 if currencySymbol is missing
   * @throws AccountError NEURON-ACCT-007 if ledger attachment is missing
   */
  buildComplete(): SharedAccount {
    const account = this.build();
    if (account.ledgerAttachment == null) {
      throw accountIncomplete('Shared account requires ledger attachment for completeness');
    }
    return account;
  }
}
