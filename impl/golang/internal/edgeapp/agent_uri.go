package edgeapp

import (
	"encoding/json"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// AgentURIServiceEntry is one services[] entry per spec 003 / EIP-8004's
// agentURI off-chain JSON. Iteration 3 builds a minimal version that lists
// the agent's three HCS topics under `neuron-topic` service entries.
type AgentURIServiceEntry struct {
	Type      string `json:"type"`
	TopicURI  string `json:"topicUri,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// AgentURIDocument is the shape the SDK serializes when registering on
// EIP-8004. Iteration 3 keeps it tiny; spec 003's full schema with
// registrations[] cross-registry references lands in iteration 4+.
type AgentURIDocument struct {
	EVMAddress string                 `json:"evmAddress"`
	Services   []AgentURIServiceEntry `json:"services"`
}

// buildSellerAgentURI returns a JSON-serialized agentURI document binding
// the seller's EVM address to its three HCS topics. The returned string is
// the entire agentURI value the SDK passes to EIP-8004 registration.
//
// Format is intentionally compact — full spec 003 agentURI schema is more
// elaborate, but iteration 3 is mock-only and the JSON shape is what the
// in-process MemoryRegistry stores.
func buildSellerAgentURI(evmHex string, stdIn, stdOut, stdErr topic.TopicRef) string {
	doc := AgentURIDocument{
		EVMAddress: evmHex,
		Services: []AgentURIServiceEntry{
			{Type: "neuron-topic", TopicURI: stdIn.URI(), Direction: "stdIn"},
			{Type: "neuron-topic", TopicURI: stdOut.URI(), Direction: "stdOut"},
			{Type: "neuron-topic", TopicURI: stdErr.URI(), Direction: "stdErr"},
		},
	}
	data, _ := json.Marshal(doc)
	return string(data)
}

// buildBuyerAgentURI is the buyer-side analogue. Buyer doesn't usually serve
// data streams to peers via these topic refs (it dials in to receive), but
// the same shape gives the registry a uniform record.
func buildBuyerAgentURI(evmHex string, stdIn, stdOut, stdErr topic.TopicRef) string {
	return buildSellerAgentURI(evmHex, stdIn, stdOut, stdErr)
}
