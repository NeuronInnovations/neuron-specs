package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discardLogger() *log.Logger { return log.New(io.Discard, "", 0) }

// fullTrackJSON is a complete SapientTrackSnapshot per
// docs/sapient-track-contract.md v1.0.0 — every optional block present.
const fullTrackJSON = `{
  "type":"sapient-track","version":"1.0.0",
  "uid":"1581F8B1234567890ABC",
  "objectId":"01KT3QF9AJHS570QGVY2JJHQE7",
  "reportId":"1581F8B1234567890ABC@1780389225555000000",
  "nodeId":"e0a80d80-7684-56ca-8968-ef4b3688bdcb",
  "position":{"lat":50.1027,"lon":-5.6635,"alt":95},
  "velocity":{"speedMps":10.0,"trackDeg":0},
  "classification":{"type":"UAV","confidence":0.99},
  "feedSource":"synthetic",
  "wire":"BSI Flex 335 v2.0 protobuf",
  "rid":{"serial":"1581F8B1234567890ABC","uasId":"1581F8B1234567890ABC","idType":"SerialNumber",
         "uaType":"Multirotor","macAddress":"AA:BB:CC:DD:EE:F0","status":"Airborne",
         "operatorId":"OP-NEURON-TEST-001","operatorLat":50.1027,"operatorLon":-5.6705,"operatorAltM":30},
  "rf":{"rssiDbm":-62,"frequencyHz":2437000000,"channel":"6","transport":"WiFi beacon"},
  "cot":{"uid":"1581F8B1234567890ABC","type":"a-f-A","how":"m-g","affiliation":"friendly","demoProfile":true},
  "agent":{"agentId":"1","sellerEVM":"0xa7D677D2C5cA238faf7Aff8cEAff3991cF888D2c",
           "peerID":"16Uiu2HAmMXGq","nodeId":"e0a80d80-7684-56ca-8968-ef4b3688bdcb",
           "service":"rid","protocol":"/sapient/detection/2.0.0","simulated":true}
}`

func taggedTrack(t *testing.T, inner string) TaggedFrame {
	t.Helper()
	return TaggedFrame{
		Source:     "sapient",
		Type:       "track",
		ReceivedAt: time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
		Frame:      json.RawMessage(inner),
	}
}

// TestApply_AcceptsFullSapientTrack: every contract block lands in state.
func TestApply_AcceptsFullSapientTrack(t *testing.T) {
	t.Parallel()
	state := NewState()

	event, err := state.Apply(taggedTrack(t, fullTrackJSON))
	require.NoError(t, err)
	require.Equal(t, "sapient-track", event.Kind)
	require.NotNil(t, event.Track)

	snaps := state.Snapshot()
	require.Len(t, snaps, 1)
	s := snaps[0]

	assert.Equal(t, "1581F8B1234567890ABC", s.UID)
	assert.Equal(t, "01KT3QF9AJHS570QGVY2JJHQE7", s.ObjectID)
	assert.NotEmpty(t, s.ReportID)
	assert.Equal(t, "e0a80d80-7684-56ca-8968-ef4b3688bdcb", s.NodeID)
	assert.Equal(t, "sapient", s.Source)
	assert.Equal(t, uint64(1), s.FrameCount)

	require.NotNil(t, s.Position)
	assert.InDelta(t, 50.1027, s.Position.Lat, 1e-9)
	require.NotNil(t, s.Velocity)
	assert.InDelta(t, 10.0, s.Velocity.SpeedMps, 1e-9)
	require.NotNil(t, s.Classification)
	assert.Equal(t, "UAV", s.Classification.Type)
	assert.InDelta(t, 0.99, s.Classification.Confidence, 1e-9)

	require.NotNil(t, s.RID)
	assert.Equal(t, "OP-NEURON-TEST-001", s.RID.OperatorID)
	require.NotNil(t, s.RID.OperatorLat, "operator position present")
	assert.InDelta(t, 50.1027, *s.RID.OperatorLat, 1e-9)
	require.NotNil(t, s.RID.OperatorLon)
	assert.InDelta(t, -5.6705, *s.RID.OperatorLon, 1e-9)

	require.NotNil(t, s.RF)
	require.NotNil(t, s.RF.RssiDbm)
	assert.InDelta(t, -62, *s.RF.RssiDbm, 1e-9)
	require.NotNil(t, s.RF.FrequencyHz)
	assert.Equal(t, "WiFi beacon", s.RF.Transport)

	assert.Equal(t, "synthetic", s.FeedSource)
	assert.Equal(t, "BSI Flex 335 v2.0 protobuf", s.Wire)
}

// TestApply_FriendlyCoTMetadata: the friendly demo profile is carried as data
// (a-f-A + demoProfile=true) for the UI's FRIENDLY pill.
func TestApply_FriendlyCoTMetadata(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, fullTrackJSON))
	require.NoError(t, err)

	s := state.Snapshot()[0]
	require.NotNil(t, s.CoT)
	assert.Equal(t, "a-f-A", s.CoT.Type)
	assert.Equal(t, "m-g", s.CoT.How)
	assert.Equal(t, "friendly", s.CoT.Affiliation)
	assert.True(t, s.CoT.DemoProfile, "FRIENDLY is labelled a demo profile")
}

// TestApply_AgentCardFieldsPersist: the EIP-8004 identity survives into state
// (and therefore /state.json) with the SIM flag intact.
func TestApply_AgentCardFieldsPersist(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, fullTrackJSON))
	require.NoError(t, err)

	s := state.Snapshot()[0]
	require.NotNil(t, s.Agent)
	assert.Equal(t, "1", s.Agent.AgentID)
	assert.Equal(t, "0xa7D677D2C5cA238faf7Aff8cEAff3991cF888D2c", s.Agent.SellerEVM)
	assert.Equal(t, "16Uiu2HAmMXGq", s.Agent.PeerID)
	assert.Equal(t, "rid", s.Agent.Service)
	assert.Equal(t, "/sapient/detection/2.0.0", s.Agent.Protocol)
	assert.True(t, s.Agent.Simulated, "in-memory registry labelled SIM")
}

// TestApply_MinimalTrackOmitsOptionals: uid+position only — optional blocks
// stay nil, nothing crashes, the track still renders.
func TestApply_MinimalTrackOmitsOptionals(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, `{"uid":"BARE","position":{"lat":1,"lon":2,"alt":3}}`))
	require.NoError(t, err)

	s := state.Snapshot()[0]
	assert.Equal(t, "BARE", s.UID)
	require.NotNil(t, s.Position)
	assert.Nil(t, s.Velocity)
	assert.Nil(t, s.Classification)
	assert.Nil(t, s.RID)
	assert.Nil(t, s.RF)
	assert.Nil(t, s.CoT)
	assert.Nil(t, s.Agent)
	assert.Empty(t, s.FeedSource)
}

// Velocity honesty passthrough (the FID "0 km/h" rule): a PRESENT zero vector
// is genuine data (stationary broadcast) and survives into state as 0; an
// ABSENT one must stay omitted from the snapshot JSON so the UI renders '—',
// never a fabricated 0 km/h.
func TestApply_ZeroVelocityPreserved(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t,
		`{"uid":"ZERO","position":{"lat":1,"lon":2,"alt":3},"velocity":{"speedMps":0,"trackDeg":0}}`))
	require.NoError(t, err)

	s := state.Snapshot()[0]
	require.NotNil(t, s.Velocity, "genuine zero velocity stays present")
	assert.Zero(t, s.Velocity.SpeedMps)
	assert.Zero(t, s.Velocity.TrackDeg)

	b, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"velocity"`, "present zero serialized for the UI")
}

func TestStateJSON_OmitsAbsentVelocity(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, `{"uid":"NOVEL","position":{"lat":1,"lon":2,"alt":3}}`))
	require.NoError(t, err)

	s := state.Snapshot()[0]
	assert.Nil(t, s.Velocity)
	b, err := json.Marshal(s)
	require.NoError(t, err)
	assert.NotContains(t, string(b), `"velocity"`, "absent velocity omitted (UI shows '—')")
}

// ─── Auto-focus (FocusFor) ──────────────────────────────────────────────────
// The five required behaviors: no tracks → nil (default view), one track →
// center on it, cluster beats outlier, operators never drive selection (only
// extend bounds when leashed to a member), invalid coordinates ignored.

func posTrack(uid string, lat, lon float64) TrackSnapshot {
	return TrackSnapshot{UID: uid, Position: &Position{Lat: lat, Lon: lon, Alt: 10}}
}

func TestFocusFor_NoTracks(t *testing.T) {
	t.Parallel()
	assert.Nil(t, FocusFor(nil), "no tracks → nil → client keeps default view")
	assert.Nil(t, FocusFor([]TrackSnapshot{{UID: "NOPOS"}}), "position-less track cannot drive focus")
}

func TestFocusFor_SingleTrack(t *testing.T) {
	t.Parallel()
	f := FocusFor([]TrackSnapshot{posTrack("A", 50.1027, -5.6705)})
	require.NotNil(t, f)
	assert.Equal(t, 1, f.Count)
	assert.InDelta(t, 50.1027, f.Lat, 1e-9)
	assert.InDelta(t, -5.6705, f.Lon, 1e-9)
}

func TestFocusFor_ClusterBeatsOutlier(t *testing.T) {
	t.Parallel()
	// Three drones within ~300 m in London + one outlier ~10 km away.
	f := FocusFor([]TrackSnapshot{
		posTrack("A", 50.1027, -5.6705),
		posTrack("B", 50.1032, -5.6697),
		posTrack("C", 50.1022, -5.6712),
		posTrack("OUTLIER", 50.1927, -5.6705), // ~10 km north
	})
	require.NotNil(t, f)
	assert.Equal(t, 3, f.Count, "outlier excluded from the cluster")
	assert.Less(t, f.MaxLat, 51.60, "bounds must not stretch to the outlier")
	assert.InDelta(t, 50.1027, f.Lat, 0.001, "centroid sits on the cluster")
}

func TestFocusFor_OperatorDoesNotDriveFocus(t *testing.T) {
	t.Parallel()
	op := func(lat, lon float64) (*float64, *float64) { return &lat, &lon }

	// Selection: a lone drone whose operator sits near the two clustered
	// drones must NOT outweigh the real two-drone cluster.
	soloOpLat, soloOpLon := op(50.2743, -5.0436)
	f := FocusFor([]TrackSnapshot{
		{UID: "SOLO", Position: &Position{Lat: 50.2742, Lon: -5.0437},
			RID: &RIDInfo{OperatorLat: soloOpLat, OperatorLon: soloOpLon}},
		posTrack("B", 50.1032, -5.6697),
		posTrack("C", 50.1022, -5.6712),
	})
	require.NotNil(t, f)
	assert.Equal(t, 2, f.Count, "the two-drone cluster wins; operators never drive selection")
	assert.Less(t, f.MaxLat, 51.6, "solo drone + its operator are outside the viewport")

	// Bounds: a member's LEASHED operator (≤2 km) extends the viewport …
	nearOpLat, nearOpLon := op(50.1092, -5.6705) // ~700 m north of the drone
	f = FocusFor([]TrackSnapshot{{
		UID: "A", Position: &Position{Lat: 50.1027, Lon: -5.6705},
		RID: &RIDInfo{OperatorLat: nearOpLat, OperatorLon: nearOpLon},
	}})
	require.NotNil(t, f)
	assert.Equal(t, 1, f.Count)
	assert.GreaterOrEqual(t, f.MaxLat, 50.1092, "nearby operator pulled into bounds")

	// … but an implausibly distant one (>2 km) does not.
	farOpLat, farOpLon := op(50.2742, -5.6705) // ~19 km away
	f = FocusFor([]TrackSnapshot{{
		UID: "A", Position: &Position{Lat: 50.1027, Lon: -5.6705},
		RID: &RIDInfo{OperatorLat: farOpLat, OperatorLon: farOpLon},
	}})
	require.NotNil(t, f)
	assert.Less(t, f.MaxLat, 51.54, "unleashed operator ignored for bounds")
}

func TestFocusFor_InvalidCoordinatesIgnored(t *testing.T) {
	t.Parallel()
	f := FocusFor([]TrackSnapshot{
		{UID: "NOPOS"},                     // no position block
		posTrack("BADLAT", 91.0, -5.6705),  // out of range
		posTrack("BADLON", 50.1027, 181.0), // out of range
		posTrack("NULLISLAND", 0, 0),       // absent-fields artifact
		posTrack("GOOD", 50.1027, -5.6705), // the only usable one
	})
	require.NotNil(t, f)
	assert.Equal(t, 1, f.Count, "only the valid coordinate drives focus")
	assert.InDelta(t, 50.1027, f.Lat, 1e-9)

	assert.Nil(t, FocusFor([]TrackSnapshot{posTrack("NULLISLAND", 0, 0)}),
		"all-invalid → nil → default view")
}

func TestStateJSON_IncludesFocus(t *testing.T) {
	t.Parallel()
	state := NewState()
	rr := httptest.NewRecorder()
	stateHandler(state)(rr, httptest.NewRequest(http.MethodGet, "/state.json", nil))
	assert.NotContains(t, rr.Body.String(), `"focus"`, "empty state omits focus")

	_, err := state.Apply(taggedTrack(t, `{"uid":"A","position":{"lat":50.1027,"lon":-5.6705,"alt":10}}`))
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	stateHandler(state)(rr, httptest.NewRequest(http.MethodGet, "/state.json", nil))
	var body struct {
		Focus *Focus `json:"focus"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotNil(t, body.Focus, "state with a positioned track carries focus")
	assert.Equal(t, 1, body.Focus.Count)
	assert.InDelta(t, 50.1027, body.Focus.Lat, 1e-9)
}

// TestApply_NoPositionStillApplies: even position is optional at the state
// layer (the UI just places no marker) — missing fields never break state.
func TestApply_NoPositionStillApplies(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, `{"uid":"NOPOS"}`))
	require.NoError(t, err)
	require.Len(t, state.Snapshot(), 1)
	assert.Nil(t, state.Snapshot()[0].Position)
}

func TestApply_RejectsMissingUID(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, `{"position":{"lat":1,"lon":2,"alt":3}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing uid")
	assert.Empty(t, state.Snapshot())
}

func TestApply_RejectsNonSapientSource(t *testing.T) {
	t.Parallel()
	state := NewState()

	// A remote-id frame must NOT be accepted — this display is SAPIENT-only.
	tf := taggedTrack(t, `{"uid":"X"}`)
	tf.Source = "remote-id"
	_, err := state.Apply(tf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown source")

	// Wrong type under the right source is also rejected.
	tf = taggedTrack(t, `{"uid":"X"}`)
	tf.Type = "drone"
	_, err = state.Apply(tf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown type")

	assert.Empty(t, state.Snapshot())
}

func TestApply_UpdateReplacesAndCountsFrames(t *testing.T) {
	t.Parallel()
	state := NewState()

	_, err := state.Apply(taggedTrack(t, `{"uid":"U1","position":{"lat":1,"lon":2,"alt":3}}`))
	require.NoError(t, err)
	_, err = state.Apply(taggedTrack(t, `{"uid":"U1","position":{"lat":9,"lon":8,"alt":7}}`))
	require.NoError(t, err)

	snaps := state.Snapshot()
	require.Len(t, snaps, 1, "same uid replaces, not duplicates")
	assert.Equal(t, uint64(2), snaps[0].FrameCount)
	assert.InDelta(t, 9, snaps[0].Position.Lat, 1e-9)
}

func TestEvict_RemovesStaleTracks(t *testing.T) {
	t.Parallel()
	state := NewState()

	old := taggedTrack(t, `{"uid":"OLD","position":{"lat":1,"lon":2,"alt":3}}`)
	old.ReceivedAt = time.Now().Add(-time.Hour)
	_, err := state.Apply(old)
	require.NoError(t, err)

	fresh := taggedTrack(t, `{"uid":"FRESH","position":{"lat":1,"lon":2,"alt":3}}`)
	fresh.ReceivedAt = time.Now()
	_, err = state.Apply(fresh)
	require.NoError(t, err)

	n := state.Evict(time.Now(), 10*time.Minute)
	assert.Equal(t, 1, n)
	snaps := state.Snapshot()
	require.Len(t, snaps, 1)
	assert.Equal(t, "FRESH", snaps[0].UID)
}

func TestStateHandler_ShapeAndContent(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, fullTrackJSON))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	stateHandler(state)(rr, httptest.NewRequest(http.MethodGet, "/state.json", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Tracks []TrackSnapshot `json:"tracks"`
		Count  int             `json:"count"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, 1, body.Count)
	require.Len(t, body.Tracks, 1)
	assert.Equal(t, "1581F8B1234567890ABC", body.Tracks[0].UID)
	require.NotNil(t, body.Tracks[0].Agent, "agent card fields persist through state.json")
	assert.Equal(t, "1", body.Tracks[0].Agent.AgentID)
	require.NotNil(t, body.Tracks[0].CoT)
	assert.Equal(t, "a-f-A", body.Tracks[0].CoT.Type)
}

func TestConfigHandler_ReturnsMapCenter(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	configHandler(MapConfig{Lat: 50.1, Lon: -5.7, Zoom: 12})(rr, httptest.NewRequest(http.MethodGet, "/config.json", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var cfg MapConfig
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&cfg))
	assert.InDelta(t, 50.1, cfg.Lat, 1e-9)
	assert.Equal(t, 12, cfg.Zoom)
}

func TestIndexHandler_ServesEmbeddedHTML(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	indexHandler()(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	body := rr.Body.String()
	assert.Contains(t, body, "Neuron SAPIENT FID")
	// The honest-labeling footer is part of the contract.
	assert.Contains(t, body, "SAPIENT protobuf")
	assert.Contains(t, body, "demo CoT profile")
}

func TestSSE_DeliversTrackUpdate(t *testing.T) {
	t.Parallel()
	state := NewState()
	server := httptest.NewServer(http.HandlerFunc(eventsHandler(state)))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = state.Apply(taggedTrack(t, `{"uid":"SSE-TRACK","position":{"lat":1,"lon":2,"alt":3}}`))
	}()

	buf := make([]byte, 4096)
	deadline := time.Now().Add(2 * time.Second)
	var combined strings.Builder
	for time.Now().Before(deadline) && !strings.Contains(combined.String(), "SSE-TRACK") {
		n, rerr := resp.Body.Read(buf)
		if rerr != nil {
			break
		}
		combined.Write(buf[:n])
	}
	assert.Contains(t, combined.String(), "SSE-TRACK")
	assert.Contains(t, combined.String(), "event: update")
	assert.Contains(t, combined.String(), `"kind":"sapient-track"`)
}

func TestSSE_InitialSnapshot(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, `{"uid":"PRE-EXISTING","position":{"lat":1,"lon":2,"alt":3}}`))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(eventsHandler(state)))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	deadline := time.Now().Add(2 * time.Second)
	var combined strings.Builder
	for time.Now().Before(deadline) && !strings.Contains(combined.String(), "PRE-EXISTING") {
		n, rerr := resp.Body.Read(buf)
		if rerr != nil {
			break
		}
		combined.Write(buf[:n])
	}
	assert.Contains(t, combined.String(), "event: snapshot")
	assert.Contains(t, combined.String(), "PRE-EXISTING")
}

// TestTCPListenerIngest: end-to-end — a consumer-style connection writes
// sapient-track JSONL; tracks land in /state.json; non-sapient lines are
// skipped without killing the connection.
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

	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	records := []string{
		fmt.Sprintf(`{"source":"sapient","type":"track","receivedAt":"2026-06-03T10:00:00Z","frame":%s}`,
			strings.ReplaceAll(fullTrackJSON, "\n", "")),
		// A legacy remote-id line must be skipped, not crash the stream.
		`{"source":"remote-id","type":"drone","receivedAt":"2026-06-03T10:00:00Z","frame":{"droneId":"IGNORED"}}`,
		`{"source":"sapient","type":"track","receivedAt":"2026-06-03T10:00:01Z","frame":{"uid":"SECOND","position":{"lat":1,"lon":2,"alt":3}}}`,
	}
	for _, rec := range records {
		_, err := conn.Write([]byte(rec + "\n"))
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		return len(state.Snapshot()) == 2
	}, 2*time.Second, 20*time.Millisecond, "expected 2 sapient tracks; remote-id line skipped")

	rr := httptest.NewRecorder()
	stateHandler(state)(rr, httptest.NewRequest(http.MethodGet, "/state.json", nil))
	body := rr.Body.String()
	assert.Contains(t, body, `"uid":"1581F8B1234567890ABC"`)
	assert.Contains(t, body, `"uid":"SECOND"`)
	assert.NotContains(t, body, "IGNORED")
}
