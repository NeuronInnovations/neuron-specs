// Spec 012 — centralised constants. No magic numbers in module bodies.
// Every value here is either:
// - traced to an FR-B## in spec.md, or
// - a tested bound from contracts/ or research.md.
// See data-model.md §"Derived constants" for the authoritative table.

/** Read-idle timeout for the data-plane stream. FR-B34. Upper bound 30 s; Phase 1 uses 15 s. */
export const READ_IDLE_MS = 15_000

/** Maximum single-frame payload size in bytes. FR-B20 (4 MiB). Matches Spec 009 framing. */
export const MAX_FRAME_BYTES = 4 * 1024 * 1024

/** Maximum total-payload size the browser will accept from frame 0 metadata. FR-B21 v1 cap (1 MiB). */
export const MAX_TOTAL_BYTES = 1 * 1024 * 1024

/** libp2p stream protocol for the in-stream TopicAdapter. FR-B11. */
export const CONTROL_PROTOCOL_ID = '/neuron/browser-profile/control/1.0.0'

/** libp2p stream protocol for the data plane (frame 0 metadata + chunks). FR-B11. */
export const DATA_PROTOCOL_ID = '/neuron/browser-profile/data/1.0.0'

/** Bootstrap JSON schema version expected by the browser. contracts/bootstrap-json.md. */
export const BOOTSTRAP_VERSION = 1

/** ECIES HKDF info string. Matches impl/golang/internal/delivery/ecies.go. FR-B16, Spec 009 FR-D16. */
export const ECIES_INFO = 'neuron-multiaddr-v1'
