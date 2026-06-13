package remoteid

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// HCSBackendEnv enumerates the env vars the HCS factory consults. They
// mirror the contract that `cmd/buyer-seller-demo` already uses, so an
// operator who has run that demo can re-use the same `.env` here.
const (
	// HCSEnvOperatorAccountID — Hedera operator account id (format "0.0.X").
	// Required for --topic-backend=hcs.
	HCSEnvOperatorAccountID = topic.HederaOperatorEnvAccountID // "HEDERA_OPERATOR_ID"

	// HCSEnvOperatorPrivateKey — ECDSA secp256k1 private key (hex / DER).
	// Required for --topic-backend=hcs. NEVER logged or printed.
	HCSEnvOperatorPrivateKey = topic.HederaOperatorEnvPrivateKey // "HEDERA_OPERATOR_KEY"
)

// HCSBackend wraps a constructed HCS adapter + the topic refs the seller
// just created. The CLI uses the topic refs to populate the AgentURI's
// `NeuronTopicService.Config["topicId"]` so the buyer can resolve them
// via the registry lookup.
type HCSBackend struct {
	Adapter      topic.TopicAdapter
	StdInRef     topic.TopicRef
	StdOutRef    topic.TopicRef
	StdErrRef    topic.TopicRef
	OperatorID   string // "0.0.X" — captured for evidence logging
	OperatorHex  string // hex-encoded operator EVM address; "" when SDK doesn't expose it
}

// HCSBackendRole controls which side calls the factory. Seller-side
// runs `CreateTopic` to allocate three fresh HCS topics; buyer-side
// only constructs the adapter and skips topic creation (the buyer
// resolves topic refs out of the seller's registered AgentURI).
type HCSBackendRole int

const (
	// HCSRoleSeller asks the factory to create three new topics.
	HCSRoleSeller HCSBackendRole = iota

	// HCSRoleBuyer asks the factory to only construct the adapter.
	HCSRoleBuyer
)

// HCSBackendOptions configures the HCS factory.
type HCSBackendOptions struct {
	// Role selects seller (create-3-topics) vs buyer (adapter-only).
	Role HCSBackendRole

	// LookupEnv is the env-var lookup function. Defaults to os.Getenv.
	// Tests inject a closure that returns deterministic values without
	// touching process env.
	LookupEnv func(key string) string

	// TopicMemoPrefix is prefixed to each of the three topic memos so
	// HashScan-style explorers can group a seller's stdIn/stdOut/stdErr
	// together. Defaults to "remoteid-".
	TopicMemoPrefix string
}

// NewHCSBackend constructs the HCS adapter from env vars (and, on the
// seller side, creates stdIn/stdOut/stdErr topics). Returns an explicit
// error listing the missing env var names — callers MUST exit 2 with
// the error visible to the operator. The function NEVER falls back to
// the memory adapter.
//
// Required env vars: HEDERA_OPERATOR_ID, HEDERA_OPERATOR_KEY (both
// consumed by `topic.NewTestnetClientFromEnv` per existing buyer-seller
// demo contract).
//
// FR anchors:
//   - 004 FR-T*: TopicAdapter contract.
//   - 008 FR-P58: heartbeat capability `commerceMode` flows through the
//     orchestrator; the backend choice is invisible to the protocol.
func NewHCSBackend(ctx context.Context, opts HCSBackendOptions) (*HCSBackend, error) {
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.Getenv
	}
	if v := lookup(HCSEnvOperatorAccountID); v == "" {
		return nil, fmt.Errorf("remoteid.NewHCSBackend: missing env %s — refusing to fall back to memory; set the Hedera operator account before --topic-backend=hcs",
			HCSEnvOperatorAccountID)
	}
	if v := lookup(HCSEnvOperatorPrivateKey); v == "" {
		return nil, fmt.Errorf("remoteid.NewHCSBackend: missing env %s — refusing to fall back to memory; set the Hedera operator private key before --topic-backend=hcs",
			HCSEnvOperatorPrivateKey)
	}

	// topic.NewTestnetClientFromEnv reads HEDERA_OPERATOR_ID +
	// HEDERA_OPERATOR_KEY internally and returns a configured Hiero
	// SDK client. We re-check the env above so a misconfigured caller
	// gets the exact env var name in the error.
	client, operatorID, err := topic.NewTestnetClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("remoteid.NewHCSBackend: %w", err)
	}

	adapter := topic.NewHCSAdapter(topic.NewRealHCSClient(client))
	out := &HCSBackend{
		Adapter:    adapter,
		OperatorID: operatorID.String(),
	}

	if opts.Role == HCSRoleBuyer {
		return out, nil
	}

	memoPrefix := opts.TopicMemoPrefix
	if memoPrefix == "" {
		memoPrefix = "remoteid-"
	}

	stdInRef, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: memoPrefix + "stdin"})
	if err != nil {
		return nil, fmt.Errorf("remoteid.NewHCSBackend: create stdIn topic: %w", err)
	}
	stdOutRef, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: memoPrefix + "stdout"})
	if err != nil {
		return nil, fmt.Errorf("remoteid.NewHCSBackend: create stdOut topic: %w", err)
	}
	stdErrRef, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: memoPrefix + "stderr"})
	if err != nil {
		return nil, fmt.Errorf("remoteid.NewHCSBackend: create stdErr topic: %w", err)
	}

	out.StdInRef = stdInRef
	out.StdOutRef = stdOutRef
	out.StdErrRef = stdErrRef
	_ = ctx // reserved for future cancellation through the SDK; topic creation is synchronous in the current adapter
	return out, nil
}

// missingHCSEnvVars returns the list of HCS env var names the lookup
// did not satisfy. Used by CLI validation tests to assert the
// no-silent-fallback exit-2 message before constructing the backend.
func missingHCSEnvVars(lookup func(key string) string) []string {
	if lookup == nil {
		lookup = os.Getenv
	}
	var missing []string
	if lookup(HCSEnvOperatorAccountID) == "" {
		missing = append(missing, HCSEnvOperatorAccountID)
	}
	if lookup(HCSEnvOperatorPrivateKey) == "" {
		missing = append(missing, HCSEnvOperatorPrivateKey)
	}
	return missing
}

// MissingHCSEnvVars is the exported view of the env-var validator. The
// seller / buyer CLI calls it before construction to bail with exit 2.
func MissingHCSEnvVars(lookup func(key string) string) []string {
	return missingHCSEnvVars(lookup)
}

// ErrHCSConfigMissing is returned when --topic-backend=hcs is requested
// without the required env vars. Wrapped through `errors.Is` so callers
// can branch on it without string-matching.
var ErrHCSConfigMissing = errors.New("remoteid: HCS topic backend config missing")
