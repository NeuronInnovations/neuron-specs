/**
 * MessageDelivery -- wrapper returned by TopicAdapter.subscribe() for each message.
 *
 * Spec reference: 004 spec.md FR-T24, FR-T25, FR-T29
 *   - FR-T24: MessageDelivery wraps TopicMessage with backend metadata.
 *   - FR-T25: BackendSequence enables at-least-once delivery deduplication.
 *   - FR-T29: ConsensusTimestamp mapping per backend (authoritative, not sender-reported).
 *
 * Spec reference: 004 data-model.md MessageDelivery entity.
 *
 * Immutable value type.
 */

import type { TopicMessage } from './message.js';

/**
 * A received message with backend-assigned metadata.
 *
 * FR-T24: Each message delivered via Subscribe() is wrapped in a MessageDelivery
 * that provides the authoritative backend consensus timestamp and sequence number.
 *
 * FR-T29: The consensusTimestamp is assigned by the backend, NOT the sender's
 * self-reported timestamp. The adapter MUST NOT use the sender's timestamp.
 *
 * FR-T25: Consumers use backendSequence for at-least-once delivery deduplication.
 */
export interface MessageDelivery {
  /**
   * The deserialized TopicMessage envelope.
   * FR-T24: The original signed message as received from the topic.
   */
  readonly message: TopicMessage;

  /**
   * Authoritative backend clock when the message was finalized (nanoseconds).
   * FR-T24, FR-T29: Source depends on backend:
   * - HCS: Hedera consensus timestamp
   * - ERC-log: block.timestamp (converted to nanoseconds)
   * - Kafka: Anchor timestamp from HCS anchoring proof
   */
  readonly consensusTimestamp: bigint;

  /**
   * Backend-native ordering number.
   * FR-T24, FR-T25: Used for resumption and deduplication:
   * - HCS: topic sequence number
   * - ERC-log: blockNumber * 10000 + logIndex
   * - Kafka: partition offset
   */
  readonly backendSequence: bigint;
}
