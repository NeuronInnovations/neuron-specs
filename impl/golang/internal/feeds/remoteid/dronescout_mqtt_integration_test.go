package remoteid

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// In-process integration tests for RunDroneScoutMQTT using
// mochi-mqtt/server/v2 as an embedded broker. The DroneScout sensor
// is **simulated** by publishing the synthetic-shape fixtures from
// testdata/dronescout/ through the broker's server-side Publish API.
//
// **This is NOT live evidence.** The Stage A research report's
// distinction (research-report §0 + §6 of dronescout-mqtt-live-feed-
// plan.md) is preserved: a seller subscribed to this in-process
// broker MUST advertise `feedSource = "replay"`, not `"live"`. Live
// classification requires a real DroneScout sensor publishing in
// real time over an operator-owned broker.

// startTestBroker boots a mochi-mqtt broker on a random localhost
// port and returns the connection URL plus a server handle so tests
// can server.Publish() simulated sensor payloads. The broker is
// torn down on t.Cleanup.
func startTestBroker(t *testing.T) (server *mochi.Server, brokerURL string) {
	t.Helper()

	server = mochi.New(&mochi.Options{
		// InlineClient enables server.Publish() to inject messages
		// as if from a connected client; required for these tests
		// to simulate the DroneScout sensor publishing fixtures.
		InlineClient: true,
		// Suppress the broker's own slog output during tests; the
		// default logs to stderr at INFO level.
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatalf("AddHook: %v", err)
	}

	listener := listeners.NewTCP(listeners.Config{
		ID:      "test-tcp",
		Address: "127.0.0.1:0",
	})
	if err := server.AddListener(listener); err != nil {
		t.Fatalf("AddListener: %v", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve()
	}()

	addr := listener.Address()
	if addr == "" {
		t.Fatal("listener.Address() returned empty string after AddListener")
	}

	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Logf("server.Close: %v", err)
		}
		// Drain the serve error to keep the goroutine clean.
		select {
		case <-serveErr:
		case <-time.After(500 * time.Millisecond):
		}
	})

	return server, "tcp://" + addr
}

// startSource spawns RunDroneScoutMQTT in a goroutine and blocks
// until the OnReady callback fires (or the test times out). Returns
// the goroutine's error channel for assertion at end-of-test.
func startSource(t *testing.T, ctx context.Context, cfg DroneScoutMQTTConfig, out chan DecodedFrame) (errCh <-chan error) {
	t.Helper()

	ready := make(chan struct{})
	var once sync.Once
	cfg.OnReady = func() { once.Do(func() { close(ready) }) }

	ch := make(chan error, 1)
	go func() {
		ch <- RunDroneScoutMQTT(ctx, cfg, out)
	}()

	select {
	case <-ready:
		// MQTT client connected + subscribed; safe to publish.
	case err := <-ch:
		t.Fatalf("RunDroneScoutMQTT exited before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("MQTT source did not signal ready within 5s")
	}

	return ch
}

func waitForDecodedFrame(t *testing.T, out <-chan DecodedFrame, timeout time.Duration) DecodedFrame {
	t.Helper()
	select {
	case df := <-out:
		return df
	case <-time.After(timeout):
		t.Fatalf("no DecodedFrame received within %s", timeout)
		return DecodedFrame{} // unreachable
	}
}

func publishFixture(t *testing.T, server *mochi.Server, topic, fixturePath string) {
	t.Helper()
	payload, err := os.ReadFile(filepath.FromSlash(fixturePath))
	if err != nil {
		t.Fatalf("read %s: %v", fixturePath, err)
	}
	if err := server.Publish(topic, payload, false, 0); err != nil {
		t.Fatalf("server.Publish to %s: %v", topic, err)
	}
}

func TestRunDroneScoutMQTT_IntegrationEndToEnd_DataMessage(t *testing.T) {
	server, brokerURL := startTestBroker(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan DecodedFrame, 8)
	cfg := DroneScoutMQTTConfig{
		URL:              brokerURL,
		Topic:            "#",
		Compression:      "none",
		SensorModel:      "dronescout-ds240",
		ConnectTimeout:   2 * time.Second,
		SubscribeTimeout: 2 * time.Second,
	}
	errCh := startSource(t, ctx, cfg, out)

	publishFixture(t, server, "remoteid/test/data", fixtureDataSingle)

	df := waitForDecodedFrame(t, out, 2*time.Second)
	if df.Type != "remote-id-frame" {
		t.Errorf("Type = %q, want %q", df.Type, "remote-id-frame")
	}
	if df.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", df.Version, "1.0.0")
	}
	if df.Source != "dronescout-ds240" {
		t.Errorf("Source = %q, want %q", df.Source, "dronescout-ds240")
	}
	if df.DroneID != syntheticUASdata1 {
		t.Errorf("DroneID = %q, want %q (UASdata preserved verbatim per Stage B scope)", df.DroneID, syntheticUASdata1)
	}
	if df.DroneIDType != DroneIDTypeUASdataBase64 {
		t.Errorf("DroneIDType = %q, want %q", df.DroneIDType, DroneIDTypeUASdataBase64)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != context.Canceled && err != nil {
			t.Errorf("source error on cancel: got %v, want context.Canceled or nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("source did not exit within 2s of cancel")
	}
}

func TestRunDroneScoutMQTT_IntegrationEndToEnd_AggregatedMessage(t *testing.T) {
	server, brokerURL := startTestBroker(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan DecodedFrame, 8)
	cfg := DroneScoutMQTTConfig{
		URL:         brokerURL,
		Topic:       "#",
		Compression: "none",
	}
	errCh := startSource(t, ctx, cfg, out)

	// One MQTT message with two concatenated JSON objects → two
	// DecodedFrames out.
	publishFixture(t, server, "remoteid/test/data", fixtureDataAggregated)

	df1 := waitForDecodedFrame(t, out, 2*time.Second)
	df2 := waitForDecodedFrame(t, out, 2*time.Second)

	if df1.DroneID != syntheticUASdata1 {
		t.Errorf("df1.DroneID = %q, want %q", df1.DroneID, syntheticUASdata1)
	}
	if df2.DroneID != syntheticUASdata2 {
		t.Errorf("df2.DroneID = %q, want %q", df2.DroneID, syntheticUASdata2)
	}

	cancel()
	<-errCh
}

func TestRunDroneScoutMQTT_IntegrationEndToEnd_SkipsNonDataKinds(t *testing.T) {
	server, brokerURL := startTestBroker(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan DecodedFrame, 8)
	cfg := DroneScoutMQTTConfig{
		URL:         brokerURL,
		Topic:       "#",
		Compression: "none",
	}
	errCh := startSource(t, ctx, cfg, out)

	// status and location messages MUST NOT produce DecodedFrames.
	publishFixture(t, server, "remoteid/test/status", fixtureStatus)
	publishFixture(t, server, "remoteid/test/location", fixtureLocation)

	// Wait a short window; the channel should stay empty.
	select {
	case df := <-out:
		t.Fatalf("non-data message produced an unexpected DecodedFrame: %+v", df)
	case <-time.After(300 * time.Millisecond):
		// expected: nothing on out
	}

	// Now publish a data message; one frame should appear.
	publishFixture(t, server, "remoteid/test/data", fixtureDataSingle)
	df := waitForDecodedFrame(t, out, 2*time.Second)
	if df.DroneID != syntheticUASdata1 {
		t.Errorf("DroneID = %q, want %q", df.DroneID, syntheticUASdata1)
	}

	cancel()
	<-errCh
}

func TestRunDroneScoutMQTT_IntegrationEndToEnd_RespectsSensorIDAllow(t *testing.T) {
	server, brokerURL := startTestBroker(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan DecodedFrame, 8)
	cfg := DroneScoutMQTTConfig{
		URL:           brokerURL,
		Topic:         "#",
		Compression:   "none",
		SensorIDAllow: map[string]bool{"sensor-other-id": true}, // does NOT include syntheticSensorID
	}
	errCh := startSource(t, ctx, cfg, out)

	// data-bt5-single has sensor ID "sensor-synthetic-001"; allowlist
	// does not include it → message is filtered.
	publishFixture(t, server, "remoteid/test/data", fixtureDataSingle)

	select {
	case df := <-out:
		t.Fatalf("disallowed sensor ID produced a DecodedFrame: %+v", df)
	case <-time.After(300 * time.Millisecond):
		// expected: filtered
	}

	cancel()
	<-errCh
}

func TestRunDroneScoutMQTT_IntegrationEndToEnd_CtxCancelReturns(t *testing.T) {
	_, brokerURL := startTestBroker(t)

	ctx, cancel := context.WithCancel(context.Background())

	out := make(chan DecodedFrame, 1)
	cfg := DroneScoutMQTTConfig{
		URL:         brokerURL,
		Topic:       "#",
		Compression: "none",
	}
	errCh := startSource(t, ctx, cfg, out)

	cancel()

	select {
	case err := <-errCh:
		// context.Canceled or nil are both acceptable: paho may
		// disconnect before the context is fully observed.
		if err != context.Canceled && err != nil {
			t.Errorf("source error on cancel: got %v, want context.Canceled or nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("source did not exit within 2s of cancel")
	}
}

func TestRunDroneScoutMQTT_IntegrationEndToEnd_UnknownKindIsSilentlySkipped(t *testing.T) {
	server, brokerURL := startTestBroker(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan DecodedFrame, 8)
	cfg := DroneScoutMQTTConfig{
		URL:         brokerURL,
		Topic:       "#",
		Compression: "none",
	}
	errCh := startSource(t, ctx, cfg, out)

	if err := server.Publish("remoteid/test/data", []byte(`{"protocol":1.0,"some-future-kind":{"foo":"bar"}}`), false, 0); err != nil {
		t.Fatalf("server.Publish: %v", err)
	}

	select {
	case df := <-out:
		t.Fatalf("unknown-kind message produced a DecodedFrame: %+v", df)
	case <-time.After(300 * time.Millisecond):
		// expected: skipped silently
	}

	cancel()
	<-errCh
}

func TestRunDroneScoutMQTT_IntegrationEndToEnd_MalformedPayloadDropsSilently(t *testing.T) {
	server, brokerURL := startTestBroker(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan DecodedFrame, 8)
	cfg := DroneScoutMQTTConfig{
		URL:         brokerURL,
		Topic:       "#",
		Compression: "none",
	}
	errCh := startSource(t, ctx, cfg, out)

	// Malformed JSON should be dropped without crashing or returning
	// from the source. Source MUST keep running.
	if err := server.Publish("remoteid/test/data", []byte("{not json}"), false, 0); err != nil {
		t.Fatalf("server.Publish malformed: %v", err)
	}

	// Source must still receive subsequent valid messages.
	time.Sleep(100 * time.Millisecond) // let the malformed publish drain through the handler
	publishFixture(t, server, "remoteid/test/data", fixtureDataSingle)

	df := waitForDecodedFrame(t, out, 2*time.Second)
	if df.DroneID != syntheticUASdata1 {
		t.Errorf("after-malformed DroneID = %q, want %q", df.DroneID, syntheticUASdata1)
	}

	cancel()
	<-errCh
}
