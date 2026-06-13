package sapient

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// buildCardJSON builds a real card via BuildSellerCard and returns its JSON —
// ParseCardMeta is tested against the genuine card shape, not a hand-rolled one.
func buildCardJSON(t *testing.T, prof *SellerProfile) json.RawMessage {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	card, err := BuildSellerCard(SellerCardOptions{ChildKey: &k, Profile: prof})
	require.NoError(t, err)
	j, err := card.AgentURI.ToJSON()
	require.NoError(t, err)
	return json.RawMessage(j)
}

func TestParseCardMeta_RIDCard(t *testing.T) {
	t.Parallel()
	meta := ParseCardMeta(buildCardJSON(t, nil))
	assert.Equal(t, "neuron.rid/1", meta.ExtensionID)
	assert.Equal(t, "rid", meta.Modality)
	assert.Nil(t, meta.Capabilities, "RID card advertises no capabilities key")
	assert.Equal(t, DefaultSensorModels, meta.SensorModels)
	assert.Equal(t, SapientWire, meta.Wire)
	assert.NotEmpty(t, meta.NodeID)
	assert.Equal(t, CommerceServiceName, meta.CommerceService)
	assert.Equal(t, "none", meta.CommerceBinding, "advertisement-only posture")
}

func TestParseCardMeta_JetVisionCard(t *testing.T) {
	t.Parallel()
	prof := JetVisionProfile()
	meta := ParseCardMeta(buildCardJSON(t, &prof))
	assert.Equal(t, "neuron.adsb/1", meta.ExtensionID)
	assert.Equal(t, "adsb", meta.Modality)
	assert.Equal(t, JVCapabilities, meta.Capabilities)
	assert.Equal(t, JVSensorModels, meta.SensorModels)
	assert.Equal(t, JVCommerceServiceName, meta.CommerceService)
	assert.Equal(t, "none", meta.CommerceBinding)
}

func TestParseCardMeta_UnknownFutureExtension(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"services":[
		{"type":"neuron-topic","channel":"stdOut","config":{"neuron.future/9":{"nodeId":"n-1","wire":"w","capabilities":["x.y"],"sensorModels":["Z 1000"]}}},
		{"type":"neuron-commerce","name":"future-svc","settlement":{"binding":"evm-escrow"}}
	]}`)
	meta := ParseCardMeta(raw)
	assert.Equal(t, "neuron.future/9", meta.ExtensionID)
	assert.Equal(t, "future", meta.Modality)
	assert.Equal(t, []string{"x.y"}, meta.Capabilities)
	assert.Equal(t, []string{"Z 1000"}, meta.SensorModels)
	assert.Equal(t, "future-svc", meta.CommerceService)
	assert.Equal(t, "evm-escrow", meta.CommerceBinding)
}

func TestParseCardMeta_ToleratesGarbageAndAbsence(t *testing.T) {
	t.Parallel()
	assert.Equal(t, CardMeta{}, ParseCardMeta(nil))
	assert.Equal(t, CardMeta{}, ParseCardMeta(json.RawMessage(`not json`)))
	assert.Equal(t, CardMeta{}, ParseCardMeta(json.RawMessage(`{"services":[]}`)))
	// stdOut topic without a neuron.* key → no extension, no panic.
	meta := ParseCardMeta(json.RawMessage(`{"services":[{"type":"neuron-topic","channel":"stdOut","config":{"other":1}}]}`))
	assert.Empty(t, meta.ExtensionID)
}
