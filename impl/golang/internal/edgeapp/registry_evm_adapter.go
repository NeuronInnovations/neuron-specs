package edgeapp

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// EVMRegistryAdapter satisfies edgeapp.RegistryAdapter over a deployed
// NeuronIdentityRegistry ERC-8004 contract via internal/registry's
// EVMRegistryContract. It is the iteration-4 replacement for the
// DisabledRegistry sentinel that iteration 2/3 returned for
// "force-testnet" / "testnet" modes.
//
// Lifecycle:
//   - Construction does NOT send any transaction. It dials the RPC,
//     binds the ABI, and prepares a bind.TransactOpts from the signing key.
//   - The first call to Register or UpdateAgentURI triggers an on-chain
//     transaction. Operators MUST have approved spending of testnet HBAR
//     gas before constructing this adapter.
//
// The adapter is **single-signer**: every Register / UpdateAgentURI is
// signed by the configured signing key. This matches the edge-seller /
// edge-buyer demo where the agent's secp256k1 private key is also the
// EVM signer. Registering on behalf of a different account is rejected
// at the LookupTokenID / Register boundary (ownerEVM mismatch).
type EVMRegistryAdapter struct {
	contract *registry.EVMRegistryContract
	client   *ethclient.Client
	signer   common.Address
	chainID  uint64
}

// EVMRegistryAdapterConfig captures the env-derived fields. Resolved from:
//
//	NEURON_EDGE_HEDERA_EVM_RPC   → RPC
//	NEURON_EDGE_CHAIN_ID         → ChainID
//	NEURON_EDGE_REGISTRY_ADDR    → ContractAddr
//	NEURON_EDGE_PRIVATE_KEY      → SigningKey (already loaded by main.go)
type EVMRegistryAdapterConfig struct {
	RPC          string
	ChainID      uint64
	ContractAddr string
	SigningKey   *keylib.NeuronPrivateKey
}

// NewEVMRegistryAdapter dials the configured RPC, binds the registry
// contract at ContractAddr, and returns an adapter ready for use. Errors
// are surfaced from the dial / bind step; no transaction is sent.
func NewEVMRegistryAdapter(ctx context.Context, cfg EVMRegistryAdapterConfig) (*EVMRegistryAdapter, error) {
	if cfg.RPC == "" {
		return nil, errors.New("evm registry: RPC required")
	}
	if cfg.ChainID == 0 {
		return nil, errors.New("evm registry: ChainID required")
	}
	if !looksLikeAddr(cfg.ContractAddr) {
		return nil, fmt.Errorf("evm registry: invalid ContractAddr %q (want 0x + 40 hex)", cfg.ContractAddr)
	}
	if cfg.SigningKey == nil {
		return nil, errors.New("evm registry: SigningKey required")
	}

	client, err := ethclient.DialContext(ctx, cfg.RPC)
	if err != nil {
		return nil, fmt.Errorf("evm registry: dial %s: %w", cfg.RPC, err)
	}

	ecdsaPriv, err := cfg.SigningKey.ToBlockchainKey()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("evm registry: convert signing key: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(ecdsaPriv, new(big.Int).SetUint64(cfg.ChainID))
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("evm registry: new transactor: %w", err)
	}
	auth.Context = ctx
	signerAddr := auth.From

	contractAddr := common.HexToAddress(cfg.ContractAddr)
	contract, err := registry.NewEVMRegistryContract(client, contractAddr, auth)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("evm registry: bind contract: %w", err)
	}

	return &EVMRegistryAdapter{
		contract: contract,
		client:   client,
		signer:   signerAddr,
		chainID:  cfg.ChainID,
	}, nil
}

// NewEVMRegistryAdapterFromContract is the test-friendly constructor. It
// bypasses the dial step and accepts an already-bound EVMRegistryContract
// + signer + client. Production code uses NewEVMRegistryAdapter; tests
// use this with a simulated.Backend-backed client.
func NewEVMRegistryAdapterFromContract(
	contract *registry.EVMRegistryContract,
	client *ethclient.Client,
	signer common.Address,
	chainID uint64,
) *EVMRegistryAdapter {
	return &EVMRegistryAdapter{
		contract: contract,
		client:   client,
		signer:   signer,
		chainID:  chainID,
	}
}

// Close releases the RPC client. The adapter is unusable after Close.
func (a *EVMRegistryAdapter) Close() {
	if a == nil || a.client == nil {
		return
	}
	a.client.Close()
}

// Signer returns the EVM address of this adapter's signing key. Useful
// for callers that need to compare against the agent's expected owner.
func (a *EVMRegistryAdapter) Signer() common.Address { return a.signer }

// LookupTokenID implements RegistryAdapter.
//
// NeuronIdentityRegistry enforces one-registration-per-address (FR-C-06).
// We look up by calling TokenOfOwnerByIndex(owner, 0); the call reverts
// on the contract side when the owner has no token. We map "ERC721:
// owner index out of bounds" / "OwnerIndexOutOfBounds" / generic "execution
// reverted" into (nil, false, nil) — the "not registered" signal.
func (a *EVMRegistryAdapter) LookupTokenID(ctx context.Context, ownerEVM string) (*big.Int, bool, error) {
	if !looksLikeAddr(ownerEVM) {
		return nil, false, fmt.Errorf("evm registry: invalid ownerEVM %q", ownerEVM)
	}
	owner := common.HexToAddress(ownerEVM)
	tokenID, err := a.contract.TokenOfOwnerByIndex(ctx, owner, big.NewInt(0))
	if err != nil {
		if isNotRegisteredErr(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return tokenID, true, nil
}

// AgentURIByTokenID implements RegistryAdapter.
func (a *EVMRegistryAdapter) AgentURIByTokenID(ctx context.Context, tokenID *big.Int) (string, error) {
	if tokenID == nil {
		return "", errors.New("evm registry: nil tokenID")
	}
	return a.contract.AgentURIOf(ctx, tokenID)
}

// Register implements RegistryAdapter.
//
// The adapter is single-signer: ownerEVM MUST match the adapter's
// configured signer. Registering on behalf of a different account is
// rejected up-front to prevent accidental cross-identity transactions.
func (a *EVMRegistryAdapter) Register(ctx context.Context, ownerEVM string, agentURI string) (*big.Int, string, error) {
	if !looksLikeAddr(ownerEVM) {
		return nil, "", fmt.Errorf("evm registry: invalid ownerEVM %q", ownerEVM)
	}
	owner := common.HexToAddress(ownerEVM)
	if owner != a.signer {
		return nil, "", fmt.Errorf("evm registry: ownerEVM %s does not match adapter signer %s",
			owner.Hex(), a.signer.Hex())
	}
	return a.contract.Register(ctx, owner, agentURI)
}

// UpdateAgentURI implements RegistryAdapter.
//
// Like Register, it requires ownerEVM == signer. Updating on behalf of a
// different account is rejected — the contract would revert anyway, but
// catching it client-side gives a clearer error.
func (a *EVMRegistryAdapter) UpdateAgentURI(ctx context.Context, ownerEVM string, tokenID *big.Int, agentURI string) (string, error) {
	if tokenID == nil {
		return "", errors.New("evm registry: nil tokenID")
	}
	if !looksLikeAddr(ownerEVM) {
		return "", fmt.Errorf("evm registry: invalid ownerEVM %q", ownerEVM)
	}
	owner := common.HexToAddress(ownerEVM)
	if owner != a.signer {
		return "", fmt.Errorf("evm registry: ownerEVM %s does not match adapter signer %s",
			owner.Hex(), a.signer.Hex())
	}
	return a.contract.UpdateAgentURI(ctx, owner, tokenID, agentURI)
}

// isNotRegisteredErr reports whether err looks like the
// "owner has no token at index" revert from ERC721Enumerable. The exact
// substring varies by node implementation (Geth, Erigon, Besu, Hashio
// gateway, Anvil); we check for the OpenZeppelin error name + the
// generic "execution reverted" with no extra data.
//
// Fail-safe stance: false positives (real failures misclassified as
// "not registered") cause EnsureRegistered to attempt a Register, which
// then surfaces the real revert reason. False negatives (real
// "no registration" misclassified as failure) cause the seller's
// EnsureRegistered to error and the operator to investigate. We bias
// toward false positives — better to retry-register than to silently
// loop on lookup failure.
func isNotRegisteredErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "out of bounds") || strings.Contains(s, "out_of_bounds") {
		return true
	}
	if strings.Contains(s, "ownerindexoutofbounds") || strings.Contains(s, "erc721enumerable") {
		return true
	}
	// Plain or wrapped "execution reverted" without a decoded reason
	// (Hashio gateway, simulated.Backend, some Geth versions). The
	// substring check tolerates wrappers like
	// "registry.TokenOfOwnerByIndex: execution reverted".
	if strings.Contains(s, "execution reverted") {
		// Hedera EVM (Hashio) returns "execution reverted: CONTRACT_REVERT_EXECUTED"
		// as a generic "the contract reverted, no decoded reason" signal.
		// This is the typical response for ERC721Enumerable.tokenOfOwnerByIndex
		// when the owner has zero tokens. Treat it as not-registered.
		if strings.Contains(s, "contract_revert_executed") {
			return true
		}
		// Any other "execution reverted: <decoded reason>" is a real revert
		// (e.g., a custom error with a meaningful name). Propagate.
		if strings.Contains(s, "execution reverted:") {
			return false
		}
		// Plain "execution reverted" without a colon ⇒ no decoded reason ⇒ treat as not-registered.
		return true
	}
	return false
}

// Compile-time assertion that the adapter satisfies the interface.
var _ RegistryAdapter = (*EVMRegistryAdapter)(nil)
