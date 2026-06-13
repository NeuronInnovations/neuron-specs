package sapient

import (
	"math"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// ulidRe matches a canonical 26-char Crockford-base32 ULID (object_id, FR-R-D02).
var ulidRe = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// hasObjInfo reports whether a TrackObjectInfo with the given type+value exists.
func hasObjInfo(infos []*sapientpb.DetectionReport_TrackObjectInfo, typ, val string) bool {
	for _, oi := range infos {
		if oi.GetType() == typ && oi.GetValue() == val {
			return true
		}
	}
	return false
}

func TestEncodeMessage_FixtureDetectionReport(t *testing.T) {
	const nodeID = "11112222-3333-4444-5555-666677778888" // some non-placeholder UUID

	msg := EncodeMessage(fixtureDetectionReport(), nodeID)

	// Must round-trip through protobuf — proves it is a valid SapientMessage.
	b, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	var got sapientpb.SapientMessage
	if err := proto.Unmarshal(b, &got); err != nil {
		t.Fatalf("proto.Unmarshal: %v", err)
	}

	if got.GetNodeId() != nodeID {
		t.Errorf("node_id = %q, want %q", got.GetNodeId(), nodeID)
	}
	if got.GetNodeId() == PlaceholderNodeID {
		t.Error("node_id is the bridge placeholder")
	}
	if got.GetTimestamp() == nil {
		t.Error("envelope timestamp not set")
	}

	dr := got.GetDetectionReport()
	if dr == nil {
		t.Fatal("content is not a DetectionReport")
	}

	// ULID object_id (M1 / FR-R-D02).
	if !ulidRe.MatchString(dr.GetObjectId()) {
		t.Errorf("object_id = %q, want a 26-char ULID", dr.GetObjectId())
	}
	// native id = RID serial (CL-R2).
	if dr.GetId() != fixtureSerial {
		t.Errorf("id = %q, want serial %q", dr.GetId(), fixtureSerial)
	}
	// detection_confidence ~= 1.
	if math.Abs(float64(dr.GetDetectionConfidence())-1.0) > 1e-6 {
		t.Errorf("detection_confidence = %v, want ~1", dr.GetDetectionConfidence())
	}
	// classification = one UAV @ 0.99 (M6).
	cls := dr.GetClassification()
	if len(cls) != 1 || cls[0].GetType() != "UAV" {
		t.Fatalf("classification = %+v, want one UAV", cls)
	}
	if math.Abs(float64(cls[0].GetConfidence())-0.99) > 1e-6 {
		t.Errorf("classification confidence = %v, want 0.99", cls[0].GetConfidence())
	}
	// location_oneof (mandatory v2.0): X = longitude, Y = latitude, WGS84 geometric.
	loc := dr.GetLocation()
	if loc == nil {
		t.Fatal("location_oneof not set")
	}
	if math.Abs(loc.GetY()-50.1027) > 1e-9 || math.Abs(loc.GetX()-(-5.6705)) > 1e-9 {
		t.Errorf("location x/y = %v/%v, want lon/lat -5.6705/50.1027", loc.GetX(), loc.GetY())
	}
	if loc.GetCoordinateSystem() != sapientpb.LocationCoordinateSystem_LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M {
		t.Errorf("coordinate_system = %v", loc.GetCoordinateSystem())
	}
	if loc.GetDatum() != sapientpb.LocationDatum_LOCATION_DATUM_WGS84_E {
		t.Errorf("datum = %v, want WGS84_E", loc.GetDatum())
	}
	// ENU velocity (M2).
	vel := dr.GetEnuVelocity()
	if vel == nil {
		t.Fatal("velocity_oneof (ENUVelocity) not set")
	}
	if math.Abs(vel.GetEastRate()-10) > 1e-9 {
		t.Errorf("east rate = %v, want 10", vel.GetEastRate())
	}
	// Signal centre_frequency is Hz: 2437 MHz → 2.437e9 Hz (float32, kHz tolerance).
	sig := dr.GetSignal()
	if len(sig) != 1 {
		t.Fatalf("signal len = %d, want 1", len(sig))
	}
	if math.Abs(float64(sig[0].GetCentreFrequency())-2437e6) > 1000 {
		t.Errorf("centre_frequency = %v Hz, want ~2.437e9", sig[0].GetCentreFrequency())
	}
	// rid.* extension survives into object_info.
	if !hasObjInfo(dr.GetObjectInfo(), "rid.uasId", fixtureSerial) {
		t.Error("object_info missing rid.uasId")
	}
	if !hasObjInfo(dr.GetObjectInfo(), "rid.auth.verification", "unsigned") {
		t.Error("object_info missing rid.auth.verification=unsigned")
	}
	if !hasObjInfo(dr.GetObjectInfo(), "rid.operatorLocationType", "Dynamic") {
		t.Error("object_info missing rid.operatorLocationType=Dynamic")
	}
}

func TestNodeIDFromIdentity_DeterministicNonPlaceholder(t *testing.T) {
	const evm = "0x1234567890abcdef1234567890abcdef12345678"

	a := NodeIDFromIdentity(evm)
	if b := NodeIDFromIdentity(evm); a != b {
		t.Errorf("not deterministic: %q != %q", a, b)
	}
	if a == PlaceholderNodeID {
		t.Errorf("derived node_id is the bridge placeholder: %q", a)
	}
	if _, err := uuid.Parse(a); err != nil {
		t.Errorf("node_id %q is not a valid UUID: %v", a, err)
	}
	if c := NodeIDFromIdentity("0x0000000000000000000000000000000000000001"); c == a {
		t.Error("distinct identities produced the same node_id")
	}
}
