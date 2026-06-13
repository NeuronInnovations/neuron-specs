// Command sapient-fid-consumer is the 018/FID display consumer in the local
// SAPIENT Remote ID demo. It is the downstream end of the FR-S91
// Buyer-Proxy↔HLDMM SAPIENT edge: it dials the generic Buyer Proxy's SAPIENT
// feed (cmd/sapient-buyer --sapient-edge), reads SapientMessages, and — unlike
// the proxy — IS allowed to parse object_info (rid.*) to project each
// DetectionReport into a display artefact:
//
//   - a remote-id TaggedFrame over TCP JSONL to cmd/fid-display (the map), and
//   - optional Cursor-on-Target XML (a pre-fusion display projection).
//
// This is the FR-S90 split: the proxy stays vendor/Neuron-blind and forwards
// SapientMessages; all rid.*-aware adaptation lives here.
//
//	sapient-fid-consumer --edge 127.0.0.1:19193 --output tcp:127.0.0.1:19191 [--cot-output file:PATH]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/cotadapt"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/fidadapt"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/receivedtap"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/edgeapp"
)

// taggedFrame mirrors the cmd/fid-display + cmd/multistream-buyer on-the-wire
// envelope (TCP JSONL). source="remote-id" routes to fid-display's applyDrone.
// SellerPeerID is intentionally empty: the Buyer Proxy stripped the Neuron
// envelope (FR-S90/S93), so this Neuron-blind consumer never learns it; the
// field is a display label only and the drone is keyed by droneId.
type taggedFrame struct {
	Source       string          `json:"source"`
	Type         string          `json:"type"`
	SellerPeerID string          `json:"sellerPeerID"`
	ReceivedAt   time.Time       `json:"receivedAt"`
	Frame        json.RawMessage `json:"frame"`
}

// buildTaggedFrame projects a SapientMessage's DetectionReport into the
// remote-id TaggedFrame fid-display renders (a temporary display projection; the
// canonical wire stays SAPIENT protobuf upstream).
func buildTaggedFrame(msg *sapientpb.SapientMessage, receivedAt time.Time) (json.RawMessage, error) {
	rif, err := fidadapt.ToRemoteIdFrame(msg)
	if err != nil {
		return nil, err
	}
	inner, err := rif.MarshalJSON()
	if err != nil {
		return nil, err
	}
	blob, err := json.Marshal(taggedFrame{
		Source:     "remote-id",
		Type:       "drone",
		ReceivedAt: receivedAt,
		Frame:      json.RawMessage(inner),
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(blob), nil
}

// ─── SapientTrackSnapshot contract (docs/sapient-track-contract.md v1.0.0) ───
//
// The rich SAPIENT projection emitted to cmd/sapient-fid-display as
// TaggedFrame{source:"sapient", type:"track"}. Everything except uid+position
// is optional; absent blocks are omitted, never zero-filled. The display
// mirrors these field NAMES only (no package import).

type sapientTrack struct {
	Type           string         `json:"type"`    // "sapient-track"
	Version        string         `json:"version"` // "1.0.0" (rid) | "1.1.0" (adsb)
	UID            string         `json:"uid"`
	ObjectID       string         `json:"objectId,omitempty"`
	ReportID       string         `json:"reportId,omitempty"`
	NodeID         string         `json:"nodeId,omitempty"`
	Position       *trackPosition `json:"position,omitempty"`
	Velocity       *trackVelocity `json:"velocity,omitempty"`
	Classification *trackClass    `json:"classification,omitempty"`
	FeedSource     string         `json:"feedSource,omitempty"`
	Wire           string         `json:"wire"` // canonical wire label (SAPIENT protobuf)
	RID            *trackRID      `json:"rid,omitempty"`
	RF             *trackRF       `json:"rf,omitempty"`
	CoT            *trackCoT      `json:"cot,omitempty"`
	Agent          *trackAgent    `json:"agent,omitempty"`
	// v1.1.0 additions — appended LAST so the rid track JSON stays
	// byte-identical (both keys are omitted on rid tracks).
	Kind string     `json:"kind,omitempty"` // "" (rid, legacy) | "adsb"
	ADSB *trackADSB `json:"adsb,omitempty"` // adsb.* object_info projection
}

// trackADSB carries the neuron.adsb/1 object_info fields (adsb.* namespace) as
// received from the JetVision bridge — interpretation lives HERE, never in the
// modality-blind Buyer Proxy. Absent fields are omitted, never zero-filled.
type trackADSB struct {
	ICAO24          string   `json:"icao24,omitempty"`
	Callsign        string   `json:"callsign,omitempty"`
	Registration    string   `json:"registration,omitempty"`
	TypeCode        string   `json:"typeCode,omitempty"`
	Operator        string   `json:"operator,omitempty"`
	OriginICAO      string   `json:"originIcao,omitempty"`
	DestICAO        string   `json:"destIcao,omitempty"`
	Country         string   `json:"country,omitempty"`
	EmitterCategory string   `json:"emitterCategory,omitempty"`
	Squawk          string   `json:"squawk,omitempty"`
	Emergency       string   `json:"emergency,omitempty"`
	AirGround       string   `json:"airGround,omitempty"`
	Source          string   `json:"source,omitempty"`     // A/M/L/F/S/D/O single-letter feed source
	Provenance      string   `json:"provenance,omitempty"` // local | relayed | unknown
	SignalDbm       *float64 `json:"signalDbm,omitempty"`  // adsb.signalDbm (MLAT keeps RSSI here)
	BaroAltFt       *float64 `json:"baroAltFt,omitempty"`
	GeoAltFt        *float64 `json:"geoAltFt,omitempty"`
}

type trackPosition struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	Alt float64 `json:"alt"`
}

type trackVelocity struct {
	SpeedMps float64 `json:"speedMps"`
	TrackDeg float64 `json:"trackDeg"` // 0 = true north; always meaningful when the block is present
}

type trackClass struct {
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence,omitempty"`
}

type trackRID struct {
	Serial         string   `json:"serial,omitempty"`
	UASID          string   `json:"uasId,omitempty"`
	IDType         string   `json:"idType,omitempty"`
	UAType         string   `json:"uaType,omitempty"`
	MacAddress     string   `json:"macAddress,omitempty"`
	Status         string   `json:"status,omitempty"`
	OperatorID     string   `json:"operatorId,omitempty"`
	OperatorIDType string   `json:"operatorIdType,omitempty"`
	OperatorLat    *float64 `json:"operatorLat,omitempty"` // set only when BOTH lat+lon parse
	OperatorLon    *float64 `json:"operatorLon,omitempty"` // (half-populated operators are suppressed)
	OperatorAltM   *float64 `json:"operatorAltM,omitempty"`
}

type trackRF struct {
	RssiDbm     *float64 `json:"rssiDbm,omitempty"`     // signal[0].amplitude, else rid.rssiDbm (CL-R4)
	FrequencyHz *float64 `json:"frequencyHz,omitempty"` // signal[0].centreFrequency
	Channel     string   `json:"channel,omitempty"`     // rid.channel
	Transport   string   `json:"transport,omitempty"`   // rid.transport
}

type trackCoT struct {
	UID         string `json:"uid"`
	Type        string `json:"type"` // e.g. a-u-A / a-f-A
	How         string `json:"how"`  // m-g
	Affiliation string `json:"affiliation"`
	DemoProfile bool   `json:"demoProfile"` // true ⇔ affiliation came from --cot-affiliation friendly
}

type trackAgent struct {
	AgentID   string `json:"agentId"`
	SellerEVM string `json:"sellerEVM"`
	PeerID    string `json:"peerID"`
	NodeID    string `json:"nodeId,omitempty"`
	Service   string `json:"service,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	Simulated bool   `json:"simulated"` // true = in-memory registry (EIP-8004 SIM), not real on-chain
}

// indexObjectInfo flattens a report's object_info into a key→value map.
func indexObjectInfo(dr *sapientpb.DetectionReport) map[string]string {
	info := map[string]string{}
	for _, oi := range dr.GetObjectInfo() {
		if oi.GetType() != "" {
			info[oi.GetType()] = oi.GetValue()
		}
	}
	return info
}

// modalityOfInfo derives the track modality from the object_info namespaces:
// any adsb.* key means the report came from an ADS-B source (neuron.adsb/1);
// everything else — including a bare report — stays on the legacy rid path.
func modalityOfInfo(info map[string]string) string {
	for k := range info {
		if strings.HasPrefix(k, "adsb.") {
			return "adsb"
		}
	}
	return "rid"
}

// messageModality is modalityOfInfo over a SapientMessage's DetectionReport.
func messageModality(msg *sapientpb.SapientMessage) string {
	return modalityOfInfo(indexObjectInfo(msg.GetDetectionReport()))
}

// parseFloatPtr parses s into a *float64, nil when absent/invalid (omitted,
// never zero-filled).
func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

// adsbBlock projects the adsb.* object_info namespace into the display block.
func adsbBlock(info map[string]string) *trackADSB {
	b := &trackADSB{
		ICAO24:          info["adsb.icao24"],
		Callsign:        info["adsb.callsign"],
		Registration:    info["adsb.registration"],
		TypeCode:        info["adsb.typeCode"],
		Operator:        info["adsb.operator"],
		OriginICAO:      info["adsb.originIcao"],
		DestICAO:        info["adsb.destIcao"],
		Country:         info["adsb.country"],
		EmitterCategory: info["adsb.emitterCategory"],
		Squawk:          info["adsb.squawk"],
		Emergency:       info["adsb.emergency"],
		AirGround:       info["adsb.airGround"],
		Source:          info["adsb.source"],
		Provenance:      info["adsb.provenance"],
		SignalDbm:       parseFloatPtr(info["adsb.signalDbm"]),
		BaroAltFt:       parseFloatPtr(info["adsb.baroAltFt"]),
		GeoAltFt:        parseFloatPtr(info["adsb.geoAltFt"]),
	}
	return b
}

// buildSapientTagged projects a SapientMessage into the full sapient-track
// TaggedFrame blob. cotOpts is the consumer's CoT projection config (the same
// one driving --cot-output); demoProfile marks an operator-selected friendly
// affiliation; agent/feedSource come from the lazily-loaded seller evidence
// (nil/"" until available — the blocks are then omitted).
//
// Modality routing (v1.1.0): a report whose object_info carries adsb.* keys is
// an ADS-B track — it gains kind="adsb" + the adsb block, and omits the rid
// and cot blocks (CoT stays rid-only in v1). A rid report stays byte-identical
// to the v1.0.0 contract.
func buildSapientTagged(msg *sapientpb.SapientMessage, receivedAt time.Time, cotOpts cotadapt.Options, demoProfile bool, agent *trackAgent, feedSource string) (json.RawMessage, error) {
	dr := msg.GetDetectionReport()
	if dr == nil {
		return nil, errors.New("sapient track: message carries no DetectionReport")
	}
	uid := dr.GetId()
	if uid == "" {
		uid = dr.GetObjectId()
	}
	if uid == "" {
		return nil, errors.New("sapient track: DetectionReport has neither id nor object_id")
	}
	loc := dr.GetLocation()
	if loc == nil {
		return nil, errors.New("sapient track: DetectionReport has no Location to place")
	}

	track := sapientTrack{
		Type:       "sapient-track",
		Version:    "1.0.0",
		UID:        uid,
		ObjectID:   dr.GetObjectId(),
		ReportID:   dr.GetReportId(),
		NodeID:     msg.GetNodeId(),
		Position:   &trackPosition{Lat: loc.GetY(), Lon: loc.GetX(), Alt: loc.GetZ()}, // proto Y=lat, X=lon
		FeedSource: feedSource,
		Wire:       sapient.SapientWire,
		Agent:      agent,
	}

	if v := dr.GetEnuVelocity(); v != nil {
		e, n := v.GetEastRate(), v.GetNorthRate()
		deg := math.Atan2(e, n) * 180 / math.Pi // bearing from true north
		if deg < 0 {
			deg += 360
		}
		track.Velocity = &trackVelocity{SpeedMps: math.Hypot(e, n), TrackDeg: deg}
	}

	if cls := dr.GetClassification(); len(cls) > 0 {
		conf := float64(cls[0].GetConfidence())
		if conf == 0 {
			conf = float64(dr.GetDetectionConfidence())
		}
		track.Classification = &trackClass{Type: cls[0].GetType(), Confidence: conf}
	}

	info := indexObjectInfo(dr)

	// RF from signal[0] is modality-independent (the wire slot is SAPIENT).
	rf := trackRF{}
	if sig := dr.GetSignal(); len(sig) > 0 {
		if sig[0].Amplitude != nil {
			v := float64(*sig[0].Amplitude)
			rf.RssiDbm = &v
		}
		if sig[0].CentreFrequency != nil {
			v := float64(*sig[0].CentreFrequency)
			rf.FrequencyHz = &v
		}
	}

	if modalityOfInfo(info) == "adsb" {
		track.Version = "1.1.0"
		track.Kind = "adsb"
		track.ADSB = adsbBlock(info)
		// MLAT omits the native Signal; its RSSI rides in adsb.signalDbm (the
		// adsb block keeps it; RF mirrors it only when signal[] was present).
		if rf != (trackRF{}) {
			track.RF = &rf
		}
		// No rid block, no CoT block (CoT typing for aircraft is deferred).
	} else {
		rid := trackRID{
			Serial:         dr.GetId(), // native tail-number = RID serial ("" when only object_id)
			UASID:          info["rid.uasId"],
			IDType:         info["rid.idType"],
			UAType:         info["rid.uaType"],
			MacAddress:     info["rid.macAddress"],
			Status:         info["rid.status"],
			OperatorID:     info["rid.operatorId"],
			OperatorIDType: info["rid.operatorIdType"],
		}
		// Operator position: require BOTH lat+lon (mirror cotadapt's suppress rule).
		if lat, lon, ok := parseLatLon(info["rid.operatorLatDeg"], info["rid.operatorLonDeg"]); ok {
			rid.OperatorLat, rid.OperatorLon = &lat, &lon
			if alt, aerr := strconv.ParseFloat(info["rid.operatorAltM"], 64); aerr == nil {
				rid.OperatorAltM = &alt
			}
		}
		if rid != (trackRID{}) {
			track.RID = &rid
		}

		rf.Channel = info["rid.channel"]
		rf.Transport = info["rid.transport"]
		if rf.RssiDbm == nil { // CL-R4: BLE channel 0 ⇒ signal omitted, RSSI in rid.rssiDbm
			if v, err := strconv.ParseFloat(info["rid.rssiDbm"], 64); err == nil {
				rf.RssiDbm = &v
			}
		}
		if rf != (trackRF{}) {
			track.RF = &rf
		}

		norm := cotadapt.Normalize(cotOpts)
		track.CoT = &trackCoT{
			UID:         uid,
			Type:        norm.Type,
			How:         norm.How,
			Affiliation: affiliationLabel(norm.Affiliation),
			DemoProfile: demoProfile,
		}
	}

	inner, err := json.Marshal(track)
	if err != nil {
		return nil, err
	}
	blob, err := json.Marshal(taggedFrame{
		Source:     "sapient",
		Type:       "track",
		ReceivedAt: receivedAt,
		Frame:      json.RawMessage(inner),
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(blob), nil
}

func parseLatLon(latStr, lonStr string) (float64, float64, bool) {
	lat, latErr := strconv.ParseFloat(latStr, 64)
	lon, lonErr := strconv.ParseFloat(lonStr, 64)
	if latErr != nil || lonErr != nil {
		return 0, 0, false
	}
	return lat, lon, true
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func affiliationLabel(aff byte) string {
	switch aff {
	case 'u':
		return "unknown"
	case 'f':
		return "friendly"
	default:
		return string(aff)
	}
}

// evidenceLoader lazily reads the seller's agent-evidence JSON. The seller
// starts (and writes the file) AFTER this consumer in the demo, so the first
// frames may arrive before the evidence exists — retry on each frame until it
// loads, then cache.
type evidenceLoader struct {
	path   string
	logger *log.Logger

	mu         sync.Mutex
	loaded     bool
	agent      *trackAgent
	feedSource string
}

func (l *evidenceLoader) get() (*trackAgent, string) {
	if l == nil || l.path == "" {
		return nil, ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.loaded {
		return l.agent, l.feedSource
	}
	ev, err := sapient.ReadEvidence(l.path)
	if err != nil {
		return nil, "" // not written yet — retry on the next frame
	}
	l.agent = &trackAgent{
		AgentID:   ev.AgentID,
		SellerEVM: ev.SellerEVM,
		PeerID:    ev.PeerID,
		NodeID:    ev.NodeID,
		Service:   ev.Service,
		Protocol:  ev.Protocol,
		Simulated: ev.Simulated,
	}
	l.feedSource = ev.FeedSource
	l.loaded = true
	l.logger.Printf("agent evidence loaded: agentId=%s evm=%s simulated=%v feedSource=%s",
		ev.AgentID, ev.SellerEVM, ev.Simulated, ev.FeedSource)
	return l.agent, l.feedSource
}

// evidenceSet routes seller evidence per source node_id for the multi-source
// demo. Compatibility rule: with exactly ONE configured path it behaves like
// the original single-file loader — the evidence is attached to every track
// regardless of node_id (the deployed single-source unit stays byte-identical).
// With two or more paths the routing is strict: a track is enriched only by
// the evidence whose NodeID equals the message's node_id — never
// cross-attached, never guessed.
type evidenceSet struct {
	loaders []*evidenceLoader
}

func newEvidenceSet(paths []string, logger *log.Logger) *evidenceSet {
	s := &evidenceSet{}
	for _, p := range paths {
		if p == "" {
			continue
		}
		s.loaders = append(s.loaders, &evidenceLoader{path: p, logger: logger})
	}
	return s
}

// get resolves the (agent, feedSource) enrichment for a message's node_id.
func (s *evidenceSet) get(nodeID string) (*trackAgent, string) {
	if s == nil || len(s.loaders) == 0 {
		return nil, ""
	}
	if len(s.loaders) == 1 {
		return s.loaders[0].get() // legacy attach-to-all
	}
	if nodeID == "" {
		return nil, ""
	}
	for _, l := range s.loaders {
		agent, feedSrc := l.get()
		if agent != nil && agent.NodeID == nodeID {
			return agent, feedSrc
		}
	}
	return nil, ""
}

// cotSink writes Cursor-on-Target XML events, one block per DetectionReport.
type cotSink struct {
	mu sync.Mutex
	w  io.Writer
	c  io.Closer
}

// newCotSink parses a --cot-output spec: ""=disabled, "stdout", "file:PATH", or a
// bare path (treated as a file).
func newCotSink(spec string) (*cotSink, error) {
	switch spec {
	case "":
		return nil, nil
	case "stdout":
		return &cotSink{w: os.Stdout}, nil
	default:
		path := strings.TrimPrefix(spec, "file:")
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		return &cotSink{w: f, c: f}, nil
	}
}

func (s *cotSink) write(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.w.Write(b); err != nil {
		return err
	}
	_, err := s.w.Write([]byte("\n"))
	return err
}

func (s *cotSink) Close() error {
	if s == nil || s.c == nil {
		return nil
	}
	return s.c.Close()
}

// cotOptionsFor maps the --cot-affiliation flag to cotadapt.Options. The zero
// Options normalizes to the unknown ('u') affiliation inside cotadapt — the
// safe, never-hostile library default. "friendly" is a demo display choice.
func cotOptionsFor(affiliation string) (cotadapt.Options, error) {
	var o cotadapt.Options
	switch affiliation {
	case "", "unknown":
		// leave zero → normalizes to 'u'.
	case "friendly":
		o.Affiliation = 'f' // → "a-f-A" / "a-f-G"
	default:
		return cotadapt.Options{}, fmt.Errorf("--cot-affiliation %q invalid (want unknown|friendly)", affiliation)
	}
	return o, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		log.Fatalf("sapient-fid-consumer: %v", err)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("sapient-fid-consumer", flag.ContinueOnError)
	var (
		edge           = fs.String("edge", "127.0.0.1:19193", "Buyer Proxy SAPIENT edge to dial (its --sapient-edge address)")
		output         = fs.String("output", "tcp:127.0.0.1:19191", "TaggedFrame sink for fid-display: stdout | file:PATH | tcp:HOST:PORT")
		cotOutput      = fs.String("cot-output", "", "also emit Cursor-on-Target XML: stdout | file:PATH (empty disables)")
		cotAffiliation = fs.String("cot-affiliation", "unknown", "CoT affiliation for emitted events: unknown (a-u-A; safe default) | friendly (a-f-A; demo display choice)")
		cotProvenance  = fs.Bool("cot-provenance", false, "append the seller's node_id to each CoT event's remarks as display provenance")
		// Rich SAPIENT path (additive). Empty --sapient-output => disabled; the
		// remote-id projection above is emitted either way (compatibility).
		sapientOutput = fs.String("sapient-output", "", "rich sapient-track sink for cmd/sapient-fid-display: stdout | file:PATH | tcp:HOST:PORT (empty disables)")
		agentEvidence []string
	)
	fs.Func("agent-evidence", "seller agent-evidence JSON (seller --registry-out) enriching sapient tracks with EIP-8004 identity; lazily loaded (the seller may start later). REPEATABLE for multi-source: one path per seller, routed by node_id (a single path keeps the legacy attach-to-all behavior)", func(v string) error {
		agentEvidence = append(agentEvidence, v)
		return nil
	})
	var (
		// Read-only received-SAPIENT verification tap (additive, partner testing).
		// Empty --sapient-received-http disables it entirely; loopback recommended.
		receivedHTTP  = fs.String("sapient-received-http", "", "optional read-only received-SAPIENT test endpoint bind address, e.g. 127.0.0.1:19200 (empty disables; loopback only)")
		receivedProto = fs.Bool("sapient-received-protobuf", false, "retain deterministic protobuf bytes so /sapient/received/latest?protobuf=1 can return base64 (off = lower memory)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	logger := log.New(os.Stderr, "[sapient-fid-consumer] ", log.LstdFlags)

	// CoT projection options. A zero Options normalizes to the unknown ('u')
	// affiliation inside cotadapt — byte-identical to the previous
	// DefaultOptions() default, so the unflagged path is unchanged.
	cotOpts, err := cotOptionsFor(*cotAffiliation)
	if err != nil {
		return err
	}

	sink, err := edgeapp.NewTaggedJSONLSink(*output)
	if err != nil {
		return fmt.Errorf("build sink %q: %w", *output, err)
	}
	defer sink.Close()

	cot, err := newCotSink(*cotOutput)
	if err != nil {
		return fmt.Errorf("build cot sink %q: %w", *cotOutput, err)
	}
	defer cot.Close()
	if cot != nil {
		logger.Printf("CoT output ON: %s (pre-fusion projection, affiliation %s, provenance %v)", *cotOutput, *cotAffiliation, *cotProvenance)
	}

	// Seller agent-evidence set (lazy, per path). Hoisted above the
	// --sapient-output guard so BOTH the rich track sink and the
	// received-SAPIENT tap can enrich with feedSource/identity, independent of
	// which output is enabled. One path = legacy attach-to-all; several paths =
	// strict per-node_id routing (multi-source).
	evset := newEvidenceSet(agentEvidence, logger)

	// Rich SAPIENT track sink (optional second output for sapient-fid-display).
	var sapientSink edgeapp.TaggedSink
	if *sapientOutput != "" {
		sapientSink, err = edgeapp.NewTaggedJSONLSink(*sapientOutput)
		if err != nil {
			return fmt.Errorf("build sapient sink %q: %w", *sapientOutput, err)
		}
		defer sapientSink.Close()
		logger.Printf("sapient-track output ON: %s (agent evidence: %s)", *sapientOutput, orDash(strings.Join(agentEvidence, ",")))
	}

	// Optional read-only received-SAPIENT verification endpoint (partner testing).
	// Captures each decoded SapientMessage BEFORE map/FID projection; never blocks
	// the data plane. Disabled unless --sapient-received-http is set.
	var receivedStore *receivedtap.ReceivedSapientStore
	if *receivedHTTP != "" {
		receivedStore = receivedtap.NewReceivedSapientStore(receivedtap.DefaultCapacity, receivedtap.DefaultMaxSubscribers)
		ln, lerr := net.Listen("tcp", *receivedHTTP)
		if lerr != nil {
			return fmt.Errorf("listen sapient-received-http %q: %w", *receivedHTTP, lerr)
		}
		srv := &http.Server{Handler: receivedtap.Handler(receivedStore, logger), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			logger.Printf("received-SAPIENT test endpoint ON: http://%s (read-only; canonical wire stays protobuf)", ln.Addr())
			if serr := srv.Serve(ln); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
				logger.Printf("received-http server: %v", serr)
			}
		}()
		go func() {
			<-ctx.Done()
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutCtx)
		}()
	}

	logger.Printf("dialing SAPIENT edge=%s output=%s", *edge, *output)
	msgs, errc := sapient.ReadBridgeFeed(ctx, *edge)

	var n uint64
	for msg := range msgs {
		// Modality routing (v1.1.0): the legacy remote-id projection and the CoT
		// stream are rid-only — projecting an aircraft as a drone would be a
		// display lie. ADS-B reports flow through the rich sapient-track path
		// (kind=adsb) and the received tap only.
		kind := messageModality(msg)
		if kind != "adsb" {
			blob, err := buildTaggedFrame(msg, time.Now().UTC())
			if err != nil {
				logger.Printf("adapt error: %v", err)
				continue
			}
			if err := sink.Emit(ctx, blob); err != nil {
				logger.Printf("sink emit error: %v", err)
				return err
			}
			if cot != nil {
				mopts := cotOpts
				if *cotProvenance {
					mopts.ProvenanceNodeID = msg.GetNodeId()
				}
				if xmlb, cerr := cotadapt.ToXML(msg, mopts); cerr != nil {
					logger.Printf("cot project error: %v", cerr)
				} else if werr := cot.write(xmlb); werr != nil {
					logger.Printf("cot write error: %v", werr)
				}
			}
		}
		if sapientSink != nil {
			// Secondary, optional enrichment path: log-and-continue on errors so
			// a dead rich display never takes the compatibility path down with it.
			agent, feedSrc := evset.get(msg.GetNodeId())
			if blob, berr := buildSapientTagged(msg, time.Now().UTC(), cotOpts, *cotAffiliation == "friendly", agent, feedSrc); berr != nil {
				logger.Printf("sapient track build error: %v", berr)
			} else if serr := sapientSink.Emit(ctx, blob); serr != nil {
				logger.Printf("sapient sink emit error: %v", serr)
			}
		}
		if receivedStore != nil {
			// Read-only verification tap: record the decoded SapientMessage as a
			// JSON test projection. Non-blocking; never affects the map/FID path.
			agent, feedSrc := evset.get(msg.GetNodeId())
			var src *receivedtap.Source
			if agent != nil {
				src = &receivedtap.Source{
					AgentID: agent.AgentID, SellerEVM: agent.SellerEVM, PeerID: agent.PeerID,
					NodeID: agent.NodeID, Service: agent.Service, Protocol: agent.Protocol, Simulated: agent.Simulated,
				}
			}
			receivedStore.Record(receivedtap.Project(msg, time.Now().UTC(), feedSrc, src, *receivedProto))
		}
		n++
	}

	select {
	case e := <-errc:
		if e != nil && !errors.Is(e, context.Canceled) {
			logger.Printf("edge feed error after %d frames: %v", n, e)
			return e
		}
	default:
	}
	logger.Printf("done; rendered %d frames", n)
	return nil
}
