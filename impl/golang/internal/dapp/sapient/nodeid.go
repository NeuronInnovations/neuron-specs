package sapient

import "github.com/google/uuid"

// neuronSapientNamespace is the fixed UUID namespace for deriving a SAPIENT
// node_id from a Neuron identity. PROVISIONAL (Phase 1): both this namespace
// value and the choice of identity anchor (the EVM address, below — vs the
// libp2p PeerID or the EIP-8004 agentId) are not yet canonical. See
// the SAPIENT RID runtime-boundary notes, "Deferred gaps".
var neuronSapientNamespace = uuid.MustParse("9f1a7b3c-2d4e-5f60-8a90-1b2c3d4e5f60")

// PlaceholderNodeID is the reference bridge's default --node-id. It MUST NEVER be
// treated as a production-valid identity (spec 015 FR-S15); a seller emitting it
// is non-conformant, and Start rejects it.
const PlaceholderNodeID = "11111111-2222-3333-4444-555555555555"

// NodeIDFromIdentity derives a deterministic, identity-bound SAPIENT node_id (a
// UUID string) from the seller's Neuron identity — here its EVM address hex. It
// is a stable RFC-4122 v5 UUID over the address: the same identity always yields
// the same node_id, and it is never the bridge placeholder.
//
// PROVISIONAL: the canonical derivation (which identity anchor + which namespace)
// is a deferred decision; this is sufficient for Phase 1's requirement that
// node_id be Neuron-identity-bound and not the placeholder (015 FR-S15).
func NodeIDFromIdentity(evmAddrHex string) string {
	return uuid.NewSHA1(neuronSapientNamespace, []byte(evmAddrHex)).String()
}
