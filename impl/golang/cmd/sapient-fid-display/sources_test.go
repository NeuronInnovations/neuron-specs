package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
)

// jvCardJSON / ridCardJSON are minimal agentURI payloads carrying the
// capability extension shapes ParseCardMeta consumes (the full-fidelity parse
// is covered against real BuildSellerCard output in internal/dapp/sapient).
const jvCardJSON = `{"services":[
  {"type":"neuron-topic","channel":"stdOut","config":{"neuron.adsb/1":{
    "nodeId":"node-jv","wire":"BSI Flex 335 v2.0 protobuf",
    "capabilities":["sapient.bsi-flex-335-v2.0","sapient.detection-report","neuron.adsb/1","jetvision.air-squitter.aircraftlist"],
    "sensorModels":["JetVision Air!Squitter"]}}},
  {"type":"neuron-commerce","name":"jetvision-adsb-sapient","settlement":{"binding":"none"}}]}`

const ridCardJSON = `{"services":[
  {"type":"neuron-topic","channel":"stdOut","config":{"neuron.rid/1":{
    "nodeId":"node-rid","wire":"BSI Flex 335 v2.0 protobuf",
    "sensorModels":["DroneScout DS240","DroneScout DS-400"]}}},
  {"type":"neuron-commerce","name":"rid","settlement":{"binding":"none"}}]}`

func jvSeed() sourceSeed {
	ev := sapient.AgentEvidence{
		AgentID: "2", SellerEVM: "0xJV", NodeID: "node-jv", PeerID: "peer-jv",
		Service: "jetvision-adsb-sapient", Protocol: "/sapient/detection/2.0.0",
		RegistryAddress: "0xREG", Simulated: false, FeedSource: "live",
		TransactionHash: "0xtx", AgentURI: json.RawMessage(jvCardJSON),
	}
	return sourceSeed{ev: ev, meta: sapient.ParseCardMeta(ev.AgentURI)}
}

func ridSeed() sourceSeed {
	ev := sapient.AgentEvidence{
		AgentID: "1", SellerEVM: "0xRID", NodeID: "node-rid", PeerID: "peer-rid",
		Service: "rid", Protocol: "/sapient/detection/2.0.0",
		RegistryAddress: "0xREG", Simulated: false, FeedSource: "live",
		AgentURI: json.RawMessage(ridCardJSON),
	}
	return sourceSeed{ev: ev, meta: sapient.ParseCardMeta(ev.AgentURI)}
}

func trackFor(nodeID, uid, kind string, lastSeen time.Time) TrackSnapshot {
	return TrackSnapshot{UID: uid, NodeID: nodeID, Kind: kind, LastSeen: lastSeen}
}

func sourceByName(t *testing.T, sources []SourceView, name string) SourceView {
	t.Helper()
	for _, s := range sources {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no source named %q in %+v", name, sources)
	return SourceView{}
}

// TestBuildSources_EvidenceOnlyOfflineCard is the DroneScout live truth:
// sessions feed healthy, no rid session → OFFLINE, with identity +
// capabilities + honest heartbeat/commerce posture intact.
func TestBuildSources_EvidenceOnlyOfflineCard(t *testing.T) {
	t.Parallel()
	now := time.Now()
	sess := &SessionsSnapshot{PolledAt: now, Entries: nil} // healthy, empty

	sources := BuildSources([]sourceSeed{ridSeed()}, sess, nil, nil, now, time.Minute)
	require.Len(t, sources, 1)
	s := sources[0]
	assert.Equal(t, "DroneScout RID", s.Name)
	assert.Equal(t, statusOffline, s.Status)
	require.NotNil(t, s.SessionConnected)
	assert.False(t, *s.SessionConnected)
	assert.Equal(t, "1", s.AgentID)
	assert.Equal(t, "rid", s.Service)
	assert.False(t, s.Simulated)
	assert.Equal(t, []string{"DroneScout DS240", "DroneScout DS-400"}, s.SensorModels)
	assert.Equal(t, "advertisement-only", s.Commerce.Mode)
	assert.Equal(t, "not-published", s.Heartbeat.Status)
	assert.Zero(t, s.TrackCounts.Total)
}

func TestBuildSources_LiveSessionWithTrackCounts(t *testing.T) {
	t.Parallel()
	now := time.Now()
	rate := 5.5
	sess := &SessionsSnapshot{PolledAt: now, Entries: []SessionEntry{{
		PeerID: "peer-jv", NodeID: "node-jv", RemoteAddr: "/ip4/1.2.3.4/udp/1/quic-v1",
		LastSeen: now.Add(-2 * time.Second), MessageCount: 1234,
	}}}
	tracks := []TrackSnapshot{
		trackFor("node-jv", "AC1", "adsb", now.Add(-1*time.Second)),
		trackFor("node-jv", "AC2", "adsb", now.Add(-5*time.Second)),
		trackFor("node-jv", "AC3", "adsb", now.Add(-2*time.Minute)), // stale
	}

	sources := BuildSources([]sourceSeed{jvSeed()}, sess, map[string]*float64{"peer-jv": &rate}, tracks, now, time.Minute)
	s := sourceByName(t, sources, "JetVision ADS-B")
	assert.Equal(t, statusLive, s.Status)
	require.NotNil(t, s.SessionConnected)
	assert.True(t, *s.SessionConnected)
	assert.Equal(t, uint64(1234), s.MessageCount)
	require.NotNil(t, s.MessageRate)
	assert.InDelta(t, 5.5, *s.MessageRate, 1e-9)
	assert.Equal(t, TrackCounts{Total: 3, Aircraft: 3, Stale: 1}, s.TrackCounts)
	require.NotNil(t, s.LastSeen)
	assert.Equal(t, "adsb", s.Modality)
	assert.Len(t, s.Capabilities, 4)
}

func TestBuildSources_NodeIDLessSessionMatchedByPeerID(t *testing.T) {
	t.Parallel()
	now := time.Now()
	// The buyer captures node_id from the FIRST message; an idle seller's
	// session has nodeId=="" and messageCount==0 — matched via evidence peerID.
	sess := &SessionsSnapshot{PolledAt: now, Entries: []SessionEntry{{
		PeerID: "peer-rid", NodeID: "", LastSeen: now, MessageCount: 0,
	}}}

	sources := BuildSources([]sourceSeed{ridSeed()}, sess, nil, nil, now, time.Minute)
	s := sources[0]
	assert.Equal(t, statusLive, s.Status)
	assert.True(t, s.AwaitingFirstMessage, "connected, awaiting first message")
	require.NotNil(t, s.SessionConnected)
	assert.True(t, *s.SessionConnected)
}

func TestBuildSources_NoSessionsURLTrackRecencyOnly(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tracks := []TrackSnapshot{trackFor("node-jv", "AC1", "adsb", now.Add(-3*time.Second))}

	sources := BuildSources([]sourceSeed{jvSeed(), ridSeed()}, nil, nil, tracks, now, time.Minute)
	jv := sourceByName(t, sources, "JetVision ADS-B")
	rid := sourceByName(t, sources, "DroneScout RID")
	assert.Equal(t, statusLive, jv.Status, "fresh tracks → live by recency")
	assert.Nil(t, jv.SessionConnected, "no sessions feed → connection unknown")
	assert.Equal(t, statusUnknown, rid.Status, "no tracks, no feed → unknown, never fake offline")

	// The JSON must omit sessionConnected entirely (nil pointer).
	b, err := json.Marshal(jv)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &raw))
	_, has := raw["sessionConnected"]
	assert.False(t, has)
}

func TestBuildSources_ConnectedIdleIsStale(t *testing.T) {
	t.Parallel()
	now := time.Now()
	sess := &SessionsSnapshot{PolledAt: now, Entries: []SessionEntry{{
		PeerID: "peer-jv", NodeID: "node-jv",
		LastSeen: now.Add(-5 * time.Minute), MessageCount: 99,
	}}}

	sources := BuildSources([]sourceSeed{jvSeed()}, sess, nil, nil, now, time.Minute)
	s := sources[0]
	assert.Equal(t, statusStale, s.Status, "connected but silent past the threshold")
	require.NotNil(t, s.SessionConnected)
	assert.True(t, *s.SessionConnected)
}

func TestBuildSources_PollErrorFallsBackHonestly(t *testing.T) {
	t.Parallel()
	now := time.Now()
	sess := &SessionsSnapshot{PolledAt: now, Err: "connection refused"}
	tracks := []TrackSnapshot{trackFor("node-jv", "AC1", "adsb", now.Add(-2*time.Second))}

	sources := BuildSources([]sourceSeed{jvSeed(), ridSeed()}, sess, nil, tracks, now, time.Minute)
	jv := sourceByName(t, sources, "JetVision ADS-B")
	rid := sourceByName(t, sources, "DroneScout RID")
	assert.Equal(t, statusLive, jv.Status, "failing poll degrades to track recency")
	assert.Nil(t, jv.SessionConnected, "failing poll cannot claim connection state")
	assert.Equal(t, statusUnknown, rid.Status, "NEVER fake offline on a failing poll")
}

func TestBuildSources_SyntheticSourceForUnknownNodeID(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tracks := []TrackSnapshot{trackFor("node-mystery", "X1", "adsb", now)}

	sources := BuildSources([]sourceSeed{jvSeed()}, nil, nil, tracks, now, time.Minute)
	require.Len(t, sources, 2)
	var synth *SourceView
	for i := range sources {
		if sources[i].Unregistered {
			synth = &sources[i]
		}
	}
	require.NotNil(t, synth, "tracks from an unknown node get an honest synthetic card")
	assert.Contains(t, synth.Name, "unregistered source")
	assert.Equal(t, "node-mystery", synth.NodeID)
	assert.Equal(t, 1, synth.TrackCounts.Total)
}

func TestSourceName_Derivation(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "JetVision ADS-B", sourceName(jvSeed().ev, jvSeed().meta))
	assert.Equal(t, "DroneScout RID", sourceName(ridSeed().ev, ridSeed().meta))
	// Fallbacks: no extension → service name; nothing → short node id.
	ev := sapient.AgentEvidence{Service: "custom-svc"}
	assert.Equal(t, "custom-svc", sourceName(ev, sapient.CardMeta{}))
	ev = sapient.AgentEvidence{NodeID: "0123456789abcdef"}
	assert.Equal(t, "source 01234567…", sourceName(ev, sapient.CardMeta{}))
}

func TestMessageRate_WindowAndCounterReset(t *testing.T) {
	t.Parallel()
	now := time.Now()
	mk := func(secAgo int, count uint64) ratePoint {
		return ratePoint{At: now.Add(-time.Duration(secAgo) * time.Second), CountByKey: map[string]uint64{"p": count}}
	}

	// Steady 2 msg/s over 20s.
	r := messageRate([]ratePoint{mk(20, 100), mk(10, 120), mk(0, 140)}, "p", time.Minute, now)
	require.NotNil(t, r)
	assert.InDelta(t, 2.0, *r, 1e-9)

	// Single sample → unknown, not zero.
	assert.Nil(t, messageRate([]ratePoint{mk(0, 100)}, "p", time.Minute, now))

	// Counter reset (reconnect): measure from the reset sample onward.
	r = messageRate([]ratePoint{mk(30, 900), mk(20, 950), mk(10, 5), mk(0, 25)}, "p", time.Minute, now)
	require.NotNil(t, r)
	assert.InDelta(t, 2.0, *r, 1e-9)

	// Samples outside the window are ignored.
	r = messageRate([]ratePoint{mk(120, 0), mk(10, 100), mk(0, 110)}, "p", time.Minute, now)
	require.NotNil(t, r)
	assert.InDelta(t, 1.0, *r, 1e-9)

	// Unknown key → nil.
	assert.Nil(t, messageRate([]ratePoint{mk(10, 1), mk(0, 2)}, "other", time.Minute, now))
}

func TestSessionsPoller_PollsAndSurfacesError(t *testing.T) {
	t.Parallel()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_ = json.NewEncoder(w).Encode(map[string]any{"sessions": []SessionEntry{{
			PeerID: "p1", NodeID: "n1", MessageCount: uint64(100 * hits), LastSeen: time.Now(),
		}}})
	}))

	p := NewSessionsPoller(srv.URL)
	ctx, cancel := context.WithCancel(t.Context())
	go p.Run(ctx, 30*time.Millisecond, log.New(io.Discard, "", 0))

	require.Eventually(t, func() bool {
		cur := p.Current()
		return cur.Err == "" && len(cur.Entries) == 1 && cur.Entries[0].MessageCount >= 200
	}, 5*time.Second, 10*time.Millisecond, "poller must accumulate samples")
	require.Eventually(t, func() bool {
		return p.RateFor("p1", time.Now()) != nil
	}, 5*time.Second, 10*time.Millisecond, "two samples → a rate")

	// Endpoint dies → the error is surfaced, entries retained from last good poll.
	srv.Close()
	require.Eventually(t, func() bool {
		return p.Current().Err != ""
	}, 5*time.Second, 10*time.Millisecond, "poll failure must be surfaced, not swallowed")
	cancel()
}

// TestStateJSONFull_AdditiveSupersetOfLegacy: every key of the legacy
// /state.json appears identically in the full handler's output; the full
// handler only ADDS (sources, staleAfterMs, now, sessions, per-track ageMs/stale).
func TestStateJSONFull_AdditiveSupersetOfLegacy(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, fullTrackJSON))
	require.NoError(t, err)
	_, err = state.Apply(taggedTrack(t, adsbTrackJSON))
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, sapient.WriteEvidence(filepath.Join(dir, "a-rid.json"), ridSeed().ev))
	require.NoError(t, sapient.WriteEvidence(filepath.Join(dir, "b-jv.json"), jvSeed().ev))
	prov := &SourceProvider{Loader: newSeedLoader(nil, dir, log.New(io.Discard, "", 0)), StaleAfter: time.Minute}

	legacyRec := httptest.NewRecorder()
	stateHandler(state)(legacyRec, httptest.NewRequest(http.MethodGet, "/state.json", nil))
	fullRec := httptest.NewRecorder()
	stateHandlerFull(state, prov)(fullRec, httptest.NewRequest(http.MethodGet, "/state.json", nil))

	var legacy, full map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(legacyRec.Body.Bytes(), &legacy))
	require.NoError(t, json.Unmarshal(fullRec.Body.Bytes(), &full))

	// Top-level: count and focus byte-identical; tracks a per-element superset.
	assert.JSONEq(t, string(legacy["count"]), string(full["count"]))
	if f, ok := legacy["focus"]; ok {
		assert.JSONEq(t, string(f), string(full["focus"]))
	}
	for _, k := range []string{"sources", "staleAfterMs", "now", "sessions"} {
		_, ok := full[k]
		assert.True(t, ok, "full payload must carry %q", k)
	}

	var legacyTracks, fullTracks []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(legacy["tracks"], &legacyTracks))
	require.NoError(t, json.Unmarshal(full["tracks"], &fullTracks))
	require.Equal(t, len(legacyTracks), len(fullTracks))
	// Order both by uid for comparison.
	uidOf := func(m map[string]json.RawMessage) string {
		var s string
		_ = json.Unmarshal(m["uid"], &s)
		return s
	}
	byUID := map[string]map[string]json.RawMessage{}
	for _, ft := range fullTracks {
		byUID[uidOf(ft)] = ft
	}
	for _, lt := range legacyTracks {
		ft, ok := byUID[uidOf(lt)]
		require.True(t, ok)
		for k, v := range lt {
			if k == "lastSeen" || k == "receivedAt" {
				continue // serialization timing identical anyway, but be robust
			}
			assert.JSONEq(t, string(v), string(ft[k]), "legacy track key %q must be unchanged", k)
		}
		for _, k := range []string{"ageMs", "stale"} {
			_, ok := ft[k]
			assert.True(t, ok, "full track must add %q", k)
		}
	}

	// sessions meta says configured=false (no poller in this provider).
	var sessMeta map[string]any
	require.NoError(t, json.Unmarshal(full["sessions"], &sessMeta))
	assert.Equal(t, false, sessMeta["configured"])
}

func TestTrackViews_AgeMsAndStale(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tracks := []TrackSnapshot{
		{UID: "fresh", LastSeen: now.Add(-10 * time.Second)},
		{UID: "old", LastSeen: now.Add(-3 * time.Minute)},
	}
	views := trackViews(tracks, now, time.Minute)
	require.Len(t, views, 2)
	assert.InDelta(t, 10_000, views[0].AgeMs, 50)
	assert.False(t, views[0].Stale)
	assert.True(t, views[1].Stale)
}

func TestSeedLoader_FilesDirAndLazyRetry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a-rid.json")
	require.NoError(t, sapient.WriteEvidence(p1, ridSeed().ev))
	p2 := filepath.Join(dir, "b-jv.json")
	require.NoError(t, sapient.WriteEvidence(p2, jvSeed().ev))

	logger := log.New(io.Discard, "", 0)
	l := newSeedLoader([]string{p1}, "", logger)
	seeds := l.seeds()
	require.Len(t, seeds, 1)
	assert.Equal(t, "node-rid", seeds[0].ev.NodeID)
	assert.Equal(t, "rid", seeds[0].meta.Modality)

	require.Len(t, newSeedLoader(nil, dir, logger).seeds(), 2)

	// LAZY RETRY — the seller writes its evidence AFTER the display starts:
	// a missing path yields nothing now and loads on a later call.
	late := filepath.Join(t.TempDir(), "late.json")
	lazy := newSeedLoader([]string{late}, "", logger)
	assert.Empty(t, lazy.seeds(), "missing evidence: no seed yet, no crash")
	require.NoError(t, sapient.WriteEvidence(late, jvSeed().ev))
	require.Len(t, lazy.seeds(), 1, "evidence picked up once the seller writes it")
}
