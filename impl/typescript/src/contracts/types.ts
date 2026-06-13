/**
 * Shared contract types for the Neuron Identity, Reputation, and Validation registries.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-01..FR-C-38: Functional requirements for on-chain registries.
 *   - FR-C-20..FR-C-26: Reputation Registry feedback model.
 *   - FR-C-27..FR-C-33: Validation Registry verification model.
 *   - FR-C-29: ValidationResponse enum (0=pending, 1=pass, 2=fail).
 *
 * These types map Solidity primitives to TypeScript equivalents:
 *   - uint256 -> bigint (Uint256)
 *   - address -> string (Address, EIP-55 checksummed)
 *   - bytes32 -> Uint8Array (Bytes32, 32 bytes)
 *   - int128  -> bigint (Int128)
 *
 * All interfaces are readonly (immutable after construction).
 * No ethers.js dependency — pure type definitions.
 */

// ---------------------------------------------------------------------------
// Solidity Primitive Mappings
// ---------------------------------------------------------------------------

/** Solidity uint256 mapped to TypeScript bigint. */
export type Uint256 = bigint;

/** Solidity address mapped to string (EIP-55 checksummed). */
export type Address = string;

/** Solidity bytes32 mapped to Uint8Array (32 bytes). */
export type Bytes32 = Uint8Array;

/** Solidity int128 mapped to bigint. */
export type Int128 = bigint;

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/** Expected byte length for Bytes32 values. */
export const BYTES32_LENGTH = 32;

/** Maximum allowed decimals for fixed-point feedback values. FR-C-21 */
export const MAX_FEEDBACK_DECIMALS = 18;

// ---------------------------------------------------------------------------
// Reputation Registry Types
// ---------------------------------------------------------------------------

/**
 * Feedback entry from a client about a registered agent.
 *
 * FR-C-20: Recorded via giveFeedback().
 * FR-C-21: Fixed-point decimal: actual rating = value / 10^decimals.
 * FR-C-22: Revocable by original giver only.
 * FR-C-23: Agent owner may append a response.
 */
export interface FeedbackEntry {
  /** Address of the client who gave the feedback. FR-C-20 */
  readonly client: Address;

  /** Signed fixed-point value (numerator). FR-C-21 */
  readonly value: Int128;

  /** Decimal places (0-18). Actual rating = value / 10^decimals. FR-C-21 */
  readonly decimals: number;

  /** First categorical tag. FR-C-20, FR-C-24 */
  readonly tag1: Bytes32;

  /** Second categorical tag. FR-C-20, FR-C-24 */
  readonly tag2: Bytes32;

  /** Off-chain URI with feedback details. FR-C-20 */
  readonly feedbackURI: string;

  /** Keccak256 integrity hash of the feedbackURI content. FR-C-20 */
  readonly feedbackHash: Bytes32;

  /** Whether this feedback has been revoked. FR-C-22 */
  readonly revoked: boolean;

  /** Off-chain URI with the agent's response. FR-C-23 */
  readonly responseURI: string;

  /** Keccak256 integrity hash of the responseURI content. FR-C-23 */
  readonly responseHash: Bytes32;
}

// ---------------------------------------------------------------------------
// Validation Registry Types
// ---------------------------------------------------------------------------

/**
 * Validation response codes per FR-C-29.
 *
 * 0 = Pending (initial state when request is created).
 * 1 = Pass (validator confirms the agent's capabilities).
 * 2 = Fail (validator rejects the agent's capabilities).
 */
export enum ValidationResponse {
  /** Initial state. FR-C-29 */
  Pending = 0,
  /** Validator confirms capabilities. FR-C-29 */
  Pass = 1,
  /** Validator rejects capabilities. FR-C-29 */
  Fail = 2,
}

/**
 * Validation record for third-party verification of agent capabilities.
 *
 * FR-C-27: Created via validationRequest().
 * FR-C-28: Responded via validationResponse().
 * FR-C-30: Queryable via getValidationStatus().
 */
export interface ValidationRecord {
  /** Address of the third-party validator. FR-C-27 */
  readonly validator: Address;

  /** The agent's tokenId from the Identity Registry. FR-C-27, FR-C-33 */
  readonly agentId: Uint256;

  /** Off-chain URI describing the validation request. FR-C-27 */
  readonly requestURI: string;

  /** Validation outcome. FR-C-29: 0=pending, 1=pass, 2=fail. */
  readonly response: ValidationResponse;

  /** Off-chain URI with the validator's response details. FR-C-28 */
  readonly responseURI: string;

  /** Keccak256 integrity hash of the responseURI content. FR-C-28 */
  readonly responseHash: Bytes32;

  /** Categorical tag for filtering. FR-C-31 */
  readonly tag: Bytes32;

  /** Timestamp of the last update (block.timestamp). FR-C-30 */
  readonly lastUpdate: Uint256;
}

// ---------------------------------------------------------------------------
// Summary Types
// ---------------------------------------------------------------------------

/**
 * Aggregated feedback summary returned by Reputation Registry getSummary().
 *
 * FR-C-24: Filtered by tags and client addresses. Revoked feedback excluded.
 */
export interface FeedbackSummary {
  /** Number of non-revoked feedback entries matching the filter. FR-C-24 */
  readonly count: Uint256;

  /** Sum of values across matching entries. FR-C-24 */
  readonly totalValue: bigint;

  /** Common decimals for interpreting totalValue. FR-C-24 */
  readonly decimals: number;
}

/**
 * Aggregated validation summary returned by Validation Registry getSummary().
 *
 * FR-C-31: Filtered by validator addresses and tag.
 */
export interface ValidationSummary {
  /** Total number of validation records matching the filter. FR-C-31 */
  readonly count: Uint256;

  /** Number of records with response = Pass. FR-C-31 */
  readonly passCount: Uint256;

  /** Number of records with response = Fail. FR-C-31 */
  readonly failCount: Uint256;
}
