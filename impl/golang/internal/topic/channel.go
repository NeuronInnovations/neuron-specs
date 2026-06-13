package topic

import (
	"regexp"
	"strings"
)

// FR-T20: stdIn supports structured signaling
// ChannelRole identifies the role of a channel within a topic-based communication system.
// Standard roles are stdIn, stdOut, and stdErr. Custom channels use the "custom:" prefix.
type ChannelRole string

// FR-T07: Three standard channel roles (stdIn, stdOut, stdErr)
const (
	// ChannelStdIn is the standard input channel role.
	ChannelStdIn ChannelRole = "stdIn"
	// ChannelStdOut is the standard output channel role.
	ChannelStdOut ChannelRole = "stdOut"
	// ChannelStdErr is the standard error channel role.
	ChannelStdErr ChannelRole = "stdErr"
)

// standardRoles is the set of reserved standard channel role names.
var standardRoles = map[ChannelRole]bool{
	ChannelStdIn:  true,
	ChannelStdOut: true,
	ChannelStdErr: true,
}

// customNamePattern validates the name portion after "custom:" prefix.
// Must be non-empty and contain only alphanumeric characters, hyphens, and underscores.
var customNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ChannelRoleFromString parses a string into a ChannelRole.
// Standard roles (stdIn, stdOut, stdErr) are accepted directly.
// Custom channels must use the "custom:" prefix followed by a valid name.
// Non-standard names without the "custom:" prefix are rejected as reserved name collisions.
func ChannelRoleFromString(s string) (ChannelRole, error) {
	if s == "" {
		return "", NewTopicError(ErrReservedChannelName, "channel role must not be empty")
	}

	role := ChannelRole(s)

	// Accept standard roles directly.
	if standardRoles[role] {
		return role, nil
	}

	// Check for custom prefix.
	if strings.HasPrefix(s, "custom:") {
		name := strings.TrimPrefix(s, "custom:")
		if name == "" {
			return "", NewTopicError(ErrReservedChannelName, "custom channel name must not be empty after 'custom:' prefix")
		}
		if !customNamePattern.MatchString(name) {
			return "", NewTopicError(ErrReservedChannelName, "custom channel name must be alphanumeric with hyphens and underscores: "+name)
		}
		return role, nil
	}

	// Reject non-standard names that don't use the custom prefix.
	return "", NewTopicError(ErrReservedChannelName, "non-standard channel name must use 'custom:' prefix: "+s)
}

// StandardChannelRoles returns all standard channel roles in order: stdIn, stdOut, stdErr.
func StandardChannelRoles() []ChannelRole {
	return []ChannelRole{ChannelStdIn, ChannelStdOut, ChannelStdErr}
}

// FR-T08: Custom channels with custom:<name> prefix
// IsCustomChannel returns true if the ChannelRole uses the "custom:" prefix.
func IsCustomChannel(role ChannelRole) bool {
	return strings.HasPrefix(string(role), "custom:")
}

// String returns the string representation of the ChannelRole.
func (r ChannelRole) String() string {
	return string(r)
}
