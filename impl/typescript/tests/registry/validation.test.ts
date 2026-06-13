/**
 * Tests for AgentURI validation -- V-REG-01..V-REG-12.
 *
 * Spec reference: 003 data-model.md, Validation Rules
 *   - V-REG-01..07, V-REG-11, V-REG-12: Locally checkable rules (implemented).
 *   - V-REG-08..10: On-chain checks (deferred, documented in this test).
 *
 * Uses a well-known test key to derive PeerID and DIDKey for validation.
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { NeuronPrivateKey } from '../../src/keylib/private-key.js';
import type { NeuronPublicKey } from '../../src/keylib/public-key.js';
import { validateRegistrationCompleteness } from '../../src/registry/validation.js';
import type { ValidationError } from '../../src/registry/validation.js';
import type {
  AgentURI,
  NeuronTopicServiceEntry,
  NeuronP2PExchangeEntry,
  DIDServiceEntry,
} from '../../src/registry/types.js';

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

/** Well-known test private key (32 bytes, valid secp256k1 scalar). */
const TEST_KEY_HEX = '0x0000000000000000000000000000000000000000000000000000000000000001';

let childPubKey: NeuronPublicKey;
let childPeerID: string;
let childDIDKey: string;

beforeAll(() => {
  const privKey = NeuronPrivateKey.fromHex(TEST_KEY_HEX);
  childPubKey = privKey.publicKey();
  childPeerID = childPubKey.peerId().toString();
  childDIDKey = childPubKey.didKey().toString();
});

/** Create a valid neuron-topic service entry for a given channel. */
function makeTopicService(channel: string): NeuronTopicServiceEntry {
  return {
    type: 'neuron-topic',
    name: channel,
    version: '1.0.0',
    channel,
    transport: 'hcs',
    anchor: 'hedera-mainnet',
    config: { network: 'hedera-mainnet', topicId: `0.0.${channel}` },
  };
}

/** Create a valid neuron-p2p-exchange service entry. */
function makeP2PService(peerID?: string, topicRef?: string): NeuronP2PExchangeEntry {
  return {
    type: 'neuron-p2p-exchange',
    name: 'p2p',
    version: '1.0.0',
    peerID: peerID ?? childPeerID,
    protocol: '/neuron/multiaddr-exchange/1.0.0',
    topicRef: topicRef ?? 'stdIn',
  };
}

/** Create a valid DID service entry. */
function makeDIDService(endpoint?: string): DIDServiceEntry {
  return {
    type: 'DID',
    name: 'DID',
    endpoint: endpoint ?? childDIDKey,
    version: 'v1',
  };
}

/** Build a complete, valid AgentURI for testing. */
function makeValidAgentURI(): AgentURI {
  return {
    services: [
      makeTopicService('stdIn'),
      makeTopicService('stdOut'),
      makeTopicService('stdErr'),
      makeP2PService(),
    ],
  };
}

/** Helper: find errors for a specific rule. */
function errorsForRule(errors: ReadonlyArray<ValidationError>, rule: string): ValidationError[] {
  return errors.filter(e => e.rule === rule);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('validateRegistrationCompleteness', () => {
  describe('valid AgentURI', () => {
    it('should pass validation for a complete AgentURI without DID', () => {
      const result = validateRegistrationCompleteness(makeValidAgentURI(), childPubKey);

      expect(result.valid).toBe(true);
      expect(result.errors).toHaveLength(0);
    });

    it('should pass validation for a complete AgentURI with valid DID', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeP2PService(),
          makeDIDService(),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(true);
      expect(result.errors).toHaveLength(0);
    });
  });

  describe('V-REG-01: Exactly 3 neuron-topic services', () => {
    it('should fail when fewer than 3 neuron-topic services', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          // Missing stdErr
          makeP2PService(),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg01 = errorsForRule(result.errors, 'V-REG-01');
      expect(vreg01.length).toBeGreaterThan(0);
    });

    it('should fail when more than 3 neuron-topic services', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeTopicService('stdIn'), // duplicate
          makeP2PService(),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg01 = errorsForRule(result.errors, 'V-REG-01');
      expect(vreg01.length).toBeGreaterThan(0);
    });

    it('should fail when 3 services but missing a standard channel', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('custom:logs'), // not stdErr
          makeP2PService(),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg01 = errorsForRule(result.errors, 'V-REG-01');
      expect(vreg01.length).toBeGreaterThan(0);
      expect(vreg01.some(e => e.message.includes('stdErr'))).toBe(true);
    });

    it('should fail with zero neuron-topic services', () => {
      const uri: AgentURI = {
        services: [makeP2PService()],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg01 = errorsForRule(result.errors, 'V-REG-01');
      expect(vreg01.length).toBeGreaterThan(0);
    });
  });

  describe('V-REG-02: At least 1 neuron-p2p-exchange service', () => {
    it('should fail when no neuron-p2p-exchange service', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          // No P2P service
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg02 = errorsForRule(result.errors, 'V-REG-02');
      expect(vreg02).toHaveLength(1);
      expect(vreg02[0]!.message).toContain('at least 1');
    });

    it('should pass with multiple neuron-p2p-exchange services', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeP2PService(),
          makeP2PService(), // second P2P service is OK
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(true);
    });
  });

  describe('V-REG-03: neuron-topic required fields', () => {
    it('should fail when neuron-topic is missing a required field', () => {
      const badTopic = {
        type: 'neuron-topic' as const,
        name: 'stdIn',
        version: '1.0.0',
        channel: 'stdIn',
        transport: 'hcs',
        anchor: '', // empty anchor
        config: {},
      };

      const uri: AgentURI = {
        services: [
          badTopic,
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeP2PService(),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg03 = errorsForRule(result.errors, 'V-REG-03');
      expect(vreg03.length).toBeGreaterThan(0);
      expect(vreg03.some(e => e.message.includes('anchor'))).toBe(true);
    });
  });

  describe('V-REG-04: neuron-p2p-exchange required fields', () => {
    it('should fail when neuron-p2p-exchange is missing a required field', () => {
      const badP2P = {
        type: 'neuron-p2p-exchange' as const,
        name: 'p2p',
        version: '1.0.0',
        peerID: childPeerID,
        protocol: '', // empty protocol
        topicRef: 'stdIn',
      };

      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          badP2P,
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg04 = errorsForRule(result.errors, 'V-REG-04');
      expect(vreg04.length).toBeGreaterThan(0);
      expect(vreg04.some(e => e.message.includes('protocol'))).toBe(true);
    });
  });

  describe('V-REG-05: topicRef matches neuron-topic service name', () => {
    it('should fail when topicRef does not match any topic name', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeP2PService(undefined, 'nonExistentChannel'),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg05 = errorsForRule(result.errors, 'V-REG-05');
      expect(vreg05).toHaveLength(1);
      expect(vreg05[0]!.message).toContain('nonExistentChannel');
    });

    it('should pass when topicRef matches a valid topic name', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeP2PService(undefined, 'stdOut'),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      // Only V-REG-05 should pass (other rules may still be valid)
      const vreg05 = errorsForRule(result.errors, 'V-REG-05');
      expect(vreg05).toHaveLength(0);
    });
  });

  describe('V-REG-06: DID service endpoint matches Child did:key', () => {
    it('should fail when DID endpoint does not match Child did:key', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeP2PService(),
          makeDIDService('did:key:zQ3sWRONG'),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg06 = errorsForRule(result.errors, 'V-REG-06');
      expect(vreg06).toHaveLength(1);
      expect(vreg06[0]!.message).toContain('does not match');
    });

    it('should pass when DID endpoint matches Child did:key', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeP2PService(),
          makeDIDService(childDIDKey),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      const vreg06 = errorsForRule(result.errors, 'V-REG-06');
      expect(vreg06).toHaveLength(0);
    });

    it('should pass when no DID service is present (optional)', () => {
      const result = validateRegistrationCompleteness(makeValidAgentURI(), childPubKey);

      const vreg06 = errorsForRule(result.errors, 'V-REG-06');
      expect(vreg06).toHaveLength(0);
    });
  });

  describe('V-REG-07: At most one DID service', () => {
    it('should fail when multiple DID services are present', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeP2PService(),
          makeDIDService(),
          makeDIDService(), // second DID
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg07 = errorsForRule(result.errors, 'V-REG-07');
      expect(vreg07).toHaveLength(1);
      expect(vreg07[0]!.message).toContain('at most 1');
    });
  });

  describe('V-REG-08: Proof-of-control (on-chain, deferred)', () => {
    it('should not be checked locally — deferred to registration time', () => {
      // V-REG-08 is an on-chain check: transaction signer matches childAddress.
      // The validation function does not check this rule.
      const result = validateRegistrationCompleteness(makeValidAgentURI(), childPubKey);

      const vreg08 = errorsForRule(result.errors, 'V-REG-08');
      expect(vreg08).toHaveLength(0);
    });
  });

  describe('V-REG-09: Duplicate registration (on-chain, deferred)', () => {
    it('should not be checked locally — deferred to registration time', () => {
      // V-REG-09 is an on-chain check: Child not already registered.
      const result = validateRegistrationCompleteness(makeValidAgentURI(), childPubKey);

      const vreg09 = errorsForRule(result.errors, 'V-REG-09');
      expect(vreg09).toHaveLength(0);
    });
  });

  describe('V-REG-10: NFT ownership (on-chain, deferred)', () => {
    it('should not be checked locally — deferred to post-registration', () => {
      // V-REG-10 is an on-chain check: NFT owner is Child's EVMAddress.
      const result = validateRegistrationCompleteness(makeValidAgentURI(), childPubKey);

      const vreg10 = errorsForRule(result.errors, 'V-REG-10');
      expect(vreg10).toHaveLength(0);
    });
  });

  describe('V-REG-11: Standard channels appear exactly once', () => {
    it('should fail when a standard channel appears more than once', () => {
      // This is a sub-check of V-REG-01, but V-REG-11 specifically flags duplicates.
      // To trigger V-REG-11 without V-REG-01, we would need exactly 3 topics
      // but with a duplicate. However, 3 topics with a duplicate means one is missing,
      // so V-REG-01 also fires. We check both are reported.
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdIn'), // duplicate stdIn
          makeTopicService('stdErr'),
          makeP2PService(),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg11 = errorsForRule(result.errors, 'V-REG-11');
      expect(vreg11.length).toBeGreaterThan(0);
      expect(vreg11.some(e => e.message.includes('stdIn'))).toBe(true);
    });
  });

  describe('V-REG-12: P2P peerID matches Child PeerID', () => {
    it('should fail when peerID does not match Child PeerID', () => {
      const uri: AgentURI = {
        services: [
          makeTopicService('stdIn'),
          makeTopicService('stdOut'),
          makeTopicService('stdErr'),
          makeP2PService('12D3KooWWRONGPeerID', 'stdIn'),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const vreg12 = errorsForRule(result.errors, 'V-REG-12');
      expect(vreg12).toHaveLength(1);
      expect(vreg12[0]!.message).toContain('does not match');
    });

    it('should pass when peerID matches Child PeerID', () => {
      const result = validateRegistrationCompleteness(makeValidAgentURI(), childPubKey);

      const vreg12 = errorsForRule(result.errors, 'V-REG-12');
      expect(vreg12).toHaveLength(0);
    });
  });

  describe('multiple validation errors', () => {
    it('should collect all errors from multiple failing rules', () => {
      const uri: AgentURI = {
        services: [
          // Only 1 topic (V-REG-01 fails)
          makeTopicService('stdIn'),
          // No P2P (V-REG-02 fails)
          // Multiple DIDs (V-REG-07 fails)
          makeDIDService('did:key:zQ3sWRONG'), // V-REG-06 fails
          makeDIDService(childDIDKey),
        ],
      };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      // Should have errors from multiple rules
      const rules = new Set(result.errors.map(e => e.rule));
      expect(rules.has('V-REG-01')).toBe(true); // wrong count
      expect(rules.has('V-REG-02')).toBe(true); // no p2p
      expect(rules.has('V-REG-06')).toBe(true); // wrong DID
      expect(rules.has('V-REG-07')).toBe(true); // multiple DIDs
    });
  });

  describe('empty AgentURI', () => {
    it('should fail with errors for V-REG-01 and V-REG-02', () => {
      const uri: AgentURI = { services: [] };

      const result = validateRegistrationCompleteness(uri, childPubKey);

      expect(result.valid).toBe(false);
      const rules = new Set(result.errors.map(e => e.rule));
      expect(rules.has('V-REG-01')).toBe(true);
      expect(rules.has('V-REG-02')).toBe(true);
    });
  });
});
