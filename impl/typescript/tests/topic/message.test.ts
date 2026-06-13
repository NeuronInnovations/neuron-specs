/**
 * Tests for TopicMessage -- signing, serialization, and verification.
 *
 * Spec reference: 004 spec.md FR-T02, FR-T03, FR-T20, FR-T21
 * Spec reference: 006 algorithm-reference.md FR-A07, FR-A08, FR-A10
 * Spec reference: 006 wire-format.md FR-W01, FR-W02, FR-W03, FR-W05, FR-W06
 *
 * Uses Chain 2 test vector values from 006 test-vectors.md to verify
 * byte-identical output at every intermediate step.
 */

import { describe, it, expect } from 'vitest';
import { CHAIN2 } from '../conformance/vectors.js';
import { hexToBytes, bytesToHex } from '../../src/wire/index.js';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import { TopicMessage } from '../../src/topic/message.js';

describe('TopicMessage', () => {
  const payloadBytes = hexToBytes(CHAIN2.payloadHex);

  describe('buildPreimage', () => {
    // FR-A08: Pre-image = timestamp (8 BE) || sequenceNumber (8 BE) || payload
    it('should construct correct signing pre-image from Chain 2 vector', () => {
      const preimage = TopicMessage.buildPreimage(
        CHAIN2.timestamp,
        CHAIN2.sequenceNumber,
        payloadBytes,
      );
      expect(bytesToHex(preimage).toLowerCase()).toBe(CHAIN2.preimageHex.toLowerCase());
    });

    it('should produce preimage of correct length (16 + payload)', () => {
      const payload = new Uint8Array([1, 2, 3]);
      const preimage = TopicMessage.buildPreimage(0n, 0n, payload);
      expect(preimage.length).toBe(16 + 3);
    });

    it('should handle empty payload', () => {
      const preimage = TopicMessage.buildPreimage(0n, 0n, new Uint8Array(0));
      expect(preimage.length).toBe(16);
    });
  });

  describe('hashPreimage', () => {
    // FR-A08: Keccak256 hash of pre-image
    it('should produce correct Keccak256 hash from Chain 2 vector', () => {
      const hash = TopicMessage.hashPreimage(
        CHAIN2.timestamp,
        CHAIN2.sequenceNumber,
        payloadBytes,
      );
      expect(bytesToHex(hash).toLowerCase()).toBe(CHAIN2.signingHashHex.toLowerCase());
    });

    it('should produce 32-byte hash', () => {
      const hash = TopicMessage.hashPreimage(0n, 0n, new Uint8Array(0));
      expect(hash.length).toBe(32);
    });
  });

  describe('create', () => {
    it('should produce correct sender address from Chain 2 vector', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      expect(msg.senderAddress).toBe(CHAIN2.senderAddress);
    });

    it('should store timestamp and sequence number', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      expect(msg.timestamp).toBe(CHAIN2.timestamp);
      expect(msg.sequenceNumber).toBe(CHAIN2.sequenceNumber);
    });

    it('should store payload as defensive copy', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const original = new Uint8Array(payloadBytes);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, original);

      // Mutate original -- should not affect message
      original[0] = 0xff;
      expect(msg.payload[0]).not.toBe(0xff);
    });
  });

  describe('signatureBytes', () => {
    // FR-A10: 65-byte R||S||V signature
    it('should produce correct signature from Chain 2 vector', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      expect(bytesToHex(msg.signatureBytes()).toLowerCase()).toBe(
        CHAIN2.signatureHex.toLowerCase(),
      );
    });

    it('should be 65 bytes', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      expect(msg.signatureBytes().length).toBe(65);
    });

    it('should return a defensive copy', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      const sig1 = msg.signatureBytes();
      const sig2 = msg.signatureBytes();
      sig1[0] = 0xff;
      expect(sig2[0]).not.toBe(0xff);
    });

    it('should have correct R component', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      const sig = msg.signatureBytes();
      expect(bytesToHex(sig.slice(0, 32)).toLowerCase()).toBe(
        CHAIN2.signatureRHex.toLowerCase(),
      );
    });

    it('should have correct S component', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      const sig = msg.signatureBytes();
      expect(bytesToHex(sig.slice(32, 64)).toLowerCase()).toBe(
        CHAIN2.signatureSHex.toLowerCase(),
      );
    });

    it('should have correct V (recovery ID)', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      const sig = msg.signatureBytes();
      expect(sig[64]).toBe(CHAIN2.signatureV);
    });
  });

  describe('signatureBase64', () => {
    // FR-W03: Signature as base64
    it('should produce correct base64 from Chain 2 vector', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      expect(msg.signatureBase64()).toBe(CHAIN2.signatureBase64);
    });
  });

  describe('payloadBase64', () => {
    // FR-W03: Payload as base64
    it('should produce correct base64 from Chain 2 vector', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      expect(msg.payloadBase64()).toBe(CHAIN2.payloadBase64);
    });
  });

  describe('toCanonicalJson', () => {
    // FR-W05: Canonical JSON field order
    // FR-W01: Compact format
    // FR-W02: UnsignedInt64 as JSON strings
    // FR-W06: EVM address in EIP-55 checksum
    it('should produce byte-identical canonical JSON from Chain 2 vector', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      expect(msg.toCanonicalJson()).toBe(CHAIN2.canonicalJson);
    });

    it('should have fields in canonical order', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      const json = msg.toCanonicalJson();

      // Verify field order by checking positions
      const senderIdx = json.indexOf('"senderAddress"');
      const sigIdx = json.indexOf('"signature"');
      const tsIdx = json.indexOf('"timestamp"');
      const seqIdx = json.indexOf('"sequenceNumber"');
      const payloadIdx = json.indexOf('"payload"');

      expect(senderIdx).toBeLessThan(sigIdx);
      expect(sigIdx).toBeLessThan(tsIdx);
      expect(tsIdx).toBeLessThan(seqIdx);
      expect(seqIdx).toBeLessThan(payloadIdx);
    });

    it('should have no whitespace (compact format)', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      const json = msg.toCanonicalJson();

      // No spaces between tokens (except inside string values)
      expect(json).not.toContain(': ');
      expect(json).not.toContain(', ');
    });

    it('should encode UnsignedInt64 as JSON strings (FR-W02)', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg = TopicMessage.create(key, CHAIN2.timestamp, CHAIN2.sequenceNumber, payloadBytes);
      const json = msg.toCanonicalJson();

      // Timestamp and sequenceNumber must be quoted strings
      expect(json).toContain('"timestamp":"1700000000000000000"');
      expect(json).toContain('"sequenceNumber":"1"');
    });
  });

  describe('fromFields', () => {
    it('should reconstruct a TopicMessage from raw fields', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const original = TopicMessage.create(
        key,
        CHAIN2.timestamp,
        CHAIN2.sequenceNumber,
        payloadBytes,
      );

      const reconstructed = TopicMessage.fromFields(
        original.senderAddress,
        original.signatureBytes(),
        original.timestamp,
        original.sequenceNumber,
        original.payload,
      );

      expect(reconstructed.senderAddress).toBe(original.senderAddress);
      expect(reconstructed.timestamp).toBe(original.timestamp);
      expect(reconstructed.sequenceNumber).toBe(original.sequenceNumber);
      expect(reconstructed.toCanonicalJson()).toBe(original.toCanonicalJson());
    });
  });

  describe('determinism (Constitution X)', () => {
    // Constitution X: Deterministic signing
    it('should produce identical signatures on repeated signing', () => {
      const key = NeuronPrivateKey.fromHex(CHAIN2.privateKeyHex);
      const msg1 = TopicMessage.create(
        key,
        CHAIN2.timestamp,
        CHAIN2.sequenceNumber,
        payloadBytes,
      );
      const msg2 = TopicMessage.create(
        key,
        CHAIN2.timestamp,
        CHAIN2.sequenceNumber,
        payloadBytes,
      );
      expect(bytesToHex(msg1.signatureBytes())).toBe(bytesToHex(msg2.signatureBytes()));
      expect(msg1.toCanonicalJson()).toBe(msg2.toCanonicalJson());
    });
  });
});
