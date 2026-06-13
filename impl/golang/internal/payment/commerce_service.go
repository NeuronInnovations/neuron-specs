package payment

import (
	"encoding/json"
	"strings"
)

// DeliveryMode identifies how service data is delivered.
// FR-P01a: "p2p", "topic", or "custom:<type>".
type DeliveryMode string

const (
	// DeliveryModeP2P indicates direct peer-to-peer stream delivery.
	DeliveryModeP2P DeliveryMode = "p2p"

	// DeliveryModeTopic indicates topic channel subscription delivery.
	DeliveryModeTopic DeliveryMode = "topic"
)

// IsCustom returns true if the delivery mode uses the custom:<type> namespace.
func (m DeliveryMode) IsCustom() bool {
	return strings.HasPrefix(string(m), "custom:")
}

// IsValid returns true if the mode is a recognized value or a valid custom mode.
func (m DeliveryMode) IsValid() bool {
	return m == DeliveryModeP2P || m == DeliveryModeTopic || m.IsCustom()
}

// DeliveryDescriptor describes how service data is delivered.
// FR-P01a: mode + mode-specific cross-references.
// Canonical field order: mode → serviceRef* → channelRef*
type DeliveryDescriptor struct {
	Mode       DeliveryMode `json:"-"` // handled by MarshalJSON
	ServiceRef string       `json:"-"` // when mode=p2p: cross-refs neuron-p2p-exchange name
	ChannelRef string       `json:"-"` // when mode=topic: cross-refs neuron-topic name
}

// MarshalJSON implements canonical field ordering for DeliveryDescriptor.
// FR-P01a: mode → serviceRef* → channelRef*
func (d DeliveryDescriptor) MarshalJSON() ([]byte, error) {
	m := make([]jsonKeyValue, 0, 3)
	m = append(m, jsonKeyValue{"mode", string(d.Mode)})
	if d.ServiceRef != "" {
		m = append(m, jsonKeyValue{"serviceRef", d.ServiceRef})
	}
	if d.ChannelRef != "" {
		m = append(m, jsonKeyValue{"channelRef", d.ChannelRef})
	}
	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a DeliveryDescriptor from JSON.
func (d *DeliveryDescriptor) UnmarshalJSON(data []byte) error {
	var raw struct {
		Mode       string `json:"mode"`
		ServiceRef string `json:"serviceRef"`
		ChannelRef string `json:"channelRef"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	d.Mode = DeliveryMode(raw.Mode)
	d.ServiceRef = raw.ServiceRef
	d.ChannelRef = raw.ChannelRef
	return nil
}

// SettlementDescriptor identifies the payment settlement binding.
// FR-P02: binding + binding-specific config.
type SettlementDescriptor struct {
	Binding string         `json:"-"` // "hedera-native", "evm-escrow", etc.
	Config  map[string]any `json:"-"` // binding-specific fields (chainId, contract, etc.)
}

// MarshalJSON implements canonical field ordering for SettlementDescriptor.
// FR-P02: binding first, then config fields alphabetically.
func (s SettlementDescriptor) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)
	m["binding"] = s.Binding
	for k, v := range s.Config {
		m[k] = v
	}
	return json.Marshal(m)
}

// UnmarshalJSON deserializes a SettlementDescriptor from JSON.
func (s *SettlementDescriptor) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if b, ok := raw["binding"]; ok {
		s.Binding, _ = b.(string)
	}
	delete(raw, "binding")
	if len(raw) > 0 {
		s.Config = raw
	}
	return nil
}

// PricingDescriptor declares pricing terms for the service.
// FR-P03: amount, currency, unit, interval.
// Canonical field order: amount → currency → unit → interval
type PricingDescriptor struct {
	Amount   string `json:"-"` // decimal string (e.g., "10")
	Currency string `json:"-"` // symbol (e.g., "USDC")
	Unit     string `json:"-"` // denomination (e.g., "token")
	Interval string `json:"-"` // billing interval in seconds; "0" = one-time
}

// MarshalJSON implements canonical field ordering for PricingDescriptor.
// FR-P03: amount → currency → unit → interval
func (p PricingDescriptor) MarshalJSON() ([]byte, error) {
	m := []jsonKeyValue{
		{"amount", p.Amount},
		{"currency", p.Currency},
		{"unit", p.Unit},
		{"interval", p.Interval},
	}
	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a PricingDescriptor from JSON.
func (p *PricingDescriptor) UnmarshalJSON(data []byte) error {
	var raw struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
		Unit     string `json:"unit"`
		Interval string `json:"interval"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Amount = raw.Amount
	p.Currency = raw.Currency
	p.Unit = raw.Unit
	p.Interval = raw.Interval
	return nil
}

// NeuronCommerceService is an EIP-8004 service object in agentURI services[].
// FR-P01: neuron-commerce service type.
// Canonical field order: type → name → version → delivery → settlement → pricing → termsRef*
type NeuronCommerceService struct {
	Type       string               `json:"-"`
	Name       string               `json:"-"`
	Version    string               `json:"-"`
	Delivery   DeliveryDescriptor   `json:"-"`
	Settlement SettlementDescriptor `json:"-"`
	Pricing    PricingDescriptor    `json:"-"`
	TermsRef   string               `json:"-"` // optional; omit if empty (FR-W04)
}

// CommerceServiceOption is a functional option for NewNeuronCommerceService.
type CommerceServiceOption func(*NeuronCommerceService)

// WithTermsRef sets the optional termsRef URI. FR-P04.
func WithTermsRef(uri string) CommerceServiceOption {
	return func(s *NeuronCommerceService) {
		s.TermsRef = uri
	}
}

// NewNeuronCommerceService constructs a validated NeuronCommerceService.
// FR-P01: All MUST fields validated. Type hardcoded to "neuron-commerce".
func NewNeuronCommerceService(
	name, version string,
	delivery DeliveryDescriptor,
	settlement SettlementDescriptor,
	pricing PricingDescriptor,
	opts ...CommerceServiceOption,
) (NeuronCommerceService, error) {
	const op = "NewNeuronCommerceService"

	svc := NeuronCommerceService{
		Type:       "neuron-commerce",
		Name:       name,
		Version:    version,
		Delivery:   delivery,
		Settlement: settlement,
		Pricing:    pricing,
	}

	for _, opt := range opts {
		opt(&svc)
	}

	// Validate MUST fields — FR-P01
	if name == "" {
		return NeuronCommerceService{}, NewPaymentError(ErrInvalidServiceOffering, op, "name is required")
	}
	if version == "" {
		return NeuronCommerceService{}, NewPaymentError(ErrInvalidServiceOffering, op, "version is required")
	}

	// Validate delivery — FR-P01a
	if !delivery.Mode.IsValid() {
		return NeuronCommerceService{}, NewPaymentError(ErrUnsupportedDeliveryMode, op,
			"delivery.mode must be 'p2p', 'topic', or 'custom:<type>'")
	}
	if delivery.Mode == DeliveryModeP2P && delivery.ServiceRef == "" {
		return NeuronCommerceService{}, NewPaymentError(ErrInvalidServiceOffering, op,
			"delivery.serviceRef is required when mode is 'p2p'")
	}
	if delivery.Mode == DeliveryModeTopic && delivery.ChannelRef == "" {
		return NeuronCommerceService{}, NewPaymentError(ErrInvalidServiceOffering, op,
			"delivery.channelRef is required when mode is 'topic'")
	}

	// Validate settlement — FR-P02
	if settlement.Binding == "" {
		return NeuronCommerceService{}, NewPaymentError(ErrInvalidServiceOffering, op,
			"settlement.binding is required")
	}

	// Validate pricing — FR-P03
	if pricing.Amount == "" {
		return NeuronCommerceService{}, NewPaymentError(ErrInvalidServiceOffering, op,
			"pricing.amount is required")
	}
	if pricing.Currency == "" {
		return NeuronCommerceService{}, NewPaymentError(ErrInvalidServiceOffering, op,
			"pricing.currency is required")
	}
	if pricing.Unit == "" {
		return NeuronCommerceService{}, NewPaymentError(ErrInvalidServiceOffering, op,
			"pricing.unit is required")
	}
	if pricing.Interval == "" {
		return NeuronCommerceService{}, NewPaymentError(ErrInvalidServiceOffering, op,
			"pricing.interval is required")
	}

	return svc, nil
}

// MarshalJSON implements canonical field ordering for NeuronCommerceService.
// FR-P01: type → name → version → delivery → settlement → pricing → termsRef*
func (s NeuronCommerceService) MarshalJSON() ([]byte, error) {
	deliveryJSON, err := json.Marshal(s.Delivery)
	if err != nil {
		return nil, err
	}
	settlementJSON, err := json.Marshal(s.Settlement)
	if err != nil {
		return nil, err
	}
	pricingJSON, err := json.Marshal(s.Pricing)
	if err != nil {
		return nil, err
	}

	m := []jsonKeyValue{
		{"type", s.Type},
		{"name", s.Name},
		{"version", s.Version},
		{"delivery", json.RawMessage(deliveryJSON)},
		{"settlement", json.RawMessage(settlementJSON)},
		{"pricing", json.RawMessage(pricingJSON)},
	}

	if s.TermsRef != "" {
		m = append(m, jsonKeyValue{"termsRef", s.TermsRef})
	}

	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a NeuronCommerceService from JSON.
func (s *NeuronCommerceService) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type       string               `json:"type"`
		Name       string               `json:"name"`
		Version    string               `json:"version"`
		Delivery   DeliveryDescriptor   `json:"delivery"`
		Settlement SettlementDescriptor `json:"settlement"`
		Pricing    PricingDescriptor    `json:"pricing"`
		TermsRef   string               `json:"termsRef"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Type = raw.Type
	s.Name = raw.Name
	s.Version = raw.Version
	s.Delivery = raw.Delivery
	s.Settlement = raw.Settlement
	s.Pricing = raw.Pricing
	s.TermsRef = raw.TermsRef
	return nil
}

// FilterByBinding returns services matching the given settlement binding. FR-P05.
func FilterByBinding(services []NeuronCommerceService, binding string) []NeuronCommerceService {
	var result []NeuronCommerceService
	for _, svc := range services {
		if svc.Settlement.Binding == binding {
			result = append(result, svc)
		}
	}
	return result
}

// FilterByName returns services matching the given service name. FR-P05.
func FilterByName(services []NeuronCommerceService, name string) []NeuronCommerceService {
	var result []NeuronCommerceService
	for _, svc := range services {
		if svc.Name == name {
			result = append(result, svc)
		}
	}
	return result
}

// --- JSON helpers for canonical field ordering ---

type jsonKeyValue struct {
	Key   string
	Value any
}

// marshalOrderedJSON produces compact JSON with keys in the given order.
func marshalOrderedJSON(pairs []jsonKeyValue) ([]byte, error) {
	buf := []byte{'{'}
	for i, kv := range pairs {
		if i > 0 {
			buf = append(buf, ',')
		}
		keyJSON, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		valJSON, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		buf = append(buf, keyJSON...)
		buf = append(buf, ':')
		buf = append(buf, valJSON...)
	}
	buf = append(buf, '}')
	return buf, nil
}
