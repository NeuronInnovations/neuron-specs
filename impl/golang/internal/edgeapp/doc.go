// Package edgeapp wires together the spec-driven Neuron Go SDK pieces into a
// runnable edge-seller / edge-buyer pair for the reverse-connect topology
// (NAT'd seller, reachable buyer).
//
// The package exposes two run loops:
//
//	RunSeller(ctx, SellerConfig) error
//	RunBuyer(ctx, BuyerConfig) error
//
// Each takes an injected topic.TopicAdapter (a memory bus for tests, or a
// real HCS adapter for testnet runs) plus the agent's own ECDSA private key
// and topic refs. The seller dials out to a public buyer over libp2p QUIC
// after observing a ReverseConnectionSetup on its stdIn topic; the buyer
// publishes that ReverseConnectionSetup to the seller's stdIn after
// announcing its presence with a heartbeat.
//
// MemoryBus is a fully Subscribe-capable in-memory topic.TopicAdapter
// suitable for a single-process E2E test of RunSeller and RunBuyer.
//
// The package is the "library" half; cmd/edge-seller and cmd/edge-buyer are
// thin main wrappers that load configuration from the environment.
package edgeapp
