package adsb

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// TestRegisterSeller_Idempotent_AlreadyRegistered pins the duplicate-registration
// fix for the ADS-B seller: an already-registered seller (restart, or stale
// pre-rename AgentURI) must NOT fail with "duplicate registration". It reuses an
// up-to-date registration and refreshes a stale one via updateAgentURI.
func TestRegisterSeller_Idempotent_AlreadyRegistered(t *testing.T) {
	ctx := context.Background()
	key := newTestKey(t)
	descriptor, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:     key,
		ChainID:      296,
		FeedSource:   FeedSourceSynthetic,
		CommerceMode: CommerceModeFull,
	})
	require.NoError(t, err)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	sellerCommon := common.BytesToAddress(key.PublicKey().EVMAddress().Bytes())

	t.Run("re-run reuses existing registration", func(t *testing.T) {
		c := registry.NewMemoryRegistryContract()
		c.SetPendingOwner(sellerCommon)

		reg1, err := RegisterSeller(ctx, key, descriptor, registryAddr, 296, c)
		require.NoError(t, err)
		assert.Equal(t, registry.OutcomeMinted, reg1.Outcome)

		reg2, err := RegisterSeller(ctx, key, descriptor, registryAddr, 296, c)
		require.NoError(t, err)
		assert.Equal(t, registry.OutcomeReused, reg2.Outcome)
		require.NotNil(t, reg2.TokenID)
		assert.Equal(t, reg1.TokenID.Int64(), reg2.TokenID.Int64())
		assert.Empty(t, reg2.TransactionHash, "pure reuse submits no transaction")
	})

	t.Run("stale on-chain AgentURI is refreshed via updateAgentURI", func(t *testing.T) {
		c := registry.NewMemoryRegistryContract()
		c.SeedToken(42, sellerCommon, `{"services":[],"stale":true}`)

		reg, err := RegisterSeller(ctx, key, descriptor, registryAddr, 296, c)
		require.NoError(t, err)
		assert.Equal(t, registry.OutcomeUpdated, reg.Outcome)
		require.NotNil(t, reg.TokenID)
		assert.Equal(t, int64(42), reg.TokenID.Int64())
		assert.Equal(t, "0xtxhash_update", reg.TransactionHash)
	})
}
