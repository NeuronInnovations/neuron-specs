/**
 * Core type definitions for the Peer Registry module.
 *
 * Spec reference: 003 spec.md
 *   - FR-R01: Registration in EIP-8004 registry linked to ERC-721 NFT.
 *   - FR-R02: Three mandatory neuron-topic services (stdIn, stdOut, stdErr).
 *   - FR-R03: Mandatory neuron-p2p-exchange service with peerID, protocol, topicRef.
 *   - FR-R04: Resolution by (registry + Child EVM address) or (registry + external id).
 *   - FR-R08: AgentURI completeness requires three neuron-topic + one neuron-p2p-exchange.
 *   - FR-R13, FR-R14: Optional DID service (did:key from Child's NeuronPublicKey).
 *
 * Spec reference: 003 data-model.md
 *   - Registration, AgentURI, NeuronTopicService, NeuronP2PExchangeService,
 *     DIDService, RegistrationResult entities.
 *
 * All types are immutable (readonly). Valid by construction where possible.
 */

// ---------------------------------------------------------------------------
// AgentURI Service Types
// ---------------------------------------------------------------------------

/**
 * A neuron-topic service entry in the AgentURI services array.
 *
 * FR-R02: One per standard channel (stdIn, stdOut, stdErr).
 * Schema authority: 004 FR-T14.
 */
export interface NeuronTopicServiceEntry {
  /** Discriminator. Always 'neuron-topic'. FR-R02, 004 FR-T14 */
  readonly type: 'neuron-topic';

  /** Channel role: stdIn, stdOut, stdErr, or custom:<name>. FR-R02, 004 FR-T14 */
  readonly name: string;

  /** Topic protocol version (semver). 004 FR-T14 */
  readonly version: string;

  /** Neuron channel role. FR-R02, 004 FR-T14 */
  readonly channel: string;

  /** Backend kind (hcs, erc-log, kafka, custom:*). FR-R02, 004 FR-T13, 004 FR-T14 */
  readonly transport: string;

  /** Anchoring ledger identifier. 004 FR-T14 */
  readonly anchor: string;

  /** Transport-specific configuration. 004 FR-T14, 004 FR-T15 */
  readonly config: Record<string, unknown>;

  /** Compact Topic URI for backward-compatible EIP-8004 consumers. 004 FR-T14 (SHOULD) */
  readonly endpoint?: string | undefined;
}

/**
 * A neuron-p2p-exchange service entry in the AgentURI services array.
 *
 * FR-R03: Mandatory. Describes how to discover the peer's multiaddress
 * via a topic-based signaling protocol.
 * Schema authority: 004 FR-T17.
 */
export interface NeuronP2PExchangeEntry {
  /** Discriminator. Always 'neuron-p2p-exchange'. FR-R03, 004 FR-T17 */
  readonly type: 'neuron-p2p-exchange';

  /** Service name (e.g., 'p2p'). 004 FR-T17 */
  readonly name: string;

  /** Exchange protocol version (semver). 004 FR-T17 */
  readonly version: string;

  /** Libp2p PeerID from Key Library 002. FR-R03, 004 FR-T17 */
  readonly peerID: string;

  /** Protocol ID for multiaddress exchange. FR-R03, 004 FR-T17 */
  readonly protocol: string;

  /**
   * Cross-reference to a neuron-topic service name in the same AgentURI.
   * FR-R03, 004 FR-T17, 004 FR-T18: MUST resolve to an existing neuron-topic service.
   */
  readonly topicRef: string;
}

/**
 * A DID service entry in the AgentURI services array.
 *
 * FR-R13, FR-R14: Optional. When present, the endpoint MUST be a did:key
 * derived from the registered Child's NeuronPublicKey (secp256k1).
 */
export interface DIDServiceEntry {
  /** Discriminator. Always 'DID'. FR-R13 */
  readonly type: 'DID';

  /** Service name. MUST be 'DID'. FR-R13 */
  readonly name: string;

  /** did:key:zQ3s... derived from Child's NeuronPublicKey. FR-R14 */
  readonly endpoint: string;

  /** Version string (e.g., 'v1'). FR-R13 */
  readonly version: string;
}

/**
 * Discriminated union of all recognized AgentURI service types.
 *
 * FR-R02, FR-R03, FR-R13: The services array contains neuron-topic,
 * neuron-p2p-exchange, and optionally DID entries.
 */
export type AgentURIService = NeuronTopicServiceEntry | NeuronP2PExchangeEntry | DIDServiceEntry;

// ---------------------------------------------------------------------------
// AgentURI
// ---------------------------------------------------------------------------

/**
 * The JSON registration file referenced by the EIP-8004 agentURI.
 *
 * FR-R08: A complete AgentURI contains exactly three neuron-topic services,
 * at least one neuron-p2p-exchange service, and optionally one DID service.
 */
export interface AgentURI {
  /** Array of EIP-8004 service objects. FR-R02, FR-R03 */
  readonly services: ReadonlyArray<AgentURIService>;
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

/**
 * Represents a Child's registration in an EIP-8004 registry.
 *
 * FR-R01: Linked to an ERC-721 NFT in the registry smart contract.
 * FR-R05: Unique per (childAddress, registryAddress).
 * FR-R10: childAddress is the NFT owner.
 */
export interface Registration {
  /** Address of the EIP-8004 registry smart contract. FR-R01 */
  readonly registryAddress: string;

  /** The Child's EVM address (NFT owner). FR-R06, FR-R10 */
  readonly childAddress: string;

  /** ERC-721 token ID (auto-incrementing agentId from register()). FR-R01 */
  readonly tokenId: bigint;

  /** The registration's agent URI (services array). FR-R02, FR-R03, FR-R08 */
  readonly agentURI: AgentURI;

  /** Blockchain chain ID where the registry is deployed. FR-R01 */
  readonly chainId: bigint;
}

// ---------------------------------------------------------------------------
// RegistrationResult
// ---------------------------------------------------------------------------

/**
 * The result of a registration operation (create, update).
 *
 * FR-R01: Contains the on-chain identifiers from the registry transaction.
 * FR-R06: transactionHash proves the on-chain execution.
 */
export interface RegistrationResult {
  /** The ERC-721 token ID of the registered NFT. FR-R01 */
  readonly tokenId: bigint;

  /** The on-chain transaction hash. FR-R06 */
  readonly transactionHash: string;

  /** The Child's EVM address (NFT owner). FR-R10 */
  readonly childAddress: string;

  /** The registry contract address. FR-R01 */
  readonly registryAddress: string;

  /** The chain ID. FR-R01 */
  readonly chainId: bigint;

  /** The agentURI string stored on-chain (JSON). FR-R08 */
  readonly agentURI: string;
}

// ---------------------------------------------------------------------------
// LookupKey
// ---------------------------------------------------------------------------

/**
 * Discriminated union for registry lookup methods.
 *
 * FR-R04: Lookup by (registry + Child EVM address) or by
 * (registry + external id) when the account has a registry binding.
 */
export type LookupKey =
  | { readonly type: 'byAddress'; readonly address: string }
  | { readonly type: 'byExternalId'; readonly externalId: string };
