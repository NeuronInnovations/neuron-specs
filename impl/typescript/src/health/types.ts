/**
 * Health module type definitions.
 *
 * Spec reference: 005 spec.md, data-model.md
 * Defines semantic types for heartbeat payloads and liveness tracking.
 */

// ---------------------------------------------------------------------------
// NodeRole — FR-H02, FR-H05
// ---------------------------------------------------------------------------

/** The role a node declares in its heartbeat. */
export type NodeRole = 'buyer' | 'seller' | 'relay' | 'validator';

/** Canonical list of valid roles for runtime validation. V-PUB-07, V-OBS */
export const VALID_ROLES: readonly NodeRole[] = Object.freeze(['buyer', 'seller', 'relay', 'validator'] as const);

// ---------------------------------------------------------------------------
// NATType — FR-H03
// ---------------------------------------------------------------------------

/** NAT traversal classification. */
export type NATType = 'no-nat' | 'endpoint-independent' | 'address-dependent' | 'address-and-port-dependent';

/** Canonical list of valid NAT types. */
export const VALID_NAT_TYPES: readonly NATType[] = [
  'no-nat',
  'endpoint-independent',
  'address-dependent',
  'address-and-port-dependent',
] as const;

// ---------------------------------------------------------------------------
// GPSFixQuality — FR-H03
// ---------------------------------------------------------------------------

/** GPS fix quality indicator. */
export type GPSFixQuality = 'none' | '2D' | '3D';

/** Canonical list of valid GPS fix qualities. */
export const VALID_GPS_FIX_QUALITIES: readonly GPSFixQuality[] = ['none', '2D', '3D'] as const;

// ---------------------------------------------------------------------------
// Capabilities — FR-H03
// ---------------------------------------------------------------------------

/** Node capabilities object. Serialization order: natReachability, natType, protocols. */
export interface Capabilities {
  readonly natReachability: boolean;
  readonly natType?: NATType;
  readonly protocols?: readonly string[];
}

// ---------------------------------------------------------------------------
// Location — FR-H03
// ---------------------------------------------------------------------------

/** Geographic location object. Serialization order: lat, lon, alt, fix. */
export interface Location {
  readonly lat: number;
  readonly lon: number;
  readonly alt?: number;
  readonly fix?: GPSFixQuality;
}

// ---------------------------------------------------------------------------
// LivenessState — FR-H18, FR-H19, FR-H20
// ---------------------------------------------------------------------------

/**
 * 5-state liveness model.
 *
 * State machine transitions (data-model.md):
 *   UNKNOWN --[first valid HB]--> ALIVE
 *   ALIVE   --[new valid HB]--> ALIVE (reset deadline)
 *   ALIVE   --[now > deadline + GP]--> SUSPECT
 *   ALIVE   --[deadline = 0]--> OFFLINE
 *   SUSPECT --[new valid HB]--> ALIVE
 *   SUSPECT --[now > deadline + GP + S2D]--> DEAD
 *   DEAD    --[new valid HB]--> ALIVE
 *   OFFLINE --[new valid HB with deadline > 0]--> ALIVE
 */
export enum LivenessState {
  UNKNOWN = 'UNKNOWN',
  ALIVE = 'ALIVE',
  SUSPECT = 'SUSPECT',
  DEAD = 'DEAD',
  OFFLINE = 'OFFLINE',
}

// ---------------------------------------------------------------------------
// LivenessRecord — FR-H17, FR-H18
// ---------------------------------------------------------------------------

/**
 * Observer-local tracking state per observed peer.
 * Not persisted on-chain -- maintained by the observer.
 */
export interface LivenessRecord {
  readonly senderAddress: string;
  readonly currentState: LivenessState;
  readonly lastDeadline: bigint;
  readonly lastSequence: bigint;
  readonly lastConsensusTimestamp: bigint;
}
