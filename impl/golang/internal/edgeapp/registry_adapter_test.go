package edgeapp

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryRegistry_RegisterReturnsFreshTokenID(t *testing.T) {
	r := NewMemoryRegistry()
	tID, txRef, err := r.Register(context.Background(), "0xABC", `{"services":[]}`)
	require.NoError(t, err)
	assert.NotNil(t, tID)
	assert.NotEmpty(t, txRef)
	assert.Equal(t, "1", tID.String(), "first token should be 1")

	tID2, _, err := r.Register(context.Background(), "0xDEF", `{}`)
	require.NoError(t, err)
	assert.Equal(t, "2", tID2.String())
}

func TestMemoryRegistry_DuplicateOwnerErrors(t *testing.T) {
	r := NewMemoryRegistry()
	_, _, err := r.Register(context.Background(), "0xABC", "{}")
	require.NoError(t, err)
	_, _, err = r.Register(context.Background(), "0xABC", "{}")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAlreadyRegistered))
}

func TestMemoryRegistry_LookupTokenID(t *testing.T) {
	r := NewMemoryRegistry()
	_, _, err := r.Register(context.Background(), "0xabc", `{"a":1}`)
	require.NoError(t, err)

	t.Run("found-with-mixed-case-input", func(t *testing.T) {
		tID, ok, err := r.LookupTokenID(context.Background(), "0xABC")
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "1", tID.String())
	})

	t.Run("not-found", func(t *testing.T) {
		_, ok, err := r.LookupTokenID(context.Background(), "0xnope")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestMemoryRegistry_AgentURIByTokenID(t *testing.T) {
	r := NewMemoryRegistry()
	tID, _, err := r.Register(context.Background(), "0xabc", `{"v":1}`)
	require.NoError(t, err)
	uri, err := r.AgentURIByTokenID(context.Background(), tID)
	require.NoError(t, err)
	assert.Equal(t, `{"v":1}`, uri)
}

func TestMemoryRegistry_UpdateAgentURI(t *testing.T) {
	r := NewMemoryRegistry()
	tID, _, err := r.Register(context.Background(), "0xabc", `{"v":1}`)
	require.NoError(t, err)

	_, err = r.UpdateAgentURI(context.Background(), "0xabc", tID, `{"v":2}`)
	require.NoError(t, err)

	uri, err := r.AgentURIByTokenID(context.Background(), tID)
	require.NoError(t, err)
	assert.Equal(t, `{"v":2}`, uri)
}

func TestMemoryRegistry_UpdateUnknownTokenErrors(t *testing.T) {
	r := NewMemoryRegistry()
	missing := big.NewInt(99)
	_, err := r.UpdateAgentURI(context.Background(), "0xabc", missing, `{}`)
	require.Error(t, err)
}

func TestEnsureRegistered_FirstRunMintsFresh(t *testing.T) {
	r := NewMemoryRegistry()
	tID, fresh, err := EnsureRegistered(context.Background(), r, "0xabc", `{"v":1}`, false)
	require.NoError(t, err)
	require.NotNil(t, tID)
	assert.True(t, fresh, "first run must mint")
	assert.Equal(t, "1", tID.String())
}

func TestEnsureRegistered_SecondRunNoOp(t *testing.T) {
	r := NewMemoryRegistry()
	tID1, _, err := EnsureRegistered(context.Background(), r, "0xabc", `{"v":1}`, false)
	require.NoError(t, err)

	tID2, fresh, err := EnsureRegistered(context.Background(), r, "0xabc", `{"v":1}`, false)
	require.NoError(t, err)
	assert.False(t, fresh, "second run with matching agentURI must not mint")
	assert.Equal(t, tID1.String(), tID2.String(), "same tokenID returned")
}

func TestEnsureRegistered_AgentURIMismatchWithoutUpdate(t *testing.T) {
	r := NewMemoryRegistry()
	_, _, err := EnsureRegistered(context.Background(), r, "0xabc", `{"v":1}`, false)
	require.NoError(t, err)

	_, fresh, err := EnsureRegistered(context.Background(), r, "0xabc", `{"v":2}`, false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAgentURIMismatch))
	assert.False(t, fresh)
}

func TestEnsureRegistered_AgentURIMismatchWithUpdate(t *testing.T) {
	r := NewMemoryRegistry()
	tID, _, err := EnsureRegistered(context.Background(), r, "0xabc", `{"v":1}`, false)
	require.NoError(t, err)

	tID2, fresh, err := EnsureRegistered(context.Background(), r, "0xabc", `{"v":2}`, true)
	require.NoError(t, err)
	assert.False(t, fresh, "update path is not 'fresh'")
	assert.Equal(t, tID.String(), tID2.String())

	uri, err := r.AgentURIByTokenID(context.Background(), tID)
	require.NoError(t, err)
	assert.Equal(t, `{"v":2}`, uri)
}

func TestEnsureRegistered_NilAdapterIsNoOp(t *testing.T) {
	tID, fresh, err := EnsureRegistered(context.Background(), nil, "0xabc", `{}`, false)
	require.NoError(t, err)
	assert.Nil(t, tID)
	assert.False(t, fresh)
}

func TestEnsureRegistered_RejectsBadInput(t *testing.T) {
	r := NewMemoryRegistry()
	_, _, err := EnsureRegistered(context.Background(), r, "", "{}", false)
	require.Error(t, err)
	_, _, err = EnsureRegistered(context.Background(), r, "0xabc", "", false)
	require.Error(t, err)
}

func TestDisabledRegistry_AlwaysErrorsWithFeatureNotImplemented(t *testing.T) {
	r := NewDisabledRegistry("testnet-not-approved")
	_, _, err := r.Register(context.Background(), "0xabc", "{}")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFeatureNotImplemented))

	_, _, err = r.LookupTokenID(context.Background(), "0xabc")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFeatureNotImplemented))
}

func TestEnsureRegistered_OnDisabledRegistrySurfacesError(t *testing.T) {
	r := NewDisabledRegistry("test")
	_, _, err := EnsureRegistered(context.Background(), r, "0xabc", "{}", false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFeatureNotImplemented))
}

