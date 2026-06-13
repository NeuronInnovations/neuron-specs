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

// mockRegistryContract aliases MemoryRegistryContract (memory_contract.go) so
// the historical test surface keeps working without churn. New tests SHOULD
// reach for MemoryRegistryContract directly.
type mockRegistryContract = MemoryRegistryContract

// newMockContract is the historical factory; prefer NewMemoryRegistryContract
// in new code.
func newMockContract() *mockRegistryContract { return NewMemoryRegistryContract() }

// --- Helper to build a valid AgentURI for test key ---

func buildValidAgentURI(t *testing.T, childKey *keylib.NeuronPrivateKey) AgentURI {
	t.Helper()
	pub := childKey.PublicKey()
	peerID, err := pub.PeerID()
	require.NoError(t, err)

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

	uri, err := NewAgentURI(
		[]NeuronTopicService{stdIn, stdOut, stdErr},
		[]NeuronP2PExchangeService{p2p},
		&didSvc,
	)
	require.NoError(t, err)
	return uri
}

// --- Registration Tests (T019-T022) ---

func TestRegister_Success(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childAddr := childKey.PublicKey().EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	mock := newMockContract()
	mock.pendingOwner = childCommon

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	agentURI := buildValidAgentURI(t, &childKey)

	result, err := Register(context.Background(), &childKey, registryAddr, 1, agentURI, mock, PermissionlessPolicy{}, "")
	require.NoError(t, err)

	assert.NotNil(t, result.tokenId)
	assert.Equal(t, "0xtxhash_register", result.transactionHash)
	assert.Equal(t, childAddr, result.childAddress)
	assert.Equal(t, registryAddr, result.registryAddress)
	assert.Equal(t, uint64(1), result.chainId)
	assert.NotEmpty(t, result.agentURIString)
}

func TestRegister_IncompleteAgentURI(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	mock := newMockContract()
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	// Empty AgentURI (missing services).
	agentURI := AgentURI{}

	_, err = Register(context.Background(), &childKey, registryAddr, 1, agentURI, mock, PermissionlessPolicy{}, "")
	require.Error(t, err)

	var incomplete IncompleteRegistration
	assert.ErrorAs(t, err, &incomplete)
}

func TestRegister_DuplicateRegistration(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childAddr := childKey.PublicKey().EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	mock := newMockContract()
	// Pre-populate an existing registration for this child.
	mock.setupToken(1, childCommon, `{"services":[]}`)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	agentURI := buildValidAgentURI(t, &childKey)

	_, err = Register(context.Background(), &childKey, registryAddr, 1, agentURI, mock, PermissionlessPolicy{}, "")
	require.Error(t, err)

	var dup DuplicateRegistration
	assert.ErrorAs(t, err, &dup)
}

func TestRegister_RegistryUnavailable(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childCommon := common.BytesToAddress(childKey.PublicKey().EVMAddress().Bytes())

	mock := newMockContract()
	mock.pendingOwner = childCommon
	mock.failRegister = true

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	agentURI := buildValidAgentURI(t, &childKey)

	_, err = Register(context.Background(), &childKey, registryAddr, 1, agentURI, mock, PermissionlessPolicy{}, "")
	require.Error(t, err)

	var unavailable RegistryUnavailable
	assert.ErrorAs(t, err, &unavailable)
}

func TestRegister_ProofOfControl(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// Create a different owner to simulate proof-of-control failure.
	otherKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	otherCommon := common.BytesToAddress(otherKey.PublicKey().EVMAddress().Bytes())

	mock := newMockContract()
	// pendingOwner is different from childKey's address.
	mock.pendingOwner = otherCommon

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	agentURI := buildValidAgentURI(t, &childKey)

	_, err = Register(context.Background(), &childKey, registryAddr, 1, agentURI, mock, PermissionlessPolicy{}, "")
	require.Error(t, err)

	var poc ProofOfControlFailed
	assert.ErrorAs(t, err, &poc)
}

func TestUpdateRegistration_Success(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childAddr := childKey.PublicKey().EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	mock := newMockContract()
	agentURI := buildValidAgentURI(t, &childKey)
	agentURIJSON, err := agentURI.ToJSON()
	require.NoError(t, err)
	mock.setupToken(1, childCommon, agentURIJSON)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	result, err := UpdateRegistration(context.Background(), &childKey, registryAddr, 1,
		big.NewInt(1), agentURI, mock)
	require.NoError(t, err)

	assert.Equal(t, "0xtxhash_update", result.transactionHash)
	assert.Equal(t, big.NewInt(1), result.tokenId)
}

func TestUpdateRegistration_NonOwner(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	otherKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	otherCommon := common.BytesToAddress(otherKey.PublicKey().EVMAddress().Bytes())

	mock := newMockContract()
	// Token owned by other, not childKey.
	mock.setupToken(1, otherCommon, `{"services":[]}`)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	agentURI := buildValidAgentURI(t, &childKey)

	_, err = UpdateRegistration(context.Background(), &childKey, registryAddr, 1,
		big.NewInt(1), agentURI, mock)
	require.Error(t, err)

	var unauth UnauthorizedOperation
	assert.ErrorAs(t, err, &unauth)
}

func TestUpdateRegistration_InvalidAgentURI(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childAddr := childKey.PublicKey().EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	mock := newMockContract()
	mock.setupToken(1, childCommon, `{"services":[]}`)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	// Empty AgentURI — invalid.
	invalidURI := AgentURI{}

	_, err = UpdateRegistration(context.Background(), &childKey, registryAddr, 1,
		big.NewInt(1), invalidURI, mock)
	require.Error(t, err)

	var incomplete IncompleteRegistration
	assert.ErrorAs(t, err, &incomplete)
}

func TestRevokeRegistration_Success(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childAddr := childKey.PublicKey().EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	mock := newMockContract()
	mock.setupToken(1, childCommon, `{"services":[]}`)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	txHash, err := RevokeRegistration(context.Background(), &childKey, registryAddr, 1,
		big.NewInt(1), mock)
	require.NoError(t, err)

	assert.Equal(t, "0xtxhash_burn", txHash)

	// Verify token is gone.
	_, exists := mock.tokens[1]
	assert.False(t, exists)
}

func TestRevokeRegistration_NonOwner(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	otherKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	otherCommon := common.BytesToAddress(otherKey.PublicKey().EVMAddress().Bytes())

	mock := newMockContract()
	mock.setupToken(1, otherCommon, `{"services":[]}`)

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	_, err = RevokeRegistration(context.Background(), &childKey, registryAddr, 1,
		big.NewInt(1), mock)
	require.Error(t, err)

	var unauth UnauthorizedOperation
	assert.ErrorAs(t, err, &unauth)
}

func TestRevokeRegistration_NotFound(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	mock := newMockContract()
	// No tokens at all.

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	_, err = RevokeRegistration(context.Background(), &childKey, registryAddr, 1,
		big.NewInt(999), mock)
	require.Error(t, err)

	var notFound RegistrationNotFound
	assert.ErrorAs(t, err, &notFound)
}

// --- Admission Policy Tests ---

func TestRegister_AllowlistRejectsUnadmittedParent(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childCommon := common.BytesToAddress(childKey.PublicKey().EVMAddress().Bytes())

	mock := newMockContract()
	mock.pendingOwner = childCommon

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	agentURI := buildValidAgentURI(t, &childKey)

	// AllowlistPolicy with no entries — should reject.
	allowlist := NewAllowlistPolicy()

	_, err = Register(context.Background(), &childKey, registryAddr, 1, agentURI, mock, allowlist, "did:key:z6Mkunadmitted")
	require.Error(t, err)

	var rejection AllowlistRejection
	assert.ErrorAs(t, err, &rejection)
	assert.Equal(t, "did:key:z6Mkunadmitted", rejection.ParentDID)
}

func TestRegister_AllowlistAdmitsParent(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	childAddr := childKey.PublicKey().EVMAddress()
	childCommon := common.BytesToAddress(childAddr.Bytes())

	mock := newMockContract()
	mock.pendingOwner = childCommon

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	agentURI := buildValidAgentURI(t, &childKey)

	// AllowlistPolicy with the parent DID on the allowlist — should admit.
	allowlist := NewAllowlistPolicy()
	allowlist.AddParentDID("did:key:z6Mkadmitted")

	result, err := Register(context.Background(), &childKey, registryAddr, 1, agentURI, mock, allowlist, "did:key:z6Mkadmitted")
	require.NoError(t, err)

	assert.NotNil(t, result.tokenId)
	assert.Equal(t, "0xtxhash_register", result.transactionHash)
	assert.Equal(t, childAddr, result.childAddress)
}
