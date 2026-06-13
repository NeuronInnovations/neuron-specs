package health

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeartbeatPayloadFields(t *testing.T) {
	t.Run("HeartbeatPayload has correct field types", func(t *testing.T) {
		p := HeartbeatPayload{
			Type:                  "heartbeat",
			Version:               "1.0.0",
			NextHeartbeatDeadline: uint64(1700000000),
			Role:                  RoleBuyer,
			Capabilities:          nil,
			Location:              nil,
			Peers:                 nil,
		}
		assert.Equal(t, "heartbeat", p.Type)
		assert.Equal(t, "1.0.0", p.Version)
		assert.Equal(t, uint64(1700000000), p.NextHeartbeatDeadline)
		assert.Equal(t, RoleBuyer, p.Role)
		assert.Nil(t, p.Capabilities)
		assert.Nil(t, p.Location)
		assert.Nil(t, p.Peers)
	})

	t.Run("all fields use json dash tag for custom serialization", func(t *testing.T) {
		// Verify that default json.Marshal on HeartbeatPayload uses our custom MarshalJSON,
		// not struct tags. Since all fields are json:"-", the custom method must handle everything.
		p := BuildHeartbeatPayload(1700000000, RoleBuyer)
		data, err := json.Marshal(p)
		require.NoError(t, err)
		// Must contain the fields from our custom serializer
		assert.Contains(t, string(data), `"type"`)
		assert.Contains(t, string(data), `"version"`)
		assert.Contains(t, string(data), `"nextHeartbeatDeadline"`)
		assert.Contains(t, string(data), `"role"`)
	})
}

func TestCapabilitiesStruct(t *testing.T) {
	t.Run("Capabilities has correct JSON tags", func(t *testing.T) {
		c := Capabilities{
			NATReachability: true,
			NATType:         NATEndpointIndependent,
			Protocols:       []ProtocolID{"/adsb/v1"},
		}
		data, err := json.Marshal(c)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"natReachability"`)
		assert.Contains(t, string(data), `"natType"`)
		assert.Contains(t, string(data), `"protocols"`)
	})
}

func TestLocationStruct(t *testing.T) {
	t.Run("Location has correct JSON tags", func(t *testing.T) {
		alt := 100.5
		l := Location{
			Lat: 37.7749,
			Lon: -122.4194,
			Alt: &alt,
			Fix: Fix3D,
		}
		data, err := json.Marshal(l)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"lat"`)
		assert.Contains(t, string(data), `"lon"`)
		assert.Contains(t, string(data), `"alt"`)
		assert.Contains(t, string(data), `"fix"`)
	})

	t.Run("Location omits alt when nil", func(t *testing.T) {
		l := Location{
			Lat: 37.7749,
			Lon: -122.4194,
			Fix: Fix2D,
		}
		data, err := json.Marshal(l)
		require.NoError(t, err)
		assert.NotContains(t, string(data), `"alt"`)
	})
}

func TestBuildHeartbeatPayload(t *testing.T) {
	t.Run("auto-sets type to heartbeat", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000000, RoleBuyer)
		assert.Equal(t, PayloadTypeHeartbeat, p.Type)
	})

	t.Run("auto-sets version to 1.0.0", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000000, RoleBuyer)
		assert.Equal(t, CurrentVersion, p.Version)
	})

	t.Run("sets deadline and role from arguments", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000060, RoleSeller)
		assert.Equal(t, uint64(1700000060), p.NextHeartbeatDeadline)
		assert.Equal(t, RoleSeller, p.Role)
	})

	t.Run("optional fields are nil by default", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000000, RoleBuyer)
		assert.Nil(t, p.Capabilities)
		assert.Nil(t, p.Location)
		assert.Nil(t, p.Peers)
	})

	t.Run("WithCapabilities sets capabilities", func(t *testing.T) {
		caps := &Capabilities{
			NATReachability: true,
			NATType:         NATNone,
			Protocols:       []ProtocolID{"/test/v1"},
		}
		p := BuildHeartbeatPayload(1700000000, RoleBuyer, WithCapabilities(caps))
		require.NotNil(t, p.Capabilities)
		assert.True(t, p.Capabilities.NATReachability)
		assert.Equal(t, NATNone, p.Capabilities.NATType)
	})

	t.Run("WithLocation sets location", func(t *testing.T) {
		loc := &Location{Lat: 40.0, Lon: -74.0, Fix: Fix2D}
		p := BuildHeartbeatPayload(1700000000, RoleBuyer, WithLocation(loc))
		require.NotNil(t, p.Location)
		assert.Equal(t, 40.0, p.Location.Lat)
	})

	t.Run("WithPeers sets peers", func(t *testing.T) {
		peers := []AbbreviatedAddress{"a1b2", "c3d4"}
		p := BuildHeartbeatPayload(1700000000, RoleBuyer, WithPeers(peers))
		require.Len(t, p.Peers, 2)
		assert.Equal(t, AbbreviatedAddress("a1b2"), p.Peers[0])
	})

	t.Run("multiple options can be combined", func(t *testing.T) {
		caps := &Capabilities{NATReachability: false, NATType: NATAddressDependent}
		loc := &Location{Lat: 51.5074, Lon: -0.1278, Fix: Fix3D}
		peers := []AbbreviatedAddress{"dead", "beef"}
		p := BuildHeartbeatPayload(1700000000, RoleRelay,
			WithCapabilities(caps),
			WithLocation(loc),
			WithPeers(peers),
		)
		assert.NotNil(t, p.Capabilities)
		assert.NotNil(t, p.Location)
		assert.Len(t, p.Peers, 2)
		assert.Equal(t, RoleRelay, p.Role)
	})
}

func TestDeterministicJSONSerialization(t *testing.T) {
	t.Run("serialize twice produces byte-equal output", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000060, RoleSeller)
		data1, err1 := json.Marshal(p)
		require.NoError(t, err1)
		data2, err2 := json.Marshal(p)
		require.NoError(t, err2)
		assert.Equal(t, data1, data2, "two serializations of the same payload must be byte-equal")
	})

	t.Run("serialize with all optional fields is deterministic", func(t *testing.T) {
		alt := 500.0
		p := BuildHeartbeatPayload(1700000060, RoleValidator,
			WithCapabilities(&Capabilities{
				NATReachability: true,
				NATType:         NATEndpointIndependent,
				Protocols:       []ProtocolID{"/adsb/v1", "/mesh/v2"},
			}),
			WithLocation(&Location{
				Lat: 37.7749,
				Lon: -122.4194,
				Alt: &alt,
				Fix: Fix3D,
			}),
			WithPeers([]AbbreviatedAddress{"a1b2", "c3d4", "e5f6"}),
		)
		data1, err1 := json.Marshal(p)
		require.NoError(t, err1)
		data2, err2 := json.Marshal(p)
		require.NoError(t, err2)
		assert.Equal(t, data1, data2, "full payload must serialize deterministically")
	})

	t.Run("canonical field order verification", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000060, RoleBuyer,
			WithCapabilities(&Capabilities{
				NATReachability: true,
				NATType:         NATNone,
				Protocols:       []ProtocolID{"/test/v1"},
			}),
			WithLocation(&Location{Lat: 40.0, Lon: -74.0, Fix: Fix2D}),
			WithPeers([]AbbreviatedAddress{"dead"}),
		)
		data, err := json.Marshal(p)
		require.NoError(t, err)
		jsonStr := string(data)

		// Verify field order: type < version < nextHeartbeatDeadline < role < capabilities < location < peers
		typeIdx := strings.Index(jsonStr, `"type"`)
		versionIdx := strings.Index(jsonStr, `"version"`)
		deadlineIdx := strings.Index(jsonStr, `"nextHeartbeatDeadline"`)
		roleIdx := strings.Index(jsonStr, `"role"`)
		capIdx := strings.Index(jsonStr, `"capabilities"`)
		locIdx := strings.Index(jsonStr, `"location"`)
		peersIdx := strings.Index(jsonStr, `"peers"`)

		assert.Greater(t, versionIdx, typeIdx, "version must come after type")
		assert.Greater(t, deadlineIdx, versionIdx, "deadline must come after version")
		assert.Greater(t, roleIdx, deadlineIdx, "role must come after deadline")
		assert.Greater(t, capIdx, roleIdx, "capabilities must come after role")
		assert.Greater(t, locIdx, capIdx, "location must come after capabilities")
		assert.Greater(t, peersIdx, locIdx, "peers must come after location")
	})

	t.Run("canonical field order with mandatory fields only", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000060, RoleBuyer)
		data, err := json.Marshal(p)
		require.NoError(t, err)
		jsonStr := string(data)

		typeIdx := strings.Index(jsonStr, `"type"`)
		versionIdx := strings.Index(jsonStr, `"version"`)
		deadlineIdx := strings.Index(jsonStr, `"nextHeartbeatDeadline"`)
		roleIdx := strings.Index(jsonStr, `"role"`)

		assert.Greater(t, versionIdx, typeIdx)
		assert.Greater(t, deadlineIdx, versionIdx)
		assert.Greater(t, roleIdx, deadlineIdx)

		// Optional fields must NOT appear
		assert.NotContains(t, jsonStr, `"capabilities"`)
		assert.NotContains(t, jsonStr, `"location"`)
		assert.NotContains(t, jsonStr, `"peers"`)
	})
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	t.Run("mandatory fields only round-trip", func(t *testing.T) {
		original := BuildHeartbeatPayload(1700000060, RoleSeller)
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored HeartbeatPayload
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Equal(t, original.Type, restored.Type)
		assert.Equal(t, original.Version, restored.Version)
		assert.Equal(t, original.NextHeartbeatDeadline, restored.NextHeartbeatDeadline)
		assert.Equal(t, original.Role, restored.Role)
		assert.Nil(t, restored.Capabilities)
		assert.Nil(t, restored.Location)
		assert.Nil(t, restored.Peers)
	})

	t.Run("all fields round-trip", func(t *testing.T) {
		alt := 250.5
		original := BuildHeartbeatPayload(1700000120, RoleValidator,
			WithCapabilities(&Capabilities{
				NATReachability: true,
				NATType:         NATEndpointIndependent,
				Protocols:       []ProtocolID{"/adsb/v1"},
			}),
			WithLocation(&Location{
				Lat: 37.7749,
				Lon: -122.4194,
				Alt: &alt,
				Fix: Fix3D,
			}),
			WithPeers([]AbbreviatedAddress{"a1b2", "c3d4"}),
		)
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored HeartbeatPayload
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Equal(t, original.Type, restored.Type)
		assert.Equal(t, original.Version, restored.Version)
		assert.Equal(t, original.NextHeartbeatDeadline, restored.NextHeartbeatDeadline)
		assert.Equal(t, original.Role, restored.Role)
		require.NotNil(t, restored.Capabilities)
		assert.Equal(t, original.Capabilities.NATReachability, restored.Capabilities.NATReachability)
		assert.Equal(t, original.Capabilities.NATType, restored.Capabilities.NATType)
		assert.Equal(t, original.Capabilities.Protocols, restored.Capabilities.Protocols)
		require.NotNil(t, restored.Location)
		assert.Equal(t, original.Location.Lat, restored.Location.Lat)
		assert.Equal(t, original.Location.Lon, restored.Location.Lon)
		require.NotNil(t, restored.Location.Alt)
		assert.Equal(t, *original.Location.Alt, *restored.Location.Alt)
		assert.Equal(t, original.Location.Fix, restored.Location.Fix)
		assert.Equal(t, original.Peers, restored.Peers)
	})

	t.Run("serialize-deserialize-reserialize produces identical bytes", func(t *testing.T) {
		alt := 100.0
		original := BuildHeartbeatPayload(1700000060, RoleRelay,
			WithCapabilities(&Capabilities{
				NATReachability: false,
				NATType:         NATAddressDependent,
				Protocols:       []ProtocolID{"/relay/v1", "/mesh/v1"},
			}),
			WithLocation(&Location{Lat: 51.5074, Lon: -0.1278, Alt: &alt, Fix: Fix3D}),
			WithPeers([]AbbreviatedAddress{"dead", "beef", "cafe"}),
		)
		data1, err := json.Marshal(original)
		require.NoError(t, err)

		var restored HeartbeatPayload
		err = json.Unmarshal(data1, &restored)
		require.NoError(t, err)

		data2, err := json.Marshal(restored)
		require.NoError(t, err)

		assert.Equal(t, data1, data2, "round-trip must produce identical bytes")
	})
}

// --- Phase 9 (T078): Deterministic Serialization Round-Trip with Multiple Variants ---

// TestDeterministicSerializationRoundTrip validates that serialize -> deserialize ->
// re-serialize produces byte-equal output for multiple HeartbeatPayload variants.
// This ensures the custom MarshalJSON/UnmarshalJSON pair is fully deterministic
// and lossless across all field combinations.
func TestDeterministicSerializationRoundTrip(t *testing.T) {
	alt := 500.0
	variants := []struct {
		name    string
		payload HeartbeatPayload
	}{
		{
			name:    "mandatory-only",
			payload: BuildHeartbeatPayload(1700000060, RoleBuyer),
		},
		{
			name: "with capabilities",
			payload: BuildHeartbeatPayload(1700000060, RoleSeller,
				WithCapabilities(&Capabilities{
					NATReachability: true,
					NATType:         NATEndpointIndependent,
					Protocols:       []ProtocolID{"/adsb/v1", "/mesh/v2"},
				}),
			),
		},
		{
			name: "with location (no alt)",
			payload: BuildHeartbeatPayload(1700000060, RoleRelay,
				WithLocation(&Location{Lat: 37.7749, Lon: -122.4194, Fix: Fix2D}),
			),
		},
		{
			name: "with location (with alt)",
			payload: BuildHeartbeatPayload(1700000060, RoleValidator,
				WithLocation(&Location{Lat: 51.5074, Lon: -0.1278, Alt: &alt, Fix: Fix3D}),
			),
		},
		{
			name: "with peers",
			payload: BuildHeartbeatPayload(1700000060, RoleBuyer,
				WithPeers([]AbbreviatedAddress{"dead", "beef", "cafe"}),
			),
		},
		{
			name: "all optional fields",
			payload: BuildHeartbeatPayload(1700000060, RoleSeller,
				WithCapabilities(&Capabilities{
					NATReachability: false,
					NATType:         NATAddressAndPortDependent,
					Protocols:       []ProtocolID{"/relay/v1"},
				}),
				WithLocation(&Location{Lat: -33.8688, Lon: 151.2093, Alt: &alt, Fix: Fix3D}),
				WithPeers([]AbbreviatedAddress{"a1b2", "c3d4", "e5f6"}),
			),
		},
		{
			name: "shutdown sentinel",
			payload: func() HeartbeatPayload {
				p := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
				p.NextHeartbeatDeadline = ShutdownSentinel
				return p
			}(),
		},
	}

	for _, tt := range variants {
		t.Run("T078: round-trip: "+tt.name, func(t *testing.T) {
			// First serialization.
			data1, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			// Deserialization.
			var restored HeartbeatPayload
			err = json.Unmarshal(data1, &restored)
			require.NoError(t, err)

			// Second serialization (from restored).
			data2, err := json.Marshal(restored)
			require.NoError(t, err)

			// Third serialization (re-serialize to verify stability).
			var restored2 HeartbeatPayload
			err = json.Unmarshal(data2, &restored2)
			require.NoError(t, err)
			data3, err := json.Marshal(restored2)
			require.NoError(t, err)

			// Assert byte-equality: data1 == data2 == data3.
			assert.Equal(t, data1, data2,
				"first and second serialization must be byte-equal for variant %q", tt.name)
			assert.Equal(t, data1, data3,
				"first and third serialization must be byte-equal for variant %q", tt.name)
		})
	}
}

func TestMandatoryFieldsBudget(t *testing.T) {
	t.Run("mandatory fields only payload is under 256 bytes", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000060, RoleBuyer)
		data, err := json.Marshal(p)
		require.NoError(t, err)
		assert.Less(t, len(data), MandatoryFieldsBudget,
			"FR-H29: mandatory-only payload must be under %d bytes, got %d bytes: %s",
			MandatoryFieldsBudget, len(data), string(data))
	})

	t.Run("mandatory fields with longest role under budget", func(t *testing.T) {
		// Use "validator" as the longest role name
		p := BuildHeartbeatPayload(1700000060, RoleValidator)
		data, err := json.Marshal(p)
		require.NoError(t, err)
		assert.Less(t, len(data), MandatoryFieldsBudget,
			"mandatory-only payload with longest role must be under %d bytes", MandatoryFieldsBudget)
	})

	t.Run("mandatory fields with max deadline value under budget", func(t *testing.T) {
		// Use a very large timestamp value
		p := BuildHeartbeatPayload(9999999999, RoleValidator)
		data, err := json.Marshal(p)
		require.NoError(t, err)
		assert.Less(t, len(data), MandatoryFieldsBudget,
			"mandatory-only payload with max deadline must be under %d bytes", MandatoryFieldsBudget)
	})
}

func TestAllOptionalFieldsSerialization(t *testing.T) {
	t.Run("all optional fields are serialized when present", func(t *testing.T) {
		alt := 1000.0
		p := BuildHeartbeatPayload(1700000060, RoleRelay,
			WithCapabilities(&Capabilities{
				NATReachability: true,
				NATType:         NATEndpointIndependent,
				Protocols:       []ProtocolID{"/adsb/v1"},
			}),
			WithLocation(&Location{
				Lat: 37.7749,
				Lon: -122.4194,
				Alt: &alt,
				Fix: Fix3D,
			}),
			WithPeers([]AbbreviatedAddress{"a1b2", "c3d4"}),
		)
		data, err := json.Marshal(p)
		require.NoError(t, err)
		jsonStr := string(data)

		assert.Contains(t, jsonStr, `"capabilities"`)
		assert.Contains(t, jsonStr, `"natReachability":true`)
		assert.Contains(t, jsonStr, `"natType":"endpoint-independent"`)
		assert.Contains(t, jsonStr, `"protocols":["/adsb/v1"]`)
		assert.Contains(t, jsonStr, `"location"`)
		assert.Contains(t, jsonStr, `"lat":37.7749`)
		assert.Contains(t, jsonStr, `"lon":-122.4194`)
		assert.Contains(t, jsonStr, `"alt":1000`)
		assert.Contains(t, jsonStr, `"fix":"3D"`)
		assert.Contains(t, jsonStr, `"peers":["a1b2","c3d4"]`)
	})

	t.Run("empty peers slice is omitted", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000060, RoleBuyer,
			WithPeers([]AbbreviatedAddress{}),
		)
		data, err := json.Marshal(p)
		require.NoError(t, err)
		assert.NotContains(t, string(data), `"peers"`)
	})
}

func TestValidateLocation(t *testing.T) {
	t.Run("nil location is valid", func(t *testing.T) {
		err := ValidateLocation(nil)
		assert.NoError(t, err)
	})

	t.Run("valid location passes", func(t *testing.T) {
		loc := &Location{Lat: 37.7749, Lon: -122.4194, Fix: Fix2D}
		err := ValidateLocation(loc)
		assert.NoError(t, err)
	})

	t.Run("zero lat and lon is valid", func(t *testing.T) {
		loc := &Location{Lat: 0, Lon: 0, Fix: FixNone}
		err := ValidateLocation(loc)
		assert.NoError(t, err)
	})

	t.Run("boundary lat values are valid", func(t *testing.T) {
		locMin := &Location{Lat: -90, Lon: 0, Fix: FixNone}
		assert.NoError(t, ValidateLocation(locMin))
		locMax := &Location{Lat: 90, Lon: 0, Fix: FixNone}
		assert.NoError(t, ValidateLocation(locMax))
	})

	t.Run("boundary lon values are valid", func(t *testing.T) {
		locMin := &Location{Lat: 0, Lon: -180, Fix: FixNone}
		assert.NoError(t, ValidateLocation(locMin))
		locMax := &Location{Lat: 0, Lon: 180, Fix: FixNone}
		assert.NoError(t, ValidateLocation(locMax))
	})

	t.Run("lat out of range is invalid", func(t *testing.T) {
		loc := &Location{Lat: 91, Lon: 0, Fix: FixNone}
		err := ValidateLocation(loc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "latitude")

		loc2 := &Location{Lat: -91, Lon: 0, Fix: FixNone}
		err2 := ValidateLocation(loc2)
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "latitude")
	})

	t.Run("lon out of range is invalid", func(t *testing.T) {
		loc := &Location{Lat: 0, Lon: 181, Fix: FixNone}
		err := ValidateLocation(loc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "longitude")

		loc2 := &Location{Lat: 0, Lon: -181, Fix: FixNone}
		err2 := ValidateLocation(loc2)
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "longitude")
	})
}

func TestAbbreviatedAddressValidationInPeers(t *testing.T) {
	t.Run("valid peers pass validation", func(t *testing.T) {
		peers := []AbbreviatedAddress{"a1b2", "c3d4", "dead", "beef"}
		for _, p := range peers {
			assert.NoError(t, ValidateAbbreviatedAddress(p))
		}
	})

	t.Run("invalid peer address is rejected", func(t *testing.T) {
		invalidPeers := []AbbreviatedAddress{"xyz", "DEAD", "toolong", "ab"}
		for _, p := range invalidPeers {
			assert.Error(t, ValidateAbbreviatedAddress(p))
		}
	})
}

// --- Phase 7 (T060): Capabilities Field Handling ---

func TestCapabilitiesFieldSerialization(t *testing.T) {
	t.Run("T060: capabilities with all sub-fields serialized in canonical order", func(t *testing.T) {
		caps := &Capabilities{
			NATReachability: true,
			NATType:         NATEndpointIndependent,
			Protocols:       []ProtocolID{"/adsb/v1", "/mesh/v2"},
		}
		p := BuildHeartbeatPayload(1700000060, RoleBuyer, WithCapabilities(caps))
		data, err := json.Marshal(p)
		require.NoError(t, err)
		jsonStr := string(data)

		// Verify capabilities sub-fields are present.
		assert.Contains(t, jsonStr, `"natReachability":true`)
		assert.Contains(t, jsonStr, `"natType":"endpoint-independent"`)
		assert.Contains(t, jsonStr, `"protocols":["/adsb/v1","/mesh/v2"]`)

		// Verify capabilities appears after role in canonical field order.
		roleIdx := strings.Index(jsonStr, `"role"`)
		capsIdx := strings.Index(jsonStr, `"capabilities"`)
		assert.Greater(t, capsIdx, roleIdx,
			"capabilities must come after role in canonical field order")
	})

	t.Run("T060: capabilities with false NATReachability serializes correctly", func(t *testing.T) {
		caps := &Capabilities{
			NATReachability: false,
			NATType:         NATAddressAndPortDependent,
			Protocols:       []ProtocolID{},
		}
		p := BuildHeartbeatPayload(1700000060, RoleSeller, WithCapabilities(caps))
		data, err := json.Marshal(p)
		require.NoError(t, err)
		jsonStr := string(data)

		assert.Contains(t, jsonStr, `"natReachability":false`)
		assert.Contains(t, jsonStr, `"natType":"address-and-port-dependent"`)
		assert.Contains(t, jsonStr, `"protocols":[]`)
	})

	t.Run("T060: capabilities round-trips through marshal/unmarshal", func(t *testing.T) {
		caps := &Capabilities{
			NATReachability: true,
			NATType:         NATAddressDependent,
			Protocols:       []ProtocolID{"/relay/v1", "/mesh/v1", "/adsb/v1"},
		}
		original := BuildHeartbeatPayload(1700000060, RoleRelay, WithCapabilities(caps))
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored HeartbeatPayload
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)
		require.NotNil(t, restored.Capabilities)
		assert.Equal(t, caps.NATReachability, restored.Capabilities.NATReachability)
		assert.Equal(t, caps.NATType, restored.Capabilities.NATType)
		assert.Equal(t, caps.Protocols, restored.Capabilities.Protocols)
	})
}

// --- Phase 7 (T061): Location Field Handling ---

func TestLocationFieldSerialization(t *testing.T) {
	t.Run("T061: location with lat+lon only (no alt)", func(t *testing.T) {
		loc := &Location{Lat: 37.7749, Lon: -122.4194, Fix: Fix2D}
		p := BuildHeartbeatPayload(1700000060, RoleBuyer, WithLocation(loc))
		data, err := json.Marshal(p)
		require.NoError(t, err)
		jsonStr := string(data)

		assert.Contains(t, jsonStr, `"lat":37.7749`)
		assert.Contains(t, jsonStr, `"lon":-122.4194`)
		assert.Contains(t, jsonStr, `"fix":"2D"`)
		assert.NotContains(t, jsonStr, `"alt"`, "alt must be omitted when nil")
	})

	t.Run("T061: location with lat+lon+alt+fix (full)", func(t *testing.T) {
		alt := 500.5
		loc := &Location{
			Lat: 51.5074,
			Lon: -0.1278,
			Alt: &alt,
			Fix: Fix3D,
		}
		p := BuildHeartbeatPayload(1700000060, RoleBuyer, WithLocation(loc))
		data, err := json.Marshal(p)
		require.NoError(t, err)
		jsonStr := string(data)

		assert.Contains(t, jsonStr, `"lat":51.5074`)
		assert.Contains(t, jsonStr, `"lon":-0.1278`)
		assert.Contains(t, jsonStr, `"alt":500.5`)
		assert.Contains(t, jsonStr, `"fix":"3D"`)
	})

	t.Run("T061: ValidateLocation rejects lat > 90", func(t *testing.T) {
		loc := &Location{Lat: 90.1, Lon: 0, Fix: FixNone}
		err := ValidateLocation(loc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "latitude")
	})

	t.Run("T061: ValidateLocation rejects lat < -90", func(t *testing.T) {
		loc := &Location{Lat: -90.1, Lon: 0, Fix: FixNone}
		err := ValidateLocation(loc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "latitude")
	})

	t.Run("T061: ValidateLocation rejects lon > 180", func(t *testing.T) {
		loc := &Location{Lat: 0, Lon: 180.1, Fix: FixNone}
		err := ValidateLocation(loc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "longitude")
	})

	t.Run("T061: ValidateLocation rejects lon < -180", func(t *testing.T) {
		loc := &Location{Lat: 0, Lon: -180.1, Fix: FixNone}
		err := ValidateLocation(loc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "longitude")
	})

	t.Run("T061: location round-trips correctly", func(t *testing.T) {
		alt := 1000.0
		original := BuildHeartbeatPayload(1700000060, RoleBuyer,
			WithLocation(&Location{Lat: -33.8688, Lon: 151.2093, Alt: &alt, Fix: Fix3D}),
		)
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored HeartbeatPayload
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)
		require.NotNil(t, restored.Location)
		assert.Equal(t, -33.8688, restored.Location.Lat)
		assert.Equal(t, 151.2093, restored.Location.Lon)
		require.NotNil(t, restored.Location.Alt)
		assert.Equal(t, 1000.0, *restored.Location.Alt)
		assert.Equal(t, Fix3D, restored.Location.Fix)
	})
}

// --- Phase 7 (T062): Peers Field Handling ---

func TestPeersFieldSerialization(t *testing.T) {
	t.Run("T062: peers serialization with valid addresses", func(t *testing.T) {
		peers := []AbbreviatedAddress{"a1b2", "dead", "beef", "cafe"}
		p := BuildHeartbeatPayload(1700000060, RoleBuyer, WithPeers(peers))
		data, err := json.Marshal(p)
		require.NoError(t, err)
		jsonStr := string(data)

		assert.Contains(t, jsonStr, `"peers":["a1b2","dead","beef","cafe"]`,
			"peers must be serialized as a JSON array of strings")
	})

	t.Run("T062: peers format validation", func(t *testing.T) {
		validPeers := []AbbreviatedAddress{"0000", "ffff", "a1b2", "c3d4"}
		for _, peer := range validPeers {
			err := ValidateAbbreviatedAddress(peer)
			assert.NoError(t, err, "peer %q must be valid", peer)
		}

		invalidPeers := []AbbreviatedAddress{"DEAD", "xyz", "12345", "ab", ""}
		for _, peer := range invalidPeers {
			err := ValidateAbbreviatedAddress(peer)
			assert.Error(t, err, "peer %q must be invalid", peer)
		}
	})

	t.Run("T062: peers round-trip serialization", func(t *testing.T) {
		peers := []AbbreviatedAddress{"dead", "beef", "cafe"}
		original := BuildHeartbeatPayload(1700000060, RoleBuyer, WithPeers(peers))
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored HeartbeatPayload
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)
		require.Len(t, restored.Peers, 3)
		assert.Equal(t, AbbreviatedAddress("dead"), restored.Peers[0])
		assert.Equal(t, AbbreviatedAddress("beef"), restored.Peers[1])
		assert.Equal(t, AbbreviatedAddress("cafe"), restored.Peers[2])
	})

	t.Run("T062: single peer serialization", func(t *testing.T) {
		peers := []AbbreviatedAddress{"abcd"}
		p := BuildHeartbeatPayload(1700000060, RoleBuyer, WithPeers(peers))
		data, err := json.Marshal(p)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"peers":["abcd"]`)
	})
}

// --- Phase 8 (T067): Backend-Agnostic Payload Schema ---

func TestBackendAgnosticPayloadSchema(t *testing.T) {
	// T067: Serialize HeartbeatPayload and assert identical JSON structure regardless
	// of target backend. The payload schema is transport-independent.
	t.Run("T067: payload JSON is identical regardless of target backend", func(t *testing.T) {
		alt := 250.0
		payload := BuildHeartbeatPayload(1700000060, RoleValidator,
			WithCapabilities(&Capabilities{
				NATReachability: true,
				NATType:         NATEndpointIndependent,
				Protocols:       []ProtocolID{"/adsb/v1"},
			}),
			WithLocation(&Location{Lat: 37.7749, Lon: -122.4194, Alt: &alt, Fix: Fix3D}),
			WithPeers([]AbbreviatedAddress{"dead", "beef"}),
		)

		// Serialize once — the result must be the same regardless of backend.
		data1, err := json.Marshal(payload)
		require.NoError(t, err)

		// Serialize a second copy (simulating "targeting a different backend").
		data2, err := json.Marshal(payload)
		require.NoError(t, err)

		assert.Equal(t, data1, data2,
			"payload JSON must be byte-identical regardless of target backend")

		// Verify the structure contains all expected top-level fields.
		var rawMap map[string]json.RawMessage
		err = json.Unmarshal(data1, &rawMap)
		require.NoError(t, err)

		// All mandatory fields must be present.
		assert.Contains(t, rawMap, "type")
		assert.Contains(t, rawMap, "version")
		assert.Contains(t, rawMap, "nextHeartbeatDeadline")
		assert.Contains(t, rawMap, "role")
		// Optional fields present because we provided them.
		assert.Contains(t, rawMap, "capabilities")
		assert.Contains(t, rawMap, "location")
		assert.Contains(t, rawMap, "peers")
	})

	t.Run("T067: mandatory-only payload schema is backend-agnostic", func(t *testing.T) {
		payload := BuildHeartbeatPayload(1700000060, RoleBuyer)
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		var rawMap map[string]json.RawMessage
		err = json.Unmarshal(data, &rawMap)
		require.NoError(t, err)

		// Exactly 4 mandatory fields — no backend-specific fields.
		assert.Len(t, rawMap, 4, "mandatory-only payload must have exactly 4 fields")
		assert.Contains(t, rawMap, "type")
		assert.Contains(t, rawMap, "version")
		assert.Contains(t, rawMap, "nextHeartbeatDeadline")
		assert.Contains(t, rawMap, "role")
	})
}

// --- Stage 3C (FR-H05a): OperationalCapabilities extension point ---

// TestCapabilities_OperationalRoundTrip asserts a heartbeat carrying
// the new Operational sub-struct serializes with all populated fields
// using canonical-JSON keys and round-trips byte-for-byte through
// MarshalJSON / UnmarshalJSON. FR-H05a anchor.
func TestCapabilities_OperationalRoundTrip(t *testing.T) {
	caps := &Capabilities{
		Protocols: []ProtocolID{"/ds240/raw/1.0.0"},
		Operational: &OperationalCapabilities{
			ServiceName:    "remote-id",
			SellerEVM:      "0xABCDEF0123456789ABCDEF0123456789ABCDEF01",
			SellerPeerID:   "12D3KooWExamplePeerIDForTesting",
			TopicBackend:   "memory",
			EscrowBackend:  "memory",
			AgentURISha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Degraded:       false,
		},
	}
	payload := BuildHeartbeatPayload(1700000060, RoleSeller, WithCapabilities(caps))
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	jsonStr := string(data)

	// All operational keys present on the wire.
	assert.Contains(t, jsonStr, `"serviceName":"remote-id"`)
	assert.Contains(t, jsonStr, `"sellerEVM":"0xABCDEF0123456789ABCDEF0123456789ABCDEF01"`)
	assert.Contains(t, jsonStr, `"sellerPeerID":"12D3KooWExamplePeerIDForTesting"`)
	assert.Contains(t, jsonStr, `"topicBackend":"memory"`)
	assert.Contains(t, jsonStr, `"escrowBackend":"memory"`)
	assert.Contains(t, jsonStr, `"agentURISha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`)
	// Degraded=false is omitempty per FR-H05a; ABSENT keeps the wire small.
	assert.NotContains(t, jsonStr, `"degraded"`,
		"degraded=false must be omitted (omitempty) so existing observers don't see a noise field")

	// Operational appears inside capabilities object.
	capsIdx := strings.Index(jsonStr, `"capabilities"`)
	opIdx := strings.Index(jsonStr, `"operational"`)
	assert.Greater(t, opIdx, capsIdx,
		"operational must appear inside the capabilities object")

	// Round-trip preserves every field.
	var restored HeartbeatPayload
	require.NoError(t, json.Unmarshal(data, &restored))
	require.NotNil(t, restored.Capabilities)
	require.NotNil(t, restored.Capabilities.Operational)
	op := restored.Capabilities.Operational
	assert.Equal(t, "remote-id", op.ServiceName)
	assert.Equal(t, "0xABCDEF0123456789ABCDEF0123456789ABCDEF01", op.SellerEVM)
	assert.Equal(t, "12D3KooWExamplePeerIDForTesting", op.SellerPeerID)
	assert.Equal(t, "memory", op.TopicBackend)
	assert.Equal(t, "memory", op.EscrowBackend)
	assert.Equal(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", op.AgentURISha256)
	assert.False(t, op.Degraded)
}

// TestCapabilities_OperationalDegradedTrueRoundTrip asserts the
// degraded=true variant. With omitempty=true Go's encoder emits the key
// only when the boolean is true, so the wire form ALWAYS carries
// degraded only when the publisher means it.
func TestCapabilities_OperationalDegradedTrueRoundTrip(t *testing.T) {
	caps := &Capabilities{
		Operational: &OperationalCapabilities{
			ServiceName: "remote-id",
			Degraded:    true,
		},
	}
	payload := BuildHeartbeatPayload(1700000060, RoleSeller, WithCapabilities(caps))
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"degraded":true`)

	var restored HeartbeatPayload
	require.NoError(t, json.Unmarshal(data, &restored))
	require.NotNil(t, restored.Capabilities)
	require.NotNil(t, restored.Capabilities.Operational)
	assert.True(t, restored.Capabilities.Operational.Degraded)
}

// TestCapabilities_OperationalOmitemptyWhenNil guarantees Stage 3C's
// backwards-compatibility promise: a Capabilities without Operational
// marshals to a wire form byte-identical to pre-Stage-3C output. No
// "operational" key, no stray separators.
func TestCapabilities_OperationalOmitemptyWhenNil(t *testing.T) {
	caps := &Capabilities{
		NATReachability: true,
		NATType:         NATEndpointIndependent,
		Protocols:       []ProtocolID{"/ds240/raw/1.0.0"},
		CommerceMode:    "full",
		FeedSource:      "live",
		Profile:         "r1-eip8004-registry/1",
		// Operational deliberately omitted.
	}
	payload := BuildHeartbeatPayload(1700000060, RoleSeller, WithCapabilities(caps))
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	jsonStr := string(data)
	assert.NotContains(t, jsonStr, `"operational"`,
		"Capabilities.Operational==nil must produce no operational key on the wire")
}
