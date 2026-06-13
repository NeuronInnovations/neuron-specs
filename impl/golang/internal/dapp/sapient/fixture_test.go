package sapient

import "time"

// Test-only ICD fixture for the EncodeMessage unit test. The live demo pipeline
// sources real SapientMessages from the bridge (ReadBridgeFeed); this constructed
// DetectionReport exists only to exercise the icd->protobuf encoder in isolation.
const (
	fixtureULID   = "01J9ZSN0W5K3QH7Y8B4F2C6XME"
	fixtureSerial = "1581F8B1234567890ABC"
)

func fixtureDetectionReport() *DetectionReport {
	return &DetectionReport{
		ReportID:            fixtureSerial + "@fixture",
		Timestamp:           time.Unix(1717200000, 0).UTC(),
		SourceID:            "fixture-asm",
		ObjectID:            fixtureULID,
		ID:                  fixtureSerial,
		DetectionConfidence: 1.0,
		Classification:      []Classification{{Type: "UAV", Confidence: 0.99}},
		Location: &Location{
			Latitude:         50.1027,
			Longitude:        -5.6705,
			AltitudeM:        95,
			AltDatum:         "geometric",
			CoordinateSystem: "WGS84",
			Error:            &LocationError{HorizontalM: 3, VerticalM: 10},
		},
		Velocity: &ENUVelocity{EastMPS: 10, NorthMPS: 0, UpMPS: 0},
		Signal:   &Signal{Amplitude: -73, CentreFrequencyMHz: 2437},
		ObjectInfo: []ObjectInfo{
			{Type: "rid.idType", Value: "SerialNumber"},
			{Type: "rid.uasId", Value: fixtureSerial},
			{Type: "rid.uaType", Value: "Multirotor"},
			{Type: "rid.macAddress", Value: "AA:BB:CC:DD:EE:F0"},
			{Type: "rid.status", Value: "Airborne"},
			{Type: "rid.operatorLocationType", Value: "Dynamic"},
			{Type: "rid.operatorLatDeg", Value: "50.1026"},
			{Type: "rid.operatorLonDeg", Value: "-5.6706"},
			{Type: "rid.auth.authType", Value: "None"},
			{Type: "rid.auth.signaturePresent", Value: "false"},
			{Type: "rid.auth.verification", Value: "unsigned"},
		},
	}
}
