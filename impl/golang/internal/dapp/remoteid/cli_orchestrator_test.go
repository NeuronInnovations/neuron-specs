package remoteid

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	remoteidsrc "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Test_FR_P13_CLIBridge_FullCommerce drives both CLI bridges
// (RunSellerCLISession + RunBuyerCLISession) over a shared memory bus
// + memory escrow. End-to-end shape: seller registers ServiceRequest →
// buyer publishes → seller responds → buyer funds escrow → seller sends
// ConnectionSetup → buyer dials seller libp2p host → 4 frames → buyer
// publishes ServiceStop → seller invoices → buyer approves.
func Test_FR_P13_CLIBridge_FullCommerce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(64)
	escrow := payment.NewMemoryEscrow()

	sellerKey := newTestKey(t)
	buyerKey := newTestKey(t)
	sellerPriv, _ := mustECDSAKeys(t, sellerKey)
	buyerPriv, _ := mustECDSAKeys(t, buyerKey)

	sellerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "cli-seller-stdin"})
	require.NoError(t, err)
	buyerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "cli-buyer-stdin"})
	require.NoError(t, err)

	descriptor, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:   sellerKey,
		FeedSource: FeedSourceSynthetic,
	})
	require.NoError(t, err)

	sellerHost, err := delivery.NewLibp2pHost(sellerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()
	buyerHost, err := delivery.NewLibp2pHost(buyerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	// Frame source: synthetic at 50 fps so 4 frames complete in well
	// under the 10s ctx deadline.
	running, err := Start(ctx, SellerConfig{
		Host:   sellerHost,
		Source: SynthSource(remoteidsrc.SynthOptions{FPS: 50, DroneCount: 1}),
	})
	require.NoError(t, err)
	defer running.Cancel()

	// Seller bridge runs in a goroutine.
	type sellerRes struct {
		res SellerSessionResult
		err error
	}
	sellerCh := make(chan sellerRes, 1)
	go func() {
		res, err := RunSellerCLISession(ctx, SellerCLIOptions{
			Key:           sellerKey,
			Adapter:       bus,
			SellerStdIn:   sellerStdIn,
			Descriptor:    descriptor,
			Host:          sellerHost,
			Escrow:        escrow,
			EscrowBinding: "memory",
			Mode:          CommerceModeFull,
			FrameSummary:  func() (uint64, uint64, uint64) { return 4, 1, 2 },
		})
		sellerCh <- sellerRes{res, err}
	}()

	// Tiny pause so the seller's ReceiveServiceRequest subscription is
	// up before the buyer publishes (memory adapter has FromSequence
	// backfill, but the test logs are cleaner this way).
	time.Sleep(50 * time.Millisecond)

	var framesReceived uint64
	var framesMu sync.Mutex
	final, err := RunBuyerCLISession(ctx, BuyerCLIOptions{
		Key:                  buyerKey,
		EcdsaPriv:            buyerPriv,
		Adapter:              bus,
		SellerStdIn:          sellerStdIn,
		BuyerStdIn:           buyerStdIn,
		RequestID:            "cli-bridge-req-1",
		ExpectedSellerPeerID: sellerHost.ID().String(),
		Mode:                 CommerceModeFull,
		Escrow:               escrow,
		EscrowBinding:        "memory",
		SellerEVM:            sellerKey.PublicKey().EVMAddress().Hex(),
		BuyerHost:            buyerHost,
		FrameLimit:           4,
		OnFrameReceived: func(_ RemoteIdFrame) error {
			framesMu.Lock()
			framesReceived++
			framesMu.Unlock()
			return nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "approved", final.FinalAction)
	assert.Equal(t, uint64(4), framesReceived, "buyer reads exactly --frame-limit=4 frames before ServiceStop")

	select {
	case sr := <-sellerCh:
		require.NoError(t, sr.err)
		assert.Equal(t, payment.StateCompleted, sr.res.FinalState)
	case <-time.After(5 * time.Second):
		t.Fatal("seller session did not complete within 5s of buyer InvoiceAck")
	}
}

// Test_FR_R01_CLIBridge_DialOverride exercises the test-only
// DialOverride path: instead of dialing the real libp2p host, the
// caller returns a pre-supplied ReadWriteCloser that streams a fixed
// frame sequence. Lets us cover the orchestrator bridge without a
// running seller stream handler.
func Test_FR_R01_CLIBridge_DialOverride(t *testing.T) {
	// (Out of scope for the Stage 2b initial cut — DialOverride is the
	// seam for Stage 3A tests that need to stub the libp2p layer when
	// driving testnet HCS only. Marked as a follow-up so the dial
	// override hook is documented; the happy path is covered above.)
	t.Skip("DialOverride coverage deferred to Stage 3A test wiring")
}

// Sanity check the package-level helpers don't accidentally panic on
// zero-value inputs.
func TestRunBuyerCLISession_RequiresHost(t *testing.T) {
	_, err := RunBuyerCLISession(context.Background(), BuyerCLIOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BuyerHost (or DialOverride) required")
}

// silence unused-import warning for protocol when DialOverride coverage
// lands later.
var _ = protocol.ID("")
var _ peer.ID
