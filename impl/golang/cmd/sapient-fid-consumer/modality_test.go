package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// loadADSBSamples reads the captured-from-the-real-neuron-jv-bridge NDJSON
// fixture (sources A=ADS-B, M=MLAT, L=OGN-local; relayed + position-less
// contacts already withheld by the bridge).
func loadADSBSamples(t *testing.T) []*sapientpb.SapientMessage {
	t.Helper()
	f, err := os.Open("testdata/adsb-sample.ndjson")
	require.NoError(t, err)
	defer f.Close()
	var msgs []*sapientpb.SapientMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		m := &sapientpb.SapientMessage{}
		require.NoError(t, protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(sc.Bytes(), m))
		msgs = append(msgs, m)
	}
	require.NoError(t, sc.Err())
	require.Len(t, msgs, 3)
	return msgs
}

func TestMessageModality(t *testing.T) {
	t.Parallel()
	for _, msg := range loadADSBSamples(t) {
		assert.Equal(t, "adsb", messageModality(msg), "jv-bridge output is adsb modality")
	}
	for _, msg := range loadSamples(t) {
		assert.Equal(t, "rid", messageModality(msg), "rid-bridge output is rid modality")
	}
	// A bare report with no object_info defaults to rid (legacy behavior).
	bare := &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{}},
	}
	assert.Equal(t, "rid", messageModality(bare))
}

// TestBuildSapientTrack_RIDByteIdenticalToGolden is the live-path regression
// gate: a RID message must project byte-identically to the pre-adsb golden
// (the staging display consumes this stream; no drift allowed).
func TestBuildSapientTrack_RIDByteIdenticalToGolden(t *testing.T) {
	msg := loadSamples(t)[0]
	msg.NodeId = proto.String("node-restamped")
	cotOpts, err := cotOptionsFor("friendly")
	require.NoError(t, err)
	agent := &trackAgent{
		AgentID: "1", SellerEVM: "0xSELLER", PeerID: "16Uiu2PEER",
		NodeID: "node-restamped", Service: "rid",
		Protocol: sapient.ProtocolDetection, Simulated: true,
	}

	blob, err := buildSapientTagged(msg, time.Unix(1764668365, 0).UTC(), cotOpts, true, agent, "synthetic")
	require.NoError(t, err)

	golden, err := os.ReadFile("testdata/rid-track-golden.json")
	require.NoError(t, err)
	require.Equal(t, string(golden), string(blob), "rid track JSON must stay byte-identical")
}

// TestBuildSapientTrack_ADSBKindAndBlock: a jv-bridge message projects with
// kind=adsb, version 1.1.0, a populated adsb block, and NO rid / cot blocks.
func TestBuildSapientTrack_ADSBKindAndBlock(t *testing.T) {
	msgs := loadADSBSamples(t)
	msg := msgs[0] // src=A ADS-B contact, callsign AFR650, reg F-GSPI
	msg.NodeId = proto.String("jv-node-restamped")
	cotOpts, err := cotOptionsFor("friendly")
	require.NoError(t, err)
	agent := &trackAgent{
		AgentID: "2", SellerEVM: "0xJVSELLER", PeerID: "16Uiu2JV",
		NodeID: "jv-node-restamped", Service: "jetvision-adsb-sapient",
		Protocol: sapient.ProtocolDetection, Simulated: true,
	}

	blob, err := buildSapientTagged(msg, time.Unix(1764668365, 0).UTC(), cotOpts, true, agent, "live")
	require.NoError(t, err)
	tf, track := decodeSapientTagged(t, blob)

	require.Equal(t, "sapient", tf.Source)
	require.Equal(t, "track", tf.Type)

	assert.Equal(t, "sapient-track", track.Type)
	assert.Equal(t, "1.1.0", track.Version, "adsb tracks carry the v1.1.0 contract")
	assert.Equal(t, "adsb", track.Kind)
	assert.Equal(t, "jv-node-restamped", track.NodeID)
	require.NotNil(t, track.Position)
	require.NotNil(t, track.Velocity, "moving airliner carries velocity")

	require.NotNil(t, track.ADSB, "adsb block populated from adsb.* object_info")
	assert.Equal(t, "3949E8", track.ADSB.ICAO24)
	assert.Equal(t, "AFR650", track.ADSB.Callsign)
	assert.Equal(t, "F-GSPI", track.ADSB.Registration)
	assert.Equal(t, "A", track.ADSB.Source)
	assert.Equal(t, "local", track.ADSB.Provenance)
	assert.Equal(t, "A5", track.ADSB.EmitterCategory)

	assert.Nil(t, track.RID, "no rid block on an adsb track")
	assert.Nil(t, track.CoT, "CoT is rid-only in v1 — omitted for adsb")

	require.NotNil(t, track.Agent)
	assert.Equal(t, "jetvision-adsb-sapient", track.Agent.Service)
	assert.Equal(t, "live", track.FeedSource)

	// RF from signal[]: the bridge resolves 1090 MHz for src=A.
	require.NotNil(t, track.RF)
	require.NotNil(t, track.RF.FrequencyHz)
	assert.InDelta(t, 1090e6, *track.RF.FrequencyHz, 1e3)
	assert.Empty(t, track.RF.Channel, "rid.channel does not exist on adsb tracks")
}

// TestBuildSapientTrack_ADSBMLATSignalFallback: the MLAT contact has no native
// Signal (the bridge omits it) — RSSI comes from adsb.signalDbm instead.
func TestBuildSapientTrack_ADSBMLATSignalFallback(t *testing.T) {
	msgs := loadADSBSamples(t)
	mlat := msgs[1] // src=M, EI-ENE
	cotOpts, err := cotOptionsFor("unknown")
	require.NoError(t, err)

	blob, err := buildSapientTagged(mlat, time.Unix(1764668365, 0).UTC(), cotOpts, false, nil, "")
	require.NoError(t, err)
	_, track := decodeSapientTagged(t, blob)

	require.Equal(t, "adsb", track.Kind)
	require.NotNil(t, track.ADSB)
	assert.Equal(t, "M", track.ADSB.Source)
	if track.RF != nil {
		assert.Nil(t, track.RF.FrequencyHz, "MLAT has no centre frequency")
	}
	require.NotNil(t, track.ADSB.SignalDbm, "MLAT diagnostics preserved under adsb.*")
}

// TestRIDTrackHasNoADSBKeys: the rid path must not grow kind/adsb keys (the
// JSON contract is checked at the raw-key level, beyond struct omitempty).
func TestRIDTrackHasNoADSBKeys(t *testing.T) {
	msg := loadSamples(t)[0]
	cotOpts, err := cotOptionsFor("unknown")
	require.NoError(t, err)
	blob, err := buildSapientTagged(msg, time.Unix(1764668365, 0).UTC(), cotOpts, false, nil, "")
	require.NoError(t, err)
	var tf taggedFrame
	require.NoError(t, json.Unmarshal(blob, &tf))
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(tf.Frame, &raw))
	_, hasKind := raw["kind"]
	_, hasADSB := raw["adsb"]
	assert.False(t, hasKind, "rid track must not carry a kind key")
	assert.False(t, hasADSB, "rid track must not carry an adsb key")
	_, hasCot := raw["cot"]
	assert.True(t, hasCot, "rid track keeps its cot block")
}

func TestEvidenceSet_SingleFileLegacyAttach(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "rid.json")
	require.NoError(t, sapient.WriteEvidence(path, sapient.AgentEvidence{
		AgentID: "1", SellerEVM: "0xRID", PeerID: "16Uiu2RID", NodeID: "node-rid",
		Service: "rid", Protocol: sapient.ProtocolDetection, Simulated: true, FeedSource: "live",
		AgentURI: json.RawMessage(`{"services":[]}`),
	}))

	set := newEvidenceSet([]string{path}, log.New(io.Discard, "", 0))

	// Single configured path = legacy attach-to-all, whatever the node_id says
	// (the deployed single-source unit must behave byte-identically).
	agent, feedSrc := set.get("some-other-node")
	require.NotNil(t, agent)
	assert.Equal(t, "1", agent.AgentID)
	assert.Equal(t, "live", feedSrc)
	agent, _ = set.get("")
	require.NotNil(t, agent)
}

func TestEvidenceSet_RoutesByNodeID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ridPath := filepath.Join(dir, "rid.json")
	jvPath := filepath.Join(dir, "jv.json")
	require.NoError(t, sapient.WriteEvidence(ridPath, sapient.AgentEvidence{
		AgentID: "1", SellerEVM: "0xRID", PeerID: "16Uiu2RID", NodeID: "node-rid",
		Service: "rid", Protocol: sapient.ProtocolDetection, Simulated: true, FeedSource: "live",
		AgentURI: json.RawMessage(`{"services":[]}`),
	}))

	set := newEvidenceSet([]string{ridPath, jvPath}, log.New(io.Discard, "", 0))

	// Strict routing with ≥2 paths: only the matching node_id is enriched.
	agent, feedSrc := set.get("node-rid")
	require.NotNil(t, agent)
	assert.Equal(t, "rid", agent.Service)
	assert.Equal(t, "live", feedSrc)

	agent, feedSrc = set.get("node-jv")
	assert.Nil(t, agent, "jv evidence not written yet — no cross-attachment")
	assert.Empty(t, feedSrc)

	agent, _ = set.get("")
	assert.Nil(t, agent, "empty node_id never matches in strict mode")

	// The jv seller writes its evidence later (lazy retry, per-path).
	require.NoError(t, sapient.WriteEvidence(jvPath, sapient.AgentEvidence{
		AgentID: "2", SellerEVM: "0xJV", PeerID: "16Uiu2JV", NodeID: "node-jv",
		Service: "jetvision-adsb-sapient", Protocol: sapient.ProtocolDetection, Simulated: true, FeedSource: "live",
		AgentURI: json.RawMessage(`{"services":[]}`),
	}))
	agent, _ = set.get("node-jv")
	require.NotNil(t, agent)
	assert.Equal(t, "jetvision-adsb-sapient", agent.Service)

	// The rid mapping is unaffected.
	agent, _ = set.get("node-rid")
	require.NotNil(t, agent)
	assert.Equal(t, "rid", agent.Service)
}

// TestADSBSkipsLegacyAndCoT drives the run() ingest routing decision helpers:
// adsb messages must not produce a remote-id TaggedFrame nor a CoT event.
func TestADSBSkipsLegacyAndCoT(t *testing.T) {
	t.Parallel()
	adsb := loadADSBSamples(t)[0]
	rid := loadSamples(t)[0]
	assert.Equal(t, "adsb", messageModality(adsb))
	assert.Equal(t, "rid", messageModality(rid))
	// The legacy drone projection would happily mis-project an adsb report
	// (it falls back to objectId) — which is exactly why run() must gate on
	// modality, not on projection failure.
	_, err := buildTaggedFrame(adsb, time.Now().UTC())
	assert.NoError(t, err, "legacy projection succeeds mechanically — the gate must be modality")
}

// TestFidConsumer_EndToEnd_MixedModalities is the multi-source chain proof at
// the consumer level: one SAPIENT edge carrying BOTH a DroneScout RID message
// and JetVision ADS-B messages (distinct node_ids, as the multi-source buyer
// emits them), two evidence files routed per node_id. Asserts:
//   - the rich sapient-track sink carries both modalities with per-source
//     agent enrichment,
//   - the legacy remote-id sink and the CoT stream carry ONLY the rid frame
//     (an aircraft never renders as a drone),
//   - tracks are distinguishable by nodeId downstream.
func TestFidConsumer_EndToEnd_MixedModalities(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv, err := sapient.ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "tagged.jsonl")
	cotPath := filepath.Join(dir, "cot.xml")
	richPath := filepath.Join(dir, "sapient.jsonl")
	ridEvPath := filepath.Join(dir, "rid-evidence.json")
	jvEvPath := filepath.Join(dir, "jv-evidence.json")

	const (
		ridNode = "11111111-1111-1111-1111-111111111111"
		jvNode  = "22222222-2222-2222-2222-222222222222"
	)
	require.NoError(t, sapient.WriteEvidence(ridEvPath, sapient.AgentEvidence{
		AgentID: "1", SellerEVM: "0xRID", PeerID: "16Uiu2RID", NodeID: ridNode,
		Service: "rid", Protocol: sapient.ProtocolDetection, Simulated: true, FeedSource: "synthetic",
		AgentURI: json.RawMessage(`{"services":[]}`),
	}))
	require.NoError(t, sapient.WriteEvidence(jvEvPath, sapient.AgentEvidence{
		AgentID: "2", SellerEVM: "0xJV", PeerID: "16Uiu2JV", NodeID: jvNode,
		Service: "jetvision-adsb-sapient", Protocol: sapient.ProtocolDetection, Simulated: true, FeedSource: "live",
		AgentURI: json.RawMessage(`{"services":[]}`),
	}))

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{
			"--edge", srv.Addr(),
			"--output", "file:" + outPath,
			"--cot-output", "file:" + cotPath,
			"--sapient-output", "file:" + richPath,
			"--agent-evidence", ridEvPath,
			"--agent-evidence", jvEvPath,
		})
	}()

	// Re-stamp fixtures the way the sellers would.
	ridMsg := proto.Clone(loadSamples(t)[0]).(*sapientpb.SapientMessage)
	ridMsg.NodeId = proto.String(ridNode)
	adsbMsg := proto.Clone(loadADSBSamples(t)[0]).(*sapientpb.SapientMessage)
	adsbMsg.NodeId = proto.String(jvNode)

	stopPub := make(chan struct{})
	go func() {
		tk := time.NewTicker(30 * time.Millisecond)
		defer tk.Stop()
		for i := 0; ; i++ {
			select {
			case <-stopPub:
				return
			case <-tk.C:
				if i%2 == 0 {
					_ = srv.Publish(ridMsg)
				} else {
					_ = srv.Publish(adsbMsg)
				}
			}
		}
	}()

	readFile := func(p string) string { b, _ := os.ReadFile(p); return string(b) }

	require.Eventually(t, func() bool {
		rich := readFile(richPath)
		return strings.Contains(rich, `"kind":"adsb"`) &&
			strings.Contains(rich, `"nodeId":"`+ridNode+`"`) &&
			strings.Contains(rich, `"nodeId":"`+jvNode+`"`)
	}, 8*time.Second, 25*time.Millisecond, "rich sink must carry both modalities with their node_ids")

	rich := readFile(richPath)
	// Per-source agent enrichment routed by node_id (never cross-attached).
	require.Contains(t, rich, `"service":"jetvision-adsb-sapient"`)
	require.Contains(t, rich, `"icao24":"3949E8"`)
	require.Contains(t, rich, `"sellerEVM":"0xRID"`)
	require.Contains(t, rich, `"sellerEVM":"0xJV"`)
	// rid track stays v1.0.0; adsb carries v1.1.0.
	require.Contains(t, rich, `"version":"1.0.0"`)
	require.Contains(t, rich, `"version":"1.1.0"`)

	// Legacy + CoT paths are rid-only.
	legacy := readFile(outPath)
	require.Contains(t, legacy, `"droneId":"1581F8B1234567890ABC"`)
	require.NotContains(t, legacy, "F-GSPI", "aircraft must not appear in the legacy drone projection")
	require.NotContains(t, legacy, "3949E8")
	cot := readFile(cotPath)
	require.NotContains(t, cot, "F-GSPI", "CoT stays rid-only in v1")

	// Cross-attachment check: the rid agent block never lands on an adsb track
	// and vice versa (line-level assertion).
	for line := range strings.SplitSeq(strings.TrimSpace(rich), "\n") {
		if strings.Contains(line, `"kind":"adsb"`) {
			require.NotContains(t, line, `"sellerEVM":"0xRID"`, "adsb track enriched only by JV evidence")
		} else if strings.Contains(line, `"source":"sapient"`) {
			require.NotContains(t, line, `"sellerEVM":"0xJV"`, "rid track enriched only by rid evidence")
		}
	}

	close(stopPub)
	cancel()
	select {
	case e := <-done:
		require.NoError(t, e)
	case <-time.After(3 * time.Second):
		t.Fatal("consumer run did not return after cancel")
	}
}
