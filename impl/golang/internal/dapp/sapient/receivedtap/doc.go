// Package receivedtap is a read-only recipient-side tap that exposes the SAPIENT
// messages received by the consumer (cmd/sapient-fid-consumer) as a
// partner-friendly JSON testing projection — DISTINCT from the map/FID/UI event
// stream.
//
// The canonical wire stays SAPIENT protobuf (BSI Flex 335 v2.0) on
// /sapient/detection/2.0.0; this package produces a lossy JSON view of the
// already-decoded *sapientpb.SapientMessage, captured at the consumer's receive
// boundary BEFORE any map/FID projection. It exists so external partners can
// verify that the payload produced by the bridge arrives and decodes correctly
// on the recipient side, independent of the map UI.
//
// Three pieces:
//
//   - Projection / Project: the JSON testing projection of one received message.
//     Optional blocks are pointers with omitempty — nil means the field was
//     absent on the wire (intentionally omitted), never zero-filled.
//   - ReceivedSapientStore: a bounded, concurrent in-memory store (a circular
//     ring of the last N projections plus a latest-per-object index) with a
//     non-blocking fan-out to streaming subscribers. It never backpressures the
//     data plane: Record does a bounded write then a drop-on-full broadcast.
//   - Handler: the read-only HTTP surface — /sapient/received/{latest,stream,
//     schema,health}. No secrets, env, keys, or host paths are ever exposed.
//
// The store and handler are transport-agnostic and import nothing from package
// main, so a standalone read-only sidecar could reuse them unchanged.
package receivedtap
