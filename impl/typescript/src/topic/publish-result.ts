/**
 * PublishResult -- result returned by TopicAdapter.publish().
 *
 * Spec reference: 004 spec.md FR-T22, FR-T23, FR-T29
 *   - FR-T22: PublishResult fields (transactionRef, consensusTimestamp, sequenceNumber, confirmed).
 *   - FR-T23: Two confirmation modes: FIRE_AND_FORGET and WAIT_FOR_CONSENSUS.
 *   - FR-T29: ConsensusTimestamp mapping per backend.
 *
 * Spec reference: 004 data-model.md PublishResult entity.
 *
 * Immutable value type.
 */

/**
 * Result returned by a successful Publish() operation.
 *
 * FR-T22: Contains the backend-specific transaction reference and optional
 * confirmation data (consensus timestamp and sequence number).
 *
 * FR-T23: The fields populated depend on the confirmation mode:
 * - FIRE_AND_FORGET: confirmed=false, consensusTimestamp=undefined, sequenceNumber=undefined
 * - WAIT_FOR_CONSENSUS: confirmed=true, consensusTimestamp and sequenceNumber populated
 */
export interface PublishResult {
  /**
   * Backend-specific transaction/receipt identifier.
   * FR-T22: Always present (e.g., Hedera transaction ID, Kafka partition:offset).
   */
  readonly transactionRef: string;

  /**
   * Authoritative backend clock at finalization (nanoseconds).
   * FR-T22, FR-T29: Absent if not confirmed (FIRE_AND_FORGET mode).
   */
  readonly consensusTimestamp?: bigint | undefined;

  /**
   * Backend-assigned sequence number.
   * FR-T22: Absent if not confirmed (FIRE_AND_FORGET mode).
   */
  readonly sequenceNumber?: bigint | undefined;

  /**
   * Whether the backend has finalized the message.
   * FR-T22, FR-T23: true for WAIT_FOR_CONSENSUS, false for FIRE_AND_FORGET.
   */
  readonly confirmed: boolean;
}

/**
 * Create a PublishResult for FIRE_AND_FORGET mode.
 *
 * FR-T23: confirmed=false, consensusTimestamp and sequenceNumber absent.
 *
 * @param transactionRef - Backend-specific transaction identifier
 * @returns PublishResult with confirmed=false
 */
export function fireAndForgetResult(transactionRef: string): PublishResult {
  return {
    transactionRef,
    consensusTimestamp: undefined,
    sequenceNumber: undefined,
    confirmed: false,
  };
}

/**
 * Create a PublishResult for WAIT_FOR_CONSENSUS mode.
 *
 * FR-T23: confirmed=true, all fields populated.
 *
 * @param transactionRef - Backend-specific transaction identifier
 * @param consensusTimestamp - Backend consensus timestamp (nanoseconds)
 * @param sequenceNumber - Backend-assigned sequence number
 * @returns PublishResult with confirmed=true and timestamps populated
 */
export function confirmedResult(
  transactionRef: string,
  consensusTimestamp: bigint,
  sequenceNumber: bigint,
): PublishResult {
  return {
    transactionRef,
    consensusTimestamp,
    sequenceNumber,
    confirmed: true,
  };
}
