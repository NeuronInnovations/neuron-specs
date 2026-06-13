package sapient

import (
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// fixedProfileTestKey is the deterministic key behind testdata/rid-card-golden.json.
func fixedProfileTestKey(t *testing.T) *keylib.NeuronPrivateKey {
	t.Helper()
	k, err := keylib.NeuronPrivateKeyFromHex("1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(t, err)
	return &k
}

func TestRIDProfile_MatchesPackageConstants(t *testing.T) {
	t.Parallel()
	p := RIDProfile()
	assert.Equal(t, CommerceServiceName, p.CommerceServiceName)
	assert.Equal(t, ExtensionID, p.ExtensionID)
	assert.Equal(t, DefaultAnchor, p.Anchor)
	assert.Equal(t, DefaultSensorModels, p.SensorModels)
	assert.Nil(t, p.Capabilities, "RID profile advertises no capabilities key (byte-compat)")
	assert.Equal(t, ProfileSAPIENTRID, p.HeartbeatProfile)
}

func TestJetVisionProfile_Values(t *testing.T) {
	t.Parallel()
	p := JetVisionProfile()
	assert.Equal(t, "jetvision-adsb-sapient", p.CommerceServiceName)
	assert.Equal(t, "neuron.adsb/1", p.ExtensionID)
	assert.Equal(t, "sapient-adsb-r1", p.Anchor)
	assert.Equal(t, []string{"JetVision Air!Squitter"}, p.SensorModels)
	assert.Equal(t, []string{
		"sapient.bsi-flex-335-v2.0",
		"sapient.detection-report",
		"neuron.adsb/1",
		"jetvision.air-squitter.aircraftlist",
	}, p.Capabilities)
	assert.Equal(t, ProfileSAPIENTADSB, p.HeartbeatProfile)
}

// TestBuildSellerCard_NilProfileByteIdenticalToRID is the live-path regression
// gate: a nil Profile must reproduce the pre-SellerProfile card byte-for-byte
// (the AgentURI SHA-256 drives idempotent RegisterOrUpdate on the staging seller).
func TestBuildSellerCard_NilProfileByteIdenticalToRID(t *testing.T) {
	t.Parallel()
	key := fixedProfileTestKey(t)

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)
	j, err := card.AgentURI.ToJSON()
	require.NoError(t, err)

	golden, err := os.ReadFile("testdata/rid-card-golden.json")
	require.NoError(t, err)
	require.Equal(t, string(golden), j, "nil-profile card must stay byte-identical to the pre-profile golden")
}

// TestBuildSellerCard_ExplicitRIDProfileByteIdentical: passing RIDProfile()
// explicitly must also reproduce the golden (nil and RID are the same posture).
func TestBuildSellerCard_ExplicitRIDProfileByteIdentical(t *testing.T) {
	t.Parallel()
	key := fixedProfileTestKey(t)
	prof := RIDProfile()

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key, Profile: &prof})
	require.NoError(t, err)
	j, err := card.AgentURI.ToJSON()
	require.NoError(t, err)

	golden, err := os.ReadFile("testdata/rid-card-golden.json")
	require.NoError(t, err)
	require.Equal(t, string(golden), j)
}

func TestBuildSellerCard_JetVisionProfile(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)
	prof := JetVisionProfile()

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key, Profile: &prof})
	require.NoError(t, err)

	valid, vErrs := registry.ValidateRegistrationCompleteness(card.AgentURI, key.PublicKey())
	require.True(t, valid, "JV card must satisfy V-REG-01..13: %v", vErrs)

	// Commerce service is named for the JV service and termsRef points at the
	// JV extension id.
	commerce := card.AgentURI.CommerceServices()
	require.Len(t, commerce, 1)
	assert.Equal(t, "jetvision-adsb-sapient", commerce[0].Name)
	assert.Equal(t, "neuron.adsb/1", commerce[0].TermsRef)

	// The capability extension lives under neuron.adsb/1 in the stdOut config,
	// carrying the JV capability vocabulary and sensor model.
	stdOut := topicByChannel(t, card, "stdOut")
	assert.Equal(t, "sapient-adsb-r1", stdOut.Anchor)
	ext, ok := stdOut.Config["neuron.adsb/1"].(map[string]any)
	require.True(t, ok, "stdOut config must carry the neuron.adsb/1 extension")
	assert.Equal(t, card.NodeID, ext["nodeId"])
	assert.Equal(t, SapientWire, ext["wire"])
	assert.Equal(t, SchemaSha256(), ext["schemaSha256"])
	assert.Equal(t, []string{"JetVision Air!Squitter"}, ext["sensorModels"])
	assert.Equal(t, []string{
		"sapient.bsi-flex-335-v2.0",
		"sapient.detection-report",
		"neuron.adsb/1",
		"jetvision.air-squitter.aircraftlist",
	}, ext["capabilities"])

	// No rid extension anywhere on a JV card.
	_, hasRid := stdOut.Config[ExtensionID]
	assert.False(t, hasRid, "JV card must not carry neuron.rid/1")
}

func TestBuildSellerCard_JetVisionDeterministicJSON(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)
	prof := JetVisionProfile()

	c1, err := BuildSellerCard(SellerCardOptions{ChildKey: key, Profile: &prof})
	require.NoError(t, err)
	c2, err := BuildSellerCard(SellerCardOptions{ChildKey: key, Profile: &prof})
	require.NoError(t, err)

	j1, err := c1.AgentURI.ToJSON()
	require.NoError(t, err)
	j2, err := c2.AgentURI.ToJSON()
	require.NoError(t, err)
	assert.Equal(t, j1, j2, "same key + profile → byte-identical card (idempotent RegisterOrUpdate)")
}

func TestEvidenceFromResultProfile_ServiceName(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)
	res := RegisterResult{
		SellerEVM: key.PublicKey().EVMAddress(),
		NodeID:    "node-1",
		PeerID:    "peer-1",
		TokenID:   big.NewInt(7),
	}

	jv := EvidenceFromResultProfile(res, true, "jetvision-adsb-sapient")
	assert.Equal(t, "jetvision-adsb-sapient", jv.Service)
	assert.Equal(t, "7", jv.AgentID)
	assert.True(t, jv.Simulated)

	// The legacy helper keeps the rid service name (live-path behavior).
	rid := EvidenceFromResult(res, false)
	assert.Equal(t, CommerceServiceName, rid.Service)
	assert.False(t, rid.Simulated)
}

func TestHeartbeatCapabilities_ProfileDefaultAndOverride(t *testing.T) {
	t.Parallel()
	base := HeartbeatOptions{
		SellerEVM:    "0xseller",
		SellerPeerID: "peer",
		ASMNodeID:    "node",
	}

	caps, _ := sapientCapabilities(base, "rid")
	assert.Equal(t, ProfileSAPIENTRID, string(caps.Profile), "empty Profile defaults to sapient-rid")

	jv := base
	jv.Profile = ProfileSAPIENTADSB
	caps, _ = sapientCapabilities(jv, "jetvision-adsb-sapient")
	assert.Equal(t, ProfileSAPIENTADSB, string(caps.Profile))
	assert.Equal(t, "jetvision-adsb-sapient", caps.Operational.ServiceName)
}
