package health

import (
	"fmt"
	"regexp"
)

// NodeRole represents the role a node declares in its heartbeat.
// FR-H02, FR-H05.
type NodeRole string

const (
	// RoleBuyer identifies a buyer node in the network.
	RoleBuyer NodeRole = "buyer"
	// RoleSeller identifies a seller node in the network.
	RoleSeller NodeRole = "seller"
	// RoleRelay identifies a relay node in the network.
	RoleRelay NodeRole = "relay"
	// RoleValidator identifies a validator node in the network.
	RoleValidator NodeRole = "validator"
)

// AllNodeRoles returns all valid NodeRole values.
func AllNodeRoles() []NodeRole {
	return []NodeRole{RoleBuyer, RoleSeller, RoleRelay, RoleValidator}
}

// ValidNodeRole returns true if the given role is one of the known NodeRole constants.
func ValidNodeRole(role NodeRole) bool {
	for _, r := range AllNodeRoles() {
		if r == role {
			return true
		}
	}
	return false
}

// NATType represents the NAT traversal classification of a node.
// FR-H03.
type NATType string

const (
	// NATNone indicates no NAT is present (publicly reachable).
	NATNone NATType = "no-nat"
	// NATEndpointIndependent indicates endpoint-independent NAT mapping.
	NATEndpointIndependent NATType = "endpoint-independent"
	// NATAddressDependent indicates address-dependent NAT mapping.
	NATAddressDependent NATType = "address-dependent"
	// NATAddressAndPortDependent indicates address-and-port-dependent NAT mapping.
	NATAddressAndPortDependent NATType = "address-and-port-dependent"
)

// AllNATTypes returns all valid NATType values.
func AllNATTypes() []NATType {
	return []NATType{NATNone, NATEndpointIndependent, NATAddressDependent, NATAddressAndPortDependent}
}

// GPSFixQuality represents the quality of a GPS position fix.
// FR-H03.
type GPSFixQuality string

const (
	// FixNone indicates no GPS fix is available.
	FixNone GPSFixQuality = "none"
	// Fix2D indicates a 2D GPS fix (latitude and longitude only).
	Fix2D GPSFixQuality = "2D"
	// Fix3D indicates a 3D GPS fix (latitude, longitude, and altitude).
	Fix3D GPSFixQuality = "3D"
)

// AllGPSFixQualities returns all valid GPSFixQuality values.
func AllGPSFixQualities() []GPSFixQuality {
	return []GPSFixQuality{FixNone, Fix2D, Fix3D}
}

// AbbreviatedAddress is a 4-character lowercase hex string representing a
// truncated peer address. Used in the peers field of HeartbeatPayload.
// This is gossip-grade data and MUST NOT be used for trust, routing, or
// identity decisions (FR-H27).
type AbbreviatedAddress string

// abbreviatedAddressRegex matches exactly 4 lowercase hexadecimal characters.
var abbreviatedAddressRegex = regexp.MustCompile(`^[0-9a-f]{4}$`)

// ValidateAbbreviatedAddress checks that the given address is exactly 4 lowercase hex characters.
// Returns an error if the address is invalid.
func ValidateAbbreviatedAddress(addr AbbreviatedAddress) error {
	if !abbreviatedAddressRegex.MatchString(string(addr)) {
		return fmt.Errorf("abbreviated address must be exactly 4 lowercase hex characters, got %q", addr)
	}
	return nil
}

// ProtocolID represents a libp2p protocol identifier string.
// FR-H03.
type ProtocolID string
