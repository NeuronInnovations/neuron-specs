/**
 * AgentURI construction, parsing, and serialization.
 *
 * Spec reference: 003 spec.md
 *   - FR-R02: neuron-topic services as EIP-8004 service objects.
 *   - FR-R03: neuron-p2p-exchange service.
 *   - FR-R08: AgentURI completeness.
 *   - FR-R13, FR-R14: Optional DID service.
 *
 * Spec reference: 003 data-model.md
 *   - AgentURI entity: serialization is JSON with services array.
 *   - Service ordering: neuron-topic first (stdIn, stdOut, stdErr),
 *     then neuron-p2p-exchange, then optional services (DID, etc.).
 *
 * SC-R01: AgentURI round-trip (serialize then parse produces identical result).
 */

import type {
  AgentURI,
  AgentURIService,
  NeuronTopicServiceEntry,
  NeuronP2PExchangeEntry,
  DIDServiceEntry,
} from './types.js';
import { invalidAgentURI } from './errors.js';

/**
 * Build an AgentURI from its component services.
 *
 * FR-R08: The resulting AgentURI contains the provided services in
 * canonical order (neuron-topic first, then neuron-p2p-exchange, then DID).
 *
 * This function does NOT validate completeness. Call
 * {@link validateRegistrationCompleteness} separately before registration.
 *
 * @param topics - neuron-topic service entries (typically 3: stdIn, stdOut, stdErr)
 * @param p2p - neuron-p2p-exchange service entries (typically 1)
 * @param did - Optional DID service entry
 * @returns AgentURI with services in canonical order
 */
export function buildAgentURI(
  topics: readonly NeuronTopicServiceEntry[],
  p2p: readonly NeuronP2PExchangeEntry[],
  did?: DIDServiceEntry,
): AgentURI {
  const services: AgentURIService[] = [];

  // Canonical order per 003 data-model.md:
  // 1. neuron-topic entries (stdIn, stdOut, stdErr order)
  const channelOrder = ['stdIn', 'stdOut', 'stdErr'];
  const sorted = [...topics].sort((a, b) => {
    const aIdx = channelOrder.indexOf(a.channel);
    const bIdx = channelOrder.indexOf(b.channel);
    // Standard channels first in order, custom channels after
    const aOrder = aIdx >= 0 ? aIdx : channelOrder.length;
    const bOrder = bIdx >= 0 ? bIdx : channelOrder.length;
    return aOrder - bOrder;
  });
  services.push(...sorted);

  // 2. neuron-p2p-exchange entries
  services.push(...p2p);

  // 3. Optional DID service
  if (did !== undefined) {
    services.push(did);
  }

  return { services };
}

/**
 * Parse an AgentURI from a JSON string.
 *
 * FR-R08: Extracts and classifies all recognized service types.
 * Unrecognized service types are preserved in the services array
 * for forward compatibility.
 *
 * @param json - Serialized AgentURI JSON string
 * @returns Parsed AgentURI
 * @throws RegistryError NEURON-REG-005 if JSON is malformed or missing services
 */
export function parseAgentURI(json: string): AgentURI {
  let doc: unknown;
  try {
    doc = JSON.parse(json) as unknown;
  } catch (e) {
    const cause = e instanceof Error ? e : new Error(String(e));
    throw invalidAgentURI('Failed to parse AgentURI JSON', cause);
  }

  if (typeof doc !== 'object' || doc === null) {
    throw invalidAgentURI('AgentURI document must be a JSON object');
  }

  const docObj = doc as Record<string, unknown>;
  const rawServices = docObj['services'];

  if (!Array.isArray(rawServices)) {
    throw invalidAgentURI('AgentURI document must have a "services" array');
  }

  const services: AgentURIService[] = [];

  for (const raw of rawServices) {
    if (typeof raw !== 'object' || raw === null) {
      throw invalidAgentURI('Each service in the AgentURI must be a JSON object');
    }

    const svcObj = raw as Record<string, unknown>;
    const svcType = svcObj['type'];

    if (svcType === 'neuron-topic') {
      services.push(parseTopicService(svcObj));
    } else if (svcType === 'neuron-p2p-exchange') {
      services.push(parseP2PService(svcObj));
    } else if (svcType === 'DID' || svcObj['name'] === 'DID') {
      services.push(parseDIDService(svcObj));
    }
    // Other types are ignored (forward-compatible)
  }

  return { services };
}

/**
 * Serialize an AgentURI to a JSON string.
 *
 * SC-R01: Round-trip guarantee (serialize then parse produces identical result).
 *
 * @param uri - The AgentURI to serialize
 * @returns JSON string representation
 */
export function serializeAgentURI(uri: AgentURI): string {
  return JSON.stringify({ services: uri.services });
}

// ---------------------------------------------------------------------------
// Internal parsers
// ---------------------------------------------------------------------------

/**
 * Parse a neuron-topic service from a raw JSON object.
 *
 * @param obj - Raw service object
 * @returns NeuronTopicServiceEntry
 * @throws RegistryError NEURON-REG-005 if required fields are missing
 */
function parseTopicService(obj: Record<string, unknown>): NeuronTopicServiceEntry {
  const name = requireString(obj, 'name', 'neuron-topic');
  const version = requireString(obj, 'version', 'neuron-topic');
  const channel = requireString(obj, 'channel', 'neuron-topic');
  const transport = requireString(obj, 'transport', 'neuron-topic');
  const anchor = requireString(obj, 'anchor', 'neuron-topic');

  const config = obj['config'];
  if (config === undefined || config === null || typeof config !== 'object') {
    throw invalidAgentURI('neuron-topic service requires a "config" object');
  }

  const endpoint = typeof obj['endpoint'] === 'string' ? obj['endpoint'] : undefined;

  return {
    type: 'neuron-topic',
    name,
    version,
    channel,
    transport,
    anchor,
    config: config as Record<string, unknown>,
    endpoint,
  };
}

/**
 * Parse a neuron-p2p-exchange service from a raw JSON object.
 *
 * @param obj - Raw service object
 * @returns NeuronP2PExchangeEntry
 * @throws RegistryError NEURON-REG-005 if required fields are missing
 */
function parseP2PService(obj: Record<string, unknown>): NeuronP2PExchangeEntry {
  const name = requireString(obj, 'name', 'neuron-p2p-exchange');
  const version = requireString(obj, 'version', 'neuron-p2p-exchange');
  const peerID = requireString(obj, 'peerID', 'neuron-p2p-exchange');
  const protocol = requireString(obj, 'protocol', 'neuron-p2p-exchange');
  const topicRef = requireString(obj, 'topicRef', 'neuron-p2p-exchange');

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
 * Parse a DID service from a raw JSON object.
 *
 * FR-R13, FR-R14: DID service with name "DID" and did:key endpoint.
 *
 * @param obj - Raw service object
 * @returns DIDServiceEntry
 * @throws RegistryError NEURON-REG-005 if required fields are missing
 */
function parseDIDService(obj: Record<string, unknown>): DIDServiceEntry {
  const name = typeof obj['name'] === 'string' ? obj['name'] : 'DID';
  const endpoint = requireString(obj, 'endpoint', 'DID service');
  const version = requireString(obj, 'version', 'DID service');

  return {
    type: 'DID',
    name,
    endpoint,
    version,
  };
}

/**
 * Require a non-empty string field from a parsed JSON object.
 *
 * @param obj - The object to read from
 * @param field - The field name
 * @param context - Error context for diagnostics
 * @returns The non-empty string value
 * @throws RegistryError NEURON-REG-005 if field is missing, not a string, or empty
 */
function requireString(
  obj: Record<string, unknown>,
  field: string,
  context: string,
): string {
  const value = obj[field];
  if (typeof value !== 'string' || value.length === 0) {
    throw invalidAgentURI(
      `${context} service requires non-empty string field "${field}"`,
    );
  }
  return value;
}
