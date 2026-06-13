package registry

import (
	"fmt"
	"sync"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
)

// ValidationError represents a single validation rule violation during
// registration completeness checks.
type ValidationError struct {
	Rule    string // e.g., "V-REG-01"
	Message string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Rule, e.Message)
}

// AdmissionPolicy describes a registry's admission policy. The mechanism is
// platform-defined; this struct represents the policy's observable properties.
type AdmissionPolicy struct {
	PolicyType  string   `json:"policyType"`  // "permissionless" or "permissioned"
	TrustAnchor string   `json:"trustAnchor"` // e.g., "parent-did"
	Allowlist   []string `json:"allowlist"`   // Parent DIDs (for permissioned registries)
}

// AdmissionChecker defines the interface for admission policy implementations.
// Registries may use permissionless or allowlist-based policies.
type AdmissionChecker interface {
	IsAdmitted(childAddress string, parentDID string) (bool, error)
}

// PermissionlessPolicy always admits any registration attempt.
type PermissionlessPolicy struct{}

// IsAdmitted always returns true for a permissionless registry.
func (p PermissionlessPolicy) IsAdmitted(childAddress, parentDID string) (bool, error) {
	return true, nil
}

// AllowlistPolicy restricts registration to children whose parent DID is on
// the allowlist.
type AllowlistPolicy struct {
	mu        sync.RWMutex
	allowlist map[string]bool
}

// NewAllowlistPolicy creates an empty AllowlistPolicy.
func NewAllowlistPolicy() *AllowlistPolicy {
	return &AllowlistPolicy{
		allowlist: make(map[string]bool),
	}
}

// IsAdmitted returns true if the parentDID is on the allowlist.
func (p *AllowlistPolicy) IsAdmitted(childAddress, parentDID string) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if parentDID == "" {
		return false, AdmissionDenied{Detail: "parentDID is required for allowlist policy"}
	}
	if p.allowlist[parentDID] {
		return true, nil
	}
	return false, AllowlistRejection{ParentDID: parentDID}
}

// AddParentDID adds a parent DID to the allowlist.
func (p *AllowlistPolicy) AddParentDID(did string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.allowlist[did] = true
}

// RemoveParentDID removes a parent DID from the allowlist.
func (p *AllowlistPolicy) RemoveParentDID(did string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.allowlist, did)
}

// Contains returns true if the given DID is on the allowlist.
func (p *AllowlistPolicy) Contains(did string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.allowlist[did]
}

// ValidateRegistrationCompleteness validates an AgentURI against all registration
// completeness rules. It collects ALL violations (never short-circuits) and
// returns (true, nil) when the registration is fully valid.
func ValidateRegistrationCompleteness(agentURI AgentURI, childPublicKey keylib.NeuronPublicKey) (bool, []ValidationError) {
	var errors []ValidationError

	// V-REG-01: Exactly 3 neuron-topic services.
	if len(agentURI.topicServices) != 3 {
		errors = append(errors, ValidationError{
			Rule:    "V-REG-01",
			Message: fmt.Sprintf("expected 3 neuron-topic services, got %d", len(agentURI.topicServices)),
		})
	}

	// V-REG-02: At least 1 neuron-p2p-exchange service.
	if len(agentURI.p2pServices) < 1 {
		errors = append(errors, ValidationError{
			Rule:    "V-REG-02",
			Message: "at least 1 neuron-p2p-exchange service is required",
		})
	}

	// V-REG-03: Each neuron-topic has MUST fields.
	for i, svc := range agentURI.topicServices {
		if svc.Type == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-03",
				Message: fmt.Sprintf("topic service[%d]: type must not be empty", i),
			})
		}
		if svc.Name == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-03",
				Message: fmt.Sprintf("topic service[%d]: name must not be empty", i),
			})
		}
		if svc.Version == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-03",
				Message: fmt.Sprintf("topic service[%d]: version must not be empty", i),
			})
		}
		if svc.Channel == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-03",
				Message: fmt.Sprintf("topic service[%d]: channel must not be empty", i),
			})
		}
		if svc.Transport == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-03",
				Message: fmt.Sprintf("topic service[%d]: transport must not be empty", i),
			})
		}
		if svc.Anchor == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-03",
				Message: fmt.Sprintf("topic service[%d]: anchor must not be empty", i),
			})
		}
		if svc.Config == nil {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-03",
				Message: fmt.Sprintf("topic service[%d]: config must not be nil", i),
			})
		}
	}

	// V-REG-04: Each neuron-p2p-exchange has MUST fields.
	for i, svc := range agentURI.p2pServices {
		if svc.Type == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-04",
				Message: fmt.Sprintf("p2p service[%d]: type must not be empty", i),
			})
		}
		if svc.Name == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-04",
				Message: fmt.Sprintf("p2p service[%d]: name must not be empty", i),
			})
		}
		if svc.Version == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-04",
				Message: fmt.Sprintf("p2p service[%d]: version must not be empty", i),
			})
		}
		if svc.PeerID == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-04",
				Message: fmt.Sprintf("p2p service[%d]: peerID must not be empty", i),
			})
		}
		if svc.Protocol == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-04",
				Message: fmt.Sprintf("p2p service[%d]: protocol must not be empty", i),
			})
		}
		if svc.TopicRef == "" {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-04",
				Message: fmt.Sprintf("p2p service[%d]: topicRef must not be empty", i),
			})
		}
	}

	// Build set of topic service names for cross-referencing.
	topicNames := make(map[string]bool)
	for _, svc := range agentURI.topicServices {
		if svc.Name != "" {
			topicNames[svc.Name] = true
		}
	}
	topicNameList := make([]string, 0, len(topicNames))
	for name := range topicNames {
		topicNameList = append(topicNameList, name)
	}

	// V-REG-05: Each p2p topicRef must match a neuron-topic service name.
	for i, svc := range agentURI.p2pServices {
		if svc.TopicRef != "" && !topicNames[svc.TopicRef] {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-05",
				Message: fmt.Sprintf("p2p service[%d]: topicRef %q not found in topic service names %v", i, svc.TopicRef, topicNameList),
			})
		}
	}

	// V-REG-06: DID (if present) matches childPublicKey.DIDKey().
	expectedDID := childPublicKey.DIDKey()
	for i, svc := range agentURI.didServices {
		if svc.Endpoint != expectedDID {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-06",
				Message: fmt.Sprintf("DID service[%d]: endpoint %q does not match expected %q", i, svc.Endpoint, expectedDID),
			})
		}
	}

	// V-REG-07: At most one DID service.
	if len(agentURI.didServices) > 1 {
		errors = append(errors, ValidationError{
			Rule:    "V-REG-07",
			Message: fmt.Sprintf("at most 1 DID service allowed, got %d", len(agentURI.didServices)),
		})
	}

	// V-REG-11: Each standard channel (stdIn, stdOut, stdErr) appears exactly once.
	channelCount := make(map[string]int)
	for _, svc := range agentURI.topicServices {
		if svc.Channel != "" {
			channelCount[svc.Channel]++
		}
	}
	for _, ch := range StandardChannels() {
		count := channelCount[ch]
		if count == 0 {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-11",
				Message: fmt.Sprintf("standard channel %q is missing", ch),
			})
		} else if count > 1 {
			errors = append(errors, ValidationError{
				Rule:    "V-REG-11",
				Message: fmt.Sprintf("standard channel %q appears %d times, expected exactly 1", ch, count),
			})
		}
	}

	// V-REG-12: Each p2p peerID matches childPublicKey.PeerID().String().
	expectedPeerID, peerErr := childPublicKey.PeerID()
	if peerErr == nil {
		for i, svc := range agentURI.p2pServices {
			if svc.PeerID != "" && svc.PeerID != expectedPeerID.String() {
				errors = append(errors, ValidationError{
					Rule:    "V-REG-12",
					Message: fmt.Sprintf("p2p service[%d]: peerID %q does not match expected %q", i, svc.PeerID, expectedPeerID.String()),
				})
			}
		}
	}

	// V-REG-13: Commerce service delivery cross-reference validation. FR-P01b.
	// Build set of p2p service names for cross-referencing.
	p2pNames := make(map[string]bool)
	for _, svc := range agentURI.p2pServices {
		if svc.Name != "" {
			p2pNames[svc.Name] = true
		}
	}

	for i, svc := range agentURI.commerceServices {
		switch svc.Delivery.Mode {
		case payment.DeliveryModeP2P:
			if svc.Delivery.ServiceRef != "" && !p2pNames[svc.Delivery.ServiceRef] {
				errors = append(errors, ValidationError{
					Rule:    "V-REG-13",
					Message: fmt.Sprintf("commerce service[%d] %q: delivery.serviceRef %q not found in p2p service names", i, svc.Name, svc.Delivery.ServiceRef),
				})
			}
		case payment.DeliveryModeTopic:
			if svc.Delivery.ChannelRef != "" && !topicNames[svc.Delivery.ChannelRef] {
				errors = append(errors, ValidationError{
					Rule:    "V-REG-13",
					Message: fmt.Sprintf("commerce service[%d] %q: delivery.channelRef %q not found in topic service names", i, svc.Name, svc.Delivery.ChannelRef),
				})
			}
		}
	}

	if len(errors) > 0 {
		return false, errors
	}
	return true, nil
}

// ValidateRoleBoundary enforces FR-R11 role-based operation constraints.
// Returns nil if the operation is allowed for the given role, or
// UnauthorizedOperation if the operation is forbidden.
func ValidateRoleBoundary(callerRole RegistryRole, operation string) error {
	switch callerRole {
	case RegistryAdmin:
		// Admin MUST NOT register, updateAgentURI, or transfer.
		switch operation {
		case "register", "updateAgentURI", "transfer":
			return UnauthorizedOperation{
				CallerRole: callerRole.String(),
				Operation:  operation,
			}
		}

	case RegisteredAgent:
		// Agent MUST NOT register for others.
		switch operation {
		case "register":
			return UnauthorizedOperation{
				CallerRole: callerRole.String(),
				Operation:  operation,
			}
		}

	case DelegatedOperator:
		// Operator MUST NOT register.
		switch operation {
		case "register":
			return UnauthorizedOperation{
				CallerRole: callerRole.String(),
				Operation:  operation,
			}
		}

	case ParentRole:
		// Parent MUST NOT register or updateAgentURI.
		switch operation {
		case "register", "updateAgentURI":
			return UnauthorizedOperation{
				CallerRole: callerRole.String(),
				Operation:  operation,
			}
		}

	default:
		return UnauthorizedOperation{
			CallerRole: callerRole.String(),
			Operation:  operation,
		}
	}

	return nil
}
