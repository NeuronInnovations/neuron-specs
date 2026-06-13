package payment

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T020: Payload Type JSON Tests ---

func TestServiceRequest_JSON_RoundTrip(t *testing.T) {
	// SC-P03: Canonical JSON round-trip.
	req := ServiceRequest{
		Type:                PayloadServiceRequest,
		Version:             "1.0.0",
		RequestID:           "550e8400-e29b-41d4-a716-446655440000",
		ServiceRef:          "adsb-v0.1",
		SettlementBinding:   "evm-escrow",
		ProposedAmount:      "10",
		ProposedCurrency:    "USDC",
		ProposedInterval:    "3600",
		NegotiationDeadline: 1711382400,
		BuyerStdIn:          "0.0.54321",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var req2 ServiceRequest
	err = json.Unmarshal(data, &req2)
	require.NoError(t, err)

	data2, err := json.Marshal(req2)
	require.NoError(t, err)

	assert.Equal(t, string(data), string(data2), "serviceRequest must round-trip identically")
}

func TestServiceRequest_JSON_FieldOrder(t *testing.T) {
	req := ServiceRequest{
		Type: PayloadServiceRequest, Version: "1.0.0", RequestID: "id",
		ServiceRef: "svc", SettlementBinding: "evm", ProposedAmount: "1",
		ProposedCurrency: "USDC", ProposedInterval: "0",
		NegotiationDeadline: 1000, BuyerStdIn: "0.0.1",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)
	s := string(data)

	assert.Greater(t, findJSONKeyIndex(s, "version"), findJSONKeyIndex(s, "type"))
	assert.Greater(t, findJSONKeyIndex(s, "requestId"), findJSONKeyIndex(s, "version"))
	assert.Greater(t, findJSONKeyIndex(s, "serviceRef"), findJSONKeyIndex(s, "requestId"))
	assert.Greater(t, findJSONKeyIndex(s, "negotiationDeadline"), findJSONKeyIndex(s, "proposedInterval"))
	assert.Greater(t, findJSONKeyIndex(s, "buyerStdIn"), findJSONKeyIndex(s, "negotiationDeadline"))
}

func TestServiceRequest_JSON_WithServiceParams(t *testing.T) {
	// FR-P07: serviceParams keys lexicographic.
	req := ServiceRequest{
		Type: PayloadServiceRequest, Version: "1.0.0", RequestID: "id",
		ServiceRef: "svc", SettlementBinding: "evm", ProposedAmount: "1",
		ProposedCurrency: "USDC", ProposedInterval: "0",
		ServiceParams:       map[string]any{"region": "VVTS", "format": "json"},
		NegotiationDeadline: 1000, BuyerStdIn: "0.0.1",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)
	s := string(data)

	// "format" before "region" (lexicographic)
	assert.Greater(t, findJSONKeyIndex(s, "region"), findJSONKeyIndex(s, "format"),
		"serviceParams keys must be lexicographically ordered")
}

func TestServiceRequest_JSON_OmitsEmptyArbiter(t *testing.T) {
	req := ServiceRequest{
		Type: PayloadServiceRequest, Version: "1.0.0", RequestID: "id",
		ServiceRef: "svc", SettlementBinding: "evm", ProposedAmount: "1",
		ProposedCurrency: "USDC", ProposedInterval: "0",
		NegotiationDeadline: 1000, BuyerStdIn: "0.0.1",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "arbiter")
}

func TestServiceResponse_JSON_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		resp ServiceResponse
	}{
		{"accept", ServiceResponse{Type: PayloadServiceResponse, Version: "1.0.0", RequestID: "id", Action: "accept"}},
		{"reject", ServiceResponse{Type: PayloadServiceResponse, Version: "1.0.0", RequestID: "id", Action: "reject"}},
		{"counter", ServiceResponse{Type: PayloadServiceResponse, Version: "1.0.0", RequestID: "id", Action: "counter", CounterAmount: "9", CounterInterval: "1800"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			require.NoError(t, err)

			var resp2 ServiceResponse
			err = json.Unmarshal(data, &resp2)
			require.NoError(t, err)

			data2, err := json.Marshal(resp2)
			require.NoError(t, err)

			assert.Equal(t, string(data), string(data2))
		})
	}
}

func TestServiceResponse_JSON_CounterOmitsWhenAccept(t *testing.T) {
	resp := ServiceResponse{Type: PayloadServiceResponse, Version: "1.0.0", RequestID: "id", Action: "accept"}
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "counterAmount")
	assert.NotContains(t, string(data), "counterInterval")
}

func TestConnectionSetup_JSON_RoundTrip(t *testing.T) {
	cs := ConnectionSetup{
		Type: PayloadConnectionSetup, Version: "1.0.0", RequestID: "id",
		PeerID: "12D3KooWTest", EncryptedMultiaddrs: "base64data==",
		Protocol: "/neuron/adsb/1.0.0", NATStatus: "private",
	}

	data, err := json.Marshal(cs)
	require.NoError(t, err)

	var cs2 ConnectionSetup
	err = json.Unmarshal(data, &cs2)
	require.NoError(t, err)

	data2, err := json.Marshal(cs2)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(data2))
}

func TestConnectionSetup_JSON_OmitsEmptyNATStatus(t *testing.T) {
	cs := ConnectionSetup{
		Type: PayloadConnectionSetup, Version: "1.0.0", RequestID: "id",
		PeerID: "12D3KooWTest", EncryptedMultiaddrs: "data==",
		Protocol: "/proto",
	}

	data, err := json.Marshal(cs)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "natStatus")
}

// --- T029: Version Validation ---

func TestValidateVersion(t *testing.T) {
	// FR-P12a: Accept 1.x.y, reject 2+.
	tests := []struct {
		version string
		valid   bool
	}{
		{"1.0.0", true},
		{"1.1.0", true},
		{"1.99.99", true},
		{"2.0.0", false},
		{"3.0.0", false},
		{"0.0.0", false},
		{"", false},
		{"abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			err := ValidateVersion(tt.version)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				var pe *PaymentError
				require.True(t, errors.As(err, &pe))
				assert.Equal(t, ErrVersionMismatch, pe.Kind())
			}
		})
	}
}
