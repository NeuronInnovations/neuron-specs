package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/adsb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// syncBuffer is a goroutine-safe bytes.Buffer used in tests where the
// run() goroutine writes to stdout/stderr while the test goroutine
// reads from it.
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

// TestCLI_BaseStationTCP_E2E spins up an in-process TCP server that streams
// SBS-1 BaseStation lines, then runs adsb-seller in fixture-direct mode
// with --feed-source=basestation-tcp pointed at the in-process port. A
// buyer host dials the seller, opens /jetvision/basestation/1.0.0, and reads
// canonical NormalizedTrack frames off the wire. Asserts the frames are
// well-formed and the source/entityType/entityID round-trip correctly.
//
// This is the headline read-only-JV-30003 test: the seller never writes
// upstream, just dials and parses. The in-process TCP server stands in for
// the JetVision port 30003.
func TestCLI_BaseStationTCP_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	// In-process SBS-1 server.
	tcpAddr, sbsStop := startInProcessSBSServer(t)
	t.Cleanup(sbsStop)

	// Spawn adsb-seller in fixture-direct mode pointing at the TCP server.
	sellerSig := make(chan os.Signal, 1)
	sellerStdout := &syncBuffer{}
	sellerStderr := &syncBuffer{}

	sellerExitCh := make(chan int, 1)
	go func() {
		sellerExitCh <- run([]string{
			"--feed-source=basestation-tcp",
			"--basestation-tcp-host=" + tcpAddr,
			"--listen=/ip4/127.0.0.1/udp/0/quic-v1",
			"--key-hex=" + fixedKeyHex,
		}, map[string]string{}, sellerStdout, sellerStderr, Deps{SignalCh: sellerSig})
	}()

	// Wait for the seller to print its libp2p multiaddr line.
	deadline := time.After(5 * time.Second)
	var sellerMultiaddr string
	for sellerMultiaddr == "" {
		select {
		case <-deadline:
			t.Fatalf("seller never printed its multiaddr; stderr=%s", sellerStderr.String())
		default:
			out := sellerStdout.String()
			for _, line := range bytesSplitLines(out) {
				if line != "" && containsP2P(line) {
					sellerMultiaddr = line
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Build a buyer host and dial the seller's /jetvision/basestation/1.0.0 stream.
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerEcdsa, err := buyerKey.ToBlockchainKey()
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerEcdsa, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = buyerHost.Close() })

	addrInfo, err := peer.AddrInfoFromString(sellerMultiaddr)
	require.NoError(t, err)
	require.NoError(t, buyerHost.Connect(ctx, *addrInfo))
	stream, err := buyerHost.NewStream(ctx, addrInfo.ID, protocol.ID(adsb.ProtocolBaseStation))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	reader := delivery.NewFrameReader(stream)
	var got []adsb.NormalizedTrack
	for len(got) < 3 {
		_ = stream.SetReadDeadline(time.Now().Add(3 * time.Second))
		data, err := reader.ReadFrame()
		if err != nil {
			break
		}
		var nt adsb.NormalizedTrack
		require.NoError(t, json.Unmarshal(data, &nt))
		got = append(got, nt)
	}
	require.GreaterOrEqual(t, len(got), 3, "expected >=3 NormalizedTrack frames from BaseStation TCP source")

	for i, nt := range got {
		assert.Equal(t, adsb.FrameType, nt.Type, "frame %d", i)
		assert.Equal(t, adsb.FrameVersion, nt.Version)
		assert.Equal(t, adsb.SourceAdsb, nt.Source)
		assert.Equal(t, adsb.EntityTypeAircraft, nt.EntityType)
		assert.NotEmpty(t, nt.EntityID, "frame %d carries an ICAO", i)
		// All fixture lines carry lat/lon → position must be present.
		require.NotNil(t, nt.Position, "frame %d", i)
	}

	// Shut the seller down cleanly.
	sellerSig <- os.Interrupt
	select {
	case rc := <-sellerExitCh:
		assert.Equal(t, 0, rc, "seller stderr=%s", sellerStderr.String())
	case <-time.After(5 * time.Second):
		t.Fatal("seller did not exit after SIGINT")
	}
}

// TestCLI_BaseStationTCP_Velocity_E2E is the regression guard for the
// "speed 0.0 km/h / heading 0.0°" bug. Vanilla SBS-1 carries ground speed and
// track ONLY in MSG type-4 records (type-3 carries position). The in-process
// server streams both for a single ICAO; the seller must parse the type-4
// record, merge it with the cached type-3 position, and emit a NormalizedTrack
// whose velocity block carries the (unit-converted) ground speed and heading.
func TestCLI_BaseStationTCP_Velocity_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	// One ICAO, alternating position (type 3) and velocity (type 4) records.
	tcpAddr, sbsStop := startInProcessSBSServerLines(t, []string{
		"MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,,,51.4700,-0.4543,,7000,0,0,0,0\n",
		"MSG,4,1,1,A1B2C3,1,2026/05/14,14:49:00.200,2026/05/14,14:49:00.200,,,485,290,,,64,,,,,\n",
	})
	t.Cleanup(sbsStop)

	sellerSig := make(chan os.Signal, 1)
	sellerStdout := &syncBuffer{}
	sellerStderr := &syncBuffer{}
	sellerExitCh := make(chan int, 1)
	go func() {
		sellerExitCh <- run([]string{
			"--feed-source=basestation-tcp",
			"--basestation-tcp-host=" + tcpAddr,
			"--listen=/ip4/127.0.0.1/udp/0/quic-v1",
			"--key-hex=" + fixedKeyHex,
		}, map[string]string{}, sellerStdout, sellerStderr, Deps{SignalCh: sellerSig})
	}()

	deadline := time.After(5 * time.Second)
	var sellerMultiaddr string
	for sellerMultiaddr == "" {
		select {
		case <-deadline:
			t.Fatalf("seller never printed its multiaddr; stderr=%s", sellerStderr.String())
		default:
			for _, line := range bytesSplitLines(sellerStdout.String()) {
				if line != "" && containsP2P(line) {
					sellerMultiaddr = line
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerEcdsa, err := buyerKey.ToBlockchainKey()
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerEcdsa, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = buyerHost.Close() })

	addrInfo, err := peer.AddrInfoFromString(sellerMultiaddr)
	require.NoError(t, err)
	require.NoError(t, buyerHost.Connect(ctx, *addrInfo))
	stream, err := buyerHost.NewStream(ctx, addrInfo.ID, protocol.ID(adsb.ProtocolBaseStation))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	// Read frames until one carries merged velocity for A1B2C3. The first emit
	// (position-only) precedes the merge; a later emit carries both.
	reader := delivery.NewFrameReader(stream)
	var withVel *adsb.NormalizedTrack
	for i := 0; i < 16 && withVel == nil; i++ {
		_ = stream.SetReadDeadline(time.Now().Add(3 * time.Second))
		data, rerr := reader.ReadFrame()
		if rerr != nil {
			break
		}
		var nt adsb.NormalizedTrack
		require.NoError(t, json.Unmarshal(data, &nt))
		if nt.EntityID == "A1B2C3" && nt.Velocity != nil && nt.Velocity.GroundSpeedSet {
			withVel = &nt
		}
	}

	require.NotNil(t, withVel, "expected a NormalizedTrack carrying merged velocity for A1B2C3")
	assert.NotNil(t, withVel.Position, "merged track must still carry position")
	assert.True(t, withVel.Velocity.GroundSpeedSet)
	assert.InDelta(t, 485.0*0.5144444444444444, withVel.Velocity.GroundSpeedMps, 1e-6,
		"485 knots converted to m/s")
	assert.True(t, withVel.Velocity.HeadingSet)
	assert.InDelta(t, 290.0, withVel.Velocity.HeadingDeg, 1e-9)

	sellerSig <- os.Interrupt
	select {
	case rc := <-sellerExitCh:
		assert.Equal(t, 0, rc, "seller stderr=%s", sellerStderr.String())
	case <-time.After(5 * time.Second):
		t.Fatal("seller did not exit after SIGINT")
	}
}

// startInProcessSBSServer streams a default set of three MSG type-3 (position)
// aircraft via startInProcessSBSServerLines.
func startInProcessSBSServer(t *testing.T) (string, func()) {
	t.Helper()
	// 3 distinct aircraft repeating every ~50ms so the seller has at least 3
	// NormalizedTracks within the test window.
	return startInProcessSBSServerLines(t, []string{
		"MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.100,2026/05/14,14:49:00.100,BAW178,37000,,,51.4700,-0.4543,,7000,0,0,0,0\n",
		"MSG,3,1,1,D5E6F7,1,2026/05/14,14:49:00.300,2026/05/14,14:49:00.300,KLM643,32000,,,51.5074,-0.1278,,7321,0,0,0,0\n",
		"MSG,3,1,1,7B8C9D,1,2026/05/14,14:49:00.500,2026/05/14,14:49:00.500,UAE12,41000,,,51.6000,-0.4000,,5512,0,0,0,0\n",
	})
}

// startInProcessSBSServerLines listens on a random localhost TCP port and
// streams the given SBS-1 lines (each MUST end with "\n") in a repeating loop
// ~20ms apart until stopped. Returns the address ("127.0.0.1:NNN") and a stop
// function. The server keeps the connection alive for the test's duration.
func startInProcessSBSServerLines(t *testing.T, lines []string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	var stopped atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		for !stopped.Load() {
			for _, line := range lines {
				if _, werr := io.WriteString(conn, line); werr != nil {
					return
				}
				time.Sleep(20 * time.Millisecond)
				if stopped.Load() {
					return
				}
			}
		}
	}()
	stopFn := func() {
		stopped.Store(true)
		_ = ln.Close()
		wg.Wait()
	}
	return addr, stopFn
}

// bytesSplitLines splits on '\n' returning non-empty lines.
func bytesSplitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// containsP2P returns true if the string contains "/p2p/" — a quick test
// for an emitted libp2p multiaddr line. Avoids strings package re-import.
func containsP2P(line string) bool {
	return indexOf(line, "/p2p/") >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Compile-time sanity check that the e2e harness can format an address.
var _ = fmt.Sprintf
