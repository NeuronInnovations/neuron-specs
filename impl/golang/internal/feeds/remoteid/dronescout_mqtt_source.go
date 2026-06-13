package remoteid

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// DroneScout MQTT source — Stage C-lite (2026-05-13).
//
// Connects to an MQTT broker, subscribes to a topic (default `#`),
// routes inbound payloads through ParseSensorPayload +
// NormalizeToDecodedFrame, and emits DecodedFrames on `out` until
// ctx is cancelled or a fatal MQTT error occurs.
//
// **Scope (Stage C-lite)**: implementation against the existing
// Stage B parser; compression == "none" only; auto-reconnect on
// broker drop (FR-R07 long-lived stream discipline); no LZMA; no
// OpenDroneID byte-level UASdata decoding. The source emits
// DecodedFrames whose DroneID carries the raw base64 UASdata with
// DroneIDType = "uasdata-base64" — the explicit "not-yet-decoded"
// framing.
//
// **Live-vs-fixture classification is operator-side, not source-side.**
// This function does not know whether the broker on the other end is
// a real ds240 sensor or an in-process broker echoing fixtures.
// `feedSource = "live"` is the operator's claim, made via the
// seller binary's `--feed-source=live` flag — and only when the
// operator has independently verified the broker is fed by a real
// sensor in real time. A seller running against an in-process broker
// (e.g., the integration tests in this package) MUST advertise
// `feedSource = "replay"`. See spec 017 FR-R15.
//
// Sibling: `dronescout_json.go` (parser + normalizer).

// DroneScoutMQTTConfig parameterises RunDroneScoutMQTT. URL is
// required; every other field has a documented default.
type DroneScoutMQTTConfig struct {
	// URL is the broker endpoint, e.g. "tcp://broker.lan:1883" or
	// "ssl://broker.lan:8883" (paho prefixes; "mqtt://" / "mqtts://"
	// also accepted). Required.
	URL string

	// Topic is the MQTT topic filter to subscribe to. Default "#"
	// (broker-wide wildcard) is safe only on single-tenant brokers; a
	// narrower filter is recommended for multi-tenant deployments.
	Topic string

	// ClientID is the broker-visible client identifier. Empty → an
	// auto-generated "neuron-seller-<rand>" string.
	ClientID string

	// Username is the broker auth username (optional).
	Username string

	// PasswordEnv names the environment variable that holds the
	// password. **The env-var name, not the password.** The source
	// reads os.Getenv(PasswordEnv) at connect time. The password
	// never appears as a CLI flag value or in logs.
	PasswordEnv string

	// Compression is "none" (default for empty) or "lzma". Stage C-
	// lite only honours "none"; "lzma" returns
	// ErrDroneScoutLZMANotYetSupported from ParseSensorPayload at
	// dispatch time.
	TLS bool

	// Compression is the wire-level payload encoding the sensor uses.
	// "none" (default) or "lzma". Stage C-lite only honours "none".
	Compression string

	// SensorModel is the value stamped into DecodedFrame.Source.
	// Empty → "dronescout-ds240" (per the 2026-05-13 likely-model
	// guidance).
	SensorModel string

	// SensorIDAllow optionally restricts which sensor IDs the source
	// forwards. Empty map → accept all. Useful when a multi-tenant
	// broker carries sensors the seller is not authorised to expose.
	SensorIDAllow map[string]bool

	// ConnectTimeout bounds how long the source waits for the broker
	// to accept the initial CONNECT. Zero → 15 seconds.
	ConnectTimeout time.Duration

	// SubscribeTimeout bounds how long the source waits for the
	// SUBACK after subscribing. Zero → 5 seconds.
	SubscribeTimeout time.Duration

	// OnReady is an optional callback invoked once the MQTT client
	// has successfully connected to the broker AND the topic
	// subscription is established. Useful for tests that need to
	// know when to start publishing fixtures, and for production
	// callers that want to log "MQTT source ready" diagnostics. The
	// callback runs on the source goroutine; do not block in it.
	OnReady func()
}

// validate enforces the minimum invariants and normalises empty
// optional fields to their defaults. Mutations are visible only
// inside RunDroneScoutMQTT (cfg is taken by value).
func (c *DroneScoutMQTTConfig) validate() error {
	if c.URL == "" {
		return errors.New("feeds/remoteid: DroneScoutMQTTConfig.URL is required")
	}
	if c.Compression == "" {
		c.Compression = "none"
	}
	switch c.Compression {
	case "none", "lzma":
		// ok (lzma rejected at ParseSensorPayload dispatch)
	default:
		return fmt.Errorf("feeds/remoteid: DroneScoutMQTTConfig.Compression %q unknown (want none|lzma)", c.Compression)
	}
	if c.Topic == "" {
		c.Topic = "#"
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = 15 * time.Second
	}
	if c.SubscribeTimeout <= 0 {
		c.SubscribeTimeout = 5 * time.Second
	}
	return nil
}

// SourceTag returns the DecodedFrame.Source label this config stamps
// onto frames. Falls back to "dronescout-ds240" (the 2026-05-13
// likely-model default) when SensorModel is empty.
func (c *DroneScoutMQTTConfig) SourceTag() string {
	if c.SensorModel != "" {
		return c.SensorModel
	}
	return "dronescout-ds240"
}

// clientID returns the configured ClientID or a randomised
// "neuron-seller-XXXXXXXX" suffix when unset. Avoids broker-side
// collisions when multiple sellers share a deployment.
func (c *DroneScoutMQTTConfig) clientID() string {
	if c.ClientID != "" {
		return c.ClientID
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("neuron-seller-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("neuron-seller-%x", b)
}

// RunDroneScoutMQTT is the FeedSource-compatible entry point for the
// DroneScout MQTT receiver. The signature matches RunReplay / RunSynth
// so the DApp seller plugs in this source identically:
//
//	source := func(ctx context.Context, out chan<- DecodedFrame) error {
//	    return remoteid.RunDroneScoutMQTT(ctx, cfg, out)
//	}
//
// Behavior:
//
//   - Connects to cfg.URL with paho.mqtt.golang. Validation errors
//     (empty URL, unknown compression) return immediately. Connect
//     failures return wrapped with the broker URL. Connect timeout is
//     cfg.ConnectTimeout (default 15s).
//   - Subscribes to cfg.Topic (default `#`) at QoS 0. Subscribe
//     timeout is cfg.SubscribeTimeout (default 5s).
//   - On each inbound message: ParseSensorPayload → for each
//     data-kind SensorMessage, NormalizeToDecodedFrame → emit on
//     `out`. status / location / aircraft / mobile-network kinds are
//     observed but produce no DecodedFrame.
//   - Auto-reconnects on broker drop (paho's SetAutoReconnect=true);
//     subscriptions are re-established automatically (CleanSession=
//     false). This honours FR-R07 "MUST NOT close streams on
//     temporary upstream issues".
//   - Returns ctx.Err() when ctx is cancelled, after a clean
//     disconnect with a 250ms quiesce period.
//   - Race-safe: the only shared state with the paho callback
//     goroutine is the `out` channel; channel sends are atomic.
//     Callback uses a non-blocking select with ctx.Done() so it
//     never blocks indefinitely.
//
// Stage C-lite limitations (called out for the next stage):
//
//   - Compression: "none" only. "lzma" is accepted by validate() but
//     rejected at parse time with ErrDroneScoutLZMANotYetSupported.
//   - TLS: paho infers from URL scheme (`mqtts://` / `ssl://`). The
//     cfg.TLS field is retained as informational metadata for the
//     heartbeat layer.
//   - Diagnostics: malformed payloads and per-frame normalize errors
//     are silently dropped. Stage C-full will plumb a logger.
//   - The function does NOT claim feedSource=live. Whether the run
//     is live or replay is the operator's claim, made via the
//     seller binary's --feed-source flag.
func RunDroneScoutMQTT(ctx context.Context, cfg DroneScoutMQTTConfig, out chan<- DecodedFrame) error {
	if err := cfg.validate(); err != nil {
		return err
	}

	// Snapshot config values for the handler closure. Capture by
	// value so the callback goroutine sees consistent state even if
	// the caller's cfg were to mutate (it shouldn't; we take cfg by
	// value but be defensive).
	sourceTag := cfg.SourceTag()
	compression := cfg.Compression
	sensorAllow := cfg.SensorIDAllow

	handler := func(_ mqtt.Client, msg mqtt.Message) {
		msgs, err := ParseSensorPayload(msg.Payload(), compression)
		if err != nil {
			// TODO(stage-c-full) diagnostics: plumb a logger so the
			// operator sees malformed-payload events. For now we
			// drop silently to avoid stdlib log spam in tests.
			return
		}
		for _, m := range msgs {
			if m.Kind != SensorMessageData {
				continue
			}
			if m.Data == nil {
				continue
			}
			if len(sensorAllow) > 0 && !sensorAllow[m.Data.SensorID] {
				continue
			}
			df, err := NormalizeToDecodedFrame(m, sourceTag)
			if err != nil {
				continue
			}
			select {
			case out <- df:
			case <-ctx.Done():
				return
			}
		}
	}

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.URL).
		SetClientID(cfg.clientID()).
		SetKeepAlive(60 * time.Second).
		SetConnectTimeout(cfg.ConnectTimeout).
		SetAutoReconnect(true).
		SetCleanSession(false).
		SetOrderMatters(false).
		SetMaxReconnectInterval(30 * time.Second).
		SetDefaultPublishHandler(handler)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		if cfg.PasswordEnv != "" {
			if pwd := os.Getenv(cfg.PasswordEnv); pwd != "" {
				opts.SetPassword(pwd)
			}
		}
	}

	// TLS: paho infers from URL scheme. We set a minimum-TLS-1.2
	// config when the URL scheme requests TLS, in case the operator
	// pointed at a "mqtts://" / "ssl://" broker — paho's default
	// TLSConfig is nil which selects all-protocol fallback.
	if isTLSScheme(cfg.URL) {
		opts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
	}

	client := mqtt.NewClient(opts)

	connectTok := client.Connect()
	if ok := connectTok.WaitTimeout(cfg.ConnectTimeout); !ok {
		return fmt.Errorf("feeds/remoteid: mqtt connect timeout after %s (broker=%s)", cfg.ConnectTimeout, cfg.URL)
	}
	if err := connectTok.Error(); err != nil {
		return fmt.Errorf("feeds/remoteid: mqtt connect: %w (broker=%s)", err, cfg.URL)
	}
	defer client.Disconnect(250)

	subTok := client.Subscribe(cfg.Topic, 0, handler)
	if ok := subTok.WaitTimeout(cfg.SubscribeTimeout); !ok {
		return fmt.Errorf("feeds/remoteid: mqtt subscribe timeout after %s (topic=%q)", cfg.SubscribeTimeout, cfg.Topic)
	}
	if err := subTok.Error(); err != nil {
		return fmt.Errorf("feeds/remoteid: mqtt subscribe: %w (topic=%q)", err, cfg.Topic)
	}

	if cfg.OnReady != nil {
		cfg.OnReady()
	}

	<-ctx.Done()
	return ctx.Err()
}

// isTLSScheme returns true if the URL string starts with a paho-
// recognised TLS scheme.
func isTLSScheme(url string) bool {
	return strings.HasPrefix(url, "ssl://") ||
		strings.HasPrefix(url, "tls://") ||
		strings.HasPrefix(url, "mqtts://") ||
		strings.HasPrefix(url, "wss://")
}
