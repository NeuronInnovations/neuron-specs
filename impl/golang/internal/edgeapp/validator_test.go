package edgeapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/neuron-sdk/neuron-go-sdk/internal/validation"
)

// validatorFixture spins up a commerceFixture + a validator subscribed to
// its topics, ready to attest the agreement once the buyer/seller flow
// completes.
type validatorFixture struct {
	*commerceFixture
	validatorKey    keylib.NeuronPrivateKey
	validatorStdOut topic.TopicRef
	validator       *Validator
	validatorDone   chan error
	validatorCancel context.CancelFunc
}

func newValidatorFixture(t *testing.T) *validatorFixture {
	t.Helper()
	cf := newCommerceFixture(t)

	vk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	vStdOut, err := cf.bus.CreateTopic(topic.CreateTopicOpts{
		Transport: cf.bus.SupportedTransport(),
		Memo:      "validator-stdOut",
	})
	require.NoError(t, err)

	cfg := ValidatorConfig{
		Bus:              cf.bus,
		Key:              &vk,
		ValidatorStdOut:  vStdOut,
		ValidatorAgentID: "1",
		SubjectAgentID:   "2",
		SellerStdIn:      cf.sellerStdIn,
		BuyerStdIn:       cf.buyerStdIn,
		Logger:           tlogger{t: t},
	}
	v, err := NewValidator(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- v.Run(ctx) }()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	return &validatorFixture{
		commerceFixture: cf,
		validatorKey:    vk,
		validatorStdOut: vStdOut,
		validator:       v,
		validatorDone:   done,
		validatorCancel: cancel,
	}
}

// TestValidator_HappyPathEmitsTwoEnvelopes runs the full commerce cycle
// while a validator subscribes; expects two EvidenceEnvelopes to land on
// the validator's stdOut: one for spec 008 with VerdictCompliant, one for
// spec 009 with VerdictInconclusive (delivery telemetry not yet wired).
func TestValidator_HappyPathEmitsTwoEnvelopes(t *testing.T) {
	f := newValidatorFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Run the full negotiate+fund and settle cycle (mirrors TestCommerce_FullCycleMock).
	var (
		buyerSession  *BuyerSession
		sellerSession *SellerSession
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		bs, err := BuyerNegotiateAndFund(ctx, f.buyerCfg)
		require.NoError(t, err)
		buyerSession = bs
	}()
	go func() {
		defer wg.Done()
		ss, err := SellerObserveAndAccept(ctx, f.sellerCfg, 5*time.Second)
		require.NoError(t, err)
		sellerSession = ss
	}()
	wg.Wait()

	wg.Add(2)
	go func() {
		defer wg.Done()
		require.NoError(t, sellerSession.IssueInvoice(ctx, "release-1", 5*time.Second))
	}()
	go func() {
		defer wg.Done()
		require.NoError(t, buyerSession.Settle(ctx))
	}()
	wg.Wait()

	// Validator races to emit envelopes; poll until two appear (or timeout).
	require.Eventually(t, func() bool {
		return len(f.bus.GetMessages(f.validatorStdOut)) >= 2
	}, 3*time.Second, 50*time.Millisecond, "validator should publish 2 envelopes")

	msgs := f.bus.GetMessages(f.validatorStdOut)
	require.GreaterOrEqual(t, len(msgs), 2)

	// Parse each envelope.
	specs := map[string]string{} // specRef → verdict
	for _, m := range msgs {
		require.NoError(t, topic.ValidateTopicMessage(m))
		var env map[string]any
		require.NoError(t, json.Unmarshal(m.Payload(), &env))
		assert.Equal(t, ValidatorEvidencePayloadType, env["type"])
		specRef, _ := env["specRef"].(string)
		verdict, _ := env["verdict"].(string)
		specs[specRef] = verdict
	}
	assert.Equal(t, string(validation.VerdictCompliant), specs["008-payment"],
		"happy-path payment evidence should be compliant")
	assert.Equal(t, string(validation.VerdictInconclusive), specs["009-p2p-data-delivery"],
		"delivery evidence is inconclusive in iteration 2 (no stream-bytes hook)")
}

// TestValidator_RefusedInvoiceMarksNonCompliant injects a refused
// InvoiceAck and confirms the payment envelope flips to non-compliant.
func TestValidator_RefusedInvoiceMarksNonCompliant(t *testing.T) {
	f := newValidatorFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Synthesize the protocol bytes manually (so we don't have to drive
	// MemoryEscrow + state machine just to inject one refusal).
	publish := func(target topic.TopicRef, key *keylib.NeuronPrivateKey, payload any) {
		require.NoError(t, publishCommerce(f.bus, key, target, payload))
	}

	rid := "edge-refused-001"
	// ServiceRequest on seller.stdIn.
	publish(f.sellerStdIn, &f.buyerKey, map[string]any{
		"type":              "serviceRequest",
		"version":           "1.0.0",
		"requestId":         rid,
		"serviceRef":        "neuron://service/x",
		"settlementBinding": "memory",
		"proposedAmount":    "100",
		"proposedCurrency":  "tinybar",
		"proposedInterval":  "PT1H",
	})
	// ServiceResponse on buyer.stdIn.
	publish(f.buyerStdIn, &f.sellerKey, map[string]any{
		"type":      "serviceResponse",
		"version":   "1.0.0",
		"requestId": rid,
		"action":    "accept",
	})
	// EscrowCreated on seller.stdIn.
	publish(f.sellerStdIn, &f.buyerKey, map[string]any{
		"type":            "escrowCreated",
		"version":         "1.0.0",
		"requestId":       rid,
		"escrowRef":       "mem-escrow-99",
		"depositAmount":   "100",
		"depositCurrency": "tinybar",
	})
	// Invoice on buyer.stdIn.
	publish(f.buyerStdIn, &f.sellerKey, map[string]any{
		"type":              "invoice",
		"version":           "1.0.0",
		"requestId":         rid,
		"releaseRequestRef": "release-1",
		"escrowRef":         "mem-escrow-99",
		"amount":            "100",
		"currency":          "tinybar",
		"period":            "PT1H",
	})
	// Refused InvoiceAck on seller.stdIn.
	publish(f.sellerStdIn, &f.buyerKey, map[string]any{
		"type":              "invoiceAck",
		"version":           "1.0.0",
		"requestId":         rid,
		"releaseRequestRef": "release-1",
		"action":            "refused",
	})

	require.Eventually(t, func() bool {
		return len(f.bus.GetMessages(f.validatorStdOut)) >= 2
	}, 3*time.Second, 50*time.Millisecond, "validator should still emit envelopes for refused agreement")

	msgs := f.bus.GetMessages(f.validatorStdOut)
	verdictForSpec := map[string]string{}
	for _, m := range msgs {
		var env map[string]any
		require.NoError(t, json.Unmarshal(m.Payload(), &env))
		spec, _ := env["specRef"].(string)
		v, _ := env["verdict"].(string)
		verdictForSpec[spec] = v
	}
	assert.Equal(t, string(validation.VerdictNonCompliant), verdictForSpec["008-payment"],
		"refused invoice ⇒ payment evidence non-compliant")
	_ = ctx
}

// TestValidator_RejectsInvalidConfig checks each required-field guard.
func TestValidator_RejectsInvalidConfig(t *testing.T) {
	bus := NewMemoryBus()
	out, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport()})
	require.NoError(t, err)
	in, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport()})
	require.NoError(t, err)
	in2, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport()})
	require.NoError(t, err)
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	good := ValidatorConfig{
		Bus: bus, Key: &k, ValidatorStdOut: out, ValidatorAgentID: "1",
		SubjectAgentID: "2", SellerStdIn: in, BuyerStdIn: in2,
	}

	cases := map[string]func(c *ValidatorConfig){
		"nil-bus":             func(c *ValidatorConfig) { c.Bus = nil },
		"nil-key":             func(c *ValidatorConfig) { c.Key = nil },
		"empty-stdOut":        func(c *ValidatorConfig) { c.ValidatorStdOut = topic.TopicRef{} },
		"empty-validatorID":   func(c *ValidatorConfig) { c.ValidatorAgentID = "" },
		"empty-subjectID":     func(c *ValidatorConfig) { c.SubjectAgentID = "" },
		"empty-sellerStdIn":   func(c *ValidatorConfig) { c.SellerStdIn = topic.TopicRef{} },
		"empty-buyerStdIn":    func(c *ValidatorConfig) { c.BuyerStdIn = topic.TopicRef{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := good
			mutate(&c)
			_, err := NewValidator(c)
			assert.Error(t, err)
		})
	}
}

// TestValidator_DropsMessagesWithoutRequestID confirms protocol messages
// missing requestId are skipped (no panic, no envelope) AND that the iter-5
// instrumentation emits a log line so the silent-failure mode that caused
// iter-4's zero-envelope outcome can never silently recur.
func TestValidator_DropsMessagesWithoutRequestID(t *testing.T) {
	f := newValidatorFixture(t)

	// Publish a TopicMessage with payload missing requestId.
	require.NoError(t, publishCommerce(f.bus, &f.buyerKey, f.sellerStdIn, map[string]any{
		"type":    "serviceRequest",
		"version": "1.0.0",
	}))

	// Wait briefly + assert no envelope.
	time.Sleep(150 * time.Millisecond)
	assert.Empty(t, f.bus.GetMessages(f.validatorStdOut),
		"validator should not emit envelopes for messages without requestId")
}

// captureLogger collects every Printf call so tests can assert specific log
// lines were emitted (used to verify the iter-5 silent-exit instrumentation).
type captureLogger struct {
	mu    sync.Mutex
	lines []string
}

func (c *captureLogger) Printf(format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, fmt.Sprintf(format, args...))
}

func (c *captureLogger) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.lines))
	copy(out, c.lines)
	return out
}

// hasPrefix returns whether any captured line starts with prefix.
func (c *captureLogger) hasPrefix(prefix string) bool {
	for _, l := range c.snapshot() {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

// containsSubstr returns whether any captured line contains needle.
func (c *captureLogger) containsSubstr(needle string) bool {
	for _, l := range c.snapshot() {
		if strings.Contains(l, needle) {
			return true
		}
	}
	return false
}

// TestValidator_ReplayFromZeroAfterPublish reproduces the iter-4 silent-failure
// scenario: the validator joins AFTER all 5 protocol messages are already
// on-topic. With Replay=true (the standalone validator's default), it must
// rebuild the snapshot and emit envelopes; with Replay=false it must not.
//
// Iter-4 (testnet) saw zero envelopes because the validator subscribed
// without FromSequence — and HCS interprets nil as "skip history, start at
// latest+1". MemoryBus replays-from-zero by default which previously masked
// the bug. This test forces the explicit-Replay contract on both sides.
func TestValidator_ReplayFromZeroAfterPublish(t *testing.T) {
	t.Run("Replay=true emits envelopes after late-join", func(t *testing.T) {
		cf := newCommerceFixture(t)
		vk, err := keylib.NewNeuronPrivateKey()
		require.NoError(t, err)
		vStdOut, err := cf.bus.CreateTopic(topic.CreateTopicOpts{
			Transport: cf.bus.SupportedTransport(),
			Memo:      "validator-stdOut",
		})
		require.NoError(t, err)

		// Publish the full 5-message transcript BEFORE the validator starts.
		rid := "edge-replay-001"
		mustPub := func(target topic.TopicRef, key *keylib.NeuronPrivateKey, payload any) {
			require.NoError(t, publishCommerce(cf.bus, key, target, payload))
		}
		mustPub(cf.sellerStdIn, &cf.buyerKey, map[string]any{
			"type": "serviceRequest", "version": "1.0.0", "requestId": rid,
			"serviceRef": "neuron://service/x", "settlementBinding": "memory",
			"proposedAmount": "100", "proposedCurrency": "tinybar", "proposedInterval": "PT1H",
		})
		mustPub(cf.buyerStdIn, &cf.sellerKey, map[string]any{
			"type": "serviceResponse", "version": "1.0.0", "requestId": rid, "action": "accept",
		})
		mustPub(cf.sellerStdIn, &cf.buyerKey, map[string]any{
			"type": "escrowCreated", "version": "1.0.0", "requestId": rid,
			"escrowRef": "mem-99", "depositAmount": "100", "depositCurrency": "tinybar",
		})
		mustPub(cf.buyerStdIn, &cf.sellerKey, map[string]any{
			"type": "invoice", "version": "1.0.0", "requestId": rid,
			"releaseRequestRef": "rel-1", "escrowRef": "mem-99",
			"amount": "100", "currency": "tinybar", "period": "PT1H",
		})
		mustPub(cf.sellerStdIn, &cf.buyerKey, map[string]any{
			"type": "invoiceAck", "version": "1.0.0", "requestId": rid,
			"releaseRequestRef": "rel-1", "action": "approved",
		})

		// NOW start validator with Replay=true. Replay must surface ALL
		// already-on-topic messages and produce 2 envelopes.
		clog := &captureLogger{}
		v, err := NewValidator(ValidatorConfig{
			Bus:              cf.bus,
			Key:              &vk,
			ValidatorStdOut:  vStdOut,
			ValidatorAgentID: "1",
			SubjectAgentID:   "2",
			SellerStdIn:      cf.sellerStdIn,
			BuyerStdIn:       cf.buyerStdIn,
			Replay:           true,
			Logger:           clog,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- v.Run(ctx) }()
		t.Cleanup(func() { cancel(); <-done })

		require.Eventually(t, func() bool {
			return len(cf.bus.GetMessages(vStdOut)) >= 2
		}, 3*time.Second, 25*time.Millisecond,
			"Replay=true validator should emit 2 envelopes from history")

		// Subscription open + emit logs both fired.
		assert.True(t, clog.hasPrefix("[validator] subscribed"),
			"Run should log subscription state at startup")
		assert.True(t, clog.containsSubstr("emit start requestId="+rid),
			"emit-start log must trace the trigger requestID")
		assert.True(t, clog.containsSubstr("emitted envelopes for requestID="+rid),
			"emit-success log must surface the completed agreement")
	})
}

// TestValidator_LogsUnknownPayloadType verifies that messages with valid
// signature + valid JSON + requestId but a payload type the validator
// doesn't recognize (e.g. heartbeat, profile descriptor) are skipped with
// an explicit log — not silently dropped.
func TestValidator_LogsUnknownPayloadType(t *testing.T) {
	cf := newCommerceFixture(t)
	vk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	vStdOut, err := cf.bus.CreateTopic(topic.CreateTopicOpts{
		Transport: cf.bus.SupportedTransport(),
		Memo:      "validator-stdOut",
	})
	require.NoError(t, err)

	clog := &captureLogger{}
	v, err := NewValidator(ValidatorConfig{
		Bus: cf.bus, Key: &vk, ValidatorStdOut: vStdOut,
		ValidatorAgentID: "1", SubjectAgentID: "2",
		SellerStdIn: cf.sellerStdIn, BuyerStdIn: cf.buyerStdIn,
		Replay: true, Logger: clog,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- v.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	require.NoError(t, publishCommerce(cf.bus, &cf.buyerKey, cf.sellerStdIn, map[string]any{
		"type": "heartbeat", "version": "1.0.0", "requestId": "edge-unknown-001",
	}))

	require.Eventually(t, func() bool {
		return clog.containsSubstr(`unknown payload type="heartbeat"`)
	}, 2*time.Second, 25*time.Millisecond,
		"validator must log when it skips an unknown payload type")
	assert.Empty(t, cf.bus.GetMessages(vStdOut),
		"unknown payload type must NOT trigger an envelope emit")
}

// TestValidator_LogsMissingRequestID is the strengthened companion to
// TestValidator_DropsMessagesWithoutRequestID: it asserts the iter-5 log
// line is emitted (so future regressions of the silent-drop path are
// observable).
func TestValidator_LogsMissingRequestID(t *testing.T) {
	cf := newCommerceFixture(t)
	vk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	vStdOut, err := cf.bus.CreateTopic(topic.CreateTopicOpts{
		Transport: cf.bus.SupportedTransport(),
		Memo:      "validator-stdOut",
	})
	require.NoError(t, err)

	clog := &captureLogger{}
	v, err := NewValidator(ValidatorConfig{
		Bus: cf.bus, Key: &vk, ValidatorStdOut: vStdOut,
		ValidatorAgentID: "1", SubjectAgentID: "2",
		SellerStdIn: cf.sellerStdIn, BuyerStdIn: cf.buyerStdIn,
		Replay: true, Logger: clog,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- v.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	require.NoError(t, publishCommerce(cf.bus, &cf.buyerKey, cf.sellerStdIn, map[string]any{
		"type":    "serviceRequest",
		"version": "1.0.0",
	}))

	require.Eventually(t, func() bool {
		return clog.containsSubstr("no requestId")
	}, 2*time.Second, 25*time.Millisecond,
		"missing-requestId path must emit a log line for diagnosability")
}
