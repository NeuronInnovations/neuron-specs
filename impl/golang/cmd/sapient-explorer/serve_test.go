package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

func testServer(t *testing.T, cfg config) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(newServer(cfg, log.New(io.Discard, "", 0)).routes())
	t.Cleanup(ts.Close)
	return ts
}

// seedAgent mirrors cmd/sapient-agent-explorer's helper: builds a real seller
// card, registers it on a shared in-memory contract, and writes the evidence
// file the explorer reads. Returns the evidence record and the seed private key
// (so tests can assert the key never leaks through the API).
func seedAgent(t *testing.T, dir string, contract *registry.MemoryRegistryContract) (sapient.AgentEvidence, keylib.NeuronPrivateKey) {
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
	return ev, k
}

func getBody(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Get(url) //nolint:noctx // test client
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp, body
}

func TestAgentsHandler_ListsSeededAgents(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev1, _ := seedAgent(t, dir, contract)
	ev2, _ := seedAgent(t, dir, contract)
	ts := testServer(t, config{evidenceDir: dir})

	resp, body := getBody(t, ts.URL+"/agents.json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out struct {
		Agents      []AgentSummary `json:"agents"`
		Count       int            `json:"count"`
		EvidenceDir string         `json:"evidenceDir"`
		Error       string         `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	require.Empty(t, out.Error)
	require.Equal(t, 2, out.Count)
	require.Len(t, out.Agents, 2)
	ids := []string{out.Agents[0].AgentID, out.Agents[1].AgentID}
	require.Contains(t, ids, ev1.AgentID)
	require.Contains(t, ids, ev2.AgentID)
	require.Equal(t, sapient.ProtocolDetection, out.Agents[0].Protocol)
	require.Equal(t, "rid", out.Agents[0].Service)
	require.True(t, out.Agents[0].Simulated)
	require.Equal(t, dir, out.EvidenceDir)
}

func TestAgentsHandler_MissingDirIsGraceful(t *testing.T) {
	ts := testServer(t, config{evidenceDir: filepath.Join(t.TempDir(), "nope")})
	resp, body := getBody(t, ts.URL+"/agents.json")
	require.Equal(t, http.StatusOK, resp.StatusCode, "missing dir must not 500")
	var out struct {
		Agents []AgentSummary `json:"agents"`
		Count  int            `json:"count"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	require.Equal(t, 0, out.Count)
	require.Empty(t, out.Agents)
}

func TestAgentsHandler_EmptyDir(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir()})
	resp, body := getBody(t, ts.URL+"/agents.json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out struct {
		Count int `json:"count"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	require.Equal(t, 0, out.Count)
}

func TestAgentsHandler_InvalidFileDegradesHonestly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not valid json"), 0o644))
	ts := testServer(t, config{evidenceDir: dir})
	resp, body := getBody(t, ts.URL+"/agents.json")
	require.Equal(t, http.StatusOK, resp.StatusCode, "bad file must not 500 the page")
	var out struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	require.NotEmpty(t, out.Error, "parse error is surfaced, not silently skipped")
}

func TestAgentsHandler_SkipsNonEvidenceJSON(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev, _ := seedAgent(t, dir, contract)
	// A track state-snapshot file lives next to the evidence (as in the captured
	// staging dirs): valid JSON, but not an AgentEvidence record. It must be
	// skipped (no agentId / agentURI), not listed as a junk agent or an error.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "state-snapshot.json"),
		[]byte(`{"tracks":[{"uid":"D1"}],"count":1,"focus":{"lat":50,"lon":-5,"count":1}}`), 0o644))
	ts := testServer(t, config{evidenceDir: dir})

	resp, body := getBody(t, ts.URL+"/agents.json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out struct {
		Agents []AgentSummary `json:"agents"`
		Count  int            `json:"count"`
		Error  string         `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	require.Empty(t, out.Error, "a non-evidence JSON file is skipped, not an error")
	require.Equal(t, 1, out.Count, "only the real evidence record is listed")
	require.Equal(t, ev.AgentID, out.Agents[0].AgentID)
}

func TestAgentDetailHandler_ReturnsCardAndProvenance(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev, _ := seedAgent(t, dir, contract)
	ts := testServer(t, config{evidenceDir: dir})

	resp, body := getBody(t, ts.URL+"/agents/"+ev.AgentID+".json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var d AgentCardDetail
	require.NoError(t, json.Unmarshal(body, &d))
	require.Equal(t, ev.AgentID, d.AgentID)
	require.Equal(t, "SIM", d.Provenance.Mode)
	require.Empty(t, d.Provenance.TransactionHash, "SIM hides placeholder tx hash")
	require.NotNil(t, d.Sensor)
	require.Equal(t, sapient.SapientWire, d.Sensor.Wire)
	require.Equal(t, sapient.DefaultSensorModels, d.Sensor.SensorModels)

	// Card byte-fidelity: the canonical (compacted) card hashes to agentURISha256.
	require.Equal(t, ev.AgentURISha256, d.AgentURISha256)
	var compact bytes.Buffer
	require.NoError(t, json.Compact(&compact, d.Card))
	sum := sha256.Sum256(compact.Bytes())
	require.Equal(t, ev.AgentURISha256, hex.EncodeToString(sum[:]), "displayed card matches the recorded hash")
}

func TestAgentDetailHandler_NotFound(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	seedAgent(t, dir, contract)
	ts := testServer(t, config{evidenceDir: dir})
	resp, _ := getBody(t, ts.URL+"/agents/9999.json")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestNoSecretLeak is the security guard: the seller's private key, and any
// secret-like JSON key, must never appear in any API response.
func TestNoSecretLeak(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev, k := seedAgent(t, dir, contract)
	ts := testServer(t, config{evidenceDir: dir})

	suspicious := regexp.MustCompile(`(?i)"(priv(ate)?_?key|secret|mnemonic|seed_?phrase|passphrase)"\s*:`)
	privHex := strings.ToLower(strings.TrimPrefix(k.Hex(), "0x"))
	require.NotEmpty(t, privHex)
	require.Len(t, privHex, 64, "32-byte secp256k1 scalar")

	for _, path := range []string{"/agents.json", "/agents/" + ev.AgentID + ".json"} {
		_, body := getBody(t, ts.URL+path)
		require.NotContains(t, strings.ToLower(string(body)), privHex,
			"private key hex must never appear in %s", path)
		require.False(t, suspicious.Match(body), "no secret-like JSON keys in %s", path)
	}
}

func TestHealthz_ReportsFidDown(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	seedAgent(t, dir, contract)
	ts := testServer(t, config{evidenceDir: dir, fidURL: "http://127.0.0.1:1"}) // unreachable

	resp, body := getBody(t, ts.URL+"/healthz")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var h struct {
		OK             bool   `json:"ok"`
		EvidenceAgents int    `json:"evidenceAgents"`
		FidDisplay     string `json:"fidDisplay"`
	}
	require.NoError(t, json.Unmarshal(body, &h))
	require.True(t, h.OK)
	require.Equal(t, 1, h.EvidenceAgents)
	require.Equal(t, "down", h.FidDisplay, "unreachable fid-url reported down")
}

func TestStaticAssetsServed(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir()})
	for _, tc := range []struct{ path, wantType string }{
		{"/", "text/html"},
		{"/app.js", "javascript"},
		{"/app.css", "text/css"},
	} {
		resp, body := getBody(t, ts.URL+tc.path)
		require.Equal(t, http.StatusOK, resp.StatusCode, tc.path)
		require.NotEmpty(t, body, tc.path)
		require.Contains(t, resp.Header.Get("Content-Type"), tc.wantType, tc.path)
	}
}
