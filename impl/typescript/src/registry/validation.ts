/**
 * AgentURI validation rules for registration completeness.
 *
 * Spec reference: 003 data-model.md, Validation Rules V-REG-01..V-REG-12
 * Spec reference: 003 spec.md
 *   - FR-R02: Three mandatory neuron-topic services (stdIn, stdOut, stdErr).
 *   - FR-R03: Mandatory neuron-p2p-exchange with peerID, protocol, topicRef.
 *   - FR-R08: Registration completeness.
 *   - FR-R13, FR-R14: Optional DID service constraints.
 *
 * This module implements the locally-checkable validation rules (V-REG-01..07,
 * V-REG-11, V-REG-12). On-chain checks (V-REG-08..10) are deferred to
 * registration time and are documented but not implemented here.
 */

import type { NeuronPublicKey } from '../keylib/public-key.js';
import type {
  AgentURI,
  NeuronTopicServiceEntry,
  NeuronP2PExchangeEntry,
  DIDServiceEntry,
} from './types.js';

// ---------------------------------------------------------------------------
// ValidationError
// ---------------------------------------------------------------------------

/**
 * A single validation failure with the rule identifier and message.
 *
 * Each rule maps to a V-REG-NN identifier from 003 data-model.md.
 */
export interface ValidationError {
  /** Validation rule identifier (e.g., "V-REG-01"). */
  readonly rule: string;

  /** Human-readable description of the validation failure. */
  readonly message: string;
}

/**
 * Result of AgentURI validation.
 *
 * `valid` is true only when `errors` is empty.
 */
export interface ValidationResult {
  readonly valid: boolean;
  readonly errors: ReadonlyArray<ValidationError>;
}

// ---------------------------------------------------------------------------
// Standard channel roles
// ---------------------------------------------------------------------------

/** The three mandatory standard channel roles. FR-R02, FR-R08 */
const STANDARD_CHANNELS: ReadonlyArray<string> = ['stdIn', 'stdOut', 'stdErr'];

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Validate an AgentURI for registration completeness.
 *
 * Checks all locally-verifiable validation rules (V-REG-01..07, V-REG-11, V-REG-12).
 * On-chain checks (V-REG-08 proof-of-control, V-REG-09 duplicate, V-REG-10 ownership)
 * are deferred to registration time.
 *
 * FR-R08: A complete AgentURI MUST contain exactly three neuron-topic services
 * (stdIn, stdOut, stdErr), at least one neuron-p2p-exchange, and at most one DID.
 *
 * @param agentURI - The AgentURI to validate
 * @param childPublicKey - The Child's NeuronPublicKey (for V-REG-06, V-REG-12)
 * @returns ValidationResult with `valid` flag and array of ValidationError
 */
export function validateRegistrationCompleteness(
  agentURI: AgentURI,
  childPublicKey: NeuronPublicKey,
): ValidationResult {
  const errors: ValidationError[] = [];

  // Classify services by type
  const topicServices: NeuronTopicServiceEntry[] = [];
  const p2pServices: NeuronP2PExchangeEntry[] = [];
  const didServices: DIDServiceEntry[] = [];

  for (const svc of agentURI.services) {
    switch (svc.type) {
      case 'neuron-topic':
        topicServices.push(svc);
        break;
      case 'neuron-p2p-exchange':
        p2pServices.push(svc);
        break;
      case 'DID':
        didServices.push(svc);
        break;
      default:
        // Unknown service types are ignored (forward-compatible)
        break;
    }
  }

  // V-REG-01: Exactly 3 neuron-topic services with channels stdIn, stdOut, stdErr
  checkVREG01(topicServices, errors);

  // V-REG-02: At least 1 neuron-p2p-exchange service
  checkVREG02(p2pServices, errors);

  // V-REG-03: Each neuron-topic has all required fields
  checkVREG03(topicServices, errors);

  // V-REG-04: Each neuron-p2p-exchange has all required fields
  checkVREG04(p2pServices, errors);

  // V-REG-05: Each p2p topicRef matches an existing neuron-topic service name
  checkVREG05(topicServices, p2pServices, errors);

  // V-REG-06: DID service endpoint is did:key from childPublicKey
  checkVREG06(didServices, childPublicKey, errors);

  // V-REG-07: At most one DID service
  checkVREG07(didServices, errors);

  // V-REG-11: Standard channels appear exactly once each
  checkVREG11(topicServices, errors);

  // V-REG-12: P2P peerID matches Child's PeerID
  checkVREG12(p2pServices, childPublicKey, errors);

  return {
    valid: errors.length === 0,
    errors,
  };
}

// ---------------------------------------------------------------------------
// Individual rule checks
// ---------------------------------------------------------------------------

/**
 * V-REG-01: AgentURI contains exactly 3 neuron-topic services (stdIn, stdOut, stdErr).
 *
 * FR-R08, FR-R02: Registration completeness requires three topic services.
 */
function checkVREG01(
  topics: readonly NeuronTopicServiceEntry[],
  errors: ValidationError[],
): void {
  if (topics.length !== 3) {
    errors.push({
      rule: 'V-REG-01',
      message: `AgentURI must contain exactly 3 neuron-topic services, found ${topics.length.toString()}`,
    });
    return;
  }

  const channels = new Set(topics.map(t => t.channel));
  for (const expected of STANDARD_CHANNELS) {
    if (!channels.has(expected)) {
      errors.push({
        rule: 'V-REG-01',
        message: `AgentURI is missing neuron-topic service for channel "${expected}"`,
      });
    }
  }
}

/**
 * V-REG-02: AgentURI contains at least 1 neuron-p2p-exchange service.
 *
 * FR-R08, FR-R03: Registration completeness requires at least one P2P service.
 */
function checkVREG02(
  p2p: readonly NeuronP2PExchangeEntry[],
  errors: ValidationError[],
): void {
  if (p2p.length < 1) {
    errors.push({
      rule: 'V-REG-02',
      message: 'AgentURI must contain at least 1 neuron-p2p-exchange service, found 0',
    });
  }
}

/**
 * V-REG-03: Each neuron-topic has all MUST fields.
 *
 * FR-R02, 004 FR-T14: type, name, version, channel, transport, anchor, config.
 */
function checkVREG03(
  topics: readonly NeuronTopicServiceEntry[],
  errors: ValidationError[],
): void {
  const requiredFields: ReadonlyArray<keyof NeuronTopicServiceEntry> = [
    'type', 'name', 'version', 'channel', 'transport', 'anchor', 'config',
  ];

  for (const svc of topics) {
    for (const field of requiredFields) {
      const value = svc[field];
      if (value === undefined || value === null) {
        errors.push({
          rule: 'V-REG-03',
          message: `neuron-topic service "${svc.name || '(unnamed)'}" is missing required field "${field}"`,
        });
      } else if (typeof value === 'string' && value.length === 0) {
        errors.push({
          rule: 'V-REG-03',
          message: `neuron-topic service "${svc.name || '(unnamed)'}" has empty required field "${field}"`,
        });
      }
    }
  }
}

/**
 * V-REG-04: Each neuron-p2p-exchange has all MUST fields.
 *
 * FR-R03, 004 FR-T17: type, name, version, peerID, protocol, topicRef.
 */
function checkVREG04(
  p2p: readonly NeuronP2PExchangeEntry[],
  errors: ValidationError[],
): void {
  const requiredFields: ReadonlyArray<keyof NeuronP2PExchangeEntry> = [
    'type', 'name', 'version', 'peerID', 'protocol', 'topicRef',
  ];

  for (const svc of p2p) {
    for (const field of requiredFields) {
      const value = svc[field];
      if (value === undefined || value === null) {
        errors.push({
          rule: 'V-REG-04',
          message: `neuron-p2p-exchange service "${svc.name || '(unnamed)'}" is missing required field "${field}"`,
        });
      } else if (typeof value === 'string' && value.length === 0) {
        errors.push({
          rule: 'V-REG-04',
          message: `neuron-p2p-exchange service "${svc.name || '(unnamed)'}" has empty required field "${field}"`,
        });
      }
    }
  }
}

/**
 * V-REG-05: topicRef in neuron-p2p-exchange matches a neuron-topic service name.
 *
 * 004 FR-T18: Cross-reference validation.
 */
function checkVREG05(
  topics: readonly NeuronTopicServiceEntry[],
  p2p: readonly NeuronP2PExchangeEntry[],
  errors: ValidationError[],
): void {
  const topicNames = new Set(topics.map(t => t.name));

  for (const svc of p2p) {
    if (svc.topicRef && !topicNames.has(svc.topicRef)) {
      errors.push({
        rule: 'V-REG-05',
        message:
          `neuron-p2p-exchange "${svc.name}" has topicRef "${svc.topicRef}" ` +
          `that does not match any neuron-topic service name. ` +
          `Available: [${Array.from(topicNames).join(', ')}]`,
      });
    }
  }
}

/**
 * V-REG-06: DID service endpoint is did:key from childPublicKey.
 *
 * FR-R14: When present, the DID value MUST be derived from the Child's
 * NeuronPublicKey (secp256k1, multicodec 0xe7, base58btc prefix zQ3s).
 */
function checkVREG06(
  dids: readonly DIDServiceEntry[],
  childPublicKey: NeuronPublicKey,
  errors: ValidationError[],
): void {
  if (dids.length === 0) {
    return; // DID service is optional
  }

  const expectedDID = childPublicKey.didKey().toString();

  for (const svc of dids) {
    if (svc.endpoint !== expectedDID) {
      errors.push({
        rule: 'V-REG-06',
        message:
          `DID service endpoint "${svc.endpoint}" does not match Child's did:key. ` +
          `Expected: "${expectedDID}"`,
      });
    }
  }
}

/**
 * V-REG-07: At most one DID service per registration.
 *
 * FR-R14: At most one DID service is permitted per registration.
 */
function checkVREG07(
  dids: readonly DIDServiceEntry[],
  errors: ValidationError[],
): void {
  if (dids.length > 1) {
    errors.push({
      rule: 'V-REG-07',
      message: `AgentURI must contain at most 1 DID service, found ${dids.length.toString()}`,
    });
  }
}

/**
 * V-REG-11: Standard channel names each appear exactly once in neuron-topic services.
 *
 * FR-R02, FR-R08: Duplicate standard channels are invalid.
 */
function checkVREG11(
  topics: readonly NeuronTopicServiceEntry[],
  errors: ValidationError[],
): void {
  const channelCounts = new Map<string, number>();

  for (const svc of topics) {
    if (STANDARD_CHANNELS.includes(svc.channel)) {
      channelCounts.set(svc.channel, (channelCounts.get(svc.channel) ?? 0) + 1);
    }
  }

  for (const [channel, count] of channelCounts) {
    if (count > 1) {
      errors.push({
        rule: 'V-REG-11',
        message: `Standard channel "${channel}" appears ${count.toString()} times, expected exactly once`,
      });
    }
  }
}

/**
 * V-REG-12: neuron-p2p-exchange peerID matches Child's PeerID.
 *
 * FR-R03: The peerID field MUST be the Child's libp2p PeerID derived
 * from the Child's NeuronPublicKey (per 002 FR-006).
 */
function checkVREG12(
  p2p: readonly NeuronP2PExchangeEntry[],
  childPublicKey: NeuronPublicKey,
  errors: ValidationError[],
): void {
  const expectedPeerID = childPublicKey.peerId().toString();

  for (const svc of p2p) {
    if (svc.peerID && svc.peerID !== expectedPeerID) {
      errors.push({
        rule: 'V-REG-12',
        message:
          `neuron-p2p-exchange "${svc.name}" has peerID "${svc.peerID}" ` +
          `that does not match Child's PeerID. Expected: "${expectedPeerID}"`,
      });
    }
  }
}
