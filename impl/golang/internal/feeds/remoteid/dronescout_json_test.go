package remoteid

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixture-driven parse tests. Each test reads one of the synthetic-shape
// fixtures under testdata/dronescout/ and asserts the parser produces
// the expected SensorMessage structure.
//
// The fixtures are synthetic-shape (not from a real sensor) but match
// the documented BlueMark MQTT envelope. When a real captured payload
// arrives, the fixtures get replaced — the parser logic does not.

const (
	fixtureDataSingle     = "testdata/dronescout/data-bt5-single.json"
	fixtureDataAggregated = "testdata/dronescout/data-bt5-aggregated.json"
	fixtureStatus         = "testdata/dronescout/status.json"
	fixtureLocation       = "testdata/dronescout/location.json"

	syntheticSensorID = "sensor-synthetic-001"
	syntheticUASdata1 = "U1lOVEgtVUFTREFUQS1ESDIzNDAxMjMtMDAwMQ=="
	syntheticUASdata2 = "U1lOVEgtVUFTREFUQS1ESDIzNDAxMjMtMDAwMg=="
)

func loadFixtureBytes(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return data
}

func TestParseSensorPayload_DataBT5Single(t *testing.T) {
	payload := loadFixtureBytes(t, fixtureDataSingle)
	msgs, err := ParseSensorPayload(payload, "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	m := msgs[0]
	if m.Kind != SensorMessageData {
		t.Errorf("Kind = %q, want %q", m.Kind, SensorMessageData)
	}
	if m.Protocol != 1.0 {
		t.Errorf("Protocol = %v, want 1.0", m.Protocol)
	}
	if m.Data == nil {
		t.Fatal("Data sub-struct is nil")
	}
	if got, want := m.Data.SensorID, syntheticSensorID; got != want {
		t.Errorf("SensorID = %q, want %q", got, want)
	}
	if got, want := m.Data.RSSI, -72; got != want {
		t.Errorf("RSSI = %d, want %d", got, want)
	}
	if got, want := m.Data.Channel, 37; got != want {
		t.Errorf("Channel = %d, want %d", got, want)
	}
	if got, want := m.Data.Timestamp, int64(1763035200000); got != want {
		t.Errorf("Timestamp = %d, want %d", got, want)
	}
	if got, want := m.Data.MACAddress, "AA:BB:CC:DD:EE:F1"; got != want {
		t.Errorf("MACAddress = %q, want %q", got, want)
	}
	if got, want := m.Data.Type, "bt5"; got != want {
		t.Errorf("Type = %q, want %q", got, want)
	}
	if got, want := m.Data.UASdataB64, syntheticUASdata1; got != want {
		t.Errorf("UASdataB64 = %q, want %q", got, want)
	}
}

func TestParseSensorPayload_DataBT5Aggregated(t *testing.T) {
	payload := loadFixtureBytes(t, fixtureDataAggregated)
	msgs, err := ParseSensorPayload(payload, "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	for i, m := range msgs {
		if m.Kind != SensorMessageData {
			t.Errorf("msg[%d].Kind = %q, want %q", i, m.Kind, SensorMessageData)
		}
		if m.Data == nil {
			t.Fatalf("msg[%d].Data is nil", i)
		}
	}
	if got, want := msgs[0].Data.UASdataB64, syntheticUASdata1; got != want {
		t.Errorf("msg[0].UASdataB64 = %q, want %q", got, want)
	}
	if got, want := msgs[1].Data.UASdataB64, syntheticUASdata2; got != want {
		t.Errorf("msg[1].UASdataB64 = %q, want %q", got, want)
	}
	if got, want := msgs[0].Data.MACAddress, "AA:BB:CC:DD:EE:F1"; got != want {
		t.Errorf("msg[0].MACAddress = %q, want %q", got, want)
	}
	if got, want := msgs[1].Data.MACAddress, "11:22:33:44:55:66"; got != want {
		t.Errorf("msg[1].MACAddress = %q, want %q", got, want)
	}
}

func TestParseSensorPayload_Status(t *testing.T) {
	payload := loadFixtureBytes(t, fixtureStatus)
	msgs, err := ParseSensorPayload(payload, "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	m := msgs[0]
	if m.Kind != SensorMessageStatus {
		t.Errorf("Kind = %q, want %q", m.Kind, SensorMessageStatus)
	}
	if m.Status == nil {
		t.Fatal("Status sub-struct is nil")
	}
	if got, want := m.Status.FirmwareVersion, "20260427-1257"; got != want {
		t.Errorf("FirmwareVersion = %q, want %q", got, want)
	}
	if got, want := m.Status.Model, "ds240"; got != want {
		t.Errorf("Model = %q, want %q", got, want)
	}
	if got, want := m.Status.Status, "ok"; got != want {
		t.Errorf("Status = %q, want %q", got, want)
	}
}

func TestParseSensorPayload_Location(t *testing.T) {
	payload := loadFixtureBytes(t, fixtureLocation)
	msgs, err := ParseSensorPayload(payload, "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	m := msgs[0]
	if m.Kind != SensorMessageLocation {
		t.Errorf("Kind = %q, want %q", m.Kind, SensorMessageLocation)
	}
	if m.Location == nil {
		t.Fatal("Location sub-struct is nil")
	}
	if got, want := m.Location.Latitude, 51.4775; got != want {
		t.Errorf("Latitude = %v, want %v", got, want)
	}
	if got, want := m.Location.Longitude, -0.4614; got != want {
		t.Errorf("Longitude = %v, want %v", got, want)
	}
	if got, want := m.Location.AltitudeMSL, 42.5; got != want {
		t.Errorf("AltitudeMSL = %v, want %v", got, want)
	}
	if got, want := m.Location.Satellites, 12; got != want {
		t.Errorf("Satellites = %d, want %d", got, want)
	}
	if got, want := m.Location.HDOP, 0.9; got != want {
		t.Errorf("HDOP = %v, want %v", got, want)
	}
}

func TestParseSensorPayload_EmptyPayload(t *testing.T) {
	msgs, err := ParseSensorPayload([]byte(""), "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("empty payload should produce 0 messages, got %d", len(msgs))
	}
}

func TestParseSensorPayload_WhitespaceOnlyPayload(t *testing.T) {
	msgs, err := ParseSensorPayload([]byte("   \n  \x00\n  "), "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("whitespace-only payload should produce 0 messages, got %d", len(msgs))
	}
}

func TestParseSensorPayload_TrimsTrailingNewlineAndNull(t *testing.T) {
	// Same single-data payload as fixture but with explicit trailing \n\x00.
	payload := []byte(`{"protocol":1.0,"data":{"sensor ID":"x","RSSI":-50,"channel":1,"timestamp":1000,"MAC address":"00:00:00:00:00:00","type":"bt5","UASdata":"AA=="}}` + "\n\x00")
	msgs, err := ParseSensorPayload(payload, "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
}

func TestParseSensorPayload_EmptyCompressionDefaultsToNone(t *testing.T) {
	payload := loadFixtureBytes(t, fixtureDataSingle)
	if _, err := ParseSensorPayload(payload, ""); err != nil {
		t.Errorf("empty compression should be treated as \"none\"; got %v", err)
	}
}

func TestParseSensorPayload_RejectsLZMA(t *testing.T) {
	payload := loadFixtureBytes(t, fixtureDataSingle)
	_, err := ParseSensorPayload(payload, "lzma")
	if err == nil {
		t.Fatal("expected LZMA-not-supported error; got nil")
	}
	if !errors.Is(err, ErrDroneScoutLZMANotYetSupported) {
		t.Errorf("expected ErrDroneScoutLZMANotYetSupported, got %v", err)
	}
}

func TestParseSensorPayload_RejectsUnknownCompression(t *testing.T) {
	payload := loadFixtureBytes(t, fixtureDataSingle)
	_, err := ParseSensorPayload(payload, "zstd")
	if err == nil {
		t.Fatal("expected unknown-compression error; got nil")
	}
	if !errors.Is(err, ErrDroneScoutInvalidCompression) {
		t.Errorf("expected ErrDroneScoutInvalidCompression, got %v", err)
	}
}

func TestParseSensorPayload_RejectsMalformedJSON(t *testing.T) {
	_, err := ParseSensorPayload([]byte("{not json}"), "none")
	if err == nil {
		t.Fatal("expected malformed-JSON error; got nil")
	}
	if !errors.Is(err, ErrDroneScoutMalformedJSON) {
		t.Errorf("expected ErrDroneScoutMalformedJSON, got %v", err)
	}
}

func TestParseSensorPayload_SkipsUnknownKind(t *testing.T) {
	// An envelope with only an unknown sub-object — parser should
	// skip silently (forward-compat with vendor adding new kinds).
	payload := []byte(`{"protocol":1.0,"some-future-kind":{"foo":"bar"}}`)
	msgs, err := ParseSensorPayload(payload, "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("unknown-only envelope should be skipped silently, got %d msgs", len(msgs))
	}
}

func TestParseSensorPayload_RecognisesAircraftKind(t *testing.T) {
	payload := []byte(`{"protocol":1.0,"aircraft":{"ICAO":"ABC123"}}`)
	msgs, err := ParseSensorPayload(payload, "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if msgs[0].Kind != SensorMessageAircraft {
		t.Errorf("Kind = %q, want %q", msgs[0].Kind, SensorMessageAircraft)
	}
}

func TestParseSensorPayload_StringContainingBraceBoundaryDoesNotSplit(t *testing.T) {
	// A `}{` substring inside a string literal must not cause a false
	// split. This is the safety improvement over the BlueMark
	// reference's naive `}{` strings.Split.
	payload := []byte(`{"protocol":1.0,"data":{"sensor ID":"a}{b","RSSI":0,"channel":0,"timestamp":0,"MAC address":"","type":"","UASdata":""}}`)
	msgs, err := ParseSensorPayload(payload, "none")
	if err != nil {
		t.Fatalf("ParseSensorPayload: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1 (must not split inside string literal)", len(msgs))
	}
	if got, want := msgs[0].Data.SensorID, "a}{b"; got != want {
		t.Errorf("SensorID = %q, want %q", got, want)
	}
}

func TestNormalizeToDecodedFrame_DataKindProducesFrame(t *testing.T) {
	msg := SensorMessage{
		Kind: SensorMessageData,
		Data: &SensorDataMessage{
			SensorID:   "sensor-x",
			Timestamp:  1763035200000,
			MACAddress: "AA:BB:CC:DD:EE:F1",
			Type:       "bt5",
			UASdataB64: syntheticUASdata1,
		},
	}
	df, err := NormalizeToDecodedFrame(msg, "dronescout-ds240")
	if err != nil {
		t.Fatalf("NormalizeToDecodedFrame: %v", err)
	}
	if got, want := df.Type, "remote-id-frame"; got != want {
		t.Errorf("Type = %q, want %q", got, want)
	}
	if got, want := df.Version, "1.0.0"; got != want {
		t.Errorf("Version = %q, want %q", got, want)
	}
	if got, want := df.Source, "dronescout-ds240"; got != want {
		t.Errorf("Source = %q, want %q", got, want)
	}
	if got, want := df.DroneID, syntheticUASdata1; got != want {
		t.Errorf("DroneID = %q, want %q (UASdata must be preserved verbatim)", got, want)
	}
	if got, want := df.DroneIDType, DroneIDTypeUASdataBase64; got != want {
		t.Errorf("DroneIDType = %q, want %q", got, want)
	}
	if df.Position != nil {
		t.Errorf("Position should be nil in Stage B (no invented position from absent data); got %+v", df.Position)
	}
	if df.Velocity != nil {
		t.Errorf("Velocity should be nil in Stage B; got %+v", df.Velocity)
	}
	if df.Operator != nil {
		t.Errorf("Operator should be nil in Stage B; got %+v", df.Operator)
	}
	if df.RegulatorVariant != "" {
		t.Errorf("RegulatorVariant should be empty in Stage B; got %q", df.RegulatorVariant)
	}
}

func TestNormalizeToDecodedFrame_ObservedAtFromTimestamp(t *testing.T) {
	const tsMs = int64(1763035200000)
	msg := SensorMessage{
		Kind: SensorMessageData,
		Data: &SensorDataMessage{
			Timestamp:  tsMs,
			UASdataB64: syntheticUASdata1,
		},
	}
	df, err := NormalizeToDecodedFrame(msg, "")
	if err != nil {
		t.Fatalf("NormalizeToDecodedFrame: %v", err)
	}
	want := time.Unix(0, tsMs*int64(time.Millisecond)).UTC()
	if !df.ObservedAt.Equal(want) {
		t.Errorf("ObservedAt = %v, want %v", df.ObservedAt, want)
	}
}

func TestNormalizeToDecodedFrame_DefaultsSourceTag(t *testing.T) {
	msg := SensorMessage{
		Kind: SensorMessageData,
		Data: &SensorDataMessage{
			Timestamp:  1,
			UASdataB64: syntheticUASdata1,
		},
	}
	df, err := NormalizeToDecodedFrame(msg, "")
	if err != nil {
		t.Fatalf("NormalizeToDecodedFrame: %v", err)
	}
	if got, want := df.Source, "dronescout-ds240"; got != want {
		t.Errorf("default Source = %q, want %q", got, want)
	}
}

func TestNormalizeToDecodedFrame_RejectsNonDataKind(t *testing.T) {
	cases := []struct {
		name string
		msg  SensorMessage
	}{
		{"status", SensorMessage{Kind: SensorMessageStatus, Status: &SensorStatusMessage{}}},
		{"location", SensorMessage{Kind: SensorMessageLocation, Location: &SensorLocationMessage{}}},
		{"aircraft", SensorMessage{Kind: SensorMessageAircraft}},
		{"mobile-network", SensorMessage{Kind: SensorMessageMobileNetwork}},
		{"empty-kind", SensorMessage{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeToDecodedFrame(tc.msg, "src")
			if err == nil {
				t.Fatal("expected ErrDroneScoutNotDataKind; got nil")
			}
			if !errors.Is(err, ErrDroneScoutNotDataKind) {
				t.Errorf("expected ErrDroneScoutNotDataKind, got %v", err)
			}
		})
	}
}

func TestNormalizeToDecodedFrame_RejectsMissingUASdata(t *testing.T) {
	msg := SensorMessage{
		Kind: SensorMessageData,
		Data: &SensorDataMessage{
			Timestamp: 1,
			// UASdataB64 deliberately empty
		},
	}
	_, err := NormalizeToDecodedFrame(msg, "")
	if err == nil {
		t.Fatal("expected ErrDroneScoutMissingUASdata; got nil")
	}
	if !errors.Is(err, ErrDroneScoutMissingUASdata) {
		t.Errorf("expected ErrDroneScoutMissingUASdata, got %v", err)
	}
}

func TestNormalizeToDecodedFrame_RejectsNilDataPointer(t *testing.T) {
	msg := SensorMessage{Kind: SensorMessageData, Data: nil}
	_, err := NormalizeToDecodedFrame(msg, "")
	if err == nil {
		t.Fatal("expected ErrDroneScoutNotDataKind; got nil")
	}
	if !errors.Is(err, ErrDroneScoutNotDataKind) {
		t.Errorf("expected ErrDroneScoutNotDataKind, got %v", err)
	}
}

// TestDroneScoutFixture_DataMessagesProduceCanonicalDecodedFrames is
// the end-to-end Stage-B fixture test: parse every data-kind fixture,
// normalize each, assert the resulting DecodedFrames carry the
// FR-R05-canonical shape (with UASdata preserved as base64 per Stage-B
// scope).
func TestDroneScoutFixture_DataMessagesProduceCanonicalDecodedFrames(t *testing.T) {
	fixtures := []string{fixtureDataSingle, fixtureDataAggregated}
	for _, f := range fixtures {
		t.Run(filepath.Base(f), func(t *testing.T) {
			payload := loadFixtureBytes(t, f)
			msgs, err := ParseSensorPayload(payload, "none")
			if err != nil {
				t.Fatalf("ParseSensorPayload: %v", err)
			}
			if len(msgs) == 0 {
				t.Fatal("expected at least one message; got 0")
			}
			for i, m := range msgs {
				if m.Kind != SensorMessageData {
					continue // non-data are not the subject of this test
				}
				df, err := NormalizeToDecodedFrame(m, "dronescout-ds240")
				if err != nil {
					t.Fatalf("msg[%d]: NormalizeToDecodedFrame: %v", i, err)
				}
				if df.Type != "remote-id-frame" {
					t.Errorf("msg[%d].Type = %q, want %q", i, df.Type, "remote-id-frame")
				}
				if df.Version != "1.0.0" {
					t.Errorf("msg[%d].Version = %q, want %q", i, df.Version, "1.0.0")
				}
				if df.Source == "" {
					t.Errorf("msg[%d].Source is empty", i)
				}
				if df.DroneID == "" {
					t.Errorf("msg[%d].DroneID is empty", i)
				}
				if df.DroneIDType != DroneIDTypeUASdataBase64 {
					t.Errorf("msg[%d].DroneIDType = %q, want %q", i, df.DroneIDType, DroneIDTypeUASdataBase64)
				}
				if df.ObservedAt.IsZero() {
					t.Errorf("msg[%d].ObservedAt is zero", i)
				}
				// DroneID must equal the raw UASdata base64 string;
				// this is the explicit "not-yet-decoded" framing.
				if !strings.HasPrefix(df.DroneID, "U1lOVEgt") {
					t.Errorf("msg[%d].DroneID = %q, want synthetic-UASdata prefix", i, df.DroneID)
				}
			}
		})
	}
}
