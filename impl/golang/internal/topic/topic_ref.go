package topic

import (
	"strings"
)

// BackendKind identifies the transport backend for a topic.
// Known backends are hcs, erc-log, and kafka. Custom backends use the "custom:" prefix.
type BackendKind string

const (
	// BackendHCS is the Hedera Consensus Service backend (Constitution VIII: primary transport).
	BackendHCS BackendKind = "hcs"
	// BackendERCLog is the ERC event log backend (read-only).
	BackendERCLog BackendKind = "erc-log"
	// BackendKafka is the Kafka backend with optional ledger anchoring.
	BackendKafka BackendKind = "kafka"
)

// knownBackends is the set of built-in backend kinds.
var knownBackends = map[BackendKind]bool{
	BackendHCS:    true,
	BackendERCLog: true,
	BackendKafka:  true,
}

// IsCustomBackend returns true if the BackendKind uses the "custom:" prefix.
func IsCustomBackend(kind BackendKind) bool {
	return strings.HasPrefix(string(kind), "custom:")
}

// ParseBackendKind validates and returns a BackendKind from the given string.
// It accepts known backends (hcs, erc-log, kafka) and custom backends (custom:xxx).
// It returns ErrUnsupportedTransport for empty or unrecognized values.
func ParseBackendKind(s string) (BackendKind, error) {
	if s == "" {
		return "", NewTopicError(ErrUnsupportedTransport, "backend kind must not be empty")
	}

	kind := BackendKind(s)

	if knownBackends[kind] {
		return kind, nil
	}

	if IsCustomBackend(kind) {
		name := strings.TrimPrefix(s, "custom:")
		if name == "" {
			return "", NewTopicError(ErrUnsupportedTransport, "custom backend name must not be empty after 'custom:' prefix")
		}
		return kind, nil
	}

	return "", NewTopicError(ErrUnsupportedTransport, "unknown backend kind: "+s)
}

// backendSchemes maps BackendKind to URI scheme for compact URI representation.
var backendSchemes = map[BackendKind]string{
	BackendHCS:    "hcs",
	BackendERCLog: "erc-log",
	BackendKafka:  "kafka+ledger",
}

// schemeToBackend is the reverse mapping from URI scheme to BackendKind.
var schemeToBackend = map[string]BackendKind{
	"hcs":          BackendHCS,
	"erc-log":      BackendERCLog,
	"kafka+ledger": BackendKafka,
}

// FR-T01: TopicRef composite reference (transport + locator)
// TopicRef is an immutable reference to a topic on a specific backend transport.
// It combines a BackendKind (transport type) with a locator (backend-specific identifier).
type TopicRef struct {
	transport BackendKind
	locator   string
}

// Transport returns the backend transport kind.
func (r *TopicRef) Transport() BackendKind { return r.transport }

// Locator returns the backend-specific locator string.
func (r *TopicRef) Locator() string { return r.locator }

// NewTopicRef constructs and validates a TopicRef.
// Returns ErrInvalidTopicRef if transport is invalid or locator is empty.
func NewTopicRef(transport BackendKind, locator string) (TopicRef, error) {
	ref := TopicRef{
		transport: transport,
		locator:   locator,
	}
	if err := ref.Validate(); err != nil {
		return TopicRef{}, err
	}
	return ref, nil
}

// FR-T12: TopicRef validation (transport registered, locator non-empty)
// Validate checks that the TopicRef has a valid transport and non-empty locator.
func (r TopicRef) Validate() error {
	if r.transport == "" {
		return NewTopicError(ErrInvalidTopicRef, "transport must not be empty")
	}

	// Validate transport is a known or custom backend.
	if !knownBackends[r.transport] && !IsCustomBackend(r.transport) {
		return NewTopicError(ErrInvalidTopicRef, "unknown transport: "+string(r.transport))
	}

	if r.locator == "" {
		return NewTopicError(ErrInvalidTopicRef, "locator must not be empty")
	}

	return nil
}

// URI returns the compact URI representation of the TopicRef.
// Format: scheme://locator (e.g., "hcs://0.0.12345", "erc-log://0x742d...", "kafka+ledger://my-topic").
// Custom backends use the format: custom-name://locator.
func (r TopicRef) URI() string {
	scheme, ok := backendSchemes[r.transport]
	if !ok {
		// For custom backends, strip "custom:" prefix and use as scheme.
		if IsCustomBackend(r.transport) {
			scheme = strings.TrimPrefix(string(r.transport), "custom:")
		} else {
			scheme = string(r.transport)
		}
	}
	return scheme + "://" + r.locator
}

// TopicRefFromURI parses a compact URI string into a TopicRef.
// Supported schemes: hcs://, erc-log://, kafka+ledger://.
// Unknown schemes are treated as custom backends (custom:scheme).
func TopicRefFromURI(uri string) (TopicRef, error) {
	if uri == "" {
		return TopicRef{}, NewTopicError(ErrInvalidTopicRef, "URI must not be empty")
	}

	// Find the "://" separator.
	idx := strings.Index(uri, "://")
	if idx < 0 {
		return TopicRef{}, NewTopicError(ErrInvalidTopicRef, "URI must contain '://' separator: "+uri)
	}

	scheme := uri[:idx]
	locator := uri[idx+3:]

	if scheme == "" {
		return TopicRef{}, NewTopicError(ErrInvalidTopicRef, "URI scheme must not be empty")
	}

	if locator == "" {
		return TopicRef{}, NewTopicError(ErrInvalidTopicRef, "URI locator must not be empty")
	}

	// Look up known scheme mappings.
	if backend, ok := schemeToBackend[scheme]; ok {
		return TopicRef{
			transport: backend,
			locator:   locator,
		}, nil
	}

	// Treat unknown schemes as custom backends.
	return TopicRef{
		transport: BackendKind("custom:" + scheme),
		locator:   locator,
	}, nil
}
