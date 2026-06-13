package payment

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T005: NeuronCommerceService Construction & Validation ---

func TestNewNeuronCommerceService_ValidP2P(t *testing.T) {
	// FR-P01, FR-P01a: Valid P2P delivery configuration.
	svc, err := NewNeuronCommerceService(
		"adsb-v0.1", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p-adsb"},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: "3600"},
	)
	require.NoError(t, err)
	assert.Equal(t, "neuron-commerce", svc.Type)
	assert.Equal(t, "adsb-v0.1", svc.Name)
	assert.Equal(t, "1.0.0", svc.Version)
	assert.Equal(t, DeliveryModeP2P, svc.Delivery.Mode)
	assert.Equal(t, "p2p-adsb", svc.Delivery.ServiceRef)
	assert.Equal(t, "evm-escrow", svc.Settlement.Binding)
}

func TestNewNeuronCommerceService_ValidTopic(t *testing.T) {
	// FR-P01a: Valid topic delivery configuration.
	svc, err := NewNeuronCommerceService(
		"metrics-v1", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeTopic, ChannelRef: "custom:metrics"},
		SettlementDescriptor{Binding: "hedera-native"},
		PricingDescriptor{Amount: "5", Currency: "HBAR", Unit: "tinybar", Interval: "0"},
	)
	require.NoError(t, err)
	assert.Equal(t, DeliveryModeTopic, svc.Delivery.Mode)
	assert.Equal(t, "custom:metrics", svc.Delivery.ChannelRef)
}

func TestNewNeuronCommerceService_ValidCustom(t *testing.T) {
	// FR-P01a: Valid custom delivery mode.
	svc, err := NewNeuronCommerceService(
		"stream-v1", "1.0.0",
		DeliveryDescriptor{Mode: "custom:websocket"},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "1", Currency: "ETH", Unit: "wei", Interval: "60"},
	)
	require.NoError(t, err)
	assert.True(t, svc.Delivery.Mode.IsCustom())
}

func TestNewNeuronCommerceService_WithTermsRef(t *testing.T) {
	// FR-P04: Optional termsRef.
	svc, err := NewNeuronCommerceService(
		"adsb-v0.1", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p"},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: "3600"},
		WithTermsRef("https://example.com/terms.json"),
	)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/terms.json", svc.TermsRef)
}

func TestNewNeuronCommerceService_MissingName(t *testing.T) {
	_, err := NewNeuronCommerceService(
		"", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p"},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: "3600"},
	)
	require.Error(t, err)
	var pe *PaymentError
	require.True(t, errors.As(err, &pe))
	assert.Equal(t, ErrInvalidServiceOffering, pe.Kind())
}

func TestNewNeuronCommerceService_MissingDeliveryMode(t *testing.T) {
	_, err := NewNeuronCommerceService(
		"svc", "1.0.0",
		DeliveryDescriptor{Mode: ""},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: "3600"},
	)
	require.Error(t, err)
	var pe *PaymentError
	require.True(t, errors.As(err, &pe))
	assert.Equal(t, ErrUnsupportedDeliveryMode, pe.Kind())
}

func TestNewNeuronCommerceService_P2PWithoutServiceRef(t *testing.T) {
	// FR-P01a: P2P mode requires serviceRef.
	_, err := NewNeuronCommerceService(
		"svc", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: "3600"},
	)
	require.Error(t, err)
	var pe *PaymentError
	require.True(t, errors.As(err, &pe))
	assert.Equal(t, ErrInvalidServiceOffering, pe.Kind())
	assert.Contains(t, pe.Error(), "serviceRef")
}

func TestNewNeuronCommerceService_TopicWithoutChannelRef(t *testing.T) {
	// FR-P01a: Topic mode requires channelRef.
	_, err := NewNeuronCommerceService(
		"svc", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeTopic},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: "3600"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channelRef")
}

func TestNewNeuronCommerceService_EmptySettlementBinding(t *testing.T) {
	_, err := NewNeuronCommerceService(
		"svc", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p"},
		SettlementDescriptor{Binding: ""},
		PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: "3600"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settlement.binding")
}

func TestNewNeuronCommerceService_EmptyPricingFields(t *testing.T) {
	// FR-P03: All pricing fields required.
	fields := []struct {
		name    string
		pricing PricingDescriptor
	}{
		{"empty amount", PricingDescriptor{Amount: "", Currency: "USDC", Unit: "token", Interval: "3600"}},
		{"empty currency", PricingDescriptor{Amount: "10", Currency: "", Unit: "token", Interval: "3600"}},
		{"empty unit", PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "", Interval: "3600"}},
		{"empty interval", PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: ""}},
	}
	for _, tt := range fields {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewNeuronCommerceService(
				"svc", "1.0.0",
				DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p"},
				SettlementDescriptor{Binding: "evm-escrow"},
				tt.pricing,
			)
			require.Error(t, err)
		})
	}
}

// --- T006: Descriptor Types ---

func TestDeliveryMode_IsValid(t *testing.T) {
	assert.True(t, DeliveryModeP2P.IsValid())
	assert.True(t, DeliveryModeTopic.IsValid())
	assert.True(t, DeliveryMode("custom:websocket").IsValid())
	assert.True(t, DeliveryMode("custom:grpc").IsValid())
	assert.False(t, DeliveryMode("").IsValid())
	assert.False(t, DeliveryMode("invalid").IsValid())
	assert.False(t, DeliveryMode("customwithoutcolon").IsValid())
}

func TestDeliveryMode_IsCustom(t *testing.T) {
	assert.False(t, DeliveryModeP2P.IsCustom())
	assert.False(t, DeliveryModeTopic.IsCustom())
	assert.True(t, DeliveryMode("custom:ws").IsCustom())
}

// --- T007: Canonical JSON Round-Trip ---

func TestNeuronCommerceService_JSON_RoundTrip(t *testing.T) {
	// SC-P02, SC-P10: Canonical JSON round-trip byte-identical.
	svc, err := NewNeuronCommerceService(
		"adsb-v0.1", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p-adsb"},
		SettlementDescriptor{Binding: "evm-escrow", Config: map[string]any{"chainId": "296"}},
		PricingDescriptor{Amount: "10", Currency: "USDC", Unit: "token", Interval: "3600"},
		WithTermsRef("https://example.com/terms.json"),
	)
	require.NoError(t, err)

	// Serialize.
	data, err := json.Marshal(svc)
	require.NoError(t, err)

	// Deserialize.
	var svc2 NeuronCommerceService
	err = json.Unmarshal(data, &svc2)
	require.NoError(t, err)

	// Re-serialize and compare bytes.
	data2, err := json.Marshal(svc2)
	require.NoError(t, err)

	assert.Equal(t, string(data), string(data2), "canonical JSON must round-trip identically")
}

func TestNeuronCommerceService_JSON_FieldOrder(t *testing.T) {
	// FR-P01: type → name → version → delivery → settlement → pricing → termsRef*
	svc, err := NewNeuronCommerceService(
		"svc", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p"},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "1", Currency: "USDC", Unit: "token", Interval: "0"},
	)
	require.NoError(t, err)

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	jsonStr := string(data)
	// Verify field ordering: type before name before version before delivery...
	typeIdx := findJSONKeyIndex(jsonStr, "type")
	nameIdx := findJSONKeyIndex(jsonStr, "name")
	versionIdx := findJSONKeyIndex(jsonStr, "version")
	deliveryIdx := findJSONKeyIndex(jsonStr, "delivery")
	settlementIdx := findJSONKeyIndex(jsonStr, "settlement")
	pricingIdx := findJSONKeyIndex(jsonStr, "pricing")

	assert.Greater(t, nameIdx, typeIdx, "name must come after type")
	assert.Greater(t, versionIdx, nameIdx, "version must come after name")
	assert.Greater(t, deliveryIdx, versionIdx, "delivery must come after version")
	assert.Greater(t, settlementIdx, deliveryIdx, "settlement must come after delivery")
	assert.Greater(t, pricingIdx, settlementIdx, "pricing must come after settlement")
}

func TestNeuronCommerceService_JSON_OmitsEmptyTermsRef(t *testing.T) {
	// FR-W04: Optional fields omitted when absent.
	svc, err := NewNeuronCommerceService(
		"svc", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p"},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "1", Currency: "USDC", Unit: "token", Interval: "0"},
	)
	require.NoError(t, err)

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "termsRef", "empty termsRef must be omitted")
}

func TestNeuronCommerceService_JSON_IncludesTermsRef(t *testing.T) {
	svc, err := NewNeuronCommerceService(
		"svc", "1.0.0",
		DeliveryDescriptor{Mode: DeliveryModeP2P, ServiceRef: "p2p"},
		SettlementDescriptor{Binding: "evm-escrow"},
		PricingDescriptor{Amount: "1", Currency: "USDC", Unit: "token", Interval: "0"},
		WithTermsRef("https://example.com"),
	)
	require.NoError(t, err)

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"termsRef"`)
}

// --- T036 (partial): Multi-binding filtering ---

func TestFilterByBinding(t *testing.T) {
	// FR-P05: Filter by settlement binding.
	svcs := []NeuronCommerceService{
		{Name: "svc1", Settlement: SettlementDescriptor{Binding: "evm-escrow"}},
		{Name: "svc2", Settlement: SettlementDescriptor{Binding: "hedera-native"}},
		{Name: "svc3", Settlement: SettlementDescriptor{Binding: "evm-escrow"}},
	}

	evm := FilterByBinding(svcs, "evm-escrow")
	assert.Len(t, evm, 2)
	assert.Equal(t, "svc1", evm[0].Name)
	assert.Equal(t, "svc3", evm[1].Name)

	hbar := FilterByBinding(svcs, "hedera-native")
	assert.Len(t, hbar, 1)

	none := FilterByBinding(svcs, "x402")
	assert.Empty(t, none)
}

func TestFilterByName(t *testing.T) {
	svcs := []NeuronCommerceService{
		{Name: "adsb-v0.1", Settlement: SettlementDescriptor{Binding: "evm-escrow"}},
		{Name: "adsb-v0.1", Settlement: SettlementDescriptor{Binding: "hedera-native"}},
		{Name: "other", Settlement: SettlementDescriptor{Binding: "evm-escrow"}},
	}

	adsb := FilterByName(svcs, "adsb-v0.1")
	assert.Len(t, adsb, 2)
}

// --- helper ---

func findJSONKeyIndex(jsonStr, key string) int {
	target := `"` + key + `"`
	for i := 0; i <= len(jsonStr)-len(target); i++ {
		if jsonStr[i:i+len(target)] == target {
			return i
		}
	}
	return -1
}
