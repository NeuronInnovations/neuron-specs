package main

// End-to-end test for the Stage D DroneScout MQTT source wiring
// (2026-05-14). Runs the seller against an in-process mochi-mqtt
// broker, publishes a synthetic-shape fixture, and asserts the
// process boots cleanly and shuts down on signal.
//
// This is the cross-cutting check that the chooseSource MQTT branch
// added in main.go actually produces a working FeedSource at runtime
// — i.e. paho dials the broker, parses inbound JSON, and feeds the
// DApp seller pipeline without crashing.
//
// Live-vs-fixture classification reminder: this test runs against an
// in-process broker fed by synthetic-shape fixtures, NOT a real
// DroneScout sensor. Per spec 017 FR-R15 the seller MUST advertise
// `feedSource = "replay"` here; the seller's auto-derive does so by
// default when --mqtt-url is set without --feed-source. The
// registry-mode unit tests (TestRun_MQTTSource_AutoDerivesFeedSourceToReplay,
// TestRun_MQTTSource_RespectsExplicitFeedSourceLive) cover the
// feedSource discipline; this E2E test focuses on the runtime
// wiring.

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// startMQTTBroker boots an in-process mochi-mqtt broker on a random
// 127.0.0.1 port and returns its tcp:// URL. The broker is torn down
// on t.Cleanup. Mirrors the helper in
// internal/feeds/remoteid/dronescout_mqtt_integration_test.go but
// inlined here so the seller test stays self-contained (the helper
// in the feeds package is lowercase and only visible to the feeds
// integration tests).
func startMQTTBroker(t *testing.T) (server *mochi.Server, brokerURL string) {
	t.Helper()

	server = mochi.New(&mochi.Options{
		InlineClient: true, // required for server.Publish() injection
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatalf("AddHook: %v", err)
	}

	listener := listeners.NewTCP(listeners.Config{
		ID:      "seller-e2e-tcp",
		Address: "127.0.0.1:0",
	})
	if err := server.AddListener(listener); err != nil {
		t.Fatalf("AddListener: %v", err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve() }()

	addr := listener.Address()
	if addr == "" {
		t.Fatal("listener.Address() returned empty after AddListener")
	}

	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Logf("server.Close: %v", err)
		}
		select {
		case <-serveErr:
		case <-time.After(500 * time.Millisecond):
		}
	})

	return server, "tcp://" + addr
}

// TestRun_MQTTSource_FixtureDirectMode_BootsAgainstInProcessBroker
// asserts that the seller binary boots, connects to a real (in-process)
// MQTT broker via the new --mqtt-url flag, and shuts down cleanly on
// signal. This is the runtime-wiring check that the chooseSource MQTT
// branch is hooked up correctly.
func TestRun_MQTTSource_FixtureDirectMode_BootsAgainstInProcessBroker(t *testing.T) {
	t.Parallel()

	server, brokerURL := startMQTTBroker(t)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	sig := make(chan os.Signal, 1)

	// Publish a synthetic fixture once the seller has had time to
	// dial + subscribe (paho's connect typically lands in <50ms
	// against an in-process broker). The seller then signals shutdown
	// shortly after.
	fixturePath := filepath.Join("..", "..", "internal", "feeds", "remoteid", "testdata", "dronescout", "data-bt5-single.json")
	payload, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "fixture must exist at %s", fixturePath)

	go func() {
		time.Sleep(200 * time.Millisecond)
		// Best-effort publish: if the seller hasn't connected yet, the
		// broker drops the message but the test still validates the
		// boot/shutdown path. The integration tests in
		// internal/feeds/remoteid cover the message-receipt path.
		if pubErr := server.Publish("remoteid/test/data", payload, false, 0); pubErr != nil {
			t.Logf("server.Publish: %v", pubErr)
		}
		time.Sleep(150 * time.Millisecond)
		sig <- os.Interrupt
	}()

	rc := run([]string{
		"--mode=fixture-direct",
		"--mqtt-url", brokerURL,
		"--mqtt-topic", "#",
		"--mqtt-compression=none",
		"--listen", "/ip4/127.0.0.1/udp/0/quic-v1",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{SignalCh: sig})

	assert.Equal(t, 0, rc, "seller must exit cleanly on signal; stderr=%s", stderr.String())
	assert.Contains(t, stdout.String(), "/p2p/", "fixture-direct mode must print at least one libp2p multiaddr")
	assert.Contains(t, stderr.String(), "mode=fixture-direct")
}

// TestRun_MQTTSource_RegistryMode_AdvertisesReplayFeedSource is the
// cross-mode E2E check: in eip8004-registry mode with a memory
// contract factory and an in-process MQTT broker, the seller's
// registered descriptor MUST carry feedSource=replay (the Stage D
// Decision-2 default). This exercises the runRegister → descriptor
// → log path under a live source-dial; the unit-test variant uses
// SkipServe so the source never actually instantiates.
func TestRun_MQTTSource_RegistryMode_AdvertisesReplayFeedSource(t *testing.T) {
	t.Parallel()

	_, brokerURL := startMQTTBroker(t)

	contract := registry.NewMemoryRegistryContract()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	sig := make(chan os.Signal, 1)

	go func() {
		time.Sleep(300 * time.Millisecond)
		sig <- os.Interrupt
	}()

	rc := run([]string{
		"--mode=eip8004-registry",
		"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		"--mqtt-url", brokerURL,
		"--mqtt-topic", "#",
		"--listen", "/ip4/127.0.0.1/udp/0/quic-v1",
		"--key-hex", fixedKeyHex,
	}, map[string]string{}, stdout, stderr, Deps{
		SignalCh:        sig,
		ContractFactory: memoryContractFactory(contract),
	})

	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	assert.Contains(t, stderr.String(), "feedSource=replay",
		"registry-mode --mqtt-url must auto-derive to feedSource=replay")
}
