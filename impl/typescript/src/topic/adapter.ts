/**
 * TopicAdapter -- interface abstracting backend-specific topic operations.
 *
 * Spec reference: 004 spec.md
 *   - FR-T04: TopicAdapter is the central abstraction for topic operations.
 *   - FR-T05: Adapter registry maps BackendKind to TopicAdapter at runtime.
 *   - FR-T22: Publish returns PublishResult.
 *   - FR-T23: ConfirmationMode controls publish behavior.
 *   - FR-T24: Subscribe returns AsyncGenerator of MessageDelivery.
 *   - FR-T27: MaxMessageSize returns maximum payload size.
 *   - FR-T28: EstimatePublishCost returns CostEstimate.
 *
 * Spec reference: 004 contracts/topic-adapter.md
 *   - Full adapter interface contract with capability requirements.
 *   - Read-only adapters MUST return UnsupportedOperation for CreateTopic/Publish.
 *
 * Spec reference: 004 data-model.md
 *   - TopicAdapter, CreateTopicOpts, PublishOpts, SubscribeOpts, TopicMetadata, CostEstimate.
 */

import type { NeuronPrivateKey } from '../keylib/private-key.js';
import type { NeuronPublicKey } from '../keylib/public-key.js';
import type { BackendKind, ConfirmationMode } from './types.js';
import type { TopicRef } from './topic-ref.js';
import type { TopicMessage } from './message.js';
import type { PublishResult } from './publish-result.js';
import type { MessageDelivery } from './message-delivery.js';
import { adapterNotRegistered } from './errors.js';

// ---------------------------------------------------------------------------
// Supporting types
// ---------------------------------------------------------------------------

/**
 * Options for creating a new topic on a backend.
 * FR-T04: CreateTopic requires transport kind, admin key, and optional config.
 */
export interface CreateTopicOpts {
  /** Which backend to create the topic on. FR-T04 */
  readonly transport: BackendKind;

  /** Key that controls the topic (from keylib 002). FR-T04 */
  readonly adminKey: NeuronPrivateKey;

  /** Optional topic description/memo. */
  readonly memo?: string | undefined;

  /** Backend-specific creation options. FR-T15 */
  readonly config?: Record<string, unknown> | undefined;
}

/**
 * Options for publishing a message.
 * FR-T23: ConfirmationMode controls whether to wait for consensus.
 */
export interface PublishOpts {
  /** Confirmation mode: FIRE_AND_FORGET or WAIT_FOR_CONSENSUS. FR-T23 */
  readonly confirmationMode: ConfirmationMode;
}

/**
 * Options for subscribing to a topic.
 * FR-T25: FromSequence enables resumption from a specific backend sequence.
 */
export interface SubscribeOpts {
  /**
   * Resume from this backend sequence number.
   * FR-T25: If set, the adapter MUST backfill messages from this sequence to current.
   * If absent, subscribe from latest.
   */
  readonly fromSequence?: bigint | undefined;
}

/**
 * Topic metadata returned by Resolve().
 * FR-T04: Contains current state of a topic on its backend.
 */
export interface TopicMetadata {
  /** The topic reference. FR-T04 */
  readonly topicRef: TopicRef;

  /** Current latest sequence number on the backend. FR-T04 */
  readonly sequenceNumber: bigint;

  /** Topic creation timestamp (nanoseconds). */
  readonly createdAt: bigint;

  /** Topic admin public key (if available). */
  readonly adminKey?: NeuronPublicKey | undefined;

  /** Topic memo/description. */
  readonly memo?: string | undefined;
}

/**
 * Estimated cost of publishing a message.
 * FR-T28: Amount and unit for per-message cost estimation.
 */
export interface CostEstimate {
  /** Estimated cost amount. FR-T28 */
  readonly amount: bigint;

  /** Cost unit (e.g., "tinybar", "wei", "USD-cents"). FR-T28 */
  readonly unit: string;
}

// ---------------------------------------------------------------------------
// TopicAdapter interface
// ---------------------------------------------------------------------------

/**
 * The central abstraction for backend-specific topic operations.
 *
 * FR-T04: All topic interactions go through this interface. Each supported
 * backend provides a concrete implementation.
 *
 * Capability contract:
 * - All adapters MUST implement subscribe(), resolve(), maxMessageSize(), supportedTransport().
 * - Read-write adapters MUST implement createTopic() and publish().
 * - Read-only adapters MUST return UnsupportedOperation from createTopic() and publish().
 * - All adapters SHOULD implement estimatePublishCost().
 */
export interface TopicAdapter {
  /**
   * Create a new topic on the backend.
   * FR-T04: Returns UnsupportedOperation on read-only backends.
   */
  createTopic(opts: CreateTopicOpts): Promise<TopicRef>;

  /**
   * Publish a signed TopicMessage to a topic.
   * FR-T04, FR-T22, FR-T23: Returns PublishResult based on confirmation mode.
   */
  publish(ref: TopicRef, msg: TopicMessage, opts?: PublishOpts): Promise<PublishResult>;

  /**
   * Subscribe to a topic message stream.
   * FR-T04, FR-T24, FR-T25: Returns async generator of MessageDelivery.
   */
  subscribe(ref: TopicRef, opts?: SubscribeOpts): AsyncGenerator<MessageDelivery>;

  /**
   * Resolve topic metadata from the backend.
   * FR-T04: Returns topic state including sequence number, creation time, admin key.
   */
  resolve(ref: TopicRef): Promise<TopicMetadata>;

  /**
   * Maximum payload size in bytes for this backend.
   * FR-T27: Publish MUST check message size against this limit BEFORE submitting.
   */
  maxMessageSize(): bigint;

  /**
   * Estimate the cost of publishing a message of the given size.
   * FR-T28: Returns CostEstimate or UnsupportedOperation.
   */
  estimatePublishCost(messageSize: bigint): Promise<CostEstimate>;

  /**
   * The transport kind this adapter handles.
   * FR-T01: Used for adapter registry lookup.
   */
  supportedTransport(): BackendKind;
}

// ---------------------------------------------------------------------------
// Adapter Registry
// ---------------------------------------------------------------------------

/**
 * Registry mapping BackendKind to TopicAdapter.
 *
 * FR-T05: Adapters are registered at runtime. Registering a new adapter
 * MUST NOT require changes to the core API.
 *
 * SC-T05: New adapters can be added without modifying existing code.
 */
export interface AdapterRegistry {
  /**
   * Register a TopicAdapter for its supported transport kind.
   * FR-T05: Registration MUST NOT silently replace existing adapters.
   *
   * @param kind - The BackendKind this adapter handles
   * @param adapter - The adapter implementation
   */
  register(kind: BackendKind, adapter: TopicAdapter): void;

  /**
   * Retrieve the registered adapter for a transport kind.
   *
   * @param kind - The BackendKind to look up
   * @returns The registered adapter, or undefined if not found
   */
  get(kind: BackendKind): TopicAdapter | undefined;
}

/**
 * Default in-memory adapter registry.
 *
 * FR-T05: Simple Map-based registry. Thread safety is not a concern
 * in single-threaded JavaScript environments.
 */
export class DefaultAdapterRegistry implements AdapterRegistry {
  private readonly _adapters = new Map<string, TopicAdapter>();

  /**
   * Register a TopicAdapter.
   *
   * FR-T05: Throws if an adapter is already registered for this transport kind.
   *
   * @param kind - BackendKind this adapter handles
   * @param adapter - The adapter implementation
   * @throws TopicError NEURON-TOPIC-006 if adapter already registered
   */
  register(kind: BackendKind, adapter: TopicAdapter): void {
    if (this._adapters.has(kind)) {
      throw adapterNotRegistered(
        `Adapter already registered for transport kind: "${kind}"`,
      );
    }
    this._adapters.set(kind, adapter);
  }

  /**
   * Retrieve a registered adapter.
   *
   * @param kind - The BackendKind to look up
   * @returns The registered adapter, or undefined if not found
   */
  get(kind: BackendKind): TopicAdapter | undefined {
    return this._adapters.get(kind);
  }

  /**
   * Check whether an adapter is registered for a transport kind.
   *
   * @param kind - The BackendKind to check
   * @returns `true` if an adapter is registered
   */
  has(kind: BackendKind): boolean {
    return this._adapters.has(kind);
  }

  /**
   * Return all registered transport kinds.
   *
   * @returns Array of registered BackendKind values
   */
  registeredKinds(): BackendKind[] {
    return Array.from(this._adapters.keys()) as BackendKind[];
  }
}
