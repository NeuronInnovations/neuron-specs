package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/edgeapp"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// TestResolveTopicsFromState_LegacyMode confirms that an empty STATE_PATH
// preserves Phase C.2 behavior: three fresh CreateTopic calls per
// invocation, no state object returned.
func TestResolveTopicsFromState_LegacyMode(t *testing.T) {
	bus := edgeapp.NewMemoryBus()
	priv, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	in1, out1, err1, st, fresh, e := resolveTopicsFromState(bus, "", &priv)
	require.NoError(t, e)
	require.Nil(t, st)
	require.True(t, fresh)

	in2, out2, err2, st2, fresh2, e := resolveTopicsFromState(bus, "", &priv)
	require.NoError(t, e)
	require.Nil(t, st2)
	require.True(t, fresh2)

	assert.NotEqual(t, in1.Locator(), in2.Locator(), "stateless mode must create fresh topics")
	assert.NotEqual(t, out1.Locator(), out2.Locator())
	assert.NotEqual(t, err1.Locator(), err2.Locator())
}

// TestResolveTopicsFromState_PersistsAndReuses confirms the canonical D1
// restart behavior: STATE_PATH set + same identity ⇒ identical topic
// locators across two calls, no new bus.CreateTopic invocations on the
// second call.
func TestResolveTopicsFromState_PersistsAndReuses(t *testing.T) {
	bus := edgeapp.NewMemoryBus()
	priv, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// First run: must create + return a state object the caller persists.
	in1, out1, err1, st1, fresh1, e := resolveTopicsFromState(bus, path, &priv)
	require.NoError(t, e)
	require.True(t, fresh1)
	require.NotNil(t, st1, "first run must return state to persist")
	assert.Equal(t, priv.PublicKey().EVMAddress().Hex(), st1.Identity.EVMAddress)
	require.NoError(t, edgeapp.SaveEdgeState(path, st1))

	// Second run: same path, same identity ⇒ persisted refs returned.
	in2, out2, err2, st2, fresh2, e := resolveTopicsFromState(bus, path, &priv)
	require.NoError(t, e)
	assert.False(t, fresh2, "second run must reuse persisted topics")
	require.NotNil(t, st2)

	assert.Equal(t, in1.Locator(), in2.Locator())
	assert.Equal(t, out1.Locator(), out2.Locator())
	assert.Equal(t, err1.Locator(), err2.Locator())
}

// TestResolveTopicsFromState_KeyRotationCreatesFresh confirms the identity
// guard: a different key ⇒ persisted state ignored, fresh topics created.
func TestResolveTopicsFromState_KeyRotationCreatesFresh(t *testing.T) {
	bus := edgeapp.NewMemoryBus()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// First run with key A.
	keyA, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	inA, _, _, stA, _, e := resolveTopicsFromState(bus, path, &keyA)
	require.NoError(t, e)
	require.NoError(t, edgeapp.SaveEdgeState(path, stA))

	// Second run with key B — different identity. Should NOT reuse A's topics.
	keyB, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	inB, _, _, stB, freshB, e := resolveTopicsFromState(bus, path, &keyB)
	require.NoError(t, e)
	assert.True(t, freshB, "key rotation must trigger fresh-create")
	require.NotNil(t, stB)
	assert.Equal(t, keyB.PublicKey().EVMAddress().Hex(), stB.Identity.EVMAddress,
		"new state object must record the rotated identity")
	assert.NotEqual(t, inA.Locator(), inB.Locator(),
		"key rotation must yield different topics — old topics belong to old key")
}
