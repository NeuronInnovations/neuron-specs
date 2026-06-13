package registry

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// Registration represents a Child's registration in an EIP-8004 registry,
// linked to an extended NFT (ERC-721). One registration per (childAddress,
// registryAddress) per FR-R05.
type Registration struct {
	registryAddress keylib.EVMAddress
	childAddress    keylib.EVMAddress
	tokenId         *big.Int
	agentURI        AgentURI
	chainId         uint64
}

// RegistryAddress returns the registry's EVM address.
func (r *Registration) RegistryAddress() keylib.EVMAddress { return r.registryAddress }

// ChildAddress returns the child's EVM address.
func (r *Registration) ChildAddress() keylib.EVMAddress { return r.childAddress }

// TokenId returns a defensive copy of the registration token ID.
func (r *Registration) TokenId() *big.Int {
	if r.tokenId == nil {
		return nil
	}
	return new(big.Int).Set(r.tokenId)
}

// AgentURI returns the agent URI for this registration.
func (r *Registration) AgentURI() AgentURI { return r.agentURI }

// ChainId returns the chain ID of the registry.
func (r *Registration) ChainId() uint64 { return r.chainId }

// RegistrationResult is the result of a registration operation (create,
// update, or revoke).
type RegistrationResult struct {
	tokenId         *big.Int
	transactionHash string
	childAddress    keylib.EVMAddress
	registryAddress keylib.EVMAddress
	chainId         uint64
	agentURIString  string
}

// TokenId returns a defensive copy of the registration token ID.
func (r *RegistrationResult) TokenId() *big.Int {
	if r.tokenId == nil {
		return nil
	}
	return new(big.Int).Set(r.tokenId)
}

// TransactionHash returns the transaction hash.
func (r *RegistrationResult) TransactionHash() string { return r.transactionHash }

// ChildAddress returns the child's EVM address.
func (r *RegistrationResult) ChildAddress() keylib.EVMAddress { return r.childAddress }

// RegistryAddress returns the registry's EVM address.
func (r *RegistrationResult) RegistryAddress() keylib.EVMAddress { return r.registryAddress }

// ChainId returns the chain ID of the registry.
func (r *RegistrationResult) ChainId() uint64 { return r.chainId }

// AgentURIString returns the serialized agent URI JSON string.
func (r *RegistrationResult) AgentURIString() string { return r.agentURIString }

// Register mints a new registration NFT for a Child agent in the given
// EIP-8004 Identity Registry.
//
// Steps:
//  1. Derive childAddress from childKey.PublicKey().EVMAddress()
//  2. Validate agentURI via ValidateRegistrationCompleteness
//  3. Check for duplicate registration (tokenOfOwnerByIndex returns existing token)
//  4. Serialize agentURI to JSON
//  5. Call contract.Register()
//  6. Verify ownerOf matches childAddress
//  7. Return RegistrationResult
func Register(
	ctx context.Context,
	childKey *keylib.NeuronPrivateKey,
	registryAddr keylib.EVMAddress,
	chainId uint64,
	agentURI AgentURI,
	contract RegistryContract,
	admission AdmissionChecker,
	parentDID string,
) (RegistrationResult, error) {
	childPub := childKey.PublicKey()
	childAddress := childPub.EVMAddress()
	childCommon := common.BytesToAddress(childAddress.Bytes())

	// Validate agentURI completeness.
	valid, validationErrors := ValidateRegistrationCompleteness(agentURI, childPub)
	if !valid {
		return RegistrationResult{}, IncompleteRegistration{
			Detail: validationErrors[0].Error(),
		}
	}

	// Check admission policy.
	if admitted, err := admission.IsAdmitted(childAddress.Hex(), parentDID); !admitted {
		if err != nil {
			return RegistrationResult{}, err
		}
		return RegistrationResult{}, UnauthorizedOperation{
			CallerRole: "parent",
			Operation:  "register",
		}
	}

	// Check for duplicate registration.
	_, err := contract.TokenOfOwnerByIndex(ctx, childCommon, big.NewInt(0))
	if err == nil {
		// Token exists — duplicate registration.
		return RegistrationResult{}, DuplicateRegistration{
			ChildAddress:    childAddress.Hex(),
			RegistryAddress: registryAddr.Hex(),
		}
	}

	// Serialize agentURI to JSON.
	agentURIJSON, err := agentURI.ToJSON()
	if err != nil {
		return RegistrationResult{}, RegistryUnavailable{Detail: "failed to serialize agentURI: " + err.Error()}
	}

	// Call contract.Register().
	tokenId, txHash, err := contract.Register(ctx, childCommon, agentURIJSON)
	if err != nil {
		return RegistrationResult{}, RegistryUnavailable{Detail: err.Error()}
	}

	// Verify ownerOf matches childAddress.
	owner, err := contract.OwnerOf(ctx, tokenId)
	if err != nil {
		return RegistrationResult{}, RegistryUnavailable{Detail: "failed to verify ownership: " + err.Error()}
	}
	if owner != childCommon {
		return RegistrationResult{}, ProofOfControlFailed{
			Expected: childAddress.Hex(),
			Actual:   owner.Hex(),
		}
	}

	return RegistrationResult{
		tokenId:         tokenId,
		transactionHash: txHash,
		childAddress:    childAddress,
		registryAddress: registryAddr,
		chainId:         chainId,
		agentURIString:  agentURIJSON,
	}, nil
}

// UpdateRegistration updates the agentURI for an existing registration.
//
// Steps:
//  1. Derive caller address
//  2. Verify caller is owner or approved operator
//  3. Validate newAgentURI
//  4. Call contract.UpdateAgentURI()
//  5. Return RegistrationResult
func UpdateRegistration(
	ctx context.Context,
	childKey *keylib.NeuronPrivateKey,
	registryAddr keylib.EVMAddress,
	chainId uint64,
	tokenId *big.Int,
	newAgentURI AgentURI,
	contract RegistryContract,
) (RegistrationResult, error) {
	childPub := childKey.PublicKey()
	childAddress := childPub.EVMAddress()
	callerCommon := common.BytesToAddress(childAddress.Bytes())

	// Verify caller is owner or approved operator.
	owner, err := contract.OwnerOf(ctx, tokenId)
	if err != nil {
		return RegistrationResult{}, RegistrationNotFound{
			Detail: "token not found: " + err.Error(),
		}
	}

	if owner != callerCommon {
		// Check if caller is an approved operator.
		approved, err := contract.GetApproved(ctx, tokenId)
		if err != nil || approved != callerCommon {
			approvedForAll, err := contract.IsApprovedForAll(ctx, owner, callerCommon)
			if err != nil || !approvedForAll {
				return RegistrationResult{}, UnauthorizedOperation{
					CallerRole: "caller",
					Operation:  "updateAgentURI",
				}
			}
		}
	}

	// Validate newAgentURI completeness.
	valid, validationErrors := ValidateRegistrationCompleteness(newAgentURI, childPub)
	if !valid {
		return RegistrationResult{}, IncompleteRegistration{
			Detail: validationErrors[0].Error(),
		}
	}

	// Serialize and update.
	agentURIJSON, err := newAgentURI.ToJSON()
	if err != nil {
		return RegistrationResult{}, RegistryUnavailable{Detail: "failed to serialize agentURI: " + err.Error()}
	}

	txHash, err := contract.UpdateAgentURI(ctx, callerCommon, tokenId, agentURIJSON)
	if err != nil {
		return RegistrationResult{}, RegistryUnavailable{Detail: err.Error()}
	}

	return RegistrationResult{
		tokenId:         tokenId,
		transactionHash: txHash,
		childAddress:    childAddress,
		registryAddress: registryAddr,
		chainId:         chainId,
		agentURIString:  agentURIJSON,
	}, nil
}

// RevokeRegistration burns (destroys) a registration NFT, revoking the
// Child's registration in the registry.
//
// Steps:
//  1. Derive caller address
//  2. Verify caller is owner
//  3. Call contract.Burn()
//  4. Return transactionHash
func RevokeRegistration(
	ctx context.Context,
	childKey *keylib.NeuronPrivateKey,
	registryAddr keylib.EVMAddress,
	chainId uint64,
	tokenId *big.Int,
	contract RegistryContract,
) (string, error) {
	childAddress := childKey.PublicKey().EVMAddress()
	callerCommon := common.BytesToAddress(childAddress.Bytes())

	// Verify caller is owner.
	owner, err := contract.OwnerOf(ctx, tokenId)
	if err != nil {
		return "", RegistrationNotFound{
			Detail: "token not found: " + err.Error(),
		}
	}

	if owner != callerCommon {
		return "", UnauthorizedOperation{
			CallerRole: "caller",
			Operation:  "burn",
		}
	}

	txHash, err := contract.Burn(ctx, callerCommon, tokenId)
	if err != nil {
		return "", RegistryUnavailable{Detail: err.Error()}
	}

	return txHash, nil
}
