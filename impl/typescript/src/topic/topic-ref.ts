/**
 * TopicRef -- immutable reference to a topic on a specific transport backend.
 *
 * Spec reference: 004 spec.md
 *   - FR-T01: TopicRef uniquely identifies a topic via (transport, locator) pair.
 *   - FR-T12: TopicRef validation rejects invalid transport kinds and empty locators.
 *
 * Spec reference: 004 data-model.md
 *   - TopicRef entity definition with Transport, Locator, URI(), Validate().
 *   - Topic URI Scheme: hcs://<topicId>, erc-log://<chainId>:<contractAddress>, etc.
 *
 * Immutable value type. Valid by construction -- the factory function rejects
 * invalid inputs with InvalidTopicRef error.
 */

import type { BackendKind } from './types.js';
import { isValidBackendKind } from './types.js';
import { invalidTopicRef } from './errors.js';

/**
 * A globally unique reference to a topic.
 *
 * FR-T01: Every topic operation requires a TopicRef. The pair (transport, locator)
 * MUST be globally unique within the system.
 *
 * FR-T12: Validated at construction time. If a TopicRef instance exists, it is
 * guaranteed to have a valid transport kind and non-empty locator.
 */
export class TopicRef {
  /** Transport backend kind. FR-T01 */
  private readonly _transport: BackendKind;

  /** Backend-specific topic address. FR-T01 */
  private readonly _locator: string;

  /** @internal -- use static factory methods instead. */
  private constructor(transport: BackendKind, locator: string) {
    this._transport = transport;
    this._locator = locator;
  }

  // ---------------------------------------------------------------------------
  // Factory methods
  // ---------------------------------------------------------------------------

  /**
   * Create a new TopicRef with validation.
   *
   * FR-T01: Transport and locator are the two required fields.
   * FR-T12: Validates transport is a valid BackendKind and locator is non-empty.
   *
   * @param transport - Backend kind (e.g., 'hcs', 'erc-log', 'kafka', 'custom:...')
   * @param locator - Backend-specific topic address (e.g., HCS topic ID, contract address)
   * @returns TopicRef instance
   * @throws TopicError NEURON-TOPIC-001 if transport is invalid or locator is empty
   */
  static create(transport: string, locator: string): TopicRef {
    if (!isValidBackendKind(transport)) {
      throw invalidTopicRef(
        `Invalid transport kind: "${transport}". Must be 'hcs', 'erc-log', 'kafka', or 'custom:<type>'`,
      );
    }

    if (locator.length === 0) {
      throw invalidTopicRef(
        'TopicRef locator must be non-empty',
      );
    }

    return new TopicRef(transport as BackendKind, locator);
  }

  /**
   * Parse a TopicRef from a compact Topic URI string.
   *
   * Supported URI schemes:
   * - `hcs://<topicId>`                        -> transport='hcs', locator=topicId
   * - `erc-log://<chainId>:<contractAddress>`   -> transport='erc-log', locator=chainId:contractAddress
   * - `kafka+ledger://<broker>/<topicName>`     -> transport='kafka', locator=topicName
   *
   * @param uri - Compact Topic URI string
   * @returns TopicRef instance
   * @throws TopicError NEURON-TOPIC-001 if URI scheme is unrecognized or malformed
   */
  static fromURI(uri: string): TopicRef {
    if (uri.startsWith('hcs://')) {
      const locator = uri.slice(6); // Length of 'hcs://'
      if (locator.length === 0) {
        throw invalidTopicRef('HCS URI has empty topic ID: ' + uri);
      }
      return new TopicRef('hcs', locator);
    }

    if (uri.startsWith('erc-log://')) {
      const locator = uri.slice(10); // Length of 'erc-log://'
      if (locator.length === 0) {
        throw invalidTopicRef('ERC-log URI has empty locator: ' + uri);
      }
      return new TopicRef('erc-log', locator);
    }

    if (uri.startsWith('kafka+ledger://')) {
      const rest = uri.slice(15); // Length of 'kafka+ledger://'
      // Extract topic name after the broker (everything after the first '/')
      const slashIndex = rest.indexOf('/');
      if (slashIndex < 0 || slashIndex === rest.length - 1) {
        throw invalidTopicRef('Kafka URI has no topic name: ' + uri);
      }
      const locator = rest.slice(slashIndex + 1);
      return new TopicRef('kafka', locator);
    }

    throw invalidTopicRef(
      `Unrecognized Topic URI scheme: "${uri}". Expected hcs://, erc-log://, or kafka+ledger://`,
    );
  }

  // ---------------------------------------------------------------------------
  // Accessors
  // ---------------------------------------------------------------------------

  /** Transport backend kind. FR-T01 */
  get transport(): BackendKind {
    return this._transport;
  }

  /** Backend-specific topic locator. FR-T01 */
  get locator(): string {
    return this._locator;
  }

  // ---------------------------------------------------------------------------
  // Serialization
  // ---------------------------------------------------------------------------

  /**
   * Serialize to a compact Topic URI string.
   *
   * URI formats per spec Topic URI Scheme:
   * - hcs -> `hcs://<locator>`
   * - erc-log -> `erc-log://<locator>`
   * - kafka -> `kafka+ledger://<locator>`
   * - custom -> `custom://<locator>`
   *
   * @returns Compact Topic URI string
   */
  toURI(): string {
    switch (this._transport) {
      case 'hcs':
        return `hcs://${this._locator}`;
      case 'erc-log':
        return `erc-log://${this._locator}`;
      case 'kafka':
        return `kafka+ledger://${this._locator}`;
      default:
        // Custom transport
        return `${this._transport}://${this._locator}`;
    }
  }

  /**
   * Check equality with another TopicRef.
   *
   * FR-T01: Uniqueness is defined by the (transport, locator) pair.
   *
   * @param other - Another TopicRef to compare
   * @returns `true` if transport and locator are identical
   */
  equals(other: TopicRef): boolean {
    return this._transport === other._transport && this._locator === other._locator;
  }

  /**
   * String representation for debugging.
   *
   * @returns Human-readable string showing transport and locator
   */
  toString(): string {
    return `TopicRef(${this._transport}, ${this._locator})`;
  }
}
