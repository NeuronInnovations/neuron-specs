package payment

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Spec 006 FR-W02: ServiceRequest.negotiationDeadline is an UnsignedInt64
// field that MUST be encoded as a JSON string containing the decimal
// representation. The existing TestServiceRequest_JSON_RoundTrip and
// TestServiceRequest_JSON_FieldOrder tests cover round-trip identity and
// key ordering but would still pass if both marshal and unmarshal silently
// regressed from quoted-string to number form together. These tests lock
// the canonical wire form explicitly and cannot silently regress.

func TestServiceRequest_FR_W02_NegotiationDeadlineIsQuotedString(t *testing.T) {
	req := ServiceRequest{
		Type:                PayloadServiceRequest,
		Version:             "1.0.0",
		RequestID:           "fr-w02-quoted",
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
	s := string(data)

	// Positive: the canonical wire form is a quoted decimal string.
	assert.Contains(t, s, `"negotiationDeadline":"1711382400"`,
		"FR-W02: negotiationDeadline must be a quoted decimal string in canonical wire form")

	// Negative: the legacy JSON-number form must NOT appear. This catches
	// regressions where strconv.FormatUint is accidentally replaced by a
	// direct uint64 value or where the field type changes in marshalOrderedJSON.
	assert.NotContains(t, s, `"negotiationDeadline":1711382400`,
		"FR-W02 regression: negotiationDeadline must not be emitted as a JSON number")
}

func TestServiceRequest_FR_W02_NegotiationDeadlineLargeValueSurvives(t *testing.T) {
	// Use a deadline above the JavaScript safe-integer ceiling (2^53). Even
	// though ServiceRequest.negotiationDeadline is documented as Unix epoch
	// seconds (well below 2^53), the FR-W02 quoted-string contract applies
	// to ALL UnsignedInt64 wire fields regardless of magnitude. Using a
	// large value here guarantees any regression to number form would be
	// observable as precision loss on a JavaScript round-trip.
	req := ServiceRequest{
		Type:                PayloadServiceRequest,
		Version:             "1.0.0",
		RequestID:           "fr-w02-large",
		ServiceRef:          "svc",
		SettlementBinding:   "evm-escrow",
		ProposedAmount:      "1",
		ProposedCurrency:    "USDC",
		ProposedInterval:    "0",
		NegotiationDeadline: 9007199254740993, // 2^53 + 1
		BuyerStdIn:          "0.0.1",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"negotiationDeadline":"9007199254740993"`,
		"FR-W02: values above 2^53 must round-trip losslessly as quoted strings")

	var restored ServiceRequest
	require.NoError(t, json.Unmarshal(data, &restored))
	assert.Equal(t, uint64(9007199254740993), restored.NegotiationDeadline,
		"FR-W02: large deadline must round-trip byte-exactly through canonical JSON")
}

func TestServiceRequest_FR_W02_RoundTripQuotedForm(t *testing.T) {
	// FR-W02 + FR-P12: marshal → unmarshal → marshal must be byte-identical
	// AND the intermediate quoted-string form must be preserved.
	original := ServiceRequest{
		Type:                PayloadServiceRequest,
		Version:             "1.0.0",
		RequestID:           "rt",
		ServiceRef:          "svc",
		SettlementBinding:   "evm-escrow",
		ProposedAmount:      "5",
		ProposedCurrency:    "USDC",
		ProposedInterval:    "60",
		NegotiationDeadline: 1711382400,
		BuyerStdIn:          "0.0.9",
	}

	data1, err := json.Marshal(original)
	require.NoError(t, err)

	var restored ServiceRequest
	require.NoError(t, json.Unmarshal(data1, &restored))
	assert.Equal(t, original.NegotiationDeadline, restored.NegotiationDeadline,
		"round-trip through canonical (quoted) form must preserve deadline value")

	data2, err := json.Marshal(restored)
	require.NoError(t, err)
	assert.Equal(t, string(data1), string(data2),
		"ServiceRequest must re-marshal byte-identically after round-trip")
}
