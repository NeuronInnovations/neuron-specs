package cotadapt

import (
	"encoding/xml"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

var update = flag.Bool("update", false, "update CoT golden files")

// fixedTime is a deterministic CoT timestamp for golden output.
var fixedTime = time.Unix(1764668365, 0).UTC()

func droneMsg(id string, withOperator bool) *sapientpb.SapientMessage {
	dr := &sapientpb.DetectionReport{
		ObjectId: proto.String("01J9ZSN0W5K3QH7Y8B4F2C6XME"),
		Id:       proto.String(id),
		LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{
			X: proto.Float64(-5.6634), Y: proto.Float64(50.1027), Z: proto.Float64(95),
			XError: proto.Float64(3), YError: proto.Float64(3), ZError: proto.Float64(10),
		}},
		Classification: []*sapientpb.DetectionReport_DetectionReportClassification{
			{Type: proto.String("UAV"), Confidence: proto.Float32(0.99)},
		},
	}
	if withOperator {
		dr.ObjectInfo = []*sapientpb.DetectionReport_TrackObjectInfo{
			{Type: proto.String("rid.operatorId"), Value: proto.String("OP-NEURON-TEST-001")},
			{Type: proto.String("rid.operatorLatDeg"), Value: proto.String("50.1027")},
			{Type: proto.String("rid.operatorLonDeg"), Value: proto.String("-5.6705")},
		}
	}
	return &sapientpb.SapientMessage{
		Timestamp: timestamppb.New(fixedTime),
		NodeId:    proto.String("9f1a-asm-node"),
		Content:   &sapientpb.SapientMessage_DetectionReport{DetectionReport: dr},
	}
}

func goldenAssert(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden %s (run: go test -run TestCoT -update)", name)
	require.Equal(t, string(want), string(got))
}

func TestCoT_DroneOnly(t *testing.T) {
	xmlb, err := ToXML(droneMsg("1581F8B1234567890ABC", false), DefaultOptions())
	require.NoError(t, err)
	// One event only — no operator.
	require.Equal(t, 1, strings.Count(string(xmlb), "<event"))
	goldenAssert(t, "drone-only.xml", xmlb)
}

func TestCoT_DroneAndOperator(t *testing.T) {
	xmlb, err := ToXML(droneMsg("1581F8B1234567890ABC", true), DefaultOptions())
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(xmlb), "<event"))
	require.Contains(t, string(xmlb), `uid="1581F8B1234567890ABC-OP"`)
	require.Contains(t, string(xmlb), `relation="p-p"`)
	goldenAssert(t, "drone-operator.xml", xmlb)
}

func TestCoT_MissingOperator_NoSecondEvent(t *testing.T) {
	// withOperator=false => exactly one event even though EmitOperator is on.
	events, err := ToCoT(droneMsg("d1", false), DefaultOptions())
	require.NoError(t, err)
	require.Len(t, events, 1)

	// OmitOperator suppresses the operator event even when present.
	opts := DefaultOptions()
	opts.OmitOperator = true
	events, err = ToCoT(droneMsg("d1", true), opts)
	require.NoError(t, err)
	require.Len(t, events, 1)
}

func TestCoT_HalfOperator_Suppressed(t *testing.T) {
	// Operator latitude present but longitude missing → the operator event must be
	// SUPPRESSED (not pinned at lon=0 / the Gulf of Guinea).
	msg := &sapientpb.SapientMessage{
		Timestamp: timestamppb.New(fixedTime),
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			Id:            proto.String("d1"),
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{X: proto.Float64(-5.6), Y: proto.Float64(50.1), Z: proto.Float64(40)}},
			ObjectInfo: []*sapientpb.DetectionReport_TrackObjectInfo{
				{Type: proto.String("rid.operatorLatDeg"), Value: proto.String("50.10")}, // no operatorLonDeg
			},
		}},
	}
	events, err := ToCoT(msg, DefaultOptions())
	require.NoError(t, err)
	require.Len(t, events, 1, "a half-populated operator (lat only) must be suppressed")
}

func TestCoT_BleSignalless(t *testing.T) {
	// Real DS240 BLE path (CL-R4): no Signal block, RSSI in rid.rssiDbm. cotadapt does
	// not read signal, so it must still produce a valid drone event.
	msg := &sapientpb.SapientMessage{
		Timestamp: timestamppb.New(fixedTime),
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			Id:            proto.String("BLEDRONE1"),
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{X: proto.Float64(-5.6), Y: proto.Float64(50.1), Z: proto.Float64(40)}},
			ObjectInfo: []*sapientpb.DetectionReport_TrackObjectInfo{
				{Type: proto.String("rid.transport"), Value: proto.String("BLE legacy")},
				{Type: proto.String("rid.rssiDbm"), Value: proto.String("-72")},
			}, // no Signal block
		}},
	}
	events, err := ToCoT(msg, DefaultOptions())
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "BLEDRONE1", events[0].UID)
}

func TestCoT_XMLEscaping(t *testing.T) {
	evil := `dr<one&"id>`
	xmlb, err := ToXML(droneMsg(evil, false), DefaultOptions())
	require.NoError(t, err)
	s := string(xmlb)
	// Raw special characters must not appear unescaped inside the attribute.
	require.NotContains(t, s, `uid="dr<one&"id>"`)
	require.Contains(t, s, "&lt;")
	require.Contains(t, s, "&amp;")
	// And it must round-trip back to the original value.
	var ev Event
	require.NoError(t, xml.Unmarshal(xmlb, &ev))
	require.Equal(t, evil, ev.UID)
}

func TestCoT_NeverHostileByDefault(t *testing.T) {
	events, err := ToCoT(droneMsg("d1", true), DefaultOptions())
	require.NoError(t, err)
	for _, e := range events {
		require.True(t, strings.HasPrefix(e.Type, "a-u-"), "default affiliation must be unknown, got %q", e.Type)
		require.NotContains(t, e.Type, "-h-", "default must never be hostile")
	}
}

func TestCoT_ConfigurableAffiliation(t *testing.T) {
	opts := Options{Affiliation: 'f'} // friend
	events, err := ToCoT(droneMsg("d1", false), opts)
	require.NoError(t, err)
	require.Equal(t, "a-f-A", events[0].Type)
}

func TestCoT_ProvenanceNodeID(t *testing.T) {
	opts := DefaultOptions()
	opts.ProvenanceNodeID = "5f3c-node-id"
	events, err := ToCoT(droneMsg("d1", false), opts)
	require.NoError(t, err)
	require.Contains(t, events[0].Detail.Remarks, "node_id=5f3c-node-id")
}

func TestCoT_ProvenanceNodeID_DefaultOmitted(t *testing.T) {
	events, err := ToCoT(droneMsg("d1", false), DefaultOptions())
	require.NoError(t, err)
	require.NotContains(t, events[0].Detail.Remarks, "node_id=", "default output unchanged (goldens stable)")
}

func TestNormalize_DefaultsAndFriendly(t *testing.T) {
	// Zero options → the library defaults (== DefaultOptions).
	require.Equal(t, DefaultOptions(), Normalize(Options{}))

	// Friendly affiliation derives the friendly type atoms.
	f := Normalize(Options{Affiliation: 'f'})
	require.Equal(t, "a-f-A", f.Type)
	require.Equal(t, "a-f-G", f.OperatorType)
	require.Equal(t, "m-g", f.How)

	// Idempotent: normalizing a normalized Options is a no-op.
	require.Equal(t, f, Normalize(f))
}

func TestCoT_Errors(t *testing.T) {
	// No DetectionReport.
	_, err := ToCoT(&sapientpb.SapientMessage{}, DefaultOptions())
	require.ErrorIs(t, err, &Error{kind: ErrNoDetection})

	// No identity.
	noID := &sapientpb.SapientMessage{Content: &sapientpb.SapientMessage_DetectionReport{
		DetectionReport: &sapientpb.DetectionReport{
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{}},
		}}}
	_, err = ToCoT(noID, DefaultOptions())
	require.ErrorIs(t, err, &Error{kind: ErrNoIdentity})

	// No location.
	noLoc := &sapientpb.SapientMessage{Content: &sapientpb.SapientMessage_DetectionReport{
		DetectionReport: &sapientpb.DetectionReport{Id: proto.String("d1")}}}
	_, err = ToCoT(noLoc, DefaultOptions())
	require.ErrorIs(t, err, &Error{kind: ErrNoLocation})
}

func TestCoT_UnknownErrorSentinel(t *testing.T) {
	// A location with no error envelope => CoT 9999999 sentinel for ce/le.
	msg := &sapientpb.SapientMessage{
		Timestamp: timestamppb.New(fixedTime),
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			Id:            proto.String("d1"),
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{X: proto.Float64(1), Y: proto.Float64(2), Z: proto.Float64(3)}},
		}},
	}
	events, err := ToCoT(msg, DefaultOptions())
	require.NoError(t, err)
	require.Equal(t, "9999999", events[0].Point.CE)
	require.Equal(t, "9999999", events[0].Point.LE)
}
