package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// memoryConfig is the resolved SIM default — what resolveRegistryConfig
// returns with no evm flags.
func memoryConfig() registryConfig {
	return registryConfig{backend: registryBackendMemory, simulated: true}
}

// memoryContractFactory returns a ContractFactory that hands back one shared
// MemoryRegistryContract, pinning the pending owner to the signer's address —
// the same trick cmd/remoteid-seller's tests use so the evm code path runs
// without an RPC.
func memoryContractFactory(shared *registry.MemoryRegistryContract) func(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error) {
	return func(_ context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error) {
		shared.SetPendingOwner(ethcrypto.PubkeyToAddress(opts.SignerKey.PublicKey))
		return shared, nil
	}
}

// TestKeyHexOrEnv: the systemd EnvironmentFile path — NEURON_KEY_HEX supplies
// the key when --key-hex is absent so the secret never appears in argv; an
// explicit flag always wins; both empty keeps the ephemeral default.
func TestKeyHexOrEnv(t *testing.T) {
	t.Setenv("NEURON_KEY_HEX", "aa11")
	require.Equal(t, "ff22", keyHexOrEnv("ff22"), "explicit flag wins over env")
	require.Equal(t, "aa11", keyHexOrEnv(""), "env fallback when flag empty")
	t.Setenv("NEURON_KEY_HEX", "")
	require.Empty(t, keyHexOrEnv(""), "both empty -> ephemeral path")
}

func sellerTestKey(t *testing.T) (*keylib.NeuronPrivateKey, string) {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pid, err := k.PublicKey().PeerID()
	require.NoError(t, err)
	return &k, pid.String()
}

// TestRegisterAgentCard_WritesEvidence proves evidence mode writes a valid card
// (the agentURI) and an evidence record that binds agentId ↔ EVM ↔ node_id ↔
// PeerID. The default seller path (no flags) never calls this.
func TestRegisterAgentCard_WritesEvidence(t *testing.T) {
	nk, peerID := sellerTestKey(t)
	dir := t.TempDir()
	cardOut := filepath.Join(dir, "card.json")
	regOut := filepath.Join(dir, "evidence.json")
	logger := log.New(io.Discard, "", 0)

	_, raErr := registerAgentCard(context.Background(), nk, peerID, cardOut, regOut, "synthetic", memoryConfig(), Deps{}, logger, nil, "", nil)
	require.NoError(t, raErr)

	// Card file is the agentURI; it parses and passes the registry validator.
	cardBytes, err := os.ReadFile(cardOut)
	require.NoError(t, err)
	uri, err := registry.AgentURIFromJSON(string(cardBytes))
	require.NoError(t, err)
	valid, vErrs := registry.ValidateRegistrationCompleteness(uri, nk.PublicKey())
	require.True(t, valid, "written card must validate: %v", vErrs)

	// Evidence record binds the identity triple + service/protocol.
	ev, err := sapient.ReadEvidence(regOut)
	require.NoError(t, err)
	require.Equal(t, peerID, ev.PeerID)
	require.Equal(t, nk.PublicKey().EVMAddress().Hex(), ev.SellerEVM)
	require.Equal(t, sapient.NodeIDFromIdentity(nk.PublicKey().EVMAddress().Hex()), ev.NodeID)
	require.Equal(t, "rid", ev.Service)
	require.Equal(t, sapient.ProtocolDetection, ev.Protocol)
	require.True(t, ev.Simulated, "memory-contract registration is labelled simulated")
	require.NotEmpty(t, ev.AgentID, "minted an agent id")
	require.Equal(t, "synthetic", ev.FeedSource, "feedSource provenance recorded in the evidence")
}

func TestRegisterAgentCard_PeerIDMismatch(t *testing.T) {
	nk, _ := sellerTestKey(t)
	_, err := registerAgentCard(context.Background(), nk, "12D3KooWBogusPeerIdThatDoesNotMatch", "", "", "synthetic", memoryConfig(), Deps{}, log.New(io.Discard, "", 0), nil, "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "identity mismatch")
}

// --- registry backend config validation (Layer 1: real-EVM EIP-8004) ---

func TestRegistryConfig_DefaultsToMemoryUnchanged(t *testing.T) {
	cfg, err := resolveRegistryConfig(registryBackendMemory, "", "", defaultChainID, func(string) string { return "" })
	require.NoError(t, err)
	require.Equal(t, registryBackendMemory, cfg.backend)
	require.True(t, cfg.simulated, "memory backend stays the simulated SIM default")
	require.Zero(t, cfg.chainID, "memory backend keeps the chainId=0 local label")
}

func TestRegistryConfig_EVMRequiresAddress(t *testing.T) {
	_, err := resolveRegistryConfig(registryBackendEVM, "", "", defaultChainID, func(string) string { return "" })
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires --registry-address")
}

func TestRegistryConfig_EVMEnvFallback(t *testing.T) {
	const addr = "0x5d9B1fE5eB02173205AEe8DC4f72db15bFB5f73C"
	lookup := func(key string) string {
		if key == envRegistryContract {
			return addr
		}
		return ""
	}
	cfg, err := resolveRegistryConfig(registryBackendEVM, "", "", defaultChainID, lookup)
	require.NoError(t, err)
	require.Equal(t, addr, cfg.addr.Hex(), "address comes from env when flag absent")
	require.Equal(t, defaultRPCURL, cfg.rpc, "rpc falls back to the hashio default")
	require.Equal(t, defaultChainID, cfg.chainID)
	require.False(t, cfg.simulated, "evm backend is the real thing")
}

func TestRegistryConfig_EVMInvalidAddress(t *testing.T) {
	_, err := resolveRegistryConfig(registryBackendEVM, "not-an-address", "", defaultChainID, func(string) string { return "" })
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid --registry-address")
}

func TestRegistryConfig_EVMChainIDZeroRejected(t *testing.T) {
	_, err := resolveRegistryConfig(registryBackendEVM, "0x5d9B1fE5eB02173205AEe8DC4f72db15bFB5f73C", "", 0, func(string) string { return "" })
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-zero")
}

func TestRegistryConfig_UnknownBackendRejected(t *testing.T) {
	_, err := resolveRegistryConfig("potato", "", "", defaultChainID, func(string) string { return "" })
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown --registry-backend")
}

// TestRegisterAgentCard_EVMPath_EvidenceSimulatedFalse drives the evm code
// path through the ContractFactory seam (memory contract standing in for the
// chain) and proves the evidence flips to simulated:false with the real
// registry coordinates — the only thing the FID needs to show ON-CHAIN.
func TestRegisterAgentCard_EVMPath_EvidenceSimulatedFalse(t *testing.T) {
	const regHex = "0x5d9B1fE5eB02173205AEe8DC4f72db15bFB5f73C"
	nk, peerID := sellerTestKey(t)
	dir := t.TempDir()
	regOut := filepath.Join(dir, "evidence.json")

	cfg, err := resolveRegistryConfig(registryBackendEVM, regHex, "", defaultChainID, func(string) string { return "" })
	require.NoError(t, err)
	deps := Deps{ContractFactory: memoryContractFactory(registry.NewMemoryRegistryContract())}

	_, raErr := registerAgentCard(context.Background(), nk, peerID, "", regOut, "placeholder", cfg, deps, log.New(io.Discard, "", 0), nil, "", nil)
	require.NoError(t, raErr)

	ev, err := sapient.ReadEvidence(regOut)
	require.NoError(t, err)
	require.False(t, ev.Simulated, "evm-backend evidence must be labelled simulated:false")
	require.Equal(t, defaultChainID, ev.ChainID)
	require.Equal(t, regHex, ev.RegistryAddress)
	require.Equal(t, "minted", ev.Outcome)
	require.Equal(t, peerID, ev.PeerID)
	require.Equal(t, nk.PublicKey().EVMAddress().Hex(), ev.SellerEVM)
	require.Equal(t, sapient.NodeIDFromIdentity(nk.PublicKey().EVMAddress().Hex()), ev.NodeID)
	require.Equal(t, "placeholder", ev.FeedSource)
}

// TestRunRegisterOnly_NoBuyerRequired: --register-only registers + writes the
// evidence and exits cleanly without --buyer and without any bridge.
func TestRunRegisterOnly_NoBuyerRequired(t *testing.T) {
	dir := t.TempDir()
	regOut := filepath.Join(dir, "evidence.json")
	nk, _ := sellerTestKey(t)
	keyHex := nk.Hex()

	err := run([]string{"--register-only", "--key-hex", keyHex, "--registry-out", regOut, "--feed-source", "placeholder"}, Deps{})
	require.NoError(t, err)

	ev, rerr := sapient.ReadEvidence(regOut)
	require.NoError(t, rerr)
	require.True(t, ev.Simulated, "default backend stays memory/SIM")
	require.NotEmpty(t, ev.AgentID)
}

// TestRunRegisterOnly_EVMMintsThenReuses: two register-only runs with the same
// key against one shared contract — first mints, second reuses (idempotent
// restarts on the real chain, FR via registry.RegisterOrUpdate).
func TestRunRegisterOnly_EVMMintsThenReuses(t *testing.T) {
	const regHex = "0x5d9B1fE5eB02173205AEe8DC4f72db15bFB5f73C"
	dir := t.TempDir()
	nk, _ := sellerTestKey(t)
	keyHex := nk.Hex()
	shared := registry.NewMemoryRegistryContract()
	deps := Deps{ContractFactory: memoryContractFactory(shared)}

	args := func(out string) []string {
		return []string{
			"--register-only", "--key-hex", keyHex,
			"--registry-backend", "evm", "--registry-address", regHex,
			"--registry-out", out, "--feed-source", "placeholder",
		}
	}

	first := filepath.Join(dir, "first.json")
	require.NoError(t, run(args(first), deps))
	ev1, err := sapient.ReadEvidence(first)
	require.NoError(t, err)
	require.Equal(t, "minted", ev1.Outcome)
	require.False(t, ev1.Simulated)

	second := filepath.Join(dir, "second.json")
	require.NoError(t, run(args(second), deps))
	ev2, err := sapient.ReadEvidence(second)
	require.NoError(t, err)
	require.Equal(t, "reused", ev2.Outcome, "same key + same card => no second mint")
	require.Equal(t, ev1.AgentID, ev2.AgentID, "same tokenId on rerun")
}

// TestEVMPath_NoKeyMaterialInEvidenceOrLog: SEC-003 — the seller key hex must
// never surface in the registration log line, the evidence record, or the
// card JSON.
func TestEVMPath_NoKeyMaterialInEvidenceOrLog(t *testing.T) {
	const regHex = "0x5d9B1fE5eB02173205AEe8DC4f72db15bFB5f73C"
	nk, peerID := sellerTestKey(t)
	keyHex := nk.Hex()
	dir := t.TempDir()
	cardOut := filepath.Join(dir, "card.json")
	regOut := filepath.Join(dir, "evidence.json")

	cfg, err := resolveRegistryConfig(registryBackendEVM, regHex, "", defaultChainID, func(string) string { return "" })
	require.NoError(t, err)
	var logBuf bytes.Buffer
	deps := Deps{ContractFactory: memoryContractFactory(registry.NewMemoryRegistryContract())}

	_, raErr := registerAgentCard(context.Background(), nk, peerID, cardOut, regOut, "placeholder", cfg, deps, log.New(&logBuf, "", 0), nil, "", nil)
	require.NoError(t, raErr)

	for name, blob := range map[string][]byte{"log": logBuf.Bytes()} {
		require.NotContains(t, string(blob), keyHex, "%s leaks the key", name)
	}
	for _, p := range []string{cardOut, regOut} {
		b, rerr := os.ReadFile(p)
		require.NoError(t, rerr)
		require.NotContains(t, string(b), keyHex, "%s leaks the key", p)
	}
}
