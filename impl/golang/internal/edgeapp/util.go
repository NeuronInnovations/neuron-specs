package edgeapp

import (
	"fmt"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// nopLogger silently discards all log messages.
type nopLogger struct{}

func (nopLogger) Printf(string, ...any) {}

// resolvePersistentTopics is the seller-side persistent-topics path.
//
// When statePath is empty, behavior matches the legacy default — three fresh
// topics are created via ensureTopics every restart, and (nil, nil) is
// returned for the state pair so the caller skips the save step.
//
// When statePath is non-empty, the loader reads the file (if present),
// verifies the persisted identity matches pubHex (case-insensitive), and
// reuses the persisted topic locators. Identity mismatch, missing file,
// schema-version mismatch, or any locator-parse error all fall back to
// fresh-create with no error to the caller. The returned *EdgeState is
// non-nil; the caller is responsible for SaveEdgeState'ing it once the
// agent has reached steady state, and again on graceful shutdown.
//
// fresh reports whether new topics were created (true) vs. reused from
// state (false). Callers can use this to gate "first-startup" side effects
// like profile-descriptor publishing.
func resolvePersistentTopics(
	bus topic.TopicAdapter,
	statePath string,
	evmAddr, pubHex, peerID, agentPrefix string,
) (in, out, errRef topic.TopicRef, state *EdgeState, fresh bool, err error) {
	// Legacy path: stateless, fresh every restart.
	if statePath == "" {
		in, out, errRef, err = ensureTopics(bus, topic.TopicRef{}, topic.TopicRef{}, topic.TopicRef{}, agentPrefix)
		return in, out, errRef, nil, true, err
	}

	loaded, loadErr := LoadEdgeState(statePath)
	if loadErr != nil {
		// Real I/O / parse error — surface so the operator sees it. We don't
		// silently overwrite a corrupt state file; ops should investigate.
		return in, out, errRef, nil, false, fmt.Errorf("load state: %w", loadErr)
	}

	if loaded != nil && loaded.MatchesIdentity(pubHex) {
		ri, ro, re, refErr := loaded.TopicRefs()
		if refErr == nil {
			// Sanity check: ensure the configured backend matches what we
			// persisted. Cross-backend reuse would yield meaningless locators.
			if ri.Transport() == bus.SupportedTransport() {
				return ri, ro, re, loaded, false, nil
			}
		}
	}

	// Either no usable state, or identity mismatch (key rotated), or backend
	// changed. Create fresh topics and build a new state object the caller
	// will persist.
	in, out, errRef, err = ensureTopics(bus, topic.TopicRef{}, topic.TopicRef{}, topic.TopicRef{}, agentPrefix)
	if err != nil {
		return in, out, errRef, nil, true, err
	}
	state = BuildEdgeState(evmAddr, pubHex, peerID, in, out, errRef)
	return in, out, errRef, state, true, nil
}

// ensureTopics returns the three topic refs the caller passed in, creating
// any that are zero-valued via bus.CreateTopic. The agent prefix is used
// only as a memo on newly-created topics — purely informational.
func ensureTopics(bus topic.TopicAdapter, in, out, errRef topic.TopicRef, agentPrefix string) (topic.TopicRef, topic.TopicRef, topic.TopicRef, error) {
	create := func(memo string) (topic.TopicRef, error) {
		return bus.CreateTopic(topic.CreateTopicOpts{
			Transport: bus.SupportedTransport(),
			Memo:      agentPrefix + "-" + memo,
		})
	}

	if in.Locator() == "" {
		ref, err := create("stdIn")
		if err != nil {
			return in, out, errRef, err
		}
		in = ref
	}
	if out.Locator() == "" {
		ref, err := create("stdOut")
		if err != nil {
			return in, out, errRef, err
		}
		out = ref
	}
	if errRef.Locator() == "" {
		ref, err := create("stdErr")
		if err != nil {
			return in, out, errRef, err
		}
		errRef = ref
	}
	return in, out, errRef, nil
}
