package sapient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// RegisterResult is the evidence-grade record of a SAPIENT seller registration.
// It never includes private key material; the seller's EVM is public and the
// AgentURI SHA-256 is a content digest of what was registered.
type RegisterResult struct {
	// SellerEVM is the seller's EVM address (the token owner / childAddress).
	SellerEVM keylib.EVMAddress

	// NodeID is the SAPIENT node_id bound to SellerEVM (NodeIDFromIdentity).
	NodeID string

	// PeerID is the seller's libp2p PeerID, as advertised in the card's
	// neuron-p2p-exchange service.
	PeerID string

	// RegistryAddress is the Identity Registry contract address.
	RegistryAddress keylib.EVMAddress

	// ChainID is the EVM chain id where the registry lives (0 for the in-memory
	// contract used by local evidence mode).
	ChainID uint64

	// TokenID is the EIP-8004 NFT tokenId minted by Register — the seller's
	// agent ID. May be a small in-memory counter when using the memory contract.
	TokenID *big.Int

	// TransactionHash is the tx hash from the contract (a synthetic
	// "0xtxhash_register" string for the memory contract).
	TransactionHash string

	// AgentURIJSON is the canonical card JSON that was registered.
	AgentURIJSON string

	// AgentURISha256 is the hex SHA-256 of AgentURIJSON.
	AgentURISha256 string

	// Outcome records whether this was a fresh mint, a stale-URI refresh, or an
	// unchanged reuse — so idempotent re-runs are evidenced.
	Outcome registry.RegisterOutcome
}

// RegisterSeller registers a SAPIENT seller's Agent Card in an EIP-8004 Identity
// Registry. It wraps registry.RegisterOrUpdate (permissionless admission), so a
// re-run never fails with "duplicate registration": it mints when unregistered,
// refreshes a stale AgentURI, and reuses an up-to-date one.
//
// The contract is taken by interface, so callers pass either a real
// EVMRegistryContract or a MemoryRegistryContract (the testnet-free default for
// local evidence mode). FR-S20a / 003 FR-R02 / V-REG-12.
func RegisterSeller(
	ctx context.Context,
	childKey *keylib.NeuronPrivateKey,
	card SellerCard,
	registryAddr keylib.EVMAddress,
	chainID uint64,
	contract registry.RegistryContract,
) (RegisterResult, error) {
	if childKey == nil {
		return RegisterResult{}, errors.New("sapient.RegisterSeller: childKey is required")
	}
	if contract == nil {
		return RegisterResult{}, errors.New("sapient.RegisterSeller: contract is required")
	}

	result, outcome, err := registry.RegisterOrUpdate(
		ctx, childKey, registryAddr, chainID,
		card.AgentURI, contract,
		registry.PermissionlessPolicy{}, "",
	)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("sapient.RegisterSeller: %w", err)
	}

	agentURIJSON := result.AgentURIString()
	sha := sha256.Sum256([]byte(agentURIJSON))

	return RegisterResult{
		SellerEVM:       result.ChildAddress(),
		NodeID:          card.NodeID,
		PeerID:          card.PeerID,
		RegistryAddress: result.RegistryAddress(),
		ChainID:         result.ChainId(),
		TokenID:         result.TokenId(),
		TransactionHash: result.TransactionHash(),
		AgentURIJSON:    agentURIJSON,
		AgentURISha256:  hex.EncodeToString(sha[:]),
		Outcome:         outcome,
	}, nil
}

// DiscoverResult summarises what a verifier learned about a SAPIENT seller
// through the registry. The SellerEVM ↔ PeerID ↔ node_id binding is the
// load-bearing fact for downstream verification.
type DiscoverResult struct {
	SellerEVM       keylib.EVMAddress
	RegistryAddress keylib.EVMAddress
	ChainID         uint64
	TokenID         *big.Int

	// PeerID is the seller's libp2p PeerID from the card's neuron-p2p-exchange
	// service. A verifier cross-checks this against the PeerID that dialled in.
	PeerID string

	// Protocol is the advertised stream protocol — /sapient/detection/2.0.0 for
	// a conforming SAPIENT seller.
	Protocol string

	// CommerceService is the "rid" neuron-commerce entry, or nil if absent.
	CommerceService *payment.NeuronCommerceService

	// AgentURIJSON is the raw card JSON read back, with its SHA-256.
	AgentURIJSON   string
	AgentURISha256 string
}

// DiscoverSeller looks a SAPIENT seller up in the registry by EVM address and
// extracts the verification-relevant view (PeerID, protocol, rid commerce).
// 003 FR-R02 + FR-R05 (lookup by EVM).
func DiscoverSeller(
	ctx context.Context,
	sellerEVM keylib.EVMAddress,
	registryAddr keylib.EVMAddress,
	chainID uint64,
	contract registry.RegistryContract,
) (DiscoverResult, error) {
	if contract == nil {
		return DiscoverResult{}, errors.New("sapient.DiscoverSeller: contract is required")
	}

	reg, err := registry.LookupRegistration(
		ctx, registryAddr, chainID,
		registry.ByEVMAddress(sellerEVM), contract,
	)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("sapient.DiscoverSeller: %w", err)
	}

	p2p, err := registry.ResolveP2PExchange(reg)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("sapient.DiscoverSeller: resolve p2p service: %w", err)
	}

	uri := reg.AgentURI()
	commerceServices := payment.FilterByName(uri.CommerceServices(), CommerceServiceName)
	var commerce *payment.NeuronCommerceService
	if len(commerceServices) > 0 {
		c := commerceServices[0]
		commerce = &c
	}

	agentURIJSON, marshalErr := uri.ToJSON()
	if marshalErr != nil {
		return DiscoverResult{}, fmt.Errorf("sapient.DiscoverSeller: serialize agentURI: %w", marshalErr)
	}
	sha := sha256.Sum256([]byte(agentURIJSON))

	return DiscoverResult{
		SellerEVM:       sellerEVM,
		RegistryAddress: registryAddr,
		ChainID:         chainID,
		TokenID:         reg.TokenId(),
		PeerID:          p2p.PeerID,
		Protocol:        p2p.Protocol,
		CommerceService: commerce,
		AgentURIJSON:    agentURIJSON,
		AgentURISha256:  hex.EncodeToString(sha[:]),
	}, nil
}
