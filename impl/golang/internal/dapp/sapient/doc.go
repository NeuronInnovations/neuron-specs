// Package sapient is the SDK-side SAPIENT (BSI Flex 335 v2.0) protocol + data
// layer for the local SAPIENT Remote ID demo. It provides:
//
//   - sapientpb: the vendored DSTL BSI Flex 335 v2.0 protobuf message set.
//   - ProtocolDetection ("/sapient/detection/2.0.0"): the libp2p protocol ID for
//     the assembled DetectionReport stream (catalog.go).
//   - NodeIDFromIdentity: derive a SAPIENT node_id from a Neuron identity; the
//     seller stamps it onto every message before publishing (the bridge's
//     placeholder node_id is never trusted — nodeid.go).
//   - ReadBridgeFeed: stream SapientMessages from the live neuron-rid-bridge
//     SAPIENT feed (its --sapient-listen address, --sapient-format json;
//     bridge_source.go).
//   - EncodeMessage + the ICD DetectionReport model (icd.go / encode.go): a
//     utility to construct a SapientMessage from a Go model. The live pipeline
//     forwards the bridge's already-encoded messages, so this is exercised by
//     tests + available for non-bridge producers.
//
// Transport roles follow the Neuron reverse-connect topology — the SELLER dials
// the reachable BUYER and pushes; the BUYER listens and receives — so they live
// in the demo cmds (cmd/sapient-rid-seller, cmd/sapient-buyer), not here. The
// buyer-side SAPIENT->FID adapter is in the fidadapt sub-package.
//
// This package is strictly additive and does NOT touch the /ds240/* Remote ID
// streams owned by internal/dapp/remoteid.
package sapient
