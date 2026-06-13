/**
 * Tests for the IReputationRegistry interface and event types.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-20: giveFeedback() records feedback. Sequential feedbackIndex.
 *   - FR-C-21: Fixed-point: int128 value with uint8 decimals (0-18).
 *   - FR-C-22: revokeFeedback() original giver only. Excluded from summaries.
 *   - FR-C-23: appendResponse() agent owner only.
 *   - FR-C-24: getSummary() aggregated, filtered by tags and clients.
 *   - FR-C-25: FeedbackGiven, FeedbackRevoked, ResponseAppended events.
 *   - FR-C-26: agentId must exist in Identity Registry.
 *
 * Tests use a mock implementation to verify the interface contract compiles
 * and behaves correctly at the type level.
 */

import { describe, it, expect } from 'vitest';
import type {
  IReputationRegistry,
  FeedbackGivenEvent,
  FeedbackRevokedEvent,
  ResponseAppendedEvent,
} from '../../src/contracts/reputation-registry.js';
import type {
  Uint256,
  Address,
  Bytes32,
  Int128,
  FeedbackEntry,
  FeedbackSummary,
} from '../../src/contracts/types.js';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function zeroBytes32(): Bytes32 {
  return new Uint8Array(32);
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

interface StoredFeedback {
  client: Address;
  value: Int128;
  decimals: number;
  tag1: Bytes32;
  tag2: Bytes32;
  feedbackURI: string;
  feedbackHash: Bytes32;
  revoked: boolean;
  responseURI: string;
  responseHash: Bytes32;
}

function createMockReputationRegistry(): IReputationRegistry {
  const feedbackStore = new Map<string, StoredFeedback[]>();

  function getKey(agentId: Uint256): string {
    return agentId.toString();
  }

  return {
    async giveFeedback(
      agentId: Uint256,
      value: Int128,
      decimals: number,
      tag1: Bytes32,
      tag2: Bytes32,
      feedbackURI: string,
      feedbackHash: Bytes32,
    ): Promise<Uint256> {
      const key = getKey(agentId);
      const entries = feedbackStore.get(key) ?? [];
      const feedbackIndex = BigInt(entries.length);
      entries.push({
        client: '0xCLIENT0000000000000000000000000000000001',
        value,
        decimals,
        tag1,
        tag2,
        feedbackURI,
        feedbackHash,
        revoked: false,
        responseURI: '',
        responseHash: zeroBytes32(),
      });
      feedbackStore.set(key, entries);
      return feedbackIndex;
    },

    async revokeFeedback(agentId: Uint256, feedbackIndex: Uint256): Promise<void> {
      const key = getKey(agentId);
      const entries = feedbackStore.get(key);
      if (entries !== undefined) {
        const idx = Number(feedbackIndex);
        const entry = entries[idx];
        if (entry !== undefined) {
          entry.revoked = true;
        }
      }
    },

    async appendResponse(
      agentId: Uint256,
      _clientAddress: Address,
      feedbackIndex: Uint256,
      responseURI: string,
      responseHash: Bytes32,
    ): Promise<void> {
      const key = getKey(agentId);
      const entries = feedbackStore.get(key);
      if (entries !== undefined) {
        const idx = Number(feedbackIndex);
        const entry = entries[idx];
        if (entry !== undefined) {
          entry.responseURI = responseURI;
          entry.responseHash = responseHash;
        }
      }
    },

    async getSummary(
      agentId: Uint256,
      _clientAddresses: ReadonlyArray<Address>,
      _tag1: Bytes32,
      _tag2: Bytes32,
    ): Promise<FeedbackSummary> {
      const key = getKey(agentId);
      const entries = feedbackStore.get(key) ?? [];
      const active = entries.filter((e) => !e.revoked);
      let totalValue = 0n;
      for (const e of active) {
        totalValue += e.value;
      }
      return {
        count: BigInt(active.length),
        totalValue,
        decimals: active[0]?.decimals ?? 0,
      };
    },

    async getFeedbackEntries(
      agentId: Uint256,
      _tags: ReadonlyArray<Bytes32>,
    ): Promise<ReadonlyArray<FeedbackEntry>> {
      const key = getKey(agentId);
      const entries = feedbackStore.get(key) ?? [];
      return entries.map((e) => ({
        client: e.client,
        value: e.value,
        decimals: e.decimals,
        tag1: e.tag1,
        tag2: e.tag2,
        feedbackURI: e.feedbackURI,
        feedbackHash: e.feedbackHash,
        revoked: e.revoked,
        responseURI: e.responseURI,
        responseHash: e.responseHash,
      }));
    },
  };
}

// ---------------------------------------------------------------------------
// Interface Contract Tests
// ---------------------------------------------------------------------------

describe('IReputationRegistry', () => {
  it('should give feedback and return feedbackIndex (FR-C-20)', async () => {
    const registry = createMockReputationRegistry();
    const index = await registry.giveFeedback(
      1n,
      450n,
      2,
      tagBytes32('quality'),
      tagBytes32('speed'),
      'https://example.com/feedback.json',
      zeroBytes32(),
    );

    expect(index).toBe(0n);
    expect(typeof index).toBe('bigint');
  });

  it('should assign sequential feedbackIndex per agent (FR-C-20)', async () => {
    const registry = createMockReputationRegistry();

    const idx0 = await registry.giveFeedback(
      1n, 450n, 2, zeroBytes32(), zeroBytes32(), '', zeroBytes32(),
    );
    const idx1 = await registry.giveFeedback(
      1n, 300n, 2, zeroBytes32(), zeroBytes32(), '', zeroBytes32(),
    );

    expect(idx0).toBe(0n);
    expect(idx1).toBe(1n);
  });

  it('should revoke feedback (FR-C-22)', async () => {
    const registry = createMockReputationRegistry();
    await registry.giveFeedback(
      1n, 450n, 2, zeroBytes32(), zeroBytes32(), '', zeroBytes32(),
    );

    await registry.revokeFeedback(1n, 0n);

    const summary = await registry.getSummary(1n, [], zeroBytes32(), zeroBytes32());
    expect(summary.count).toBe(0n);
  });

  it('should exclude revoked feedback from summaries (FR-C-22, FR-C-24)', async () => {
    const registry = createMockReputationRegistry();
    await registry.giveFeedback(
      1n, 450n, 2, zeroBytes32(), zeroBytes32(), '', zeroBytes32(),
    );
    await registry.giveFeedback(
      1n, 300n, 2, zeroBytes32(), zeroBytes32(), '', zeroBytes32(),
    );

    // Revoke the first one
    await registry.revokeFeedback(1n, 0n);

    const summary = await registry.getSummary(1n, [], zeroBytes32(), zeroBytes32());
    expect(summary.count).toBe(1n);
    expect(summary.totalValue).toBe(300n);
  });

  it('should append response to feedback (FR-C-23)', async () => {
    const registry = createMockReputationRegistry();
    await registry.giveFeedback(
      1n, 450n, 2, zeroBytes32(), zeroBytes32(),
      'https://example.com/fb.json', zeroBytes32(),
    );

    const responseHash = new Uint8Array(32);
    responseHash.fill(0xaa);
    await registry.appendResponse(
      1n,
      '0xCLIENT0000000000000000000000000000000001',
      0n,
      'https://example.com/response.json',
      responseHash,
    );

    const entries = await registry.getFeedbackEntries(1n, []);
    expect(entries).toHaveLength(1);
    expect(entries[0]!.responseURI).toBe('https://example.com/response.json');
    expect(entries[0]!.responseHash).toEqual(responseHash);
  });

  it('should return aggregated summary (FR-C-24)', async () => {
    const registry = createMockReputationRegistry();
    await registry.giveFeedback(
      1n, 450n, 2, zeroBytes32(), zeroBytes32(), '', zeroBytes32(),
    );
    await registry.giveFeedback(
      1n, 350n, 2, zeroBytes32(), zeroBytes32(), '', zeroBytes32(),
    );

    const summary = await registry.getSummary(1n, [], zeroBytes32(), zeroBytes32());

    expect(summary.count).toBe(2n);
    expect(summary.totalValue).toBe(800n); // 450 + 350
    expect(summary.decimals).toBe(2);
  });

  it('should return empty summary for agent with no feedback', async () => {
    const registry = createMockReputationRegistry();
    const summary = await registry.getSummary(99n, [], zeroBytes32(), zeroBytes32());

    expect(summary.count).toBe(0n);
    expect(summary.totalValue).toBe(0n);
  });
});

// ---------------------------------------------------------------------------
// Event Type Shape Tests
// ---------------------------------------------------------------------------

describe('FeedbackGivenEvent', () => {
  it('should have correct shape (FR-C-25)', () => {
    const event: FeedbackGivenEvent = {
      agentId: 1n,
      client: '0xCLIENT0000000000000000000000000000000001',
      feedbackIndex: 0n,
      value: 450n,
      decimals: 2,
      tag1: tagBytes32('quality'),
      tag2: tagBytes32('speed'),
    };

    expect(event.agentId).toBe(1n);
    expect(event.client).toMatch(/^0x/);
    expect(event.feedbackIndex).toBe(0n);
    expect(event.value).toBe(450n);
    expect(event.decimals).toBe(2);
    expect(event.tag1.length).toBe(32);
    expect(event.tag2.length).toBe(32);
  });
});

describe('FeedbackRevokedEvent', () => {
  it('should have correct shape (FR-C-25)', () => {
    const event: FeedbackRevokedEvent = {
      agentId: 1n,
      client: '0xCLIENT0000000000000000000000000000000001',
      feedbackIndex: 0n,
    };

    expect(event.agentId).toBe(1n);
    expect(event.client).toMatch(/^0x/);
    expect(event.feedbackIndex).toBe(0n);
  });
});

describe('ResponseAppendedEvent', () => {
  it('should have correct shape (FR-C-25)', () => {
    const event: ResponseAppendedEvent = {
      agentId: 1n,
      clientAddress: '0xCLIENT0000000000000000000000000000000001',
      feedbackIndex: 0n,
    };

    expect(event.agentId).toBe(1n);
    expect(event.clientAddress).toMatch(/^0x/);
    expect(event.feedbackIndex).toBe(0n);
  });
});
