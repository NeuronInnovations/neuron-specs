/**
 * Tests for TopicError -- TOPIC domain error factories.
 *
 * Spec reference: 006 error-taxonomy.md, TOPIC Domain (NEURON-TOPIC-001..010)
 * FR-T11: All 11 error kinds must be covered.
 */

import { describe, it, expect } from 'vitest';
import {
  TopicError,
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
} from '../../src/topic/errors.js';
import { NeuronError } from '../../src/errors.js';

describe('TopicError', () => {
  it('should extend NeuronError', () => {
    const err = invalidTopicRef('test');
    expect(err).toBeInstanceOf(TopicError);
    expect(err).toBeInstanceOf(NeuronError);
    expect(err).toBeInstanceOf(Error);
  });

  it('should preserve message, name, and code', () => {
    const err = invalidTopicRef('bad ref');
    expect(err.code).toBe('NEURON-TOPIC-001');
    expect(err.name).toBe('InvalidTopicRef');
    expect(err.message).toBe('bad ref');
  });

  it('should preserve optional cause', () => {
    const cause = new Error('underlying');
    const err = publishFailed('publish failed', cause);
    expect(err.cause).toBe(cause);
  });

  it('should have undefined cause when not provided', () => {
    const err = invalidSignature('bad sig');
    expect(err.cause).toBeUndefined();
  });
});

describe('TopicError factory functions', () => {
  const factories = [
    { fn: invalidTopicRef, code: 'NEURON-TOPIC-001', name: 'InvalidTopicRef' },
    { fn: unsupportedOperation, code: 'NEURON-TOPIC-002', name: 'UnsupportedOperation' },
    { fn: invalidSignature, code: 'NEURON-TOPIC-003', name: 'InvalidSignature' },
    { fn: sequenceViolation, code: 'NEURON-TOPIC-004', name: 'SequenceViolation' },
    { fn: payloadTooLarge, code: 'NEURON-TOPIC-005', name: 'PayloadTooLarge' },
    { fn: adapterNotRegistered, code: 'NEURON-TOPIC-006', name: 'AdapterNotRegistered' },
    { fn: publishFailed, code: 'NEURON-TOPIC-007', name: 'PublishFailed' },
    { fn: subscribeFailed, code: 'NEURON-TOPIC-008', name: 'SubscribeFailed' },
    { fn: topicResolveFailed, code: 'NEURON-TOPIC-009', name: 'TopicResolveFailed' },
    { fn: invalidTimestamp, code: 'NEURON-TOPIC-010', name: 'InvalidTimestamp' },
  ];

  for (const { fn, code, name } of factories) {
    it(`${name} (${code}) should produce correct error`, () => {
      const err = fn('test message');
      expect(err.code).toBe(code);
      expect(err.name).toBe(name);
      expect(err.message).toBe('test message');
      expect(err).toBeInstanceOf(TopicError);
    });
  }

  it('should cover all 10 TOPIC error codes', () => {
    expect(factories).toHaveLength(10);
  });
});
