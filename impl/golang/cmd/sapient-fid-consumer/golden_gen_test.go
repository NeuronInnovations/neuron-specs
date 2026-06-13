package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
)

// TestGenerateRIDTrackGolden regenerates testdata/rid-track-golden.json — the
// CURRENT buildSapientTagged bytes for the first RID bridge sample with fixed
// inputs. Run only when SAPIENT_WRITE_GOLDEN=1; the rid byte-identity
// regression test consumes the file.
func TestGenerateRIDTrackGolden(t *testing.T) {
	if os.Getenv("SAPIENT_WRITE_GOLDEN") != "1" {
		t.Skip("set SAPIENT_WRITE_GOLDEN=1 to regenerate")
	}
	msg := loadSamples(t)[0]
	msg.NodeId = proto.String("node-restamped")
	cotOpts, err := cotOptionsFor("friendly")
	require.NoError(t, err)
	agent := &trackAgent{
		AgentID: "1", SellerEVM: "0xSELLER", PeerID: "16Uiu2PEER",
		NodeID: "node-restamped", Service: "rid",
		Protocol: sapient.ProtocolDetection, Simulated: true,
	}
	blob, err := buildSapientTagged(msg, time.Unix(1764668365, 0).UTC(), cotOpts, true, agent, "synthetic")
	require.NoError(t, err)
	// Persist the INNER track frame bytes (the envelope carries a receivedAt
	// that the regression test fixes the same way).
	var pretty json.RawMessage = blob
	require.NoError(t, os.WriteFile("testdata/rid-track-golden.json", pretty, 0o644))
	t.Logf("golden written: %d bytes", len(blob))
}
