package sapient

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// TestGenerateRIDCardGolden regenerates testdata/rid-card-golden.json from the
// CURRENT BuildSellerCard output for the fixed test key. Run only when
// SAPIENT_WRITE_GOLDEN=1; the byte-identity regression test consumes the file.
func TestGenerateRIDCardGolden(t *testing.T) {
	if os.Getenv("SAPIENT_WRITE_GOLDEN") != "1" {
		t.Skip("set SAPIENT_WRITE_GOLDEN=1 to regenerate")
	}
	k, err := keylib.NeuronPrivateKeyFromHex("1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(t, err)
	card, err := BuildSellerCard(SellerCardOptions{ChildKey: &k})
	require.NoError(t, err)
	j, err := card.AgentURI.ToJSON()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("testdata/rid-card-golden.json", []byte(j), 0o644))
	t.Logf("golden written: nodeID=%s peerID=%s bytes=%d", card.NodeID, card.PeerID, len(j))
}
