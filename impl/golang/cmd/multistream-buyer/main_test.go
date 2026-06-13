package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p/core/host"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/adsb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	feedsremoteid "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/remoteid"
	sbs "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// syncBuffer is a goroutine-safe wrapper around bytes.Buffer for tests
// that scan stderr concurrently while run() is executing.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// testLogWriter pipes io.Writer output into t.Logf for debugging.
type testLogWriter struct{ t *testing.T }

func (w testLogWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

func newTestLogger(t *testing.T) *log.Logger {
	return log.New(testLogWriter{t: t}, "", log.LstdFlags)
}

// ─── ParseSeller — flag-parsing tests ──────────────────────────────────

func TestParseSeller_FixtureDirect_BasicAdsb(t *testing.T) {
	t.Parallel()
	spec, err := ParseSeller("role=adsb,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmExample")
	require.NoError(t, err)
	assert.Equal(t, "adsb", spec.Role)
	assert.Equal(t, "/ip4/127.0.0.1/tcp/4001/p2p/QmExample", spec.Multiaddr)
	assert.Equal(t, "", spec.EVM)
	assert.Equal(t, adsb.ProtocolBaseStation, spec.Protocol, "adsb role should default to ProtocolBaseStation")
	assert.Equal(t, "/jetvision/basestation/1.0.0", spec.Protocol, "adsb default must be the JetVision path, not the pre-rename /adsb/ path")
}

func TestParseSeller_FixtureDirect_BasicRemoteID(t *testing.T) {
	t.Parallel()
	spec, err := ParseSeller("role=remoteid,multiaddr=/ip4/127.0.0.1/tcp/4002/p2p/QmExample")
	require.NoError(t, err)
	assert.Equal(t, "remoteid", spec.Role)
	assert.Equal(t, remoteid.ProtocolBasestation, spec.Protocol, "remoteid role should default to ProtocolBasestation")
	assert.Equal(t, "/ds240/basestation/1.0.0", spec.Protocol, "remoteid default must be the DS240 path, not the pre-rename /remoteid/ path")
}

func TestParseSeller_ProtocolOverride(t *testing.T) {
	t.Parallel()
	spec, err := ParseSeller("role=remoteid,multiaddr=/ip4/127.0.0.1/tcp/4002/p2p/QmExample,protocol=/ds240/raw/1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "/ds240/raw/1.0.0", spec.Protocol)
}

func TestParseSeller_EvmMode(t *testing.T) {
	t.Parallel()
	spec, err := ParseSeller("role=adsb,evm=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28", spec.EVM)
	assert.Equal(t, "", spec.Multiaddr)
}

func TestParseSeller_UnknownRole(t *testing.T) {
	t.Parallel()
	_, err := ParseSeller("role=foo,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmX")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown role")
}

func TestParseSeller_UnknownKey(t *testing.T) {
	t.Parallel()
	_, err := ParseSeller("role=adsb,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmX,bogus=value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown key")
}

func TestParseSeller_MissingRole(t *testing.T) {
	t.Parallel()
	_, err := ParseSeller("multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmX")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required key: role")
}

func TestValidateForMode_RejectsBothMultiaddrAndEvmInFixtureDirect(t *testing.T) {
	t.Parallel()
	// Phase 3B: ParseSeller accepts multiaddr+evm together because
	// eip8004-registry + commerce-mode=full uses both (multiaddr= as
	// override). validateForMode is the gate that rejects the combo in
	// modes where it's nonsensical.
	spec, err := ParseSeller("role=adsb,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmX,evm=0xdead")
	require.NoError(t, err)
	require.Equal(t, "/ip4/127.0.0.1/tcp/4001/p2p/QmX", spec.Multiaddr)
	require.Equal(t, "0xdead", spec.EVM)

	require.Error(t, spec.validateForMode(modeFixtureDirect, commerceModeRegistrationOnly))
	require.Error(t, spec.validateForMode(modeEIP8004Registry, commerceModeRegistrationOnly))
	// In commerce-mode=full both fields coexist legitimately (override path).
	require.NoError(t, spec.validateForMode(modeEIP8004Registry, commerceModeFull))
}

func TestParseSeller_RequiresOneOf(t *testing.T) {
	t.Parallel()
	_, err := ParseSeller("role=adsb")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "either multiaddr= or evm=")
}

func TestParseSeller_MalformedPair(t *testing.T) {
	t.Parallel()
	_, err := ParseSeller("role=adsb,not-a-pair")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "malformed key=value pair")
}

func TestParseSeller_EmptyValue(t *testing.T) {
	t.Parallel()
	_, err := ParseSeller("role=,multiaddr=/ip4/127.0.0.1/tcp/4001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty value for key")
}

func TestParseSeller_EmptyString(t *testing.T) {
	t.Parallel()
	_, err := ParseSeller("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// ─── run() flag-validation tests ───────────────────────────────────────

func TestRun_ZeroSellers(t *testing.T) {
	t.Parallel()
	stderr := &syncBuffer{}
	rc := run([]string{}, map[string]string{}, &bytes.Buffer{}, stderr, Deps{})
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "at least one --seller required")
}

func TestRun_OneSellerFixtureDirect_BadMultiaddr(t *testing.T) {
	t.Parallel()
	// Multiaddr survives ParseSeller + validateForMode but the
	// resolveSeller call's peer.AddrInfoFromString rejects the malformed
	// peerID, yielding a runtime error (exit 1) — NOT a flag error
	// (exit 2). The distinction matters for operators wiring up the
	// CLI from scripts.
	stderr := &syncBuffer{}
	rc := run(
		[]string{"--seller=role=adsb,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmInvalidPeerID"},
		map[string]string{}, &bytes.Buffer{}, stderr, Deps{},
	)
	assert.Equal(t, 1, rc, "exit code")
	assert.Contains(t, stderr.String(), "resolve --seller")
}

func TestRun_TwoSellersMixedRoles_ParseSucceeds(t *testing.T) {
	t.Parallel()
	specs := []string{
		"role=adsb,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmA",
		"role=remoteid,multiaddr=/ip4/127.0.0.1/tcp/4002/p2p/QmB",
	}
	parsed := make([]SellerSpec, 0, len(specs))
	for _, s := range specs {
		spec, err := ParseSeller(s)
		require.NoError(t, err)
		parsed = append(parsed, spec)
	}
	require.Len(t, parsed, 2)
	assert.Equal(t, "adsb", parsed[0].Role)
	assert.Equal(t, "remoteid", parsed[1].Role)
}

func TestRun_InvalidRole(t *testing.T) {
	t.Parallel()
	stderr := &syncBuffer{}
	rc := run(
		[]string{"--seller=role=foo,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmX"},
		map[string]string{}, &bytes.Buffer{}, stderr, Deps{},
	)
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "unknown role")
}

func TestRun_Eip8004RequiresEvm(t *testing.T) {
	t.Parallel()
	stderr := &syncBuffer{}
	rc := run(
		[]string{
			"--mode=eip8004-registry",
			"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
			"--seller=role=adsb,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmX",
		},
		map[string]string{}, &bytes.Buffer{}, stderr, Deps{},
	)
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "eip8004-registry mode requires evm=")
}

func TestRun_FixtureDirectRejectsEvm(t *testing.T) {
	t.Parallel()
	stderr := &syncBuffer{}
	rc := run(
		[]string{
			"--mode=fixture-direct",
			"--seller=role=adsb,evm=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		},
		map[string]string{}, &bytes.Buffer{}, stderr, Deps{},
	)
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "fixture-direct mode requires multiaddr=")
}

func TestRun_CommerceModeFullRequiresRegistry(t *testing.T) {
	t.Parallel()
	// Phase 3B: --commerce-mode=full requires --mode=eip8004-registry
	// (the registry is the only seller-discovery path the orchestrator
	// supports). fixture-direct + full is a flag misconfiguration.
	stderr := &syncBuffer{}
	rc := run(
		[]string{
			"--commerce-mode=full",
			"--seller=role=adsb,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmX",
		},
		map[string]string{}, &bytes.Buffer{}, stderr, Deps{},
	)
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "--commerce-mode=full requires --mode=eip8004-registry")
}

func TestRun_UnknownMode(t *testing.T) {
	t.Parallel()
	stderr := &syncBuffer{}
	rc := run(
		[]string{
			"--mode=registry-something",
			"--seller=role=adsb,multiaddr=/ip4/127.0.0.1/tcp/4001/p2p/QmX",
		},
		map[string]string{}, &bytes.Buffer{}, stderr, Deps{},
	)
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "unknown --mode")
}

func TestRun_Eip8004RequiresRegistryAddress(t *testing.T) {
	t.Parallel()
	stderr := &syncBuffer{}
	rc := run(
		[]string{
			"--mode=eip8004-registry",
			"--seller=role=adsb,evm=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
		},
		map[string]string{}, &bytes.Buffer{}, stderr, Deps{},
	)
	assert.Equal(t, 2, rc)
	assert.Contains(t, stderr.String(), "requires --registry-address")
}

// ─── Integration test: two mock sellers, one multistream-buyer ─────────

// mockSeller stands up a libp2p host with a single stream handler that
// pumps a slice of canonical JSON frames through delivery.FrameWriter.
// Mirrors the pattern in internal/dapp/{adsb,remoteid}/integration_test.go.
type mockSeller struct {
	host     host.Host
	addrInfo peer.AddrInfo
}

func makeECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return priv
}

func newNeuronKey(t *testing.T) *keylib.NeuronPrivateKey {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	return &k
}

func mustBlockchainKey(t *testing.T, key *keylib.NeuronPrivateKey) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := key.ToBlockchainKey()
	require.NoError(t, err)
	return priv
}

// startMockSeller registers a stream handler on a fresh libp2p host that
// writes each frame in `frames` per opened stream (looping), then exits
// when the test context is cancelled. The returned mockSeller's AddrInfo
// is the multiaddr the buyer should dial.
func startMockSeller(
	t *testing.T,
	ctx context.Context,
	protocolID string,
	frames [][]byte,
) mockSeller {
	t.Helper()

	key := makeECDSAKey(t)
	h, err := delivery.NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })

	h.SetStreamHandler(protocol.ID(protocolID), func(s libp2pnetwork.Stream) {
		defer s.Close()
		writer := delivery.NewFrameWriter(s)
		// Keep emitting until ctx cancels or the buyer closes the stream.
		for ctx.Err() == nil {
			for _, f := range frames {
				if ctx.Err() != nil {
					return
				}
				if err := writer.WriteFrame(f); err != nil {
					return
				}
				select {
				case <-time.After(10 * time.Millisecond):
				case <-ctx.Done():
					return
				}
			}
		}
	})

	return mockSeller{
		host:     h,
		addrInfo: peer.AddrInfo{ID: h.ID(), Addrs: h.Addrs()},
	}
}

// addrInfoString returns the full /p2p/<peerID> multiaddr string the
// multistream-buyer CLI parses for fixture-direct mode.
func addrInfoString(t *testing.T, info peer.AddrInfo) string {
	t.Helper()
	require.NotEmpty(t, info.Addrs, "mockSeller must have at least one listen multiaddr")
	return info.Addrs[0].String() + "/p2p/" + info.ID.String()
}

// canonicalNormalizedTrackJSON returns a valid NormalizedTrack canonical
// JSON blob fid-display will accept. Uses the public package types so
// the test asserts wire compatibility.
func canonicalNormalizedTrackJSON(t *testing.T, icao string, lat, lon float64) []byte {
	t.Helper()
	track := adsb.NormalizedTrack{
		Type:       adsb.FrameType,
		Version:    adsb.FrameVersion,
		ObservedAt: time.Now().UTC(),
		Source:     "adsb",
		EntityType: "aircraft",
		EntityID:   icao,
		Position: &adsb.NormalizedPosition{
			Lat:         lat,
			Lon:         lon,
			AltitudeM:   1000,
			AltitudeSet: true,
		},
	}
	data, err := json.Marshal(track)
	require.NoError(t, err)
	return data
}

// canonicalRemoteIdFrameJSON returns a canonical RemoteIdFrame JSON blob
// stamped with `source="basestation-tcp-synthetic"` so the integration
// test can assert the inner-source field survives the multistream-buyer
// envelope round-trip.
func canonicalRemoteIdFrameJSON(t *testing.T, droneID string) []byte {
	t.Helper()
	df := feedsremoteid.DecodedFrame{
		ObservedAt:  time.Now().UTC(),
		DroneID:     droneID,
		DroneIDType: "serial-number",
		Position: &feedsremoteid.Position{
			Lat: 51.4775,
			Lon: -0.4614,
			Alt: 100,
			Fix: "3D",
		},
		Source: "basestation-tcp-synthetic",
	}
	frame := remoteid.FromDecoded(df)
	data, err := json.Marshal(frame)
	require.NoError(t, err)
	return data
}

// startTCPCollector listens on an ephemeral loopback port; every JSONL
// line received from any accepted connection is forwarded to lineCh.
// Returns the listener (caller calls Close to tear down) and its
// "127.0.0.1:NNNNN" address. Per the existing TCP sink contract, the
// sink connects lazily so the listener can be brought up first.
func startTCPCollector(t *testing.T, lineCh chan<- []byte) (net.Listener, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				for scanner.Scan() {
					line := append([]byte(nil), scanner.Bytes()...)
					select {
					case lineCh <- line:
					case <-time.After(2 * time.Second):
						return
					}
				}
			}(c)
		}
	}()

	return l, l.Addr().String()
}

// TestIntegration_TwoMockSellers_FixtureDirect spins up:
//   - an ADS-B mock seller on /jetvision/basestation/1.0.0 emitting NormalizedTrack JSON
//   - a Remote-ID mock seller on /ds240/basestation/1.0.0 emitting
//     RemoteIdFrame JSON stamped source="basestation-tcp-synthetic"
//   - a TCP listener on an ephemeral port collecting consolidated output
//
// Then it runs the multistream-buyer in --mode=fixture-direct against
// both sellers and asserts:
//   - at least one TaggedFrame arrives with source="adsb"
//   - at least one TaggedFrame arrives with source="remote-id"
//   - the remote-id frame's inner frame.source MUST equal
//     "basestation-tcp-synthetic" (the seller-stamped value survives
//     the verbatim-forward path; this is what drives the fid-display
//     SYN badge in Phase 1)
func TestIntegration_TwoMockSellers_FixtureDirect(t *testing.T) {
	// Race-detector + libp2p QUIC handshake can add a few seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Mock sellers — each loops through a small frame set.
	adsbFrames := [][]byte{
		canonicalNormalizedTrackJSON(t, "ABCDEF", 51.50, -0.10),
		canonicalNormalizedTrackJSON(t, "123456", 51.51, -0.11),
	}
	ridFrames := [][]byte{
		canonicalRemoteIdFrameJSON(t, "DRONE-001"),
		canonicalRemoteIdFrameJSON(t, "DRONE-002"),
	}
	adsbSeller := startMockSeller(t, ctx, adsb.ProtocolBaseStation, adsbFrames)
	ridSeller := startMockSeller(t, ctx, remoteid.ProtocolBasestation, ridFrames)

	// TCP collector for the consolidated output sink.
	lineCh := make(chan []byte, 256)
	listener, addr := startTCPCollector(t, lineCh)
	defer listener.Close()

	// SignalCh for clean shutdown. We don't actually fire SIGINT — we
	// rely on --frame-limit so each session terminates on its own.
	sigCh := make(chan os.Signal, 1)
	args := []string{
		"--mode=fixture-direct",
		"--output=tcp:" + addr,
		"--frame-limit=4",
		"--listen=/ip4/127.0.0.1/udp/0/quic-v1",
		"--seller=role=adsb,multiaddr=" + addrInfoString(t, adsbSeller.addrInfo) +
			",protocol=" + adsb.ProtocolBaseStation,
		"--seller=role=remoteid,multiaddr=" + addrInfoString(t, ridSeller.addrInfo) +
			",protocol=" + remoteid.ProtocolBasestation,
	}

	stderr := &syncBuffer{}
	rcCh := make(chan int, 1)
	go func() {
		rc := run(args, map[string]string{}, &bytes.Buffer{}, stderr, Deps{
			SignalCh: sigCh,
		})
		rcCh <- rc
	}()

	// Collect lines until we see at least one of each Source — or timeout.
	var (
		sawAdsb             bool
		sawRemoteID         bool
		ridInnerSourceMatch bool
		collectedLines      int
		maxCollect          = 64
		deadline            = time.Now().Add(25 * time.Second)
	)
	for time.Now().Before(deadline) && collectedLines < maxCollect &&
		!(sawAdsb && sawRemoteID && ridInnerSourceMatch) {
		select {
		case line := <-lineCh:
			collectedLines++
			var tf struct {
				Source string          `json:"source"`
				Type   string          `json:"type"`
				Frame  json.RawMessage `json:"frame"`
			}
			if err := json.Unmarshal(line, &tf); err != nil {
				t.Logf("malformed line: %v", err)
				continue
			}
			switch tf.Source {
			case "adsb":
				sawAdsb = true
			case "remote-id":
				sawRemoteID = true
				var inner struct {
					Source string `json:"source"`
				}
				if jerr := json.Unmarshal(tf.Frame, &inner); jerr == nil {
					if strings.Contains(inner.Source, "basestation-tcp-synthetic") {
						ridInnerSourceMatch = true
					}
				}
			}
		case <-time.After(500 * time.Millisecond):
			// Loop check on deadline.
		}
	}

	// Trigger shutdown defensively in case one session never got an
	// addr to dial (CI flake): signal-channel close is interpreted as a
	// shutdown signal by the buyer.
	select {
	case sigCh <- syscall.SIGINT:
	default:
	}

	// Wait for run() to exit. Once both frame-limits fire, sessionDone
	// closes and run() returns 0.
	select {
	case rc := <-rcCh:
		assert.Equal(t, 0, rc, "expected clean exit; stderr=%s", stderr.String())
	case <-time.After(15 * time.Second):
		t.Fatalf("run() did not exit; stderr=%s", stderr.String())
	}

	require.True(t, sawAdsb,
		"expected at least one TaggedFrame source=\"adsb\" after %d lines; stderr=%s",
		collectedLines, stderr.String())
	require.True(t, sawRemoteID,
		"expected at least one TaggedFrame source=\"remote-id\" after %d lines; stderr=%s",
		collectedLines, stderr.String())
	require.True(t, ridInnerSourceMatch,
		"expected remote-id frame to carry inner frame.source=\"basestation-tcp-synthetic\" after %d lines; stderr=%s",
		collectedLines, stderr.String())
}

// ─── resolveSeller eip8004-registry tests ──────────────────────────────

// TestResolveSeller_Eip8004_LookupSeed verifies that a real seller
// registration produced via remoteid.RegisterSeller can be discovered
// via the multistream-buyer's resolveSeller path. Uses the local
// MemoryRegistryContract so no live RPC is needed.
func TestResolveSeller_Eip8004_LookupSeed(t *testing.T) {
	t.Parallel()

	sellerKey, err := keylib.NeuronPrivateKeyFromHex(
		"59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
	)
	require.NoError(t, err)

	descriptor, err := remoteid.BuildServiceDescriptor(remoteid.DescriptorOptions{
		ChildKey:   &sellerKey,
		Multiaddrs: []string{"/ip4/127.0.0.1/tcp/4001"},
	})
	require.NoError(t, err)

	contract := registry.NewMemoryRegistryContract()
	contract.SetPendingOwner(common.BytesToAddress(sellerKey.PublicKey().EVMAddress().Bytes()))
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	_, err = remoteid.RegisterSeller(context.Background(), &sellerKey, descriptor, registryAddr, 296, contract)
	require.NoError(t, err)

	pid, err := sellerKey.PublicKey().PeerID()
	require.NoError(t, err)

	logger := newTestLogger(t)
	spec := SellerSpec{
		Role:     "remoteid",
		EVM:      sellerKey.PublicKey().EVMAddress().Hex(),
		Protocol: remoteid.ProtocolBasestation, // operator-picked override
	}
	rs, err := resolveSeller(
		context.Background(), spec, modeEIP8004Registry,
		contract, registryAddr, 296, logger,
	)
	require.NoError(t, err)
	// keylib.PeerID wraps the libp2p peer.ID; compare via the canonical
	// string representation (both serialise identically per Spec 002).
	assert.Equal(t, pid.String(), rs.AddrInfo.ID.String(), "resolved peerID must match registered peerID")
	assert.NotEmpty(t, rs.AddrInfo.Addrs, "resolved AddrInfo must carry the registered multiaddr")
	assert.Equal(t, remoteid.ProtocolBasestation, rs.Protocol,
		"operator-supplied protocol must win over AgentURI's primary p2p protocol")
}

// TestResolveSeller_Eip8004_LookupNotFound asserts the failure mode
// when the EVM address has no registration — we want a clear error,
// not a panic.
func TestResolveSeller_Eip8004_LookupNotFound(t *testing.T) {
	t.Parallel()

	contract := registry.NewMemoryRegistryContract()
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	logger := newTestLogger(t)
	spec := SellerSpec{
		Role:     "adsb",
		EVM:      "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Protocol: adsb.ProtocolBaseStation,
	}
	_, err = resolveSeller(
		context.Background(), spec, modeEIP8004Registry,
		contract, registryAddr, 296, logger,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LookupRegistration")
}

// ============================================================================
// Phase 3B full-commerce orchestration
// ============================================================================

type fullCommerceSellerResult struct {
	role       string
	finalState payment.AgreementState
	err        error
}

func topicConfig(stdIn, stdOut, stdErr topic.TopicRef) map[string]map[string]any {
	return map[string]map[string]any{
		"stdIn":  {"topicId": stdIn.Locator()},
		"stdOut": {"topicId": stdOut.Locator()},
		"stdErr": {"topicId": stdErr.Locator()},
	}
}

func hostMultiaddrs(h host.Host) []string {
	out := make([]string, 0, len(h.Addrs()))
	for _, addr := range h.Addrs() {
		out = append(out, addr.String())
	}
	return out
}

func startAdsbFullCommerceSeller(
	t *testing.T,
	ctx context.Context,
	bus topic.TopicAdapter,
	escrow payment.EscrowAdapter,
	contract *registry.MemoryRegistryContract,
	registryAddr keylib.EVMAddress,
	withOverride bool,
) (SellerSpec, <-chan fullCommerceSellerResult) {
	t.Helper()

	sellerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-seller-stdin"})
	require.NoError(t, err)
	sellerStdOut, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-seller-stdout"})
	require.NoError(t, err)
	sellerStdErr, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-seller-stderr"})
	require.NoError(t, err)

	sellerKey := newNeuronKey(t)
	sellerPriv := mustBlockchainKey(t, sellerKey)
	sellerHost, err := delivery.NewLibp2pHost(sellerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sellerHost.Close() })
	require.NotEmpty(t, sellerHost.Addrs())

	descriptor, err := adsb.BuildServiceDescriptor(adsb.DescriptorOptions{
		ChildKey:     sellerKey,
		ChainID:      296,
		FeedSource:   adsb.FeedSourceSynthetic,
		CommerceMode: adsb.CommerceModeFull,
		TopicConfig:  topicConfig(sellerStdIn, sellerStdOut, sellerStdErr),
		Multiaddrs:   hostMultiaddrs(sellerHost),
	})
	require.NoError(t, err)

	contract.SetPendingOwner(common.BytesToAddress(sellerKey.PublicKey().EVMAddress().Bytes()))
	_, err = adsb.RegisterSeller(ctx, sellerKey, descriptor, registryAddr, 296, contract)
	require.NoError(t, err)

	running, err := adsb.Start(ctx, adsb.SellerConfig{
		Host:   sellerHost,
		Source: adsb.BaseStationSynthSource(sbs.SynthOptions{Aircraft: 1, Fps: 50}),
	})
	require.NoError(t, err)
	t.Cleanup(running.Cancel)

	resultCh := make(chan fullCommerceSellerResult, 1)
	go func() {
		res, err := adsb.RunSellerSession(ctx, adsb.SellerSessionOptions{
			Key:         sellerKey,
			Adapter:     bus,
			SellerStdIn: sellerStdIn,
			Descriptor:  descriptor,
			Host:        sellerHost,
			Escrow:      escrow,
			Mode:        adsb.CommerceModeFull,
			Logger:      newTestLogger(t),
			FrameSummary: func() (uint64, uint64, uint64) {
				return 4, 1, 1
			},
		})
		resultCh <- fullCommerceSellerResult{role: roleAdsb, finalState: res.FinalState, err: err}
	}()

	spec := SellerSpec{
		Role: roleAdsb,
		EVM:  sellerKey.PublicKey().EVMAddress().Hex(),
	}
	if withOverride {
		spec.Multiaddr = sellerHost.Addrs()[0].String() + "/p2p/" + sellerHost.ID().String()
		spec.AllowOverride = true
	}
	return spec, resultCh
}

func startRemoteIDFullCommerceSeller(
	t *testing.T,
	ctx context.Context,
	bus topic.TopicAdapter,
	escrow payment.EscrowAdapter,
	contract *registry.MemoryRegistryContract,
	registryAddr keylib.EVMAddress,
) (SellerSpec, <-chan fullCommerceSellerResult) {
	t.Helper()

	sellerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "remoteid-seller-stdin"})
	require.NoError(t, err)
	sellerStdOut, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "remoteid-seller-stdout"})
	require.NoError(t, err)
	sellerStdErr, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "remoteid-seller-stderr"})
	require.NoError(t, err)

	sellerKey := newNeuronKey(t)
	sellerPriv := mustBlockchainKey(t, sellerKey)
	sellerHost, err := delivery.NewLibp2pHost(sellerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sellerHost.Close() })
	require.NotEmpty(t, sellerHost.Addrs())

	descriptor, err := remoteid.BuildServiceDescriptor(remoteid.DescriptorOptions{
		ChildKey:     sellerKey,
		ChainID:      296,
		FeedSource:   remoteid.FeedSourceSynthetic,
		CommerceMode: remoteid.CommerceModeFull,
		TopicConfig:  topicConfig(sellerStdIn, sellerStdOut, sellerStdErr),
		Multiaddrs:   hostMultiaddrs(sellerHost),
		Catalog:      remoteid.CatalogOptions{IncludeBasestation: true},
	})
	require.NoError(t, err)

	contract.SetPendingOwner(common.BytesToAddress(sellerKey.PublicKey().EVMAddress().Bytes()))
	_, err = remoteid.RegisterSeller(ctx, sellerKey, descriptor, registryAddr, 296, contract)
	require.NoError(t, err)

	running, err := remoteid.Start(ctx, remoteid.SellerConfig{
		Host:        sellerHost,
		Source:      remoteid.SynthSource(feedsremoteid.SynthOptions{FPS: 50, DroneCount: 1}),
		ProtocolIDs: []string{remoteid.ProtocolRaw, remoteid.ProtocolBasestation},
	})
	require.NoError(t, err)
	t.Cleanup(running.Cancel)

	resultCh := make(chan fullCommerceSellerResult, 1)
	go func() {
		res, err := remoteid.RunSellerSession(ctx, remoteid.SellerSessionOptions{
			Key:         sellerKey,
			Adapter:     bus,
			SellerStdIn: sellerStdIn,
			Descriptor:  descriptor,
			Host:        sellerHost,
			Escrow:      escrow,
			Mode:        remoteid.CommerceModeFull,
			Logger:      newTestLogger(t),
			FrameSummary: func() (uint64, uint64, uint64) {
				return 4, 1, 1
			},
		})
		resultCh <- fullCommerceSellerResult{role: roleRemoteID, finalState: res.FinalState, err: err}
	}()

	return SellerSpec{
		Role: roleRemoteID,
		EVM:  sellerKey.PublicKey().EVMAddress().Hex(),
	}, resultCh
}

func assertSellerCompleted(t *testing.T, ch <-chan fullCommerceSellerResult) {
	t.Helper()
	select {
	case res := <-ch:
		require.NoError(t, res.err, "seller role=%s", res.role)
		assert.Equal(t, payment.StateCompleted, res.finalState, "seller role=%s", res.role)
	case <-time.After(5 * time.Second):
		t.Fatal("seller session did not complete")
	}
}

func TestRunSellerFullCommerce_TwoSellersOneHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(128)
	escrow := &serializedEscrow{inner: payment.NewMemoryEscrow()}
	contract := registry.NewMemoryRegistryContract()
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	adsbSpec, adsbResultCh := startAdsbFullCommerceSeller(t, ctx, bus, escrow, contract, registryAddr, true)
	ridSpec, ridResultCh := startRemoteIDFullCommerceSeller(t, ctx, bus, escrow, contract, registryAddr)

	buyerKey := newNeuronKey(t)
	buyerPriv := mustBlockchainKey(t, buyerKey)
	buyerHost, err := delivery.NewLibp2pHost(buyerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	frameCh := make(chan []byte, 64)
	results := make(chan bool, 2)
	var wg sync.WaitGroup
	for _, spec := range []SellerSpec{adsbSpec, ridSpec} {
		spec := spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- runSellerFullCommerce(ctx, spec, buyerHost, bus, escrow, "memory",
				contract, registryAddr, 296, buyerKey, buyerPriv, "1", 3, frameCh, newTestLogger(t))
		}()
	}
	wg.Wait()
	close(results)
	close(frameCh)

	var completed int
	for ok := range results {
		if ok {
			completed++
		}
	}
	assert.Equal(t, 2, completed)

	var sawAdsb, sawRemoteID bool
	for blob := range frameCh {
		var tf TaggedFrame
		require.NoError(t, json.Unmarshal(blob, &tf))
		switch tf.Source {
		case sourceAdsb:
			sawAdsb = true
		case sourceRemoteID:
			sawRemoteID = true
		}
	}
	assert.True(t, sawAdsb, "ADS-B full-commerce callback should emit TaggedFrame")
	assert.True(t, sawRemoteID, "Remote ID full-commerce callback should emit TaggedFrame")
	assertSellerCompleted(t, adsbResultCh)
	assertSellerCompleted(t, ridResultCh)
}

func TestRunSellerFullCommerce_PartialFailureDoesNotKillSibling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(128)
	escrow := &serializedEscrow{inner: payment.NewMemoryEscrow()}
	contract := registry.NewMemoryRegistryContract()
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)

	ridSpec, ridResultCh := startRemoteIDFullCommerceSeller(t, ctx, bus, escrow, contract, registryAddr)
	badSpec := SellerSpec{
		Role: roleAdsb,
		EVM:  "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	}

	buyerKey := newNeuronKey(t)
	buyerPriv := mustBlockchainKey(t, buyerKey)
	buyerHost, err := delivery.NewLibp2pHost(buyerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	frameCh := make(chan []byte, 64)
	results := make(chan bool, 2)
	var wg sync.WaitGroup
	for _, spec := range []SellerSpec{badSpec, ridSpec} {
		spec := spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- runSellerFullCommerce(ctx, spec, buyerHost, bus, escrow, "memory",
				contract, registryAddr, 296, buyerKey, buyerPriv, "1", 3, frameCh, newTestLogger(t))
		}()
	}
	wg.Wait()
	close(results)
	close(frameCh)

	var completed, failed int
	for ok := range results {
		if ok {
			completed++
		} else {
			failed++
		}
	}
	assert.Equal(t, 1, completed)
	assert.Equal(t, 1, failed)

	var sawRemoteID bool
	for blob := range frameCh {
		var tf TaggedFrame
		require.NoError(t, json.Unmarshal(blob, &tf))
		if tf.Source == sourceRemoteID {
			sawRemoteID = true
		}
	}
	assert.True(t, sawRemoteID, "good sibling should still emit frames")
	assertSellerCompleted(t, ridResultCh)
}

type recordingEscrow struct {
	mu        sync.Mutex
	active    int
	maxActive int
}

func (e *recordingEscrow) enter() {
	e.mu.Lock()
	e.active++
	if e.active > e.maxActive {
		e.maxActive = e.active
	}
	e.mu.Unlock()

	time.Sleep(5 * time.Millisecond)

	e.mu.Lock()
	e.active--
	e.mu.Unlock()
}

func (e *recordingEscrow) maxConcurrent() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.maxActive
}

func (e *recordingEscrow) CreateEscrow(context.Context, string, string, *string, string, uint64, [32]byte, uint64) (payment.EscrowRef, error) {
	e.enter()
	return payment.EscrowRef{Binding: "memory", Locator: "escrow"}, nil
}

func (e *recordingEscrow) Deposit(context.Context, payment.EscrowRef, string) (payment.DepositResult, error) {
	e.enter()
	return payment.DepositResult{TransactionRef: "tx", NewBalance: "1"}, nil
}

func (e *recordingEscrow) GetBalance(context.Context, payment.EscrowRef) (payment.Balance, error) {
	e.enter()
	return payment.Balance{Available: "1", Currency: "NTT"}, nil
}

func (e *recordingEscrow) RequestRelease(context.Context, payment.EscrowRef, string, string, [32]byte) (payment.ReleaseRequestRef, error) {
	e.enter()
	return payment.ReleaseRequestRef{Binding: "memory", Locator: "release"}, nil
}

func (e *recordingEscrow) ApproveRelease(context.Context, payment.EscrowRef, payment.ReleaseRequestRef) (payment.ReleaseResult, error) {
	e.enter()
	return payment.ReleaseResult{TransactionRef: "tx", Released: "1", Recipient: "seller"}, nil
}

func (e *recordingEscrow) ClaimRefund(context.Context, payment.EscrowRef) error {
	e.enter()
	return nil
}

func TestSerializedEscrow_SerializesConcurrentCalls(t *testing.T) {
	inner := &recordingEscrow{}
	escrow := &serializedEscrow{inner: inner}

	ctx := context.Background()
	ref := payment.EscrowRef{Binding: "memory", Locator: "escrow"}
	releaseRef := payment.ReleaseRequestRef{Binding: "memory", Locator: "release"}

	var wg sync.WaitGroup
	calls := []func(){
		func() { _, _ = escrow.CreateEscrow(ctx, "buyer", "seller", nil, "NTT", 1, [32]byte{}, 0) },
		func() { _, _ = escrow.Deposit(ctx, ref, "1") },
		func() { _, _ = escrow.GetBalance(ctx, ref) },
		func() { _, _ = escrow.RequestRelease(ctx, ref, "1", "seller", [32]byte{}) },
		func() { _, _ = escrow.ApproveRelease(ctx, ref, releaseRef) },
		func() { _ = escrow.ClaimRefund(ctx, ref) },
	}
	for _, call := range calls {
		call := call
		wg.Add(1)
		go func() {
			defer wg.Done()
			call()
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, inner.maxConcurrent(), "serializedEscrow must guard shared nonce-sensitive adapter calls")
}

// ============================================================================
// Phase 5 B4 — runSellerSessionWithReconnect supervisor
// ============================================================================
//
// Tests use a fake `sessionRunner` so we exercise the supervisor's loop +
// backoff + termination logic without spinning up real libp2p hosts. The
// `reconnectBackoffSequence` package var is temporarily overridden to
// millisecond values so tests complete quickly; helpers ensure
// per-test restoration. These tests are NOT parallelized (shared global).

// withFastBackoff swaps reconnectBackoffSequence to ms-scale values for the
// duration of the test body, then restores. Returns the override so the
// test body can inspect the exact wait values.
func withFastBackoff(t *testing.T) []time.Duration {
	t.Helper()
	return withReconnectBackoff(t, []time.Duration{2 * time.Millisecond, 4 * time.Millisecond, 8 * time.Millisecond, 16 * time.Millisecond, 32 * time.Millisecond})
}

func withReconnectBackoff(t *testing.T, override []time.Duration) []time.Duration {
	t.Helper()
	orig := reconnectBackoffSequence
	reconnectBackoffSequence = override
	t.Cleanup(func() { reconnectBackoffSequence = orig })
	return override
}

func TestRunSellerSessionWithReconnect_ReconnectsAfterEOF(t *testing.T) {
	_ = withFastBackoff(t)
	logger := log.New(io.Discard, "", 0)

	var calls int
	runner := func(ctx context.Context, _ host.Host, _ SellerSpec, _ peer.AddrInfo, _ string, frameCh chan<- []byte, _ uint64, _ *log.Logger) uint64 {
		calls++
		// First call emits 3 frames then "EOFs" (returns); second call
		// emits 2 more frames; third call returns 0 (transient failure);
		// after that the test cancels ctx.
		switch calls {
		case 1:
			for i := 0; i < 3; i++ {
				select {
				case <-ctx.Done():
					return uint64(i)
				case frameCh <- []byte(`{"i":1}`):
				}
			}
			return 3
		case 2:
			for i := 0; i < 2; i++ {
				select {
				case <-ctx.Done():
					return uint64(i)
				case frameCh <- []byte(`{"i":2}`):
				}
			}
			return 2
		default:
			return 0
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	frameCh := make(chan []byte, 16)
	done := make(chan struct{})

	go func() {
		total, attempts := runSellerSessionWithReconnect(ctx, runner,
			nil, SellerSpec{Role: roleAdsb}, peer.AddrInfo{}, "/test/1.0.0", frameCh, 0, logger)
		assert.Equal(t, uint64(5), total, "lifetime emitted = 3 + 2 + 0 + … = 5 (only first 3 calls emitted before cancel)")
		assert.GreaterOrEqual(t, attempts, 3, "supervisor reconnected at least twice (3 calls = 2 reconnects + 1 initial)")
		close(done)
	}()

	// Allow enough time for ~5 reconnect cycles (each ≤ 32ms), then cancel.
	time.Sleep(250 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor did not exit after ctx cancel within 2s")
	}
	assert.GreaterOrEqual(t, calls, 3, "runner called at least 3 times")
}

func TestRunSellerSessionWithReconnect_FrameLimitStopsReconnect(t *testing.T) {
	_ = withFastBackoff(t)
	logger := log.New(io.Discard, "", 0)

	const frameLimit uint64 = 10
	var calls int
	runner := func(_ context.Context, _ host.Host, _ SellerSpec, _ peer.AddrInfo, _ string, _ chan<- []byte, fl uint64, _ *log.Logger) uint64 {
		calls++
		assert.Equal(t, frameLimit, fl, "frame-limit must be plumbed through")
		return frameLimit // first call hits the limit
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	frameCh := make(chan []byte, 16)
	total, attempts := runSellerSessionWithReconnect(ctx, runner,
		nil, SellerSpec{Role: roleAdsb}, peer.AddrInfo{}, "/test/1.0.0", frameCh, frameLimit, logger)

	assert.Equal(t, 1, calls, "runner called exactly once — frame-limit stops the supervisor")
	assert.Equal(t, frameLimit, total)
	assert.Equal(t, 1, attempts)
}

func TestRunSellerSessionWithReconnect_CtxCancellationExitsCleanly(t *testing.T) {
	_ = withFastBackoff(t)
	logger := log.New(io.Discard, "", 0)

	runner := func(ctx context.Context, _ host.Host, _ SellerSpec, _ peer.AddrInfo, _ string, _ chan<- []byte, _ uint64, _ *log.Logger) uint64 {
		<-ctx.Done() // simulate session that blocks until cancel
		return 0
	}

	ctx, cancel := context.WithCancel(context.Background())
	frameCh := make(chan []byte, 16)
	done := make(chan struct{})
	go func() {
		runSellerSessionWithReconnect(ctx, runner,
			nil, SellerSpec{Role: roleAdsb}, peer.AddrInfo{}, "/test/1.0.0", frameCh, 0, logger)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("supervisor did not exit within 500ms of ctx cancel")
	}
}

func TestRunSellerSessionWithReconnect_BackoffEscalates(t *testing.T) {
	override := withFastBackoff(t)
	logger := log.New(io.Discard, "", 0)

	// Capture wall-clock between successive runner invocations.
	var stamps []time.Time
	runner := func(_ context.Context, _ host.Host, _ SellerSpec, _ peer.AddrInfo, _ string, _ chan<- []byte, _ uint64, _ *log.Logger) uint64 {
		stamps = append(stamps, time.Now())
		return 0 // always-failing session
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	frameCh := make(chan []byte, 1)
	runSellerSessionWithReconnect(ctx, runner,
		nil, SellerSpec{Role: roleAdsb}, peer.AddrInfo{}, "/test/1.0.0", frameCh, 0, logger)

	require.GreaterOrEqual(t, len(stamps), 4, "at least 4 attempts within 200ms (backoff ≤ 16ms across first 4 steps)")
	// Check that observed gaps escalate (each subsequent gap ≥ override[i-1]
	// with a small tolerance for scheduler jitter).
	for i := 1; i < 4; i++ {
		gap := stamps[i].Sub(stamps[i-1])
		wantMin := override[i-1] - 1*time.Millisecond // tolerance
		assert.GreaterOrEqual(t, gap, wantMin,
			"attempt %d→%d gap %s must be >= backoff step[%d]=%s", i-1, i, gap, i-1, override[i-1])
	}
}

func TestRunSellerSessionWithReconnect_ResetsBackoffOnSuccessfulSession(t *testing.T) {
	override := withReconnectBackoff(t, []time.Duration{
		10 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		200 * time.Millisecond,
	})
	logger := log.New(io.Discard, "", 0)

	// First call: fails (0 frames). Second call: succeeds (1 frame).
	// Third call onward: fails again. After the successful call, backoff
	// should reset to override[0] (2ms), NOT continue from override[2].
	var stamps []time.Time
	var calls int
	runner := func(_ context.Context, _ host.Host, _ SellerSpec, _ peer.AddrInfo, _ string, frameCh chan<- []byte, _ uint64, _ *log.Logger) uint64 {
		stamps = append(stamps, time.Now())
		calls++
		if calls == 2 {
			// Emit one frame to mark this session as successful.
			select {
			case frameCh <- []byte(`{"i":1}`):
				return 1
			default:
				return 0
			}
		}
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 450*time.Millisecond)
	defer cancel()
	frameCh := make(chan []byte, 4)
	runSellerSessionWithReconnect(ctx, runner,
		nil, SellerSpec{Role: roleAdsb}, peer.AddrInfo{}, "/test/1.0.0", frameCh, 0, logger)

	require.GreaterOrEqual(t, len(stamps), 4, "at least 4 attempts in 300ms")
	// Gap between attempts 2→3 should be the reset (~override[0]), NOT
	// continuing escalation from attempt 1→2's override[1]. The large
	// separation avoids depending on microsecond scheduler precision under
	// the race detector.
	gap2to3 := stamps[2].Sub(stamps[1])
	assert.Less(t, gap2to3, override[1]/2,
		"after successful session (call 2 emitted 1 frame), backoff MUST reset; got gap %s; max allowed %s",
		gap2to3, override[1]/2)
}
