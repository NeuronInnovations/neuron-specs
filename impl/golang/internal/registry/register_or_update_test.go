package registry

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RegisterOrUpdate is the idempotent registration entry point: it mints a fresh
// registration when none exists, refreshes the AgentURI when an existing
// registration carries a stale one (e.g. after a protocol-ID rename), and
// reuses an existing registration untouched when the AgentURI already matches.
//
// These tests pin all three branches against the in-memory contract.

func TestRegisterOrUpdate_FreshMint(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childCommon := common.BytesToAddress(childKey.PublicKey().EVMAddress().Bytes())

	mock := NewMemoryRegistryContract()
	mock.SetPendingOwner(childCommon)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	agentURI := buildValidAgentURI(t, &childKey)

	result, outcome, err := RegisterOrUpdate(context.Background(), &childKey, registryAddr, 1,
		agentURI, mock, PermissionlessPolicy{}, "")
	require.NoError(t, err)

	assert.Equal(t, OutcomeMinted, outcome)
	assert.NotNil(t, result.TokenId())
	assert.Equal(t, "0xtxhash_register", result.TransactionHash())
	assert.NotEmpty(t, result.AgentURIString())
}

func TestRegisterOrUpdate_ReuseWhenAgentURIMatches(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childCommon := common.BytesToAddress(childKey.PublicKey().EVMAddress().Bytes())

	agentURI := buildValidAgentURI(t, &childKey)
	agentURIJSON, err := agentURI.ToJSON()
	require.NoError(t, err)

	mock := NewMemoryRegistryContract()
	// Existing registration whose on-chain AgentURI already matches.
	mock.SeedToken(7, childCommon, agentURIJSON)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	result, outcome, err := RegisterOrUpdate(context.Background(), &childKey, registryAddr, 1,
		agentURI, mock, PermissionlessPolicy{}, "")
	require.NoError(t, err)

	assert.Equal(t, OutcomeReused, outcome)
	require.NotNil(t, result.TokenId())
	assert.Equal(t, int64(7), result.TokenId().Int64())
	// Pure reuse: no transaction submitted.
	assert.Empty(t, result.TransactionHash())
	assert.Equal(t, agentURIJSON, result.AgentURIString())
}

func TestRegisterOrUpdate_UpdateWhenAgentURIChanged(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childCommon := common.BytesToAddress(childKey.PublicKey().EVMAddress().Bytes())

	// Existing registration carries a STALE AgentURI (simulates the pre-rename
	// on-chain state — different JSON than the seller now computes).
	mock := NewMemoryRegistryContract()
	mock.SeedToken(9, childCommon, `{"services":[],"stale":true}`)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	agentURI := buildValidAgentURI(t, &childKey)
	newJSON, err := agentURI.ToJSON()
	require.NoError(t, err)

	result, outcome, err := RegisterOrUpdate(context.Background(), &childKey, registryAddr, 1,
		agentURI, mock, PermissionlessPolicy{}, "")
	require.NoError(t, err)

	assert.Equal(t, OutcomeUpdated, outcome)
	require.NotNil(t, result.TokenId())
	assert.Equal(t, int64(9), result.TokenId().Int64())
	assert.Equal(t, "0xtxhash_update", result.TransactionHash())
	assert.Equal(t, newJSON, result.AgentURIString())

	// On-chain AgentURI is now the refreshed one.
	stored, err := mock.AgentURIOf(context.Background(), big.NewInt(9))
	require.NoError(t, err)
	assert.Equal(t, newJSON, stored)
}

func TestRegisterOrUpdate_OutcomeString(t *testing.T) {
	assert.Equal(t, "minted", OutcomeMinted.String())
	assert.Equal(t, "updated", OutcomeUpdated.String())
	assert.Equal(t, "reused", OutcomeReused.String())
}
