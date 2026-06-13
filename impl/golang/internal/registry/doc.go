// Package registry implements Spec 003 (Peer Registry) — EIP-8004 registration
// lifecycle for Neuron Child accounts.
//
// A Child account registers in an EIP-8004 Identity Registry smart contract,
// receiving an extended NFT (ERC-721). The registration's agentURI carries
// mandatory services: three neuron-topic services (stdIn, stdOut, stdErr) and
// one neuron-p2p-exchange service.
//
// Registration operations (create, update, revoke) are signed by the Child's
// NeuronPrivateKey as proof-of-control (FR-R06). The registry MAY enforce an
// admission policy anchored to the Parent's DID (FR-R09).
//
// Dependencies:
//   - internal/keylib (Spec 002): NeuronPrivateKey, NeuronPublicKey, EVMAddress, PeerID, DIDKey
//   - internal/account (Spec 001): Child identity, RegistryBinding
package registry
