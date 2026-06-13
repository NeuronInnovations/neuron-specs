package sapient

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

func checkByName(t *testing.T, checks []CardCheck, name string) CardCheck {
	t.Helper()
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("check %q not found in %+v", name, checks)
	return CardCheck{}
}

// TestVerifyResolvedCard_AllBindingsPass: a freshly built card, resolved with
// its true owner, passes node_id/protocol/commerce and reports V-REG-12 as
// SKIPPED (the chain has no pubkey).
func TestVerifyResolvedCard_AllBindingsPass(t *testing.T) {
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	card, err := BuildSellerCard(SellerCardOptions{ChildKey: &k})
	require.NoError(t, err)

	// Round-trip through JSON: the explorer always sees the on-chain string,
	// never the in-memory struct.
	js, err := card.AgentURI.ToJSON()
	require.NoError(t, err)
	uri, err := registry.AgentURIFromJSON(js)
	require.NoError(t, err)

	checks := VerifyResolvedCard(uri, k.PublicKey().EVMAddress())
	require.Equal(t, CheckPass, checkByName(t, checks, "node_id↔owner").Status)
	require.Equal(t, CheckPass, checkByName(t, checks, "protocol").Status)
	require.Equal(t, CheckPass, checkByName(t, checks, "commerce[rid]").Status)
	require.Equal(t, CheckSkipped, checkByName(t, checks, "peerID↔pubkey (V-REG-12)").Status)
}

// TestVerifyResolvedCard_NodeIDMismatchFails: resolving the same card against
// a DIFFERENT owner address must fail the node_id binding — the check that
// catches a card registered under a key it does not belong to.
func TestVerifyResolvedCard_NodeIDMismatchFails(t *testing.T) {
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	card, err := BuildSellerCard(SellerCardOptions{ChildKey: &k})
	require.NoError(t, err)

	other, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	checks := VerifyResolvedCard(card.AgentURI, other.PublicKey().EVMAddress())
	nodeCheck := checkByName(t, checks, "node_id↔owner")
	require.Equal(t, CheckFail, nodeCheck.Status)
	require.Contains(t, nodeCheck.Detail, "!=")
}

// TestExtensionNodeID_RoundTrip: the nodeId survives the on-chain JSON
// round-trip (map[string]any decoding).
func TestExtensionNodeID_RoundTrip(t *testing.T) {
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	card, err := BuildSellerCard(SellerCardOptions{ChildKey: &k})
	require.NoError(t, err)

	js, err := card.AgentURI.ToJSON()
	require.NoError(t, err)
	uri, err := registry.AgentURIFromJSON(js)
	require.NoError(t, err)

	require.Equal(t, card.NodeID, ExtensionNodeID(uri))
}

// TestExtensionNodeID_AbsentIsEmpty: a card with no neuron.rid/1 extension
// yields "" rather than a fabricated id.
func TestExtensionNodeID_AbsentIsEmpty(t *testing.T) {
	require.Empty(t, ExtensionNodeID(registry.AgentURI{}))
}
