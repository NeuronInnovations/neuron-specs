package remoteid

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// Test_FR_R01_RegistrySellerDiscoveryRoundtrip groups every assertion that
// proves the seller's on-chain advertisement is what the buyer reads back.
// This is the load-bearing assertion for the Level-R1 demo path.
func Test_FR_R01_RegistrySellerDiscoveryRoundtrip(t *testing.T) {
	t.Run("happy-path-roundtrip", TestRegisterSeller_DiscoverSeller_RoundTrip)
	t.Run("discover-missing-seller-not-found", TestDiscoverSeller_NotFound)
	t.Run("register-requires-child-key", TestRegisterSeller_RequiresChildKey)
	t.Run("register-requires-contract", TestRegisterSeller_RequiresContract)
	t.Run("discover-requires-contract", TestDiscoverSeller_RequiresContract)
	t.Run("discover-parses-tampered-agenturi", TestDiscoverSeller_TamperedAgentURIFailsParse)
}

func registryFixture(t *testing.T) keylib.EVMAddress {
	t.Helper()
	addr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	return addr
}

func TestRegisterSeller_DiscoverSeller_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	key := newTestKey(t)
	descriptor, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:       key,
		ChainID:        296,
		EscrowContract: "0xCAFE0000000000000000000000000000000000ce",
	})
	require.NoError(t, err)

	regAddr := registryFixture(t)
	contract := registry.NewMemoryRegistryContract()
	// Pin msg.sender to the seller's EVM so registry.Register's
	// proof-of-control check (ownerOf == childAddress) passes.
	contract.SetPendingOwner(common.BytesToAddress(key.PublicKey().EVMAddress().Bytes()))

	// --- Seller side ---
	reg, err := RegisterSeller(ctx, key, descriptor, regAddr, 296, contract)
	require.NoError(t, err)

	assert.Equal(t, key.PublicKey().EVMAddress(), reg.SellerEVM, "seller EVM matches child key")
	assert.Equal(t, regAddr, reg.RegistryAddress)
	assert.Equal(t, uint64(296), reg.ChainID)
	require.NotNil(t, reg.TokenID, "memory contract must mint a tokenId")
	assert.Equal(t, "0xtxhash_register", reg.TransactionHash)
	assert.NotEmpty(t, reg.AgentURIJSON)
	assert.Len(t, reg.AgentURISha256, 64, "sha256 hex is 64 chars")

	// --- Buyer side ---
	disc, err := DiscoverSeller(ctx, reg.SellerEVM, regAddr, 296, contract)
	require.NoError(t, err)

	assert.Equal(t, reg.SellerEVM, disc.SellerEVM)
	assert.Equal(t, reg.TokenID, disc.TokenID, "buyer reads back the same tokenId")
	assert.Equal(t, reg.AgentURISha256, disc.AgentURISha256,
		"seller's on-chain agentURI hash equals what the buyer reads (round-trip integrity)")

	// FR-R02: protocol-id is /ds240/raw/1.0.0.
	assert.Equal(t, ProtocolRaw, disc.Protocol)

	// PeerID matches the child key's deterministic PeerID.
	expectedPeer, err := key.PublicKey().PeerID()
	require.NoError(t, err)
	assert.Equal(t, expectedPeer.String(), disc.PeerID)

	// FR-R01: commerce service is present with name="remote-id" and pricing.unit="frame".
	require.NotNil(t, disc.CommerceService, "FR-R01 requires a neuron-commerce entry named 'remote-id'")
	assert.Equal(t, CommerceServiceName, disc.CommerceService.Name)
	assert.Equal(t, PricingUnit, disc.CommerceService.Pricing.Unit)
}

func TestDiscoverSeller_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	unregistered, err := keylib.EVMAddressFromHex("0xDEAD000000000000000000000000000000000DEAD"[:42])
	require.NoError(t, err)

	contract := registry.NewMemoryRegistryContract()
	_, err = DiscoverSeller(ctx, unregistered, registryFixture(t), 296, contract)
	require.Error(t, err)

	var notFound registry.RegistrationNotFound
	assert.ErrorAs(t, err, &notFound)
}

func TestRegisterSeller_RequiresChildKey(t *testing.T) {
	t.Parallel()
	_, err := RegisterSeller(
		context.Background(),
		nil,
		ServiceDescriptor{},
		registryFixture(t),
		1,
		registry.NewMemoryRegistryContract(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "childKey is required")
}

func TestRegisterSeller_RequiresContract(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)
	_, err := RegisterSeller(
		context.Background(),
		key,
		ServiceDescriptor{},
		registryFixture(t),
		1,
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contract is required")
}

func TestDiscoverSeller_RequiresContract(t *testing.T) {
	t.Parallel()
	_, err := DiscoverSeller(
		context.Background(),
		registryFixture(t),
		registryFixture(t),
		1,
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contract is required")
}

// Test_FR_R03b_DiscoverResolvesAddrInfo proves the Stage 3B
// registry-only discovery path: a seller registered with multiaddrs
// embedded in its NeuronP2PExchangeService — and a buyer that reads
// them straight out of DiscoverResult and dials without out-of-band data.
func Test_FR_R03b_DiscoverResolvesAddrInfo(t *testing.T) {
	t.Run("happy-path-roundtrip", TestDiscoverResolveDialAddrs_HappyPath)
	t.Run("empty-multiaddrs-errors-clearly", TestDiscoverResolveDialAddrs_EmptyMultiaddrsErrors)
}

func TestDiscoverResolveDialAddrs_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	key := newTestKey(t)
	descriptor, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey: key,
		Multiaddrs: []string{
			"/ip4/127.0.0.1/udp/40001/quic-v1",
			"/ip4/10.0.0.5/udp/40001/quic-v1",
		},
	})
	require.NoError(t, err)

	contract := registry.NewMemoryRegistryContract()
	contract.SetPendingOwner(common.BytesToAddress(key.PublicKey().EVMAddress().Bytes()))
	_, err = RegisterSeller(ctx, key, descriptor, registryFixture(t), 296, contract)
	require.NoError(t, err)

	discovery, err := DiscoverSeller(ctx, key.PublicKey().EVMAddress(), registryFixture(t), 296, contract)
	require.NoError(t, err)

	addrInfo, err := discovery.ResolveDialAddrs()
	require.NoError(t, err)
	expectedPeer, err := key.PublicKey().PeerID()
	require.NoError(t, err)
	// keylib.PeerID and libp2p peer.ID have the same canonical string
	// form (the multihash) — compare via String() to avoid the type
	// mismatch between the two wrappers.
	assert.Equal(t, expectedPeer.String(), addrInfo.ID.String())
	require.Len(t, addrInfo.Addrs, 2)
	assert.Equal(t, "/ip4/127.0.0.1/udp/40001/quic-v1", addrInfo.Addrs[0].String())
	assert.Equal(t, "/ip4/10.0.0.5/udp/40001/quic-v1", addrInfo.Addrs[1].String())
}

func TestDiscoverResolveDialAddrs_EmptyMultiaddrsErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	key := newTestKey(t)
	// Build a descriptor WITHOUT Multiaddrs — Stage 1/2 compatibility.
	descriptor, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: key})
	require.NoError(t, err)

	contract := registry.NewMemoryRegistryContract()
	contract.SetPendingOwner(common.BytesToAddress(key.PublicKey().EVMAddress().Bytes()))
	_, err = RegisterSeller(ctx, key, descriptor, registryFixture(t), 296, contract)
	require.NoError(t, err)

	discovery, err := DiscoverSeller(ctx, key.PublicKey().EVMAddress(), registryFixture(t), 296, contract)
	require.NoError(t, err)

	_, err = discovery.ResolveDialAddrs()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoRegistryMultiaddrs,
		"Stage 3B requires the seller to embed multiaddrs; absent → clear sentinel error so the buyer can branch")
}

// TestDiscoverSeller_TamperedAgentURIFailsParse simulates a registry whose
// stored agentURI string is corrupt (e.g., the on-chain value was overwritten
// by an external admin). The buyer's lookup MUST fail loudly rather than
// silently treat the seller as discoverable.
func TestDiscoverSeller_TamperedAgentURIFailsParse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	owner := common.HexToAddress("0xA111111111111111111111111111111111111111")
	contract := registry.NewMemoryRegistryContract()
	contract.SeedToken(1, owner, "not-a-valid-json-blob")

	sellerEVM, err := keylib.EVMAddressFromHex(owner.Hex())
	require.NoError(t, err)

	_, err = DiscoverSeller(ctx, sellerEVM, registryFixture(t), 296, contract)
	require.Error(t, err)
	// Surfaces as RegistryUnavailable per resolver.go:86 ("failed to parse agentURI JSON").
	var unavailable registry.RegistryUnavailable
	assert.ErrorAs(t, err, &unavailable)
}

// TestRegisterSeller_Idempotent_AlreadyRegistered pins the duplicate-registration
// fix: a Remote ID seller that is already registered (across a restart, or with a
// stale pre-rename AgentURI) must NOT fail with "duplicate registration". It
// reuses an up-to-date registration and refreshes a stale one via updateAgentURI.
func TestRegisterSeller_Idempotent_AlreadyRegistered(t *testing.T) {
	ctx := context.Background()
	key := newTestKey(t)
	descriptor, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:       key,
		ChainID:        296,
		EscrowContract: "0xCAFE0000000000000000000000000000000000ce",
	})
	require.NoError(t, err)
	regAddr := registryFixture(t)
	sellerCommon := common.BytesToAddress(key.PublicKey().EVMAddress().Bytes())

	t.Run("re-run reuses existing registration", func(t *testing.T) {
		c := registry.NewMemoryRegistryContract()
		c.SetPendingOwner(sellerCommon)

		reg1, err := RegisterSeller(ctx, key, descriptor, regAddr, 296, c)
		require.NoError(t, err)
		assert.Equal(t, registry.OutcomeMinted, reg1.Outcome)

		// Second call with the same AgentURI must reuse, not error.
		reg2, err := RegisterSeller(ctx, key, descriptor, regAddr, 296, c)
		require.NoError(t, err)
		assert.Equal(t, registry.OutcomeReused, reg2.Outcome)
		require.NotNil(t, reg2.TokenID)
		assert.Equal(t, reg1.TokenID.Int64(), reg2.TokenID.Int64())
		assert.Empty(t, reg2.TransactionHash, "pure reuse submits no transaction")
	})

	t.Run("stale on-chain AgentURI is refreshed via updateAgentURI", func(t *testing.T) {
		c := registry.NewMemoryRegistryContract()
		c.SeedToken(42, sellerCommon, `{"services":[],"stale":true}`)

		reg, err := RegisterSeller(ctx, key, descriptor, regAddr, 296, c)
		require.NoError(t, err)
		assert.Equal(t, registry.OutcomeUpdated, reg.Outcome)
		require.NotNil(t, reg.TokenID)
		assert.Equal(t, int64(42), reg.TokenID.Int64())
		assert.Equal(t, "0xtxhash_update", reg.TransactionHash)
	})
}
