package remoteid

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds/remoteid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 017 FR-R05 + 006 wire-format.md §2 RemoteIdFrame canonical ordering:
//   type → version → observedAt → source → droneId → droneIdType →
//     position* → velocity* → operator* → regulatorVariant*

const (
	testObservedAtNs = int64(1704067200000000000) // 2024-01-01T00:00:00Z
	testDroneID      = "MFR1234567890ABC"
)

func testObservedAt() time.Time {
	return time.Unix(0, testObservedAtNs).UTC()
}

func TestRemoteIdFrame_Canonical_Minimal(t *testing.T) {
	t.Parallel()

	f := RemoteIdFrame{
		Type:        FrameType,
		Version:     FrameVersion,
		ObservedAt:  testObservedAt(),
		Source:      "replay",
		DroneID:     testDroneID,
		DroneIDType: "serial",
	}

	got, err := json.Marshal(f)
	require.NoError(t, err)

	const want = `{"type":"remote-id-frame","version":"1.0.0","observedAt":"1704067200000000000","source":"replay","droneId":"MFR1234567890ABC","droneIdType":"serial"}`
	assert.Equal(t, want, string(got))
}

func TestRemoteIdFrame_Canonical_WithPosition(t *testing.T) {
	t.Parallel()

	f := RemoteIdFrame{
		Type:        FrameType,
		Version:     FrameVersion,
		ObservedAt:  testObservedAt(),
		Source:      "replay",
		DroneID:     testDroneID,
		DroneIDType: "serial",
		Position: &Position{
			Lat: 51.4775,
			Lon: -0.4614,
			Alt: 100.0,
			Fix: "3D",
		},
	}

	got, err := json.Marshal(f)
	require.NoError(t, err)

	const want = `{"type":"remote-id-frame","version":"1.0.0","observedAt":"1704067200000000000","source":"replay","droneId":"MFR1234567890ABC","droneIdType":"serial","position":{"lat":51.4775,"lon":-0.4614,"alt":100,"fix":"3D"}}`
	assert.Equal(t, want, string(got))
}

func TestRemoteIdFrame_Canonical_Full(t *testing.T) {
	t.Parallel()

	f := RemoteIdFrame{
		Type:        FrameType,
		Version:     FrameVersion,
		ObservedAt:  testObservedAt(),
		Source:      "dronescout-ds400",
		DroneID:     testDroneID,
		DroneIDType: "serial",
		Position: &Position{
			Lat: 51.4775,
			Lon: -0.4614,
			Alt: 100.0,
			Fix: "3D",
		},
		Velocity: &Velocity{
			SpeedHorizontal: 25.0,
			SpeedVertical:   0.0,
			Track:           90.0,
		},
		Operator: &Operator{
			IDType: "caa",
			ID:     "OP-GB-001",
		},
		RegulatorVariant: "asd-faa",
	}

	got, err := json.Marshal(f)
	require.NoError(t, err)

	const want = `{"type":"remote-id-frame","version":"1.0.0","observedAt":"1704067200000000000","source":"dronescout-ds400","droneId":"MFR1234567890ABC","droneIdType":"serial","position":{"lat":51.4775,"lon":-0.4614,"alt":100,"fix":"3D"},"velocity":{"speedHorizontal":25,"speedVertical":0,"track":90},"operator":{"idType":"caa","id":"OP-GB-001"},"regulatorVariant":"asd-faa"}`
	assert.Equal(t, want, string(got))
}

func TestRemoteIdFrame_OptionalFieldsOmittedWhenNil(t *testing.T) {
	t.Parallel()

	f := RemoteIdFrame{
		Type:        FrameType,
		Version:     FrameVersion,
		ObservedAt:  testObservedAt(),
		Source:      "replay",
		DroneID:     testDroneID,
		DroneIDType: "serial",
		// Position / Velocity / Operator / RegulatorVariant all absent.
	}

	got, err := json.Marshal(f)
	require.NoError(t, err)
	assert.NotContains(t, string(got), "position")
	assert.NotContains(t, string(got), "velocity")
	assert.NotContains(t, string(got), "operator")
	assert.NotContains(t, string(got), "regulatorVariant")
}

func TestRemoteIdFrame_RoundTrip(t *testing.T) {
	t.Parallel()

	original := RemoteIdFrame{
		Type:        FrameType,
		Version:     FrameVersion,
		ObservedAt:  testObservedAt(),
		Source:      "dronescout-ds400",
		DroneID:     testDroneID,
		DroneIDType: "serial",
		Position: &Position{
			Lat: 51.4775,
			Lon: -0.4614,
			Alt: 100.0,
			Fix: "3D",
		},
		Velocity: &Velocity{
			SpeedHorizontal: 25.0,
			SpeedVertical:   1.5,
			Track:           90.0,
		},
		Operator:         &Operator{IDType: "caa", ID: "OP-GB-001"},
		RegulatorVariant: "asd-easa",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded RemoteIdFrame
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original.Type, decoded.Type)
	assert.Equal(t, original.Version, decoded.Version)
	assert.True(t, original.ObservedAt.Equal(decoded.ObservedAt))
	assert.Equal(t, original.Source, decoded.Source)
	assert.Equal(t, original.DroneID, decoded.DroneID)
	assert.Equal(t, original.DroneIDType, decoded.DroneIDType)
	assert.Equal(t, original.RegulatorVariant, decoded.RegulatorVariant)
	require.NotNil(t, decoded.Position)
	assert.InDelta(t, original.Position.Lat, decoded.Position.Lat, 1e-9)
	assert.Equal(t, original.Position.Fix, decoded.Position.Fix)
	require.NotNil(t, decoded.Velocity)
	assert.InDelta(t, original.Velocity.SpeedHorizontal, decoded.Velocity.SpeedHorizontal, 1e-9)
	require.NotNil(t, decoded.Operator)
	assert.Equal(t, original.Operator.ID, decoded.Operator.ID)
}

func TestRemoteIdFrame_MarshalRejectsMissingMandatoryFields(t *testing.T) {
	t.Parallel()

	noID := RemoteIdFrame{Type: FrameType, Version: FrameVersion, ObservedAt: testObservedAt(), Source: "x"}
	_, err := json.Marshal(noID)
	require.Error(t, err)

	noIDType := RemoteIdFrame{Type: FrameType, Version: FrameVersion, ObservedAt: testObservedAt(), Source: "x", DroneID: "abc"}
	_, err = json.Marshal(noIDType)
	require.Error(t, err)
}

func TestRemoteIdFrame_DefaultsTypeAndVersionWhenEmpty(t *testing.T) {
	t.Parallel()

	f := RemoteIdFrame{
		ObservedAt:  testObservedAt(),
		Source:      "replay",
		DroneID:     testDroneID,
		DroneIDType: "serial",
	}
	got, err := json.Marshal(f)
	require.NoError(t, err)
	assert.Contains(t, string(got), `"type":"remote-id-frame"`)
	assert.Contains(t, string(got), `"version":"1.0.0"`)
}

func TestRemoteIdFrame_UnmarshalMalformedRejected(t *testing.T) {
	t.Parallel()

	var f RemoteIdFrame
	err := json.Unmarshal([]byte(`{"observedAt":"not-a-number"}`), &f)
	require.Error(t, err)
}

func TestFromDecoded_PreservesAllFields(t *testing.T) {
	t.Parallel()

	d := remoteid.DecodedFrame{
		Type:        FrameType,
		Version:     FrameVersion,
		ObservedAt:  testObservedAt(),
		Source:      "dronescout-ds400",
		DroneID:     testDroneID,
		DroneIDType: "serial",
		Position: &remoteid.Position{
			Lat: 51.4775,
			Lon: -0.4614,
			Alt: 100.0,
			Fix: "3D",
		},
		Velocity: &remoteid.Velocity{
			SpeedHorizontal: 25.0,
			SpeedVertical:   1.5,
			Track:           90.0,
		},
		Operator: &remoteid.Operator{
			IDType: "caa",
			ID:     "OP-GB-001",
		},
		RegulatorVariant: "asd-faa",
	}

	f := FromDecoded(d)

	assert.Equal(t, d.DroneID, f.DroneID)
	require.NotNil(t, f.Position)
	assert.InDelta(t, d.Position.Lat, f.Position.Lat, 1e-9)
	require.NotNil(t, f.Velocity)
	assert.InDelta(t, d.Velocity.Track, f.Velocity.Track, 1e-9)
	require.NotNil(t, f.Operator)
	assert.Equal(t, d.Operator.ID, f.Operator.ID)
	assert.Equal(t, "asd-faa", f.RegulatorVariant)
}

func TestFromDecoded_AppliesDefaultsForTypeAndVersion(t *testing.T) {
	t.Parallel()

	d := remoteid.DecodedFrame{
		// No Type, no Version.
		ObservedAt:  testObservedAt(),
		Source:      "replay",
		DroneID:     testDroneID,
		DroneIDType: "serial",
	}
	f := FromDecoded(d)
	assert.Equal(t, FrameType, f.Type)
	assert.Equal(t, FrameVersion, f.Version)
}
