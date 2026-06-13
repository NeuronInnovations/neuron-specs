/**
 * Health module protocol constants.
 *
 * Spec reference: 005 spec.md, data-model.md
 * All time-based constants are in seconds as bigint values.
 */

/** Minimum deadline delta in seconds (FR-H06). */
export const MIN_DEADLINE_DELTA = 10n;

/** Maximum deadline delta in seconds — 24 hours (FR-H07). */
export const MAX_DEADLINE_DELTA = 86400n;

/** Grace period in seconds before transitioning from ALIVE to SUSPECT (FR-H08). */
export const GRACE_PERIOD = 30n;

/** Time in seconds after grace period before transitioning from SUSPECT to DEAD (FR-H09). */
export const SUSPECT_TO_DEAD = 120n;

/** Shutdown sentinel value for nextHeartbeatDeadline (FR-H12). */
export const SHUTDOWN_SENTINEL = 0n;

/** Maximum size in bytes for mandatory payload fields (FR-H29). */
export const MAX_MANDATORY_PAYLOAD_SIZE = 256;
