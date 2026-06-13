/**
 * TopicError -- structured error type for the TOPIC domain.
 *
 * Spec reference: 006 error-taxonomy.md, TOPIC Domain (NEURON-TOPIC-001..010)
 * Spec reference: 004 spec.md FR-T11 (structured error types for topic operations)
 *
 * Each factory function returns a TopicError with the appropriate code and
 * human-readable name. The error extends NeuronError for consistent SDK error
 * handling across all domains.
 */

import {
  NeuronError,
  TOPIC_INVALID_TOPIC_REF,
  TOPIC_UNSUPPORTED_OPERATION,
  TOPIC_INVALID_SIGNATURE,
  TOPIC_SEQUENCE_VIOLATION,
  TOPIC_PAYLOAD_TOO_LARGE,
  TOPIC_ADAPTER_NOT_REGISTERED,
  TOPIC_PUBLISH_FAILED,
  TOPIC_SUBSCRIBE_FAILED,
  TOPIC_RESOLVE_FAILED,
  TOPIC_INVALID_TIMESTAMP,
} from '../errors.js';

/**
 * Domain-specific error for Topic System operations.
 *
 * All topic-related errors use codes in the NEURON-TOPIC-001..010 range.
 * FR-T11: Structured error types with specific error kinds.
 */
export class TopicError extends NeuronError {}

/**
 * NEURON-TOPIC-001: Invalid topic reference.
 *
 * Thrown when a TopicRef has an invalid transport kind or empty/malformed locator.
 * FR-T12: TopicRef validation rejects invalid inputs.
 *
 * @param message - Descriptive message about the validation failure
 * @param cause - Optional underlying error
 * @returns TopicError with code NEURON-TOPIC-001
 */
export function invalidTopicRef(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_INVALID_TOPIC_REF, 'InvalidTopicRef', message, cause);
}

/**
 * NEURON-TOPIC-002: Unsupported operation.
 *
 * Thrown when an operation is not supported by the adapter (e.g., publish on
 * a read-only adapter like ERC event log).
 * FR-T04: Read-only adapters return UnsupportedOperation for CreateTopic/Publish.
 *
 * @param message - Descriptive message about the unsupported operation
 * @param cause - Optional underlying error
 * @returns TopicError with code NEURON-TOPIC-002
 */
export function unsupportedOperation(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_UNSUPPORTED_OPERATION, 'UnsupportedOperation', message, cause);
}

/**
 * NEURON-TOPIC-003: Invalid signature.
 *
 * Thrown when a TopicMessage has an invalid or malformed signature,
 * or when signature recovery fails.
 * FR-T10: Signature verification and sender address recovery.
 *
 * @param message - Descriptive message about the signature failure
 * @param cause - Optional underlying error
 * @returns TopicError with code NEURON-TOPIC-003
 */
export function invalidSignature(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_INVALID_SIGNATURE, 'InvalidSignature', message, cause);
}

/**
 * NEURON-TOPIC-004: Sequence violation.
 *
 * Thrown when a TopicMessage has a sequence number that violates the
 * monotonically increasing requirement per sender per topic.
 * FR-T06: SequenceNumber MUST be monotonically increasing.
 *
 * @param message - Descriptive message about the sequence violation
 * @param cause - Optional underlying error
 * @returns TopicError with code NEURON-TOPIC-004
 */
export function sequenceViolation(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_SEQUENCE_VIOLATION, 'SequenceViolation', message, cause);
}

/**
 * NEURON-TOPIC-005: Payload too large.
 *
 * Thrown when a message payload exceeds the backend's maximum size limit.
 * FR-T27: Publish MUST check message size BEFORE submitting to backend.
 *
 * @param message - Descriptive message about the size violation
 * @param cause - Optional underlying error
 * @returns TopicError with code NEURON-TOPIC-005
 */
export function payloadTooLarge(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_PAYLOAD_TOO_LARGE, 'PayloadTooLarge', message, cause);
}

/**
 * NEURON-TOPIC-006: Adapter not registered.
 *
 * Thrown when no adapter is registered for the requested BackendKind.
 * FR-T05: Adapters are registered at runtime; unregistered kinds produce this error.
 *
 * @param message - Descriptive message identifying the missing adapter
 * @param cause - Optional underlying error
 * @returns TopicError with code NEURON-TOPIC-006
 */
export function adapterNotRegistered(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_ADAPTER_NOT_REGISTERED, 'AdapterNotRegistered', message, cause);
}

/**
 * NEURON-TOPIC-007: Publish failed.
 *
 * Thrown when a message publish operation fails after retries.
 * FR-T22: Publish returns typed error on permanent failure.
 *
 * @param message - Descriptive message about the publish failure
 * @param cause - Optional underlying error from the backend
 * @returns TopicError with code NEURON-TOPIC-007
 */
export function publishFailed(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_PUBLISH_FAILED, 'PublishFailed', message, cause);
}

/**
 * NEURON-TOPIC-008: Subscribe failed.
 *
 * Thrown when a subscription operation fails.
 * FR-T24: Subscribe returns typed error on permanent failure.
 *
 * @param message - Descriptive message about the subscription failure
 * @param cause - Optional underlying error from the backend
 * @returns TopicError with code NEURON-TOPIC-008
 */
export function subscribeFailed(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_SUBSCRIBE_FAILED, 'SubscribeFailed', message, cause);
}

/**
 * NEURON-TOPIC-009: Topic resolve failed.
 *
 * Thrown when topic metadata resolution fails (topic not found or backend error).
 * FR-T04: Resolve returns typed error on failure.
 *
 * @param message - Descriptive message about the resolution failure
 * @param cause - Optional underlying error from the backend
 * @returns TopicError with code NEURON-TOPIC-009
 */
export function topicResolveFailed(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_RESOLVE_FAILED, 'TopicResolveFailed', message, cause);
}

/**
 * NEURON-TOPIC-010: Invalid timestamp.
 *
 * Thrown when a TopicMessage timestamp is invalid (e.g., zero, negative, or
 * exceeds the UnsignedInt64 range).
 * FR-T02: Timestamp is a required field on TopicMessage.
 *
 * @param message - Descriptive message about the invalid timestamp
 * @param cause - Optional underlying error
 * @returns TopicError with code NEURON-TOPIC-010
 */
export function invalidTimestamp(message: string, cause?: Error): TopicError {
  return new TopicError(TOPIC_INVALID_TIMESTAMP, 'InvalidTimestamp', message, cause);
}
