/**
 * Tests for the IValidationRegistry interface and event types.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-27: validationRequest() creates request. Agent owner only.
 *   - FR-C-28: validationResponse() addressed validator only.
 *   - FR-C-29: Response codes: 0=pending, 1=pass, 2=fail.
 *   - FR-C-30: getValidationStatus() returns complete record.
 *   - FR-C-31: getSummary() returns aggregated counts.
 *   - FR-C-32: ValidationRequested, ValidationResponded events.
 *   - FR-C-33: agentId must exist in Identity Registry.
 *
 * Tests use a mock implementation to verify the interface contract compiles
 * and behaves correctly at the type level.
 */

import { describe, it, expect } from 'vitest';
import { ValidationResponse } from '../../src/contracts/types.js';
import type {
  IValidationRegistry,
  ValidationRequestedEvent,
  ValidationRespondedEvent,
} from '../../src/contracts/validation-registry.js';
import type {
  Uint256,
  Address,
  Bytes32,
  ValidationRecord,
  ValidationSummary,
} from '../../src/contracts/types.js';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function zeroBytes32(): Bytes32 {
  return new Uint8Array(32);
}

function hashBytes32(seed: number): Bytes32 {
  const buf = new Uint8Array(32);
  buf[0] = seed & 0xff;
  buf[1] = (seed >> 8) & 0xff;
  return buf;
}

function tagBytes32(tag: string): Bytes32 {
  const buf = new Uint8Array(32);
  const encoded = new TextEncoder().encode(tag);
  buf.set(encoded.subarray(0, 32));
  return buf;
}

// ---------------------------------------------------------------------------
// Mock Implementation
// ---------------------------------------------------------------------------

interface StoredValidation {
  validator: Address;
  agentId: Uint256;
  requestURI: string;
  response: ValidationResponse;
  responseURI: string;
  responseHash: Bytes32;
  tag: Bytes32;
  lastUpdate: Uint256;
}

function createMockValidationRegistry(): IValidationRegistry {
  const store = new Map<string, StoredValidation>();
  let requestCounter = 0;

  function hashKey(buf: Bytes32): string {
    return Array.from(buf).map((b) => b.toString(16).padStart(2, '0')).join('');
  }

  return {
    async validationRequest(
      agentId: Uint256,
      validator: Address,
      requestURI: string,
      tag: Bytes32,
    ): Promise<Bytes32> {
      requestCounter++;
      const requestHash = hashBytes32(requestCounter);
      store.set(hashKey(requestHash), {
        validator,
        agentId,
        requestURI,
        response: ValidationResponse.Pending,
        responseURI: '',
        responseHash: zeroBytes32(),
        tag,
        lastUpdate: BigInt(Date.now()),
      });
      return requestHash;
    },

    async validationResponse(
      requestHash: Bytes32,
      response: ValidationResponse,
      responseURI: string,
      responseHash: Bytes32,
      tag: Bytes32,
    ): Promise<void> {
      const key = hashKey(requestHash);
      const record = store.get(key);
      if (record !== undefined) {
        record.response = response;
        record.responseURI = responseURI;
        record.responseHash = responseHash;
        record.tag = tag;
        record.lastUpdate = BigInt(Date.now());
      }
    },

    async getValidationStatus(requestHash: Bytes32): Promise<ValidationRecord> {
      const key = hashKey(requestHash);
      const record = store.get(key);
      if (record !== undefined) {
        return {
          validator: record.validator,
          agentId: record.agentId,
          requestURI: record.requestURI,
          response: record.response,
          responseURI: record.responseURI,
          responseHash: record.responseHash,
          tag: record.tag,
          lastUpdate: record.lastUpdate,
        };
      }
      return {
        validator: '0x0000000000000000000000000000000000000000',
        agentId: 0n,
        requestURI: '',
        response: ValidationResponse.Pending,
        responseURI: '',
        responseHash: zeroBytes32(),
        tag: zeroBytes32(),
        lastUpdate: 0n,
      };
    },

    async getSummary(
      agentId: Uint256,
      _validatorAddresses: ReadonlyArray<Address>,
      _tag: Bytes32,
    ): Promise<ValidationSummary> {
      let count = 0n;
      let passCount = 0n;
      let failCount = 0n;
      for (const record of store.values()) {
        if (record.agentId === agentId) {
          count++;
          if (record.response === ValidationResponse.Pass) passCount++;
          if (record.response === ValidationResponse.Fail) failCount++;
        }
      }
      return { count, passCount, failCount };
    },
  };
}

// ---------------------------------------------------------------------------
// Interface Contract Tests
// ---------------------------------------------------------------------------

describe('IValidationRegistry', () => {
  it('should create a validation request and return requestHash (FR-C-27)', async () => {
    const registry = createMockValidationRegistry();
    const requestHash = await registry.validationRequest(
      1n,
      '0xVALIDATOR000000000000000000000000000001',
      'https://example.com/validation-request.json',
      tagBytes32('security'),
    );

    expect(requestHash).toBeInstanceOf(Uint8Array);
    expect(requestHash.length).toBe(32);
  });

  it('should return Pending status for new request (FR-C-29, FR-C-30)', async () => {
    const registry = createMockValidationRegistry();
    const requestHash = await registry.validationRequest(
      1n,
      '0xVALIDATOR000000000000000000000000000001',
      'https://example.com/req.json',
      zeroBytes32(),
    );

    const status = await registry.getValidationStatus(requestHash);

    expect(status.response).toBe(ValidationResponse.Pending);
    expect(status.agentId).toBe(1n);
    expect(status.validator).toBe('0xVALIDATOR000000000000000000000000000001');
    expect(status.requestURI).toBe('https://example.com/req.json');
  });

  it('should record a Pass response (FR-C-28, FR-C-29)', async () => {
    const registry = createMockValidationRegistry();
    const requestHash = await registry.validationRequest(
      1n,
      '0xVALIDATOR000000000000000000000000000001',
      'https://example.com/req.json',
      tagBytes32('security'),
    );

    const responseHash = hashBytes32(0xab);
    await registry.validationResponse(
      requestHash,
      ValidationResponse.Pass,
      'https://example.com/pass-response.json',
      responseHash,
      tagBytes32('security'),
    );

    const status = await registry.getValidationStatus(requestHash);
    expect(status.response).toBe(ValidationResponse.Pass);
    expect(status.responseURI).toBe('https://example.com/pass-response.json');
    expect(status.responseHash).toEqual(responseHash);
  });

  it('should record a Fail response (FR-C-28, FR-C-29)', async () => {
    const registry = createMockValidationRegistry();
    const requestHash = await registry.validationRequest(
      1n,
      '0xVALIDATOR000000000000000000000000000001',
      'https://example.com/req.json',
      zeroBytes32(),
    );

    await registry.validationResponse(
      requestHash,
      ValidationResponse.Fail,
      'https://example.com/fail.json',
      hashBytes32(0xcd),
      tagBytes32('compliance'),
    );

    const status = await registry.getValidationStatus(requestHash);
    expect(status.response).toBe(ValidationResponse.Fail);
  });

  it('should return aggregated summary (FR-C-31)', async () => {
    const registry = createMockValidationRegistry();

    // Create 3 requests for the same agent
    const h1 = await registry.validationRequest(
      1n, '0xV1', 'uri1', zeroBytes32(),
    );
    const h2 = await registry.validationRequest(
      1n, '0xV2', 'uri2', zeroBytes32(),
    );
    const h3 = await registry.validationRequest(
      1n, '0xV3', 'uri3', zeroBytes32(),
    );

    // Respond: pass, fail, pending
    await registry.validationResponse(
      h1, ValidationResponse.Pass, '', zeroBytes32(), zeroBytes32(),
    );
    await registry.validationResponse(
      h2, ValidationResponse.Fail, '', zeroBytes32(), zeroBytes32(),
    );
    // h3 stays pending

    const summary = await registry.getSummary(1n, [], zeroBytes32());

    expect(summary.count).toBe(3n);
    expect(summary.passCount).toBe(1n);
    expect(summary.failCount).toBe(1n);
  });

  it('should return empty summary for agent with no validations', async () => {
    const registry = createMockValidationRegistry();
    const summary = await registry.getSummary(99n, [], zeroBytes32());

    expect(summary.count).toBe(0n);
    expect(summary.passCount).toBe(0n);
    expect(summary.failCount).toBe(0n);
  });

  it('should return default record for unknown requestHash (FR-C-30)', async () => {
    const registry = createMockValidationRegistry();
    const unknownHash = hashBytes32(0xff);

    const status = await registry.getValidationStatus(unknownHash);

    expect(status.agentId).toBe(0n);
    expect(status.response).toBe(ValidationResponse.Pending);
    expect(status.validator).toBe('0x0000000000000000000000000000000000000000');
  });
});

// ---------------------------------------------------------------------------
// Event Type Shape Tests
// ---------------------------------------------------------------------------

describe('ValidationRequestedEvent', () => {
  it('should have correct shape (FR-C-32)', () => {
    const event: ValidationRequestedEvent = {
      requestHash: hashBytes32(1),
      agentId: 1n,
      validatorAddress: '0xVALIDATOR000000000000000000000000000001',
    };

    expect(event.requestHash).toBeInstanceOf(Uint8Array);
    expect(event.requestHash.length).toBe(32);
    expect(event.agentId).toBe(1n);
    expect(event.validatorAddress).toMatch(/^0x/);
  });
});

describe('ValidationRespondedEvent', () => {
  it('should have correct shape (FR-C-32)', () => {
    const event: ValidationRespondedEvent = {
      requestHash: hashBytes32(1),
      response: ValidationResponse.Pass,
      tag: tagBytes32('security'),
    };

    expect(event.requestHash).toBeInstanceOf(Uint8Array);
    expect(event.response).toBe(ValidationResponse.Pass);
    expect(event.tag.length).toBe(32);
  });

  it('should accept all response types', () => {
    const pending: ValidationRespondedEvent = {
      requestHash: hashBytes32(1),
      response: ValidationResponse.Pending,
      tag: zeroBytes32(),
    };
    const pass: ValidationRespondedEvent = {
      requestHash: hashBytes32(2),
      response: ValidationResponse.Pass,
      tag: zeroBytes32(),
    };
    const fail: ValidationRespondedEvent = {
      requestHash: hashBytes32(3),
      response: ValidationResponse.Fail,
      tag: zeroBytes32(),
    };

    expect(pending.response).toBe(0);
    expect(pass.response).toBe(1);
    expect(fail.response).toBe(2);
  });
});
