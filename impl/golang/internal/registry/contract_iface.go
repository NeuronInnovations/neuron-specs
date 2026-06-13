package registry

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// RegistryContract abstracts the abigen-generated EIP-8004 Identity Registry
// contract bindings. All on-chain interactions go through this interface,
// enabling mock-based testing without a live blockchain.
type RegistryContract interface {
	// Register mints a new registration NFT with the given agentURI.
	// The signer address identifies which account is sending the transaction.
	// Returns (tokenId, transactionHash, error).
	Register(ctx context.Context, signer common.Address, agentURI string) (*big.Int, string, error)

	// UpdateAgentURI updates the agentURI for an existing registration.
	// The signer address identifies which account is sending the transaction.
	// Returns (transactionHash, error).
	UpdateAgentURI(ctx context.Context, signer common.Address, tokenId *big.Int, agentURI string) (string, error)

	// OwnerOf returns the EVM address that owns the given token.
	OwnerOf(ctx context.Context, tokenId *big.Int) (common.Address, error)

	// GetApproved returns the approved operator address for a token.
	GetApproved(ctx context.Context, tokenId *big.Int) (common.Address, error)

	// IsApprovedForAll checks if an operator is approved for all tokens of an owner.
	IsApprovedForAll(ctx context.Context, owner, operator common.Address) (bool, error)

	// TokenOfOwnerByIndex returns the token ID at a given index for an owner.
	// Used for lookup: tokenOfOwnerByIndex(childAddress, 0) gets the first
	// (and per FR-R05, only) token.
	TokenOfOwnerByIndex(ctx context.Context, owner common.Address, index *big.Int) (*big.Int, error)

	// AgentURIOf returns the agentURI string for a given token ID.
	AgentURIOf(ctx context.Context, tokenId *big.Int) (string, error)

	// Burn destroys the registration NFT (revocation).
	// The signer address identifies which account is sending the transaction.
	// Returns (transactionHash, error).
	Burn(ctx context.Context, signer common.Address, tokenId *big.Int) (string, error)
}
