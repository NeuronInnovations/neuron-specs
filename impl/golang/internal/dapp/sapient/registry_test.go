package sapient

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

func registryFixture(t *testing.T) keylib.EVMAddress {
	t.Helper()
	addr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	return addr
}

// memoryContractFor returns a fresh MemoryRegistryContract with msg.sender
// pinned to the seller's EVM so registry.Register's proof-of-control passes.
func memoryContractFor(t *testing.T, key *keylib.NeuronPrivateKey) *registry.MemoryRegistryContract {
	t.Helper()
	c := registry.NewMemoryRegistryContract()
	c.SetPendingOwner(common.BytesToAddress(key.PublicKey().EVMAddress().Bytes()))
	return c
}

func TestRegisterSeller_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	key := newCardTestKey(t)

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key, ChainID: 296})
	require.NoError(t, err)

	regAddr := registryFixture(t)
	contract := memoryContractFor(t, key)

	reg, err := RegisterSeller(ctx, key, card, regAddr, 296, contract)
	require.NoError(t, err)

	assert.Equal(t, registry.OutcomeMinted, reg.Outcome)
	assert.Equal(t, key.PublicKey().EVMAddress(), reg.SellerEVM)
	assert.Equal(t, card.NodeID, reg.NodeID)
	assert.Equal(t, card.PeerID, reg.PeerID)
	require.NotNil(t, reg.TokenID, "memory contract mints an agent ID")
	assert.Equal(t, "0xtxhash_register", reg.TransactionHash)
	assert.Len(t, reg.AgentURISha256, 64)

	// Downstream verification: discover by EVM, cross-check the binding.
	disc, err := DiscoverSeller(ctx, reg.SellerEVM, regAddr, 296, contract)
	require.NoError(t, err)
	assert.Equal(t, reg.TokenID, disc.TokenID)
	assert.Equal(t, reg.AgentURISha256, disc.AgentURISha256, "round-trip integrity")
	assert.Equal(t, ProtocolDetection, disc.Protocol)
	assert.Equal(t, card.PeerID, disc.PeerID, "advertised PeerID == seller PeerID")
	require.NotNil(t, disc.CommerceService)
	assert.Equal(t, "rid", disc.CommerceService.Name)
}

func TestRegisterSeller_Idempotent_Reused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	key := newCardTestKey(t)

	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)
	regAddr := registryFixture(t)
	contract := memoryContractFor(t, key)

	first, err := RegisterSeller(ctx, key, card, regAddr, 0, contract)
	require.NoError(t, err)
	require.Equal(t, registry.OutcomeMinted, first.Outcome)

	// Re-run with the identical card → reused unchanged, same tokenId, no error.
	second, err := RegisterSeller(ctx, key, card, regAddr, 0, contract)
	require.NoError(t, err)
	assert.Equal(t, registry.OutcomeReused, second.Outcome, "no duplicate-registration failure on re-run")
	assert.Equal(t, first.TokenID, second.TokenID)
}

func TestRegisterSeller_Idempotent_Updated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	key := newCardTestKey(t)
	regAddr := registryFixture(t)
	contract := memoryContractFor(t, key)

	card1, err := BuildSellerCard(SellerCardOptions{ChildKey: key, SensorModels: []string{"DroneScout DS240"}})
	require.NoError(t, err)
	first, err := RegisterSeller(ctx, key, card1, regAddr, 0, contract)
	require.NoError(t, err)
	require.Equal(t, registry.OutcomeMinted, first.Outcome)

	// Same key, different capability set → stale on-chain URI → refreshed in place.
	card2, err := BuildSellerCard(SellerCardOptions{ChildKey: key, SensorModels: []string{"DroneScout DS-400"}})
	require.NoError(t, err)
	second, err := RegisterSeller(ctx, key, card2, regAddr, 0, contract)
	require.NoError(t, err)
	assert.Equal(t, registry.OutcomeUpdated, second.Outcome)
	assert.Equal(t, first.TokenID, second.TokenID, "same agent ID, refreshed card")
	assert.NotEqual(t, first.AgentURISha256, second.AgentURISha256)
}

func TestDiscoverSeller_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	key := newCardTestKey(t)
	contract := registry.NewMemoryRegistryContract()

	_, err := DiscoverSeller(ctx, key.PublicKey().EVMAddress(), registryFixture(t), 0, contract)
	require.Error(t, err)
}

func TestRegisterSeller_RequiresChildKey(t *testing.T) {
	t.Parallel()
	_, err := RegisterSeller(context.Background(), nil, SellerCard{}, registryFixture(t), 0, registry.NewMemoryRegistryContract())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "childKey is required")
}

func TestRegisterSeller_RequiresContract(t *testing.T) {
	t.Parallel()
	key := newCardTestKey(t)
	card, err := BuildSellerCard(SellerCardOptions{ChildKey: key})
	require.NoError(t, err)
	_, err = RegisterSeller(context.Background(), key, card, registryFixture(t), 0, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contract is required")
}
