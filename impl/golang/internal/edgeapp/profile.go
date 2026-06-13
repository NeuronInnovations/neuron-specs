package edgeapp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// ProfileDescriptorPayloadType is the topic-message payload type discriminator
// used when the seller publishes its descriptor on stdOut. Validators and
// observers filter on this value to skip non-descriptor envelopes.
const ProfileDescriptorPayloadType = "neuron-profile-descriptor/1"

// DefaultEdgeProfileID is the spec-013 Profile E identifier; current edge
// seller / buyer pair satisfies the Profile E NATed-seller→public-buyer
// shape. See specs/013-connectivity-profiles/profiles/E-natd-seller.md.
const DefaultEdgeProfileID = "e-natd-seller/1"

// ProfileDescriptor is the structured form of the spec-013 profile descriptor
// the seller publishes on its stdOut at startup. The struct mirrors the
// "Profile Descriptor Model" section of specs/013-connectivity-profiles/spec.md
// while staying minimal enough to build from edgeapp's runtime state without
// pulling in a full schema validator.
//
// Canonical JSON is produced by Marshal — fields are emitted in a fixed
// alphabetical order with maps' keys sorted, so the resulting bytes hash
// stably across restarts (modulo `version` and `transports[].multiaddr`,
// which the caller controls via the `version` field and Topology choice).
//
// **Size budget for HCS publish:** the signed TopicMessage envelope wrapping
// this payload must fit under HCS's 1024-byte single-message limit (spec 004
// + plan §15.14 R3). The signing wrapper adds ~470 bytes (signature +
// pubkey + canonical-JSON envelope), so the descriptor payload itself must
// stay under ~550 bytes. Capabilities + per-binding CapabilitiesProvided
// + Protocols can blow this budget; we publish only the small "pointer"
// shape (NeuronProfileID + Version + Identity + Transports[].Binding +
// IssuedAt) and rely on observers to look up the full capability vector
// via NeuronProfileID against the spec doc + the agentURI's services list.
//
// Profile E's authoritative content lives at:
// specs/013-connectivity-profiles/profiles/E-natd-seller.md.
type ProfileDescriptor struct {
	// NeuronProfileID is the registered profile identifier (e.g.
	// "e-natd-seller/1"). REQUIRED per spec 013.
	NeuronProfileID string `json:"neuronProfileId"`

	// Version is the descriptor revision counter — bump when the publisher
	// wants observers to re-fetch. Distinct from the profile's own
	// version inside NeuronProfileID. REQUIRED, integer ≥ 1.
	Version int `json:"version"`

	// Identity carries the publisher's stable identity binding so the
	// descriptor can be cross-checked against on-chain agentURI and the
	// libp2p PeerID.
	Identity ProfileIdentity `json:"identity"`

	// Capabilities is the closed-vocabulary capability vector (per spec
	// 013's capability-vector contract). For HCS-published descriptors
	// this field is omitted to fit the 1024-byte signed-envelope budget;
	// observers look up the full vector via NeuronProfileID against the
	// spec doc. Set this manually when serializing for HTTPS / IPFS
	// publishers that don't have the per-message size constraint.
	Capabilities map[string]any `json:"capabilities,omitempty"`

	// Transports is the list of valid transport bindings. Each entry
	// declares its protocol id, a multiaddr (when applicable), and the
	// capabilities the binding *provides* — must be a superset of the
	// profile's required vector (FR-CP-012). For HCS-published
	// descriptors, CapabilitiesProvided is omitted (size budget) and
	// inheritance from the top-level vector is implied.
	Transports []ProfileTransport `json:"transports,omitempty"`

	// Protocols maps semantic protocol names to libp2p protocol IDs the
	// profile uses on top of the bindings. Omitted for HCS publishes
	// where size matters; observers can derive from NeuronProfileID.
	Protocols map[string]string `json:"protocols,omitempty"`

	// LegacyCompatibility, when set, advertises the legacy bootstrap
	// paths the publisher continues to serve in parallel for backward
	// compat (per FR-CP-009).
	LegacyCompatibility *ProfileLegacyCompat `json:"legacyCompatibility,omitempty"`

	// IssuedAt is the wall-clock UTC time the descriptor was built, in
	// RFC 3339. Informational; not load-bearing for the hash.
	IssuedAt string `json:"issuedAt"`
}

// ProfileIdentity carries the on-chain + libp2p identity binding. EVMAddress
// is required; PeerID is optional (some publishers may not run libp2p).
type ProfileIdentity struct {
	EVMAddress string `json:"evmAddress"`
	PeerID     string `json:"peerId,omitempty"`
}

// ProfileTransport is one transport binding entry per spec 013's binding model.
type ProfileTransport struct {
	Binding              string         `json:"binding"`
	Multiaddr            string         `json:"multiaddr,omitempty"`
	CapabilitiesProvided map[string]any `json:"capabilitiesProvided,omitempty"`
}

// ProfileLegacyCompat advertises legacy bootstrap paths a publisher continues
// to serve (FR-CP-009 for the N+2 release window).
type ProfileLegacyCompat struct {
	BootstrapPath   string `json:"bootstrapPath,omitempty"`
	BootstrapWTPath string `json:"bootstrapWtPath,omitempty"`
}

// EdgeProfileECapabilities returns the capability vector for the seller side
// of Profile E (NATed seller → public buyer). Mirrors profiles/E-natd-seller.md
// § "Capability vector — machine-readable form".
func EdgeProfileECapabilities() map[string]any {
	return map[string]any{
		"control-plane":         "topic",
		"audit-trail":           "client-publish",
		"identity-lifetime":     "persistent",
		"listen-capability":     "outbound-dial-only",
		"nat-traversal":         "outbound-dial-only",
		"settlement":            "mock",
		"max-payload":           65536,
		"confidentiality":       "transport+payload-ecies",
		"ordering":              "fifo-per-stream",
		"reconnect-semantics":   "seller-driven",
	}
}

// BuildProfileDescriptor assembles a ProfileDescriptor for a Profile E seller
// from its identity + the libp2p stream protocol it speaks. Returns the struct;
// callers serialize via Marshal for hashing or publishing.
//
// version: monotonic descriptor-revision counter; pass the current value
// from EdgeState (zero-value 0 ⇒ first revision, normalized to 1 here).
func BuildProfileDescriptor(
	evmAddress, peerID, libp2pProtocol string,
	version int,
) ProfileDescriptor {
	if version < 1 {
		version = 1
	}
	d := ProfileDescriptor{
		NeuronProfileID: DefaultEdgeProfileID,
		Version:         version,
		Identity: ProfileIdentity{
			EVMAddress: evmAddress,
			PeerID:     peerID,
		},
		// Capabilities + Protocols intentionally omitted to fit the HCS
		// 1024-byte signed-envelope budget. The full capability vector is
		// derivable from NeuronProfileID against profiles/E-natd-seller.md;
		// the protocol map duplicates information already in the spec.
		Transports: []ProfileTransport{
			{Binding: "T-QUIC"},
		},
		IssuedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return d
}

// Marshal returns a canonical-JSON serialization of d: object keys are
// emitted in a fixed order (defined by the explicit field order in the
// helper below), and embedded map keys (Capabilities, Protocols,
// CapabilitiesProvided) are alphabetized so the resulting bytes are
// reproducible across restarts and across libraries.
//
// The standard library's encoding/json sorts map keys alphabetically by
// default; combining that with our struct-field ordering gives a fully
// canonical form.
func (d ProfileDescriptor) Marshal() ([]byte, error) {
	if d.NeuronProfileID == "" {
		return nil, errors.New("profile descriptor: NeuronProfileID required")
	}
	if d.Version < 1 {
		return nil, errors.New("profile descriptor: Version must be >= 1")
	}
	if d.Identity.EVMAddress == "" {
		return nil, errors.New("profile descriptor: Identity.EVMAddress required")
	}

	// Sort transports by Binding for stability.
	if len(d.Transports) > 1 {
		sort.SliceStable(d.Transports, func(i, j int) bool {
			return d.Transports[i].Binding < d.Transports[j].Binding
		})
	}

	return json.Marshal(d)
}

// Hash returns the lowercase hex SHA256 of d's canonical JSON. Callers
// compare this against EdgeState.ProfileDescriptorHash to decide whether
// to re-publish the descriptor on stdOut. IssuedAt is excluded from the
// hash so a hash-stable descriptor that only differs in publish wall-clock
// is treated as unchanged.
func (d ProfileDescriptor) Hash() (string, error) {
	dCopy := d
	dCopy.IssuedAt = "" // exclude from hash
	data, err := dCopy.Marshal()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// PublishProfileDescriptor signs and publishes the descriptor as a
// TopicMessage on the seller's stdOut. The published payload is the
// canonical-JSON bytes from Marshal. Returns (hash, publishErr) — the
// hash is recorded into EdgeState.ProfileDescriptorHash on success so
// the next startup can compare and skip a redundant publish.
//
// fireAndForget: TopicAdapter.Publish is invoked with FireAndForget; HCS
// confirmation latency would otherwise gate startup.
func PublishProfileDescriptor(
	bus topic.TopicAdapter,
	key *keylib.NeuronPrivateKey,
	stdOut topic.TopicRef,
	d ProfileDescriptor,
) (string, error) {
	if bus == nil {
		return "", errors.New("profile descriptor: bus required")
	}
	if key == nil {
		return "", errors.New("profile descriptor: key required")
	}
	if stdOut.Locator() == "" {
		return "", errors.New("profile descriptor: stdOut required")
	}

	data, err := d.Marshal()
	if err != nil {
		return "", err
	}

	now := uint64(time.Now().UnixNano())
	msg, err := topic.NewTopicMessage(key, now, now, data)
	if err != nil {
		return "", fmt.Errorf("profile descriptor: sign: %w", err)
	}
	if _, err := bus.Publish(stdOut, msg, topic.PublishOpts{ConfirmationMode: topic.FireAndForget}); err != nil {
		return "", fmt.Errorf("profile descriptor: publish: %w", err)
	}
	hash, _ := d.Hash()
	return hash, nil
}

// EnsurePublishedDescriptor publishes d only if its hash differs from the
// hash recorded in state.ProfileDescriptorHash, or if state is nil
// (publish-once-per-startup-when-state-disabled semantics).
//
// On success, state.ProfileDescriptorHash is updated in-place; the caller
// is responsible for SaveEdgeState'ing it. Returns (newHash, published,
// err). published=false means the on-disk hash matched and publish was
// skipped (no error).
func EnsurePublishedDescriptor(
	bus topic.TopicAdapter,
	key *keylib.NeuronPrivateKey,
	stdOut topic.TopicRef,
	d ProfileDescriptor,
	state *EdgeState,
) (string, bool, error) {
	want, err := d.Hash()
	if err != nil {
		return "", false, err
	}
	if state != nil && state.ProfileDescriptorHash != "" && state.ProfileDescriptorHash == want {
		return want, false, nil
	}
	got, err := PublishProfileDescriptor(bus, key, stdOut, d)
	if err != nil {
		return "", false, err
	}
	if state != nil {
		state.ProfileDescriptorHash = got
	}
	return got, true, nil
}
