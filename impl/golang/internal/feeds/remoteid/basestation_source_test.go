package remoteid

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
)

// parseRIDForTest is a small test helper that exposes ParseRIDMSG
// through the test boundary so the cache-isolation tests can drive
// observe() with realistic RIDSBSRecord values.
func parseRIDForTest(t *testing.T, line string) (sbs.RIDSBSRecord, error) {
	t.Helper()
	rec, err := sbs.ParseRIDMSG(line)
	if rec == nil {
		return sbs.RIDSBSRecord{}, err
	}
	return *rec, err
}

// Canonical SBS lines from docs/neuron-rid-seller/README.md §181–187.
// Duplicated locally rather than imported from internal/feeds/sbs to
// keep the test data exactly aligned with the README.
const (
	bsLineDroneMSG1 = "MSG,1,,,FF0700,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,15810ABC,,,,,,,,0,0,0,0"
	bsLineDroneMSG3 = "MSG,3,,,FF0700,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,,,,090,50.1027,-5.6705,,,0,0,0,0"
	bsLineDroneMSG4 = "MSG,4,,,FF0700,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,,312,33,090,,,,,0,0,0,0"
	bsLineOpMSG1    = "MSG,1,,,FE5051,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,OP-O-001,,,,,,,,0,0,0,0"
	bsLineOpMSG2    = "MSG,2,,,FE5051,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,,49,0,0,50.1027,-5.6705,,,0,0,0,1"
)

// scriptedSBSServer is a tiny in-process TCP listener that emits a
// scripted sequence of SBS lines on each connection. Tests use it as
// a stand-in for the BlueMark bridge.
//
// Connection lifecycle: Close() closes the listener AND every
// accepted connection. The connection close makes the client-side
// bufio.Scanner unblock with EOF — without this, the basestation
// pump (which uses bufio.Scanner via internal/feeds/sbs) would block
// forever on a connection the test no longer cares about.
type scriptedSBSServer struct {
	t        *testing.T
	listener net.Listener

	mu        sync.Mutex
	connCount int
	conns     []net.Conn
	// Sequence sent on the first connection. On subsequent connections
	// (reconnect path), reuses the same sequence — simulates a
	// long-running bridge.
	sequence []string
	// After sending sequence, if closeAfterSeq is true, the server
	// closes the connection (triggers a reconnect on the client side).
	// If false, holds the connection open until Close() is called.
	closeAfterSeq bool
}

func newScriptedSBSServer(t *testing.T, sequence []string, closeAfterSeq bool) *scriptedSBSServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &scriptedSBSServer{
		t:             t,
		listener:      ln,
		sequence:      sequence,
		closeAfterSeq: closeAfterSeq,
	}
	go srv.serve()
	return srv
}

func (s *scriptedSBSServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		s.mu.Lock()
		s.connCount++
		s.conns = append(s.conns, conn)
		s.mu.Unlock()
		go s.handleConn(conn)
	}
}

func (s *scriptedSBSServer) handleConn(conn net.Conn) {
	defer conn.Close()
	for _, line := range s.sequence {
		if _, err := conn.Write([]byte(line + "\n")); err != nil {
			return
		}
		// Tiny inter-line delay so the client-side pump has time to
		// consume each record. Without this, the bursty write can
		// race the per-record select in the basestation cache loop.
		time.Sleep(2 * time.Millisecond)
	}
	if s.closeAfterSeq {
		return
	}
	// Hold the connection open until the connection is closed via
	// Close() (which calls conn.Close() under the lock).
	for {
		buf := make([]byte, 64)
		if _, err := conn.Read(buf); err != nil {
			return
		}
	}
}

func (s *scriptedSBSServer) Addr() string {
	return s.listener.Addr().String()
}

func (s *scriptedSBSServer) Close() {
	_ = s.listener.Close()
	s.mu.Lock()
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

func (s *scriptedSBSServer) Connections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connCount
}

// collectFrames pulls n frames from out or fails the test on timeout.
func collectFrames(t *testing.T, out <-chan DecodedFrame, n int, timeout time.Duration) []DecodedFrame {
	t.Helper()
	var frames []DecodedFrame
	deadline := time.After(timeout)
	for len(frames) < n {
		select {
		case f := <-out:
			frames = append(frames, f)
		case <-deadline:
			t.Fatalf("timeout waiting for %d frames; got %d", n, len(frames))
		}
	}
	return frames
}

// shutdownBasestation cancels the ctx and waits for the RunBasestation
// goroutine to exit. The order matters: srv.Close() must run BEFORE
// the errCh wait so the bridge-side connection closes and the
// bufio.Scanner inside the pump goroutine unblocks with EOF (mirrors
// the existing internal/feeds/sbs.TestRunBaseStationTCP_ExitsOnContextCancel
// pattern — see source_tcp_test.go).
func shutdownBasestation(t *testing.T, cancel context.CancelFunc, srv *scriptedSBSServer, errCh <-chan error) {
	t.Helper()
	cancel()
	srv.Close()
	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("RunBasestation did not exit within 3s of cancel + srv.Close — likely goroutine leak")
	}
}

// TestRunBasestation_DroneOnlyNoOperator covers the simple case: a
// drone MSG,1 + MSG,3 pair, no FE records ever observed. The frame
// must have Position but no Operator.
func TestRunBasestation_DroneOnlyNoOperator(t *testing.T) {
	t.Parallel()

	srv := newScriptedSBSServer(t, []string{bsLineDroneMSG1, bsLineDroneMSG3}, false)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunBasestation(ctx, BasestationConfig{
			HostPort: srv.Addr(),
		}, out)
	}()

	frames := collectFrames(t, out, 1, 2*time.Second)
	require.Len(t, frames, 1)
	f := frames[0]

	assert.Equal(t, "remote-id-frame", f.Type)
	assert.Equal(t, "1.0.0", f.Version)
	assert.Equal(t, "15810ABC", f.DroneID, "MSG,1 callsign should carry through as DroneID")
	assert.Equal(t, "serial", f.DroneIDType)
	require.NotNil(t, f.Position)
	assert.InDelta(t, 50.1027, f.Position.Lat, 1e-9)
	assert.InDelta(t, -5.6705, f.Position.Lon, 1e-9)
	assert.Equal(t, "3D", f.Position.Fix)
	assert.Nil(t, f.Operator, "no FE records observed; operator must be nil")

	shutdownBasestation(t, cancel, srv, errCh)
}

// TestRunBasestation_DroneAndOperatorPaired covers the canonical case
// — operator identity → drone identity → drone position. The emitted
// frame must carry operator enrichment.
func TestRunBasestation_DroneAndOperatorPaired(t *testing.T) {
	t.Parallel()

	srv := newScriptedSBSServer(t, []string{
		bsLineOpMSG1,
		bsLineOpMSG2,
		bsLineDroneMSG1,
		bsLineDroneMSG3,
	}, false)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunBasestation(ctx, BasestationConfig{
			HostPort: srv.Addr(),
		}, out)
	}()

	frames := collectFrames(t, out, 1, 2*time.Second)
	require.Len(t, frames, 1)
	f := frames[0]

	require.NotNil(t, f.Operator, "FE records were observed; operator must be enriched")
	assert.Equal(t, "OP-O-001", f.Operator.ID)
	assert.Equal(t, "caa", f.Operator.IDType)
	require.NotNil(t, f.Operator.Position)
	assert.InDelta(t, 50.1027, f.Operator.Position.Lat, 1e-9)
	assert.InDelta(t, -5.6705, f.Operator.Position.Lon, 1e-9)

	shutdownBasestation(t, cancel, srv, errCh)
}

// TestRunBasestation_VelocityCarryover covers MSG,1 → MSG,4 → MSG,3
// ordering: velocity from the prior MSG,4 must surface on the frame
// emitted by MSG,3.
func TestRunBasestation_VelocityCarryover(t *testing.T) {
	t.Parallel()

	srv := newScriptedSBSServer(t, []string{
		bsLineDroneMSG1,
		bsLineDroneMSG4,
		bsLineDroneMSG3,
	}, false)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunBasestation(ctx, BasestationConfig{HostPort: srv.Addr()}, out)
	}()

	frames := collectFrames(t, out, 1, 2*time.Second)
	require.Len(t, frames, 1)
	f := frames[0]

	require.NotNil(t, f.Velocity, "MSG,4 velocity must carry over to the MSG,3 frame")
	// MSG,3 itself carries trk=090 inline; the test asserts that
	// inline value takes precedence (same value as MSG,4's 090 here,
	// so verifies equivalence).
	assert.InDelta(t, 90.0, f.Velocity.Track, 1e-9)
	// MSG,4 reported SpdKnots=33 → 33 × 0.514444 = 16.97 m/s.
	assert.InDelta(t, 33.0*knotsToMps, f.Velocity.SpeedHorizontal, 1e-6)

	shutdownBasestation(t, cancel, srv, errCh)
}

// TestRunBasestation_UnitConversion proves feet→meters, knots→m/s,
// fpm→m/s on a single MSG,3 with all velocity components set.
func TestRunBasestation_UnitConversion(t *testing.T) {
	t.Parallel()

	// Hand-crafted MSG,3 with altitude=328.084 feet (=100m), spd=10
	// knots (=5.144 m/s), trk=90°, vrt=196.85 fpm (=1 m/s).
	customMSG3 := "MSG,3,,,FF0700,,2026/05/15,12:00:00.123,2026/05/15,12:00:00.124,,328.084,10,90,50.1027,-5.6705,196.85,,0,0,0,0"
	srv := newScriptedSBSServer(t, []string{customMSG3}, false)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunBasestation(ctx, BasestationConfig{HostPort: srv.Addr()}, out)
	}()

	frames := collectFrames(t, out, 1, 2*time.Second)
	require.Len(t, frames, 1)
	f := frames[0]

	require.NotNil(t, f.Position)
	assert.InDelta(t, 100.0, f.Position.Alt, 1e-3, "328.084 ft should convert to ~100 m")
	require.NotNil(t, f.Velocity)
	assert.InDelta(t, 5.14444, f.Velocity.SpeedHorizontal, 1e-3, "10 knots → ~5.144 m/s")
	assert.InDelta(t, 90.0, f.Velocity.Track, 1e-9)
	assert.InDelta(t, 1.0, f.Velocity.SpeedVertical, 1e-3, "196.85 fpm → ~1 m/s")

	shutdownBasestation(t, cancel, srv, errCh)
}

// TestRunBasestation_DefaultSourceLabel covers the empty-SourceLabel
// case: frames are stamped with DefaultBasestationSourceLabel.
func TestRunBasestation_DefaultSourceLabel(t *testing.T) {
	t.Parallel()

	srv := newScriptedSBSServer(t, []string{bsLineDroneMSG1, bsLineDroneMSG3}, false)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunBasestation(ctx, BasestationConfig{
			HostPort: srv.Addr(),
			// SourceLabel empty
		}, out)
	}()

	frames := collectFrames(t, out, 1, 2*time.Second)
	require.Len(t, frames, 1)
	assert.Equal(t, DefaultBasestationSourceLabel, frames[0].Source,
		"empty SourceLabel must default to DefaultBasestationSourceLabel")

	shutdownBasestation(t, cancel, srv, errCh)
}

// TestRunBasestation_OverrideSourceLabel covers the operator-supplied
// label path.
func TestRunBasestation_OverrideSourceLabel(t *testing.T) {
	t.Parallel()

	srv := newScriptedSBSServer(t, []string{bsLineDroneMSG1, bsLineDroneMSG3}, false)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunBasestation(ctx, BasestationConfig{
			HostPort:    srv.Addr(),
			SourceLabel: "custom-label-x",
		}, out)
	}()

	frames := collectFrames(t, out, 1, 2*time.Second)
	require.Len(t, frames, 1)
	assert.Equal(t, "custom-label-x", frames[0].Source)

	shutdownBasestation(t, cancel, srv, errCh)
}

// TestRunBasestation_ContextCancel verifies clean shutdown:
// ctx.Cancel() → returns ctx.Err() and the goroutine exits within a
// reasonable window (timeout-based leak guard).
func TestRunBasestation_ContextCancel(t *testing.T) {
	t.Parallel()

	srv := newScriptedSBSServer(t, []string{bsLineDroneMSG1, bsLineDroneMSG3}, false)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	out := make(chan DecodedFrame, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunBasestation(ctx, BasestationConfig{HostPort: srv.Addr()}, out)
	}()

	// Let the pump connect and emit at least one frame, then cancel.
	collectFrames(t, out, 1, 2*time.Second)
	shutdownBasestation(t, cancel, srv, errCh)
}

// TestRunBasestation_PairingTTLExpiry covers the case where the
// operator FE record was observed long ago; subsequent drone MSG,3
// frames must NOT carry operator enrichment.
//
// Drives the cache directly (rather than through the TCP path) so the
// test can deterministically advance the clock past the TTL between
// operator observation and drone-position emission. The TCP-path
// tests cover the integration; this test isolates the cache TTL
// logic.
func TestRunBasestation_PairingTTLExpiry(t *testing.T) {
	t.Parallel()

	// Mock clock controllable by the test.
	var clockMu sync.Mutex
	current := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	advance := func(d time.Duration) {
		clockMu.Lock()
		defer clockMu.Unlock()
		current = current.Add(d)
	}
	clockNow := func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return current
	}

	c := &basestationCache{
		ttl: 30 * time.Second,
		now: clockNow,
	}

	// Observe operator identity at t0.
	opRec, err := parseRIDForTest(t, bsLineOpMSG1)
	require.NoError(t, err)
	_, emit := c.observe(opRec, DefaultBasestationSourceLabel)
	require.False(t, emit, "operator MSG,1 must not emit a frame")

	// Advance the clock 1 hour — well past the TTL.
	advance(1 * time.Hour)

	// Now observe drone identity + position.
	dRec1, err := parseRIDForTest(t, bsLineDroneMSG1)
	require.NoError(t, err)
	_, emit = c.observe(dRec1, DefaultBasestationSourceLabel)
	require.False(t, emit, "drone MSG,1 must not emit a frame")

	dRec3, err := parseRIDForTest(t, bsLineDroneMSG3)
	require.NoError(t, err)
	frame, emit := c.observe(dRec3, DefaultBasestationSourceLabel)
	require.True(t, emit, "drone MSG,3 must emit a frame")
	assert.Nil(t, frame.Operator,
		"operator observed before TTL window should NOT enrich the frame")
}

// TestRunBasestation_PairingWithinTTL is the positive counterpart to
// TestRunBasestation_PairingTTLExpiry — operator observed within the
// TTL window enriches the frame.
func TestRunBasestation_PairingWithinTTL(t *testing.T) {
	t.Parallel()

	clock := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	c := &basestationCache{
		ttl: 30 * time.Second,
		now: func() time.Time { return clock },
	}

	opRec, err := parseRIDForTest(t, bsLineOpMSG1)
	require.NoError(t, err)
	c.observe(opRec, DefaultBasestationSourceLabel)

	// Within TTL — same clock, no advance.
	dRec1, _ := parseRIDForTest(t, bsLineDroneMSG1)
	c.observe(dRec1, DefaultBasestationSourceLabel)
	dRec3, _ := parseRIDForTest(t, bsLineDroneMSG3)
	frame, emit := c.observe(dRec3, DefaultBasestationSourceLabel)
	require.True(t, emit)
	require.NotNil(t, frame.Operator)
	assert.Equal(t, "OP-O-001", frame.Operator.ID)
}

// TestRunBasestation_ReconnectOnEOF covers the server-closed case: the
// pump must reconnect and continue emitting frames.
func TestRunBasestation_ReconnectOnEOF(t *testing.T) {
	t.Parallel()

	// closeAfterSeq=true: server sends the sequence then closes the
	// connection, forcing the client to reconnect.
	srv := newScriptedSBSServer(t, []string{
		bsLineDroneMSG1,
		bsLineDroneMSG3,
	}, true)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	out := make(chan DecodedFrame, 16)
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunBasestation(ctx, BasestationConfig{HostPort: srv.Addr()}, out)
	}()

	// Collect two frames — the second one can only arrive after a
	// successful reconnect (the server re-runs the same sequence on
	// each accept).
	frames := collectFrames(t, out, 2, 5*time.Second)
	require.GreaterOrEqual(t, len(frames), 2)
	// And the server should have accepted >= 2 connections.
	assert.GreaterOrEqual(t, srv.Connections(), 2,
		"reconnect path must dial the server >= 2 times")

	shutdownBasestation(t, cancel, srv, errCh)
}

// TestRunBasestation_RequiresHostPort covers the validation path.
func TestRunBasestation_RequiresHostPort(t *testing.T) {
	t.Parallel()
	out := make(chan DecodedFrame, 1)
	err := RunBasestation(context.Background(), BasestationConfig{}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host:port")
}
