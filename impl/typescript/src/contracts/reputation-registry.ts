/**
 * IReputationRegistry -- Reputation Registry interface for client feedback.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-20: giveFeedback() records feedback from msg.sender about an agent.
 *   - FR-C-21: Fixed-point arithmetic: int128 value with uint8 decimals (0-18).
 *   - FR-C-22: revokeFeedback() allows only the original giver to revoke.
 *   - FR-C-23: appendResponse() allows only the agent owner to append responses.
 *   - FR-C-24: getSummary() returns aggregated counts filtered by tags and clients.
 *   - FR-C-25: Events: FeedbackGiven, FeedbackRevoked, ResponseAppended.
 *   - FR-C-26: agentId must exist in the linked Identity Registry.
 *
 * Cross-registry identity: the agentId (tokenId from Identity Registry) is the
 * key that links feedback to a specific registered agent (DD-04).
 */

import type { Uint256, Int128, Address, Bytes32, FeedbackEntry, FeedbackSummary } from './types.js';

// ---------------------------------------------------------------------------
// Interface
// ---------------------------------------------------------------------------

/**
 * Reputation Registry -- records feedback from clients about registered agents.
 *
 * Supports fixed-point value ratings with categorical tags, off-chain detail
 * URIs with Keccak256 integrity hashes, agent responses, and tag-filtered
 * summary queries.
 */
export interface IReputationRegistry {
  /**
   * Record feedback for a registered agent.
   *
   * FR-C-20: Records feedback from msg.sender. Assigns sequential feedbackIndex.
   * FR-C-21: value/decimals encode a fixed-point rating. decimals MUST be 0-18.
   * FR-C-25: Emits FeedbackGiven event.
   * FR-C-26: Reverts if agentId does not exist in Identity Registry.
   *
   * @param agentId - The agent's tokenId from the Identity Registry.
   * @param value - Signed fixed-point value (numerator). Rating = value / 10^decimals.
   * @param decimals - Decimal places (0-18).
   * @param tag1 - First categorical tag (bytes32). bytes32(0) for no tag.
   * @param tag2 - Second categorical tag (bytes32). bytes32(0) for no tag.
   * @param feedbackURI - Off-chain URI with feedback details.
   * @param feedbackHash - Keccak256 hash of the feedbackURI content.
   * @returns The sequential feedbackIndex for this (agentId, client) pair.
   */
  giveFeedback(
    agentId: Uint256,
    value: Int128,
    decimals: number,
    tag1: Bytes32,
    tag2: Bytes32,
    feedbackURI: string,
    feedbackHash: Bytes32,
  ): Promise<Uint256>;

  /**
   * Revoke previously given feedback.
   *
   * FR-C-22: Only the original feedback giver (msg.sender matches recorded client)
   *          may revoke. Revoked feedback is excluded from summary calculations.
   * FR-C-25: Emits FeedbackRevoked event.
   *
   * @param agentId - The agent's tokenId.
   * @param feedbackIndex - The index of the feedback entry to revoke.
   */
  revokeFeedback(agentId: Uint256, feedbackIndex: Uint256): Promise<void>;

  /**
   * Append a response to existing feedback.
   *
   * FR-C-23: Only the agent owner (owner of agentId in Identity Registry)
   *          may append a response.
   * FR-C-25: Emits ResponseAppended event.
   *
   * @param agentId - The agent's tokenId.
   * @param clientAddress - The address of the client who gave the feedback.
   * @param feedbackIndex - The index of the feedback entry to respond to.
   * @param responseURI - Off-chain URI with the agent's response.
   * @param responseHash - Keccak256 hash of the responseURI content.
   */
  appendResponse(
    agentId: Uint256,
    clientAddress: Address,
    feedbackIndex: Uint256,
    responseURI: string,
    responseHash: Bytes32,
  ): Promise<void>;

  /**
   * Query aggregated feedback summary for an agent.
   *
   * FR-C-24: Returns count and totalValue filtered by tags and client addresses.
   *          bytes32(0) tags are wildcards. Empty clientAddresses includes all clients.
   *          Revoked feedback is excluded.
   *
   * @param agentId - The agent's tokenId.
   * @param clientAddresses - Filter by specific clients. Empty array = all clients.
   * @param tag1 - Filter by first tag. bytes32(0) = wildcard.
   * @param tag2 - Filter by second tag. bytes32(0) = wildcard.
   * @returns Aggregated feedback summary (count, totalValue, decimals).
   */
  getSummary(
    agentId: Uint256,
    clientAddresses: ReadonlyArray<Address>,
    tag1: Bytes32,
    tag2: Bytes32,
  ): Promise<FeedbackSummary>;

  /**
   * Query individual feedback entries for an agent.
   *
   * Convenience method for retrieving full FeedbackEntry objects.
   * Useful for displaying feedback details rather than just summaries.
   *
   * @param agentId - The agent's tokenId.
   * @param tags - Filter tags. bytes32(0) entries are wildcards.
   * @returns Array of matching FeedbackEntry objects.
   */
  getFeedbackEntries(
    agentId: Uint256,
    tags: ReadonlyArray<Bytes32>,
  ): Promise<ReadonlyArray<FeedbackEntry>>;
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

/**
 * Emitted when feedback is given for an agent.
 *
 * FR-C-25: FeedbackGiven(uint256 indexed agentId, address indexed client,
 *          uint256 feedbackIndex, int128 value, uint8 decimals, bytes32 tag1, bytes32 tag2).
 */
export interface FeedbackGivenEvent {
  /** The agent's tokenId from Identity Registry. FR-C-20 */
  readonly agentId: Uint256;

  /** Address of the feedback giver (msg.sender). FR-C-20 */
  readonly client: Address;

  /** Sequential feedback index for this (agentId, client) pair. FR-C-20 */
  readonly feedbackIndex: Uint256;

  /** Signed fixed-point value. FR-C-21 */
  readonly value: Int128;

  /** Decimal places (0-18). FR-C-21 */
  readonly decimals: number;

  /** First categorical tag. FR-C-20 */
  readonly tag1: Bytes32;

  /** Second categorical tag. FR-C-20 */
  readonly tag2: Bytes32;
}

/**
 * Emitted when feedback is revoked.
 *
 * FR-C-25: FeedbackRevoked(uint256 indexed agentId, address indexed client, uint256 feedbackIndex).
 */
export interface FeedbackRevokedEvent {
  /** The agent's tokenId. FR-C-22 */
  readonly agentId: Uint256;

  /** Address of the feedback giver who revoked. FR-C-22 */
  readonly client: Address;

  /** The revoked feedback index. FR-C-22 */
  readonly feedbackIndex: Uint256;
}

/**
 * Emitted when an agent appends a response to feedback.
 *
 * FR-C-25: ResponseAppended(uint256 indexed agentId, address indexed clientAddress, uint256 feedbackIndex).
 */
export interface ResponseAppendedEvent {
  /** The agent's tokenId. FR-C-23 */
  readonly agentId: Uint256;

  /** Address of the client whose feedback was responded to. FR-C-23 */
  readonly clientAddress: Address;

  /** The feedback index that received a response. FR-C-23 */
  readonly feedbackIndex: Uint256;
}
