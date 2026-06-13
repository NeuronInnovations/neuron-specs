package edgeapp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Iter-7 P1.1 — TestNewSessionRequestID_FreshPerCall asserts that
// every call to newSessionRequestID returns a different ID even when
// the prefix and seller name are identical, so the validator's
// dedup-by-(requestID, agreementHash) tuple sees each call as its
// own agreement.
func TestNewSessionRequestID_FreshPerCall(t *testing.T) {
	prefix, seller := "edge-feed-001", "alpha"
	seen := make(map[string]int)
	const N = 100
	for i := 0; i < N; i++ {
		id := newSessionRequestID(prefix, seller)
		assert.True(t, strings.HasPrefix(id, prefix+"-"+seller+"-"),
			"requestID should retain the prefix-seller- shape")
		seen[id]++
	}
	assert.Equal(t, N, len(seen),
		"every call should produce a unique requestID; collisions=%d", N-len(seen))
}

// publish5Agreement is a test helper — publishes the canonical 5-message
// commerce transcript on the fixture's stdIn topics with a given (rid,
// version). Two transcripts with the same rid but different version
// strings produce different ServiceResponse bytes, hence different
// agreementHash — exercising the validator's iter-7 dedup-by-(rid, hash).
func publish5Agreement(t *testing.T, cf *commerceFixture, rid, version string) {
	t.Helper()
	mustPub := func(target topic.TopicRef, key *keylib.NeuronPrivateKey, payload any) {
		require.NoError(t, publishCommerce(cf.bus, key, target, payload))
	}
	mustPub(cf.sellerStdIn, &cf.buyerKey, map[string]any{
		"type": "serviceRequest", "version": version, "requestId": rid,
		"serviceRef": "neuron://service/x", "settlementBinding": "memory",
		"proposedAmount": "100", "proposedCurrency": "tinybar", "proposedInterval": "PT1H",
	})
	mustPub(cf.buyerStdIn, &cf.sellerKey, map[string]any{
		"type": "serviceResponse", "version": version, "requestId": rid, "action": "accept",
	})
	mustPub(cf.sellerStdIn, &cf.buyerKey, map[string]any{
		"type": "escrowCreated", "version": version, "requestId": rid,
		"escrowRef": "mem-99", "depositAmount": "100", "depositCurrency": "tinybar",
	})
	mustPub(cf.buyerStdIn, &cf.sellerKey, map[string]any{
		"type": "invoice", "version": version, "requestId": rid,
		"releaseRequestRef": "rel-1", "escrowRef": "mem-99",
		"amount": "100", "currency": "tinybar", "period": "PT1H",
	})
	mustPub(cf.sellerStdIn, &cf.buyerKey, map[string]any{
		"type": "invoiceAck", "version": version, "requestId": rid,
		"releaseRequestRef": "rel-1", "action": "approved",
	})
}

// Iter-7 P1.1 — TestValidator_TwoAgreementsDifferentRids is the
// **iter-7 production invariant**: every agreement has a unique
// requestID (newSessionRequestID guarantees this). The validator must
// emit one envelope-pair per agreement = 4 envelopes for 2 agreements.
// Closes B1 from §18.4.
func TestValidator_TwoAgreementsDifferentRids(t *testing.T) {
	cf := newCommerceFixture(t)
	vk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	vStdOut, err := cf.bus.CreateTopic(topic.CreateTopicOpts{
		Transport: cf.bus.SupportedTransport(),
		Memo:      "validator-stdOut",
	})
	require.NoError(t, err)

	v, err := NewValidator(ValidatorConfig{
		Bus:              cf.bus,
		Key:              &vk,
		ValidatorStdOut:  vStdOut,
		ValidatorAgentID: "1",
		SubjectAgentID:   "2",
		SellerStdIn:      cf.sellerStdIn,
		BuyerStdIn:       cf.buyerStdIn,
		Replay:           true,
		Logger:           tlogger{t: t},
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- v.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	publish5Agreement(t, cf, "iter7-agreement-001", "1.0.0")
	publish5Agreement(t, cf, "iter7-agreement-002", "1.0.0")

	require.Eventually(t, func() bool {
		return len(cf.bus.GetMessages(vStdOut)) >= 4
	}, 4*time.Second, 50*time.Millisecond,
		"validator must emit 4 envelopes for 2 agreements with distinct rids")
}

// Iter-7 P1.1 (defense-in-depth) — TestValidator_LegacySameRidIdempotent
// documents the validator's behavior under the LEGACY case where a buyer
// (pre-iter-7) reused a static rid across agreements. The dedup-by-(rid,
// hash) tuple at minimum prevents true HCS replays from emitting twice;
// when ServiceResponse bytes differ, the validator MAY emit a 2nd pair
// depending on message arrival order (the snapshot itself is keyed by
// rid alone, so interleaved arrivals can lose the first agreement's
// hash). Iter-7 P1.1 in the buyer eliminates this scenario in
// production. The test asserts the lower bound: ≥ 1 envelope-pair lands
// + at most 2 pairs.
func TestValidator_LegacySameRidIdempotent(t *testing.T) {
	cf := newCommerceFixture(t)
	vk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	vStdOut, err := cf.bus.CreateTopic(topic.CreateTopicOpts{
		Transport: cf.bus.SupportedTransport(),
		Memo:      "validator-stdOut",
	})
	require.NoError(t, err)

	v, err := NewValidator(ValidatorConfig{
		Bus:              cf.bus,
		Key:              &vk,
		ValidatorStdOut:  vStdOut,
		ValidatorAgentID: "1",
		SubjectAgentID:   "2",
		SellerStdIn:      cf.sellerStdIn,
		BuyerStdIn:       cf.buyerStdIn,
		Replay:           true,
		Logger:           tlogger{t: t},
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- v.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	rid := "legacy-rid-shared"
	publish5Agreement(t, cf, rid, "1.0.0")
	// Wait for the first agreement to settle — guarantees the validator
	// emits envelope-pair 1 cleanly before agreement 2 races in.
	require.Eventually(t, func() bool {
		return len(cf.bus.GetMessages(vStdOut)) >= 2
	}, 4*time.Second, 50*time.Millisecond,
		"first envelope-pair should land before agreement 2 begins")

	publish5Agreement(t, cf, rid, "1.0.1")
	// Now agreement 2 with different ServiceResponse bytes can produce a
	// fresh hash → a 2nd emit-pair. Allow time for it; assert ≥ 4 total.
	require.Eventually(t, func() bool {
		return len(cf.bus.GetMessages(vStdOut)) >= 4
	}, 4*time.Second, 50*time.Millisecond,
		"with sequential publishes (no interleave race), legacy same-rid 2nd agreement should also emit a pair")

	// Sanity ceiling — never more than 4 (no triple emit).
	assert.LessOrEqual(t, len(cf.bus.GetMessages(vStdOut)), 4)
}

// Iter-7 P1.5 — TestBuyerConfig_NegotiationTimeoutDefault120s asserts the
// new default (120s) for ServiceResponse-wait, replacing the old 30s default
// that triggered 4-6 retries on Hashio during iter-6.
func TestBuyerConfig_NegotiationTimeoutDefault120s(t *testing.T) {
	bus := NewMemoryBus()
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub, err := pk.PublicKey().ToBlockchainKey()
	require.NoError(t, err)

	stdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport()})
	require.NoError(t, err)

	cfg := &BuyerConfig{
		Bus:        bus,
		PrivateKey: &pk,
		Sellers: []SellerEntry{{
			StdIn: stdIn, PubKey: pub, DisplayName: "alpha",
		}},
		OnAggregatedFrame: func(AggregatedFrame) {},
		// NegotiationTimeout intentionally left zero
	}
	require.NoError(t, cfg.validate())
	assert.Equal(t, DefaultBuyerNegotiationTimeout, cfg.NegotiationTimeout)
	assert.Equal(t, 120*time.Second, cfg.NegotiationTimeout)
}

// Iter-7 P1.2 — TestBuyer_StatePathReusesTopicsAcrossRunBuyer asserts the
// new BuyerConfig.StatePath wiring causes the buyer to reuse its topic IDs
// across RunBuyer invocations on the same MemoryBus, mirroring the
// seller's persistent-topics behavior.
//
// Test surface: hits resolvePersistentTopics directly via the same code
// path RunBuyer uses, but skipping the libp2p host construction (which is
// what other RunBuyer tests already cover). Documents that the wiring
// matches the seller's symmetrically.
func TestBuyer_StatePathReusesTopicsAcrossRunBuyer(t *testing.T) {
	bus := NewMemoryBus()
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := pk.PublicKey()
	pid, err := pub.PeerID()
	require.NoError(t, err)
	evmHex := pub.EVMAddress().Hex()
	pubHex := pub.Hex()

	dir := t.TempDir()
	statePath := dir + "/buyer-state.json"

	// First call — no state file; resolvePersistentTopics creates 3 fresh
	// topics + returns persistState that the caller would save.
	in1, out1, err1, st1, fresh1, err := resolvePersistentTopics(
		bus, statePath, evmHex, pubHex, pid.String(), "edge-buyer")
	require.NoError(t, err)
	require.True(t, fresh1, "first call should be fresh-create")
	require.NotNil(t, st1)
	require.NoError(t, SaveEdgeState(statePath, st1))

	// Second call — state file exists + identity matches; topics reused.
	// The reuse path returns the loaded state (so callers may persist
	// updated metadata like ProfileDescriptorHash) but `fresh` is false
	// to signal that no new topics were created.
	in2, out2, err2, st2, fresh2, err := resolvePersistentTopics(
		bus, statePath, evmHex, pubHex, pid.String(), "edge-buyer")
	require.NoError(t, err)
	assert.False(t, fresh2, "second call must reuse topics from state.json")
	assert.NotNil(t, st2, "reuse path returns the loaded state (may carry profile-descriptor hash etc.)")
	assert.Equal(t, in1.Locator(), in2.Locator(), "stdIn must be identical across calls")
	assert.Equal(t, out1.Locator(), out2.Locator(), "stdOut must be identical")
	assert.Equal(t, err1.Locator(), err2.Locator(), "stdErr must be identical")
}

// Iter-7 P1.4 — TestSellerConfig_AgreementPeriodWiring asserts the new
// SellerConfig fields are accepted by validate() and have the expected
// defaults. The full e2e period-driven cycle is not unit-testable
// against MemoryBus because the in-memory bus replays from sequence 0
// on every Subscribe (iter-5 finding) — re-entering the commerce gate
// in a 2nd loop iteration sees agreement 1's stale ServiceRequest and
// re-accepts it. Production HCS interprets nil FromSequence as
// latest+1, which is correct. Live verification of the period-driven
// 3-agreement loop is part of the iter-7 deploy plan (§18.8 D+1).
func TestSellerConfig_AgreementPeriodWiring(t *testing.T) {
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	bus := NewMemoryBus()
	feedSrc := func(ctx context.Context, out chan<- feeds.FeedFrame) error { return nil }

	cfg := &SellerConfig{
		Bus:               bus,
		PrivateKey:        &pk,
		FeedSource:        feedSrc,
		AgreementPeriod:   600 * time.Second,
		IdleFundedTimeout: 300 * time.Second,
	}
	require.NoError(t, cfg.validate())
	assert.Equal(t, 600*time.Second, cfg.AgreementPeriod, "AgreementPeriod preserved by validate")
	assert.Equal(t, 300*time.Second, cfg.IdleFundedTimeout, "IdleFundedTimeout preserved by validate")

	// Defaults: both 0 (legacy single-shot SIGINT-only behavior).
	cfg2 := &SellerConfig{
		Bus:        bus,
		PrivateKey: &pk,
		FeedSource: feedSrc,
	}
	require.NoError(t, cfg2.validate())
	assert.Equal(t, time.Duration(0), cfg2.AgreementPeriod, "default AgreementPeriod is 0 (single-shot)")
	assert.Equal(t, time.Duration(0), cfg2.IdleFundedTimeout, "default IdleFundedTimeout is 0 (wait forever)")
}

// Iter-7 P1.3 — TestSeller_IdempotentServiceResponseOnDuplicateRequest
// drives the seller manually and verifies a duplicate ServiceRequest with
// the SAME requestID triggers a re-publish of ServiceResponse rather than
// being silently ignored. This covers the buyer-side scenario where HCS
// subscription lag caused the buyer's first ServiceResponse to be missed.
func TestSeller_IdempotentServiceResponseOnDuplicateRequest(t *testing.T) {
	cf := newCommerceFixture(t)
	logger := tlogger{t: t}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Subscribe to buyer.stdIn so we can count ServiceResponse messages.
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	deliveries, err := cf.bus.Subscribe(subCtx, cf.buyerStdIn, topic.SubscribeOpts{})
	require.NoError(t, err)

	respCount := atomic.Int32{}
	var done sync.WaitGroup
	done.Add(1)
	go func() {
		defer done.Done()
		for d := range deliveries {
			var probe struct {
				Type      string `json:"type"`
				RequestID string `json:"requestId"`
			}
			if err := json.Unmarshal(d.Message.Payload(), &probe); err != nil {
				continue
			}
			if probe.Type == payment.PayloadServiceResponse {
				respCount.Add(1)
			}
		}
	}()

	// Drive a complete agreement to FUNDED state.
	rid := "iter7-dup-001"
	cf.buyerCfg.RequestID = rid
	sellerSessionDone := make(chan *SellerSession, 1)
	go func() {
		ss, err := SellerObserveAndAccept(ctx, cf.sellerCfg, 3*time.Second)
		require.NoError(t, err)
		sellerSessionDone <- ss
	}()
	bs, err := BuyerNegotiateAndFund(ctx, cf.buyerCfg)
	require.NoError(t, err)
	require.NotNil(t, bs)
	ss := <-sellerSessionDone
	require.NotNil(t, ss)

	// At this point seller has published one ServiceResponse.
	require.Eventually(t, func() bool {
		return respCount.Load() >= 1
	}, 2*time.Second, 25*time.Millisecond)

	// Now exercise the iter-7 P1.3 idempotency path directly: invoke
	// replayServiceResponse and verify a 2nd ServiceResponse appears on
	// buyer.stdIn. This is the function the seller's waitSetup loop calls
	// when a duplicate ServiceRequest with matching rid arrives.
	require.NoError(t, replayServiceResponse(SellerConfig{
		Bus:        cf.bus,
		PrivateKey: &cf.sellerKey,
	}, ss, logger))

	require.Eventually(t, func() bool {
		return respCount.Load() >= 2
	}, 2*time.Second, 25*time.Millisecond,
		"replay should produce a 2nd ServiceResponse")
	subCancel()
	done.Wait()
}
