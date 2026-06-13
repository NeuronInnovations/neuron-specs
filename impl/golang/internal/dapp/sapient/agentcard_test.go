package sapient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// newCardTestKey returns a fresh secp256k1 key for card/registry tests.
func newCardTestKey(t *testing.T) *keylib.NeuronPrivateKey {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	return &k
}

// topicByChannel finds the NeuronTopicService for a standard channel.
func topicByChannel(t *testing.T, card SellerCard, channel string) registry.NeuronTopicService {
	t.Helper()
	for _, svc := range card.AgentURI.TopicServices() {
		if svc.Channel == channel {
			return svc
		}
	}
	t.Fatalf("no topic service for channel %q", channel)
	return registry.NeuronTopicService{}
}

func TestBuildSellerCard_PassesRegistrationValidator(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)

	valid, vErrs := registry.ValidateRegistrationCompleteness(card.AgentURI, key.PublicKey())
	require.True(t, valid, "card must satisfy V-REG-01..13: %v", vErrs)
}

func TestBuildSellerCard_ServiceAndProtocol(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)

	// The "rid" commerce service (advertisement-only).
	commerce := card.AgentURI.CommerceServices()
	require.Len(t, commerce, 1)
	assert.Equal(t, "rid", commerce[0].Name)
	assert.Equal(t, P2PServiceName, commerce[0].Delivery.ServiceRef, "commerce delivers via the detection p2p service")
	assert.Equal(t, "none", commerce[0].Settlement.Binding, "advertisement-only: no payment binding")
	assert.Equal(t, "0", commerce[0].Pricing.Amount, "advertisement-only: price 0")
	assert.Equal(t, ExtensionID, commerce[0].TermsRef)

	// The /sapient/detection/2.0.0 p2p service.
	p2p := card.AgentURI.P2PServices()
	require.Len(t, p2p, 1)
	assert.Equal(t, ProtocolDetection, p2p[0].Protocol, "FR-S11 protocol")
	assert.Equal(t, TopicNameStdOut, p2p[0].TopicRef)
}

func TestBuildSellerCard_IdentityBinding(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)
	pub := key.PublicKey()
	wantPeerID, err := pub.PeerID()
	require.NoError(t, err)

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)

	// V-REG-12: p2p peerID == key PeerID; also surfaced on the SellerCard.
	assert.Equal(t, wantPeerID.String(), card.AgentURI.P2PServices()[0].PeerID)
	assert.Equal(t, wantPeerID.String(), card.PeerID)

	// FR-S94: node_id is identity-bound (derived from the EVM address).
	wantNodeID := NodeIDFromIdentity(pub.EVMAddress().Hex())
	assert.Equal(t, wantNodeID, card.NodeID)
	assert.NotEqual(t, PlaceholderNodeID, card.NodeID, "never the bridge placeholder")

	// V-REG-06: DID endpoint == key DIDKey.
	did := card.AgentURI.DIDServices()
	require.Len(t, did, 1)
	assert.Equal(t, pub.DIDKey(), did[0].Endpoint)
}

func TestBuildSellerCard_RidExtension(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)
	pub := key.PublicKey()

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)

	stdOut := topicByChannel(t, card, "stdOut")
	raw, ok := stdOut.Config[ExtensionID]
	require.True(t, ok, "stdOut config carries the %s extension", ExtensionID)
	ext, ok := raw.(map[string]any)
	require.True(t, ok, "extension is a JSON object")

	assert.Equal(t, NodeIDFromIdentity(pub.EVMAddress().Hex()), ext["nodeId"])
	assert.Equal(t, SapientWire, ext["wire"])
	assert.Equal(t, SchemaURL, ext["schema"])
	assert.Equal(t, SchemaSha256(), ext["schemaSha256"])
	assert.Len(t, ext["schemaSha256"], 64, "sha256 hex is 64 chars")

	models, ok := ext["sensorModels"].([]string)
	require.True(t, ok, "sensorModels is a []string")
	assert.Equal(t, DefaultSensorModels, models)

	// Other channels carry no extension.
	assert.NotContains(t, topicByChannel(t, card, "stdIn").Config, ExtensionID)
	assert.NotContains(t, topicByChannel(t, card, "stdErr").Config, ExtensionID)
}

func TestBuildSellerCard_AuditLaneTransportSwitch(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)

	// File mode (default): transport stays auditlane-file, no topic IDs advertised
	// — the card must not overclaim a real ledger lane when running on a file.
	fileCard, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)
	for _, ch := range []string{"stdIn", "stdOut", "stdErr"} {
		svc := topicByChannel(t, fileCard, ch)
		assert.Equal(t, DefaultTopicTransport, svc.Transport, "%s transport (file mode)", ch)
		assert.NotContains(t, svc.Config, "topicId", "%s file mode must not advertise a topic id", ch)
	}

	// HCS mode: transport flips to hcs and each channel carries its real topic id.
	hcsCard, err := BuildSellerCard(SellerCardOptions{
		ChildKey:       key,
		TopicTransport: "hcs",
		TopicConfig: map[string]map[string]any{
			"stdIn":  {"topicId": "0.0.1001"},
			"stdOut": {"topicId": "0.0.1002"},
			"stdErr": {"topicId": "0.0.1003"},
		},
	})
	require.NoError(t, err)
	for ch, want := range map[string]string{"stdIn": "0.0.1001", "stdOut": "0.0.1002", "stdErr": "0.0.1003"} {
		svc := topicByChannel(t, hcsCard, ch)
		assert.Equal(t, "hcs", svc.Transport, "%s transport flips to hcs", ch)
		assert.Equal(t, want, svc.Config["topicId"], "%s topic id advertised", ch)
	}
	// Service names + the rid extension survive the switch (stdOut keeps both).
	assert.Equal(t, TopicNameStdOut, topicByChannel(t, hcsCard, "stdOut").Name)
	assert.Contains(t, topicByChannel(t, hcsCard, "stdOut").Config, ExtensionID)
}

func TestBuildSellerCard_SensorModelsOverride(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)

	card, err := BuildSellerCard(SellerCardOptions{
		ChildKey:     key,
		SensorModels: []string{"DroneScout DS240"},
	})
	require.NoError(t, err)

	ext := topicByChannel(t, card, "stdOut").Config[ExtensionID].(map[string]any)
	assert.Equal(t, []string{"DroneScout DS240"}, ext["sensorModels"])
}

func TestBuildSellerCard_DeterministicJSON(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)

	c1, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)
	c2, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)

	j1, err := c1.AgentURI.ToJSON()
	require.NoError(t, err)
	j2, err := c2.AgentURI.ToJSON()
	require.NoError(t, err)
	assert.Equal(t, j1, j2, "same key → byte-identical card (idempotent RegisterOrUpdate relies on this)")
}

func TestBuildSellerCard_JSONRoundTripPreservesExtension(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)

	jsonStr, err := card.AgentURI.ToJSON()
	require.NoError(t, err)
	parsed, err := registry.AgentURIFromJSON(jsonStr)
	require.NoError(t, err)

	var stdOut registry.NeuronTopicService
	for _, svc := range parsed.TopicServices() {
		if svc.Channel == "stdOut" {
			stdOut = svc
		}
	}
	ext, ok := stdOut.Config[ExtensionID].(map[string]any)
	require.True(t, ok, "extension survives JSON round-trip")
	assert.Equal(t, NodeIDFromIdentity(key.PublicKey().EVMAddress().Hex()), ext["nodeId"])
}

func TestBuildSellerCard_RejectsNilKey(t *testing.T) {
	t.Parallel()
	_, err := BuildSellerCard(SellerCardOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ChildKey is required")
}

func TestSchemaSha256_StableAndNonEmpty(t *testing.T) {
	t.Parallel()
	h := SchemaSha256()
	assert.Len(t, h, 64)
	assert.Equal(t, h, SchemaSha256(), "stable across calls")
}

func TestBuildSellerCard_StreamCatalog(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)

	require.Len(t, card.Streams, 1, "Phase-1 detection-only catalog")
	assert.Equal(t, ProtocolDetection, card.Streams[0].ProtocolID)
}
