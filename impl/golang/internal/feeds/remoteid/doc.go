// Package remoteid provides edge-side Remote ID source adapters for the
// Neuron Go SDK.
//
// "Remote ID" here refers to ASTM F3411-22a / FAA Part 89 / EASA 2019/945
// drone broadcast identification — Open Drone ID. A Remote ID source emits
// DecodedFrame records (one per detected drone broadcast) that downstream
// code can normalize into the canonical-JSON RemoteIdFrame payload defined
// by spec 017 FR-R05.
//
// Sources currently provided:
//
//   - RunReplay — replays a JSON fixture file (an array of frames with
//     per-entry offsetMs pacing hints). Useful as the Phase 2 fixture
//     input for the reference MVP vertical slice, and as a deterministic
//     test fixture once real DS-400 captures are available.
//   - RunSynth — emits synthetic Remote ID frames at a configurable rate
//     by simulating N drones moving in deterministic circular paths
//     around a fixed center. Useful when no fixture file is available
//     and the operator wants a plausible live feed.
//
// All sources share the shape:
//
//	func RunXxx(ctx context.Context, ..., out chan<- DecodedFrame) error
//
// They emit DecodedFrames on out until ctx is cancelled or the source is
// exhausted, then return. They do not close out — that is the caller's
// responsibility (the caller knows when no further sources will write).
//
// Downstream code in internal/dapp/remoteid converts DecodedFrame to the
// canonical-JSON RemoteIdFrame envelope per 017 FR-R05 + 006 wire-format
// §2 RemoteIdFrame ordering.
//
// A future ds400_source.go in this package will provide a RunDS400 source
// reading from a DroneScout DS-400 receiver's UDP/TCP feed; that work is
// gated on DS-400 device access (Phase 3 of the reference MVP plan) and is
// out of scope for this initial replay/synth pair.
package remoteid
