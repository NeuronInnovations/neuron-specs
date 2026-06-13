package payment

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FR-P33a: ConnectionSetup with streams[] catalog.
// Canonical order: type → version → requestId → peerID → encryptedMultiaddrs → protocol* → streams* → natStatus*.
// At least one of protocol or streams MUST be present per FR-P33.
func TestConnectionSetup_LegacyProtocolOnly(t *testing.T) {
	t.Parallel()

	cs := ConnectionSetup{
		Type:                PayloadConnectionSetup,
		Version:             "1.0.0",
		RequestID:           "req-1",
		PeerID:              "12D3Koo",
		EncryptedMultiaddrs: "AAAA",
		Protocol:            "/neuron/adsb/1.0.0",
	}

	got, err := json.Marshal(cs)
	require.NoError(t, err)

	const want = `{"type":"connectionSetup","version":"1.0.0","requestId":"req-1","peerID":"12D3Koo","encryptedMultiaddrs":"AAAA","protocol":"/neuron/adsb/1.0.0"}`
	assert.Equal(t, want, string(got))
}

func TestConnectionSetup_StreamsCatalogOnly(t *testing.T) {
	t.Parallel()

	cs := ConnectionSetup{
		Type:                PayloadConnectionSetup,
		Version:             "1.0.0",
		RequestID:           "req-1",
		PeerID:              "12D3Koo",
		EncryptedMultiaddrs: "AAAA",
		Streams: []StreamCatalogEntry{
			{Name: "raw", ProtocolID: "/jetvision/raw/1.0.0", Direction: StreamDirectionSeller},
			{Name: "filtered", ProtocolID: "/jetvision/filtered/*", Direction: StreamDirectionSeller},
			{Name: "status", ProtocolID: "/jetvision/status/1.0.0", Direction: StreamDirectionBuyer},
		},
	}

	got, err := json.Marshal(cs)
	require.NoError(t, err)

	const want = `{"type":"connectionSetup","version":"1.0.0","requestId":"req-1","peerID":"12D3Koo","encryptedMultiaddrs":"AAAA","streams":[{"name":"raw","protocolID":"/jetvision/raw/1.0.0","direction":"seller-initiates"},{"name":"filtered","protocolID":"/jetvision/filtered/*","direction":"seller-initiates"},{"name":"status","protocolID":"/jetvision/status/1.0.0","direction":"buyer-initiates"}]}`
	assert.Equal(t, want, string(got))
}

func TestConnectionSetup_BothProtocolAndStreams(t *testing.T) {
	t.Parallel()

	cs := ConnectionSetup{
		Type:                PayloadConnectionSetup,
		Version:             "1.0.0",
		RequestID:           "req-1",
		PeerID:              "12D3Koo",
		EncryptedMultiaddrs: "AAAA",
		Protocol:            "/jetvision/raw/1.0.0",
		Streams: []StreamCatalogEntry{
			{Name: "raw", ProtocolID: "/jetvision/raw/1.0.0", Direction: StreamDirectionSeller},
		},
		NATStatus: "public",
	}

	got, err := json.Marshal(cs)
	require.NoError(t, err)

	const want = `{"type":"connectionSetup","version":"1.0.0","requestId":"req-1","peerID":"12D3Koo","encryptedMultiaddrs":"AAAA","protocol":"/jetvision/raw/1.0.0","streams":[{"name":"raw","protocolID":"/jetvision/raw/1.0.0","direction":"seller-initiates"}],"natStatus":"public"}`
	assert.Equal(t, want, string(got))
}

func TestConnectionSetup_RoundTripWithStreams(t *testing.T) {
	t.Parallel()

	original := ConnectionSetup{
		Type:                PayloadConnectionSetup,
		Version:             "1.0.0",
		RequestID:           "req-1",
		PeerID:              "12D3Koo",
		EncryptedMultiaddrs: "AAAA",
		Streams: []StreamCatalogEntry{
			{Name: "raw", ProtocolID: "/jetvision/raw/1.0.0", Direction: StreamDirectionSeller, Schema: "https://specs/adsb-raw.md"},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ConnectionSetup
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestStreamCatalogEntry_SchemaOptional(t *testing.T) {
	t.Parallel()

	entry := StreamCatalogEntry{
		Name:       "raw",
		ProtocolID: "/jetvision/raw/1.0.0",
		Direction:  StreamDirectionSeller,
	}
	got, err := json.Marshal(entry)
	require.NoError(t, err)
	const want = `{"name":"raw","protocolID":"/jetvision/raw/1.0.0","direction":"seller-initiates"}`
	assert.Equal(t, want, string(got))
}

func TestStreamCatalogEntry_WithSchema(t *testing.T) {
	t.Parallel()

	entry := StreamCatalogEntry{
		Name:       "raw",
		ProtocolID: "/jetvision/raw/1.0.0",
		Direction:  StreamDirectionSeller,
		Schema:     "https://specs.neuron.network/dapp/adsb/v1/raw-frame.md",
	}
	got, err := json.Marshal(entry)
	require.NoError(t, err)
	const want = `{"name":"raw","protocolID":"/jetvision/raw/1.0.0","direction":"seller-initiates","schema":"https://specs.neuron.network/dapp/adsb/v1/raw-frame.md"}`
	assert.Equal(t, want, string(got))
}
