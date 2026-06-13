package registry

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/neuron-sdk/neuron-go-sdk/internal/registry/bindings"
)

// EVMRegistryContract implements RegistryContract by calling a deployed
// NeuronIdentityRegistry ERC-721 contract via go-ethereum's generated bindings.
//
// For testnet use, a single TransactOpts (operator account) is used for all
// write operations. Production usage would require per-agent signing.
type EVMRegistryContract struct {
	contract *bindings.NeuronIdentityRegistry
	// client is held as the broader bind.DeployBackend interface (which
	// *ethclient.Client + simulated.Client both satisfy) so the same
	// EVMRegistryContract can be exercised against in-process test
	// backends without forcing a separate adapter.
	client bind.DeployBackend
	auth   *bind.TransactOpts
}

// NewEVMRegistryContract creates an adapter backed by a deployed contract.
// The auth parameter provides the transaction signer for all write operations.
//
// `backend` is a bind.ContractBackend (typically *ethclient.Client in
// production). It must also satisfy bind.DeployBackend so the wrapper can
// call bind.WaitMined on the returned tx hashes — both *ethclient.Client
// and simulated.Client meet this requirement.
func NewEVMRegistryContract(backend ContractDeployBackend, contractAddr common.Address, auth *bind.TransactOpts) (*EVMRegistryContract, error) {
	contract, err := bindings.NewNeuronIdentityRegistry(contractAddr, backend)
	if err != nil {
		return nil, fmt.Errorf("registry.NewEVMRegistryContract: bind contract at %s: %w", contractAddr.Hex(), err)
	}
	return &EVMRegistryContract{
		contract: contract,
		client:   backend,
		auth:     auth,
	}, nil
}

// ContractDeployBackend is the union of bind.ContractBackend +
// bind.DeployBackend that EVMRegistryContract needs from its client.
// *ethclient.Client and simulated.Client both implement this interface.
type ContractDeployBackend interface {
	bind.ContractBackend
	bind.DeployBackend
}

// Register mints a new registration NFT with the given agentURI.
// The signer parameter is informational — the actual msg.sender is
// determined by the TransactOpts provided at construction.
func (e *EVMRegistryContract) Register(ctx context.Context, signer common.Address, agentURI string) (*big.Int, string, error) {
	opts := e.txOpts(ctx)
	opts.From = signer
	tx, err := e.contract.Register(opts, agentURI)
	if err != nil {
		return nil, "", fmt.Errorf("registry.Register: %w", err)
	}

	// Wait for the transaction to be mined.
	receipt, err := bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return nil, "", fmt.Errorf("registry.Register: wait mined: %w", err)
	}

	// Parse the ERC-8004 Registered event to get the agentId.
	for _, log := range receipt.Logs {
		event, err := e.contract.ParseRegistered(*log)
		if err == nil {
			return event.AgentId, tx.Hash().Hex(), nil
		}
	}

	return nil, tx.Hash().Hex(), fmt.Errorf("registry.Register: Registered event not found in receipt")
}

// UpdateAgentURI updates the agentURI for an existing registration.
// Calls the contract's ERC-8004 `setAgentURI` function.
func (e *EVMRegistryContract) UpdateAgentURI(ctx context.Context, signer common.Address, tokenId *big.Int, agentURI string) (string, error) {
	opts := e.txOpts(ctx)
	opts.From = signer
	tx, err := e.contract.SetAgentURI(opts, tokenId, agentURI)
	if err != nil {
		return "", fmt.Errorf("registry.UpdateAgentURI: %w", err)
	}

	_, err = bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return "", fmt.Errorf("registry.UpdateAgentURI: wait mined: %w", err)
	}

	return tx.Hash().Hex(), nil
}

// OwnerOf returns the EVM address that owns the given token.
func (e *EVMRegistryContract) OwnerOf(ctx context.Context, tokenId *big.Int) (common.Address, error) {
	owner, err := e.contract.OwnerOf(&bind.CallOpts{Context: ctx}, tokenId)
	if err != nil {
		return common.Address{}, fmt.Errorf("registry.OwnerOf: %w", err)
	}
	return owner, nil
}

// GetApproved returns the approved operator address for a token.
func (e *EVMRegistryContract) GetApproved(ctx context.Context, tokenId *big.Int) (common.Address, error) {
	approved, err := e.contract.GetApproved(&bind.CallOpts{Context: ctx}, tokenId)
	if err != nil {
		return common.Address{}, fmt.Errorf("registry.GetApproved: %w", err)
	}
	return approved, nil
}

// IsApprovedForAll checks if an operator is approved for all tokens of an owner.
func (e *EVMRegistryContract) IsApprovedForAll(ctx context.Context, owner, operator common.Address) (bool, error) {
	approved, err := e.contract.IsApprovedForAll(&bind.CallOpts{Context: ctx}, owner, operator)
	if err != nil {
		return false, fmt.Errorf("registry.IsApprovedForAll: %w", err)
	}
	return approved, nil
}

// TokenOfOwnerByIndex returns the token ID at a given index for an owner.
func (e *EVMRegistryContract) TokenOfOwnerByIndex(ctx context.Context, owner common.Address, index *big.Int) (*big.Int, error) {
	tokenId, err := e.contract.TokenOfOwnerByIndex(&bind.CallOpts{Context: ctx}, owner, index)
	if err != nil {
		return nil, fmt.Errorf("registry.TokenOfOwnerByIndex: %w", err)
	}
	return tokenId, nil
}

// AgentURIOf returns the agentURI string for a given token ID.
func (e *EVMRegistryContract) AgentURIOf(ctx context.Context, tokenId *big.Int) (string, error) {
	uri, err := e.contract.AgentURI(&bind.CallOpts{Context: ctx}, tokenId)
	if err != nil {
		return "", fmt.Errorf("registry.AgentURIOf: %w", err)
	}
	return uri, nil
}

// Burn destroys the registration NFT (revocation).
func (e *EVMRegistryContract) Burn(ctx context.Context, signer common.Address, tokenId *big.Int) (string, error) {
	opts := e.txOpts(ctx)
	opts.From = signer
	tx, err := e.contract.Revoke(opts, tokenId)
	if err != nil {
		return "", fmt.Errorf("registry.Burn: %w", err)
	}

	_, err = bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return "", fmt.Errorf("registry.Burn: wait mined: %w", err)
	}

	return tx.Hash().Hex(), nil
}

// txOpts creates a copy of the base TransactOpts with the given context.
func (e *EVMRegistryContract) txOpts(ctx context.Context) *bind.TransactOpts {
	return &bind.TransactOpts{
		From:     e.auth.From,
		Signer:   e.auth.Signer,
		GasLimit: e.auth.GasLimit,
		Context:  ctx,
	}
}

// Compile-time assertion: EVMRegistryContract implements RegistryContract.
var _ RegistryContract = (*EVMRegistryContract)(nil)
