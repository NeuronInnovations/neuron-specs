/**
 * Service schema types -- NeuronTopicService and NeuronP2PExchangeService.
 *
 * Spec reference: 004 spec.md
 *   - FR-T14: NeuronTopicService EIP-8004 service object.
 *   - FR-T15: Transport-specific configuration schemas.
 *   - FR-T17: NeuronP2PExchangeService for multiaddress discovery.
 *   - FR-T18: Cross-reference validation (topicRef resolves to existing service).
 *
 * Spec reference: 004 contracts/service-schema.md
 *   - Full JSON schemas for NeuronTopicService, NeuronP2PExchangeService.
 *   - Transport config schemas: HCSConfig, ERCLogConfig, KafkaConfig.
 *   - Parsing, validation, and serialization functions.
 *
 * SC-T09: Round-trip guarantee (serialize then parse produces identical result).
 * SC-T10: Cross-reference validation catches broken topicRef references.
 */

import type { BackendKind, ChannelRole } from './types.js';
import { isValidBackendKind, isValidChannelRole } from './types.js';
import { TopicRef } from './topic-ref.js';
import { invalidTopicRef } from './errors.js';

// ---------------------------------------------------------------------------
// Transport Config types
// ---------------------------------------------------------------------------

/**
 * HCS transport configuration.
 * FR-T15: Hedera Consensus Service backend.
 */
export interface HCSConfig {
  /** Hedera network identifier (e.g., 'hedera-mainnet', 'hedera-testnet'). FR-T15 */
  readonly network: string;

  /** HCS topic ID in shard.realm.num format (e.g., '0.0.4515382'). FR-T15 */
  readonly topicId: string;
}

/**
 * ERC event log transport configuration.
 * FR-T15: Read-only ERC event logs on Ethereum/EVM chains.
 */
export interface ERCLogConfig {
  /** EVM chain ID (e.g., 1 for Ethereum mainnet). FR-T15 */
  readonly chainId: number;

  /** EVM contract address (EIP-55 checksummed). FR-T15 */
  readonly contractAddress: string;

  /** Solidity event signature. FR-T15 */
  readonly eventSignature: string;
}

/**
 * Kafka anchoring configuration.
 * FR-T16: Non-ledger-native transports MUST have valid anchoring config.
 */
export interface AnchoringConfig {
  /** Anchoring method (e.g., 'hcs-hash-chain'). FR-T16 */
  readonly method: string;

  /** Topic ID on the anchor ledger. FR-T16 */
  readonly anchorTopicId: string;

  /** Network identifier of the anchor ledger. FR-T16 */
  readonly anchorNetwork: string;

  /** Anchoring frequency (e.g., 'every-batch', 'every-100-messages'). FR-T16 */
  readonly interval: string;
}

/**
 * Kafka transport configuration.
 * FR-T15, FR-T16: Kafka with ledger anchoring.
 */
export interface KafkaConfig {
  /** Kafka broker addresses. FR-T15 */
  readonly bootstrapServers: readonly string[];

  /** Kafka topic name. FR-T15 */
  readonly topicName: string;

  /** SASL authentication mechanism (optional). FR-T15 */
  readonly saslMechanism?: string | undefined;

  /** Ledger anchoring configuration. FR-T16 */
  readonly anchoring: AnchoringConfig;
}

/** Union of all transport config types. FR-T15 */
export type TransportConfig = HCSConfig | ERCLogConfig | KafkaConfig | Record<string, unknown>;

// ---------------------------------------------------------------------------
// NeuronTopicService
// ---------------------------------------------------------------------------

/**
 * EIP-8004 service object with type "neuron-topic".
 *
 * FR-T14: Represents a single topic channel in the agentURI services array.
 * All MUST fields are required by the JSON schema.
 */
export interface NeuronTopicService {
  /** Discriminator. Always 'neuron-topic'. FR-T14 */
  readonly type: 'neuron-topic';

  /** Channel role: stdIn, stdOut, stdErr, or custom:<name>. FR-T14, FR-T07, FR-T08 */
  readonly name: string;

  /** Compact Topic URI for backward-compatible EIP-8004 consumers. FR-T14 (SHOULD) */
  readonly endpoint?: string | undefined;

  /** Topic protocol version (semver). FR-T14 */
  readonly version: string;

  /** Neuron channel role (same as name for standard channels). FR-T14 */
  readonly channel: string;

  /** Backend kind. FR-T14, FR-T15 */
  readonly transport: string;

  /** Ledger that anchors the topic. FR-T14 */
  readonly anchor: string;

  /** Transport-specific configuration. FR-T14, FR-T15 */
  readonly config: TransportConfig;
}

// ---------------------------------------------------------------------------
// NeuronP2PExchangeService
// ---------------------------------------------------------------------------

/**
 * EIP-8004 service object with type "neuron-p2p-exchange".
 *
 * FR-T17: Defines the method for multiaddress discovery.
 * All MUST fields are required.
 */
export interface NeuronP2PExchangeService {
  /** Discriminator. Always 'neuron-p2p-exchange'. FR-T17 */
  readonly type: 'neuron-p2p-exchange';

  /** Service name (e.g., 'p2p'). FR-T17 */
  readonly name: string;

  /** Exchange protocol version (semver). FR-T17 */
  readonly version: string;

  /** Libp2p PeerID from Key Library 002. FR-T17 */
  readonly peerID: string;

  /** Protocol ID for multiaddress exchange. FR-T17 */
  readonly protocol: string;

  /**
   * Cross-reference to a neuron-topic service name in the same agentURI.
   * FR-T17, FR-T18: Must resolve to an existing neuron-topic service.
   */
  readonly topicRef: string;
}

// ---------------------------------------------------------------------------
// Parsing functions
// ---------------------------------------------------------------------------

/**
 * Parse a NeuronTopicService from a JSON object.
 *
 * FR-T14: Validates all required fields.
 * FR-T15: Validates transport-specific config structure.
 *
 * @param obj - Parsed JSON object with type 'neuron-topic'
 * @returns Validated NeuronTopicService
 * @throws TopicError NEURON-TOPIC-001 if required fields are missing or invalid
 */
export function parseNeuronTopicService(obj: Record<string, unknown>): NeuronTopicService {
  if (obj['type'] !== 'neuron-topic') {
    throw invalidTopicRef(
      `Expected service type "neuron-topic", got "${String(obj['type'])}"`,
    );
  }

  const name = requireString(obj, 'name', 'NeuronTopicService');
  const version = requireString(obj, 'version', 'NeuronTopicService');
  const channel = requireString(obj, 'channel', 'NeuronTopicService');
  const transport = requireString(obj, 'transport', 'NeuronTopicService');
  const anchor = requireString(obj, 'anchor', 'NeuronTopicService');

  if (!isValidChannelRole(channel)) {
    throw invalidTopicRef(
      `Invalid channel role in NeuronTopicService: "${channel}"`,
    );
  }

  if (!isValidBackendKind(transport)) {
    throw invalidTopicRef(
      `Invalid transport kind in NeuronTopicService: "${transport}"`,
    );
  }

  const config = obj['config'];
  if (config === undefined || config === null || typeof config !== 'object') {
    throw invalidTopicRef('NeuronTopicService requires a "config" object');
  }

  const endpoint = typeof obj['endpoint'] === 'string' ? obj['endpoint'] : undefined;

  return {
    type: 'neuron-topic',
    name,
    endpoint,
    version,
    channel,
    transport,
    anchor,
    config: config as TransportConfig,
  };
}

/**
 * Parse a NeuronP2PExchangeService from a JSON object.
 *
 * FR-T17: Validates all required fields.
 *
 * @param obj - Parsed JSON object with type 'neuron-p2p-exchange'
 * @returns Validated NeuronP2PExchangeService
 * @throws TopicError NEURON-TOPIC-001 if required fields are missing or invalid
 */
export function parseNeuronP2PExchangeService(
  obj: Record<string, unknown>,
): NeuronP2PExchangeService {
  if (obj['type'] !== 'neuron-p2p-exchange') {
    throw invalidTopicRef(
      `Expected service type "neuron-p2p-exchange", got "${String(obj['type'])}"`,
    );
  }

  const name = requireString(obj, 'name', 'NeuronP2PExchangeService');
  const version = requireString(obj, 'version', 'NeuronP2PExchangeService');
  const peerID = requireString(obj, 'peerID', 'NeuronP2PExchangeService');
  const protocol = requireString(obj, 'protocol', 'NeuronP2PExchangeService');
  const topicRef = requireString(obj, 'topicRef', 'NeuronP2PExchangeService');

  if (!protocol.startsWith('/')) {
    throw invalidTopicRef(
      `NeuronP2PExchangeService protocol must start with "/", got "${protocol}"`,
    );
  }

  if (!isValidChannelRole(topicRef)) {
    throw invalidTopicRef(
      `Invalid topicRef in NeuronP2PExchangeService: "${topicRef}". Must be a valid channel role`,
    );
  }

  return {
    type: 'neuron-p2p-exchange',
    name,
    version,
    peerID,
    protocol,
    topicRef,
  };
}

/**
 * Parse all Neuron services from an agentURI JSON document.
 *
 * FR-T09: Extracts neuron-topic and neuron-p2p-exchange services from the
 * services array. Other service types are ignored (forward-compatible).
 *
 * @param json - Raw JSON string of the agentURI document
 * @returns Tuple of [topic services, p2p exchange services]
 * @throws TopicError NEURON-TOPIC-001 if parsing fails
 */
export function parseAgentURIServices(
  json: string,
): [NeuronTopicService[], NeuronP2PExchangeService[]] {
  let doc: unknown;
  try {
    doc = JSON.parse(json) as unknown;
  } catch (e) {
    const cause = e instanceof Error ? e : new Error(String(e));
    throw invalidTopicRef('Failed to parse agentURI JSON', cause);
  }

  if (typeof doc !== 'object' || doc === null) {
    throw invalidTopicRef('agentURI document must be a JSON object');
  }

  const docObj = doc as Record<string, unknown>;
  const services = docObj['services'];

  if (!Array.isArray(services)) {
    throw invalidTopicRef('agentURI document must have a "services" array');
  }

  const topics: NeuronTopicService[] = [];
  const p2pExchanges: NeuronP2PExchangeService[] = [];

  for (const svc of services) {
    if (typeof svc !== 'object' || svc === null) {
      continue;
    }

    const svcObj = svc as Record<string, unknown>;
    const svcType = svcObj['type'];

    if (svcType === 'neuron-topic') {
      topics.push(parseNeuronTopicService(svcObj));
    } else if (svcType === 'neuron-p2p-exchange') {
      p2pExchanges.push(parseNeuronP2PExchangeService(svcObj));
    }
    // Other types are ignored (forward-compatible)
  }

  return [topics, p2pExchanges];
}

/**
 * Validate cross-references between P2P exchange services and topic services.
 *
 * FR-T18, SC-T10: Every topicRef in a NeuronP2PExchangeService MUST resolve
 * to an existing NeuronTopicService name in the same agentURI document.
 *
 * @param topics - Parsed topic services from the agentURI
 * @param p2p - Parsed P2P exchange services
 * @throws TopicError NEURON-TOPIC-001 if any cross-reference is broken
 */
export function validateCrossReferences(
  topics: readonly NeuronTopicService[],
  p2p: readonly NeuronP2PExchangeService[],
): void {
  const topicNames = new Set(topics.map(t => t.name));

  for (const svc of p2p) {
    if (!topicNames.has(svc.topicRef)) {
      throw invalidTopicRef(
        `Broken topicRef in NeuronP2PExchangeService "${svc.name}": ` +
        `topicRef "${svc.topicRef}" does not match any neuron-topic service name. ` +
        `Available names: [${Array.from(topicNames).join(', ')}]`,
      );
    }
  }
}

/**
 * Extract a TopicRef from a NeuronTopicService.
 *
 * FR-T09: The locator is extracted from the config based on transport:
 * - hcs: config.topicId
 * - erc-log: config.chainId:config.contractAddress
 * - kafka: config.topicName
 * - custom: adapter-defined extraction (falls back to endpoint)
 *
 * @param svc - A parsed neuron-topic service
 * @returns TopicRef instance
 * @throws TopicError NEURON-TOPIC-001 if extraction fails
 */
export function extractTopicRef(svc: NeuronTopicService): TopicRef {
  const config = svc.config as Record<string, unknown>;

  switch (svc.transport) {
    case 'hcs': {
      const topicId = config['topicId'];
      if (typeof topicId !== 'string' || topicId.length === 0) {
        throw invalidTopicRef('HCS config missing required field "topicId"');
      }
      return TopicRef.create('hcs', topicId);
    }
    case 'erc-log': {
      const chainId = config['chainId'];
      const contractAddress = config['contractAddress'];
      if (typeof chainId !== 'number' || typeof contractAddress !== 'string') {
        throw invalidTopicRef(
          'ERC-log config missing required fields "chainId" and/or "contractAddress"',
        );
      }
      return TopicRef.create('erc-log', `${chainId.toString()}:${contractAddress}`);
    }
    case 'kafka': {
      const topicName = config['topicName'];
      if (typeof topicName !== 'string' || topicName.length === 0) {
        throw invalidTopicRef('Kafka config missing required field "topicName"');
      }
      return TopicRef.create('kafka', topicName);
    }
    default: {
      // Custom transport: try config-based or fall back to endpoint
      if (svc.endpoint !== undefined && svc.endpoint.length > 0) {
        return TopicRef.create(svc.transport as BackendKind, svc.endpoint);
      }
      throw invalidTopicRef(
        `Cannot extract TopicRef from custom transport "${svc.transport}" without endpoint`,
      );
    }
  }
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/**
 * Require a string field from a parsed JSON object.
 *
 * @param obj - The object to read from
 * @param field - The field name
 * @param context - Error context (e.g., 'NeuronTopicService')
 * @returns The string value
 * @throws TopicError NEURON-TOPIC-001 if field is missing or not a string
 */
function requireString(
  obj: Record<string, unknown>,
  field: string,
  context: string,
): string {
  const value = obj[field];
  if (typeof value !== 'string' || value.length === 0) {
    throw invalidTopicRef(
      `${context} requires non-empty string field "${field}"`,
    );
  }
  return value;
}
