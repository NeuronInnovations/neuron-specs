package registry

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullLifecycle exercises the complete registration lifecycle:
// create child identity, build AgentURI, register, lookup, resolve topics,
// resolve p2p, update, verify update, revoke, verify not found.
func TestFullLifecycle(t *testing.T) {
	ctx := context.Background()

	// 1. Create child identity.
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pub := childKey.PublicKey()
	childAddr := pub.EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	peerID, err := pub.PeerID()
	require.NoError(t, err)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	chainId := uint64(1)

	// 2. Build AgentURI with 3 topic services + 1 p2p + DID.
	stdIn, err := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.1"})
	require.NoError(t, err)

	stdOut, err := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.2"})
	require.NoError(t, err)

	stdErr, err := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.3"})
	require.NoError(t, err)

	p2p, err := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(),
		"/neuron/multiaddr-exchange/1.0.0", "stdIn")
	require.NoError(t, err)

	didSvc, err := NewDIDService(pub)
	require.NoError(t, err)

	agentURI, err := NewAgentURI(
		[]NeuronTopicService{stdIn, stdOut, stdErr},
		[]NeuronP2PExchangeService{p2p},
		&didSvc,
	)
	require.NoError(t, err)

	// 3. Validate completeness.
	valid, validationErrors := ValidateRegistrationCompleteness(agentURI, pub)
	require.True(t, valid, "AgentURI should be valid, errors: %v", validationErrors)

	// 4. Register.
	mock := newMockContract()
	mock.pendingOwner = childCommon

	result, err := Register(ctx, &childKey, registryAddr, chainId, agentURI, mock, PermissionlessPolicy{}, "")
	require.NoError(t, err)

	assert.NotNil(t, result.tokenId)
	assert.Equal(t, childAddr, result.childAddress)
	assert.Equal(t, registryAddr, result.registryAddress)
	assert.Equal(t, chainId, result.chainId)
	tokenId := result.tokenId

	// 5. Lookup by EVM address.
	reg, err := LookupRegistration(ctx, registryAddr, chainId,
		ByEVMAddress(childAddr), mock)
	require.NoError(t, err)

	assert.Equal(t, childAddr, reg.childAddress)
	assert.Equal(t, tokenId, reg.tokenId)

	// 6. Resolve topics.
	si, so, se, err := ResolveTopics(reg)
	require.NoError(t, err)

	assert.Equal(t, "stdIn", si.Channel)
	assert.Equal(t, "stdOut", so.Channel)
	assert.Equal(t, "stdErr", se.Channel)

	// 7. Resolve P2P exchange.
	p2pResult, err := ResolveP2PExchange(reg)
	require.NoError(t, err)

	assert.Equal(t, peerID.String(), p2pResult.PeerID)
	assert.Equal(t, "stdIn", p2pResult.TopicRef)

	// 8. Update registration (change topic config).
	updatedStdIn, err := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.100"}) // Changed topicId.
	require.NoError(t, err)

	updatedURI, err := NewAgentURI(
		[]NeuronTopicService{updatedStdIn, stdOut, stdErr},
		[]NeuronP2PExchangeService{p2p},
		&didSvc,
	)
	require.NoError(t, err)

	updateResult, err := UpdateRegistration(ctx, &childKey, registryAddr, chainId,
		tokenId, updatedURI, mock)
	require.NoError(t, err)

	assert.Equal(t, tokenId, updateResult.tokenId)
	assert.NotEmpty(t, updateResult.transactionHash)

	// 9. Verify update by looking up again.
	regAfterUpdate, err := LookupRegistration(ctx, registryAddr, chainId,
		ByEVMAddress(childAddr), mock)
	require.NoError(t, err)

	siUpdated, _, _, err := ResolveTopics(regAfterUpdate)
	require.NoError(t, err)
	assert.Equal(t, "0.0.100", siUpdated.Config["topicId"])

	// 10. Revoke registration.
	txHash, err := RevokeRegistration(ctx, &childKey, registryAddr, chainId,
		tokenId, mock)
	require.NoError(t, err)
	assert.NotEmpty(t, txHash)

	// 11. Verify not found after revocation.
	_, err = LookupRegistration(ctx, registryAddr, chainId,
		ByEVMAddress(childAddr), mock)
	require.Error(t, err)

	var notFound RegistrationNotFound
	assert.ErrorAs(t, err, &notFound)
}

// TestFullLifecycle_WithMixedTransports verifies the lifecycle works with
// different transports per channel.
func TestFullLifecycle_WithMixedTransports(t *testing.T) {
	ctx := context.Background()

	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pub := childKey.PublicKey()
	childAddr := pub.EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	peerID, err := pub.PeerID()
	require.NoError(t, err)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	// Mixed transports.
	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "kafka", "kafka-cluster",
		map[string]any{"broker": "kafka:9092"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "libp2p", "libp2p-net",
		map[string]any{"relay": true})
	p2p, _ := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(),
		"/neuron/multiaddr-exchange/1.0.0", "stdIn")
	didSvc, _ := NewDIDService(pub)

	uri, err := NewAgentURI(
		[]NeuronTopicService{stdIn, stdOut, stdErr},
		[]NeuronP2PExchangeService{p2p},
		&didSvc,
	)
	require.NoError(t, err)

	mock := newMockContract()
	mock.pendingOwner = childCommon

	result, err := Register(ctx, &childKey, registryAddr, 1, uri, mock, PermissionlessPolicy{}, "")
	require.NoError(t, err)

	reg, err := LookupRegistration(ctx, registryAddr, 1,
		ByEVMAddress(childAddr), mock)
	require.NoError(t, err)

	si, so, se, err := ResolveTopics(reg)
	require.NoError(t, err)

	assert.Equal(t, "hcs", si.Transport)
	assert.Equal(t, "kafka", so.Transport)
	assert.Equal(t, "libp2p", se.Transport)

	// Cleanup.
	_, err = RevokeRegistration(ctx, &childKey, registryAddr, 1,
		result.tokenId, mock)
	require.NoError(t, err)
}

// TestFullLifecycle_NoDIDService verifies the lifecycle works without a DID service.
func TestFullLifecycle_NoDIDService(t *testing.T) {
	ctx := context.Background()

	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pub := childKey.PublicKey()
	childAddr := pub.EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	peerID, err := pub.PeerID()
	require.NoError(t, err)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera", map[string]any{"t": "3"})
	p2p, _ := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(), "/proto", "stdIn")

	// No DID service.
	uri, err := NewAgentURI(
		[]NeuronTopicService{stdIn, stdOut, stdErr},
		[]NeuronP2PExchangeService{p2p},
		nil,
	)
	require.NoError(t, err)

	mock := newMockContract()
	mock.pendingOwner = childCommon

	result, err := Register(ctx, &childKey, registryAddr, 1, uri, mock, PermissionlessPolicy{}, "")
	require.NoError(t, err)
	assert.NotNil(t, result.tokenId)

	reg, err := LookupRegistration(ctx, registryAddr, 1,
		ByEVMAddress(childAddr), mock)
	require.NoError(t, err)
	assert.Empty(t, reg.agentURI.didServices)
}
