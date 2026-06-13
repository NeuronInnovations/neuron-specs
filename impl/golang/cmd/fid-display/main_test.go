package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sample tagged JSONL produced by the Phase-2 buyer + seller smoke test.
// One Remote ID drone, full canonical-JSON envelope. Used as the
// in-test fixture so we don't depend on a network seller for unit tests.
const sampleTaggedJSONL = `{"source":"remote-id","type":"drone","sellerPeerID":"16Uiu2HAmHQeghrZWfosnoEqXH3FL2LYEKuFEJBDjghUUXGY4Xgog","receivedAt":"2026-05-11T08:03:26.451706Z","frame":{"type":"remote-id-frame","version":"1.0.0","observedAt":"1778486606450818000","source":"synth","droneId":"SYNTH-0000000000","droneIdType":"serial","position":{"lat":51.47765725068178,"lon":-0.4585264219723189,"alt":100,"fix":"3D"},"velocity":{"speedHorizontal":69.81317007977317,"speedVertical":0,"track":95.02128},"regulatorVariant":"asd-faa"}}`

func TestApply_AcceptsCanonicalTaggedFrame(t *testing.T) {
	t.Parallel()
	state := NewState()
	var tf TaggedFrame
	require.NoError(t, json.Unmarshal([]byte(sampleTaggedJSONL), &tf))

	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.Equal(t, "drone", event.Kind)
	require.NotNil(t, event.Drone)
	assert.Equal(t, "SYNTH-0000000000", event.Drone.DroneID)
	assert.Equal(t, "remote-id", event.Drone.Source)
	assert.InDelta(t, 51.47765725068178, event.Drone.Lat, 1e-9)
	assert.InDelta(t, -0.4585264219723189, event.Drone.Lon, 1e-9)
	assert.Equal(t, "asd-faa", event.Drone.RegulatorVariant)
	assert.NotEmpty(t, event.Drone.SellerPeerID)
}

func TestApply_RejectsFrameWithoutPosition(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "remote-id",
		Type:   "drone",
		Frame:  []byte(`{"droneId":"X","droneIdType":"serial"}`),
	}
	_, err := state.Apply(tf)
	require.Error(t, err)
}

func TestApply_RejectsFrameWithoutDroneID(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "remote-id",
		Type:   "drone",
		Frame:  []byte(`{"position":{"lat":51,"lon":-0.5,"alt":100,"fix":"3D"}}`),
	}
	_, err := state.Apply(tf)
	require.Error(t, err)
}

func TestApply_UpdateReplacesPriorSnapshot(t *testing.T) {
	t.Parallel()
	state := NewState()

	tf1 := TaggedFrame{
		Source: "remote-id", Type: "drone", SellerPeerID: "PID",
		ReceivedAt: time.Now().UTC(),
		Frame:      []byte(`{"droneId":"D1","droneIdType":"serial","position":{"lat":51.0,"lon":-0.5,"alt":50,"fix":"3D"}}`),
	}
	_, err := state.Apply(tf1)
	require.NoError(t, err)

	tf2 := TaggedFrame{
		Source: "remote-id", Type: "drone", SellerPeerID: "PID",
		ReceivedAt: time.Now().UTC(),
		Frame:      []byte(`{"droneId":"D1","droneIdType":"serial","position":{"lat":51.5,"lon":-0.4,"alt":100,"fix":"3D"}}`),
	}
	event, err := state.Apply(tf2)
	require.NoError(t, err)

	all := state.Snapshot()
	require.Len(t, all, 1, "same droneId should not duplicate")
	require.NotNil(t, event.Drone)
	assert.InDelta(t, 51.5, event.Drone.Lat, 1e-9, "later update wins")
}

func TestSnapshot_ReturnsCopy(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "remote-id", Type: "drone",
		Frame: []byte(`{"droneId":"A","droneIdType":"serial","position":{"lat":51,"lon":-0.5,"alt":100,"fix":"3D"}}`),
	}
	_, err := state.Apply(tf)
	require.NoError(t, err)

	snap := state.Snapshot()
	require.Len(t, snap, 1)
	// Mutating the returned slice element MUST NOT affect state.
	snap[0].DroneID = "MUTATED"
	again := state.Snapshot()
	assert.Equal(t, "A", again[0].DroneID)
}

func TestEvict_RemovesStaleDrones(t *testing.T) {
	t.Parallel()
	state := NewState()
	old := time.Now().Add(-10 * time.Minute)
	fresh := time.Now()

	for _, p := range []struct {
		id   string
		seen time.Time
	}{
		{"OLD-1", old},
		{"OLD-2", old},
		{"FRESH", fresh},
	} {
		tf := TaggedFrame{
			Source: "remote-id", Type: "drone", ReceivedAt: p.seen,
			Frame: fmt.Appendf(nil, `{"droneId":"%s","droneIdType":"serial","position":{"lat":51,"lon":-0.5,"alt":100,"fix":"3D"}}`, p.id),
		}
		_, err := state.Apply(tf)
		require.NoError(t, err)
	}

	n := state.Evict(time.Now(), 5*time.Minute)
	assert.Equal(t, 2, n)
	require.Len(t, state.Snapshot(), 1)
	assert.Equal(t, "FRESH", state.Snapshot()[0].DroneID)
}

func TestStateHandler_ReturnsCurrentSnapshot(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "remote-id", Type: "drone", ReceivedAt: time.Now().UTC(),
		Frame: []byte(`{"droneId":"A","droneIdType":"serial","position":{"lat":51,"lon":-0.5,"alt":100,"fix":"3D"}}`),
	}
	_, err := state.Apply(tf)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	stateHandler(state)(rr, httptest.NewRequest(http.MethodGet, "/state.json", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Drones []DroneSnapshot `json:"drones"`
		Count  int             `json:"count"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, 1, body.Count)
	require.Len(t, body.Drones, 1)
	assert.Equal(t, "A", body.Drones[0].DroneID)
}

func TestConfigHandler_ReturnsMapCenter(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	configHandler(MapConfig{Lat: 1.5, Lon: 2.5, Zoom: 11})(rr, httptest.NewRequest(http.MethodGet, "/config.json", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var cfg MapConfig
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&cfg))
	assert.InDelta(t, 1.5, cfg.Lat, 1e-9)
	assert.InDelta(t, 2.5, cfg.Lon, 1e-9)
	assert.Equal(t, 11, cfg.Zoom)
}

func TestIndexHandler_ServesEmbeddedHTML(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	indexHandler()(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rr.Body.String(), "Neuron FID")
	assert.Contains(t, rr.Body.String(), "Leaflet")
}

func TestIndexHandler_404OnUnknownPath(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	indexHandler()(rr, httptest.NewRequest(http.MethodGet, "/random", nil))
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestTCPListenerIngest verifies the display's TCP listener accepts a
// buyer-style connection, parses newline-delimited TaggedFrame records,
// and surfaces them in /state.json.
func TestTCPListenerIngest(t *testing.T) {
	t.Parallel()

	state := NewState()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		acceptLoop(ctx, listener, state, discardLogger())
	}()
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		<-done
	})

	// Connect as the buyer would.
	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	// Write three sample frames.
	for _, droneID := range []string{"A", "B", "A"} {
		rec := fmt.Sprintf(
			`{"source":"remote-id","type":"drone","sellerPeerID":"PID","receivedAt":"2026-05-11T08:00:00Z","frame":{"droneId":"%s","droneIdType":"serial","position":{"lat":51,"lon":-0.5,"alt":100,"fix":"3D"}}}`,
			droneID,
		)
		_, err := conn.Write([]byte(rec + "\n"))
		require.NoError(t, err)
	}

	// Wait briefly for ingestion.
	require.Eventually(t, func() bool {
		return len(state.Snapshot()) == 2
	}, 2*time.Second, 20*time.Millisecond, "expected 2 unique drones (A, B) after 3 frames")

	rr := httptest.NewRecorder()
	stateHandler(state)(rr, httptest.NewRequest(http.MethodGet, "/state.json", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `"droneId":"A"`)
	assert.Contains(t, body, `"droneId":"B"`)
}

func TestSSE_DeliversUpdatesAfterApply(t *testing.T) {
	t.Parallel()

	state := NewState()
	server := httptest.NewServer(http.HandlerFunc(eventsHandler(state)))
	defer server.Close()

	// Open SSE.
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")

	// Push an update.
	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = state.Apply(TaggedFrame{
			Source: "remote-id", Type: "drone", ReceivedAt: time.Now().UTC(),
			Frame: []byte(`{"droneId":"SSE-DRONE","droneIdType":"serial","position":{"lat":51,"lon":-0.5,"alt":100,"fix":"3D"}}`),
		})
	}()

	// Read SSE stream until we see the update.
	buf := make([]byte, 4096)
	deadline := time.Now().Add(2 * time.Second)
	var combined strings.Builder
	for time.Now().Before(deadline) && !strings.Contains(combined.String(), "SSE-DRONE") {
		n, err := resp.Body.Read(buf)
		if err != nil {
			break
		}
		combined.Write(buf[:n])
	}
	assert.Contains(t, combined.String(), "SSE-DRONE", "expected SSE stream to carry the update")
	assert.Contains(t, combined.String(), "event: update", "expected event: update header")
}

// sampleAdsbTaggedJSONL mirrors the cmd/edge-buyer Phase 5 dual-stream
// envelope (NEURON_EDGE_TAGGED_OUTPUT) and the FID display contract
// at docs/fid-display-contract.md line 81-86.
const sampleAdsbTaggedJSONL = `{"source":"adsb","type":"aircraft","sellerPeerID":"12D3KooWAbc","receivedAt":"2026-05-11T08:03:26.451706Z","frame":{"sellerEVM":"0x1234567890abcdef1234567890abcdef12345678","sellerName":"jv-london","sellerPeerID":"12D3KooWAbc","meta":{"DF":17,"ICAO":"ABC123"}}}`

// TestApply_AcceptsAdsBTaggedFrame verifies State.Apply ingests the
// Phase 5 dual-stream ADS-B envelope and surfaces it as an aircraft
// snapshot — the buyer-side composition contract for fused-buyer
// scenario US4 (017 SC-R06). Closes the coverage gap noted in
// week-1 gap analysis §5.3 (12/12 prior tests exercised
// source="remote-id" only).
func TestApply_AcceptsAdsBTaggedFrame(t *testing.T) {
	t.Parallel()
	state := NewState()
	var tf TaggedFrame
	require.NoError(t, json.Unmarshal([]byte(sampleAdsbTaggedJSONL), &tf))

	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.Equal(t, "aircraft", event.Kind, "Apply must dispatch source=adsb to the aircraft path")
	require.NotNil(t, event.Aircraft)

	got := event.Aircraft
	assert.Equal(t, "abc123", got.ICAO, "ICAO must be lower-cased per applyAircraft normalization")
	assert.Equal(t, "adsb", got.Source)
	assert.Equal(t, "0x1234567890abcdef1234567890abcdef12345678", got.SellerEVM)
	assert.Equal(t, "jv-london", got.SellerName)
	assert.Equal(t, "12D3KooWAbc", got.SellerPeerID, "SellerPeerID must come from the outer envelope, not the AggregatedFrame inner")
	assert.Equal(t, 17, got.DF, "DF=17 ADS-B Extended Squitter must be preserved")
	assert.True(t, got.PositionFake, "Phase 5 aircraft positions are placeholder (per main.go:118 + week-1-gap-report.md §5.1)")
	assert.Equal(t, uint64(1), got.FrameCount, "first frame for this ICAO has FrameCount=1")

	// Surfaces correctly in /state.json — drones and aircraft are
	// stored in independent keyspaces so the dual-source independence
	// rule (017 US4 acceptance scenario 2: kill one, the other lives)
	// is naturally satisfied.
	assert.Empty(t, state.Snapshot(), "ADS-B ingest must not populate the drone keyspace")
	require.Len(t, state.AircraftSnapshot(), 1)
	assert.Equal(t, "abc123", state.AircraftSnapshot()[0].ICAO)

	// FrameCount increments on a second frame for the same ICAO.
	tf2 := tf
	tf2.ReceivedAt = tf.ReceivedAt.Add(1 * time.Second)
	event2, err := state.Apply(tf2)
	require.NoError(t, err)
	require.NotNil(t, event2.Aircraft)
	assert.Equal(t, uint64(2), event2.Aircraft.FrameCount, "repeated ICAO must increment FrameCount, not append a new aircraft")
	require.Len(t, state.AircraftSnapshot(), 1, "same ICAO must not duplicate the aircraft entry")
}

// TestApply_AdsBPositionIsPlaceholder pins the Phase 5 placeholder
// behavior so a future regression to "real" position is loud rather
// than silent. Per cmd/fid-display/main.go:101-107 (Phase 5 ships
// without a CPR position decoder; Phase 6A replaces this).
//
// Invariants checked:
//   - PositionFake is true on every aircraft.
//   - The placeholder is deterministic: same ICAO + same map center
//     yields the same (lat, lon) every time.
//   - Different ICAOs yield different placeholder positions.
//   - All placeholders fall within ~0.1° of the map center (the
//     ±5km bounding box defined in placeholderPositionFromICAO).
//   - The map-center anchor is honored: NewStateWithCenter(lat, lon)
//     anchors the placeholder cluster around (lat, lon).
func TestApply_AdsBPositionIsPlaceholder(t *testing.T) {
	t.Parallel()

	mkFrame := func(icao string) TaggedFrame {
		body := fmt.Sprintf(
			`{"sellerEVM":"0xaaa","sellerName":"jv","sellerPeerID":"PID","meta":{"DF":17,"ICAO":"%s"}}`,
			icao,
		)
		return TaggedFrame{
			Source:       "adsb",
			Type:         "aircraft",
			SellerPeerID: "PID",
			ReceivedAt:   time.Now().UTC(),
			Frame:        []byte(body),
		}
	}

	// Determinism: same ICAO into two independent State instances
	// (same map center) yields the same placeholder.
	stateA := NewStateWithCenter(51.4775, -0.4614)
	stateB := NewStateWithCenter(51.4775, -0.4614)
	frame := mkFrame("ABC123")

	evA, err := stateA.Apply(frame)
	require.NoError(t, err)
	require.NotNil(t, evA.Aircraft)
	evB, err := stateB.Apply(frame)
	require.NoError(t, err)
	require.NotNil(t, evB.Aircraft)

	assert.True(t, evA.Aircraft.PositionFake, "PositionFake=true is the FAKE-position UI cue")
	assert.True(t, evB.Aircraft.PositionFake)
	assert.Equal(t, evA.Aircraft.Lat, evB.Aircraft.Lat, "placeholder Lat must be deterministic for a given ICAO + center")
	assert.Equal(t, evA.Aircraft.Lon, evB.Aircraft.Lon, "placeholder Lon must be deterministic for a given ICAO + center")

	// Bounding box: placeholderPositionFromICAO promises within ~5km
	// of map center. 5km lat ≈ 0.0449°; 5km lon at 51° ≈ 0.0714°.
	// Use a 0.1° loose bound — anything outside that signals a
	// regression in the placeholder math, not jitter.
	const latBound = 0.1
	const lonBound = 0.1
	assert.InDelta(t, 51.4775, evA.Aircraft.Lat, latBound, "placeholder must fall within ±0.1° of map center latitude")
	assert.InDelta(t, -0.4614, evA.Aircraft.Lon, lonBound, "placeholder must fall within ±0.1° of map center longitude")

	// Different ICAOs yield different placeholders. SHA256 collisions
	// on a 4-byte prefix are astronomically unlikely for any two real
	// ICAO addresses; we sample 4 ICAOs and require all positions are
	// pairwise distinct.
	icaos := []string{"AAAA01", "BBBB02", "CCCC03", "DDDD04"}
	positions := make(map[string][2]float64, len(icaos))
	for _, icao := range icaos {
		state := NewStateWithCenter(51.4775, -0.4614)
		ev, err := state.Apply(mkFrame(icao))
		require.NoError(t, err)
		require.NotNil(t, ev.Aircraft)
		positions[icao] = [2]float64{ev.Aircraft.Lat, ev.Aircraft.Lon}
	}
	for i := range icaos {
		for j := i + 1; j < len(icaos); j++ {
			a, b := positions[icaos[i]], positions[icaos[j]]
			if a[0] == b[0] && a[1] == b[1] {
				t.Fatalf("placeholder positions collided for distinct ICAOs %q vs %q at %v",
					icaos[i], icaos[j], a)
			}
		}
	}

	// Map-center anchor: different center → placeholder shifts with it.
	// Same ICAO around two different centers must produce two distinct
	// (lat, lon) results, each clustered around its own center.
	stateC := NewStateWithCenter(40.0, -74.0) // New York-ish
	stateD := NewStateWithCenter(51.4775, -0.4614)
	evC, err := stateC.Apply(mkFrame("ABC123"))
	require.NoError(t, err)
	evD, err := stateD.Apply(mkFrame("ABC123"))
	require.NoError(t, err)
	assert.NotEqual(t, evC.Aircraft.Lat, evD.Aircraft.Lat, "placeholder must shift with map-center latitude")
	assert.NotEqual(t, evC.Aircraft.Lon, evD.Aircraft.Lon, "placeholder must shift with map-center longitude")
	assert.InDelta(t, 40.0, evC.Aircraft.Lat, latBound, "NYC-centered placeholder must cluster around 40.0° N")
	assert.InDelta(t, -74.0, evC.Aircraft.Lon, lonBound, "NYC-centered placeholder must cluster around -74.0° W")
}

// ---- MVP Phase 1: FrameSource pass-through (plan §"Step 8") ----

// TestApply_DronePropagatesInnerFrameSource asserts the inner
// RemoteIdFrame.Source field surfaces as DroneSnapshot.FrameSource.
// This is the field the browser substring-matches to render the SYN
// badge.
func TestApply_DronePropagatesInnerFrameSource(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "remote-id",
		Type:   "drone",
		Frame: []byte(`{
			"droneId": "FF0700-DEMO",
			"droneIdType": "serial",
			"source": "basestation-tcp-synthetic",
			"position": {"lat": 50.1027, "lon": -5.6705, "alt": 95.0, "fix": "3D"}
		}`),
	}
	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.NotNil(t, event.Drone)
	assert.Equal(t, "basestation-tcp-synthetic", event.Drone.FrameSource,
		"inner frame.source must carry through to DroneSnapshot.FrameSource")
}

// TestApply_DroneDropsEmptyInnerFrameSource asserts the omitempty
// behaviour: an empty inner source must not pollute the snapshot or
// the wire JSON.
func TestApply_DroneDropsEmptyInnerFrameSource(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "remote-id",
		Type:   "drone",
		// No "source" field on the inner frame.
		Frame: []byte(`{
			"droneId": "FF0700-DEMO",
			"droneIdType": "serial",
			"position": {"lat": 50.1027, "lon": -5.6705, "alt": 95.0, "fix": "3D"}
		}`),
	}
	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.NotNil(t, event.Drone)
	assert.Empty(t, event.Drone.FrameSource,
		"absent inner frame.source must leave DroneSnapshot.FrameSource empty (omitempty)")
}

// ─── Phase 3 (2026-05-18): operator/pilot keyspace tests ─────────────
//
// These tests close the gap exposed by the 2026-05-18 real-device
// evidence pack: the wire-level SBS sample contained both FF0700 drone
// records AND FE5051 operator records, the seller correctly emitted
// paired RemoteIdFrames with operator.id + operator.position, but
// fid-display's state.json only showed the drone — operatorId was null
// and no separate pilot marker rendered.
//
// Fix: frameInner gains an Operator field; State gains an operators
// keyspace; applyDrone materialises an OperatorSnapshot when the
// inner frame has an operator with a ground position; stateHandler
// includes operators[]; Evict iterates operators independently.

// remoteIDFrameWithOperator is a helper that builds a canonical
// RemoteIdFrame JSON shape including an operator object with ground
// position. Matches Operator.MarshalJSON in
// internal/dapp/remoteid/frame.go.
func remoteIDFrameWithOperator(droneID, operatorID string,
	droneLat, droneLon, opLat, opLon float64,
	frameSource string,
) []byte {
	type pos struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
		Alt float64 `json:"alt"`
		Fix string  `json:"fix"`
	}
	type op struct {
		IDType   string `json:"idType,omitempty"`
		ID       string `json:"id"`
		Position *pos   `json:"position,omitempty"`
	}
	body := struct {
		DroneID     string `json:"droneId"`
		DroneIDType string `json:"droneIdType"`
		Source      string `json:"source,omitempty"`
		Position    *pos   `json:"position"`
		Operator    *op    `json:"operator,omitempty"`
	}{
		DroneID:     droneID,
		DroneIDType: "serial",
		Source:      frameSource,
		Position:    &pos{Lat: droneLat, Lon: droneLon, Fix: "3D"},
	}
	if operatorID != "" {
		body.Operator = &op{
			IDType:   "caa",
			ID:       operatorID,
			Position: &pos{Lat: opLat, Lon: opLon, Fix: "3D"},
		}
	}
	b, _ := json.Marshal(body)
	return b
}

// TestApply_DroneWithOperator_PopulatesBothKeyspaces — the headline
// Phase 3 fix: when the inner frame carries operator+position, both
// state.drones[d] AND state.operators[op] are populated and the
// SSEEvent carries both fields.
func TestApply_DroneWithOperator_PopulatesBothKeyspaces(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source:       "remote-id",
		Type:         "drone",
		SellerPeerID: "16Uiu2HAm-test-seller",
		ReceivedAt:   time.Now().UTC(),
		Frame: remoteIDFrameWithOperator(
			"15810ABC", "OP-O-001",
			50.1071, -5.6685, // drone orbiting
			50.1027, -5.6705, // operator stationary at Land's End
			"basestation-tcp-synthetic",
		),
	}

	event, err := state.Apply(tf)
	require.NoError(t, err)

	// Drone snapshot present + correct.
	require.NotNil(t, event.Drone, "SSEEvent.Drone must be populated")
	assert.Equal(t, "15810ABC", event.Drone.DroneID)
	assert.InDelta(t, 50.1071, event.Drone.Lat, 1e-9)

	// Operator snapshot present + correct.
	require.NotNil(t, event.Operator, "SSEEvent.Operator must be populated when inner frame has operator")
	assert.Equal(t, "OP-O-001", event.Operator.OperatorID)
	assert.Equal(t, "caa", event.Operator.OperatorIDType)
	assert.InDelta(t, 50.1027, event.Operator.Lat, 1e-9, "operator lat must match inner.operator.position.lat")
	assert.InDelta(t, -5.6705, event.Operator.Lon, 1e-9, "operator lon must match inner.operator.position.lon")
	assert.Equal(t, "15810ABC", event.Operator.DroneID, "soft pairing hint: operator.droneId mirrors the parent drone")
	assert.Equal(t, "remote-id", event.Operator.Source)
	assert.Equal(t, "16Uiu2HAm-test-seller", event.Operator.SellerPeerID)
	assert.Equal(t, "basestation-tcp-synthetic", event.Operator.FrameSource,
		"operator.frameSource mirrors the inner RemoteIdFrame.source so the SYN PILOT badge can render")

	// Both keyspaces populated.
	drones := state.Snapshot()
	require.Len(t, drones, 1)
	operators := state.OperatorSnapshots()
	require.Len(t, operators, 1)
}

// TestApply_DroneWithoutOperator_OnlyDronePopulated — backward
// compatibility: a drone-only inner frame must still work and must
// NOT materialise an operator snapshot.
func TestApply_DroneWithoutOperator_OnlyDronePopulated(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source:     "remote-id",
		Type:       "drone",
		ReceivedAt: time.Now().UTC(),
		Frame: remoteIDFrameWithOperator(
			"D-NO-OP", "", // empty operatorID disables operator emission
			51.0, -0.5, 0, 0,
			"",
		),
	}
	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.NotNil(t, event.Drone)
	assert.Nil(t, event.Operator, "no inner operator → SSEEvent.Operator must be nil")
	assert.Empty(t, state.OperatorSnapshots(), "operators keyspace stays empty")
	assert.Len(t, state.Snapshot(), 1)
}

// TestApply_OperatorWithoutPosition_NotMaterialised — guard against
// rendering a marker we have no coordinates for.
func TestApply_OperatorWithoutPosition_NotMaterialised(t *testing.T) {
	t.Parallel()
	state := NewState()
	// Hand-build a frame whose operator has an ID but no position.
	tf := TaggedFrame{
		Source:     "remote-id",
		Type:       "drone",
		ReceivedAt: time.Now().UTC(),
		Frame: []byte(`{
			"droneId": "D-POSLESS-OP",
			"droneIdType": "serial",
			"position": {"lat": 51, "lon": -0.5, "alt": 100, "fix": "3D"},
			"operator": {"idType": "caa", "id": "OP-POSLESS"}
		}`),
	}
	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.NotNil(t, event.Drone)
	assert.Nil(t, event.Operator, "operator without position must NOT materialise a snapshot")
	assert.Empty(t, state.OperatorSnapshots())
}

// TestApply_OperatorIDChanges_BothKeyed — two distinct operator IDs
// over time both land in the operators keyspace (keyed independently).
// Mirrors the multi-operator real-world case (rare but possible).
func TestApply_OperatorIDChanges_BothKeyed(t *testing.T) {
	t.Parallel()
	state := NewState()
	now := time.Now().UTC()

	for _, op := range []string{"OP-A", "OP-B"} {
		tf := TaggedFrame{
			Source:     "remote-id",
			Type:       "drone",
			ReceivedAt: now,
			Frame: remoteIDFrameWithOperator(
				"D-MULTI", op,
				51.0, -0.5,
				50.1027, -5.6705,
				"",
			),
		}
		_, err := state.Apply(tf)
		require.NoError(t, err)
	}

	operators := state.OperatorSnapshots()
	require.Len(t, operators, 2, "distinct operator IDs must coexist in the operators keyspace")
	ids := map[string]bool{operators[0].OperatorID: true, operators[1].OperatorID: true}
	assert.True(t, ids["OP-A"])
	assert.True(t, ids["OP-B"])
}

// TestStateHandler_IncludesOperators — /state.json surface contract:
// the operators array MUST be present, and `count` MUST include it.
func TestStateHandler_IncludesOperators(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source:       "remote-id",
		Type:         "drone",
		SellerPeerID: "PID",
		ReceivedAt:   time.Now().UTC(),
		Frame: remoteIDFrameWithOperator(
			"D-STATE", "OP-STATE",
			51.0, -0.5,
			50.1027, -5.6705,
			"basestation-tcp-synthetic",
		),
	}
	_, err := state.Apply(tf)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/state.json", nil)
	rec := httptest.NewRecorder()
	stateHandler(state)(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	operators, ok := body["operators"].([]any)
	require.True(t, ok, "/state.json must include an operators array")
	require.Len(t, operators, 1)

	// count must include drones + aircraft + normalizedTracks + operators.
	count, ok := body["count"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(2), count, "count = 1 drone + 1 operator")
}

// TestApply_OperatorEvictionIndependent — drone and operator have
// independent LastSeen TTLs, so a stalled drone feed must not evict
// the operator (and vice versa).
func TestApply_OperatorEvictionIndependent(t *testing.T) {
	t.Parallel()
	state := NewState()
	old := time.Now().Add(-10 * time.Minute)
	fresh := time.Now()

	// Old drone+operator pair (both stale).
	oldFrame := TaggedFrame{
		Source: "remote-id", Type: "drone", ReceivedAt: old,
		Frame: remoteIDFrameWithOperator("D-OLD", "OP-OLD",
			51.0, -0.5, 50.1, -5.6, ""),
	}
	_, err := state.Apply(oldFrame)
	require.NoError(t, err)

	// Fresh drone-only frame (no operator).
	freshDroneFrame := TaggedFrame{
		Source: "remote-id", Type: "drone", ReceivedAt: fresh,
		Frame: remoteIDFrameWithOperator("D-FRESH", "",
			51.0, -0.5, 0, 0, ""),
	}
	_, err = state.Apply(freshDroneFrame)
	require.NoError(t, err)

	// Evict everything older than 5 minutes.
	n := state.Evict(time.Now(), 5*time.Minute)
	assert.Equal(t, 2, n, "old drone + old operator both evicted")

	assert.Len(t, state.Snapshot(), 1, "fresh drone remains")
	assert.Empty(t, state.OperatorSnapshots(), "old operator evicted; fresh drone has no operator")
}

// TestSSEEvent_DroneEventCarriesOperator — verifies the SSE event
// shape we emit so the browser can dispatch ev.operator alongside
// ev.drone in lockstep.
func TestSSEEvent_DroneEventCarriesOperator(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "remote-id", Type: "drone", ReceivedAt: time.Now().UTC(),
		Frame: remoteIDFrameWithOperator(
			"D-SSE", "OP-SSE",
			51.0, -0.5,
			50.1027, -5.6705,
			"basestation-tcp-synthetic",
		),
	}
	event, err := state.Apply(tf)
	require.NoError(t, err)
	assert.Equal(t, "drone", event.Kind, "Kind stays \"drone\" — additive extension only")
	require.NotNil(t, event.Drone)
	require.NotNil(t, event.Operator, "drone events MUST carry the paired operator when the inner frame has one")

	// Round-trip the SSEEvent through JSON to confirm the wire shape
	// the browser will see.
	b, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"operator":{`, "marshalled SSEEvent must contain the operator object")
	assert.Contains(t, string(b), `"drone":{`, "marshalled SSEEvent must still contain the drone object")
}
