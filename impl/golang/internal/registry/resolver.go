package registry

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// LookupKeyType identifies the variant of a LookupKey.
type LookupKeyType int

const (
	// LookupByEVMAddress looks up a registration by the Child's EVM address.
	LookupByEVMAddress LookupKeyType = iota

	// LookupByExternalID looks up a registration by an external identifier
	// (e.g., a registry binding from the account module).
	LookupByExternalID
)

// LookupKey is a sum type for registration lookup. Use ByEVMAddress() or
// ByExternalID() constructors.
type LookupKey struct {
	keyType    LookupKeyType
	evmAddress keylib.EVMAddress
	externalID string
}

// ByEVMAddress creates a LookupKey that searches by EVM address.
func ByEVMAddress(addr keylib.EVMAddress) LookupKey {
	return LookupKey{keyType: LookupByEVMAddress, evmAddress: addr}
}

// ByExternalID creates a LookupKey that searches by external identifier.
func ByExternalID(id string) LookupKey {
	return LookupKey{keyType: LookupByExternalID, externalID: id}
}

// Type returns the lookup key variant.
func (k LookupKey) Type() LookupKeyType {
	return k.keyType
}

// EVMAddress returns the EVM address for an EVM address lookup.
// Returns the zero value if this is not a ByEVMAddress key.
func (k LookupKey) EVMAddress() keylib.EVMAddress {
	return k.evmAddress
}

// ExternalID returns the external identifier for an external ID lookup.
// Returns empty string if this is not a ByExternalID key.
func (k LookupKey) ExternalID() string {
	return k.externalID
}

// LookupRegistration resolves a registration from the registry contract
// using the provided LookupKey.
//
// For ByEVMAddress: calls tokenOfOwnerByIndex(addr, 0) to get the tokenId,
// then agentURIOf(tokenId) to get the agentURI JSON, and parses it.
//
// For ByExternalID: not yet resolvable (returns RegistrationNotFound).
func LookupRegistration(
	ctx context.Context,
	registryAddr keylib.EVMAddress,
	chainId uint64,
	lookupKey LookupKey,
	contract RegistryContract,
) (Registration, error) {
	switch lookupKey.Type() {
	case LookupByEVMAddress:
		addr := lookupKey.EVMAddress()
		addrCommon := common.BytesToAddress(addr.Bytes())

		tokenId, err := contract.TokenOfOwnerByIndex(ctx, addrCommon, big.NewInt(0))
		if err != nil {
			return Registration{}, RegistrationNotFound{
				Detail: addr.Hex(),
			}
		}

		agentURIStr, err := contract.AgentURIOf(ctx, tokenId)
		if err != nil {
			return Registration{}, RegistryUnavailable{
				Detail: "failed to read agentURI: " + err.Error(),
			}
		}

		agentURI, err := AgentURIFromJSON(agentURIStr)
		if err != nil {
			return Registration{}, RegistryUnavailable{
				Detail: "failed to parse agentURI JSON: " + err.Error(),
			}
		}

		return Registration{
			registryAddress: registryAddr,
			childAddress:    addr,
			tokenId:         tokenId,
			agentURI:        agentURI,
			chainId:         chainId,
		}, nil

	case LookupByExternalID:
		// FR-R05, FR-X04: externalID maps to the EIP-8004 tokenId (agentId).
		// Parse as *big.Int, then resolve via AgentURIOf and OwnerOf.
		tokenId := new(big.Int)
		if _, ok := tokenId.SetString(lookupKey.ExternalID(), 10); !ok {
			return Registration{}, RegistrationNotFound{
				Detail: "invalid externalID (not a decimal integer): " + lookupKey.ExternalID(),
			}
		}

		// Resolve agentURI from tokenId.
		agentURIStr, err := contract.AgentURIOf(ctx, tokenId)
		if err != nil {
			return Registration{}, RegistrationNotFound{
				Detail: lookupKey.ExternalID(),
			}
		}

		agentURI, err := AgentURIFromJSON(agentURIStr)
		if err != nil {
			return Registration{}, RegistryUnavailable{
				Detail: "failed to parse agentURI JSON: " + err.Error(),
			}
		}

		// Resolve owner address from tokenId.
		ownerCommon, err := contract.OwnerOf(ctx, tokenId)
		if err != nil {
			return Registration{}, RegistryUnavailable{
				Detail: "failed to resolve owner for tokenId: " + err.Error(),
			}
		}

		ownerAddr, err := keylib.EVMAddressFromHex(ownerCommon.Hex())
		if err != nil {
			return Registration{}, RegistryUnavailable{
				Detail: "failed to parse owner address: " + err.Error(),
			}
		}

		return Registration{
			registryAddress: registryAddr,
			childAddress:    ownerAddr,
			tokenId:         tokenId,
			agentURI:        agentURI,
			chainId:         chainId,
		}, nil

	default:
		return Registration{}, RegistrationNotFound{
			Detail: "unsupported lookup key type",
		}
	}
}

// ResolveTopics extracts the three standard channel topic services (stdIn,
// stdOut, stdErr) from a registration. Returns IncompleteRegistration if any
// standard channel is missing.
func ResolveTopics(registration Registration) (stdIn, stdOut, stdErr NeuronTopicService, err error) {
	channels := make(map[string]NeuronTopicService)
	for _, svc := range registration.agentURI.topicServices {
		channels[svc.Channel] = svc
	}

	var missing []string
	if svc, ok := channels["stdIn"]; ok {
		stdIn = svc
	} else {
		missing = append(missing, "stdIn")
	}
	if svc, ok := channels["stdOut"]; ok {
		stdOut = svc
	} else {
		missing = append(missing, "stdOut")
	}
	if svc, ok := channels["stdErr"]; ok {
		stdErr = svc
	} else {
		missing = append(missing, "stdErr")
	}

	if len(missing) > 0 {
		return NeuronTopicService{}, NeuronTopicService{}, NeuronTopicService{},
			IncompleteRegistration{
				Detail: "missing standard channel(s): " + joinStrings(missing),
			}
	}

	return stdIn, stdOut, stdErr, nil
}

// ResolveP2PExchange extracts the first neuron-p2p-exchange service from a
// registration and validates that its topicRef references an existing topic
// service name. Returns IncompleteRegistration if no P2P service exists,
// or BrokenTopicRef if the topicRef is invalid.
func ResolveP2PExchange(registration Registration) (NeuronP2PExchangeService, error) {
	if len(registration.agentURI.p2pServices) == 0 {
		return NeuronP2PExchangeService{}, IncompleteRegistration{
			Detail: "no neuron-p2p-exchange service found",
		}
	}

	p2p := registration.agentURI.p2pServices[0]

	// Validate topicRef against topic service names.
	topicNames := make([]string, 0, len(registration.agentURI.topicServices))
	found := false
	for _, svc := range registration.agentURI.topicServices {
		topicNames = append(topicNames, svc.Name)
		if svc.Name == p2p.TopicRef {
			found = true
		}
	}

	if !found {
		return NeuronP2PExchangeService{}, BrokenTopicRef{
			TopicRef:       p2p.TopicRef,
			AvailableNames: topicNames,
		}
	}

	return p2p, nil
}

// joinStrings joins strings with ", " separator.
func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
