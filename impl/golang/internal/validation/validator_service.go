package validation

import (
	"encoding/json"
	"fmt"
)

// ServiceTypeValidator is the agentURI service type for validators. FR-V11.
const ServiceTypeValidator = "neuron-validator"

// VerdictDeliveryTopic is the only supported verdict delivery mode. FR-V12.
const VerdictDeliveryTopic = "topic"

// NeuronValidatorService represents a validator's service entry in agentURI. FR-V12.
type NeuronValidatorService struct {
	serviceType     string   // "neuron-validator"
	name            string   // e.g. "validation"
	version         string   // e.g. "1.0.0"
	domains         []string // e.g. ["005-health", "008-payment"]
	verdictDelivery string   // "topic"
}

// NewNeuronValidatorService constructs a validated NeuronValidatorService. FR-V11, FR-V12.
func NewNeuronValidatorService(name, version string, domains []string, verdictDelivery string) (*NeuronValidatorService, error) {
	if name == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "validator service name is required")
	}
	if version == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "validator service version is required")
	}
	if err := validateVersion(version); err != nil {
		return nil, err
	}
	if len(domains) == 0 {
		return nil, NewValidationError(ErrInvalidDomains, "validator service domains must not be empty")
	}
	for _, d := range domains {
		if !IsValidSpecRef(d) {
			return nil, NewValidationError(ErrInvalidDomains,
				fmt.Sprintf("domain %q is not a valid spec reference", d))
		}
	}
	if verdictDelivery != VerdictDeliveryTopic {
		return nil, NewValidationError(ErrInvalidVerdictDelivery,
			fmt.Sprintf("verdictDelivery %q is not supported; only %q is allowed", verdictDelivery, VerdictDeliveryTopic))
	}

	copiedDomains := make([]string, len(domains))
	copy(copiedDomains, domains)

	return &NeuronValidatorService{
		serviceType:     ServiceTypeValidator,
		name:            name,
		version:         version,
		domains:         copiedDomains,
		verdictDelivery: verdictDelivery,
	}, nil
}

// Accessor methods.

func (s *NeuronValidatorService) ServiceType() string     { return s.serviceType }
func (s *NeuronValidatorService) Name() string             { return s.name }
func (s *NeuronValidatorService) Version() string          { return s.version }
func (s *NeuronValidatorService) VerdictDelivery() string  { return s.verdictDelivery }

// Domains returns a copy of the domains array.
func (s *NeuronValidatorService) Domains() []string {
	copied := make([]string, len(s.domains))
	copy(copied, s.domains)
	return copied
}

// MarshalJSON implements canonical JSON serialization for the validator service.
// Field order: domains→name→type→verdictDelivery→version (alphabetical per agentURI convention).
func (s NeuronValidatorService) MarshalJSON() ([]byte, error) {
	buf := []byte{'{'}

	// domains
	buf = append(buf, `"domains":[`...)
	for i, d := range s.domains {
		if i > 0 {
			buf = append(buf, ',')
		}
		dBytes, _ := json.Marshal(d)
		buf = append(buf, dBytes...)
	}
	buf = append(buf, ']')

	// name
	buf = append(buf, ',')
	nameBytes, _ := json.Marshal(s.name)
	buf = append(buf, `"name":`...)
	buf = append(buf, nameBytes...)

	// type
	buf = append(buf, ',')
	typeBytes, _ := json.Marshal(s.serviceType)
	buf = append(buf, `"type":`...)
	buf = append(buf, typeBytes...)

	// verdictDelivery
	buf = append(buf, ',')
	vdBytes, _ := json.Marshal(s.verdictDelivery)
	buf = append(buf, `"verdictDelivery":`...)
	buf = append(buf, vdBytes...)

	// version
	buf = append(buf, ',')
	verBytes, _ := json.Marshal(s.version)
	buf = append(buf, `"version":`...)
	buf = append(buf, verBytes...)

	buf = append(buf, '}')
	return buf, nil
}

// ParseValidatorService parses a neuron-validator service from an agentURI JSON service object.
// FR-V12: extracts type, name, version, domains, verdictDelivery.
func ParseValidatorService(serviceJSON map[string]any) (*NeuronValidatorService, error) {
	svcType, ok := serviceJSON["type"].(string)
	if !ok || svcType != ServiceTypeValidator {
		return nil, NewValidationError(ErrInvalidServiceType,
			fmt.Sprintf("expected service type %q, got %q", ServiceTypeValidator, svcType))
	}

	name, _ := serviceJSON["name"].(string)
	version, _ := serviceJSON["version"].(string)
	verdictDelivery, _ := serviceJSON["verdictDelivery"].(string)

	domainsRaw, ok := serviceJSON["domains"].([]any)
	if !ok {
		return nil, NewValidationError(ErrInvalidDomains, "domains must be an array")
	}
	domains := make([]string, 0, len(domainsRaw))
	for _, d := range domainsRaw {
		ds, ok := d.(string)
		if !ok {
			return nil, NewValidationError(ErrInvalidDomains,
				fmt.Sprintf("domain entry must be a string, got %T", d))
		}
		domains = append(domains, ds)
	}

	return NewNeuronValidatorService(name, version, domains, verdictDelivery)
}
