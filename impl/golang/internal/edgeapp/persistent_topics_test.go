package edgeapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolvePersistentTopics_LegacyMode confirms the empty-StatePath path
// is byte-identical to the Phase C.2 behavior: three fresh CreateTopic
// calls per invocation, no state object returned, fresh=true.
func TestResolvePersistentTopics_LegacyMode(t *testing.T) {
	bus := NewMemoryBus()

	in1, out1, err1, state, fresh, e := resolvePersistentTopics(
		bus, "", "0xabc", "0a", "p1", "test-agent")
	require.NoError(t, e)
	require.Nil(t, state, "stateless mode must not return a state object")
	require.True(t, fresh)

	// Second call with the same args returns DIFFERENT locators — no caching.
	in2, out2, err2, state2, fresh2, e := resolvePersistentTopics(
		bus, "", "0xabc", "0a", "p1", "test-agent")
	require.NoError(t, e)
	require.Nil(t, state2)
	require.True(t, fresh2)

	assert.NotEqual(t, in1.Locator(), in2.Locator(), "stateless mode must create fresh topics each call")
	assert.NotEqual(t, out1.Locator(), out2.Locator())
	assert.NotEqual(t, err1.Locator(), err2.Locator())
}

// TestResolvePersistentTopics_FirstRunCreatesAndPersists confirms that the
// first call with a non-empty StatePath but no existing file creates fresh
// topics AND returns a *EdgeState the caller can save.
func TestResolvePersistentTopics_FirstRunCreatesAndPersists(t *testing.T) {
	bus := NewMemoryBus()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	in, out, errRef, state, fresh, err := resolvePersistentTopics(
		bus, path, "0xabc", "0aBcDeF", "peer-id-1", "test-agent")
	require.NoError(t, err)
	require.True(t, fresh, "first run with no state file ⇒ fresh=true")
	require.NotNil(t, state, "first run must return a state object so caller can persist")

	assert.Equal(t, "0xabc", state.Identity.EVMAddress)
	assert.Equal(t, "0aBcDeF", state.Identity.PublicKeyHex)
	assert.Equal(t, in.Locator(), state.Topics.StdInLocator)
	assert.Equal(t, out.Locator(), state.Topics.StdOutLocator)
	assert.Equal(t, errRef.Locator(), state.Topics.StdErrLocator)
}

// TestResolvePersistentTopics_SecondRunReusesTopics confirms the canonical
// restart story: same StatePath + same identity ⇒ identical topic locators
// across two calls, no new bus.CreateTopic invocations.
func TestResolvePersistentTopics_SecondRunReusesTopics(t *testing.T) {
	bus := NewMemoryBus()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// First run: create + persist.
	in1, out1, err1, state1, fresh1, err := resolvePersistentTopics(
		bus, path, "0xabc", "0aBcDeF", "peer-id-1", "test-agent")
	require.NoError(t, err)
	require.True(t, fresh1)
	require.NotNil(t, state1)
	require.NoError(t, SaveEdgeState(path, state1))

	// Capture how many topics the bus has after run 1.
	bus.mu.Lock()
	beforeCount := len(bus.topics)
	bus.mu.Unlock()

	// Second run: same path, same identity. Must return the SAME locators
	// and must NOT have asked the bus for any new topics.
	in2, out2, err2, state2, fresh2, err := resolvePersistentTopics(
		bus, path, "0xabc", "0aBcDeF", "peer-id-1", "test-agent")
	require.NoError(t, err)
	assert.False(t, fresh2, "second run must report fresh=false")
	require.NotNil(t, state2, "second run still returns a state object so caller can re-save UpdatedAt")

	assert.Equal(t, in1.Locator(), in2.Locator(), "stdIn locator must persist across restarts")
	assert.Equal(t, out1.Locator(), out2.Locator(), "stdOut locator must persist")
	assert.Equal(t, err1.Locator(), err2.Locator(), "stdErr locator must persist")

	bus.mu.Lock()
	afterCount := len(bus.topics)
	bus.mu.Unlock()
	assert.Equal(t, beforeCount, afterCount, "no new topics should have been created on the bus")
}

// TestResolvePersistentTopics_KeyRotationDiscardsState confirms the
// identity guard: if the running pubkey doesn't match the persisted one,
// the state file is ignored and fresh topics are created. This protects
// against accidentally reusing topics signed by a different key.
func TestResolvePersistentTopics_KeyRotationDiscardsState(t *testing.T) {
	bus := NewMemoryBus()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// First run with key "alice".
	in1, _, _, state1, _, err := resolvePersistentTopics(
		bus, path, "0xalice", "alice-pubkey-hex", "peer-alice", "test-agent")
	require.NoError(t, err)
	require.NoError(t, SaveEdgeState(path, state1))

	// Second run with a DIFFERENT key "bob". Same state file present.
	in2, _, _, state2, fresh2, err := resolvePersistentTopics(
		bus, path, "0xbob", "bob-pubkey-hex", "peer-bob", "test-agent")
	require.NoError(t, err)
	assert.True(t, fresh2, "key mismatch must trigger fresh-create")
	require.NotNil(t, state2)
	assert.Equal(t, "bob-pubkey-hex", state2.Identity.PublicKeyHex,
		"new state object must record the rotated identity")
	assert.NotEqual(t, in1.Locator(), in2.Locator(),
		"key rotation must yield different topics — old topics belong to old key")
}

// TestResolvePersistentTopics_CorruptStateSurfacesError confirms a parse
// failure is loud: we don't silently overwrite a state file the operator
// might still want to inspect.
func TestResolvePersistentTopics_CorruptStateSurfacesError(t *testing.T) {
	bus := NewMemoryBus()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	require.NoError(t, writeFile(path, "this is not json{{{"))

	_, _, _, state, _, err := resolvePersistentTopics(
		bus, path, "0xabc", "0a", "p", "test-agent")
	require.Error(t, err, "corrupt state must surface a load error, not silently overwrite")
	assert.Nil(t, state)
}

// TestResolvePersistentTopics_FutureSchemaFallsBackToFresh confirms a
// state file written by a newer binary doesn't crash an older binary —
// it's silently ignored and treated as missing.
func TestResolvePersistentTopics_FutureSchemaFallsBackToFresh(t *testing.T) {
	bus := NewMemoryBus()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write a state file claiming schema version 99.
	require.NoError(t, writeFile(path, `{"schemaVersion":99,"identity":{"evmAddress":"0xabc"}}`))

	_, _, _, state, fresh, err := resolvePersistentTopics(
		bus, path, "0xabc", "0a", "p", "test-agent")
	require.NoError(t, err)
	assert.True(t, fresh, "future schema must trigger fresh-create")
	require.NotNil(t, state, "fresh-create still returns a state object for re-persisting")
	assert.Equal(t, EdgeStateSchemaVersion, state.SchemaVersion)
}

// writeFile is a tiny helper for the test fixtures above.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
