package registry

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// RegisterOutcome describes which idempotent path RegisterOrUpdate took.
type RegisterOutcome int

const (
	// OutcomeMinted means a fresh registration NFT was minted.
	OutcomeMinted RegisterOutcome = iota
	// OutcomeUpdated means an existing registration's AgentURI was refreshed.
	OutcomeUpdated
	// OutcomeReused means an existing registration was reused unchanged (no tx).
	OutcomeReused
)

func (o RegisterOutcome) String() string {
	switch o {
	case OutcomeMinted:
		return "minted"
	case OutcomeUpdated:
		return "updated"
	case OutcomeReused:
		return "reused"
	default:
		return "unknown"
	}
}

// RegisterOrUpdate is the idempotent registration entry point for long-lived
// agents that may already be registered — across restarts, or after a
// protocol-ID rename changed the AgentURI. It removes the operational
// requirement to manually revoke a stale registration before re-running.
//
// Behaviour:
//   - No existing registration -> mint (delegates to Register) -> OutcomeMinted.
//   - Existing registration whose on-chain AgentURI already matches -> reuse
//     untouched, no transaction -> OutcomeReused.
//   - Existing registration with a differing (stale) AgentURI -> refresh via
//     updateAgentURI (delegates to UpdateRegistration) -> OutcomeUpdated.
//
// All completeness/admission/proof-of-control checks run inside Register on the
// mint path; the recovery path is reached only after Register reports
// DuplicateRegistration, i.e. once those checks have already passed. Any other
// error (incomplete URI, admission denial, registry unavailable, proof-of-
// control) propagates unchanged — RegisterOrUpdate only recovers duplicates.
func RegisterOrUpdate(
	ctx context.Context,
	childKey *keylib.NeuronPrivateKey,
	registryAddr keylib.EVMAddress,
	chainId uint64,
	agentURI AgentURI,
	contract RegistryContract,
	admission AdmissionChecker,
	parentDID string,
) (RegistrationResult, RegisterOutcome, error) {
	result, err := Register(ctx, childKey, registryAddr, chainId, agentURI, contract, admission, parentDID)
	if err == nil {
		return result, OutcomeMinted, nil
	}

	var dup DuplicateRegistration
	if !errors.As(err, &dup) {
		// Not a recoverable duplicate — surface the original error.
		return RegistrationResult{}, OutcomeMinted, err
	}

	childCommon := common.BytesToAddress(childKey.PublicKey().EVMAddress().Bytes())

	// Resolve the existing registration's token (per FR-R05, index 0 is the
	// child's single token).
	tokenId, lookupErr := contract.TokenOfOwnerByIndex(ctx, childCommon, big.NewInt(0))
	if lookupErr != nil {
		return RegistrationResult{}, OutcomeMinted, RegistryUnavailable{
			Detail: "duplicate registration but token lookup failed: " + lookupErr.Error(),
		}
	}

	onChainURI, uriErr := contract.AgentURIOf(ctx, tokenId)
	if uriErr != nil {
		return RegistrationResult{}, OutcomeMinted, RegistryUnavailable{
			Detail: "duplicate registration but agentURI lookup failed: " + uriErr.Error(),
		}
	}

	newURI, marshalErr := agentURI.ToJSON()
	if marshalErr != nil {
		return RegistrationResult{}, OutcomeMinted, RegistryUnavailable{
			Detail: "failed to serialize agentURI: " + marshalErr.Error(),
		}
	}

	if onChainURI == newURI {
		// Already current — reuse without a transaction.
		return RegistrationResult{
			tokenId:         tokenId,
			transactionHash: "",
			childAddress:    childKey.PublicKey().EVMAddress(),
			registryAddress: registryAddr,
			chainId:         chainId,
			agentURIString:  onChainURI,
		}, OutcomeReused, nil
	}

	// Stale on-chain AgentURI (e.g. pre-rename) — refresh it in place.
	updated, updErr := UpdateRegistration(ctx, childKey, registryAddr, chainId, tokenId, agentURI, contract)
	if updErr != nil {
		return RegistrationResult{}, OutcomeMinted, updErr
	}
	return updated, OutcomeUpdated, nil
}
