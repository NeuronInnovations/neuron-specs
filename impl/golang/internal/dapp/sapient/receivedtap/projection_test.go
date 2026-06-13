package receivedtap

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// fullDetection builds a DetectionReport SapientMessage exercising every block
// the projection extracts: identity, location, ENU velocity, classification,
// rid.* object_info, and an RF signal.
func fullDetection() *sapientpb.SapientMessage {
	f32 := func(v float32) *float32 { return &v }
	return &sapientpb.SapientMessage{
		Timestamp: timestamppb.New(time.Unix(1764668365, 0).UTC()),
		NodeId:    proto.String("node-abc"),
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			Id:       proto.String("SERIAL-123"),
			ObjectId: proto.String("01OBJULID"),
			ReportId: proto.String("01REPORTULID"),
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{
				X: proto.Float64(-5.6705), Y: proto.Float64(50.1027), Z: proto.Float64(73.4),
				XError: proto.Float64(1.5), YError: proto.Float64(2.5),
			}},
			VelocityOneof: &sapientpb.DetectionReport_EnuVelocity{EnuVelocity: &sapientpb.ENUVelocity{
				EastRate: proto.Float64(3), NorthRate: proto.Float64(4), UpRate: proto.Float64(0.5),
			}},
			Classification: []*sapientpb.DetectionReport_DetectionReportClassification{
				{Type: proto.String("UAV"), Confidence: f32(0.86)},
			},
			Signal: []*sapientpb.DetectionReport_Signal{
				{Amplitude: f32(-71), CentreFrequency: f32(2437000000)},
			},
			ObjectInfo: []*sapientpb.DetectionReport_TrackObjectInfo{
				{Type: proto.String("rid.uasId"), Value: proto.String("SERIAL-123")},
				{Type: proto.String("rid.idType"), Value: proto.String("SerialNumber")},
				{Type: proto.String("rid.uaType"), Value: proto.String("Multirotor")},
				{Type: proto.String("rid.status"), Value: proto.String("Airborne")},
				{Type: proto.String("rid.operatorId"), Value: proto.String("OP-001")},
				{Type: proto.String("rid.operatorLatDeg"), Value: proto.String("50.1027")},
				{Type: proto.String("rid.operatorLonDeg"), Value: proto.String("-5.6705")},
				{Type: proto.String("rid.channel"), Value: proto.String("6")},
				{Type: proto.String("rid.transport"), Value: proto.String("WiFi")},
			},
		}},
	}
}

func TestProject_DetectionReportFull(t *testing.T) {
	t.Parallel()
	now := time.Unix(1764668400, 0).UTC()
	p := Project(fullDetection(), now, "synthetic",
		&Source{AgentID: "1", SellerEVM: "0xSELLER", Simulated: true}, false)

	assert.Equal(t, "DetectionReport", p.MessageType)
	assert.Equal(t, "node-abc", p.NodeID)
	assert.Equal(t, "SERIAL-123", p.ID)
	assert.Equal(t, "01OBJULID", p.ObjectID)
	assert.Equal(t, "01REPORTULID", p.ReportID)
	assert.Equal(t, now, p.ReceivedAt)
	assert.Equal(t, sapient.SapientWire, p.Wire)
	require.NotNil(t, p.MessageTimestamp)
	assert.Equal(t, time.Unix(1764668365, 0).UTC(), *p.MessageTimestamp)

	require.NotNil(t, p.Position)
	assert.InDelta(t, 50.1027, p.Position.Lat, 1e-9)
	assert.InDelta(t, -5.6705, p.Position.Lon, 1e-9)
	assert.InDelta(t, 73.4, p.Position.Alt, 1e-9)
	require.NotNil(t, p.Position.LatError)
	assert.InDelta(t, 2.5, *p.Position.LatError, 1e-9) // proto y_error → latError
	require.NotNil(t, p.Position.LonError)
	assert.InDelta(t, 1.5, *p.Position.LonError, 1e-9) // proto x_error → lonError
	assert.Nil(t, p.Position.AltError, "no z_error → omitted, not zeroed")

	require.NotNil(t, p.Velocity)
	assert.InDelta(t, 5.0, p.Velocity.SpeedMps, 1e-9) // hypot(3,4)
	assert.InDelta(t, 36.8699, p.Velocity.TrackDeg, 1e-3)
	assert.InDelta(t, 3, p.Velocity.EastRate, 1e-9)
	assert.InDelta(t, 4, p.Velocity.NorthRate, 1e-9)
	assert.InDelta(t, 0.5, p.Velocity.UpRate, 1e-9)

	require.NotNil(t, p.Classification)
	assert.Equal(t, "UAV", p.Classification.Type)
	assert.InDelta(t, 0.86, p.Classification.Confidence, 1e-6)

	require.NotNil(t, p.RID)
	assert.Equal(t, "SERIAL-123", p.RID.Serial)
	assert.Equal(t, "SERIAL-123", p.RID.UASID)
	assert.Equal(t, "SerialNumber", p.RID.IDType)
	assert.Equal(t, "Multirotor", p.RID.UAType)
	assert.Equal(t, "Airborne", p.RID.Status)
	assert.Equal(t, "OP-001", p.RID.OperatorID)
	require.NotNil(t, p.RID.OperatorLat)
	assert.InDelta(t, 50.1027, *p.RID.OperatorLat, 1e-9)
	require.NotNil(t, p.RID.OperatorLon)
	assert.InDelta(t, -5.6705, *p.RID.OperatorLon, 1e-9)

	require.NotNil(t, p.RF)
	require.NotNil(t, p.RF.RssiDbm)
	assert.InDelta(t, -71, *p.RF.RssiDbm, 1e-6)
	require.NotNil(t, p.RF.FrequencyHz)
	assert.InDelta(t, 2437000000, *p.RF.FrequencyHz, 200) // float32 quantization
	assert.Equal(t, "6", p.RF.Channel)
	assert.Equal(t, "WiFi", p.RF.Transport)

	assert.Equal(t, "synthetic", p.FeedSource)
	require.NotNil(t, p.Source)
	assert.Equal(t, "1", p.Source.AgentID)
	assert.True(t, p.Source.Simulated)

	assert.True(t, strings.HasPrefix(p.MessageHash, "sha256:"))
	assert.Empty(t, p.ProtobufBase64, "base64 off unless requested")
}

func TestProject_HashIsDeterministic(t *testing.T) {
	t.Parallel()
	msg := fullDetection()
	a := Project(msg, time.Unix(1, 0), "", nil, false)
	b := Project(msg, time.Unix(2, 0), "", nil, false)
	assert.Equal(t, a.MessageHash, b.MessageHash, "hash depends only on the message, not receivedAt")
	assert.True(t, strings.HasPrefix(a.MessageHash, "sha256:"))
}

func TestProject_StatusReportDegenerate(t *testing.T) {
	t.Parallel()
	msg := &sapientpb.SapientMessage{
		NodeId:  proto.String("node-status"),
		Content: &sapientpb.SapientMessage_StatusReport{StatusReport: &sapientpb.StatusReport{}},
	}
	p := Project(msg, time.Unix(10, 0), "", nil, false)

	assert.Equal(t, "StatusReport", p.MessageType)
	assert.Equal(t, "node-status", p.NodeID)
	assert.Nil(t, p.Position)
	assert.Nil(t, p.Velocity)
	assert.Nil(t, p.Classification)
	assert.Nil(t, p.RID)
	assert.Nil(t, p.RF)
	assert.True(t, strings.HasPrefix(p.MessageHash, "sha256:"))
}

func TestProject_NoLocationOmitsPosition(t *testing.T) {
	t.Parallel()
	msg := &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			Id: proto.String("d1"),
			Classification: []*sapientpb.DetectionReport_DetectionReportClassification{
				{Type: proto.String("UAV")},
			},
			ObjectInfo: []*sapientpb.DetectionReport_TrackObjectInfo{
				{Type: proto.String("rid.uasId"), Value: proto.String("d1")},
			},
		}},
	}
	p := Project(msg, time.Unix(10, 0), "", nil, false)

	assert.Nil(t, p.Position, "no Location → position omitted, never invented")
	assert.Nil(t, p.Velocity)
	require.NotNil(t, p.Classification, "other blocks still project")
	require.NotNil(t, p.RID)
	assert.Equal(t, "d1", p.RID.UASID)
}

func TestProject_AbsentSignalOmitsRF(t *testing.T) {
	t.Parallel()
	// No Signal and no rid.rssiDbm → RF omitted.
	bare := &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			ObjectId: proto.String("o1"),
		}},
	}
	assert.Nil(t, Project(bare, time.Unix(0, 0), "", nil, false).RF)

	// A present amplitude → RF.RssiDbm set (pointer presence, not getter zero).
	f32 := func(v float32) *float32 { return &v }
	withSig := &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			ObjectId: proto.String("o1"),
			Signal:   []*sapientpb.DetectionReport_Signal{{Amplitude: f32(0)}}, // genuine 0 dBm
		}},
	}
	rf := Project(withSig, time.Unix(0, 0), "", nil, false).RF
	require.NotNil(t, rf)
	require.NotNil(t, rf.RssiDbm, "a present zero amplitude is reported, not dropped")
	assert.Zero(t, *rf.RssiDbm)
}

func TestProject_ProtobufOptInRoundTrips(t *testing.T) {
	t.Parallel()
	msg := fullDetection()

	off := Project(msg, time.Unix(0, 0), "", nil, false)
	assert.Empty(t, off.ProtobufBase64)

	on := Project(msg, time.Unix(0, 0), "", nil, true)
	require.NotEmpty(t, on.ProtobufBase64)

	raw, err := base64.StdEncoding.DecodeString(on.ProtobufBase64)
	require.NoError(t, err)
	var got sapientpb.SapientMessage
	require.NoError(t, proto.Unmarshal(raw, &got))
	assert.True(t, proto.Equal(msg, &got), "base64 round-trips to an equal message")
}

func TestProject_NilFeedSourceAndSourceRenderSafely(t *testing.T) {
	t.Parallel()
	p := Project(fullDetection(), time.Unix(0, 0), "", nil, false)
	assert.Empty(t, p.FeedSource)
	assert.Nil(t, p.Source)
}

func TestMessageType(t *testing.T) {
	t.Parallel()
	cases := map[string]*sapientpb.SapientMessage{
		"DetectionReport": {Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{}}},
		"StatusReport":    {Content: &sapientpb.SapientMessage_StatusReport{StatusReport: &sapientpb.StatusReport{}}},
		"Task":            {Content: &sapientpb.SapientMessage_Task{Task: &sapientpb.Task{}}},
		"Alert":           {Content: &sapientpb.SapientMessage_Alert{Alert: &sapientpb.Alert{}}},
		"unknown":         {},
	}
	for want, msg := range cases {
		assert.Equal(t, want, messageType(msg))
	}
}
