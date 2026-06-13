/**
 * T002: Types tests -- AccountType enum, NeuronDID, LedgerAttachment, RegistryBinding.
 *
 * Source: 001 spec.md
 * Verifies: FR-013 (AccountType), FR-012 (NeuronDID), FR-018 (LedgerAttachment),
 *           FR-022 (RegistryBinding), FR-019 (VerificationStatus).
 */

import { describe, it, expect } from 'vitest';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import { AccountError } from '../../src/account/errors.js';
import {
  AccountType,
  createNeuronDID,
} from '../../src/account/types.js';
import type {
  NeuronDID,
  LedgerState,
  VerificationStatus,
  LedgerAttachment,
  RegistryBinding,
  LedgerAccountId,
  VerificationResult,
} from '../../src/account/types.js';

describe('AccountType enum (FR-013)', () => {
  it('should have Parent = 1', () => {
    expect(AccountType.Parent).toBe(1);
  });

  it('should have Child = 2', () => {
    expect(AccountType.Child).toBe(2);
  });

  it('should have Shared = 3', () => {
    expect(AccountType.Shared).toBe(3);
  });

  it('should have exactly 3 numeric values', () => {
    // Numeric enum members: filter out reverse-mapping string keys
    const numericValues = Object.values(AccountType).filter(
      (v): v is number => typeof v === 'number',
    );
    expect(numericValues).toHaveLength(3);
    expect(numericValues).toContain(1);
    expect(numericValues).toContain(2);
    expect(numericValues).toContain(3);
  });
});

describe('NeuronDID (FR-012)', () => {
  it('should create a NeuronDID from a valid did:key: string', () => {
    const did = createNeuronDID('did:key:zQ3shunBKsXixLxoQHQMjemrCzN4yuJofyhGKnmRPFhpnPHgq');
    expect(did.identifier).toBe('did:key:zQ3shunBKsXixLxoQHQMjemrCzN4yuJofyhGKnmRPFhpnPHgq');
  });

  it('should reject identifiers not starting with did:key:', () => {
    expect(() => createNeuronDID('did:web:example.com')).toThrow(AccountError);
    expect(() => createNeuronDID('did:web:example.com')).toThrow(/did:key:/);
  });

  it('should reject empty strings', () => {
    expect(() => createNeuronDID('')).toThrow(AccountError);
  });

  it('should reject partial prefixes', () => {
    expect(() => createNeuronDID('did:ke')).toThrow(AccountError);
    expect(() => createNeuronDID('did:')).toThrow(AccountError);
  });

  it('should accept minimal did:key: prefix', () => {
    // The createNeuronDID only validates the prefix, not the full DID
    const did = createNeuronDID('did:key:z');
    expect(did.identifier).toBe('did:key:z');
  });

  it('NEURON-ACCT-004 error code for invalid DID', () => {
    try {
      createNeuronDID('invalid');
      expect.fail('Expected AccountError');
    } catch (e) {
      expect(e).toBeInstanceOf(AccountError);
      expect((e as AccountError).code).toBe('NEURON-ACCT-004');
    }
  });
});

describe('LedgerAttachment (FR-018)', () => {
  it('should hold all required fields', () => {
    const pk = NeuronPrivateKey.fromHex(
      '0x0000000000000000000000000000000000000000000000000000000000000001',
    );
    const evmAddr = pk.publicKey().evmAddress();

    const attachment: LedgerAttachment = {
      ledgerIdentifier: 'ethereum-mainnet',
      attachedAddress: evmAddr,
      state: 'attached',
      verificationStatus: 'verified',
      lastSyncedAt: new Date('2026-01-01T00:00:00Z'),
    };

    expect(attachment.ledgerIdentifier).toBe('ethereum-mainnet');
    expect(attachment.attachedAddress.toString()).toBe(evmAddr.toString());
    expect(attachment.state).toBe('attached');
    expect(attachment.verificationStatus).toBe('verified');
    expect(attachment.lastSyncedAt).toBeInstanceOf(Date);
  });

  it('should accept null lastSyncedAt for never-synced state', () => {
    const pk = NeuronPrivateKey.fromHex(
      '0x0000000000000000000000000000000000000000000000000000000000000001',
    );
    const evmAddr = pk.publicKey().evmAddress();

    const attachment: LedgerAttachment = {
      ledgerIdentifier: 'hedera-mainnet',
      attachedAddress: evmAddr,
      state: 'detached',
      verificationStatus: 'unverified',
      lastSyncedAt: null,
    };

    expect(attachment.lastSyncedAt).toBeNull();
    expect(attachment.state).toBe('detached');
  });

  it('should accept all valid LedgerState values', () => {
    const states: LedgerState[] = ['attached', 'detached'];
    expect(states).toHaveLength(2);
  });

  it('should accept all valid VerificationStatus values', () => {
    const statuses: VerificationStatus[] = ['verified', 'unverified', 'failed'];
    expect(statuses).toHaveLength(3);
  });
});

describe('RegistryBinding (FR-022)', () => {
  it('should hold registryIdentifier and externalId', () => {
    const binding: RegistryBinding = {
      registryIdentifier: 'eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18',
      externalId: '42',
    };

    expect(binding.registryIdentifier).toContain('eip155');
    expect(binding.externalId).toBe('42');
  });
});

describe('LedgerAccountId (FR-026)', () => {
  it('should be a string type alias', () => {
    const id: LedgerAccountId = '0.0.12345';
    expect(typeof id).toBe('string');
  });
});

describe('VerificationResult (FR-017, FR-019)', () => {
  it('should represent a verified result', () => {
    const result: VerificationResult = { status: 'verified' };
    expect(result.status).toBe('verified');
  });

  it('should represent an unverified result with optional reason', () => {
    const result: VerificationResult = {
      status: 'unverified',
      reason: 'Ledger not yet synced',
    };
    expect(result.status).toBe('unverified');
    if (result.status === 'unverified') {
      expect(result.reason).toBe('Ledger not yet synced');
    }
  });

  it('should represent an unverified result without reason', () => {
    const result: VerificationResult = { status: 'unverified' };
    expect(result.status).toBe('unverified');
  });

  it('should represent a failed result with error', () => {
    const result: VerificationResult = {
      status: 'failed',
      error: 'Connection timeout',
    };
    expect(result.status).toBe('failed');
    if (result.status === 'failed') {
      expect(result.error).toBe('Connection timeout');
    }
  });
});
