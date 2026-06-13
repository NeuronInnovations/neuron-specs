package registry

import (
	"context"
	"errors"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// memoryToken is the in-memory representation of a registration NFT.
type memoryToken struct {
	owner    common.Address
	agentURI string
	approved common.Address
}

// MemoryRegistryContract is an in-memory RegistryContract suitable for unit
// tests, demo CLIs, and offline conformance runs. It mirrors NeuronIdentityRegistry
// semantics: register mints, agentURI is stored as a JSON string, tokenOfOwnerByIndex
// resolves owner→token, burn destroys.
//
// The mock honours msg.sender ownership in two ways. The default mode tracks
// the pendingOwner field, which a caller sets BEFORE calling Register; this
// matches the original test pattern where ownership is wired up by the test
// harness. Callers MAY also leave pendingOwner zero, in which case the
// signer common.Address argument passed into Register becomes the owner.
//
// All operations are safe for concurrent use.
type MemoryRegistryContract struct {
	mu             sync.Mutex
	tokens         map[int64]memoryToken
	nextId         int64
	ownerTokens    map[common.Address][]int64
	approvedForAll map[common.Address]map[common.Address]bool

	// pendingOwner — when set non-zero, the next Register() call assigns
	// ownership to this address rather than the signer argument. Tests set
	// this before invoking Register to simulate msg.sender.
	pendingOwner common.Address

	// Failure-injection knobs. Tests flip these to exercise error branches.
	failRegister bool
	failBurn     bool
	failOwnerOf  bool
	failAgentURI bool
	failTokenOf  bool
}

// Compile-time interface assertion.
var _ RegistryContract = (*MemoryRegistryContract)(nil)

// NewMemoryRegistryContract returns a fresh in-memory contract with no
// tokens, pendingOwner zero, and all failure knobs off.
func NewMemoryRegistryContract() *MemoryRegistryContract {
	return &MemoryRegistryContract{
		tokens:         make(map[int64]memoryToken),
		nextId:         1,
		ownerTokens:    make(map[common.Address][]int64),
		approvedForAll: make(map[common.Address]map[common.Address]bool),
	}
}

// SetPendingOwner stores the address that the next Register() call will use
// as msg.sender (overriding the signer argument). Used by tests that want
// to register on behalf of an arbitrary key without an EVM signer.
func (m *MemoryRegistryContract) SetPendingOwner(addr common.Address) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingOwner = addr
}

// SeedToken pre-populates a token for tests that need to assert lookup or
// burn behaviour without going through Register first.
func (m *MemoryRegistryContract) SeedToken(tokenId int64, owner common.Address, agentURI string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seedTokenLocked(tokenId, owner, agentURI)
}

// SetFailRegister toggles the failure-injection knob for Register/UpdateAgentURI.
func (m *MemoryRegistryContract) SetFailRegister(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failRegister = v
}

// SetFailBurn toggles the failure-injection knob for Burn.
func (m *MemoryRegistryContract) SetFailBurn(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failBurn = v
}

// SetFailOwnerOf toggles the failure-injection knob for OwnerOf.
func (m *MemoryRegistryContract) SetFailOwnerOf(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failOwnerOf = v
}

// SetFailAgentURI toggles the failure-injection knob for AgentURIOf.
func (m *MemoryRegistryContract) SetFailAgentURI(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failAgentURI = v
}

// SetFailTokenOf toggles the failure-injection knob for TokenOfOwnerByIndex.
func (m *MemoryRegistryContract) SetFailTokenOf(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failTokenOf = v
}

// seedTokenLocked is the unsynchronised seed helper used both by SeedToken
// and by the legacy lowercase setupToken alias below.
func (m *MemoryRegistryContract) seedTokenLocked(tokenId int64, owner common.Address, agentURI string) {
	m.tokens[tokenId] = memoryToken{owner: owner, agentURI: agentURI}
	m.ownerTokens[owner] = append(m.ownerTokens[owner], tokenId)
	if tokenId >= m.nextId {
		m.nextId = tokenId + 1
	}
}

// setupToken is the historical lowercase helper used by in-package tests.
// New callers should prefer SeedToken.
func (m *MemoryRegistryContract) setupToken(tokenId int64, owner common.Address, agentURI string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seedTokenLocked(tokenId, owner, agentURI)
}

// --- RegistryContract interface implementation ---

func (m *MemoryRegistryContract) Register(_ context.Context, signer common.Address, agentURI string) (*big.Int, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failRegister {
		return nil, "", errors.New("registry unavailable")
	}
	owner := m.pendingOwner
	if (owner == common.Address{}) {
		owner = signer
	}

	tokenId := m.nextId
	m.nextId++
	m.tokens[tokenId] = memoryToken{owner: owner, agentURI: agentURI}
	m.ownerTokens[owner] = append(m.ownerTokens[owner], tokenId)
	return big.NewInt(tokenId), "0xtxhash_register", nil
}

func (m *MemoryRegistryContract) UpdateAgentURI(_ context.Context, _ common.Address, tokenId *big.Int, agentURI string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failRegister {
		return "", errors.New("registry unavailable")
	}
	id := tokenId.Int64()
	tok, ok := m.tokens[id]
	if !ok {
		return "", errors.New("token not found")
	}
	tok.agentURI = agentURI
	m.tokens[id] = tok
	return "0xtxhash_update", nil
}

func (m *MemoryRegistryContract) OwnerOf(_ context.Context, tokenId *big.Int) (common.Address, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failOwnerOf {
		return common.Address{}, errors.New("owner lookup failed")
	}
	tok, ok := m.tokens[tokenId.Int64()]
	if !ok {
		return common.Address{}, errors.New("token not found")
	}
	return tok.owner, nil
}

func (m *MemoryRegistryContract) GetApproved(_ context.Context, tokenId *big.Int) (common.Address, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tok, ok := m.tokens[tokenId.Int64()]
	if !ok {
		return common.Address{}, errors.New("token not found")
	}
	return tok.approved, nil
}

func (m *MemoryRegistryContract) IsApprovedForAll(_ context.Context, owner, operator common.Address) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ops, ok := m.approvedForAll[owner]; ok {
		return ops[operator], nil
	}
	return false, nil
}

func (m *MemoryRegistryContract) TokenOfOwnerByIndex(_ context.Context, owner common.Address, index *big.Int) (*big.Int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failTokenOf {
		return nil, errors.New("token lookup failed")
	}
	tokens := m.ownerTokens[owner]
	idx := int(index.Int64())
	if idx >= len(tokens) {
		return nil, errors.New("index out of bounds")
	}
	return big.NewInt(tokens[idx]), nil
}

func (m *MemoryRegistryContract) AgentURIOf(_ context.Context, tokenId *big.Int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failAgentURI {
		return "", errors.New("agentURI lookup failed")
	}
	tok, ok := m.tokens[tokenId.Int64()]
	if !ok {
		return "", errors.New("token not found")
	}
	return tok.agentURI, nil
}

func (m *MemoryRegistryContract) Burn(_ context.Context, _ common.Address, tokenId *big.Int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failBurn {
		return "", errors.New("burn failed")
	}
	id := tokenId.Int64()
	tok, ok := m.tokens[id]
	if !ok {
		return "", errors.New("token not found")
	}
	owner := tok.owner
	tokens := m.ownerTokens[owner]
	for i, tid := range tokens {
		if tid == id {
			m.ownerTokens[owner] = append(tokens[:i], tokens[i+1:]...)
			break
		}
	}
	delete(m.tokens, id)
	return "0xtxhash_burn", nil
}
