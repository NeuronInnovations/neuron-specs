// Package remoteid is the Remote ID DApp (spec 017).
//
// It composes Core SDK primitives (008 commerce, 009 P2P delivery, 011 relay,
// 013 Profile E) into a Remote ID-specific application: a seller advertises
// a Remote ID stream catalog (see BuildRemoteIDStreamCatalog), a buyer
// negotiates an agreement, and the seller publishes normalized
// canonical-JSON RemoteIdFrame payloads (per 017 FR-R05) on the libp2p
// stream protocol "/ds240/raw/1.0.0".
//
// Per Constitution Principle XII (v1.7.0), this is a DApp spec, not Core
// SDK. The package contains domain-specific decisions (the canonical
// frame envelope shape, the stream catalog paths, the seller's choice of
// fixture vs DS-400 source) that do NOT live in the Core SDK. The frame
// format is normalized canonical JSON per the precedent rule R-FF-02
// (`docs/dapp-frame-format-precedent.md`) — Remote ID has multiple
// regulatory variants (FAA / EASA / ASD-STAN) that benefit from
// seller-side normalization to a uniform consumer shape.
//
// Phase 2 (reference MVP) of this DApp ships:
//   - RemoteIdFrame canonical-JSON encoder + Marshal/Unmarshal round-trip.
//   - BuildRemoteIDStreamCatalog returning the {raw, filtered, status}
//     catalog. Phase 2 wires only `/ds240/raw/1.0.0` end-to-end; the
//     filtered/status entries are advertised but not yet handled.
//   - A seller orchestrator (Run) that registers the libp2p stream
//     handler and pumps frames from a feed source through the canonical
//     encoder onto each accepted stream. Uses the existing
//     delivery.FrameWriter for length-prefixed framing (009 FR-D22).
//
// Out of scope for this package:
//   - The Open Drone ID decoder (lives in internal/feeds/remoteid).
//   - The DS-400 vendor source adapter (Phase 3).
//   - Fan-out via gossipsub (Phase 6 production-readiness).
//   - Concrete AdmissionPolicy implementations (Phase 6).
//
// See specs/017-remote-id-dapp/spec.md for the full DApp specification.
package remoteid
