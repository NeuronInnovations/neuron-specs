package receivedtap

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// Projection is the partner-facing JSON testing projection of one received
// SAPIENT message. It is a lossy view of the decoded *sapientpb.SapientMessage;
// the canonical wire stays SAPIENT protobuf upstream. Optional blocks are
// pointers with omitempty: a nil block means the field was absent on the wire
// (intentionally omitted) — never zero-filled.
type Projection struct {
	ReceivedAt       time.Time       `json:"receivedAt"`                 // wall-clock at the tap (consumer receive time)
	MessageTimestamp *time.Time      `json:"messageTimestamp,omitempty"` // the message's own embedded timestamp, if any
	MessageType      string          `json:"messageType"`                // "DetectionReport" | "StatusReport" | … | "unknown"
	NodeID           string          `json:"nodeId,omitempty"`           // sending node UUID (SapientMessage.node_id)
	ObjectID         string          `json:"objectId,omitempty"`         // DetectionReport.object_id (ULID)
	ID               string          `json:"id,omitempty"`               // DetectionReport.id — native RID serial / tail-number
	ReportID         string          `json:"reportId,omitempty"`         // DetectionReport.report_id (ULID)
	Position         *Position       `json:"position,omitempty"`
	Velocity         *Velocity       `json:"velocity,omitempty"`
	Classification   *Classification `json:"classification,omitempty"`
	RID              *RID            `json:"rid,omitempty"`
	RF               *RF             `json:"rf,omitempty"`
	FeedSource       string          `json:"feedSource,omitempty"`     // live|replay|synthetic|placeholder; only with --agent-evidence
	Source           *Source         `json:"source,omitempty"`         // public seller/card identity only
	Wire             string          `json:"wire"`                     // canonical wire label (sapient.SapientWire)
	MessageHash      string          `json:"messageHash"`              // "sha256:<hex>" of the deterministic re-encoding
	ProtobufBase64   string          `json:"protobufBase64,omitempty"` // opt-in; deterministic re-encoding, NOT original wire bytes
}

// Position mirrors the SAPIENT Location (proto X=lon, Y=lat, Z=alt). Errors are
// pointers so an absent error is omitted rather than reported as 0.
type Position struct {
	Lat      float64  `json:"lat"`
	Lon      float64  `json:"lon"`
	Alt      float64  `json:"alt"`
	LatError *float64 `json:"latError,omitempty"` // proto y_error
	LonError *float64 `json:"lonError,omitempty"` // proto x_error
	AltError *float64 `json:"altError,omitempty"` // proto z_error
}

// Velocity carries both the derived speed/track and the raw ENU components, so a
// partner can re-derive either. The block is present only when the report
// carried an ENU velocity (a present zero vector = genuinely stationary).
type Velocity struct {
	SpeedMps  float64 `json:"speedMps"`  // hypot(east, north)
	TrackDeg  float64 `json:"trackDeg"`  // atan2(east, north) in degrees true; 0 = north
	EastRate  float64 `json:"eastRate"`  // m/s
	NorthRate float64 `json:"northRate"` // m/s
	UpRate    float64 `json:"upRate"`    // m/s (vertical)
}

type Classification struct {
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence,omitempty"`
}

// RID carries the Remote ID object_info fields (rid.* namespace) as received.
type RID struct {
	Serial         string   `json:"serial,omitempty"`
	UASID          string   `json:"uasId,omitempty"`
	IDType         string   `json:"idType,omitempty"`
	UAType         string   `json:"uaType,omitempty"`
	MacAddress     string   `json:"macAddress,omitempty"`
	Status         string   `json:"status,omitempty"`
	OperatorID     string   `json:"operatorId,omitempty"`
	OperatorIDType string   `json:"operatorIdType,omitempty"`
	OperatorLat    *float64 `json:"operatorLat,omitempty"` // set only when BOTH lat+lon parse
	OperatorLon    *float64 `json:"operatorLon,omitempty"`
	OperatorAltM   *float64 `json:"operatorAltM,omitempty"`
}

type RF struct {
	RssiDbm     *float64 `json:"rssiDbm,omitempty"`     // signal[0].amplitude, else rid.rssiDbm (CL-R4)
	FrequencyHz *float64 `json:"frequencyHz,omitempty"` // signal[0].centreFrequency
	Channel     string   `json:"channel,omitempty"`     // rid.channel
	Transport   string   `json:"transport,omitempty"`   // rid.transport
}

// Source carries ONLY public identity fields from the seller agent-evidence
// record. It deliberately excludes registry address, transaction hash, agentURI,
// and chainId — those are not received-payload and not part of a test
// projection. No secret is ever carried here.
type Source struct {
	AgentID   string `json:"agentId,omitempty"`
	SellerEVM string `json:"sellerEVM,omitempty"`
	PeerID    string `json:"peerID,omitempty"`
	NodeID    string `json:"nodeId,omitempty"`
	Service   string `json:"service,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	Simulated bool   `json:"simulated"`
}

// deterministicMarshal produces a stable byte encoding of the decoded message so
// messageHash is reproducible across calls. Note: ReadBridgeFeed already applied
// DiscardUnknown on receipt, so this is a canonical re-encoding of the DECODED
// message, not the original wire bytes — see /sapient/received/schema.
var deterministicMarshal = proto.MarshalOptions{Deterministic: true}

// Project builds a Projection from a received SapientMessage. receivedAt is the
// tap wall-clock; feedSource and source come from the optional seller evidence
// (both may be "" / nil — the fields are then omitted). includeProtobuf controls
// whether the deterministic protobuf base64 is attached (default off, gated by
// the process flag at the call site).
func Project(msg *sapientpb.SapientMessage, receivedAt time.Time, feedSource string, source *Source, includeProtobuf bool) Projection {
	p := Projection{
		ReceivedAt:  receivedAt,
		MessageType: messageType(msg),
		NodeID:      msg.GetNodeId(),
		FeedSource:  feedSource,
		Source:      source,
		Wire:        sapient.SapientWire,
	}
	if ts := msg.GetTimestamp(); ts != nil {
		mt := ts.AsTime()
		p.MessageTimestamp = &mt
	}

	if dr := msg.GetDetectionReport(); dr != nil {
		applyDetection(&p, dr)
	}

	if raw, err := deterministicMarshal.Marshal(msg); err == nil {
		sum := sha256.Sum256(raw)
		p.MessageHash = "sha256:" + hex.EncodeToString(sum[:])
		if includeProtobuf {
			p.ProtobufBase64 = base64.StdEncoding.EncodeToString(raw)
		}
	} else {
		p.MessageHash = "sha256:unavailable"
	}
	return p
}

// applyDetection fills the DetectionReport-derived blocks. It mirrors the field
// extraction in cmd/sapient-fid-consumer's buildSapientTagged: proto Y=lat,
// X=lon, Z=alt; ENU velocity → speed+track; classification[0] with a
// detection-confidence fallback; object_info → rid.* map; RF read via the proto
// pointer fields so an absent signal is omitted, never reported as 0.
func applyDetection(p *Projection, dr *sapientpb.DetectionReport) {
	p.ObjectID = dr.GetObjectId()
	p.ID = dr.GetId()
	p.ReportID = dr.GetReportId()

	if loc := dr.GetLocation(); loc != nil {
		pos := &Position{Lat: loc.GetY(), Lon: loc.GetX(), Alt: loc.GetZ()}
		if loc.YError != nil {
			v := loc.GetYError()
			pos.LatError = &v
		}
		if loc.XError != nil {
			v := loc.GetXError()
			pos.LonError = &v
		}
		if loc.ZError != nil {
			v := loc.GetZError()
			pos.AltError = &v
		}
		p.Position = pos
	}

	if v := dr.GetEnuVelocity(); v != nil {
		e, n, u := v.GetEastRate(), v.GetNorthRate(), v.GetUpRate()
		p.Velocity = &Velocity{
			SpeedMps:  math.Hypot(e, n),
			TrackDeg:  trackDeg(e, n),
			EastRate:  e,
			NorthRate: n,
			UpRate:    u,
		}
	}

	if cls := dr.GetClassification(); len(cls) > 0 {
		conf := float64(cls[0].GetConfidence())
		if conf == 0 {
			conf = float64(dr.GetDetectionConfidence())
		}
		p.Classification = &Classification{Type: cls[0].GetType(), Confidence: conf}
	}

	info := indexObjectInfo(dr.GetObjectInfo())

	rid := RID{
		Serial:         dr.GetId(),
		UASID:          info["rid.uasId"],
		IDType:         info["rid.idType"],
		UAType:         info["rid.uaType"],
		MacAddress:     info["rid.macAddress"],
		Status:         info["rid.status"],
		OperatorID:     info["rid.operatorId"],
		OperatorIDType: info["rid.operatorIdType"],
	}
	// Operator position: require BOTH lat+lon (suppress half-populated operators).
	if lat, lon, ok := parseLatLon(info["rid.operatorLatDeg"], info["rid.operatorLonDeg"]); ok {
		rid.OperatorLat, rid.OperatorLon = &lat, &lon
		if alt, ok := parseFloat(info["rid.operatorAltM"]); ok {
			rid.OperatorAltM = &alt
		}
	}
	if rid != (RID{}) {
		p.RID = &rid
	}

	rf := RF{Channel: info["rid.channel"], Transport: info["rid.transport"]}
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
	if rf.RssiDbm == nil { // CL-R4: BLE channel 0 ⇒ signal omitted, RSSI in rid.rssiDbm
		if v, ok := parseFloat(info["rid.rssiDbm"]); ok {
			rf.RssiDbm = &v
		}
	}
	if rf != (RF{}) {
		p.RF = &rf
	}
}

// messageType returns a stable label for the SapientMessage content oneof.
func messageType(msg *sapientpb.SapientMessage) string {
	switch msg.GetContent().(type) {
	case *sapientpb.SapientMessage_Registration:
		return "Registration"
	case *sapientpb.SapientMessage_RegistrationAck:
		return "RegistrationAck"
	case *sapientpb.SapientMessage_StatusReport:
		return "StatusReport"
	case *sapientpb.SapientMessage_DetectionReport:
		return "DetectionReport"
	case *sapientpb.SapientMessage_Task:
		return "Task"
	case *sapientpb.SapientMessage_TaskAck:
		return "TaskAck"
	case *sapientpb.SapientMessage_Alert:
		return "Alert"
	case *sapientpb.SapientMessage_AlertAck:
		return "AlertAck"
	case *sapientpb.SapientMessage_Error:
		return "Error"
	default:
		return "unknown"
	}
}

func indexObjectInfo(infos []*sapientpb.DetectionReport_TrackObjectInfo) map[string]string {
	m := make(map[string]string, len(infos))
	for _, oi := range infos {
		if oi.GetType() != "" {
			m[oi.GetType()] = oi.GetValue()
		}
	}
	return m
}

// trackDeg converts an East/North vector to a true-track heading in degrees
// [0,360); 0°/360° = due north, 90° = due east.
func trackDeg(e, n float64) float64 {
	if e == 0 && n == 0 {
		return 0
	}
	deg := math.Atan2(e, n) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}

func parseLatLon(latStr, lonStr string) (float64, float64, bool) {
	lat, ok1 := parseFloat(latStr)
	lon, ok2 := parseFloat(lonStr)
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return lat, lon, true
}

func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
