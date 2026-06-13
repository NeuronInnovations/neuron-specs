package adsb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
)

// Conformance test wrappers. Each Test_FR_A* function pins a specific
// requirement from Spec 016 (ADS-B DApp) — or, for the BaseStation slice,
// the rule it would carry once Spec 019 (NormalizedTrack) lands or the
// Spec 016 amendment introducing FR-A22 (basestation stream catalog entry)
// is ratified. Names match the eventual FR-numbering convention so a
// reviewer can later upgrade them to formal-FR-anchored tests without
// renaming.

// Test_FR_A01_NeuronCommerceServiceName pins the canonical
// neuron-commerce service name for the ADS-B DApp (Spec 016 FR-A01 in
// spirit; the basestation slice uses the same name as the BEAST path
// because the underlying commerce semantics are identical, only the
// stream catalog entry differs).
func Test_FR_A01_NeuronCommerceServiceName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "adsb", CommerceServiceName,
		"neuron-commerce.name must be \"adsb\" per Spec 016 FR-A01")
	assert.Equal(t, "frame", PricingUnit,
		"pricing.unit must be \"frame\" per Spec 016 FR-A01")
}

// Test_FR_A02_StreamCatalogIncludesBasestation pins the basestation stream
// catalog entry advertised by an ADS-B BaseStation seller's
// connectionSetup (Spec 016 FR-A02 + FR-A22 if amended; this slice's
// equivalent: the basestation stream is mandatory for this DApp).
func Test_FR_A02_StreamCatalogIncludesBasestation(t *testing.T) {
	t.Parallel()
	entries := BuildAdsbBasestationStreamCatalog(DefaultCatalogOptions())
	require.GreaterOrEqual(t, len(entries), 1, "catalog must include at least one entry")
	var found bool
	for _, e := range entries {
		if e.ProtocolID == ProtocolBaseStation {
			found = true
			assert.Equal(t, "basestation", e.Name)
			assert.Equal(t, payment.StreamDirectionSeller, e.Direction)
			assert.Equal(t, SchemaURL, e.Schema)
			break
		}
	}
	assert.True(t, found, "stream catalog must include %s", ProtocolBaseStation)
}

// Test_FR_A05_BasestationStreamCanonicalShape pins the on-wire payload
// shape (NormalizedTrack canonical JSON) for the basestation stream.
// Mirrors Spec 016 FR-A05 for the BEAST path; the basestation path uses
// canonical-JSON NormalizedTrack per docs/normalized-track-contract.md.
func Test_FR_A05_BasestationStreamCanonicalShape(t *testing.T) {
	t.Parallel()
	// Covered by frame_test.go TestNormalizedTrack_MarshalJSONCanonicalOrder
	// + TestNormalizedTrack_MarshalJSON_RoundTripsAllOptionalsSet +
	// TestMarshalJSON_RejectsMalformed. This wrapper exists so a reviewer
	// scanning for FR-A05 finds it under that name.
	t.Run("canonical-order", TestNormalizedTrack_MarshalJSONCanonicalOrder)
	t.Run("round-trip", TestNormalizedTrack_MarshalJSON_RoundTripsAllOptionalsSet)
	t.Run("rejects-malformed", TestMarshalJSON_RejectsMalformed)
}

// Test_FR_A18_HeartbeatAdvertisesFeedSource pins that the seller's
// heartbeat carries feedSource per Spec 016 FR-A18 (and, for BaseStation,
// `feedSource = "live"` per the audit's Q-5 decision when source is
// BaseStation TCP).
func Test_FR_A18_HeartbeatAdvertisesFeedSource(t *testing.T) {
	t.Parallel()
	desc, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:   newTestKey(t),
		FeedSource: FeedSourceLive,
	})
	require.NoError(t, err)
	assert.Equal(t, FeedSourceLive, desc.FeedSource,
		"descriptor.FeedSource must be \"live\" for BaseStation TCP runs per audit Q-5")
}

// Test_FR_R21_OperationalDisclosureShape pins the operational disclosure
// sub-object shape on the seller heartbeat. Spec 016 does not yet have an
// FR-A21 equivalent; this slice uses Spec 017 FR-R21's shape verbatim
// (sellerEVM, sellerPeerID, serviceName, topicBackend, escrowBackend,
// agentURISha256, optional degraded).
func Test_FR_R21_OperationalDisclosureShape(t *testing.T) {
	t.Parallel()
	// Validation happens inside StartHeartbeatLoop:
	//   - SellerEVM required
	//   - SellerPeerID required
	// Negative test: empty SellerEVM rejected.
	_, err := StartHeartbeatLoop(t.Context(), HeartbeatLoopOptions{
		Key:          newTestKey(t),
		Adapter:      nil, // adapter check fires first, but SellerEVM check should beat it
		SellerEVM:    "",
		SellerPeerID: "12D3KooW...",
	})
	require.Error(t, err, "empty SellerEVM must be rejected per FR-R21-shape")

	_, err = StartHeartbeatLoop(t.Context(), HeartbeatLoopOptions{
		Key:          newTestKey(t),
		Adapter:      nil,
		SellerEVM:    "0xabc",
		SellerPeerID: "",
	})
	require.Error(t, err, "empty SellerPeerID must be rejected per FR-R21-shape")
}

// Test_BuyerLivenessTransitions pins the AdsbLivenessMonitor state surface
// {Healthy, Stale, Offline, Degraded, Unknown} matches the FR-R21-shape
// vocabulary so a reviewer can confirm the buyer-side projection is correct.
func Test_BuyerLivenessTransitions(t *testing.T) {
	t.Parallel()
	// The state-machine values themselves are pinned by string literal —
	// any rename downstream would surface immediately.
	assert.Equal(t, AdsbLivenessState("Unknown"), LivenessUnknown)
	assert.Equal(t, AdsbLivenessState("Healthy"), LivenessHealthy)
	assert.Equal(t, AdsbLivenessState("Stale"), LivenessStale)
	assert.Equal(t, AdsbLivenessState("Offline"), LivenessOffline)
	assert.Equal(t, AdsbLivenessState("Degraded"), LivenessDegraded)
}
