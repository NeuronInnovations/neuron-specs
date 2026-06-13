package edgeapp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// commerceFixture sets up a self-contained commerce test rig: one MemoryBus
// shared between buyer and seller, two keys, four topics, one MemoryEscrow.
type commerceFixture struct {
	bus           *MemoryBus
	buyerKey      keylib.NeuronPrivateKey
	sellerKey     keylib.NeuronPrivateKey
	buyerStdIn    topic.TopicRef
	sellerStdIn   topic.TopicRef
	escrow        *payment.MemoryEscrow
	buyerCfg      BuyerSessionConfig
	sellerCfg     SellerSessionConfig
}

func newCommerceFixture(t *testing.T) *commerceFixture {
	t.Helper()

	bus := NewMemoryBus()
	mk := func(memo string) topic.TopicRef {
		ref, err := bus.CreateTopic(topic.CreateTopicOpts{
			Transport: bus.SupportedTransport(),
			Memo:      memo,
		})
		require.NoError(t, err)
		return ref
	}
	buyerStdIn := mk("buyer-stdIn")
	sellerStdIn := mk("seller-stdIn")

	bk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	sk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	escrow := payment.NewMemoryEscrow()
	logger := tlogger{t: t}

	return &commerceFixture{
		bus:         bus,
		buyerKey:    bk,
		sellerKey:   sk,
		buyerStdIn:  buyerStdIn,
		sellerStdIn: sellerStdIn,
		escrow:      escrow,
		buyerCfg: BuyerSessionConfig{
			Bus:              bus,
			Key:              &bk,
			BuyerStdIn:       buyerStdIn,
			SellerStdIn:      sellerStdIn,
			RequestID:        "edge-commerce-001",
			ServiceRef:       "neuron://service/edge-feed/v1",
			BuyerEVM:         bk.PublicKey().EVMAddress().Hex(),
			SellerEVM:        sk.PublicKey().EVMAddress().Hex(),
			Currency:         "tinybar",
			Price:            "100",
			NegotiationTTL:   2 * time.Second,
			AgreementTimeout: 24 * time.Hour,
			Escrow:           escrow,
			Logger:           logger,
		},
		sellerCfg: SellerSessionConfig{
			Bus:           bus,
			Key:           &sk,
			SellerStdIn:   sellerStdIn,
			BuyerStdIn:    buyerStdIn,
			Escrow:        escrow,
			SellerEVM:     sk.PublicKey().EVMAddress().Hex(),
			EscrowBinding: "memory",
			Logger:        logger,
		},
	}
}

func TestCommerce_FullCycleMock(t *testing.T) {
	f := newCommerceFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var (
		buyerSession  *BuyerSession
		sellerSession *SellerSession
		buyerErr      error
		sellerErr     error
	)

	// Phase 1: parallel negotiate + fund (buyer) / observe + accept (seller).
	var negotiateWG sync.WaitGroup
	negotiateWG.Add(2)
	go func() {
		defer negotiateWG.Done()
		buyerSession, buyerErr = BuyerNegotiateAndFund(ctx, f.buyerCfg)
	}()
	go func() {
		defer negotiateWG.Done()
		sellerSession, sellerErr = SellerObserveAndAccept(ctx, f.sellerCfg, 5*time.Second)
	}()
	negotiateWG.Wait()
	require.NoError(t, buyerErr)
	require.NoError(t, sellerErr)
	require.NotNil(t, buyerSession)
	require.NotNil(t, sellerSession)

	assert.Equal(t, payment.StateFunded, buyerSession.State())
	assert.Equal(t, payment.StateFunded, sellerSession.State())
	assert.Equal(t, buyerSession.AgreementHash(), sellerSession.AgreementHash(),
		"both sides must compute the same agreement hash from the same canonical ServiceResponse bytes")

	// Phase 2: simulated data delivery — pretend frames flowed for some time.
	time.Sleep(50 * time.Millisecond)

	// Phase 3: parallel issue invoice (seller) / settle (buyer).
	var settleWG sync.WaitGroup
	settleWG.Add(2)
	go func() {
		defer settleWG.Done()
		// In a real flow the releaseRequestRef comes from the escrow's
		// RequestRelease the buyer initiates. The seller gets it by reading
		// the buyer's later RequestRelease tx (or the buyer's optimistic ACK).
		// For mock we generate a placeholder; the buyer will call
		// RequestRelease itself in Settle.
		sellerErr = sellerSession.IssueInvoice(ctx, "release-pending-001", 5*time.Second)
	}()
	go func() {
		defer settleWG.Done()
		buyerErr = buyerSession.Settle(ctx)
	}()
	settleWG.Wait()
	require.NoError(t, buyerErr)
	require.NoError(t, sellerErr)

	// Phase 4: assert final state.
	assert.Equal(t, payment.StateActive, buyerSession.State(),
		"buyer agreement state after InvoiceAck approved")
	assert.Equal(t, payment.StateActive, sellerSession.State(),
		"seller agreement state after InvoiceAck approved")

	// Buyer's escrow balance should be zero after release.
	bal, err := f.escrow.GetBalance(ctx, buyerSession.EscrowRef())
	require.NoError(t, err)
	assert.Equal(t, "0", bal.Available, "escrow drained after settlement")
}

func TestCommerce_BuyerNegotiationTimeout(t *testing.T) {
	f := newCommerceFixture(t)
	f.buyerCfg.NegotiationTTL = 200 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// No seller running. The buyer should publish ServiceRequest and time out.
	_, err := BuyerNegotiateAndFund(ctx, f.buyerCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ServiceResponse: timeout")
}

func TestCommerce_SellerWaitTimeout(t *testing.T) {
	f := newCommerceFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// No buyer running. The seller should subscribe and time out waiting
	// for ServiceRequest.
	_, err := SellerObserveAndAccept(ctx, f.sellerCfg, 200*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ServiceRequest: timeout")
}

func TestCommerce_ConfigValidation(t *testing.T) {
	f := newCommerceFixture(t)
	ctx := context.Background()

	bad := f.buyerCfg
	bad.RequestID = ""
	_, err := BuyerNegotiateAndFund(ctx, bad)
	require.Error(t, err)

	badSeller := f.sellerCfg
	badSeller.Key = nil
	_, err = SellerObserveAndAccept(ctx, badSeller, time.Second)
	require.Error(t, err)
}

func TestCommerce_BuyerSessionGetters(t *testing.T) {
	// Direct exercise of getters without a full E2E.
	f := newCommerceFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var buyerSession *BuyerSession
	done := make(chan struct{})
	go func() {
		defer close(done)
		buyerSession, _ = BuyerNegotiateAndFund(ctx, f.buyerCfg)
	}()
	sellerSession, err := SellerObserveAndAccept(ctx, f.sellerCfg, 5*time.Second)
	require.NoError(t, err)
	<-done
	require.NotNil(t, buyerSession)

	assert.NotEmpty(t, buyerSession.EscrowRef().Locator)
	assert.Equal(t, payment.StateFunded, buyerSession.State())
	assert.Equal(t, sellerSession.AgreementHash(), buyerSession.AgreementHash())
}
