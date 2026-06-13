// Package browserprofile implements the server-side pieces of the Neuron
// Browser Client Profile (spec 012) that are needed by the 2a-wt
// WebTransport spike.
//
// Tier A (this pass): an echo stream handler + a bootstrap JSON writer
// paired with a WebTransport listener. Proves direct browser -> VPS
// connectivity without the SSH tunnel used by Tier 1.
//
// Tier B (contingent): port of the 4-message control flow and
// SHA-256-terminated file-send protocol currently in
// impl/typescript/src/server-demo/. Not included yet -- gated on Tier A
// success per the approved plan.
//
// This package is intentionally a PARALLEL of the TypeScript Tier 1
// seller. It does NOT replace any existing code. Spec 012 v1 (FR-B09)
// still mandates WSS-only for the browser profile; this work is a v2a
// feasibility experiment.
package browserprofile
