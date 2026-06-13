package edgeapp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

func TestBuildProfileDescriptor_DefaultsToProfileE(t *testing.T) {
	d := BuildProfileDescriptor("0xabc", "12D3KooWPeer", "/neuron/edge-feed/1.0.0", 0)
	assert.Equal(t, DefaultEdgeProfileID, d.NeuronProfileID)
	assert.Equal(t, 1, d.Version, "version 0 should normalize to 1")
	assert.Equal(t, "0xabc", d.Identity.EVMAddress)
	assert.Equal(t, "12D3KooWPeer", d.Identity.PeerID)
	assert.NotEmpty(t, d.IssuedAt)
	assert.Len(t, d.Transports, 1)
	assert.Equal(t, "T-QUIC", d.Transports[0].Binding)
	// Per HCS size budget, BuildProfileDescriptor omits Capabilities and
	// Protocols by default. Callers serializing for non-HCS publishers
	// (HTTPS, IPFS) can assign these manually.
	assert.Empty(t, d.Capabilities)
	assert.Empty(t, d.Protocols)

	// EdgeProfileECapabilities() still returns the full vector for callers
	// that need it (validators, off-line snapshots).
	caps := EdgeProfileECapabilities()
	assert.Equal(t, "topic", caps["control-plane"])
	assert.Equal(t, "outbound-dial-only", caps["nat-traversal"])
	assert.Equal(t, "outbound-dial-only", caps["listen-capability"])
	assert.Equal(t, "persistent", caps["identity-lifetime"])
	assert.Equal(t, "transport+payload-ecies", caps["confidentiality"])
}

func TestProfileDescriptor_MarshalIsCanonicalJSON(t *testing.T) {
	d := BuildProfileDescriptor("0xabc", "p1", "/proto/1", 1)
	b1, err := d.Marshal()
	require.NoError(t, err)
	b2, err := d.Marshal()
	require.NoError(t, err)
	assert.Equal(t, b1, b2, "marshal must be deterministic across calls")

	// Round-trip back to map. With the trimmed defaults Capabilities is
	// not in the JSON; confirm the expected fields are.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(b1, &raw))
	assert.Equal(t, DefaultEdgeProfileID, raw["neuronProfileId"])
	assert.Equal(t, float64(1), raw["version"])
	idMap, ok := raw["identity"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "0xabc", idMap["evmAddress"])
	// `capabilities` and `protocols` are omitted by default to fit HCS budget.
	_, hasCaps := raw["capabilities"]
	assert.False(t, hasCaps, "Capabilities should be omitted in HCS-publish defaults")
}

func TestProfileDescriptor_HashStableAcrossIssuedAt(t *testing.T) {
	d1 := BuildProfileDescriptor("0xabc", "p1", "/proto/1", 1)
	// Force a different IssuedAt:
	d2 := d1
	d2.IssuedAt = "2099-12-31T23:59:59Z"

	h1, err := d1.Hash()
	require.NoError(t, err)
	h2, err := d2.Hash()
	require.NoError(t, err)
	assert.Equal(t, h1, h2, "Hash must exclude IssuedAt — descriptors with same content should hash equal")
}

func TestProfileDescriptor_HashChangesOnContentChange(t *testing.T) {
	d1 := BuildProfileDescriptor("0xabc", "p1", "/proto/1", 1)
	d2 := BuildProfileDescriptor("0xdef", "p2", "/proto/1", 1) // different identity

	h1, _ := d1.Hash()
	h2, _ := d2.Hash()
	assert.NotEqual(t, h1, h2, "different identity must yield different hash")

	// Version bump also changes the hash.
	d3 := BuildProfileDescriptor("0xabc", "p1", "/proto/1", 2)
	h3, _ := d3.Hash()
	assert.NotEqual(t, h1, h3)
}

func TestProfileDescriptor_MarshalRejectsInvalid(t *testing.T) {
	cases := map[string]ProfileDescriptor{
		"empty profileID": {Version: 1, Identity: ProfileIdentity{EVMAddress: "0xa"}},
		"version zero":    {NeuronProfileID: "x/1", Identity: ProfileIdentity{EVMAddress: "0xa"}},
		"missing EVM":     {NeuronProfileID: "x/1", Version: 1},
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := d.Marshal()
			assert.Error(t, err)
		})
	}
}

func TestPublishProfileDescriptor_PublishesSignedTopicMessage(t *testing.T) {
	bus := NewMemoryBus()
	out, err := bus.CreateTopic(topic.CreateTopicOpts{
		Transport: bus.SupportedTransport(),
		Memo:      "test-stdOut",
	})
	require.NoError(t, err)

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := key.PublicKey()

	d := BuildProfileDescriptor(pub.EVMAddress().Hex(), "p1", "/proto/1", 1)
	hash, err := PublishProfileDescriptor(bus, &key, out, d)
	require.NoError(t, err)
	assert.Len(t, hash, 64, "hash should be 32-byte sha256 in hex")

	// Verify a message landed on the topic and validates as TopicMessage.
	msgs := bus.GetMessages(out)
	require.Len(t, msgs, 1)
	require.NoError(t, topic.ValidateTopicMessage(msgs[0]))

	// Payload should round-trip into ProfileDescriptor.
	var parsed ProfileDescriptor
	require.NoError(t, json.Unmarshal(msgs[0].Payload(), &parsed))
	assert.Equal(t, DefaultEdgeProfileID, parsed.NeuronProfileID)
	assert.Equal(t, pub.EVMAddress().Hex(), parsed.Identity.EVMAddress)
}

func TestEnsurePublishedDescriptor_PublishOnlyOnHashChange(t *testing.T) {
	bus := NewMemoryBus()
	out, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport()})
	require.NoError(t, err)
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := key.PublicKey()

	state := &EdgeState{} // empty hash ⇒ first publish must fire
	d := BuildProfileDescriptor(pub.EVMAddress().Hex(), "p1", "/proto/1", 1)

	hash1, published1, err := EnsurePublishedDescriptor(bus, &key, out, d, state)
	require.NoError(t, err)
	assert.True(t, published1, "first call with empty state hash must publish")
	assert.Equal(t, hash1, state.ProfileDescriptorHash)
	assert.Len(t, bus.GetMessages(out), 1)

	// Second call with same descriptor + state-recorded hash ⇒ skip publish.
	hash2, published2, err := EnsurePublishedDescriptor(bus, &key, out, d, state)
	require.NoError(t, err)
	assert.False(t, published2, "matching state hash must skip publish")
	assert.Equal(t, hash1, hash2)
	assert.Len(t, bus.GetMessages(out), 1, "no second message published")

	// Bump descriptor version ⇒ new hash ⇒ publish fires again.
	d2 := BuildProfileDescriptor(pub.EVMAddress().Hex(), "p1", "/proto/1", 2)
	hash3, published3, err := EnsurePublishedDescriptor(bus, &key, out, d2, state)
	require.NoError(t, err)
	assert.True(t, published3)
	assert.NotEqual(t, hash1, hash3)
	assert.Len(t, bus.GetMessages(out), 2)
}

func TestEnsurePublishedDescriptor_NilStateAlwaysPublishes(t *testing.T) {
	bus := NewMemoryBus()
	out, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport()})
	require.NoError(t, err)
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := key.PublicKey()

	d := BuildProfileDescriptor(pub.EVMAddress().Hex(), "p1", "/proto/1", 1)
	_, published, err := EnsurePublishedDescriptor(bus, &key, out, d, nil)
	require.NoError(t, err)
	assert.True(t, published)
}
