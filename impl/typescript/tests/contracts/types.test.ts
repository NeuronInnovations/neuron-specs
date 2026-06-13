/**
 * Tests for shared contract types.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-29: ValidationResponse enum (0=pending, 1=pass, 2=fail).
 *   - FR-C-20..FR-C-23: FeedbackEntry structure.
 *   - FR-C-27..FR-C-30: ValidationRecord structure.
 *   - FR-C-24: FeedbackSummary structure.
 *   - FR-C-31: ValidationSummary structure.
 *
 * Verifies that type definitions accept well-formed data, enum values
 * map to correct integers, and constants are properly defined.
 */

import { describe, it, expect } from 'vitest';
import {
  ValidationResponse,
  BYTES32_LENGTH,
  MAX_FEEDBACK_DECIMALS,
} from '../../src/contracts/types.js';
import type {
  Uint256,
  Address,
  Bytes32,
  Int128,
  FeedbackEntry,
  ValidationRecord,
  FeedbackSummary,
  ValidationSummary,
} from '../../src/contracts/types.js';

// ---------------------------------------------------------------------------
// Helper: create a zero-filled Bytes32
// ---------------------------------------------------------------------------
function zeroBytes32(): Bytes32 {
  return new Uint8Array(32);
}

function filledBytes32(value: number): Bytes32 {
  const buf = new Uint8Array(32);
  buf.fill(value);
  return buf;
}

// ---------------------------------------------------------------------------
// Solidity Primitive Mappings
// ---------------------------------------------------------------------------

describe('Uint256', () => {
  it('should be represented as bigint', () => {
    const val: Uint256 = 42n;
    expect(typeof val).toBe('bigint');
    expect(val).toBe(42n);
  });

  it('should support large values (2^256 - 1)', () => {
    const max: Uint256 = 2n ** 256n - 1n;
    expect(max).toBeGreaterThan(0n);
  });
});

describe('Address', () => {
  it('should be represented as a string', () => {
    const addr: Address = '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B';
    expect(typeof addr).toBe('string');
    expect(addr).toMatch(/^0x[0-9a-fA-F]{40}$/);
  });
});

describe('Bytes32', () => {
  it('should be represented as a 32-byte Uint8Array', () => {
    const b: Bytes32 = zeroBytes32();
    expect(b).toBeInstanceOf(Uint8Array);
    expect(b.length).toBe(32);
  });

  it('should accept non-zero values', () => {
    const b: Bytes32 = filledBytes32(0xff);
    expect(b[0]).toBe(0xff);
    expect(b[31]).toBe(0xff);
  });
});

describe('Int128', () => {
  it('should be represented as bigint and support negative values', () => {
    const positive: Int128 = 450n;
    const negative: Int128 = -100n;
    expect(typeof positive).toBe('bigint');
    expect(positive).toBe(450n);
    expect(negative).toBe(-100n);
  });
});

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

describe('BYTES32_LENGTH', () => {
  it('should equal 32', () => {
    expect(BYTES32_LENGTH).toBe(32);
  });
});

describe('MAX_FEEDBACK_DECIMALS', () => {
  it('should equal 18 (FR-C-21)', () => {
    expect(MAX_FEEDBACK_DECIMALS).toBe(18);
  });
});

// ---------------------------------------------------------------------------
// ValidationResponse Enum
// ---------------------------------------------------------------------------

describe('ValidationResponse', () => {
  it('should map Pending to 0 (FR-C-29)', () => {
    expect(ValidationResponse.Pending).toBe(0);
  });

  it('should map Pass to 1 (FR-C-29)', () => {
    expect(ValidationResponse.Pass).toBe(1);
  });

  it('should map Fail to 2 (FR-C-29)', () => {
    expect(ValidationResponse.Fail).toBe(2);
  });

  it('should have exactly 3 members', () => {
    // Numeric enums have both forward and reverse mappings, so divide by 2.
    const members = Object.keys(ValidationResponse).filter((k) => isNaN(Number(k)));
    expect(members).toHaveLength(3);
    expect(members).toEqual(['Pending', 'Pass', 'Fail']);
  });

  it('should support reverse lookup from number', () => {
    expect(ValidationResponse[0]).toBe('Pending');
    expect(ValidationResponse[1]).toBe('Pass');
    expect(ValidationResponse[2]).toBe('Fail');
  });
});

// ---------------------------------------------------------------------------
// FeedbackEntry
// ---------------------------------------------------------------------------

describe('FeedbackEntry', () => {
  it('should construct with all required fields (FR-C-20, FR-C-21)', () => {
    const entry: FeedbackEntry = {
      client: '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      value: 450n,
      decimals: 2,
      tag1: filledBytes32(0x01),
      tag2: filledBytes32(0x02),
      feedbackURI: 'https://example.com/feedback/1.json',
      feedbackHash: filledBytes32(0xab),
      revoked: false,
      responseURI: '',
      responseHash: zeroBytes32(),
    };

    expect(entry.client).toMatch(/^0x/);
    expect(entry.value).toBe(450n);
    expect(entry.decimals).toBe(2);
    expect(entry.tag1.length).toBe(32);
    expect(entry.tag2.length).toBe(32);
    expect(entry.feedbackURI).toContain('feedback');
    expect(entry.feedbackHash.length).toBe(32);
    expect(entry.revoked).toBe(false);
    expect(entry.responseURI).toBe('');
    expect(entry.responseHash).toEqual(zeroBytes32());
  });

  it('should represent a revoked entry (FR-C-22)', () => {
    const entry: FeedbackEntry = {
      client: '0x1234567890123456789012345678901234567890',
      value: -100n,
      decimals: 0,
      tag1: zeroBytes32(),
      tag2: zeroBytes32(),
      feedbackURI: '',
      feedbackHash: zeroBytes32(),
      revoked: true,
      responseURI: '',
      responseHash: zeroBytes32(),
    };

    expect(entry.revoked).toBe(true);
    expect(entry.value).toBe(-100n);
  });

  it('should represent an entry with a response (FR-C-23)', () => {
    const entry: FeedbackEntry = {
      client: '0x1234567890123456789012345678901234567890',
      value: 500n,
      decimals: 2,
      tag1: zeroBytes32(),
      tag2: zeroBytes32(),
      feedbackURI: 'https://example.com/fb.json',
      feedbackHash: filledBytes32(0xcc),
      revoked: false,
      responseURI: 'https://example.com/response.json',
      responseHash: filledBytes32(0xdd),
    };

    expect(entry.responseURI).toContain('response');
    expect(entry.responseHash).toEqual(filledBytes32(0xdd));
  });
});

// ---------------------------------------------------------------------------
// ValidationRecord
// ---------------------------------------------------------------------------

describe('ValidationRecord', () => {
  it('should construct with Pending response (FR-C-27, FR-C-29)', () => {
    const record: ValidationRecord = {
      validator: '0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B',
      agentId: 1n,
      requestURI: 'https://example.com/validation-request.json',
      response: ValidationResponse.Pending,
      responseURI: '',
      responseHash: zeroBytes32(),
      tag: filledBytes32(0x01),
      lastUpdate: 1710000000n,
    };

    expect(record.validator).toMatch(/^0x/);
    expect(record.agentId).toBe(1n);
    expect(record.response).toBe(ValidationResponse.Pending);
    expect(record.responseURI).toBe('');
    expect(record.lastUpdate).toBe(1710000000n);
  });

  it('should construct with Pass response (FR-C-28, FR-C-29)', () => {
    const record: ValidationRecord = {
      validator: '0x1234567890123456789012345678901234567890',
      agentId: 42n,
      requestURI: 'https://example.com/req.json',
      response: ValidationResponse.Pass,
      responseURI: 'https://example.com/pass-response.json',
      responseHash: filledBytes32(0xee),
      tag: filledBytes32(0x02),
      lastUpdate: 1710000001n,
    };

    expect(record.response).toBe(ValidationResponse.Pass);
    expect(record.responseURI).toContain('pass');
  });

  it('should construct with Fail response (FR-C-29)', () => {
    const record: ValidationRecord = {
      validator: '0x1234567890123456789012345678901234567890',
      agentId: 42n,
      requestURI: 'https://example.com/req.json',
      response: ValidationResponse.Fail,
      responseURI: 'https://example.com/fail-response.json',
      responseHash: filledBytes32(0xff),
      tag: zeroBytes32(),
      lastUpdate: 1710000002n,
    };

    expect(record.response).toBe(ValidationResponse.Fail);
  });
});

// ---------------------------------------------------------------------------
// FeedbackSummary
// ---------------------------------------------------------------------------

describe('FeedbackSummary', () => {
  it('should construct with aggregated values (FR-C-24)', () => {
    const summary: FeedbackSummary = {
      count: 10n,
      totalValue: 4500n,
      decimals: 2,
    };

    expect(summary.count).toBe(10n);
    expect(summary.totalValue).toBe(4500n);
    expect(summary.decimals).toBe(2);
  });

  it('should represent an empty summary', () => {
    const summary: FeedbackSummary = {
      count: 0n,
      totalValue: 0n,
      decimals: 0,
    };

    expect(summary.count).toBe(0n);
    expect(summary.totalValue).toBe(0n);
  });
});

// ---------------------------------------------------------------------------
// ValidationSummary
// ---------------------------------------------------------------------------

describe('ValidationSummary', () => {
  it('should construct with aggregated counts (FR-C-31)', () => {
    const summary: ValidationSummary = {
      count: 5n,
      passCount: 3n,
      failCount: 2n,
    };

    expect(summary.count).toBe(5n);
    expect(summary.passCount).toBe(3n);
    expect(summary.failCount).toBe(2n);
  });

  it('should represent an empty summary', () => {
    const summary: ValidationSummary = {
      count: 0n,
      passCount: 0n,
      failCount: 0n,
    };

    expect(summary.count).toBe(0n);
    expect(summary.passCount).toBe(0n);
    expect(summary.failCount).toBe(0n);
  });

  it('should satisfy count >= passCount + failCount (pending entries)', () => {
    const summary: ValidationSummary = {
      count: 10n,
      passCount: 4n,
      failCount: 3n,
    };

    // Remaining 3 are pending
    expect(summary.count).toBeGreaterThanOrEqual(summary.passCount + summary.failCount);
  });
});
