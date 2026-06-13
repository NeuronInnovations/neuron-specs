/**
 * Chain 3: HeartbeatPayload Signing — Golden test vector conformance.
 *
 * Source: specs/006-protocol-determinism/contracts/test-vectors.md §Chain 3
 * Verifies HeartbeatPayload canonical serialization within TopicMessage signing chain.
 * HeartbeatPayload JSON → TopicMessage payload → sign envelope.
 */

import { describe, it, expect } from 'vitest';
import { CHAIN3 } from './vectors.js';
import { bytesToHex } from '../../src/wire/index.js';
import { canonicalJsonToBytes } from '../../src/wire/canonical-json.js';
import { base64Encode } from '../../src/wire/base64.js';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import { HeartbeatPayload } from '../../src/health/payload.js';
import { TopicMessage } from '../../src/topic/message.js';

describe('Chain 3: HeartbeatPayload Signing', () => {
  // §3.1 HeartbeatPayload Canonical JSON
  // FR-W05: Field order: type → version → nextHeartbeatDeadline → role → capabilities? → location? → peers?
  // FR-W02: nextHeartbeatDeadline as JSON string
  // FR-W04: location and peers absent → omitted
  it('should serialize HeartbeatPayload to correct canonical JSON', () => {
    const payload = HeartbeatPayload.build({
      nextHeartbeatDeadline: CHAIN3.nextHeartbeatDeadline,
      role: CHAIN3.role,
      capabilities: CHAIN3.capabilities,
    });
    expect(payload.toCanonicalJson()).toBe(CHAIN3.payloadJson);
  });

  // Verify payload hex matches (JSON bytes as UTF-8)
  it('should produce correct payload hex (UTF-8 bytes of JSON)', () => {
    const payload = HeartbeatPayload.build({
      nextHeartbeatDeadline: CHAIN3.nextHeartbeatDeadline,
      role: CHAIN3.role,
      capabilities: CHAIN3.capabilities,
    });
    const jsonBytes = canonicalJsonToBytes(payload.toCanonicalJson());
    expect(bytesToHex(jsonBytes).toLowerCase()).toBe(CHAIN3.payloadHex.toLowerCase());
  });

  // Verify payload base64
  it('should produce correct payload base64', () => {
    const payload = HeartbeatPayload.build({
      nextHeartbeatDeadline: CHAIN3.nextHeartbeatDeadline,
      role: CHAIN3.role,
      capabilities: CHAIN3.capabilities,
    });
    const jsonBytes = canonicalJsonToBytes(payload.toCanonicalJson());
    expect(base64Encode(jsonBytes)).toBe(CHAIN3.payloadBase64);
  });

  // §3.2 TopicMessage signing with HeartbeatPayload as payload
  // FR-A09: HeartbeatPayload → canonical JSON → payload bytes → TopicMessage pre-image
  it('should produce correct signing pre-image', () => {
    const payload = HeartbeatPayload.build({
      nextHeartbeatDeadline: CHAIN3.nextHeartbeatDeadline,
      role: CHAIN3.role,
      capabilities: CHAIN3.capabilities,
    });
    const payloadBytes = canonicalJsonToBytes(payload.toCanonicalJson());
    const preimage = TopicMessage.buildPreimage(
      CHAIN3.timestamp,
      CHAIN3.sequenceNumber,
      payloadBytes,
    );
    expect(bytesToHex(preimage).toLowerCase()).toBe(CHAIN3.signingPreimageHex.toLowerCase());
  });

  it('should produce correct signing hash', () => {
    const payload = HeartbeatPayload.build({
      nextHeartbeatDeadline: CHAIN3.nextHeartbeatDeadline,
      role: CHAIN3.role,
      capabilities: CHAIN3.capabilities,
    });
    const payloadBytes = canonicalJsonToBytes(payload.toCanonicalJson());
    const hash = TopicMessage.hashPreimage(
      CHAIN3.timestamp,
      CHAIN3.sequenceNumber,
      payloadBytes,
    );
    expect(bytesToHex(hash).toLowerCase()).toBe(CHAIN3.signingHashHex.toLowerCase());
  });

  // FR-A07: RFC 6979 signature
  it('should produce correct ECDSA signature', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN3.privateKeyHex);
    const payload = HeartbeatPayload.build({
      nextHeartbeatDeadline: CHAIN3.nextHeartbeatDeadline,
      role: CHAIN3.role,
      capabilities: CHAIN3.capabilities,
    });
    const payloadBytes = canonicalJsonToBytes(payload.toCanonicalJson());
    const msg = TopicMessage.create(key, CHAIN3.timestamp, CHAIN3.sequenceNumber, payloadBytes);
    expect(bytesToHex(msg.signatureBytes()).toLowerCase()).toBe(
      CHAIN3.signatureHex.toLowerCase(),
    );
    expect(msg.signatureBase64()).toBe(CHAIN3.signatureBase64);
  });

  // §3.3 Complete TopicMessage canonical JSON
  it('should produce byte-identical complete TopicMessage canonical JSON', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN3.privateKeyHex);
    const payload = HeartbeatPayload.build({
      nextHeartbeatDeadline: CHAIN3.nextHeartbeatDeadline,
      role: CHAIN3.role,
      capabilities: CHAIN3.capabilities,
    });
    const payloadBytes = canonicalJsonToBytes(payload.toCanonicalJson());
    const msg = TopicMessage.create(key, CHAIN3.timestamp, CHAIN3.sequenceNumber, payloadBytes);
    expect(msg.toCanonicalJson()).toBe(CHAIN3.canonicalJson);
  });

  // Constitution X: Determinism
  it('should produce identical results on repeated serialization + signing', () => {
    const key = NeuronPrivateKey.fromHex(CHAIN3.privateKeyHex);
    const buildAndSign = (): string => {
      const payload = HeartbeatPayload.build({
        nextHeartbeatDeadline: CHAIN3.nextHeartbeatDeadline,
        role: CHAIN3.role,
        capabilities: CHAIN3.capabilities,
      });
      const payloadBytes = canonicalJsonToBytes(payload.toCanonicalJson());
      const msg = TopicMessage.create(key, CHAIN3.timestamp, CHAIN3.sequenceNumber, payloadBytes);
      return msg.toCanonicalJson();
    };
    expect(buildAndSign()).toBe(buildAndSign());
  });
});
