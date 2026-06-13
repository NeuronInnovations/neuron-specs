package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// memoryConfig is the resolved SIM default — what resolveRegistryConfig
// returns with no evm flags.
func memoryConfig() registryConfig {
	return registryConfig{backend: registryBackendMemory, simulated: true}
}

func sellerTestKey(t *testing.T) (*keylib.NeuronPrivateKey, string) {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pid, err := k.PublicKey().PeerID()
	require.NoError(t, err)
	return &k, pid.String()
}

// loadADSBFixture reads the captured-from-the-real-neuron-jv-bridge NDJSON.
func loadADSBFixture(t *testing.T) []*sapientpb.SapientMessage {
	t.Helper()
	f, err := os.Open("testdata/adsb-sample.ndjson")
	require.NoError(t, err)
	defer f.Close()
	var msgs []*sapientpb.SapientMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		m := &sapientpb.SapientMessage{}
		require.NoError(t, protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(sc.Bytes(), m))
		msgs = append(msgs, m)
	}
	require.NotEmpty(t, msgs)
	return msgs
}

func TestKeyHexOrEnv(t *testing.T) {
	t.Setenv("NEURON_KEY_HEX", "aa11")
	require.Equal(t, "ff22", keyHexOrEnv("ff22"), "explicit flag wins over env")
	require.Equal(t, "aa11", keyHexOrEnv(""), "env fallback when flag empty")
	t.Setenv("NEURON_KEY_HEX", "")
	require.Empty(t, keyHexOrEnv(""), "both empty -> ephemeral path")
}

// TestJVSeller_RegisterOnlyWritesEvidence: the full run() register-only path
// writes a valid JetVision card (jetvision-adsb-sapient service, neuron.adsb/1
// extension + capabilities) and an evidence record with the JV service name.
func TestJVSeller_RegisterOnlyWritesEvidence(t *testing.T) {
	nk, peerID := sellerTestKey(t)
	dir := t.TempDir()
	cardOut := filepath.Join(dir, "card.json")
	regOut := filepath.Join(dir, "evidence.json")

	err := run([]string{
		"--register-only",
		"--key-hex", nk.Hex(),
		"--agent-card-out", cardOut,
		"--registry-out", regOut,
		"--feed-source", "live",
	}, Deps{})
	require.NoError(t, err)

	// Card file is the agentURI; it parses, validates, and carries the JV
	// service + extension.
	cardBytes, err := os.ReadFile(cardOut)
	require.NoError(t, err)
	uri, err := registry.AgentURIFromJSON(string(cardBytes))
	require.NoError(t, err)
	valid, vErrs := registry.ValidateRegistrationCompleteness(uri, nk.PublicKey())
	require.True(t, valid, "written card must validate: %v", vErrs)

	commerce := uri.CommerceServices()
	require.Len(t, commerce, 1)
	assert.Equal(t, sapient.JVCommerceServiceName, commerce[0].Name)

	var sawExt bool
	for _, ts := range uri.TopicServices() {
		if ts.Channel != "stdOut" {
			continue
		}
		ext, ok := ts.Config[sapient.JVExtensionID].(map[string]any)
		require.True(t, ok, "stdOut config carries neuron.adsb/1")
		caps, ok := ext["capabilities"].([]any)
		require.True(t, ok, "JV extension advertises capabilities")
		labels := make([]string, 0, len(caps))
		for _, c := range caps {
			labels = append(labels, c.(string))
		}
		assert.ElementsMatch(t, sapient.JVCapabilities, labels)
		assert.Equal(t, []any{"JetVision Air!Squitter"}, ext["sensorModels"].([]any))
		sawExt = true
	}
	require.True(t, sawExt)

	// Evidence record binds identity + JV service name + provenance.
	ev, err := sapient.ReadEvidence(regOut)
	require.NoError(t, err)
	assert.Equal(t, peerID, ev.PeerID)
	assert.Equal(t, nk.PublicKey().EVMAddress().Hex(), ev.SellerEVM)
	assert.Equal(t, sapient.NodeIDFromIdentity(nk.PublicKey().EVMAddress().Hex()), ev.NodeID)
	assert.Equal(t, sapient.JVCommerceServiceName, ev.Service)
	assert.Equal(t, sapient.ProtocolDetection, ev.Protocol)
	assert.True(t, ev.Simulated, "memory-contract registration is labelled simulated")
	assert.NotEmpty(t, ev.AgentID)
	assert.Equal(t, "live", ev.FeedSource)
}

func TestJVSeller_RegisterAgentCard_PeerIDMismatch(t *testing.T) {
	nk, _ := sellerTestKey(t)
	_, err := registerAgentCard(context.Background(), nk, "12D3KooWBogusPeerIdThatDoesNotMatch", "", "", "live", memoryConfig(), Deps{}, log.New(io.Discard, "", 0), "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "identity mismatch")
}

// TestJVSeller_RejectsCommerceFlags: the commerce surface was deliberately
// removed — passing a commerce flag is a hard flag-parse error, not a silent
// no-op.
func TestJVSeller_RejectsCommerceFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--commerce-mode", "full"},
		{"--escrow-backend", "evm"},
		{"--pricing-amount", "5"},
		{"--commerce-evidence-out", "/tmp/x.json"},
		{"--topic-backend", "hcs"},
	} {
		err := run(args, Deps{})
		require.Error(t, err, "args %v must be rejected", args)
	}
}

// TestJVSeller_PushesRestampedFrames: the seller sources a fake jv-bridge
// (real FeedServer serving the captured ADS-B fixture), dials an in-process
// buyer, and pushes every frame with node_id re-stamped to its Neuron
// identity. The adsb.* object_info passes through untouched (opaque data
// plane).
func TestJVSeller_PushesRestampedFrames(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	sellerNK, _ := sellerTestKey(t)
	wantNodeID := sapient.NodeIDFromIdentity(sellerNK.PublicKey().EVMAddress().Hex())

	// Fake jv-bridge: LE-framed SAPIENT feed serving the fixture on repeat.
	bridge, err := sapient.ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer bridge.Close()
	fixture := loadADSBFixture(t)
	go func() {
		for i := 0; ctx.Err() == nil; i++ {
			_ = bridge.Publish(fixture[i%len(fixture)])
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// In-process buyer host: captures frames, then resets the stream so the
	// seller's run() returns (it owns its own signal context).
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerECDSA, err := buyerKey.ToBlockchainKey()
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	var (
		mu  sync.Mutex
		got []*sapientpb.SapientMessage
	)
	buyerHost.SetStreamHandler(protocol.ID(sapient.ProtocolDetection), func(stream libp2pnetwork.Stream) {
		r := delivery.NewFrameReader(stream)
		for {
			data, rerr := r.ReadFrame()
			if rerr != nil {
				return
			}
			m := &sapientpb.SapientMessage{}
			if uerr := (proto.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, m); uerr != nil {
				continue
			}
			mu.Lock()
			got = append(got, m)
			n := len(got)
			mu.Unlock()
			if n >= 6 {
				_ = stream.Reset()
				return
			}
		}
	})
	buyerMA := buyerHost.Addrs()[0].String() + "/p2p/" + buyerHost.ID().String()

	sellerErr := make(chan error, 1)
	go func() {
		sellerErr <- run([]string{
			"--key-hex", sellerNK.Hex(),
			"--bridge-addr", bridge.Addr(),
			"--buyer", buyerMA,
		}, Deps{})
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) >= 6
	}, 20*time.Second, 20*time.Millisecond, "buyer must capture pushed frames")

	mu.Lock()
	defer mu.Unlock()
	for _, m := range got {
		assert.Equal(t, wantNodeID, m.GetNodeId(), "every frame re-stamped with the seller identity")
		dr := m.GetDetectionReport()
		require.NotNil(t, dr, "DetectionReport forwarded intact")
		var sawADSB bool
		for _, oi := range dr.GetObjectInfo() {
			if len(oi.GetType()) > 5 && oi.GetType()[:5] == "adsb." {
				sawADSB = true
				break
			}
		}
		assert.True(t, sawADSB, "adsb.* object_info passes through opaque")
	}
}

// --- registry backend config validation (forked surface) ---

func TestJVRegistryConfig_DefaultsToMemory(t *testing.T) {
	cfg, err := resolveRegistryConfig("memory", "", "", defaultChainID, func(string) string { return "" })
	require.NoError(t, err)
	require.Equal(t, registryBackendMemory, cfg.backend)
	require.True(t, cfg.simulated)
}

func TestJVRegistryConfig_EVMRequiresAddress(t *testing.T) {
	_, err := resolveRegistryConfig("evm", "", "", defaultChainID, func(string) string { return "" })
	require.Error(t, err)
	require.Contains(t, err.Error(), envRegistryContract)
}

func TestJVRegistryConfig_EVMEnvFallback(t *testing.T) {
	cfg, err := resolveRegistryConfig("evm", "", "", defaultChainID, func(k string) string {
		if k == envRegistryContract {
			return "0x000000000000000000000000000000000000dEaD"
		}
		return ""
	})
	require.NoError(t, err)
	require.Equal(t, registryBackendEVM, cfg.backend)
	require.False(t, cfg.simulated)
	require.Equal(t, defaultRPCURL, cfg.rpc)
}
