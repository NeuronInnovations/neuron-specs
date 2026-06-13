/**
 * Registration resolution -- extract topics and P2P exchange from a Registration.
 *
 * Spec reference: 003 spec.md
 *   - FR-R02: Three neuron-topic services (stdIn, stdOut, stdErr).
 *   - FR-R03: At least one neuron-p2p-exchange service.
 *   - FR-R08: Registration completeness.
 *   - SC-R02: Public comms resolvable from registration as EIP-8004 services.
 *   - SC-R03: Multiaddress discoverable from registration.
 *
 * These functions assume the Registration has a valid (complete) AgentURI.
 * Callers should validate the AgentURI before resolving.
 */

import type { Registration } from './types.js';
import type { NeuronTopicServiceEntry, NeuronP2PExchangeEntry } from './types.js';
import { invalidAgentURI } from './errors.js';

/**
 * Resolved standard topic channels from a Registration's AgentURI.
 *
 * FR-R02: Every registration MUST include three neuron-topic services,
 * one for each standard channel role.
 */
export interface ResolvedTopics {
  /** The stdIn neuron-topic service. FR-R02 */
  readonly stdIn: NeuronTopicServiceEntry;

  /** The stdOut neuron-topic service. FR-R02 */
  readonly stdOut: NeuronTopicServiceEntry;

  /** The stdErr neuron-topic service. FR-R02 */
  readonly stdErr: NeuronTopicServiceEntry;
}

/**
 * Resolve the three standard topic channels from a Registration.
 *
 * FR-R02: Extracts stdIn, stdOut, and stdErr neuron-topic services.
 * SC-R02: Public comms are resolvable from the registration.
 *
 * @param registration - A complete Registration with a valid AgentURI
 * @returns ResolvedTopics with stdIn, stdOut, and stdErr entries
 * @throws RegistryError NEURON-REG-005 if any standard channel is missing
 */
export function resolveTopics(registration: Registration): ResolvedTopics {
  const topics = registration.agentURI.services.filter(
    (s): s is NeuronTopicServiceEntry => s.type === 'neuron-topic',
  );

  const stdIn = topics.find(t => t.channel === 'stdIn');
  const stdOut = topics.find(t => t.channel === 'stdOut');
  const stdErr = topics.find(t => t.channel === 'stdErr');

  if (stdIn === undefined) {
    throw invalidAgentURI('Registration AgentURI is missing neuron-topic service for channel "stdIn"');
  }
  if (stdOut === undefined) {
    throw invalidAgentURI('Registration AgentURI is missing neuron-topic service for channel "stdOut"');
  }
  if (stdErr === undefined) {
    throw invalidAgentURI('Registration AgentURI is missing neuron-topic service for channel "stdErr"');
  }

  return { stdIn, stdOut, stdErr };
}

/**
 * Resolve the first neuron-p2p-exchange service from a Registration.
 *
 * FR-R03: Every registration MUST include at least one neuron-p2p-exchange.
 * SC-R03: Multiaddress is discoverable from the registration's P2P service.
 *
 * @param registration - A complete Registration with a valid AgentURI
 * @returns The first NeuronP2PExchangeEntry
 * @throws RegistryError NEURON-REG-005 if no neuron-p2p-exchange service is found
 */
export function resolveP2PExchange(registration: Registration): NeuronP2PExchangeEntry {
  const p2p = registration.agentURI.services.find(
    (s): s is NeuronP2PExchangeEntry => s.type === 'neuron-p2p-exchange',
  );

  if (p2p === undefined) {
    throw invalidAgentURI('Registration AgentURI is missing neuron-p2p-exchange service');
  }

  return p2p;
}
