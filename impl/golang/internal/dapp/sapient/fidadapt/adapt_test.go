package fidadapt

import (
	"bufio"
	"os"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// TestToRemoteIdFrame_BleSignalless covers the real DS240 BLE path (CL-R4): no
// Signal block. fidadapt never reads signal, so a signal-less report must still
// yield a valid remote-id frame (droneId + droneIdType).
func TestToRemoteIdFrame_BleSignalless(t *testing.T) {
	msg := &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			Id:            proto.String("BLEDRONE1"),
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{X: proto.Float64(-5.6), Y: proto.Float64(50.1), Z: proto.Float64(40)}},
			ObjectInfo: []*sapientpb.DetectionReport_TrackObjectInfo{
				{Type: proto.String("rid.transport"), Value: proto.String("BLE legacy")},
				{Type: proto.String("rid.rssiDbm"), Value: proto.String("-72")},
			},
		}},
	}
	f, err := ToRemoteIdFrame(msg)
	require.NoError(t, err)
	require.Equal(t, "BLEDRONE1", f.DroneID)
	require.NotEmpty(t, f.DroneIDType)
}

// firstSample loads the first SapientMessage from the captured-from-the-real-bridge
// NDJSON sample (shared with the parent package's testdata).
func firstSample(t *testing.T) *sapientpb.SapientMessage {
	t.Helper()
	f, err := os.Open("../testdata/bridge-sample.ndjson")
	require.NoError(t, err)
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	require.True(t, sc.Scan(), "sample file has at least one line")
	msg := &sapientpb.SapientMessage{}
	require.NoError(t, protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(sc.Bytes(), msg))
	return msg
}

func TestToRemoteIdFrame_FromLiveBridgeSample(t *testing.T) {
	rif, err := ToRemoteIdFrame(firstSample(t))
	require.NoError(t, err)

	// applyDrone preconditions (cmd/fid-display): droneId + position.
	require.Equal(t, "1581F8B1234567890ABC", rif.DroneID)
	require.Equal(t, "serial", rif.DroneIDType)
	require.Equal(t, "sapient-rid", rif.Source)
	require.NotNil(t, rif.Position)
	require.InDelta(t, 50.1027, rif.Position.Lat, 1e-3)
	require.InDelta(t, -5.66, rif.Position.Lon, 0.1)
	require.Equal(t, "3D", rif.Position.Fix)

	// ENU{0,10,0} -> due north, ~10 m/s.
	require.NotNil(t, rif.Velocity)
	require.InDelta(t, 10.0, rif.Velocity.SpeedHorizontal, 0.2)
	require.InDelta(t, 0.0, rif.Velocity.Track, 1.0)

	// Operator -> pilot marker at Land's End.
	require.NotNil(t, rif.Operator, "operator marker present")
	require.Equal(t, "OP-NEURON-TEST-001", rif.Operator.ID)
	require.NotNil(t, rif.Operator.Position)
	require.InDelta(t, 50.1027, rif.Operator.Position.Lat, 1e-4)
	require.InDelta(t, -5.6705, rif.Operator.Position.Lon, 1e-4)

	// Must marshal to the canonical wire shape fid-display's frameInner consumes.
	b, err := rif.MarshalJSON()
	require.NoError(t, err)
	require.Contains(t, string(b), `"droneId":"1581F8B1234567890ABC"`)
	require.Contains(t, string(b), `"source":"sapient-rid"`)
	require.Contains(t, string(b), `"operator"`)
}

// TestToRemoteIdFrame_NoVelocityOmitted: when the bridge omits the velocity
// oneof (unknown ODID track/speed — presence-honest since bridge e165d15) the
// legacy remote-id frame must omit its velocity block too, keeping the wire
// honest (spec 006: absent optionals omitted, never zero-filled).
func TestToRemoteIdFrame_NoVelocityOmitted(t *testing.T) {
	msg := &sapientpb.SapientMessage{
		Content: &sapientpb.SapientMessage_DetectionReport{DetectionReport: &sapientpb.DetectionReport{
			Id:            proto.String("NOVEL1"),
			LocationOneof: &sapientpb.DetectionReport_Location{Location: &sapientpb.Location{X: proto.Float64(-5.6), Y: proto.Float64(50.1), Z: proto.Float64(40)}},
		}},
	}
	f, err := ToRemoteIdFrame(msg)
	require.NoError(t, err)
	require.Nil(t, f.Velocity, "no velocity oneof → no velocity block")

	b, err := f.MarshalJSON()
	require.NoError(t, err)
	require.NotContains(t, string(b), `"velocity"`)
}

func TestToRemoteIdFrame_Errors(t *testing.T) {
	_, err := ToRemoteIdFrame(&sapientpb.SapientMessage{})
	require.Error(t, err, "no DetectionReport")
}
