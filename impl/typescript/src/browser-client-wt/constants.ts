// 2a-wt — constants for the WebTransport browser-demo variant.
//
// Distinct from src/browser-client/constants.ts to keep the Tier 1 WSS
// profile untouched. Values here parallel the Go seller's
// internal/browserprofile package.

/** Bootstrap JSON schema version for the WebTransport variant. */
export const BOOTSTRAP_VERSION_WT = '2a-wt'

/** Tier A echo protocol — proves transport + Noise handshake. */
export const ECHO_PROTOCOL_ID = '/neuron/webtransport-spike/echo/1.0.0'

/** Reserved for Tier B — matches Spec 012 constants. */
export const CONTROL_PROTOCOL_ID = '/neuron/browser-profile/control/1.0.0'

/** Reserved for Tier B — matches Spec 012 constants. */
export const DATA_PROTOCOL_ID = '/neuron/browser-profile/data/1.0.0'

/** Bounds for a single echo payload line (matches Go handler). */
export const ECHO_MAX_LINE_BYTES = 4096
