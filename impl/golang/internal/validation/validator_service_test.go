package validation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNeuronValidatorService_Valid(t *testing.T) {
	svc, err := NewNeuronValidatorService("validation", "1.0.0",
		[]string{"005-health", "008-payment"}, "topic")
	require.NoError(t, err)
	assert.Equal(t, ServiceTypeValidator, svc.ServiceType())
	assert.Equal(t, "validation", svc.Name())
	assert.Equal(t, "1.0.0", svc.Version())
	assert.Equal(t, []string{"005-health", "008-payment"}, svc.Domains())
	assert.Equal(t, "topic", svc.VerdictDelivery())
}

func TestNewNeuronValidatorService_EmptyName(t *testing.T) {
	_, err := NewNeuronValidatorService("", "1.0.0", []string{"005-health"}, "topic")
	require.Error(t, err)
}

func TestNewNeuronValidatorService_EmptyVersion(t *testing.T) {
	_, err := NewNeuronValidatorService("validation", "", []string{"005-health"}, "topic")
	require.Error(t, err)
}

func TestNewNeuronValidatorService_EmptyDomains(t *testing.T) {
	_, err := NewNeuronValidatorService("validation", "1.0.0", []string{}, "topic")
	require.Error(t, err)
}

func TestNewNeuronValidatorService_InvalidDomain(t *testing.T) {
	_, err := NewNeuronValidatorService("validation", "1.0.0", []string{"has spaces"}, "topic")
	require.Error(t, err)
}

func TestNewNeuronValidatorService_InvalidVerdictDelivery(t *testing.T) {
	_, err := NewNeuronValidatorService("validation", "1.0.0", []string{"005-health"}, "email")
	require.Error(t, err)
}

func TestNewNeuronValidatorService_IncompatibleVersion(t *testing.T) {
	_, err := NewNeuronValidatorService("validation", "2.0.0", []string{"005-health"}, "topic")
	require.Error(t, err)
}

func TestNeuronValidatorService_MarshalJSON(t *testing.T) {
	svc, err := NewNeuronValidatorService("validation", "1.0.0",
		[]string{"005-health"}, "topic")
	require.NoError(t, err)

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	// Alphabetical field order for agentURI convention.
	expected := `{"domains":["005-health"],"name":"validation","type":"neuron-validator","verdictDelivery":"topic","version":"1.0.0"}`
	assert.Equal(t, expected, string(data))
}

func TestNeuronValidatorService_DomainsIsolation(t *testing.T) {
	original := []string{"005-health", "008-payment"}
	svc, err := NewNeuronValidatorService("validation", "1.0.0", original, "topic")
	require.NoError(t, err)

	// Mutate the original slice — should not affect the service.
	original[0] = "mutated"
	assert.Equal(t, "005-health", svc.Domains()[0])

	// Mutate the returned slice — should not affect the service.
	domains := svc.Domains()
	domains[0] = "mutated"
	assert.Equal(t, "005-health", svc.Domains()[0])
}

func TestParseValidatorService_Valid(t *testing.T) {
	serviceJSON := map[string]any{
		"type":            "neuron-validator",
		"name":            "validation",
		"version":         "1.0.0",
		"domains":         []any{"005-health", "008-payment"},
		"verdictDelivery": "topic",
	}

	svc, err := ParseValidatorService(serviceJSON)
	require.NoError(t, err)
	assert.Equal(t, "validation", svc.Name())
	assert.Equal(t, []string{"005-health", "008-payment"}, svc.Domains())
}

func TestParseValidatorService_WrongType(t *testing.T) {
	serviceJSON := map[string]any{
		"type":    "neuron-topic",
		"name":    "stdOut",
		"version": "1.0.0",
	}
	_, err := ParseValidatorService(serviceJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "neuron-validator")
}

func TestParseValidatorService_MissingDomains(t *testing.T) {
	serviceJSON := map[string]any{
		"type":            "neuron-validator",
		"name":            "validation",
		"version":         "1.0.0",
		"verdictDelivery": "topic",
	}
	_, err := ParseValidatorService(serviceJSON)
	require.Error(t, err)
}
