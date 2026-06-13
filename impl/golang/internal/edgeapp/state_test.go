package edgeapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

func newTestRefs(t *testing.T) (in, out, errRef topic.TopicRef) {
	t.Helper()
	var err error
	in, err = topic.NewTopicRef(topic.BackendHCS, "0.0.1001")
	require.NoError(t, err)
	out, err = topic.NewTopicRef(topic.BackendHCS, "0.0.1002")
	require.NoError(t, err)
	errRef, err = topic.NewTopicRef(topic.BackendHCS, "0.0.1003")
	require.NoError(t, err)
	return
}

func TestEdgeState_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "absent.json")
	s, err := LoadEdgeState(path)
	require.NoError(t, err)
	assert.Nil(t, s, "missing file should return nil state without error")
}

func TestEdgeState_LoadEmptyPath(t *testing.T) {
	s, err := LoadEdgeState("")
	require.NoError(t, err)
	assert.Nil(t, s)
}

func TestEdgeState_SaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	in, out, errRef := newTestRefs(t)
	original := BuildEdgeState(
		"0xabcd000000000000000000000000000000001234",
		"04aabbccddeeff",
		"12D3KooWExamplePeerID",
		in, out, errRef,
	)
	require.NoError(t, SaveEdgeState(path, original))

	loaded, err := LoadEdgeState(path)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, EdgeStateSchemaVersion, loaded.SchemaVersion)
	assert.Equal(t, original.Identity.EVMAddress, loaded.Identity.EVMAddress)
	assert.Equal(t, original.Identity.PublicKeyHex, loaded.Identity.PublicKeyHex)
	assert.Equal(t, original.Topics.StdInLocator, loaded.Topics.StdInLocator)
	assert.Equal(t, original.Topics.StdOutLocator, loaded.Topics.StdOutLocator)
	assert.Equal(t, original.Topics.StdErrLocator, loaded.Topics.StdErrLocator)
	assert.Equal(t, string(topic.BackendHCS), loaded.Topics.BackendKind)
	assert.NotEmpty(t, loaded.UpdatedAt)
}

func TestEdgeState_SaveAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	in, out, errRef := newTestRefs(t)
	first := BuildEdgeState("0xfirst", "0a", "p1", in, out, errRef)
	require.NoError(t, SaveEdgeState(path, first))

	// Overwrite. The previous file must remain readable until the rename
	// completes — no truncated intermediate.
	in2, _ := topic.NewTopicRef(topic.BackendHCS, "0.0.9999")
	second := BuildEdgeState("0xsecond", "0b", "p2", in2, out, errRef)
	require.NoError(t, SaveEdgeState(path, second))

	loaded, err := LoadEdgeState(path)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "0xsecond", loaded.Identity.EVMAddress)
	assert.Equal(t, "0.0.9999", loaded.Topics.StdInLocator)

	// No leftover .tmp files in the directory.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp", "save left a temp artifact: %s", e.Name())
	}
}

func TestEdgeState_LoadFutureSchemaTreatedAsAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write a state file with an unknown schema version.
	bogus := struct {
		SchemaVersion int `json:"schemaVersion"`
	}{SchemaVersion: 99}
	data, err := json.Marshal(bogus)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loaded, err := LoadEdgeState(path)
	require.NoError(t, err, "future schema must not error — degrade gracefully")
	assert.Nil(t, loaded)
}

func TestEdgeState_LoadCorruptReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte("not valid json{{{"), 0o644))

	_, err := LoadEdgeState(path)
	require.Error(t, err)
}

func TestEdgeState_MatchesIdentity(t *testing.T) {
	in, out, errRef := newTestRefs(t)
	s := BuildEdgeState("0xabc", "0aBcDeF0", "p", in, out, errRef)

	assert.True(t, s.MatchesIdentity("0aBcDeF0"))
	assert.True(t, s.MatchesIdentity("0abcdef0"), "case-insensitive")
	assert.True(t, s.MatchesIdentity("0x0aBcDeF0"), "0x prefix tolerated")
	assert.False(t, s.MatchesIdentity("deadbeef"))
	assert.False(t, s.MatchesIdentity(""))

	var nilState *EdgeState
	assert.False(t, nilState.MatchesIdentity("0aBcDeF0"))
}

func TestEdgeState_TopicRefsRoundtrip(t *testing.T) {
	in, out, errRef := newTestRefs(t)
	s := BuildEdgeState("0xabc", "0a", "p", in, out, errRef)

	gotIn, gotOut, gotErr, err := s.TopicRefs()
	require.NoError(t, err)
	assert.Equal(t, in.Locator(), gotIn.Locator())
	assert.Equal(t, out.Locator(), gotOut.Locator())
	assert.Equal(t, errRef.Locator(), gotErr.Locator())
	assert.Equal(t, topic.BackendHCS, gotIn.Transport())
}

func TestEdgeState_TopicRefsEmptyBackendDefaultsToHCS(t *testing.T) {
	s := &EdgeState{
		SchemaVersion: EdgeStateSchemaVersion,
		Topics: EdgeTopics{
			BackendKind:   "", // older state files might not record this
			StdInLocator:  "0.0.1",
			StdOutLocator: "0.0.2",
			StdErrLocator: "0.0.3",
		},
	}
	in, _, _, err := s.TopicRefs()
	require.NoError(t, err)
	assert.Equal(t, topic.BackendHCS, in.Transport())
}

func TestEdgeState_TopicRefsRejectsEmpty(t *testing.T) {
	s := &EdgeState{
		SchemaVersion: EdgeStateSchemaVersion,
		Topics: EdgeTopics{
			BackendKind:   string(topic.BackendHCS),
			StdInLocator:  "",
			StdOutLocator: "0.0.2",
			StdErrLocator: "0.0.3",
		},
	}
	_, _, _, err := s.TopicRefs()
	require.Error(t, err)
}

func TestEdgeState_SaveSetsSchemaAndUpdatedAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Build a state with zero SchemaVersion + empty UpdatedAt; SaveEdgeState
	// should fill them in.
	s := &EdgeState{
		Identity: EdgeIdentity{EVMAddress: "0xabc", PublicKeyHex: "0a"},
		Topics:   EdgeTopics{BackendKind: string(topic.BackendHCS), StdInLocator: "0.0.1", StdOutLocator: "0.0.2", StdErrLocator: "0.0.3"},
	}
	require.NoError(t, SaveEdgeState(path, s))
	assert.Equal(t, EdgeStateSchemaVersion, s.SchemaVersion)
	assert.NotEmpty(t, s.UpdatedAt)

	loaded, err := LoadEdgeState(path)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, EdgeStateSchemaVersion, loaded.SchemaVersion)
	assert.NotEmpty(t, loaded.UpdatedAt)
}

func TestEdgeState_SaveRejectsEmptyPath(t *testing.T) {
	in, out, errRef := newTestRefs(t)
	s := BuildEdgeState("0xabc", "0a", "p", in, out, errRef)
	err := SaveEdgeState("", s)
	require.Error(t, err)
}

func TestEdgeState_SaveRejectsNil(t *testing.T) {
	dir := t.TempDir()
	err := SaveEdgeState(filepath.Join(dir, "x.json"), nil)
	require.Error(t, err)
}
