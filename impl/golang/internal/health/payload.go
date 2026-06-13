package health

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Capabilities holds optional network capability metadata.
// Included in HeartbeatPayload when the publisher wants to advertise its network properties.
// FR-H03.
//
// CommerceMode / FeedSource / Profile are optional disclosure fields
// added 2026-05-13 to satisfy:
//   - 008 FR-P58 (commerce mode descriptor: full / registration-only / data-only)
//   - 017 FR-R15 (feed source: live / replay / synthetic / placeholder)
//   - 013 FR-F-02 (connectivity profile id)
//
// All three are `omitempty` so a heartbeat that doesn't advertise them
// marshals byte-for-byte identically to pre-2026-05-13 output. Validator
// agents (010) reading a heartbeat MUST treat absent fields as the
// per-FR defaults (`full` / `live` / unspecified).
//
// Operational is the FR-H05a extension point (Stage 3C, 2026-05-13):
// a DApp-defined disclosure block carrying operational metadata such as
// seller identity bindings, backend selectors, and a degraded flag.
// Schema is owned by the consuming DApp spec (e.g., Spec 017 FR-R21 for
// Remote ID). Observers MUST tolerate unknown keys (forward-compat).
type Capabilities struct {
	NATReachability bool         `json:"natReachability"`
	NATType         NATType      `json:"natType"`
	Protocols       []ProtocolID `json:"protocols"`
	// CommerceMode advertises the seller's 008 commerce posture.
	// 008 FR-P58 allowed values: "full", "registration-only", "data-only".
	// Absent → "full" (operational posture default).
	CommerceMode string `json:"commerceMode,omitempty"`
	// FeedSource advertises a DApp's data-plane provenance.
	// 017 FR-R15 (Remote ID) allowed values: "live", "replay",
	// "synthetic", "placeholder". 016 FR-A18 mirrors for ADS-B.
	// Absent → "live" (operational default).
	FeedSource string `json:"feedSource,omitempty"`
	// Profile advertises the connectivity-profile identifier per
	// 013 FR-F-02 (e.g., "f-fixture-direct/1" for Profile F, or
	// DApp-defined profile ids for higher-layer postures).
	// Absent → no claim; validator interprets per context.
	Profile string `json:"profile,omitempty"`
	// Operational is the FR-H05a extension point for DApp-defined
	// operational disclosure. Schema belongs to the consuming DApp spec.
	// Absent → no claim; observers MUST tolerate absence.
	Operational *OperationalCapabilities `json:"operational,omitempty"`
}

// OperationalCapabilities is the FR-H05a DApp-defined disclosure block.
// Spec 017 FR-R21 defines the Remote ID DApp's required + optional fields;
// other DApps may attach additional keys (forward-compat via
// observer-MUST-tolerate-unknown-keys rule).
//
// All fields are omitempty so a publisher populates only what it knows.
// Wire keys are camelCase per 006 canonical JSON conventions.
type OperationalCapabilities struct {
	// ServiceName identifies the DApp commerce service the publisher
	// is advertising — e.g., "remote-id" (017), "adsb" (016).
	ServiceName string `json:"serviceName,omitempty"`
	// SellerEVM is the publisher's EVM address (hex, 0x-prefixed).
	// Useful so observers can correlate the heartbeat with an on-chain
	// registration without re-deriving from the signer field.
	SellerEVM string `json:"sellerEVM,omitempty"`
	// SellerPeerID is the publisher's libp2p PeerID (base58 multihash).
	// Observers MAY use it as a defence-in-depth check against a
	// registered AgentURI's neuron-p2p-exchange.peerID.
	SellerPeerID string `json:"sellerPeerID,omitempty"`
	// ASMNodeID is a DApp-defined node identity carried alongside the
	// EVM/PeerID bindings. Spec 015 (SAPIENT) populates it with the ASM's
	// identity-bound node_id (the UUID a Task addresses, NodeIDFromIdentity);
	// other DApps MAY leave it empty. Observer-tolerated like every other
	// operational key.
	ASMNodeID string `json:"asmNodeId,omitempty"`
	// TopicBackend identifies the topic transport in use (e.g., "memory",
	// "hcs"). DApp-defined vocabulary; observers MUST tolerate unknowns.
	TopicBackend string `json:"topicBackend,omitempty"`
	// EscrowBackend identifies the escrow backend in use (e.g., "memory",
	// "evm"). DApp-defined vocabulary; observers MUST tolerate unknowns.
	EscrowBackend string `json:"escrowBackend,omitempty"`
	// AgentURISha256 is the hex SHA-256 of the publisher's registered
	// AgentURI JSON. Observers MAY cross-check against the value they
	// computed at registry-lookup time; mismatch is evidence-grade but
	// MUST NOT cause silent fallback or state transition.
	AgentURISha256 string `json:"agentURISha256,omitempty"`
	// Degraded signals that the publisher believes its data plane or
	// session state is impaired (e.g., upstream feed silent, libp2p
	// stream lost). False is the default and is omitted from the wire
	// (omitempty) so the "degraded" key appears only when set true.
	Degraded bool `json:"degraded,omitempty"`
}

// Location holds optional geographic position metadata.
// When present, both Lat and Lon are required. Alt is optional.
// FR-H03.
type Location struct {
	Lat float64       `json:"lat"`
	Lon float64       `json:"lon"`
	Alt *float64      `json:"alt,omitempty"`
	Fix GPSFixQuality `json:"fix"`
}

// HeartbeatPayload is the core health protocol message.
// All fields use json:"-" because serialization is handled by custom MarshalJSON
// to guarantee deterministic canonical field ordering.
// FR-H01, FR-H02, FR-H03, FR-H04.
type HeartbeatPayload struct {
	Type                  string               `json:"-"`
	Version               string               `json:"-"`
	NextHeartbeatDeadline uint64               `json:"-"`
	Role                  NodeRole             `json:"-"`
	Capabilities          *Capabilities        `json:"-"`
	Location              *Location            `json:"-"`
	Peers                 []AbbreviatedAddress `json:"-"`
}

// HeartbeatOption is a functional option for BuildHeartbeatPayload.
type HeartbeatOption func(*HeartbeatPayload)

// WithCapabilities sets the optional capabilities field on a HeartbeatPayload.
func WithCapabilities(c *Capabilities) HeartbeatOption {
	return func(p *HeartbeatPayload) {
		p.Capabilities = c
	}
}

// WithLocation sets the optional location field on a HeartbeatPayload.
func WithLocation(l *Location) HeartbeatOption {
	return func(p *HeartbeatPayload) {
		p.Location = l
	}
}

// WithPeers sets the optional peers field on a HeartbeatPayload.
func WithPeers(peers []AbbreviatedAddress) HeartbeatOption {
	return func(p *HeartbeatPayload) {
		p.Peers = peers
	}
}

// BuildHeartbeatPayload constructs a HeartbeatPayload with mandatory fields auto-set.
// Type is always "heartbeat" and Version is always CurrentVersion ("1.0.0").
// Optional fields can be provided via HeartbeatOption functions.
// FR-H01, FR-H02.
func BuildHeartbeatPayload(deadline uint64, role NodeRole, opts ...HeartbeatOption) HeartbeatPayload {
	p := HeartbeatPayload{
		Type:                  PayloadTypeHeartbeat,
		Version:               CurrentVersion,
		NextHeartbeatDeadline: deadline,
		Role:                  role,
	}
	for _, opt := range opts {
		opt(&p)
	}
	return p
}

// MarshalJSON implements json.Marshaler with deterministic canonical field ordering.
// Field order: type, version, nextHeartbeatDeadline, role, then optional fields
// (capabilities, location, peers) only if non-nil/non-empty.
// FR-H04, SC-H10.
func (p HeartbeatPayload) MarshalJSON() ([]byte, error) {
	// Build the JSON manually to guarantee field order.
	// We use a byte slice builder approach for deterministic output.
	buf := []byte{'{'}

	// 1. type (mandatory)
	buf = append(buf, `"type":`...)
	typeBytes, err := json.Marshal(p.Type)
	if err != nil {
		return nil, fmt.Errorf("marshal type: %w", err)
	}
	buf = append(buf, typeBytes...)

	// 2. version (mandatory)
	buf = append(buf, `,"version":`...)
	versionBytes, err := json.Marshal(p.Version)
	if err != nil {
		return nil, fmt.Errorf("marshal version: %w", err)
	}
	buf = append(buf, versionBytes...)

	// 3. nextHeartbeatDeadline (mandatory, JSON string per 006 FR-W02)
	buf = append(buf, `,"nextHeartbeatDeadline":"`...)
	buf = strconv.AppendUint(buf, p.NextHeartbeatDeadline, 10)
	buf = append(buf, '"')

	// 4. role (mandatory)
	buf = append(buf, `,"role":`...)
	roleBytes, err := json.Marshal(p.Role)
	if err != nil {
		return nil, fmt.Errorf("marshal role: %w", err)
	}
	buf = append(buf, roleBytes...)

	// 5. capabilities (optional)
	if p.Capabilities != nil {
		buf = append(buf, `,"capabilities":`...)
		capBytes, err := json.Marshal(p.Capabilities)
		if err != nil {
			return nil, fmt.Errorf("marshal capabilities: %w", err)
		}
		buf = append(buf, capBytes...)
	}

	// 6. location (optional)
	if p.Location != nil {
		buf = append(buf, `,"location":`...)
		locBytes, err := json.Marshal(p.Location)
		if err != nil {
			return nil, fmt.Errorf("marshal location: %w", err)
		}
		buf = append(buf, locBytes...)
	}

	// 7. peers (optional, only if non-empty)
	if len(p.Peers) > 0 {
		buf = append(buf, `,"peers":`...)
		peersBytes, err := json.Marshal(p.Peers)
		if err != nil {
			return nil, fmt.Errorf("marshal peers: %w", err)
		}
		buf = append(buf, peersBytes...)
	}

	buf = append(buf, '}')
	return buf, nil
}

// UnmarshalJSON implements json.Unmarshaler for HeartbeatPayload.
// It deserializes from JSON into the struct fields, regardless of field order in the input.
//
// Per 006 FR-W02 the canonical wire form has `nextHeartbeatDeadline` as a
// quoted decimal string. Liberal in what we accept: legacy JSON-number form
// is also tolerated for backward compatibility with previously-marshaled
// in-memory state.
func (p *HeartbeatPayload) UnmarshalJSON(data []byte) error {
	// Use a temporary struct with exported fields matching the JSON keys.
	var raw struct {
		Type                  string               `json:"type"`
		Version               string               `json:"version"`
		NextHeartbeatDeadline flexibleUint64       `json:"nextHeartbeatDeadline"`
		Role                  NodeRole             `json:"role"`
		Capabilities          *Capabilities        `json:"capabilities"`
		Location              *Location            `json:"location"`
		Peers                 []AbbreviatedAddress `json:"peers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal heartbeat payload: %w", err)
	}

	p.Type = raw.Type
	p.Version = raw.Version
	p.NextHeartbeatDeadline = uint64(raw.NextHeartbeatDeadline)
	p.Role = raw.Role
	p.Capabilities = raw.Capabilities
	p.Location = raw.Location
	p.Peers = raw.Peers

	return nil
}

// flexibleUint64 is a uint64 that accepts both quoted-decimal-string and
// JSON-number forms during deserialization. The canonical wire form per
// 006 FR-W02 is the quoted string; the number form is accepted only for
// backward compatibility with legacy in-memory artifacts.
type flexibleUint64 uint64

// UnmarshalJSON parses either `"123"` or `123` into a uint64.
func (f *flexibleUint64) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("flexibleUint64: empty input")
	}
	s := string(data)
	if s[0] == '"' && len(s) >= 2 && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("flexibleUint64: parse %q: %w", s, err)
	}
	*f = flexibleUint64(n)
	return nil
}

// ValidateLocation validates that a Location, when present, has valid coordinates.
// Lat must be in [-90, 90] and Lon must be in [-180, 180].
// Returns nil if loc is nil (location is optional).
func ValidateLocation(loc *Location) error {
	if loc == nil {
		return nil
	}
	if loc.Lat < -90 || loc.Lat > 90 {
		return fmt.Errorf("latitude must be in [-90, 90], got %f", loc.Lat)
	}
	if loc.Lon < -180 || loc.Lon > 180 {
		return fmt.Errorf("longitude must be in [-180, 180], got %f", loc.Lon)
	}
	return nil
}
