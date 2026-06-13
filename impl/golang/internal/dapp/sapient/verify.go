package sapient

import (
	"fmt"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// CardCheck statuses for VerifyResolvedCard lines. SKIPPED marks a check a
// chain-side verifier cannot run (honest evidence: say what was NOT checked).
const (
	CheckPass    = "PASS"
	CheckFail    = "FAIL"
	CheckSkipped = "SKIPPED"
)

// CardCheck is one verification line a verifier (the agent explorer, a buyer)
// derives from an Agent Card resolved out of the Identity Registry.
type CardCheck struct {
	Name   string
	Status string
	Detail string
}

// ExtensionNodeID extracts the neuron.rid/1 nodeId from the card's stdOut
// topic config. Returns "" when the extension or field is absent. Works on
// cards freshly built AND cards round-tripped through on-chain JSON (the
// extension decodes back as map[string]any).
func ExtensionNodeID(uri registry.AgentURI) string {
	for _, ts := range uri.TopicServices() {
		if ts.Channel != "stdOut" {
			continue
		}
		ext, ok := ts.Config[ExtensionID].(map[string]any)
		if !ok {
			continue
		}
		if v, ok := ext["nodeId"].(string); ok {
			return v
		}
	}
	return ""
}

// VerifyResolvedCard runs the chain-side verification a buyer can perform on
// a card resolved from the Identity Registry:
//
//   - node_id ↔ owner — the SAPIENT node_id in the neuron.rid/1 extension must
//     equal NodeIDFromIdentity(token owner) (015 FR-S94 identity binding);
//   - protocol — a neuron-p2p-exchange service must advertise
//     /sapient/detection/2.0.0 (015 FR-S11);
//   - commerce[rid] — the rid neuron-commerce entry must be present (FR-S20a).
//
// The pubkey-derived PeerID check (V-REG-12) is reported SKIPPED: the chain
// stores the owner ADDRESS, not the public key, so it cannot be re-derived by
// a chain-only verifier — the PeerID binding is verified at dial time instead
// (the seller dials with the same key it registered with).
func VerifyResolvedCard(uri registry.AgentURI, owner keylib.EVMAddress) []CardCheck {
	checks := make([]CardCheck, 0, 4)

	wantNode := NodeIDFromIdentity(owner.Hex())
	gotNode := ExtensionNodeID(uri)
	nodeCheck := CardCheck{Name: "node_id↔owner", Status: CheckPass, Detail: gotNode}
	if gotNode != wantNode {
		nodeCheck.Status = CheckFail
		nodeCheck.Detail = fmt.Sprintf("card nodeId %q != NodeIDFromIdentity(owner) %q", gotNode, wantNode)
	}
	checks = append(checks, nodeCheck)

	protoCheck := CardCheck{Name: "protocol", Status: CheckFail,
		Detail: "no neuron-p2p-exchange service advertises " + ProtocolDetection}
	for _, p := range uri.P2PServices() {
		if p.Protocol == ProtocolDetection {
			protoCheck = CardCheck{Name: "protocol", Status: CheckPass, Detail: ProtocolDetection}
			break
		}
	}
	checks = append(checks, protoCheck)

	commerceCheck := CardCheck{Name: "commerce[rid]", Status: CheckFail,
		Detail: "no neuron-commerce service named " + CommerceServiceName}
	for _, c := range uri.CommerceServices() {
		if c.Name == CommerceServiceName {
			commerceCheck = CardCheck{Name: "commerce[rid]", Status: CheckPass,
				Detail: fmt.Sprintf("%s %s (settlement %s, %s %s/%s)",
					c.Name, c.Version, c.Settlement.Binding,
					c.Pricing.Amount, c.Pricing.Currency, c.Pricing.Unit)}
			break
		}
	}
	checks = append(checks, commerceCheck)

	checks = append(checks, CardCheck{
		Name:   "peerID↔pubkey (V-REG-12)",
		Status: CheckSkipped,
		Detail: "chain stores the owner address, not the public key — PeerID binding is verified at dial time",
	})
	return checks
}
