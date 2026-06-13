/**
 * HeartbeatPayload -- the primary heartbeat data structure.
 *
 * Spec reference: 005 spec.md
 *   - FR-H01: HeartbeatPayload is a JSON object published as TopicMessage payload.
 *   - FR-H02: Mandatory fields: type, version, nextHeartbeatDeadline, role.
 *   - FR-H03: Optional fields: capabilities, location, peers.
 *   - FR-H04: Canonical serialization order: type -> version -> nextHeartbeatDeadline -> role -> capabilities -> location -> peers.
 *   - FR-H05: Role values: buyer, seller, relay. Validator reserved.
 *
 * Spec reference: 006 wire-format.md
 *   - FR-W01: Compact JSON (no whitespace).
 *   - FR-W02: UnsignedInt64 as JSON strings.
 *   - FR-W04: Absent optional fields omitted (not null).
 *   - FR-W05: Keys in canonical order per entity.
 *   - FR-W10: Float64 with .0 for integers.
 *
 * Immutable value type. Valid by construction.
 */

import { serializeCanonicalJson } from '../wire/canonical-json.js';
import type { CanonicalField } from '../wire/canonical-json.js';
import type { NodeRole, Capabilities, Location } from './types.js';

// ---------------------------------------------------------------------------
// Build options
// ---------------------------------------------------------------------------

/** Options for constructing a HeartbeatPayload. */
export interface HeartbeatPayloadOpts {
  /** Unix timestamp (nanoseconds) for next heartbeat deadline. 0n = shutdown. FR-H02. */
  readonly nextHeartbeatDeadline: bigint;
  /** Node role. FR-H02, FR-H05. */
  readonly role: NodeRole;
  /** Node capabilities. FR-H03. */
  readonly capabilities?: Capabilities;
  /** Geographic location. FR-H03. */
  readonly location?: Location;
  /** Abbreviated peer addresses (last 4 hex chars). FR-H03, FR-H27. */
  readonly peers?: readonly string[];
}

// ---------------------------------------------------------------------------
// HeartbeatPayload class
// ---------------------------------------------------------------------------

/**
 * HeartbeatPayload -- immutable value type representing a heartbeat message.
 *
 * FR-H01: Constructed via static `build()` factory which auto-sets type and version.
 * FR-H04: `toCanonicalJson()` serializes in the canonical field order.
 *
 * Construction through `build()` ensures every instance has type="heartbeat"
 * and version="1.0.0" set correctly.
 */
export class HeartbeatPayload {
  /** FR-H02: Always "heartbeat". */
  readonly type = 'heartbeat' as const;

  /** FR-H02, FR-H28: SemVer version string. */
  readonly version = '1.0.0';

  /** FR-H02: Next heartbeat deadline in nanoseconds. 0n = shutdown sentinel (FR-H12). */
  readonly nextHeartbeatDeadline: bigint;

  /** FR-H02, FR-H05: Node role. */
  readonly role: NodeRole;

  /** FR-H03: Node capabilities (optional). */
  readonly capabilities?: Capabilities | undefined;

  /** FR-H03: Geographic location (optional). */
  readonly location?: Location | undefined;

  /** FR-H03, FR-H27: Abbreviated peer addresses (optional). */
  readonly peers?: readonly string[] | undefined;

  private constructor(opts: HeartbeatPayloadOpts) {
    this.nextHeartbeatDeadline = opts.nextHeartbeatDeadline;
    this.role = opts.role;
    if (opts.capabilities !== undefined) {
      this.capabilities = opts.capabilities;
    }
    if (opts.location !== undefined) {
      this.location = opts.location;
    }
    if (opts.peers !== undefined) {
      this.peers = [...opts.peers];
    }
  }

  // -------------------------------------------------------------------------
  // Factory
  // -------------------------------------------------------------------------

  /**
   * Build a HeartbeatPayload from the given options.
   *
   * FR-H01: type and version are set automatically.
   * FR-H02: nextHeartbeatDeadline and role are required.
   *
   * This factory does NOT validate. Call `validateOutboundHeartbeat()` separately
   * before signing and publishing.
   *
   * @param opts - Heartbeat payload options
   * @returns Immutable HeartbeatPayload instance
   */
  static build(opts: HeartbeatPayloadOpts): HeartbeatPayload {
    return new HeartbeatPayload(opts);
  }

  // -------------------------------------------------------------------------
  // Canonical JSON serialization
  // -------------------------------------------------------------------------

  /**
   * Serialize to canonical JSON.
   *
   * FR-H04: Field order: type -> version -> nextHeartbeatDeadline -> role -> capabilities -> location -> peers.
   * FR-W01: Compact format -- no whitespace.
   * FR-W02: nextHeartbeatDeadline as JSON string (UnsignedInt64).
   * FR-W04: Absent optional fields omitted.
   * FR-W05: Keys emitted in canonical order.
   * FR-W10: Float64 values (lat, lon, alt) with .0 for integers.
   *
   * Nested objects (capabilities, location) are serialized with their own canonical
   * field order using separate serializeCanonicalJson calls, then embedded as
   * pre-serialized JSON fragments in the outer object.
   *
   * @returns Canonical JSON string
   */
  toCanonicalJson(): string {
    // --- Serialize nested capabilities object ---
    let capabilitiesJson: string | undefined;
    if (this.capabilities !== undefined) {
      const capFields: CanonicalField[] = [
        { key: 'natReachability', type: 'boolean', value: this.capabilities.natReachability },
        { key: 'natType', type: 'string', value: this.capabilities.natType },
        { key: 'protocols', type: 'array_string', value: this.capabilities.protocols ? [...this.capabilities.protocols] : undefined },
      ];
      capabilitiesJson = serializeCanonicalJson(capFields);
    }

    // --- Serialize nested location object ---
    let locationJson: string | undefined;
    if (this.location !== undefined) {
      const locFields: CanonicalField[] = [
        { key: 'lat', type: 'float64', value: this.location.lat },
        { key: 'lon', type: 'float64', value: this.location.lon },
        { key: 'alt', type: 'float64', value: this.location.alt },
        { key: 'fix', type: 'string', value: this.location.fix },
      ];
      locationJson = serializeCanonicalJson(locFields);
    }

    // --- Build top-level fields in canonical order (FR-H04) ---
    const fields: CanonicalField[] = [
      { key: 'type', type: 'string', value: this.type },
      { key: 'version', type: 'string', value: this.version },
      { key: 'nextHeartbeatDeadline', type: 'uint64', value: this.nextHeartbeatDeadline },
      { key: 'role', type: 'string', value: this.role },
      { key: 'capabilities', type: 'json', value: capabilitiesJson },
      { key: 'location', type: 'json', value: locationJson },
      { key: 'peers', type: 'array_string', value: this.peers ? [...this.peers] : undefined },
    ];
    return serializeCanonicalJson(fields);
  }
}
