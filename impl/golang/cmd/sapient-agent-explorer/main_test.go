package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// seedAgent builds a real seller card, registers it on the shared in-memory
// contract (distinct tokenIds 1,2,…), and writes the evidence file the explorer
// reads. Returns the evidence record.
func seedAgent(t *testing.T, dir string, contract *registry.MemoryRegistryContract) sapient.AgentEvidence {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	card, err := sapient.BuildSellerCard(sapient.SellerCardOptions{ChildKey: &k})
	require.NoError(t, err)
	contract.SetPendingOwner(common.BytesToAddress(k.PublicKey().EVMAddress().Bytes()))
	addr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	res, err := sapient.RegisterSeller(context.Background(), &k, card, addr, 0, contract)
	require.NoError(t, err)
	ev := sapient.EvidenceFromResult(res, true)
	require.NoError(t, sapient.WriteEvidence(filepath.Join(dir, ev.AgentID+".json"), ev))
	return ev
}

func TestExplorer_Table(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev1 := seedAgent(t, dir, contract)
	ev2 := seedAgent(t, dir, contract)
	require.NotEqual(t, ev1.AgentID, ev2.AgentID, "distinct agent ids")

	var buf bytes.Buffer
	require.NoError(t, run(&buf, []string{"--dir", dir}))
	out := buf.String()

	require.Contains(t, out, "AGENT ID")
	require.Contains(t, out, "PROTOCOL")
	for _, ev := range []sapient.AgentEvidence{ev1, ev2} {
		require.Contains(t, out, ev.AgentID)
		require.Contains(t, out, ev.SellerEVM)
		require.Contains(t, out, ev.PeerID)
		require.Contains(t, out, ev.NodeID)
	}
	require.Contains(t, out, sapient.ProtocolDetection)
	require.Contains(t, out, "rid")
}

func TestExplorer_JSONDumpsCard(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev := seedAgent(t, dir, contract)

	var buf bytes.Buffer
	require.NoError(t, run(&buf, []string{"--dir", dir, "--json", ev.AgentID}))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed), "output is valid JSON")
	require.Contains(t, parsed, "services", "dumped the agentURI card")
}

func TestExplorer_EmptyDirIsError(t *testing.T) {
	var buf bytes.Buffer
	require.Error(t, run(&buf, []string{"--dir", t.TempDir()}))
}

func TestExplorer_JSONNotFound(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	seedAgent(t, dir, contract)

	var buf bytes.Buffer
	require.Error(t, run(&buf, []string{"--dir", dir, "--json", "9999"}))
}

// --- on-chain resolve mode (buyer-side verification, read-only) ---

const testRegistryHex = "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28"

// memFactory injects the shared in-memory contract through the resolveDeps
// seam — the on-chain code path runs without any RPC.
func memFactory(contract *registry.MemoryRegistryContract) resolveDeps {
	return resolveDeps{ContractFactory: func(_ context.Context, _ string, _ keylib.EVMAddress) (registry.RegistryContract, error) {
		return contract, nil
	}}
}

func TestExplorer_OnChain_ResolveByOwner(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev := seedAgent(t, dir, contract)

	var buf bytes.Buffer
	require.NoError(t, runWithDeps(&buf, []string{
		"--registry-address", testRegistryHex,
		"--owner", ev.SellerEVM,
	}, memFactory(contract)))
	out := buf.String()

	require.Contains(t, out, "agentId:")
	require.Contains(t, out, ev.AgentID)
	require.Contains(t, out, ev.SellerEVM)
	require.Contains(t, out, ev.PeerID)
	require.Contains(t, out, ev.NodeID)
	require.Contains(t, out, sapient.ProtocolDetection)
	require.Contains(t, out, "[PASS] node_id↔owner")
	require.Contains(t, out, "[PASS] protocol")
	require.Contains(t, out, "[PASS] commerce[rid]")
	require.Contains(t, out, "[SKIPPED] peerID↔pubkey (V-REG-12)")
}

func TestExplorer_OnChain_ResolveByAgentID(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev := seedAgent(t, dir, contract)

	var buf bytes.Buffer
	require.NoError(t, runWithDeps(&buf, []string{
		"--registry-address", testRegistryHex,
		"--agent-id", ev.AgentID,
	}, memFactory(contract)))
	out := buf.String()

	require.Contains(t, out, ev.SellerEVM, "owner resolved from OwnerOf(tokenId)")
	require.Contains(t, out, "[PASS] node_id↔owner")
}

func TestExplorer_OnChain_JSONCardDump(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev := seedAgent(t, dir, contract)

	var buf bytes.Buffer
	require.NoError(t, runWithDeps(&buf, []string{
		"--registry-address", testRegistryHex,
		"--owner", ev.SellerEVM,
		"--json-card",
	}, memFactory(contract)))
	require.Contains(t, buf.String(), `"services"`, "dumped the resolved card JSON")
}

func TestExplorer_OnChain_RequiresOwnerXorAgentID(t *testing.T) {
	contract := registry.NewMemoryRegistryContract()
	var buf bytes.Buffer
	// neither
	err := runWithDeps(&buf, []string{"--registry-address", testRegistryHex}, memFactory(contract))
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one of")
	// both
	err = runWithDeps(&buf, []string{
		"--registry-address", testRegistryHex, "--owner", testRegistryHex, "--agent-id", "1",
	}, memFactory(contract))
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one of")
}

func TestExplorer_OnChain_DirMutuallyExclusive(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, []string{"--dir", t.TempDir(), "--registry-address", testRegistryHex, "--owner", testRegistryHex})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestExplorer_OnChain_NotFound(t *testing.T) {
	contract := registry.NewMemoryRegistryContract()
	var buf bytes.Buffer
	err := runWithDeps(&buf, []string{
		"--registry-address", testRegistryHex,
		"--owner", "0x0000000000000000000000000000000000000001",
	}, memFactory(contract))
	require.Error(t, err)
}
