package remoteid

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Unit tests for DroneScoutMQTTConfig validation and the connect-error
// surface. End-to-end fixture-driven tests live in
// dronescout_mqtt_integration_test.go (uses an in-process mochi-mqtt
// broker).
//
// Parser + normalizer tests live in dronescout_json_test.go (Stage B).

func TestRunDroneScoutMQTT_RejectsEmptyURL(t *testing.T) {
	cfg := DroneScoutMQTTConfig{Topic: "#"}
	out := make(chan DecodedFrame, 1)
	err := RunDroneScoutMQTT(context.Background(), cfg, out)
	if err == nil {
		t.Fatal("expected validation error for empty URL; got nil")
	}
	if !strings.Contains(err.Error(), "URL is required") {
		t.Errorf("expected URL-required error, got %v", err)
	}
}

func TestRunDroneScoutMQTT_RejectsUnknownCompression(t *testing.T) {
	cfg := DroneScoutMQTTConfig{
		URL:         "tcp://127.0.0.1:1",
		Compression: "zstd",
	}
	out := make(chan DecodedFrame, 1)
	err := RunDroneScoutMQTT(context.Background(), cfg, out)
	if err == nil {
		t.Fatal("expected validation error for unknown compression; got nil")
	}
	if !strings.Contains(err.Error(), "Compression") {
		t.Errorf("expected compression-validation error, got %v", err)
	}
}

func TestRunDroneScoutMQTT_ReturnsConnectErrorWhenBrokerUnreachable(t *testing.T) {
	// Port 1 is a privileged port that no listener can bind to (without
	// root), and is highly unlikely to have an active listener on the
	// test host. paho returns "connection refused" almost immediately.
	cfg := DroneScoutMQTTConfig{
		URL:            "tcp://127.0.0.1:1",
		Topic:          "#",
		Compression:    "none",
		ConnectTimeout: 2 * time.Second,
	}
	out := make(chan DecodedFrame, 1)
	err := RunDroneScoutMQTT(context.Background(), cfg, out)
	if err == nil {
		t.Fatal("expected mqtt connect error against unreachable broker; got nil")
	}
	if !strings.Contains(err.Error(), "mqtt connect") {
		t.Errorf("expected error to mention \"mqtt connect\", got %v", err)
	}
}

func TestDroneScoutMQTTConfig_ValidateNormalisesCompression(t *testing.T) {
	c := DroneScoutMQTTConfig{URL: "tcp://localhost:1883"}
	if err := c.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if c.Compression != "none" {
		t.Errorf("validate() should normalise empty Compression to \"none\"; got %q", c.Compression)
	}
}

func TestDroneScoutMQTTConfig_ValidateNormalisesTopic(t *testing.T) {
	c := DroneScoutMQTTConfig{URL: "tcp://localhost:1883"}
	if err := c.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if c.Topic != "#" {
		t.Errorf("validate() should normalise empty Topic to \"#\"; got %q", c.Topic)
	}
}

func TestDroneScoutMQTTConfig_ValidateNormalisesTimeouts(t *testing.T) {
	c := DroneScoutMQTTConfig{URL: "tcp://localhost:1883"}
	if err := c.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if c.ConnectTimeout <= 0 {
		t.Errorf("ConnectTimeout default = %v, want >0", c.ConnectTimeout)
	}
	if c.SubscribeTimeout <= 0 {
		t.Errorf("SubscribeTimeout default = %v, want >0", c.SubscribeTimeout)
	}
}

func TestDroneScoutMQTTConfig_SourceTagDefault(t *testing.T) {
	c := DroneScoutMQTTConfig{}
	if got, want := c.SourceTag(), "dronescout-ds240"; got != want {
		t.Errorf("default SourceTag = %q, want %q", got, want)
	}
}

func TestDroneScoutMQTTConfig_SourceTagOverride(t *testing.T) {
	c := DroneScoutMQTTConfig{SensorModel: "dronescout-ds230"}
	if got, want := c.SourceTag(), "dronescout-ds230"; got != want {
		t.Errorf("override SourceTag = %q, want %q", got, want)
	}
}

func TestDroneScoutMQTTConfig_ClientIDDefaultIsRandom(t *testing.T) {
	c1 := DroneScoutMQTTConfig{}
	c2 := DroneScoutMQTTConfig{}
	id1 := c1.clientID()
	id2 := c2.clientID()
	if !strings.HasPrefix(id1, "neuron-seller-") {
		t.Errorf("clientID prefix = %q, want \"neuron-seller-\"", id1)
	}
	if id1 == id2 {
		t.Errorf("clientID should be random; got identical %q twice", id1)
	}
}

func TestDroneScoutMQTTConfig_ClientIDOverride(t *testing.T) {
	c := DroneScoutMQTTConfig{ClientID: "my-explicit-id"}
	if got, want := c.clientID(), "my-explicit-id"; got != want {
		t.Errorf("clientID = %q, want %q", got, want)
	}
}
