package registry

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- LookupRegistration Tests (T030-T031, T039) ---

func TestLookupRegistration_ByEVMAddress(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childAddr := childKey.PublicKey().EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	agentURI := buildValidAgentURI(t, &childKey)
	agentURIJSON, err := agentURI.ToJSON()
	require.NoError(t, err)

	mock := newMockContract()
	mock.setupToken(42, childCommon, agentURIJSON)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	reg, err := LookupRegistration(context.Background(), registryAddr, 1,
		ByEVMAddress(childAddr), mock)
	require.NoError(t, err)

	assert.Equal(t, registryAddr, reg.registryAddress)
	assert.Equal(t, childAddr, reg.childAddress)
	assert.Equal(t, big.NewInt(42), reg.tokenId)
	assert.Equal(t, uint64(1), reg.chainId)
	assert.Len(t, reg.agentURI.topicServices, 3)
	assert.Len(t, reg.agentURI.p2pServices, 1)
}

func TestLookupRegistration_NotFound(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	childAddr := childKey.PublicKey().EVMAddress()

	mock := newMockContract()
	// No tokens registered.

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	_, err = LookupRegistration(context.Background(), registryAddr, 1,
		ByEVMAddress(childAddr), mock)
	require.Error(t, err)

	var notFound RegistrationNotFound
	assert.ErrorAs(t, err, &notFound)
}

func TestLookupRegistration_Unavailable(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childAddr := childKey.PublicKey().EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	mock := newMockContract()
	mock.setupToken(1, childCommon, `{"services":[]}`)
	mock.failAgentURI = true

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	_, err = LookupRegistration(context.Background(), registryAddr, 1,
		ByEVMAddress(childAddr), mock)
	require.Error(t, err)

	var unavailable RegistryUnavailable
	assert.ErrorAs(t, err, &unavailable)
}

// --- FR-R05, FR-X04: LookupByExternalID Tests ---

func TestLookupRegistration_ByExternalID_Success(t *testing.T) {
	// FR-R05: externalID maps to EIP-8004 tokenId.
	// Setup: create a token with ID=42 and valid agentURI.
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childAddr := childKey.PublicKey().EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	agentURI := buildValidAgentURI(t, &childKey)
	agentURIJSON, err := agentURI.ToJSON()
	require.NoError(t, err)

	mock := newMockContract()
	mock.setupToken(42, childCommon, agentURIJSON)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	// Lookup by externalID "42" (the tokenId as decimal string).
	reg, err := LookupRegistration(context.Background(), registryAddr, 1,
		ByExternalID("42"), mock)
	require.NoError(t, err)

	assert.Equal(t, registryAddr, reg.registryAddress)
	assert.Equal(t, childAddr, reg.childAddress)
	assert.Equal(t, big.NewInt(42), reg.tokenId)
	assert.Equal(t, uint64(1), reg.chainId)
	assert.Len(t, reg.agentURI.topicServices, 3)
	assert.Len(t, reg.agentURI.p2pServices, 1)
}

func TestLookupRegistration_ByExternalID_NotFound(t *testing.T) {
	mock := newMockContract()

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	// Token 999 does not exist.
	_, err = LookupRegistration(context.Background(), registryAddr, 1,
		ByExternalID("999"), mock)
	require.Error(t, err)

	var notFound RegistrationNotFound
	assert.ErrorAs(t, err, &notFound)
}

func TestLookupRegistration_ByExternalID_InvalidFormat(t *testing.T) {
	mock := newMockContract()

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	// Non-numeric externalID must be rejected.
	_, err = LookupRegistration(context.Background(), registryAddr, 1,
		ByExternalID("not-a-number"), mock)
	require.Error(t, err)

	var notFound RegistrationNotFound
	assert.ErrorAs(t, err, &notFound)
	assert.Contains(t, notFound.Detail, "invalid externalID")
}

// --- ResolveTopics Tests ---

func TestResolveTopics_Success(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pub := childKey.PublicKey()
	uri := validAgentURIForKey(t, pub)

	reg := Registration{
		agentURI: uri,
	}

	stdIn, stdOut, stdErr, err := ResolveTopics(reg)
	require.NoError(t, err)

	assert.Equal(t, "stdIn", stdIn.Channel)
	assert.Equal(t, "stdOut", stdOut.Channel)
	assert.Equal(t, "stdErr", stdErr.Channel)
}

func TestResolveTopics_MissingChannel(t *testing.T) {
	// Registration with only 2 topic services (missing stdErr).
	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})

	reg := Registration{
		agentURI: AgentURI{
			topicServices: []NeuronTopicService{stdIn, stdOut},
		},
	}

	_, _, _, err := ResolveTopics(reg)
	require.Error(t, err)

	var incomplete IncompleteRegistration
	assert.ErrorAs(t, err, &incomplete)
	assert.Contains(t, incomplete.Detail, "stdErr")
}

func TestResolveTopics_MixedTransports(t *testing.T) {
	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "kafka", "kafka-cluster", map[string]any{"broker": "kafka:9092"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "libp2p", "libp2p-net", map[string]any{"relay": true})

	reg := Registration{
		agentURI: AgentURI{
			topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
		},
	}

	si, so, se, err := ResolveTopics(reg)
	require.NoError(t, err)

	assert.Equal(t, "hcs", si.Transport)
	assert.Equal(t, "kafka", so.Transport)
	assert.Equal(t, "libp2p", se.Transport)
}

// --- ResolveP2PExchange Tests ---

func TestResolveP2PExchange_Success(t *testing.T) {
	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera", map[string]any{"t": "3"})
	p2p, _ := NewNeuronP2PExchangeService("p2p", "1.0.0", "12D3KooWTest", "/proto", "stdIn")

	reg := Registration{
		agentURI: AgentURI{
			topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
			p2pServices:   []NeuronP2PExchangeService{p2p},
		},
	}

	result, err := ResolveP2PExchange(reg)
	require.NoError(t, err)

	assert.Equal(t, "p2p", result.Name)
	assert.Equal(t, "stdIn", result.TopicRef)
	assert.Equal(t, "12D3KooWTest", result.PeerID)
}

func TestResolveP2PExchange_Missing(t *testing.T) {
	reg := Registration{
		agentURI: AgentURI{
			topicServices: []NeuronTopicService{
				{Type: ServiceTypeNeuronTopic, Name: "stdIn", Channel: "stdIn"},
			},
			p2pServices: nil,
		},
	}

	_, err := ResolveP2PExchange(reg)
	require.Error(t, err)

	var incomplete IncompleteRegistration
	assert.ErrorAs(t, err, &incomplete)
}

func TestResolveP2PExchange_BrokenTopicRef(t *testing.T) {
	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})

	p2p := NeuronP2PExchangeService{
		Type:     ServiceTypeNeuronP2PExchange,
		Name:     "p2p",
		Version:  "1.0.0",
		PeerID:   "12D3KooWTest",
		Protocol: "/proto",
		TopicRef: "nonExistent", // No matching topic.
	}

	reg := Registration{
		agentURI: AgentURI{
			topicServices: []NeuronTopicService{stdIn},
			p2pServices:   []NeuronP2PExchangeService{p2p},
		},
	}

	_, err := ResolveP2PExchange(reg)
	require.Error(t, err)

	var broken BrokenTopicRef
	assert.ErrorAs(t, err, &broken)
	assert.Equal(t, "nonExistent", broken.TopicRef)
	assert.Contains(t, broken.AvailableNames, "stdIn")
}
