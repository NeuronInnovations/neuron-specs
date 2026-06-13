package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeRole(t *testing.T) {
	t.Run("enum values", func(t *testing.T) {
		assert.Equal(t, NodeRole("buyer"), RoleBuyer)
		assert.Equal(t, NodeRole("seller"), RoleSeller)
		assert.Equal(t, NodeRole("relay"), RoleRelay)
		assert.Equal(t, NodeRole("validator"), RoleValidator)
	})

	t.Run("AllNodeRoles returns all 4 roles", func(t *testing.T) {
		roles := AllNodeRoles()
		require.Len(t, roles, 4)
		assert.Contains(t, roles, RoleBuyer)
		assert.Contains(t, roles, RoleSeller)
		assert.Contains(t, roles, RoleRelay)
		assert.Contains(t, roles, RoleValidator)
	})

	t.Run("ValidNodeRole accepts known roles", func(t *testing.T) {
		for _, role := range AllNodeRoles() {
			assert.True(t, ValidNodeRole(role), "expected %q to be valid", role)
		}
	})

	t.Run("ValidNodeRole rejects unknown roles", func(t *testing.T) {
		assert.False(t, ValidNodeRole(NodeRole("unknown")))
		assert.False(t, ValidNodeRole(NodeRole("")))
		assert.False(t, ValidNodeRole(NodeRole("BUYER")))
		assert.False(t, ValidNodeRole(NodeRole("miner")))
	})
}

func TestNATType(t *testing.T) {
	t.Run("enum values", func(t *testing.T) {
		assert.Equal(t, NATType("no-nat"), NATNone)
		assert.Equal(t, NATType("endpoint-independent"), NATEndpointIndependent)
		assert.Equal(t, NATType("address-dependent"), NATAddressDependent)
		assert.Equal(t, NATType("address-and-port-dependent"), NATAddressAndPortDependent)
	})

	t.Run("AllNATTypes returns all 4 types", func(t *testing.T) {
		types := AllNATTypes()
		require.Len(t, types, 4)
		assert.Contains(t, types, NATNone)
		assert.Contains(t, types, NATEndpointIndependent)
		assert.Contains(t, types, NATAddressDependent)
		assert.Contains(t, types, NATAddressAndPortDependent)
	})
}

func TestGPSFixQuality(t *testing.T) {
	t.Run("enum values", func(t *testing.T) {
		assert.Equal(t, GPSFixQuality("none"), FixNone)
		assert.Equal(t, GPSFixQuality("2D"), Fix2D)
		assert.Equal(t, GPSFixQuality("3D"), Fix3D)
	})

	t.Run("AllGPSFixQualities returns all 3 qualities", func(t *testing.T) {
		qualities := AllGPSFixQualities()
		require.Len(t, qualities, 3)
		assert.Contains(t, qualities, FixNone)
		assert.Contains(t, qualities, Fix2D)
		assert.Contains(t, qualities, Fix3D)
	})
}

func TestAbbreviatedAddress(t *testing.T) {
	t.Run("ValidateAbbreviatedAddress accepts valid 4-hex addresses", func(t *testing.T) {
		validCases := []AbbreviatedAddress{"a1b2", "0000", "ffff", "dead", "beef", "1234"}
		for _, addr := range validCases {
			err := ValidateAbbreviatedAddress(addr)
			assert.NoError(t, err, "expected %q to be valid", addr)
		}
	})

	t.Run("ValidateAbbreviatedAddress rejects 3-char address", func(t *testing.T) {
		err := ValidateAbbreviatedAddress(AbbreviatedAddress("abc"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exactly 4 lowercase hex characters")
	})

	t.Run("ValidateAbbreviatedAddress rejects 5-char address", func(t *testing.T) {
		err := ValidateAbbreviatedAddress(AbbreviatedAddress("abcde"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exactly 4 lowercase hex characters")
	})

	t.Run("ValidateAbbreviatedAddress rejects non-hex characters", func(t *testing.T) {
		invalidCases := []AbbreviatedAddress{"ghij", "ABCD", "12g4", "zzzz", "ab-c"}
		for _, addr := range invalidCases {
			err := ValidateAbbreviatedAddress(addr)
			assert.Error(t, err, "expected %q to be invalid", addr)
		}
	})

	t.Run("ValidateAbbreviatedAddress rejects empty string", func(t *testing.T) {
		err := ValidateAbbreviatedAddress(AbbreviatedAddress(""))
		assert.Error(t, err)
	})
}

func TestProtocolID(t *testing.T) {
	t.Run("ProtocolID is a string type", func(t *testing.T) {
		pid := ProtocolID("/adsb/v1")
		assert.Equal(t, "/adsb/v1", string(pid))
	})
}
