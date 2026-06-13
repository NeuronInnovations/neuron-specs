// Package adsb is the ADS-B DApp (spec 016) BaseStation fast-path slice.
//
// This package is the sibling of internal/dapp/remoteid — same template,
// different domain. Where remoteid serves Open Drone ID broadcasts on
// /ds240/raw/1.0.0 with canonical-JSON RemoteIdFrame, adsb serves
// decoded ADS-B tracks on /jetvision/basestation/1.0.0 with canonical-JSON
// NormalizedTrack per docs/normalized-track-contract.md.
//
// Scope (per the BaseStation fast-fusion audit §1):
// BaseStation is additive / demo-grade / decoded-track shortcut /
// lower-fidelity than raw paths / not sufficient by itself for raw
// evidence claims. The existing /jetvision/raw/1.0.0 BEAST path (served by
// cmd/edge-seller / cmd/edge-buyer / internal/edgeapp) is untouched and
// remains the high-fidelity evidence-bearing path.
//
// This package's load-bearing surface:
//   - NormalizedTrack (frame.go) — wire-format payload per the contract doc.
//   - FromSBSTrack (frame.go) — convert internal/feeds/sbs.SBSTrack to
//     NormalizedTrack, performing imperial → SI metric conversion
//     (feet → metres, knots → m/s, fpm → m/s).
//   - BuildAdsbBasestationStreamCatalog (catalog.go) — the
//     payment.StreamCatalogEntry advertisement for the basestation stream.
//   - Start (seller.go) — register the libp2p stream handler on
//     /jetvision/basestation/1.0.0 and pump a feed source through canonical
//     JSON to the wire.
//
// Out of v1 scope (deferred to follow-up sessions; see plan and audit):
//   - heartbeat.go / liveness_monitor.go (FR-R21-equivalent operational
//     disclosure + buyer-side state machine).
//   - registry.go / connection.go (full EIP-8004 + ECIES wrappers).
//   - cli_orchestrator.go / lifecycle.go / negotiation.go / session.go /
//     settlement.go (Stage-2b commerce orchestration).
//   - evm_backend.go / hcs_backend.go (HCS + EVM backend factories).
//
// Until the deferred surface lands, the demo slice runs in fixture-direct
// mode only (no on-chain registration); the audit and the contract doc
// document this gap. The pattern is fully transferable from
// internal/dapp/remoteid when the orchestration layer is brought across.
package adsb
