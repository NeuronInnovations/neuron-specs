package adsb

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// HCS env vars — same names as internal/dapp/remoteid so a single Hedera
// operator key works for both DApps.
const (
	HCSEnvOperatorAccountID  = topic.HederaOperatorEnvAccountID  // "HEDERA_OPERATOR_ID"
	HCSEnvOperatorPrivateKey = topic.HederaOperatorEnvPrivateKey // "HEDERA_OPERATOR_KEY"
)

// HCSBackend wraps a constructed HCS adapter + the three topic refs the
// seller just created.
type HCSBackend struct {
	Adapter     topic.TopicAdapter
	StdInRef    topic.TopicRef
	StdOutRef   topic.TopicRef
	StdErrRef   topic.TopicRef
	OperatorID  string
	OperatorHex string
}

// HCSBackendRole controls which side calls the factory.
type HCSBackendRole int

const (
	HCSRoleSeller HCSBackendRole = iota
	HCSRoleBuyer
)

// HCSBackendOptions configures the HCS factory.
type HCSBackendOptions struct {
	Role            HCSBackendRole
	LookupEnv       func(key string) string
	TopicMemoPrefix string // defaults to "adsb-"
}

// NewHCSBackend constructs the HCS adapter from env vars. Seller-side
// creates three fresh topics with the "adsb-" memo prefix. NEVER falls
// back to the memory adapter — callers MUST exit 2 on missing env vars.
func NewHCSBackend(ctx context.Context, opts HCSBackendOptions) (*HCSBackend, error) {
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.Getenv
	}
	if v := lookup(HCSEnvOperatorAccountID); v == "" {
		return nil, fmt.Errorf("adsb.NewHCSBackend: missing env %s — refusing to fall back to memory; set the Hedera operator account before --topic-backend=hcs",
			HCSEnvOperatorAccountID)
	}
	if v := lookup(HCSEnvOperatorPrivateKey); v == "" {
		return nil, fmt.Errorf("adsb.NewHCSBackend: missing env %s — refusing to fall back to memory; set the Hedera operator private key before --topic-backend=hcs",
			HCSEnvOperatorPrivateKey)
	}

	client, operatorID, err := topic.NewTestnetClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("adsb.NewHCSBackend: %w", err)
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
		memoPrefix = "adsb-"
	}

	stdInRef, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: memoPrefix + "stdin"})
	if err != nil {
		return nil, fmt.Errorf("adsb.NewHCSBackend: create stdIn topic: %w", err)
	}
	stdOutRef, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: memoPrefix + "stdout"})
	if err != nil {
		return nil, fmt.Errorf("adsb.NewHCSBackend: create stdOut topic: %w", err)
	}
	stdErrRef, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: memoPrefix + "stderr"})
	if err != nil {
		return nil, fmt.Errorf("adsb.NewHCSBackend: create stdErr topic: %w", err)
	}

	out.StdInRef = stdInRef
	out.StdOutRef = stdOutRef
	out.StdErrRef = stdErrRef
	_ = ctx
	return out, nil
}

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

// MissingHCSEnvVars is the exported view used by CLI validation tests.
func MissingHCSEnvVars(lookup func(key string) string) []string {
	return missingHCSEnvVars(lookup)
}

// ErrHCSConfigMissing is returned when --topic-backend=hcs is requested
// without the required env vars.
var ErrHCSConfigMissing = errors.New("adsb: HCS topic backend config missing")
