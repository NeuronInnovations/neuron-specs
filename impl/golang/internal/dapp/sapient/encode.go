package sapient

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// EncodeMessage maps the ICD DetectionReport (the sensor-side bridge's content)
// onto the generated SAPIENT v2.0 protobuf SapientMessage envelope, stamping
// nodeID — the seller's Neuron identity (NodeIDFromIdentity) — as the mandatory
// envelope node_id. Field numbers/shape come from the DSTL bsi_flex_335_v2_0
// protos (internal/dapp/sapient/sapientpb).
//
// Ported verbatim-in-behaviour from neuron-rid-bridge wire.go::toSapientMessage.
// The single behavioural seam vs the bridge: node_id is the seller-supplied
// Neuron identity (the runtime boundary), never the bridge placeholder.
func EncodeMessage(r *DetectionReport, nodeID string) *sapientpb.SapientMessage {
	dr := &sapientpb.DetectionReport{
		ReportId:            proto.String(r.ReportID),
		ObjectId:            proto.String(r.ObjectID), // ULID (mandatory, FR-R-D02)
		DetectionConfidence: proto.Float32(float32(r.DetectionConfidence)),
	}
	if r.ID != "" {
		dr.Id = proto.String(r.ID) // native "tail number" = RID serial (CL-R2)
	}

	if r.Location != nil {
		loc := &sapientpb.Location{
			X:                proto.Float64(r.Location.Longitude), // X = longitude
			Y:                proto.Float64(r.Location.Latitude),  // Y = latitude
			Z:                proto.Float64(r.Location.AltitudeM),
			CoordinateSystem: sapientpb.LocationCoordinateSystem_LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M.Enum(),
			Datum:            sapientpb.LocationDatum_LOCATION_DATUM_WGS84_E.Enum(), // geometric/WGS84 ellipsoid
		}
		if e := r.Location.Error; e != nil {
			// The ODID horizontal accuracy is a radial bound → both x and y;
			// vertical → z. (SAPIENT has no single radial-error field.)
			if e.HorizontalM != 0 {
				loc.XError = proto.Float64(e.HorizontalM)
				loc.YError = proto.Float64(e.HorizontalM)
			}
			if e.VerticalM != 0 {
				loc.ZError = proto.Float64(e.VerticalM)
			}
		}
		dr.LocationOneof = &sapientpb.DetectionReport_Location{Location: loc}
	}

	if v := r.Velocity; v != nil {
		dr.VelocityOneof = &sapientpb.DetectionReport_EnuVelocity{
			EnuVelocity: &sapientpb.ENUVelocity{
				EastRate:  proto.Float64(v.EastMPS),
				NorthRate: proto.Float64(v.NorthMPS),
				UpRate:    proto.Float64(v.UpMPS),
			},
		}
	}

	for _, c := range r.Classification {
		dr.Classification = append(dr.Classification, &sapientpb.DetectionReport_DetectionReportClassification{
			Type:       proto.String(c.Type),
			Confidence: proto.Float32(float32(c.Confidence)),
		})
	}

	if s := r.Signal; s != nil {
		dr.Signal = append(dr.Signal, &sapientpb.DetectionReport_Signal{
			Amplitude: proto.Float32(float32(s.Amplitude)), // RSSI (dBm)
			// SAPIENT reports centre_frequency in Hz (SI; spec 017 FR-R-M08).
			// The model carries MHz (the channel-plan unit), so convert at the
			// wire boundary. NOTE: the proto field is float32, so Hz at GHz
			// magnitudes is only ~hundreds-of-Hz precise — compare with
			// tolerance in tests.
			CentreFrequency: proto.Float32(float32(s.CentreFrequencyMHz * 1e6)),
		})
	}

	// rid.* extension payload → object_info (TrackObjectInfo type/value). The
	// proto's TrackObjectInfo has no units field; units are static per key and
	// declared in the neuron.rid/1 schema (kept in the JSON projection only).
	for _, oi := range r.ObjectInfo {
		dr.ObjectInfo = append(dr.ObjectInfo, &sapientpb.DetectionReport_TrackObjectInfo{
			Type:  proto.String(oi.Type),
			Value: proto.String(oi.Value),
		})
	}

	return &sapientpb.SapientMessage{
		Timestamp: timestamppb.New(r.Timestamp),
		NodeId:    proto.String(nodeID), // seller's Neuron identity — the runtime boundary
		Content:   &sapientpb.SapientMessage_DetectionReport{DetectionReport: dr},
	}
}
