/**
 * topic -- Neuron SDK Topic System barrel exports.
 *
 * Spec reference: 004 spec.md
 *
 * Re-exports all public types, factories, and utilities from the topic system.
 * Consumers should import from this module rather than individual files.
 *
 * Usage:
 *   import { TopicMessage, TopicRef, TopicAdapter } from '@neuron-sdk/topic';
 */

// --- Errors ---
export { TopicError } from './errors.js';
export {
  invalidTopicRef,
  unsupportedOperation,
  invalidSignature,
  sequenceViolation,
  payloadTooLarge,
  adapterNotRegistered,
  publishFailed,
  subscribeFailed,
  topicResolveFailed,
  invalidTimestamp,
} from './errors.js';

// --- Types and Enums ---
export type { BackendKind, ChannelRole, ConfirmationMode } from './types.js';
export {
  isReservedChannel,
  isValidChannelRole,
  isValidBackendKind,
  standardChannelRoles,
} from './types.js';

// --- TopicMessage ---
export { TopicMessage } from './message.js';

// --- TopicRef ---
export { TopicRef } from './topic-ref.js';

// --- Adapter ---
export type {
  TopicAdapter,
  AdapterRegistry,
  CreateTopicOpts,
  PublishOpts,
  SubscribeOpts,
  TopicMetadata,
  CostEstimate,
} from './adapter.js';
export { DefaultAdapterRegistry } from './adapter.js';

// --- PublishResult ---
export type { PublishResult } from './publish-result.js';
export { fireAndForgetResult, confirmedResult } from './publish-result.js';

// --- MessageDelivery ---
export type { MessageDelivery } from './message-delivery.js';

// --- Channel ---
export { channelRoleFromString, assertNotReservedChannel } from './channel.js';

// --- Service Schemas ---
export type {
  NeuronTopicService,
  NeuronP2PExchangeService,
  HCSConfig,
  ERCLogConfig,
  KafkaConfig,
  AnchoringConfig,
  TransportConfig,
} from './service.js';
export {
  parseNeuronTopicService,
  parseNeuronP2PExchangeService,
  parseAgentURIServices,
  validateCrossReferences,
  extractTopicRef,
} from './service.js';
