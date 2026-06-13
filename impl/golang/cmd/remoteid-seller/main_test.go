package main

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// memoryContractFactory returns a Deps.ContractFactory that hands out the
// SAME *MemoryRegistryContract on every call, so a test can pre-seed it
// before invoking run() and read state after.
//
// It also pins pendingOwner from SignerKey so registry.Register's
// proof-of-control check (ownerOf(tokenId) == childAddress) is satisfied
// without the test having to thread the seller key into the closure.
func memoryContractFactory(contract *registry.MemoryRegistryContract) func(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error) {
	return func(_ context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error) {
		addr := ethcrypto.PubkeyToAddress(opts.SignerKey.PublicKey)
		contract.SetPendingOwner(addr)
		return contract, nil
	}
}

// fixedKeyHex is a deterministic test key. Avoids the entropy of the
// ephemeral fallback so tests are reproducible.
const fixedKeyHex = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"

// TestRun_FixtureDirectMode_StillBuildsHost runs the default-mode CLI with
// a short SIGINT-equivalent so the goroutines wind down. We don't assert
// on the full serve loop — we just verify the mode hasn't been broken by
// the run() refactor.
func TestRun_FixtureDirectMode_StillBuildsHost(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	sig := make(chan os.Signal, 1)
	// Fire the shutdown signal asynchronously; the seller reaches the
	// signal-wait, picks it up, and unwinds cleanly.
	go func() {
		time.Sleep(150 * time.Millisecond)
		sig <- os.Interrupt
	}()

	rc := run([]string{
		"--synth",
		"--synth-fps", "1",
		"--listen", "/ip4/127.0.0.1/udp/0/quic-v1",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SignalCh: sig})

	assert.Equal(t, 0, rc)
	// In fixture-direct mode the seller MUST print at least one multiaddr.
	assert.Contains(t, stdout.String(), "/p2p/", "fixture-direct mode should print the seller's libp2p multiaddr")
	// And the per-mode banner should reflect the chosen mode.
	assert.Contains(t, stderr.String(), "mode=fixture-direct")
}

func TestRun_RegistryMode_RequiresRegistryAddress(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mode=eip8004-registry",
		"--synth",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})

	assert.Equal(t, 2, rc, "missing --registry-address must be a configuration error (exit 2)")
	assert.Contains(t, stderr.String(), "requires --registry-address")
}

func TestRun_RegistryMode_RejectsUnknownModeValue(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--mode=eip8004-something-else",
		"--synth",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "unknown --mode")
}

func TestRun_RegistryMode_RequiresSource(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "pick exactly one source")
}

func TestRun_RegistryMode_RegistersAndLogsEvidence(t *testing.T) {
	t.Parallel()

	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--rpc-url", "memory://unused",
		"--chain-id", "296",
		"--escrow-contract", "0xCAFE0000000000000000000000000000000000ce",
		"--synth",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SkipServe:       true,
		ContractFactory: memoryContractFactory(contract),
	})
	require.Equal(t, 0, rc, "registry mode should succeed against memory contract; stderr=%s", stderr.String())

	logs := stderr.String()
	logsLower := strings.ToLower(logs)
	// Identity evidence (address rendered with EIP-55 checksum in the log;
	// compare case-insensitively against the canonical lowercase form).
	assert.Contains(t, logs, "[registry] mode=eip8004-registry")
	assert.Contains(t, logsLower, "registry=0x742d35cc6634c0532925a3b844bc9e7595f2bd28")
	assert.Contains(t, logs, "chainId=296")
	assert.Contains(t, logs, "tokenId=1", "first registration in the memory contract gets tokenId=1")
	assert.Contains(t, logs, "txHash=0xtxhash_register")
	assert.Contains(t, logs, "agentURISha256=")
	// Descriptor disclosure
	assert.Contains(t, logs, "name=remote-id")
	assert.Contains(t, logs, "pricing.unit=frame")
	assert.Contains(t, logs, "commerceMode=registration-only")
	assert.Contains(t, logs, "feedSource=synthetic", "FR-R15 auto-derived from --synth")

	// The mock should hold a single token now. Look it up at index 0.
	expectedKey, err := keylib.NeuronPrivateKeyFromHex(fixedKeyHex)
	require.NoError(t, err)
	expectedAddr := common.BytesToAddress(expectedKey.PublicKey().EVMAddress().Bytes())
	tok, err := contract.TokenOfOwnerByIndex(context.Background(), expectedAddr, big0())
	require.NoError(t, err)
	assert.Equal(t, int64(1), tok.Int64())

	uriJSON, err := contract.AgentURIOf(context.Background(), tok)
	require.NoError(t, err)
	assert.Contains(t, uriJSON, "remote-id", "stored agentURI must include the FR-R01 commerce name")
	assert.Contains(t, uriJSON, "/ds240/raw/1.0.0", "stored agentURI must include the FR-R02 raw protocol-id")
}

func TestRun_RegistryMode_CommerceModeOverride(t *testing.T) {
	t.Parallel()
	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--commerce-mode=full",
		"--feed-source=replay",
		"--synth",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SkipServe:       true,
		ContractFactory: memoryContractFactory(contract),
	})
	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	assert.Contains(t, stderr.String(), "commerceMode=full")
	assert.Contains(t, stderr.String(), "feedSource=replay")
	// Stage 2b: the CLI now wires the orchestrator when commerce-mode=full.
	// It creates 3 topics and logs the locator so the buyer can resolve
	// them out of the registered AgentURI. The orchestrator goroutine
	// itself is gated on SkipServe (true in this test).
	assert.Contains(t, stderr.String(), "topics created: stdIn=")
	assert.Contains(t, stderr.String(), "stdOut=")
	assert.Contains(t, stderr.String(), "stdErr=")
}

// TestRun_RegistryMode_DefaultRPCFromEnv proves that HEDERA_EVM_RPC env
// is consulted when --rpc-url is omitted.
func TestRun_RegistryMode_DefaultRPCFromEnv(t *testing.T) {
	t.Parallel()
	contract := registry.NewMemoryRegistryContract()
	var observed string
	factory := func(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error) {
		observed = opts.RPCURL
		contract.SetPendingOwner(ethcrypto.PubkeyToAddress(opts.SignerKey.PublicKey))
		return contract, nil
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--synth",
		"--key-hex", fixedKeyHex,
	}, map[string]string{"HEDERA_EVM_RPC": "https://override.example/rpc"}, stdout, stderr,
		Deps{SkipServe: true, ContractFactory: factory})
	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	assert.Equal(t, "https://override.example/rpc", observed,
		"factory should receive RPC URL from HEDERA_EVM_RPC when --rpc-url omitted")
}

// Stage 3A — no-silent-fallback config validation.

func TestRun_RegistryMode_HCSBackend_RejectsMissingEnv(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--mode=eip8004-registry",
		"--commerce-mode=full",
		"--topic-backend=hcs",
		"--escrow-backend=memory",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--synth",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "--topic-backend=hcs requires env")
	assert.Contains(t, stderr.String(), "HEDERA_OPERATOR_ID")
	assert.Contains(t, stderr.String(), "HEDERA_OPERATOR_KEY")
	assert.Contains(t, stderr.String(), "refusing to fall back to memory")
}

func TestRun_RegistryMode_EVMBackend_RejectsMissingEnv(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := run([]string{
		"--mode=eip8004-registry",
		"--commerce-mode=full",
		"--topic-backend=memory",
		"--escrow-backend=evm",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--synth",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "--escrow-backend=evm requires env")
	assert.Contains(t, stderr.String(), "NEURON_ESCROW_CONTRACT")
	assert.Contains(t, stderr.String(), "NEURON_TOKEN_CONTRACT")
	assert.Contains(t, stderr.String(), "refusing to fall back to memory")
}

func TestRun_RegistryMode_UnknownBackendsRejected(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{
			"unknown-topic",
			[]string{"--topic-backend=foo"},
			"unknown --topic-backend",
		},
		{
			"unknown-escrow",
			[]string{"--escrow-backend=bar"},
			"unknown --escrow-backend",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			args := append([]string{
				"--mode=eip8004-registry",
				"--commerce-mode=full",
				"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
				"--synth",
				"--key-hex", fixedKeyHex,
			}, c.args...)
			rc := run(args, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
			assert.Equal(t, 2, rc)
			assert.Contains(t, stderr.String(), c.want)
		})
	}
}

// TestDeriveFeedSource maps CLI flags to FR-R15 enum.
//
// Stage D (2026-05-14): added the mqttURL branch. DroneScout MQTT
// defaults to "replay", not "live": the source layer can't tell a
// real ds240 broker from an in-process test broker. Operators opt in
// to the live claim via `--feed-source=live`.
//
// 2026-05-18 (MVP Phase 1): added the basestationHost branch with
// the same "default to replay" discipline — the VPS-1 fake-DS240
// bridge is a Python simulator, not real hardware.
func TestDeriveFeedSource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		replay      string
		synth       bool
		ds400       string
		mqtt        string
		basestation string
		expected    string
	}{
		{replay: "fixture.json", expected: "replay"},
		{synth: true, expected: "synthetic"},
		{ds400: "udp", expected: "live"},
		{mqtt: "tcp://broker:1883", expected: "replay"},
		{basestation: "127.0.0.1:30003", expected: "replay"},
		{expected: "live"},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, deriveFeedSource(c.replay, c.synth, c.ds400, c.mqtt, c.basestation))
	}
}

func big0() *big.Int { return big.NewInt(0) }

// ---- Stage D: DroneScout MQTT source CLI wiring (2026-05-14) -------------
//
// The five tests below cover the seller-side flag-parsing and feedSource
// auto-derivation behavior for --mqtt-url. Connection-time behavior
// (paho dial, broker subscribe, payload normalisation) is covered by the
// MQTT E2E test in mqtt_e2e_test.go.

// TestRun_MQTTSource_RejectsMissingURL asserts that passing
// MQTT-related flags WITHOUT --mqtt-url is rejected by the
// source-count check. --mqtt-url is the source discriminator (mirror
// of --ds400-transport); setting peripheral MQTT flags without the
// URL never picks the source, so we fall through to "no source".
func TestRun_MQTTSource_RejectsMissingURL(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mqtt-username=foo",
		"--mqtt-password-env=MQTT_PWD",
		"--mqtt-topic=remoteid/#",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})

	assert.Equal(t, 2, rc, "MQTT flags without --mqtt-url must be a configuration error (exit 2)")
	assert.Contains(t, stderr.String(), "pick exactly one source")
	assert.Contains(t, stderr.String(), "--mqtt-url=<url>", "error message must point operators at the MQTT discriminator")
}

// TestRun_MQTTSource_RejectsConflictingSources asserts that
// --mqtt-url is mutually exclusive with --replay / --synth / --ds400-*.
func TestRun_MQTTSource_RejectsConflictingSources(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		args []string
	}{
		{"mqtt-and-synth", []string{"--mqtt-url=tcp://127.0.0.1:1883", "--synth"}},
		{"mqtt-and-replay", []string{"--mqtt-url=tcp://127.0.0.1:1883", "--replay=anywhere.json"}},
		{"mqtt-and-ds400", []string{"--mqtt-url=tcp://127.0.0.1:1883", "--ds400-transport=udp", "--ds400-address=x"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			args := append([]string{"--key-hex", fixedKeyHex}, c.args...)
			rc := run(args, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
			assert.Equal(t, 2, rc)
			assert.Contains(t, stderr.String(), "mutually exclusive")
		})
	}
}

// TestRun_MQTTSource_AutoDerivesFeedSourceToReplay asserts the
// Stage D Decision-2 default: --mqtt-url with no explicit
// --feed-source auto-derives to "replay", not "live". This is the
// conservative default because the source layer cannot distinguish a
// real ds240 broker from an in-process test broker.
//
// Uses --mode=eip8004-registry + memory contract factory + SkipServe
// so we exit after registration (before chooseSource attempts a
// broker dial).
func TestRun_MQTTSource_AutoDerivesFeedSourceToReplay(t *testing.T) {
	t.Parallel()
	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--mqtt-url=tcp://broker.example:1883",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SkipServe:       true,
		ContractFactory: memoryContractFactory(contract),
	})

	require.Equal(t, 0, rc, "registry mode with --mqtt-url must succeed (SkipServe short-circuits before broker dial); stderr=%s", stderr.String())
	assert.Contains(t, stderr.String(), "feedSource=replay",
		"--mqtt-url without --feed-source must auto-derive to 'replay' (Stage D Decision-2)")
}

// TestRun_MQTTSource_RespectsExplicitFeedSourceLive asserts that the
// operator can opt in to a live-evidence claim with --feed-source=live.
// This is the only way a DroneScout MQTT seller advertises live; the
// source layer never does so on its own.
func TestRun_MQTTSource_RespectsExplicitFeedSourceLive(t *testing.T) {
	t.Parallel()
	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--mqtt-url=mqtts://real-sensor.example:8883",
		"--feed-source=live",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SkipServe:       true,
		ContractFactory: memoryContractFactory(contract),
	})

	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	assert.Contains(t, stderr.String(), "feedSource=live",
		"--feed-source=live must override the auto-derived replay default")
}

// TestRun_MQTTSource_DoesNotLogPassword asserts that the password
// value referenced by --mqtt-password-env never appears in CLI output.
// The MQTT_PWD env var is set to a recognisable sentinel; the CLI is
// invoked with --mqtt-password-env=MQTT_PWD but WITHOUT --mqtt-url,
// so the error path fires before any source instantiation. This
// validates flag-parsing hygiene: the CLI never reads env-var values
// at parse time. (Connection-time hygiene is covered separately by
// the integration tests in internal/feeds/remoteid.)
func TestRun_MQTTSource_DoesNotLogPassword(t *testing.T) {
	t.Parallel()

	const sentinelPassword = "DO-NOT-LEAK-SECRET-VALUE-7f4c2a"

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mqtt-username=foo",
		"--mqtt-password-env=MQTT_PWD",
		"--key-hex", fixedKeyHex,
	}, map[string]string{"MQTT_PWD": sentinelPassword}, stdout, stderr, Deps{SkipServe: true})

	assert.Equal(t, 2, rc, "expected source-count error exit (no --mqtt-url given)")
	combined := stdout.String() + stderr.String()
	assert.NotContains(t, combined, sentinelPassword,
		"password value must NEVER appear in CLI output, even when the operator misuses the flags")
}

// ---- MVP Phase 1: VPS-1 fake-DS240 BaseStation source CLI tests --------
//
// Plan: §"Step 6 — Seller CLI flags & wiring", test table.

// TestRun_BasestationSource_RejectsMissingHost asserts that passing
// the source-related flags WITHOUT --basestation-tcp-host falls through
// to the "no source" error. --basestation-tcp-host is the
// source discriminator (mirror of --mqtt-url).
func TestRun_BasestationSource_RejectsMissingHost(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--basestation-source-label=custom",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SkipServe: true})

	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "pick exactly one source")
	assert.Contains(t, stderr.String(), "--basestation-tcp-host",
		"error message must mention the BaseStation discriminator")
}

// TestRun_BasestationSource_RejectsConflictingSources asserts mutual
// exclusion between --basestation-tcp-host and the other source
// discriminators.
func TestRun_BasestationSource_RejectsConflictingSources(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		args []string
	}{
		{"basestation-and-synth", []string{"--basestation-tcp-host=127.0.0.1:30003", "--synth"}},
		{"basestation-and-replay", []string{"--basestation-tcp-host=127.0.0.1:30003", "--replay=anywhere.json"}},
		{"basestation-and-mqtt", []string{"--basestation-tcp-host=127.0.0.1:30003", "--mqtt-url=tcp://broker:1883"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			args := append([]string{"--key-hex", fixedKeyHex}, c.args...)
			rc := run(args, map[string]string{}, stdout, stderr, Deps{SkipServe: true})
			assert.Equal(t, 2, rc)
			assert.Contains(t, stderr.String(), "mutually exclusive")
		})
	}
}

// TestRun_BasestationSource_AutoDerivesFeedSourceToReplay asserts the
// VPS-1 default: --basestation-tcp-host with no explicit
// --feed-source auto-derives to "replay" (synthetic upstream, not a
// real DS240).
func TestRun_BasestationSource_AutoDerivesFeedSourceToReplay(t *testing.T) {
	t.Parallel()
	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--basestation-tcp-host=127.0.0.1:30003",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SkipServe:       true,
		ContractFactory: memoryContractFactory(contract),
	})

	require.Equal(t, 0, rc, "registry mode with --basestation-tcp-host must succeed (SkipServe short-circuits before TCP dial); stderr=%s", stderr.String())
	assert.Contains(t, stderr.String(), "feedSource=replay",
		"--basestation-tcp-host without --feed-source must auto-derive to 'replay' (synthetic upstream discipline)")
}

// TestRun_BasestationSource_RespectsExplicitFeedSourceLive asserts an
// operator who has independently verified a real DS240 swap-in can
// opt in to the live-evidence claim.
func TestRun_BasestationSource_RespectsExplicitFeedSourceLive(t *testing.T) {
	t.Parallel()
	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--basestation-tcp-host=127.0.0.1:30003",
		"--feed-source=live",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SkipServe:       true,
		ContractFactory: memoryContractFactory(contract),
	})

	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	assert.Contains(t, stderr.String(), "feedSource=live",
		"--feed-source=live must override the auto-derived replay default")
}

// TestRun_BasestationSource_AdvertisesBasestationProtocolByDefault
// asserts that --basestation-tcp-host auto-flips
// --advertise-basestation-protocol to true (operator-friendly default).
//
// Note: this test only verifies the CLI parses + registers without
// error. The deeper assertion (that the seller actually opens TWO
// stream handlers) lives in the dapp/remoteid seller_test.go
// TestSeller_Start_RegistersMultipleProtocols.
func TestRun_BasestationSource_AdvertisesBasestationProtocolByDefault(t *testing.T) {
	t.Parallel()
	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--basestation-tcp-host=127.0.0.1:30003",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SkipServe:       true,
		ContractFactory: memoryContractFactory(contract),
	})

	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	// SkipServe short-circuits before Start, so the "advertising 2
	// stream protocols" line never fires. The auto-derive logic itself
	// is exercised in this test; the multi-protocol Start path is
	// covered in dapp/remoteid/seller_test.go.
}
