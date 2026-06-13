package edgeapp

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry/bindings"
)

// evmHarness packages a simulated.Backend with a funded signer + a deployed
// NeuronIdentityRegistry. Each test gets a fresh harness; they don't share
// state.
type evmHarness struct {
	backend *simulated.Backend
	auth    *bind.TransactOpts
	addr    common.Address
	signer  common.Address
	chainID uint64
	wrapped *registry.EVMRegistryContract
	adapter *EVMRegistryAdapter
}

func newEVMHarness(t *testing.T) *evmHarness {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	signerAddr := crypto.PubkeyToAddress(key.PublicKey)

	chainID := uint64(1337)
	auth, err := bind.NewKeyedTransactorWithChainID(key, new(big.Int).SetUint64(chainID))
	require.NoError(t, err)

	// Pre-fund the signer account.
	alloc := types.GenesisAlloc{
		signerAddr: {Balance: new(big.Int).Lsh(big.NewInt(1), 100)},
	}
	backend := simulated.NewBackend(alloc, simulated.WithBlockGasLimit(30_000_000))
	t.Cleanup(func() { _ = backend.Close() })

	client := backend.Client()

	// Deploy NeuronIdentityRegistry.
	addr, tx, _, err := bindings.DeployNeuronIdentityRegistry(auth, client)
	require.NoError(t, err)
	backend.Commit()
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.Status, "deploy tx failed")

	wrapped, err := registry.NewEVMRegistryContract(client, addr, auth)
	require.NoError(t, err)
	adapter := NewEVMRegistryAdapterFromContract(wrapped, nil, signerAddr, chainID)

	return &evmHarness{
		backend: backend,
		auth:    auth,
		addr:    addr,
		signer:  signerAddr,
		chainID: chainID,
		wrapped: wrapped,
		adapter: adapter,
	}
}

// Goroutine-friendly call: simulated backend WaitMined needs the chain
// to advance, so background-tick during synchronous Register/Update
// invocations.
func (h *evmHarness) withMining(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	for {
		select {
		case <-done:
			return
		default:
			h.backend.Commit()
		}
	}
}

func TestEVMRegistryAdapter_LookupOnEmptyRegistry(t *testing.T) {
	h := newEVMHarness(t)

	tID, found, err := h.adapter.LookupTokenID(context.Background(), h.signer.Hex())
	require.NoError(t, err, "lookup against empty registry should not error — should return found=false")
	assert.False(t, found)
	assert.Nil(t, tID)
}

func TestEVMRegistryAdapter_RegisterRoundTrip(t *testing.T) {
	h := newEVMHarness(t)

	var (
		gotTokenID *big.Int
		gotTxHash  string
		regErr     error
	)
	h.withMining(func() {
		gotTokenID, gotTxHash, regErr = h.adapter.Register(context.Background(),
			h.signer.Hex(), `{"v":1}`)
	})
	require.NoError(t, regErr)
	require.NotNil(t, gotTokenID)
	assert.NotEmpty(t, gotTxHash)
	assert.NotEqual(t, "0", gotTokenID.String(), "tokenId should be > 0")

	tID, found, err := h.adapter.LookupTokenID(context.Background(), h.signer.Hex())
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, gotTokenID.String(), tID.String())

	uri, err := h.adapter.AgentURIByTokenID(context.Background(), tID)
	require.NoError(t, err)
	assert.Equal(t, `{"v":1}`, uri)
}

func TestEVMRegistryAdapter_UpdateAgentURI(t *testing.T) {
	h := newEVMHarness(t)

	var tID *big.Int
	h.withMining(func() {
		t1, _, err := h.adapter.Register(context.Background(), h.signer.Hex(), `{"v":1}`)
		require.NoError(t, err)
		tID = t1
	})

	h.withMining(func() {
		txHash, err := h.adapter.UpdateAgentURI(context.Background(), h.signer.Hex(), tID, `{"v":2}`)
		require.NoError(t, err)
		assert.NotEmpty(t, txHash)
	})

	uri, err := h.adapter.AgentURIByTokenID(context.Background(), tID)
	require.NoError(t, err)
	assert.Equal(t, `{"v":2}`, uri)
}

func TestEVMRegistryAdapter_RegisterOnBehalfRejected(t *testing.T) {
	h := newEVMHarness(t)
	otherKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	otherAddr := crypto.PubkeyToAddress(otherKey.PublicKey)
	require.NotEqual(t, h.signer, otherAddr)

	_, _, regErr := h.adapter.Register(context.Background(), otherAddr.Hex(), `{"v":1}`)
	require.Error(t, regErr, "register on behalf of a different account must be rejected client-side")
	assert.Contains(t, regErr.Error(), "does not match adapter signer")
}

func TestEVMRegistryAdapter_UpdateOnBehalfRejected(t *testing.T) {
	h := newEVMHarness(t)
	otherKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	otherAddr := crypto.PubkeyToAddress(otherKey.PublicKey)

	_, err2 := h.adapter.UpdateAgentURI(context.Background(), otherAddr.Hex(), big.NewInt(1), `{}`)
	require.Error(t, err2)
	assert.Contains(t, err2.Error(), "does not match adapter signer")
}

func TestEVMRegistryAdapter_RejectsBadAddress(t *testing.T) {
	h := newEVMHarness(t)

	_, _, err := h.adapter.LookupTokenID(context.Background(), "0xnope")
	require.Error(t, err)

	_, _, err = h.adapter.Register(context.Background(), "0xtoo-short", `{}`)
	require.Error(t, err)
}

func TestNewEVMRegistryAdapter_RejectsBadConfig(t *testing.T) {
	cases := map[string]EVMRegistryAdapterConfig{
		"missing-rpc":     {ChainID: 1, ContractAddr: "0x" + "ab" + "cdef0123456789"+"abcdef0123456789"+"abcdef00", SigningKey: nil},
		"missing-chainID": {RPC: "http://localhost", ContractAddr: "0xabcdef" + "0123456789abcdef0123456789abcdef0000"},
		"bad-addr":        {RPC: "http://localhost", ChainID: 1, ContractAddr: "deadbeef"},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewEVMRegistryAdapter(context.Background(), cfg)
			require.Error(t, err)
		})
	}
}

func TestEVMRegistryAdapter_EnsureRegisteredEndToEnd(t *testing.T) {
	h := newEVMHarness(t)

	// First call: not registered ⇒ Register fires.
	var (
		tID    *big.Int
		fresh  bool
		regErr error
	)
	h.withMining(func() {
		tID, fresh, regErr = EnsureRegistered(context.Background(), h.adapter,
			h.signer.Hex(), `{"v":1}`, true)
	})
	require.NoError(t, regErr)
	require.True(t, fresh, "first call to EnsureRegistered must mint")
	require.NotNil(t, tID)

	// Second call with same agentURI: no-op.
	tID2, fresh2, err := EnsureRegistered(context.Background(), h.adapter,
		h.signer.Hex(), `{"v":1}`, true)
	require.NoError(t, err)
	assert.False(t, fresh2)
	assert.Equal(t, tID.String(), tID2.String())

	// Third call with new agentURI + update=true: UpdateAgentURI fires.
	h.withMining(func() {
		_, _, err = EnsureRegistered(context.Background(), h.adapter,
			h.signer.Hex(), `{"v":2}`, true)
	})
	require.NoError(t, err)
	uri, err := h.adapter.AgentURIByTokenID(context.Background(), tID)
	require.NoError(t, err)
	assert.Equal(t, `{"v":2}`, uri)
}

func TestIsNotRegisteredErr(t *testing.T) {
	cases := map[string]bool{
		"":                                                          false,
		"some unrelated error":                                      false,
		"ERC721Enumerable: owner index out of bounds":                true,
		"OwnerIndexOutOfBounds(0x...,0)":                             true,
		"execution reverted":                                         true,
		"failed to estimate gas: execution reverted: nope":           false, // has decoded reason
		// Hedera/Hashio-specific "no decoded reason" wrapper:
		"registry.TokenOfOwnerByIndex: [Request ID: abc] execution reverted: CONTRACT_REVERT_EXECUTED": true,
		// Plain wrapped revert with no reason:
		"registry.TokenOfOwnerByIndex: execution reverted":           true,
	}
	for s, want := range cases {
		t.Run(s, func(t *testing.T) {
			var err error
			if s != "" {
				err = errors.New(s)
			}
			assert.Equal(t, want, isNotRegisteredErr(err))
		})
	}
}
