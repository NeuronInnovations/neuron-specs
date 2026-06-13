package registry

// RegistryRole represents the caller's role within an EIP-8004 Identity
// Registry. The role determines which operations the caller is authorized
// to perform.
type RegistryRole int

const (
	// RegistryAdmin is the registry contract owner with full administrative
	// privileges (deploy, configure admission policy, burn any token).
	RegistryAdmin RegistryRole = iota

	// RegisteredAgent is an agent that holds a registration NFT in the
	// registry. Can update its own agentURI and burn its own token.
	RegisteredAgent

	// DelegatedOperator is an address approved to act on behalf of a
	// RegisteredAgent (via ERC-721 approval or setApprovalForAll).
	DelegatedOperator

	// ParentRole is the parent account that controls one or more Child
	// agents. May be required for admission in permissioned registries.
	ParentRole
)

// String returns the canonical uppercase string representation of the role.
func (r RegistryRole) String() string {
	switch r {
	case RegistryAdmin:
		return "REGISTRY_ADMIN"
	case RegisteredAgent:
		return "REGISTERED_AGENT"
	case DelegatedOperator:
		return "DELEGATED_OPERATOR"
	case ParentRole:
		return "PARENT"
	default:
		return "UNKNOWN"
	}
}
