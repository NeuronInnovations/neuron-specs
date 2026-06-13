package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// fixedKeyHex — deterministic seller key for reproducible tests.
const fixedKeyHex = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"

// memoryContractFactory hands out the SAME *MemoryRegistryContract on every
// call and pins pendingOwner to the seller's address.
func memoryContractFactory(contract *registry.MemoryRegistryContract) func(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error) {
	return func(_ context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error) {
		addr := ethcrypto.PubkeyToAddress(opts.SignerKey.PublicKey)
		contract.SetPendingOwner(addr)
		return contract, nil
	}
}

// fixturePath returns the absolute path to a shared SBS fixture for replay tests.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "internal", "feeds", "sbs", "testdata", name))
	require.NoError(t, err)
	return abs
}

// --- Flag-parsing / invalid-config ---

func TestRun_RequiresFeedSource(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{"--key-hex", fixedKeyHex}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc, "missing --feed-source must exit 2")
	assert.Contains(t, stderr.String(), "--feed-source is required")
}

func TestRun_UnknownFeedSource(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--feed-source=mqtt", // not in adsb's vocabulary
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "unknown --feed-source")
}

func TestRun_ReplayRequiresPath(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--feed-source=replay",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "--feed-source=replay requires --replay")
}

func TestRun_BasestationTCPRequiresHost(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--feed-source=basestation-tcp",
		"--basestation-tcp-host=", // explicitly empty
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "--basestation-tcp-host")
}

func TestRun_RejectsUnknownMode(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--mode=eip8004-something-else",
		"--feed-source=synthetic",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "unknown --mode")
}

func TestRun_RegistryMode_RequiresRegistryAddress(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--mode=eip8004-registry",
		"--feed-source=synthetic",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "requires --registry-address")
}

// --- No silent fallback ---

func TestRun_NoSilentFallback_HCSMissingEnv(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--feed-source=synthetic",
		"--topic-backend=hcs",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "--topic-backend=hcs requires env")
	assert.Contains(t, stderr.String(), "refusing to fall back to memory")
}

func TestRun_NoSilentFallback_EVMMissingEnv(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--feed-source=synthetic",
		"--escrow-backend=evm",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "--escrow-backend=evm requires env")
	assert.Contains(t, stderr.String(), "refusing to fall back to memory")
}

// --- Happy-path: fixture-direct + synth ---

func TestRun_FixtureDirectMode_Synth(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	sig := make(chan os.Signal, 1)
	go func() {
		time.Sleep(150 * time.Millisecond)
		sig <- os.Interrupt
	}()

	rc := run([]string{
		"--feed-source=synthetic",
		"--synth-aircraft", "2",
		"--synth-fps", "5",
		"--listen", "/ip4/127.0.0.1/udp/0/quic-v1",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SignalCh: sig})

	assert.Equal(t, 0, rc)
	assert.Contains(t, stdout.String(), "/p2p/",
		"fixture-direct mode must print at least one /p2p/ multiaddr")
	assert.Contains(t, stderr.String(), "mode=fixture-direct")
	assert.Contains(t, stderr.String(), "feedSource=synthetic")
}

// --- Happy-path: fixture-direct + replay ---

func TestRun_FixtureDirectMode_Replay(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	sig := make(chan os.Signal, 1)
	go func() {
		time.Sleep(150 * time.Millisecond)
		sig <- os.Interrupt
	}()

	rc := run([]string{
		"--feed-source=replay",
		"--replay", fixturePath(t, "vanilla-jv.sbs"),
		"--loop",
		"--speedup", "1000",
		"--listen", "/ip4/127.0.0.1/udp/0/quic-v1",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SignalCh: sig})

	assert.Equal(t, 0, rc)
	assert.Contains(t, stdout.String(), "/p2p/")
	assert.Contains(t, stderr.String(), "feedSource=replay")
}

// --- Registry mode: registration evidence ---

func TestRun_RegistryMode_RegistersAndLogsEvidence(t *testing.T) {
	t.Parallel()
	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mode=eip8004-registry",
		"--feed-source=synthetic",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--chain-id=296",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SkipServe:       true,
		ContractFactory: memoryContractFactory(contract),
	})

	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	out := stderr.String()
	// Fresh registration logs outcome=minted (idempotent RegisterOrUpdate).
	assert.Contains(t, out, "[registry] mode=eip8004-registry outcome=minted sellerEVM=")
	assert.Contains(t, out, "name=adsb")
	assert.Contains(t, out, "pricing.unit=frame")
	assert.Contains(t, out, "feedSource=synthetic")
}

// --- Registry mode: BaseStation TCP logs the feedSourceConfig lineage ---

func TestRun_RegistryMode_BaseStationTCPLogsUpstream(t *testing.T) {
	t.Parallel()
	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mode=eip8004-registry",
		"--feed-source=basestation-tcp",
		"--basestation-tcp-host=127.0.0.1:30003",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--chain-id=296",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SkipServe:       true,
		ContractFactory: memoryContractFactory(contract),
	})
	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	out := stderr.String()
	// Audit Q-5: BaseStation TCP maps to feedSource=live with upstream lineage.
	assert.Contains(t, out, "feedSource=live")
	assert.Contains(t, out, "basestation-tcp source: upstream=127.0.0.1:30003")
}

// --- deriveFeedSource is the load-bearing CLI ↔ heartbeat mapping ---

func TestDeriveFeedSource(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"basestation-tcp": "live",
		"replay":          "replay",
		"synthetic":       "synthetic",
		"":                "live",
	}
	for in, want := range cases {
		assert.Equal(t, want, deriveFeedSource(in), "deriveFeedSource(%q)", in)
	}
}

// --- Key resolution sanity ---

func TestResolveKey_ExplicitHex(t *testing.T) {
	t.Parallel()
	k, err := resolveKey(fixedKeyHex)
	require.NoError(t, err)
	require.NotNil(t, k)

	// Confirm we got the same key bytes back through keylib.
	neuronKey, err := keylib.NeuronPrivateKeyFromHex(fixedKeyHex)
	require.NoError(t, err)
	want, err := neuronKey.ToBlockchainKey()
	require.NoError(t, err)
	assert.Equal(t, want.D.Cmp(k.D), 0)
}

func TestResolveKey_EphemeralWhenEmpty(t *testing.T) {
	t.Parallel()
	a, err := resolveKey("")
	require.NoError(t, err)
	b, err := resolveKey("")
	require.NoError(t, err)
	assert.NotEqual(t, a.D.Cmp(b.D), 0, "ephemeral keys must differ across calls")
}

// --- Smoke: chooseSource returns non-nil for each feed-source ---

func TestChooseSource_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	for _, fs := range []string{"basestation-tcp", "replay", "synthetic"} {
		src := chooseSource(sourceParams{
			feedSource:         fs,
			replayPath:         fixturePath(t, "vanilla-jv.sbs"),
			basestationTCPHost: "127.0.0.1:30003",
			synthAircraft:      1,
			synthFPS:           1,
			speedup:            1.0,
		})
		assert.NotNil(t, src, "chooseSource(feedSource=%s) must return a FeedSource", fs)
	}
}

func TestRun_FixtureDirectMode_Replay_FixtureExists(t *testing.T) {
	t.Parallel()
	// Sanity: the fixture path resolves to an actual file.
	path := fixturePath(t, "vanilla-jv.sbs")
	_, err := os.Stat(path)
	require.NoError(t, err, "expected SBS fixture at %s", path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	// Fixture has at least one MSG,3 line (the SBS parser only filters
	// type-3 records; the rest are dropped).
	assert.True(t, strings.Contains(string(data), "MSG,3,"),
		"fixture must contain at least one MSG,3 record")
}
