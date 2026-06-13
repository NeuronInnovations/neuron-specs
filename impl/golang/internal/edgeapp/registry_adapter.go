package edgeapp

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
)

// RegistryAdapter is the edgeapp-friendly facade over EIP-8004 Identity
// Registry operations. It is intentionally narrower than
// internal/registry.RegistryContract — the edge demo only needs:
//
//   - lookup-by-owner (so the seller can check whether it's already registered
//     without knowing its tokenId in advance)
//   - register a new agent with a given agentURI
//   - update the agentURI for an existing tokenId (when the descriptor or
//     topic IDs have changed)
//
// Two implementations ship with the package:
//
//   - MemoryRegistry — in-process map keyed by EVM address; for unit tests
//     and mock-mode demos. Default everywhere; never fires a tx.
//   - EVMRegistryAdapter — wraps internal/registry.RegistryContract. Disabled
//     by default (every call returns ErrFeatureNotImplemented) and activates
//     only when the operator explicitly opts in via NEURON_EDGE_REGISTRATION_MODE
//     equal to "force-testnet" / "force-evm". This satisfies the project rule
//     "no testnet transactions unless explicitly approved".
type RegistryAdapter interface {
	// LookupTokenID returns (tokenId, found=true, nil) when the given owner
	// has an existing registration. (nil, false, nil) means no registration.
	// A non-nil error indicates the lookup itself failed (e.g. RPC down).
	LookupTokenID(ctx context.Context, ownerEVM string) (*big.Int, bool, error)

	// AgentURIByTokenID returns the agentURI string recorded for a token.
	AgentURIByTokenID(ctx context.Context, tokenID *big.Int) (string, error)

	// Register mints a new registration NFT for ownerEVM with the given
	// agentURI. Returns (tokenId, transactionRef, error).
	Register(ctx context.Context, ownerEVM string, agentURI string) (*big.Int, string, error)

	// UpdateAgentURI replaces the agentURI for an existing tokenId. Returns
	// transactionRef on success.
	UpdateAgentURI(ctx context.Context, ownerEVM string, tokenID *big.Int, agentURI string) (string, error)
}

// MemoryRegistry is an in-process RegistryAdapter — registrations live in
// a map keyed by lowercased EVM address. Tokens are sequentially-allocated
// big.Int counters starting at 1. Thread-safe.
type MemoryRegistry struct {
	mu        sync.Mutex
	nextToken int64
	byOwner   map[string]*big.Int // ownerEVM (lowercased) → tokenID
	byToken   map[string]string   // tokenID.String() → agentURI
}

// NewMemoryRegistry constructs an empty MemoryRegistry.
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		byOwner: make(map[string]*big.Int),
		byToken: make(map[string]string),
	}
}

// LookupTokenID implements RegistryAdapter.
func (m *MemoryRegistry) LookupTokenID(_ context.Context, ownerEVM string) (*big.Int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byOwner[normalizeEVM(ownerEVM)]
	if !ok {
		return nil, false, nil
	}
	return new(big.Int).Set(t), true, nil
}

// AgentURIByTokenID implements RegistryAdapter.
func (m *MemoryRegistry) AgentURIByTokenID(_ context.Context, tokenID *big.Int) (string, error) {
	if tokenID == nil {
		return "", errors.New("memory registry: nil tokenID")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	uri, ok := m.byToken[tokenID.String()]
	if !ok {
		return "", fmt.Errorf("memory registry: token %s not found", tokenID.String())
	}
	return uri, nil
}

// Register implements RegistryAdapter.
func (m *MemoryRegistry) Register(_ context.Context, ownerEVM string, agentURI string) (*big.Int, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	owner := normalizeEVM(ownerEVM)
	if _, dup := m.byOwner[owner]; dup {
		// Per FR-R05 (single registration per owner) — return a typed error
		// so EnsureRegistered's idempotent path can branch.
		return nil, "", ErrAlreadyRegistered
	}
	m.nextToken++
	tokenID := big.NewInt(m.nextToken)
	m.byOwner[owner] = tokenID
	m.byToken[tokenID.String()] = agentURI
	txRef := fmt.Sprintf("memory-register-%d", m.nextToken)
	return new(big.Int).Set(tokenID), txRef, nil
}

// UpdateAgentURI implements RegistryAdapter.
func (m *MemoryRegistry) UpdateAgentURI(_ context.Context, _ string, tokenID *big.Int, agentURI string) (string, error) {
	if tokenID == nil {
		return "", errors.New("memory registry: nil tokenID")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byToken[tokenID.String()]; !ok {
		return "", fmt.Errorf("memory registry: token %s not found", tokenID.String())
	}
	m.byToken[tokenID.String()] = agentURI
	return fmt.Sprintf("memory-update-%s", tokenID.String()), nil
}

// ErrAlreadyRegistered is returned by Register when the owner already holds
// a registration. Callers (EnsureRegistered) treat this as a soft signal to
// fall back to lookup + (optionally) update.
var ErrAlreadyRegistered = errors.New("registry: owner already holds a registration")

// disabledRegistry is a RegistryAdapter whose every method returns
// ErrFeatureNotImplemented. Used as a sentinel when force-testnet is OFF
// but the caller still asked for an EVM-shaped adapter — keeps the
// dispatch table simple at the integration_stubs.go layer.
type disabledRegistry struct{ reason string }

func (d disabledRegistry) LookupTokenID(context.Context, string) (*big.Int, bool, error) {
	return nil, false, fmt.Errorf("%w (registry %s)", ErrFeatureNotImplemented, d.reason)
}
func (d disabledRegistry) AgentURIByTokenID(context.Context, *big.Int) (string, error) {
	return "", fmt.Errorf("%w (registry %s)", ErrFeatureNotImplemented, d.reason)
}
func (d disabledRegistry) Register(context.Context, string, string) (*big.Int, string, error) {
	return nil, "", fmt.Errorf("%w (registry %s)", ErrFeatureNotImplemented, d.reason)
}
func (d disabledRegistry) UpdateAgentURI(context.Context, string, *big.Int, string) (string, error) {
	return "", fmt.Errorf("%w (registry %s)", ErrFeatureNotImplemented, d.reason)
}

// NewDisabledRegistry returns a RegistryAdapter that errors on every call
// with ErrFeatureNotImplemented, naming the reason in the message. Useful
// when integration_stubs.go's testnet branch is opt-out by default and
// callers need a concrete adapter to pass through.
func NewDisabledRegistry(reason string) RegistryAdapter {
	if reason == "" {
		reason = "disabled"
	}
	return disabledRegistry{reason: reason}
}

// normalizeEVM returns a lowercased, "0x"-prefixed-or-not-prefix-tolerant
// version of an EVM address suitable as a map key. Behavior:
//
//   - lowercase for case-insensitive matching against EIP-55-mixed-case input
//   - any leading "0x" / "0X" is preserved when present (so "0xabc" stays
//     "0xabc" rather than becoming "abc"); inputs without the prefix are
//     left as-is
//
// The contract is: two callers passing the same address (in any case-mix)
// must hash to the same key. This is the minimum needed for MemoryRegistry's
// dedupe; the EVM adapter has its own normalization.
func normalizeEVM(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
