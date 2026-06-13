package registry

import (
	"fmt"
	"strings"
)

// RegistryUnavailable indicates the registry contract or its RPC endpoint
// cannot be reached.
type RegistryUnavailable struct {
	Detail string // registry address or RPC error description
}

func (e RegistryUnavailable) Error() string {
	return fmt.Sprintf("registry unavailable: %s", e.Detail)
}

// RegistrationNotFound indicates no registration was found for the given
// lookup key (EVM address or external ID).
type RegistrationNotFound struct {
	Detail string // the lookup key that was not found
}

func (e RegistrationNotFound) Error() string {
	return fmt.Sprintf("registration not found: %s", e.Detail)
}

// IncompleteRegistration indicates a registration is missing required fields
// or services.
type IncompleteRegistration struct {
	Detail string // description of what is missing
}

func (e IncompleteRegistration) Error() string {
	return fmt.Sprintf("incomplete registration: %s", e.Detail)
}

// ProofOfControlFailed indicates the proof-of-control check failed because the
// signer address does not match the expected owner.
type ProofOfControlFailed struct {
	Expected string // expected EVM address (hex)
	Actual   string // actual signer EVM address (hex)
}

func (e ProofOfControlFailed) Error() string {
	return fmt.Sprintf("proof of control failed: expected %s, got %s", e.Expected, e.Actual)
}

// AdmissionDenied indicates the admission policy rejected the registration
// attempt.
type AdmissionDenied struct {
	Detail string // reason for denial
}

func (e AdmissionDenied) Error() string {
	return fmt.Sprintf("admission denied: %s", e.Detail)
}

// DuplicateRegistration indicates an agent is already registered in the target
// registry.
type DuplicateRegistration struct {
	ChildAddress    string // the child's EVM address
	RegistryAddress string // the registry contract address
}

func (e DuplicateRegistration) Error() string {
	return fmt.Sprintf("duplicate registration: child %s already registered in %s",
		e.ChildAddress, e.RegistryAddress)
}

// InvalidDIDService indicates a DID service entry is malformed or missing
// required fields.
type InvalidDIDService struct {
	Detail string // description of what is invalid
}

func (e InvalidDIDService) Error() string {
	return fmt.Sprintf("invalid DID service: %s", e.Detail)
}

// BrokenTopicRef indicates a neuron-p2p-exchange service references a topic
// name that does not exist in the services array.
type BrokenTopicRef struct {
	TopicRef       string   // the referenced topic name
	AvailableNames []string // names of available topic services
}

func (e BrokenTopicRef) Error() string {
	return fmt.Sprintf("broken topic ref: %q not found in available names [%s]",
		e.TopicRef, strings.Join(e.AvailableNames, ", "))
}

// InvalidServiceSchema indicates a service entry has a schema violation
// (wrong type, missing required field, invalid value).
type InvalidServiceSchema struct {
	ServiceType string // the service type (e.g., "neuron-topic")
	Field       string // the offending field name
	Detail      string // what is wrong
}

func (e InvalidServiceSchema) Error() string {
	return fmt.Sprintf("invalid service schema [%s]: field %q: %s",
		e.ServiceType, e.Field, e.Detail)
}

// UnauthorizedOperation indicates the caller does not have the required role
// for the attempted operation.
type UnauthorizedOperation struct {
	CallerRole string // the caller's current role
	Operation  string // the operation that was attempted
}

func (e UnauthorizedOperation) Error() string {
	return fmt.Sprintf("unauthorized operation: role %q cannot perform %q",
		e.CallerRole, e.Operation)
}

// AllowlistRejection indicates the parent's DID is not on the registry's
// allowlist, preventing registration.
type AllowlistRejection struct {
	ParentDID string // the parent DID that was rejected
}

func (e AllowlistRejection) Error() string {
	return fmt.Sprintf("allowlist rejection: parent DID %s is not permitted", e.ParentDID)
}

// InvalidDeliveryRef indicates a neuron-commerce service's delivery descriptor
// references a service name that does not exist in the agentURI.
// FR-P01b: Cross-reference validation for delivery.
type InvalidDeliveryRef struct {
	ServiceName string // the neuron-commerce service name
	DeliveryRef string // the referenced service name (serviceRef or channelRef)
	Mode        string // the delivery mode ("p2p" or "topic")
}

func (e InvalidDeliveryRef) Error() string {
	return fmt.Sprintf("invalid delivery ref in %q: %s ref %q not found in agentURI",
		e.ServiceName, e.Mode, e.DeliveryRef)
}
