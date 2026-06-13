package main

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/adsb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// TestCLI_FullCommerce_MemoryBackends drives the adsb-seller CLI's run()
// against a memory topic adapter + memory escrow + memory registry
// contract while an in-process buyer goroutine drives RunBuyerCLISession
// over the same shared backends. End-to-end shape: seller registers →
// buyer publishes ServiceRequest → seller responds → buyer funds escrow →
// seller publishes ConnectionSetup → buyer dials seller libp2p host → N
// NormalizedTrack frames → buyer publishes ServiceStop → seller invoices
// → buyer approves → COMPLETED.
//
// This validates that adsb-seller's flag set + Deps wiring + spawned
// session goroutine work end-to-end without operator-side process juggling.
func TestCLI_FullCommerce_MemoryBackends(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)

	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(64)
	escrow := payment.NewMemoryEscrow()
	contract := registry.NewMemoryRegistryContract()

	sellerNeuronKey, err := keylib.NeuronPrivateKeyFromHex(fixedKeyHex)
	require.NoError(t, err)
	contract.SetPendingOwner(common.BytesToAddress(sellerNeuronKey.PublicKey().EVMAddress().Bytes()))
	sellerEVMHex := sellerNeuronKey.PublicKey().EVMAddress().Hex()

	sellerSig := make(chan os.Signal, 1)
	sellerStdout := &bytes.Buffer{}
	sellerStderr := &bytes.Buffer{}

	sellerDeps := Deps{
		TopicAdapter:      bus,
		EscrowAdapter:     escrow,
		ContractFactory:   memoryContractFactory(contract),
		SignalCh:          sellerSig,
		HeartbeatInterval: 50 * time.Millisecond,
	}

	buyerNeuronKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerEcdsa, err := buyerNeuronKey.ToBlockchainKey()
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	var sellerExit int
	go func() {
		defer wg.Done()
		sellerExit = run([]string{
			"--mode=eip8004-registry",
			"--commerce-mode=full",
			"--topic-backend=memory",
			"--escrow-backend=memory",
			"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
			"--rpc-url=memory://unused",
			"--chain-id=296",
			"--feed-source=synthetic",
			"--synth-aircraft=1",
			"--synth-fps=20",
			"--key-hex=" + fixedKeyHex,
			"--listen=/ip4/127.0.0.1/udp/0/quic-v1",
		}, map[string]string{}, sellerStdout, sellerStderr, sellerDeps)
	}()

	// Let the seller register, build host, and spawn the session goroutine.
	time.Sleep(500 * time.Millisecond)

	buyerHost, err := delivery.NewLibp2pHost(buyerEcdsa, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = buyerHost.Close() })

	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	sellerEVM, err := keylib.EVMAddressFromHex(sellerEVMHex)
	require.NoError(t, err)

	discovery, err := adsb.DiscoverSeller(ctx, sellerEVM, registryAddr, 296, contract)
	require.NoError(t, err)
	require.Equal(t, "adsb", discovery.CommerceService.Name)
	require.Equal(t, "frame", discovery.CommerceService.Pricing.Unit)

	sellerStdInLocator := discovery.TopicConfigFor("stdIn")
	require.NotEmpty(t, sellerStdInLocator, "seller's AgentURI must carry stdIn topicId")

	sellerStdIn, err := topic.NewTopicRef(bus.SupportedTransport(), sellerStdInLocator)
	require.NoError(t, err)
	buyerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "test-adsb-buyer-stdin"})
	require.NoError(t, err)

	var frameCount uint64
	var frameMu sync.Mutex
	go func() {
		defer wg.Done()
		final, err := adsb.RunBuyerCLISession(ctx, adsb.BuyerCLIOptions{
			Key:                  &buyerNeuronKey,
			EcdsaPriv:            buyerEcdsa,
			Adapter:              bus,
			SellerStdIn:          sellerStdIn,
			BuyerStdIn:           buyerStdIn,
			RequestID:            "cli-adsb-stage2b-1",
			ExpectedSellerPeerID: discovery.PeerID,
			Mode:                 adsb.CommerceModeFull,
			Escrow:               escrow,
			EscrowBinding:        "memory",
			SellerEVM:            sellerEVMHex,
			BuyerHost:            buyerHost,
			FrameLimit:           4,
			OnFrameReceived: func(track adsb.NormalizedTrack) error {
				frameMu.Lock()
				defer frameMu.Unlock()
				frameCount++
				// Sanity-check the wire shape on each frame.
				if track.Type != adsb.FrameType {
					t.Errorf("unexpected frame type %q", track.Type)
				}
				if track.Source != adsb.SourceAdsb {
					t.Errorf("unexpected source %q", track.Source)
				}
				if track.EntityType != adsb.EntityTypeAircraft {
					t.Errorf("unexpected entityType %q", track.EntityType)
				}
				return nil
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "approved", final.FinalAction)
	}()

	wg.Wait()

	assert.Equal(t, 0, sellerExit, "seller CLI returned non-zero: stderr=%s", sellerStderr.String())
	frameMu.Lock()
	defer frameMu.Unlock()
	assert.Equal(t, uint64(4), frameCount, "buyer should read exactly --frame-limit=4 frames")

	logs := sellerStderr.String()
	assert.Contains(t, logs, "topics created: stdIn=", "seller logs adsb-* topic creation")
	assert.Contains(t, logs, "[lifecycle] requestID=cli-adsb-stage2b-1 IDLE→REQUESTED")
	assert.Contains(t, logs, "ACTIVE→COMPLETED", "seller logs lifecycle reaches COMPLETED")
	assert.Contains(t, logs, "[seller-session] complete state=COMPLETED")
	assert.Contains(t, logs, "AgentURI carries", "seller embeds multiaddrs for Stage 3B discovery")
	// Heartbeat operational disclosure (FR-R21-shape) MUST appear in seller logs.
	assert.Contains(t, logs, "[heartbeat] loop started sellerEVM=")
	assert.Contains(t, logs, "topicBackend=memory escrowBackend=memory")
	// Frame service name matches Spec 016 FR-A01.
	assert.Contains(t, logs, "name=adsb")
}
