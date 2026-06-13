/**
 * Tests for TopicRef -- immutable topic reference with validation.
 *
 * Spec reference: 004 spec.md
 *   - FR-T01: TopicRef uniquely identifies a topic via (transport, locator).
 *   - FR-T12: TopicRef validation rejects invalid inputs.
 *
 * Spec reference: 004 data-model.md TopicRef entity.
 */

import { describe, it, expect } from 'vitest';
import { TopicRef } from '../../src/topic/topic-ref.js';
import { TopicError } from '../../src/topic/errors.js';

describe('TopicRef.create', () => {
  it('should create a valid HCS TopicRef', () => {
    const ref = TopicRef.create('hcs', '0.0.4515382');
    expect(ref.transport).toBe('hcs');
    expect(ref.locator).toBe('0.0.4515382');
  });

  it('should create a valid ERC-log TopicRef', () => {
    const ref = TopicRef.create('erc-log', '1:0xAbCdEf1234567890AbCdEf1234567890AbCdEf12');
    expect(ref.transport).toBe('erc-log');
    expect(ref.locator).toBe('1:0xAbCdEf1234567890AbCdEf1234567890AbCdEf12');
  });

  it('should create a valid Kafka TopicRef', () => {
    const ref = TopicRef.create('kafka', 'neuron.agent.alice.stdout');
    expect(ref.transport).toBe('kafka');
    expect(ref.locator).toBe('neuron.agent.alice.stdout');
  });

  it('should create a valid custom TopicRef', () => {
    const ref = TopicRef.create('custom:mqtt', 'some/topic');
    expect(ref.transport).toBe('custom:mqtt');
    expect(ref.locator).toBe('some/topic');
  });

  it('should reject invalid transport kind', () => {
    expect(() => TopicRef.create('invalid', 'locator')).toThrow(TopicError);
    expect(() => TopicRef.create('', 'locator')).toThrow(TopicError);
    expect(() => TopicRef.create('HCS', 'locator')).toThrow(TopicError);
  });

  it('should reject empty locator', () => {
    expect(() => TopicRef.create('hcs', '')).toThrow(TopicError);
  });

  it('should reject custom transport with empty type', () => {
    expect(() => TopicRef.create('custom:', 'locator')).toThrow(TopicError);
  });
});

describe('TopicRef.fromURI', () => {
  it('should parse HCS URI', () => {
    const ref = TopicRef.fromURI('hcs://0.0.4515382');
    expect(ref.transport).toBe('hcs');
    expect(ref.locator).toBe('0.0.4515382');
  });

  it('should parse ERC-log URI', () => {
    const ref = TopicRef.fromURI('erc-log://1:0xAbCdEf1234567890AbCdEf1234567890AbCdEf12');
    expect(ref.transport).toBe('erc-log');
    expect(ref.locator).toBe('1:0xAbCdEf1234567890AbCdEf1234567890AbCdEf12');
  });

  it('should parse Kafka URI with topic name extraction', () => {
    const ref = TopicRef.fromURI('kafka+ledger://kafka1.neuron.network:9092/neuron.agent.alice.stdout');
    expect(ref.transport).toBe('kafka');
    expect(ref.locator).toBe('neuron.agent.alice.stdout');
  });

  it('should reject unknown URI scheme', () => {
    expect(() => TopicRef.fromURI('http://example.com')).toThrow(TopicError);
    expect(() => TopicRef.fromURI('invalid')).toThrow(TopicError);
  });

  it('should reject HCS URI with empty topic ID', () => {
    expect(() => TopicRef.fromURI('hcs://')).toThrow(TopicError);
  });

  it('should reject ERC-log URI with empty locator', () => {
    expect(() => TopicRef.fromURI('erc-log://')).toThrow(TopicError);
  });

  it('should reject Kafka URI with no topic name', () => {
    expect(() => TopicRef.fromURI('kafka+ledger://broker:9092/')).toThrow(TopicError);
    expect(() => TopicRef.fromURI('kafka+ledger://broker:9092')).toThrow(TopicError);
  });
});

describe('TopicRef.toURI', () => {
  it('should serialize HCS URI', () => {
    const ref = TopicRef.create('hcs', '0.0.4515382');
    expect(ref.toURI()).toBe('hcs://0.0.4515382');
  });

  it('should serialize ERC-log URI', () => {
    const ref = TopicRef.create('erc-log', '1:0xAbC');
    expect(ref.toURI()).toBe('erc-log://1:0xAbC');
  });

  it('should serialize Kafka URI', () => {
    const ref = TopicRef.create('kafka', 'neuron.agent.alice.stdout');
    expect(ref.toURI()).toBe('kafka+ledger://neuron.agent.alice.stdout');
  });

  it('should serialize custom URI', () => {
    const ref = TopicRef.create('custom:mqtt', 'my/topic');
    expect(ref.toURI()).toBe('custom:mqtt://my/topic');
  });

  it('should round-trip HCS URI', () => {
    const original = TopicRef.create('hcs', '0.0.4515382');
    const parsed = TopicRef.fromURI(original.toURI());
    expect(parsed.transport).toBe(original.transport);
    expect(parsed.locator).toBe(original.locator);
  });
});

describe('TopicRef.equals', () => {
  it('should return true for identical refs', () => {
    const a = TopicRef.create('hcs', '0.0.1');
    const b = TopicRef.create('hcs', '0.0.1');
    expect(a.equals(b)).toBe(true);
  });

  it('should return false for different transport', () => {
    const a = TopicRef.create('hcs', '0.0.1');
    const b = TopicRef.create('kafka', '0.0.1');
    expect(a.equals(b)).toBe(false);
  });

  it('should return false for different locator', () => {
    const a = TopicRef.create('hcs', '0.0.1');
    const b = TopicRef.create('hcs', '0.0.2');
    expect(a.equals(b)).toBe(false);
  });
});

describe('TopicRef.toString', () => {
  it('should produce human-readable string', () => {
    const ref = TopicRef.create('hcs', '0.0.4515382');
    expect(ref.toString()).toBe('TopicRef(hcs, 0.0.4515382)');
  });
});
