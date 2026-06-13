// Package delivery implements Spec 009 — P2P Data Delivery, the data plane
// for Neuron agent-to-agent service delivery.
//
// # Architecture
//
// The delivery package sits at the end of the SDK dependency chain:
//
//	keylib → payment → delivery
//
// It consumes Spec 008's control-plane hooks (delivery descriptor in
// neuron-commerce, connectionSetup message, encryption requirement FR-P34)
// and provides:
//
//   - An abstract DeliveryAdapter interface (connect, send, receive,
//     disconnect, getStatus) analogous to Spec 004's TopicAdapter
//   - ECIES multiaddr encryption (secp256k1 ECDH + HKDF-SHA256 + AES-256-GCM)
//     satisfying 008 FR-P34
//   - A connection lifecycle state machine (6 states, 12 transitions)
//   - Length-prefixed stream data framing (4-byte BE + payload, 4 MiB max)
//   - Exponential backoff reconnection (5s initial, 2x, 10min cap, 1hr max)
//   - libp2p binding as the normative first DeliveryAdapter implementation
//
// # Design Decisions
//
// DD-D01: ECDH is self-contained in ecies.go using go-ethereum/crypto
// directly, not added to keylib (Spec 002). Keeps ECIES co-located with
// its only consumer. Avoids spec amendment.
//
// DD-D02: libp2p host is created per-agent (not per-channel) and shared
// across delivery channels. Current binding listens on QUIC-v1 only
// (FR-D25 WebRTC/WebTransport remain future work). When WithRelay is passed
// the host enables Circuit Relay v2 client, AutoNAT v2, hole punching (DCUtR),
// UPnP, and — if static relays are supplied — autorelay with those relays.
//
// DD-D03: ConnectionState machine is local runtime state, not protocol-level.
// SC-D03 tests determinism given identical transport event sequences.
//
// DD-D04: Length-prefix framing wraps application data on libp2p streams.
// Implemented as FrameReader/FrameWriter wrapping io.Reader/io.Writer.
//
// DD-D05: Backoff parameters (5s/2x/10min/1hr) are defaults in BackoffConfig.
// Configurable for testing.
//
// # Spec References
//
// Spec 009: specs/009-p2p-data-delivery/spec.md
// Data Model: specs/009-p2p-data-delivery/data-model.md
// Contracts: specs/009-p2p-data-delivery/contracts/
package delivery
