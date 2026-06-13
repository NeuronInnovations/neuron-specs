/**
 * Chain 2: TopicMessage Signing — Golden test vector conformance.
 *
 * Source: specs/006-protocol-determinism/contracts/test-vectors.md §Chain 2
 * Verifies the TopicMessage signing chain: pre-image → Keccak256 → RFC 6979 → canonical JSON.
 * Every intermediate hex value is asserted.
 */

import { describe, it, expect } from 'vitest';
import { CHAIN2 } from './vectors.js';
import { hexToBytes, bytesToHex } from '../../src/wire/index.js';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import { TopicMessage } from '../../src/topic/message.js';

describe('Chain 2: TopicMessage Signing', () => {
  const payloadBytes = hexToBytes(CHAIN2.payloadHex);

  // FR-A08: Keccak256 pre-image for TopicMessage
  // Pre-image: timestamp (8 bytes BE) || sequenceNumber (8 bytes BE) || payload
  it('should construct correct signing pre-image', () => {
    const preimage = TopicMessage.buildPreimage(
      CHAIN2.timestamp,
      CHAIN2.sequenceNumber,
      payloadBytes,
    );
    expect(bytesToHex(preimage).toLowerCase()).toBe(CHAIN2.preimageHex.toLowerCase());
  });

  // FR-A08: Keccak256 hash of pre-image
  it('should produce correct Keccak256 hash of pre-image', () => {
    const hash = TopicMessage.hashPreimage(
      CHAIN2.timestamp,
      CHAIN2.sequenceNumber,
      payloadBytes,
    );
    expect(bytesToHex(hash).toLowerCase()).toBe(CHAIN2.signingHashHex.toLowerCase());
  });

  // FR-A07: RFC 6979 deterministic signing
  // FR-A10: ECDSA R||S||V encoding (65 bytes)
  it('should produce correct ECDSA signature (R||S||V)', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
    const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
    expect(bytesToHex(msg.signatureBytes()).toLowerCase()).toBe(
      CHAIN2.signatureHex.toLowerCase(),
    );
  });

  // Signature components
  it('should have correct R component', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
    const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
    const sig = msg.signatureBytes();
    expect(bytesToHex(sig.slice(0, 32)).toLowerCase()).toBe(CHAIN2.signatureRHex.toLowerCase());
  });

  it('should have correct S component', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
    const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
    const sig = msg.signatureBytes();
    expect(bytesToHex(sig.slice(32, 64)).toLowerCase()).toBe(CHAIN2.signatureSHex.toLowerCase());
  });

  it('should have correct V (recovery ID)', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
    const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
    const sig = msg.signatureBytes();
    expect(sig[64]).toBe(CHAIN2.signatureV);
  });

  // FR-W03: Signature as base64
  it('should encode signature as correct base64', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
    const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
    expect(msg.signatureBase64()).toBe(CHAIN2.signatureBase64);
  });

  // FR-W03: Payload as base64
  it('should encode payload as correct base64', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
    const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
    expect(msg.payloadBase64()).toBe(CHAIN2.payloadBase64);
  });

  // FR-W05: Canonical JSON with field order senderAddress → signature → timestamp → sequenceNumber → payload
  // FR-W01: Compact format
  // FR-W02: UnsignedInt64 as JSON strings
  // FR-W06: EVM address in EIP-55 checksum
  it('should produce byte-identical canonical JSON', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
    const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
    expect(msg.toCanonicalJson()).toBe(CHAIN2.canonicalJson);
  });

  // Constitution X: Deterministic signing — sign same message twice, assert byte-equal
  it('should produce identical signatures on repeated signing (determinism)', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
    const msg1 = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
    const msg2 = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
    expect(bytesToHex(msg1.signatureBytes())).toBe(bytesToHex(msg2.signatureBytes()));
    expect(msg1.toCanonicalJson()).toBe(msg2.toCanonicalJson());
  });
});
