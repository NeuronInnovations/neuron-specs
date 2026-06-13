package account

import (
	"encoding/json"
	"fmt"
)

// FR-025: EIP-8004 registry compatibility
// RegistryBinding represents the binding between a Neuron account and an external
// identity registry (EIP-8004). This connects a Child account to its on-chain
// registration entry.
//
// registryIdentifier follows the agentRegistry format: "{namespace}:{chainId}:{identityRegistry}"
// (e.g. "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18").
//
// externalID is the identifier within that registry (e.g. an agentId or token ID).
type RegistryBinding struct {
	registryIdentifier string
	externalID         string
}

// RegistryIdentifier returns the registry identifier string.
func (rb *RegistryBinding) RegistryIdentifier() string { return rb.registryIdentifier }

// ExternalID returns the external ID string.
func (rb *RegistryBinding) ExternalID() string { return rb.externalID }

// NewRegistryBinding constructs a RegistryBinding with the given identifier and external ID.
func NewRegistryBinding(registryIdentifier, externalID string) RegistryBinding {
	return RegistryBinding{
		registryIdentifier: registryIdentifier,
		externalID:         externalID,
	}
}

// MarshalJSON implements json.Marshaler for RegistryBinding.
func (rb RegistryBinding) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		RegistryIdentifier string `json:"registryIdentifier"`
		ExternalID         string `json:"externalID"`
	}{
		RegistryIdentifier: rb.registryIdentifier,
		ExternalID:         rb.externalID,
	})
}

// UnmarshalJSON implements json.Unmarshaler for RegistryBinding.
func (rb *RegistryBinding) UnmarshalJSON(data []byte) error {
	var j struct {
		RegistryIdentifier string `json:"registryIdentifier"`
		ExternalID         string `json:"externalID"`
	}
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	rb.registryIdentifier = j.RegistryIdentifier
	rb.externalID = j.ExternalID
	return nil
}

// ValidateRegistryBinding checks that a RegistryBinding has non-empty fields.
// Returns an error if registryIdentifier or externalID is empty.
func ValidateRegistryBinding(rb RegistryBinding) error {
	if rb.registryIdentifier == "" {
		return fmt.Errorf("registry binding: RegistryIdentifier must not be empty")
	}
	if rb.externalID == "" {
		return fmt.Errorf("registry binding: ExternalID must not be empty")
	}
	return nil
}
