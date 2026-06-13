package sapient

// SellerProfile parameterises the modality-specific identity surface of a
// SAPIENT seller: the commerce service name it advertises, the capability
// extension its metadata is namespaced under, the topic anchor, the sensor
// models, and the connectivity-profile id stamped on heartbeats. The SAPIENT
// data plane (opaque SapientMessage forwarding) is profile-independent — only
// the Agent Card / evidence / heartbeat disclosures differ per modality.
//
// A nil *SellerProfile everywhere means RIDProfile(): the original SAPIENT
// Remote ID seller posture, byte-identical to the pre-profile card (the
// AgentURI SHA-256 drives idempotent RegisterOrUpdate on deployed sellers, so
// the default MUST NOT drift).
type SellerProfile struct {
	// CommerceServiceName is the neuron-commerce service name advertised on the
	// card (advertisement/discovery descriptor; FR-S20a).
	CommerceServiceName string

	// ExtensionID is the capability-extension identifier the seller's metadata
	// is namespaced under inside the stdOut topic Config, and the commerce
	// service termsRef.
	ExtensionID string

	// Anchor is the NeuronTopicService.anchor (required non-empty by V-REG-03);
	// identifies the demo profile.
	Anchor string

	// SensorModels is the sensor family advertised in the capability extension.
	// SellerCardOptions.SensorModels still overrides per card.
	SensorModels []string

	// Capabilities, when non-empty, is published as the "capabilities" key of
	// the extension (closed vocabulary per modality). nil omits the key — the
	// RID posture, keeping pre-profile cards byte-identical.
	Capabilities []string

	// HeartbeatProfile is the connectivity-profile id (013 FR-F-02) advertised
	// in heartbeat Capabilities.Profile.
	HeartbeatProfile string
}

// JetVision / ADS-B profile constants (the sibling modality of the RID seller;
// the RID constants live in agentcard.go for historical byte-compat reasons).
const (
	// JVCommerceServiceName is the JetVision ADS-B SAPIENT service name.
	JVCommerceServiceName = "jetvision-adsb-sapient"

	// JVExtensionID namespaces the ADS-B capability extension (the bridge's
	// adsb.* object_info schema is declared under the same id).
	JVExtensionID = "neuron.adsb/1"

	// JVAnchor is the JetVision topic anchor (sibling of sapient-rid-r1).
	JVAnchor = "sapient-adsb-r1"
)

// JVSensorModels is the JetVision sensor family sourced by neuron-jv-bridge.
var JVSensorModels = []string{"JetVision Air!Squitter"}

// JVCapabilities is the closed capability vocabulary the JetVision ADS-B
// seller advertises in its neuron.adsb/1 extension.
var JVCapabilities = []string{
	"sapient.bsi-flex-335-v2.0",
	"sapient.detection-report",
	"neuron.adsb/1",
	"jetvision.air-squitter.aircraftlist",
}

// RIDProfile returns the SAPIENT Remote ID seller posture — the package
// defaults, reproducing the pre-profile card byte-for-byte.
func RIDProfile() SellerProfile {
	return SellerProfile{
		CommerceServiceName: CommerceServiceName,
		ExtensionID:         ExtensionID,
		Anchor:              DefaultAnchor,
		SensorModels:        append([]string(nil), DefaultSensorModels...),
		Capabilities:        nil,
		HeartbeatProfile:    ProfileSAPIENTRID,
	}
}

// JetVisionProfile returns the JetVision ADS-B seller posture
// (cmd/sapient-jv-seller).
func JetVisionProfile() SellerProfile {
	return SellerProfile{
		CommerceServiceName: JVCommerceServiceName,
		ExtensionID:         JVExtensionID,
		Anchor:              JVAnchor,
		SensorModels:        append([]string(nil), JVSensorModels...),
		Capabilities:        append([]string(nil), JVCapabilities...),
		HeartbeatProfile:    ProfileSAPIENTADSB,
	}
}
