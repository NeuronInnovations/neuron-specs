package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/cotadapt"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

func TestCotOptionsFor(t *testing.T) {
	for _, want := range []struct {
		in  string
		aff byte
	}{{"unknown", 0}, {"", 0}, {"friendly", 'f'}} {
		o, err := cotOptionsFor(want.in)
		require.NoError(t, err)
		require.Equal(t, want.aff, o.Affiliation, "affiliation for %q", want.in)
	}
	_, err := cotOptionsFor("hostile")
	require.Error(t, err, "only unknown|friendly are accepted")
}

// TestCotFriendlyAndProvenance proves the friendly demo profile: events come out
// a-f-A (never the library default a-u-A) and carry the seller node_id as
// display provenance.
func TestCotFriendlyAndProvenance(t *testing.T) {
	msg := loadSamples(t)[0]
	msg.NodeId = proto.String("node-xyz")

	opts, err := cotOptionsFor("friendly")
	require.NoError(t, err)
	opts.ProvenanceNodeID = msg.GetNodeId()
	xmlb, err := cotadapt.ToXML(msg, opts)
	require.NoError(t, err)
	require.Contains(t, string(xmlb), `type="a-f-A"`)
	require.Contains(t, string(xmlb), "node_id=node-xyz")

	// Default profile (unknown, no provenance) stays a-u-A.
	du, err := cotOptionsFor("unknown")
	require.NoError(t, err)
	xmlb, err = cotadapt.ToXML(msg, du)
	require.NoError(t, err)
	require.Contains(t, string(xmlb), `type="a-u-A"`)
	require.NotContains(t, string(xmlb), "node_id=")
}

func loadSamples(t *testing.T) []*sapientpb.SapientMessage {
	t.Helper()
	f, err := os.Open("testdata/bridge-sample.ndjson")
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
	require.NotEmpty(t, msgs)
	return msgs
}

// decodeSapientTagged unmarshals a buildSapientTagged blob: envelope + inner track.
func decodeSapientTagged(t *testing.T, blob []byte) (taggedFrame, sapientTrack) {
	t.Helper()
	var tf taggedFrame
	require.NoError(t, json.Unmarshal(blob, &tf))
	var track sapientTrack
	require.NoError(t, json.Unmarshal(tf.Frame, &track))
	return tf, track
}

// TestBuildSapientTrack_FromSample proves the rich projection end-to-end from a
// live bridge capture: identity, position, velocity, classification, RID,
// RF (signal[0]), CoT (friendly demo profile), agent enrichment, wire label.
func TestBuildSapientTrack_FromSample(t *testing.T) {
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
	tf, track := decodeSapientTagged(t, blob)

	require.Equal(t, "sapient", tf.Source)
	require.Equal(t, "track", tf.Type)
	require.Empty(t, tf.SellerPeerID, "proxy stripped the Neuron envelope")

	// Identity + position (sample: id=serial, objectId=ULID, y=lat x=lon z=alt).
	require.Equal(t, "sapient-track", track.Type)
	require.Equal(t, "1.0.0", track.Version)
	require.Equal(t, "1581F8B1234567890ABC", track.UID)
	require.Equal(t, "01KT3QF9AJHS570QGVY2JJHQE7", track.ObjectID)
	require.NotEmpty(t, track.ReportID)
	require.Equal(t, "node-restamped", track.NodeID)
	require.NotNil(t, track.Position)
	require.InDelta(t, 50.1027, track.Position.Lat, 1e-9)
	require.InDelta(t, -5.6634855, track.Position.Lon, 1e-6)
	require.InDelta(t, 95, track.Position.Alt, 1e-9)

	// Velocity: north-only 10.005 m/s → track 0° (a real heading, not omitted).
	require.NotNil(t, track.Velocity)
	require.InDelta(t, 10.005, track.Velocity.SpeedMps, 1e-2)
	require.InDelta(t, 0, track.Velocity.TrackDeg, 1e-9)

	// Classification.
	require.NotNil(t, track.Classification)
	require.Equal(t, "UAV", track.Classification.Type)
	require.InDelta(t, 0.99, track.Classification.Confidence, 1e-6)

	// RID block.
	require.NotNil(t, track.RID)
	require.Equal(t, "1581F8B1234567890ABC", track.RID.Serial)
	require.Equal(t, "1581F8B1234567890ABC", track.RID.UASID)
	require.Equal(t, "SerialNumber", track.RID.IDType)
	require.Equal(t, "Multirotor", track.RID.UAType)
	require.Equal(t, "AA:BB:CC:DD:EE:F0", track.RID.MacAddress)
	require.Equal(t, "Airborne", track.RID.Status)
	require.Equal(t, "OP-NEURON-TEST-001", track.RID.OperatorID)
	require.NotNil(t, track.RID.OperatorLat)
	require.InDelta(t, 50.1027, *track.RID.OperatorLat, 1e-9)
	require.NotNil(t, track.RID.OperatorLon)
	require.InDelta(t, -5.6705, *track.RID.OperatorLon, 1e-9)

	// RF block from signal[0] (+ rid.channel / rid.transport).
	require.NotNil(t, track.RF)
	require.NotNil(t, track.RF.RssiDbm)
	require.InDelta(t, -62, *track.RF.RssiDbm, 1e-9)
	require.NotNil(t, track.RF.FrequencyHz)
	// The DSTL proto carries centre_frequency as float32 → ~64 Hz quantization
	// at 2.437 GHz. Assert within float32 precision, not exact.
	require.InDelta(t, 2437000000, *track.RF.FrequencyHz, 200)
	require.Equal(t, "6", track.RF.Channel)
	require.Equal(t, "WiFi beacon", track.RF.Transport)

	// CoT — friendly demo profile.
	require.NotNil(t, track.CoT)
	require.Equal(t, track.UID, track.CoT.UID)
	require.Equal(t, "a-f-A", track.CoT.Type)
	require.Equal(t, "m-g", track.CoT.How)
	require.Equal(t, "friendly", track.CoT.Affiliation)
	require.True(t, track.CoT.DemoProfile)

	// Agent enrichment + provenance + honest wire label.
	require.NotNil(t, track.Agent)
	require.Equal(t, "1", track.Agent.AgentID)
	require.Equal(t, "0xSELLER", track.Agent.SellerEVM)
	require.True(t, track.Agent.Simulated)
	require.Equal(t, "synthetic", track.FeedSource)
	require.Equal(t, sapient.SapientWire, track.Wire)
}

// TestBuildSapientTrack_MinimalOmitsOptionals: a bare DetectionReport (objectId
// + location only) still projects — optional blocks are omitted, not zeroed.
func TestBuildSapientTrack_MinimalOmitsOptionals(t *testing.T) {
	msg := &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			ObjectId: proto.String("01TRACKULID"),
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{
				X: proto.Float64(-5.6), Y: proto.Float64(50.1), Z: proto.Float64(10),
			}},
		}},
	}
	cotOpts, err := cotOptionsFor("unknown")
	require.NoError(t, err)

	blob, err := buildSapientTagged(msg, time.Unix(1764668365, 0).UTC(), cotOpts, false, nil, "")
	require.NoError(t, err)
	_, track := decodeSapientTagged(t, blob)

	require.Equal(t, "01TRACKULID", track.UID, "uid falls back to objectId")
	require.NotNil(t, track.Position)
	require.Nil(t, track.Velocity)
	require.Nil(t, track.Classification)
	require.Nil(t, track.RID, "empty RID block omitted")
	require.Nil(t, track.RF, "empty RF block omitted")
	require.Nil(t, track.Agent, "no evidence → no agent block")
	require.Empty(t, track.FeedSource)
	require.NotNil(t, track.CoT, "cot metadata always present")
	require.Equal(t, "a-u-A", track.CoT.Type)
	require.Equal(t, "unknown", track.CoT.Affiliation)
	require.False(t, track.CoT.DemoProfile)
}

// Velocity honesty (the FID "0 km/h" rule). Absence is covered by
// TestBuildSapientTrack_MinimalOmitsOptionals (no oneof → velocity omitted,
// never zeroed); these pin the present cases: ENU components map to
// speed+heading, and a present ZERO vector stays a genuine 0.
func TestBuildSapientTrack_VelocityFromENU(t *testing.T) {
	msg := &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			ObjectId: proto.String("01TRACKULID"),
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{
				X: proto.Float64(-5.6), Y: proto.Float64(50.1), Z: proto.Float64(10),
			}},
			VelocityOneof: &sapientpb.DetectionReport_EnuVelocity{EnuVelocity: &sapientpb.ENUVelocity{
				EastRate: proto.Float64(3), NorthRate: proto.Float64(4),
			}},
		}},
	}
	cotOpts, err := cotOptionsFor("unknown")
	require.NoError(t, err)

	blob, err := buildSapientTagged(msg, time.Unix(1764668365, 0).UTC(), cotOpts, false, nil, "")
	require.NoError(t, err)
	_, track := decodeSapientTagged(t, blob)

	require.NotNil(t, track.Velocity)
	require.InDelta(t, 5.0, track.Velocity.SpeedMps, 1e-9, "hypot(east=3, north=4)")
	require.InDelta(t, 36.8699, track.Velocity.TrackDeg, 1e-3, "atan2(east, north) degrees true")
}

func TestBuildSapientTrack_GenuineZeroVelocityStaysPresent(t *testing.T) {
	msg := &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			ObjectId: proto.String("01TRACKULID"),
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{
				X: proto.Float64(-5.6), Y: proto.Float64(50.1), Z: proto.Float64(10),
			}},
			VelocityOneof: &sapientpb.DetectionReport_EnuVelocity{EnuVelocity: &sapientpb.ENUVelocity{
				EastRate: proto.Float64(0), NorthRate: proto.Float64(0), UpRate: proto.Float64(0),
			}},
		}},
	}
	cotOpts, err := cotOptionsFor("unknown")
	require.NoError(t, err)

	blob, err := buildSapientTagged(msg, time.Unix(1764668365, 0).UTC(), cotOpts, false, nil, "")
	require.NoError(t, err)
	_, track := decodeSapientTagged(t, blob)

	require.NotNil(t, track.Velocity, "present zero vector = genuinely stationary, NOT omitted")
	require.Zero(t, track.Velocity.SpeedMps)
	require.Zero(t, track.Velocity.TrackDeg)
}

func TestBuildSapientTrack_Errors(t *testing.T) {
	cotOpts, _ := cotOptionsFor("unknown")
	// No DetectionReport.
	_, err := buildSapientTagged(&sapientpb.SapientMessage{}, time.Unix(0, 0), cotOpts, false, nil, "")
	require.Error(t, err)
	// No location.
	noLoc := &sapientpb.SapientMessage{Content: &sapientpb.SapientMessage_DetectionReport{
		DetectionReport: &sapientpb.DetectionReport{Id: proto.String("d1")}}}
	_, err = buildSapientTagged(noLoc, time.Unix(0, 0), cotOpts, false, nil, "")
	require.Error(t, err)
}

// TestEvidenceLoader_LazyRetry: the seller writes the evidence AFTER the
// consumer starts — the loader returns nothing until the file appears, then
// caches the parsed identity.
func TestEvidenceLoader_LazyRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seller.json")
	l := &evidenceLoader{path: path, logger: log.New(io.Discard, "", 0)}

	agent, feedSrc := l.get()
	require.Nil(t, agent, "no evidence yet")
	require.Empty(t, feedSrc)

	require.NoError(t, sapient.WriteEvidence(path, sapient.AgentEvidence{
		AgentID: "7", SellerEVM: "0xS", PeerID: "16Uiu2X", NodeID: "n-1",
		Service: "rid", Protocol: sapient.ProtocolDetection,
		Simulated: true, FeedSource: "synthetic",
		AgentURI: json.RawMessage(`{"services":[]}`),
	}))

	agent, feedSrc = l.get()
	require.NotNil(t, agent, "loads once the seller wrote it")
	require.Equal(t, "7", agent.AgentID)
	require.True(t, agent.Simulated)
	require.Equal(t, "synthetic", feedSrc)
}

// TestFidConsumer_RemoteIdFrame is the moved display assertion (was the buyer's
// ReverseConnect test): a sample DetectionReport projects to a remote-id
// TaggedFrame that satisfies fid-display's applyDrone contract (droneId +
// position + paired operator marker).
func TestFidConsumer_RemoteIdFrame(t *testing.T) {
	msg := loadSamples(t)[0]
	blob, err := buildTaggedFrame(msg, time.Unix(1764668365, 0).UTC())
	require.NoError(t, err)

	var tf taggedFrame
	require.NoError(t, json.Unmarshal(blob, &tf))
	require.Equal(t, "remote-id", tf.Source, "routes to fid-display applyDrone")
	require.Equal(t, "drone", tf.Type)
	require.Empty(t, tf.SellerPeerID, "Neuron-blind consumer never learns the seller PeerID")

	var inner struct {
		DroneID  string `json:"droneId"`
		Source   string `json:"source"`
		Position *struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"position"`
		Operator *struct {
			ID string `json:"id"`
		} `json:"operator"`
	}
	require.NoError(t, json.Unmarshal(tf.Frame, &inner))
	require.Equal(t, "1581F8B1234567890ABC", inner.DroneID)
	require.Equal(t, "sapient-rid", inner.Source)
	require.NotNil(t, inner.Position)
	require.InDelta(t, 50.1027, inner.Position.Lat, 1e-3)
	require.NotNil(t, inner.Operator, "operator/pilot marker present")
	require.Equal(t, "OP-NEURON-TEST-001", inner.Operator.ID)
}

// TestFidConsumer_ReceivedTapEndpoint proves the read-only verification endpoint
// end-to-end: with --sapient-received-http set, the consumer exposes the decoded
// SAPIENT payloads it receives as a JSON test projection — distinct from the
// map/FID output — while the map output keeps working. It also checks the
// endpoint leaks no secrets or host paths.
func TestFidConsumer_ReceivedTapEndpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv, err := sapient.ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	// Reserve a loopback port for the received-tap endpoint, then release it so
	// the consumer can bind it.
	pl, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	tapAddr := pl.Addr().String()
	require.NoError(t, pl.Close())

	dir := t.TempDir()
	outPath := filepath.Join(dir, "tagged.jsonl")

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{
			"--edge", srv.Addr(),
			"--output", "file:" + outPath,
			"--sapient-received-http", tapAddr,
		})
	}()

	// Live feed on a ticker (absorbs the dial/accept race).
	samples := loadSamples(t)
	stopPub := make(chan struct{})
	go func() {
		tk := time.NewTicker(30 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-stopPub:
				return
			case <-tk.C:
				_ = srv.Publish(samples[0])
			}
		}
	}()

	get := func(path string) (int, string) {
		req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+tapAddr+path, nil)
		if rerr != nil {
			return 0, ""
		}
		resp, derr := http.DefaultClient.Do(req)
		if derr != nil {
			return 0, ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	// /latest eventually carries the received DetectionReport projection.
	require.Eventually(t, func() bool {
		code, body := get("/sapient/received/latest")
		return code == http.StatusOK && strings.Contains(body, `"messageType":"DetectionReport"`)
	}, 6*time.Second, 25*time.Millisecond, "received tap must expose the decoded SAPIENT payload")

	_, latest := get("/sapient/received/latest")
	require.Contains(t, latest, `"wire":"`+sapient.SapientWire+`"`)
	require.Contains(t, latest, `"messageHash":"sha256:`)
	require.NotContains(t, latest, `"protobufBase64"`, "base64 off by default")
	for _, bad := range []string{"PRIVATE", "/Users/", "/home/", "agentURI", "mnemonic"} {
		require.NotContains(t, latest, bad, "endpoint must not leak %q", bad)
	}

	code, health := get("/sapient/received/health")
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, health, `"retainedCount"`)

	code, schema := get("/sapient/received/schema")
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, schema, "testing projection")

	// The map/FID output kept working alongside the tap.
	require.Eventually(t, func() bool {
		b, _ := os.ReadFile(outPath)
		return strings.Contains(string(b), `"source":"remote-id"`)
	}, 6*time.Second, 25*time.Millisecond, "map projection unaffected by the tap")

	close(stopPub)
	cancel()
	select {
	case e := <-done:
		require.NoError(t, e)
	case <-time.After(3 * time.Second):
		t.Fatal("consumer run did not return after cancel")
	}
}

// TestFidConsumer_EndToEnd runs the consumer against an in-process SAPIENT edge
// (FeedServer) and asserts the full wiring: it dials the edge, projects each
// DetectionReport to BOTH the remote-id TaggedFrame (fid-display) AND the CoT
// XML, with the pre-fusion affiliation (a-u-A, never hostile).
func TestFidConsumer_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv, err := sapient.ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "tagged.jsonl")
	cotPath := filepath.Join(dir, "cot.xml")

	// Consumer under test (blocks until the edge feed ends / ctx cancels).
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{
			"--edge", srv.Addr(),
			"--output", "file:" + outPath,
			"--cot-output", "file:" + cotPath,
		})
	}()

	// Live feed: publish the sample on a ticker until the test tears down — this
	// absorbs the dial/accept race (an early Publish with no client is dropped).
	samples := loadSamples(t)
	stopPub := make(chan struct{})
	go func() {
		tk := time.NewTicker(30 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-stopPub:
				return
			case <-tk.C:
				_ = srv.Publish(samples[0])
			}
		}
	}()

	readFile := func(p string) string { b, _ := os.ReadFile(p); return string(b) }

	require.Eventually(t, func() bool {
		return strings.Contains(readFile(outPath), `"source":"remote-id"`) &&
			strings.Contains(readFile(cotPath), "<event")
	}, 6*time.Second, 25*time.Millisecond, "consumer must write both the tagged frame and the CoT event")

	tagged := readFile(outPath)
	require.Contains(t, tagged, `"droneId":"1581F8B1234567890ABC"`)
	require.Contains(t, tagged, `"source":"sapient-rid"`)
	require.Contains(t, tagged, `"operator"`)

	cot := readFile(cotPath)
	require.Contains(t, cot, `uid="1581F8B1234567890ABC"`)
	require.Contains(t, cot, `type="a-u-A"`, "pre-fusion: affiliation unknown, never hostile")
	require.NotContains(t, cot, "-h-", "never hostile by default")

	close(stopPub)
	cancel()
	select {
	case e := <-done:
		require.NoError(t, e)
	case <-time.After(3 * time.Second):
		t.Fatal("consumer run did not return after cancel")
	}
}
