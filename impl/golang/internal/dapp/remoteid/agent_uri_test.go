package remoteid

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// --- Test_FR_R01 wrappers (CONFORMANCE.md Option B style) ---

// Test_FR_R01_AgentURIServiceDescriptor groups every assertion that traces
// to FR-R01 (registry presence, neuron-commerce entry shape, pricing.unit).
func Test_FR_R01_AgentURIServiceDescriptor(t *testing.T) {
	t.Run("happy-path-defaults", TestBuildServiceDescriptor_HappyPathDefaults)
	t.Run("commerce-entry-name-version-pricing", TestBuildServiceDescriptor_CommerceEntryShape)
	t.Run("commerce-mode-default-registration-only", TestBuildServiceDescriptor_CommerceModeDefault)
	t.Run("rejects-missing-child-key", TestBuildServiceDescriptor_RejectsMissingChildKey)
	t.Run("agent-uri-passes-registration-validator", TestBuildServiceDescriptor_AgentURIPassesValidator)
	t.Run("custom-escrow-binding-honoured", TestBuildServiceDescriptor_CustomEscrowBindingHonoured)
	t.Run("json-roundtrip-stable", TestBuildServiceDescriptor_JSONRoundTripStable)
}

// Test_FR_R02_AgentURIStreamCatalog groups stream-catalog assertions. The
// catalog is part of the *descriptor* (off-wire); on-wire emission inside
// 008 connectionSetup is Stage 2.
func Test_FR_R02_AgentURIStreamCatalog(t *testing.T) {
	t.Run("default-catalog-raw-only", TestBuildServiceDescriptor_DefaultCatalogRawOnly)
	t.Run("filtered-and-status-opt-in", TestBuildServiceDescriptor_FilteredAndStatusOptIn)
	t.Run("raw-protocol-id-canonical", TestBuildServiceDescriptor_RawProtocolIDCanonical)
}

// Test_FR_R14_FeedSourceCapability traces FR-R14/R15 feedSource disclosure.
func Test_FR_R14_FeedSourceCapability(t *testing.T) {
	t.Run("default-live", TestBuildServiceDescriptor_FeedSourceDefaultLive)
	t.Run("respects-replay-synth-placeholder", TestBuildServiceDescriptor_FeedSourceRespectsOverrides)
}

// Test_FR_P58_CommerceModeCapability traces 008 FR-P58 commerceMode.
func Test_FR_P58_CommerceModeCapability(t *testing.T) {
	t.Run("default-registration-only", TestBuildServiceDescriptor_CommerceModeDefault)
	t.Run("accepts-full-and-data-only", TestBuildServiceDescriptor_CommerceModeRespectsOverrides)
}

// --- Underlying tests ---

func newTestKey(t *testing.T) *keylib.NeuronPrivateKey {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	return &k
}

func TestBuildServiceDescriptor_HappyPathDefaults(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)

	desc, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: key})
	require.NoError(t, err)

	// FR-R01: neuron-commerce entry must exist.
	commerceServices := desc.AgentURI.CommerceServices()
	require.Len(t, commerceServices, 1, "exactly one neuron-commerce entry expected")
	assert.Equal(t, CommerceServiceName, commerceServices[0].Name)
	assert.Equal(t, CommerceServiceVersion, commerceServices[0].Version)
	assert.Equal(t, PricingUnit, commerceServices[0].Pricing.Unit, "FR-R01 pricing.unit MUST be 'frame'")

	// Defaults pulled through. Stage-2 flipped the default from
	// "registration-only" to "full" — callers who want the R1
	// short-circuit must opt in explicitly.
	assert.Equal(t, FeedSourceLive, desc.FeedSource)
	assert.Equal(t, CommerceModeFull, desc.CommerceMode)
	assert.Equal(t, ProfileR1, desc.ProfileID)
}

func TestBuildServiceDescriptor_CommerceEntryShape(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)

	desc, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:        key,
		ChainID:         296,
		EscrowContract:  "0xCAFE0000000000000000000000000000000000ce",
		PricingAmount:   "0",
		PricingCurrency: "USDC",
		PricingInterval: "0",
	})
	require.NoError(t, err)

	svc := desc.AgentURI.CommerceServices()[0]

	// FR-R01: name + version + pricing.unit.
	assert.Equal(t, "remote-id", svc.Name)
	assert.Equal(t, "1.0.0", svc.Version)
	assert.Equal(t, "frame", svc.Pricing.Unit)

	// Pricing fields plumbed.
	assert.Equal(t, "0", svc.Pricing.Amount)
	assert.Equal(t, "USDC", svc.Pricing.Currency)
	assert.Equal(t, "0", svc.Pricing.Interval)

	// V-REG-13: commerce delivery.serviceRef cross-refs neuron-p2p-exchange name.
	assert.Equal(t, payment.DeliveryModeP2P, svc.Delivery.Mode)
	assert.Equal(t, P2PServiceName, svc.Delivery.ServiceRef)

	// Settlement binding + config plumbed.
	assert.Equal(t, DefaultSettlementBinding, svc.Settlement.Binding)
	assert.Equal(t, uint64(296), svc.Settlement.Config["chainId"])
	assert.Equal(t, "0xCAFE0000000000000000000000000000000000ce", svc.Settlement.Config["contract"])
}

func TestBuildServiceDescriptor_CustomEscrowBindingHonoured(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)
	desc, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:      key,
		EscrowBinding: "hedera-native",
	})
	require.NoError(t, err)
	assert.Equal(t, "hedera-native", desc.AgentURI.CommerceServices()[0].Settlement.Binding)
}

func TestBuildServiceDescriptor_CommerceModeDefault(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)
	desc, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: key})
	require.NoError(t, err)
	// FR-P58: Stage-2 default is "full" (the demo engages the full
	// 008 lifecycle). Callers can still pass CommerceModeRegistrationOnly
	// explicitly to recreate the Stage-1 R1 short-circuit.
	assert.Equal(t, CommerceModeFull, desc.CommerceMode)
}

func TestBuildServiceDescriptor_CommerceModeRespectsOverrides(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{CommerceModeFull, CommerceModeRegistrationOnly, CommerceModeDataOnly} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			key := newTestKey(t)
			desc, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: key, CommerceMode: mode})
			require.NoError(t, err)
			assert.Equal(t, mode, desc.CommerceMode)
		})
	}
}

func TestBuildServiceDescriptor_FeedSourceDefaultLive(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)
	desc, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: key})
	require.NoError(t, err)
	// FR-R15 default.
	assert.Equal(t, FeedSourceLive, desc.FeedSource)
}

func TestBuildServiceDescriptor_FeedSourceRespectsOverrides(t *testing.T) {
	t.Parallel()
	for _, src := range []string{FeedSourceLive, FeedSourceReplay, FeedSourceSynthetic, FeedSourcePlaceholder} {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			key := newTestKey(t)
			desc, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: key, FeedSource: src})
			require.NoError(t, err)
			assert.Equal(t, src, desc.FeedSource)
		})
	}
}

func TestBuildServiceDescriptor_RejectsMissingChildKey(t *testing.T) {
	t.Parallel()
	_, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: nil})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ChildKey is required")
}

func TestBuildServiceDescriptor_AgentURIPassesValidator(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)
	desc, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: key})
	require.NoError(t, err)

	// Re-run ValidateRegistrationCompleteness explicitly so we trace the
	// V-REG-* rules even when the builder's internal call passes silently.
	valid, vErrs := registry.ValidateRegistrationCompleteness(desc.AgentURI, key.PublicKey())
	require.True(t, valid, "agentURI must satisfy V-REG-01..V-REG-13; got: %v", vErrs)

	// Spot-check the rules that matter most to the R1 demo:
	topics := desc.AgentURI.TopicServices()
	require.Len(t, topics, 3, "V-REG-01 expects exactly 3 topic services")

	channels := map[string]int{}
	for _, ts := range topics {
		channels[ts.Channel]++
	}
	// V-REG-11: stdIn / stdOut / stdErr each appear once.
	assert.Equal(t, 1, channels["stdIn"])
	assert.Equal(t, 1, channels["stdOut"])
	assert.Equal(t, 1, channels["stdErr"])

	// V-REG-12: PeerID matches childPub.PeerID().
	expectedPeer, err := key.PublicKey().PeerID()
	require.NoError(t, err)
	p2pSvcs := desc.AgentURI.P2PServices()
	require.Len(t, p2pSvcs, 1)
	assert.Equal(t, expectedPeer.String(), p2pSvcs[0].PeerID)
	// V-REG-05: p2p.topicRef points at one of the topic names.
	assert.Equal(t, TopicNameStdOut, p2pSvcs[0].TopicRef)
	// FR-R02: protocol-id is /ds240/raw/1.0.0.
	assert.Equal(t, ProtocolRaw, p2pSvcs[0].Protocol)
}

func TestBuildServiceDescriptor_JSONRoundTripStable(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)
	desc, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:       key,
		ChainID:        296,
		EscrowContract: "0xCAFE0000000000000000000000000000000000ce",
	})
	require.NoError(t, err)

	uriJSON, err := desc.AgentURI.ToJSON()
	require.NoError(t, err)

	// Sanity: the raw-stream protocol-id appears as a value somewhere.
	assert.Contains(t, uriJSON, ProtocolRaw)
	assert.Contains(t, uriJSON, P2PServiceName)
	assert.Contains(t, uriJSON, CommerceServiceName)

	// Round-trip parse → original equality (defensive: this guards against
	// MarshalJSON omitting fields we just wrote).
	parsed, err := registry.AgentURIFromJSON(uriJSON)
	require.NoError(t, err)

	parsedJSON, err := parsed.ToJSON()
	require.NoError(t, err)

	// Comparing as parsed JSON to remain stable under any future canonical
	// ordering tweaks.
	var raw1, raw2 any
	require.NoError(t, json.Unmarshal([]byte(uriJSON), &raw1))
	require.NoError(t, json.Unmarshal([]byte(parsedJSON), &raw2))
	assert.Equal(t, raw1, raw2)
}

func TestBuildServiceDescriptor_DefaultCatalogRawOnly(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)
	desc, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: key})
	require.NoError(t, err)

	require.Len(t, desc.Streams, 1, "default catalog is raw-only (017 FR-R02)")
	assert.Equal(t, "raw", desc.Streams[0].Name)
	assert.Equal(t, ProtocolRaw, desc.Streams[0].ProtocolID)
}

func TestBuildServiceDescriptor_FilteredAndStatusOptIn(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)
	desc, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey: key,
		Catalog: CatalogOptions{
			IncludeFiltered: true,
			IncludeStatus:   true,
		},
	})
	require.NoError(t, err)

	require.Len(t, desc.Streams, 3)
	assert.Equal(t, ProtocolRaw, desc.Streams[0].ProtocolID)
	assert.Equal(t, ProtocolFilteredPattern, desc.Streams[1].ProtocolID)
	assert.Equal(t, ProtocolStatus, desc.Streams[2].ProtocolID)
}

func TestBuildServiceDescriptor_RawProtocolIDCanonical(t *testing.T) {
	t.Parallel()
	// Belt-and-braces guard against accidental rename of ProtocolRaw.
	assert.Equal(t, "/ds240/raw/1.0.0", ProtocolRaw)
}

// TestBuildServiceDescriptor_WithMultiaddrs asserts the Stage 3B
// plumbing: DescriptorOptions.Multiaddrs land verbatim on the
// generated AgentURI's neuron-p2p-exchange service so the registry can
// surface them to buyers via DiscoverResult.ResolveDialAddrs.
func TestBuildServiceDescriptor_WithMultiaddrs(t *testing.T) {
	t.Parallel()
	key, err := keylib.NewNeuronPrivateKey()
	if err != nil {
		t.Fatalf("new key: %v", err)
	}
	addrs := []string{
		"/ip4/10.0.0.5/udp/41523/quic-v1",
		"/ip4/192.168.1.7/udp/41523/quic-v1",
	}
	desc, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:   &key,
		Multiaddrs: addrs,
	})
	if err != nil {
		t.Fatalf("BuildServiceDescriptor: %v", err)
	}
	p2p := desc.AgentURI.P2PServices()
	if len(p2p) != 1 {
		t.Fatalf("want 1 p2p service, got %d", len(p2p))
	}
	assert.Equal(t, addrs, p2p[0].Multiaddrs,
		"Stage 3B: DescriptorOptions.Multiaddrs must round-trip onto NeuronP2PExchangeService.Multiaddrs")
}

// TestBuildServiceDescriptor_OmitsMultiaddrsByDefault asserts the
// descriptor remains backwards-compatible: when DescriptorOptions does
// not pass Multiaddrs, the AgentURI carries no multiaddrs field
// (omitempty JSON tag preserves wire compatibility with Stage 1/2
// consumers).
func TestBuildServiceDescriptor_OmitsMultiaddrsByDefault(t *testing.T) {
	t.Parallel()
	key, err := keylib.NewNeuronPrivateKey()
	if err != nil {
		t.Fatalf("new key: %v", err)
	}
	desc, err := BuildServiceDescriptor(DescriptorOptions{ChildKey: &key})
	if err != nil {
		t.Fatalf("BuildServiceDescriptor: %v", err)
	}
	p2p := desc.AgentURI.P2PServices()
	if len(p2p) != 1 {
		t.Fatalf("want 1 p2p service, got %d", len(p2p))
	}
	assert.Empty(t, p2p[0].Multiaddrs,
		"Stage 3B: omitting Multiaddrs in options must leave NeuronP2PExchangeService.Multiaddrs empty")
}
