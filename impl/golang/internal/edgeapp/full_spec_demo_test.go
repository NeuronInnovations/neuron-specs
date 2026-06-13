package edgeapp

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/neuron-sdk/neuron-go-sdk/internal/validation"
)

// TestEdgeDemo_FullSpecFlowMock is the canonical end-to-end demo
// exercising the full spec stack through RunSeller + RunBuyer + Validator
// in a single process against one MemoryBus:
//
//   - spec 002 keylib (always)
//   - spec 003 registration via MemoryRegistry (Registry config flag set)
//   - spec 004 topic envelopes (always)
//   - spec 005 heartbeat publish + (optional) deadline observation
//   - spec 008 commerce + escrow via MemoryEscrow (Escrow flag set)
//   - spec 009 reverse-connect delivery (always — the data plane)
//   - spec 010 validator + EvidenceEnvelope (Validator running)
//   - spec 013 Profile E descriptor publish (PublishProfileDescriptor flag set)
//
// Asserts every phase reaches its expected state. No testnet calls; no
// libp2p host (we use synthetic frames + skip the actual stream — the
// test exists to verify the wiring, not the libp2p adapter which has its
// own coverage).
//
// This test is the iteration 3 north star: if it goes red, demo
// completeness has regressed.
func TestEdgeDemo_FullSpecFlowMock(t *testing.T) {
	bus := NewMemoryBus()

	mk := func(memo string) topic.TopicRef {
		ref, err := bus.CreateTopic(topic.CreateTopicOpts{
			Transport: bus.SupportedTransport(),
			Memo:      memo,
		})
		require.NoError(t, err)
		return ref
	}

	sellerStdIn := mk("seller-stdIn")
	sellerStdOut := mk("seller-stdOut")
	sellerStdErr := mk("seller-stdErr")
	buyerStdIn := mk("buyer-stdIn")
	buyerStdOut := mk("buyer-stdOut")
	buyerStdErr := mk("buyer-stdErr")
	validatorStdOut := mk("validator-stdOut")

	sellerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	validatorKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	sellerPub := sellerKey.PublicKey()
	sellerEcdsaPub, err := sellerPub.ToBlockchainKey()
	require.NoError(t, err)

	registry := NewMemoryRegistry()
	escrow := payment.NewMemoryEscrow()

	logger := tlogger{t: t}

	// The validator starts first so it's subscribed before the agreement begins.
	validatorCfg := ValidatorConfig{
		Bus:              bus,
		Key:              &validatorKey,
		ValidatorStdOut:  validatorStdOut,
		ValidatorAgentID: "1",
		SubjectAgentID:   "2",
		SellerStdIn:      sellerStdIn,
		BuyerStdIn:       buyerStdIn,
		Logger:           logger,
	}
	v, err := NewValidator(validatorCfg)
	require.NoError(t, err)

	rootCtx, rootCancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer rootCancel()

	validatorDone := make(chan error, 1)
	go func() { validatorDone <- v.Run(rootCtx) }()

	// Buyer pushes received frames to a counter.
	var received atomic.Uint64
	onAgg := func(af AggregatedFrame) { received.Add(1) }

	var settledSession atomic.Pointer[BuyerSession]
	onSettled := func(s *BuyerSession) { settledSession.Store(s) }

	// Bounded-feed wrapper: produce 150 frames at 200 fps then return nil
	// (synthesizes a graceful seller-side close). This drives SendStream
	// to end naturally, which closes the buyer's stream and triggers
	// Settle without anyone having to cancel the root context. The
	// validator stays alive throughout to observe the InvoiceAck.
	boundedSynth := func(ctx context.Context, out chan<- feeds.FeedFrame) error {
		inner, cancelInner := context.WithCancel(ctx)
		defer cancelInner()
		count := 0
		const target = 150
		framesIn := make(chan feeds.FeedFrame, 64)
		errCh := make(chan error, 1)
		go func() { errCh <- feeds.RunSynth(inner, 200, framesIn) }()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case f := <-framesIn:
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- f:
				}
				count++
				if count >= target {
					cancelInner()
					<-errCh
					return nil
				}
			}
		}
	}

	sellerCfg := SellerConfig{
		Bus:                      bus,
		PrivateKey:               &sellerKey,
		StdIn:                    sellerStdIn,
		StdOut:                   sellerStdOut,
		StdErr:                   sellerStdErr,
		LibP2PListenAddr:         "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:                 DefaultProtocol,
		HeartbeatPeriod:          10 * time.Second,
		FeedSource:               boundedSynth,
		Logger:                   logger,
		PublishProfileDescriptor: true,
		Registry:                 registry,
		Escrow:                   escrow,
	}

	buyerCfg := BuyerConfig{
		Bus:                bus,
		PrivateKey:         &buyerKey,
		StdIn:              buyerStdIn,
		StdOut:             buyerStdOut,
		StdErr:             buyerStdErr,
		Sellers:            []SellerEntry{{StdIn: sellerStdIn, StdOut: sellerStdOut, PubKey: sellerEcdsaPub, DisplayName: "alpha"}},
		LibP2PListenAddr:   "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:           DefaultProtocol,
		RequestID:          "edge-full-001",
		HeartbeatPeriod:    10 * time.Second,
		ReconnectBackoff:   500 * time.Millisecond,
		SellerDialTimeout:  5 * time.Second,
		OnAggregatedFrame:  onAgg,
		Logger:             logger,
		Registry:           registry,
		Escrow:             escrow,
		PaymentCurrency:    "tinybar",
		PaymentPriceTinybar: "100",
		PaymentRequestID:   "edge-full-001",
		OnAgreementSettled: onSettled,
	}

	sellerDone := make(chan error, 1)
	buyerDone := make(chan error, 1)
	go func() { sellerDone <- RunSeller(rootCtx, sellerCfg) }()
	time.Sleep(200 * time.Millisecond)
	go func() { buyerDone <- RunBuyer(rootCtx, buyerCfg) }()

	// Wait for the bounded feed to drain + the agreement to settle.
	deadline := time.After(10 * time.Second)
	for settledSession.Load() == nil {
		select {
		case <-deadline:
			t.Fatalf("agreement never settled; received %d frames", received.Load())
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Logf("agreement settled after %d frames", received.Load())

	// Give the validator a beat to process the InvoiceAck delivery before
	// we cancel everything. The InvoiceAck is the message that flips the
	// 008-payment verdict; without this drain we'd race the validator's
	// goroutine.
	require.Eventually(t, func() bool {
		return len(bus.GetMessages(validatorStdOut)) >= 2
	}, 3*time.Second, 50*time.Millisecond, "validator should publish 2 envelopes")

	// Graceful shutdown — seller already exited (bounded feed); just
	// cancel the buyer + validator.
	rootCancel()

	for _, ch := range []chan error{sellerDone, buyerDone, validatorDone} {
		select {
		case err := <-ch:
			assert.True(t, err == nil || err == context.Canceled || err == context.DeadlineExceeded,
				"goroutine returned unexpected error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("a goroutine did not return within 5s of cancel")
		}
	}

	// Phase assertions:

	// 1. spec 003 — both agents registered in the MemoryRegistry.
	sellerToken, found, _ := registry.LookupTokenID(context.Background(), sellerPub.EVMAddress().Hex())
	assert.True(t, found, "seller should be registered")
	assert.NotNil(t, sellerToken)

	buyerToken, found, _ := registry.LookupTokenID(context.Background(), buyerKey.PublicKey().EVMAddress().Hex())
	assert.True(t, found, "buyer should be registered")
	assert.NotNil(t, buyerToken)

	sellerURI, _ := registry.AgentURIByTokenID(context.Background(), sellerToken)
	var doc AgentURIDocument
	require.NoError(t, json.Unmarshal([]byte(sellerURI), &doc))
	assert.Len(t, doc.Services, 3, "seller agentURI should list 3 topic services")

	// 2. spec 013 — descriptor published on seller.stdOut.
	stdOutMsgs := bus.GetMessages(sellerStdOut)
	descCount := 0
	for _, m := range stdOutMsgs {
		var probe struct {
			NeuronProfileID string `json:"neuronProfileId"`
		}
		if err := json.Unmarshal(m.Payload(), &probe); err == nil && probe.NeuronProfileID == DefaultEdgeProfileID {
			descCount++
		}
	}
	assert.GreaterOrEqual(t, descCount, 1, "expected at least one Profile E descriptor on seller stdOut")

	// 3. spec 008 — commerce + escrow consumed.
	require.NotNil(t, settledSession.Load(), "buyer should have hit OnAgreementSettled")
	bal, err := escrow.GetBalance(context.Background(), settledSession.Load().EscrowRef())
	require.NoError(t, err)
	assert.Equal(t, "0", bal.Available, "escrow balance drained after settlement")

	// 4. spec 010 — validator published 2 envelopes.
	envelopeCount := 0
	verdicts := map[string]string{}
	for _, m := range bus.GetMessages(validatorStdOut) {
		var env map[string]any
		if err := json.Unmarshal(m.Payload(), &env); err != nil {
			continue
		}
		if env["type"] != ValidatorEvidencePayloadType {
			continue
		}
		envelopeCount++
		verdicts[env["specRef"].(string)] = env["verdict"].(string)
	}
	assert.GreaterOrEqual(t, envelopeCount, 2, "validator should publish ≥ 2 envelopes")
	assert.Equal(t, string(validation.VerdictCompliant), verdicts["008-payment"],
		"happy-path payment evidence ⇒ compliant")
	assert.Equal(t, string(validation.VerdictInconclusive), verdicts["009-p2p-data-delivery"],
		"delivery evidence is inconclusive in iter-3 (no stream-bytes hook)")

	// 5. Frame count summary.
	t.Logf("end-of-run: frames=%d agreement=%s payment=%s delivery=%s",
		received.Load(),
		settledSession.Load().State(),
		verdicts["008-payment"],
		verdicts["009-p2p-data-delivery"])
}
