package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// TestCLI_FullCommerce_MemoryBackends drives the seller CLI's run()
// against a memory topic adapter + memory escrow + memory registry
// contract while an in-process buyer goroutine drives RunBuyerCLISession
// over the same shared backends. End-to-end shape: seller registers →
// buyer publishes ServiceRequest → seller responds → buyer funds escrow →
// seller publishes ConnectionSetup → buyer dials seller libp2p host → 4
// frames → buyer publishes ServiceStop → seller invoices → buyer approves
// → COMPLETED.
//
// This is the Stage 2b validation gate's load-bearing assertion: the
// seller CLI's flag set + Deps wiring + spawned session goroutine work
// without requiring the operator to launch two separate processes.
func TestCLI_FullCommerce_MemoryBackends(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	// Shared backends.
	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(64)
	escrow := payment.NewMemoryEscrow()
	contract := registry.NewMemoryRegistryContract()

	// Pin pendingOwner so the seller's RegisterSeller succeeds.
	sellerNeuronKey, err := keylib.NeuronPrivateKeyFromHex(fixedKeyHex)
	require.NoError(t, err)
	contract.SetPendingOwner(common.BytesToAddress(sellerNeuronKey.PublicKey().EVMAddress().Bytes()))

	sellerEVMHex := sellerNeuronKey.PublicKey().EVMAddress().Hex()

	// Seller signal channel — we close it after the buyer's session
	// completes to unblock the seller's run() loop. The seller's
	// session goroutine actually terminates the run() loop via the
	// sessionCh path, but the signal channel is the fallback.
	sellerSig := make(chan os.Signal, 1)
	sellerStdout := &bytes.Buffer{}
	sellerStderr := &bytes.Buffer{}

	sellerDeps := Deps{
		TopicAdapter:    bus,
		EscrowAdapter:   escrow,
		ContractFactory: memoryContractFactory(contract),
		SignalCh:        sellerSig,
	}

	// Buyer's own ECDSA key (different from the seller).
	buyerNeuronKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerEcdsa, err := buyerNeuronKey.ToBlockchainKey()
	require.NoError(t, err)
	buyerEVMHex := ethcrypto.PubkeyToAddress(buyerEcdsa.PublicKey).Hex()
	_ = buyerEVMHex

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
			"--synth",
			"--synth-fps=20",
			"--key-hex=" + fixedKeyHex,
			"--listen=/ip4/127.0.0.1/udp/0/quic-v1",
		}, map[string]string{}, sellerStdout, sellerStderr, sellerDeps)
	}()

	// Give the seller's run() time to register, build host, and spawn
	// the session goroutine.
	time.Sleep(500 * time.Millisecond)

	// Buyer side: drive RunBuyerCLISession over the shared bus + escrow.
	buyerHost, err := delivery.NewLibp2pHost(buyerEcdsa, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = buyerHost.Close() })

	// Discover the seller and resolve sellerStdIn from the AgentURI.
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	sellerEVM, err := keylib.EVMAddressFromHex(sellerEVMHex)
	require.NoError(t, err)

	discovery, err := remoteid.DiscoverSeller(ctx, sellerEVM, registryAddr, 296, contract)
	require.NoError(t, err)
	sellerStdInLocator := discovery.TopicConfigFor("stdIn")
	require.NotEmpty(t, sellerStdInLocator, "seller's AgentURI must carry stdIn topicId in Stage 2b")

	sellerStdIn, err := topic.NewTopicRef(bus.SupportedTransport(), sellerStdInLocator)
	require.NoError(t, err)
	buyerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "test-buyer-stdin"})
	require.NoError(t, err)

	var frameCount uint64
	var frameMu sync.Mutex
	go func() {
		defer wg.Done()
		final, err := remoteid.RunBuyerCLISession(ctx, remoteid.BuyerCLIOptions{
			Key:                  &buyerNeuronKey,
			EcdsaPriv:            buyerEcdsa,
			Adapter:              bus,
			SellerStdIn:          sellerStdIn,
			BuyerStdIn:           buyerStdIn,
			RequestID:            "cli-stage2b-1",
			ExpectedSellerPeerID: discovery.PeerID,
			Mode:                 remoteid.CommerceModeFull,
			Escrow:               escrow,
			EscrowBinding:        "memory",
			SellerEVM:            sellerEVMHex,
			BuyerHost:            buyerHost,
			FrameLimit:           4,
			OnFrameReceived: func(_ remoteid.RemoteIdFrame) error {
				frameMu.Lock()
				frameCount++
				frameMu.Unlock()
				return nil
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "approved", final.FinalAction, "Stage 2b lifecycle ends in InvoiceAck(approved)")
		// Do NOT close sellerSig — let the seller's session goroutine
		// terminate the run() loop via sessionCh (we assert the
		// "[seller-session] complete state=COMPLETED" log below).
	}()

	wg.Wait()

	assert.Equal(t, 0, sellerExit, "seller CLI returned non-zero: stderr=%s", sellerStderr.String())
	frameMu.Lock()
	defer frameMu.Unlock()
	assert.Equal(t, uint64(4), frameCount, "buyer should read exactly --frame-limit=4 frames")

	logs := sellerStderr.String()
	assert.Contains(t, logs, "topics created: stdIn=", "seller logs topic creation")
	assert.Contains(t, logs, "[lifecycle] requestID=cli-stage2b-1 IDLE→REQUESTED")
	assert.Contains(t, logs, "ACTIVE→COMPLETED", "seller logs the Stage-2 lifecycle terminal state")
	assert.Contains(t, logs, "[seller-session] complete state=COMPLETED")
	// Stage 3B: the seller embedded its libp2p listen multiaddrs in the
	// AgentURI before registering. The buyer's discovery sees them.
	assert.Contains(t, logs, "AgentURI carries", "seller logs multiaddr embedding for Stage 3B discovery")
}

// TestCLI_FullCommerce_Stage3B_RegistryOnlyDiscovery asserts the Stage 3B
// registry-only discovery path: the buyer reads multiaddrs straight out
// of the registered AgentURI — no --seller-multiaddr, no ECIES-decrypt
// dependency for the dial. The buyer's CLI orchestrator passes
// DialAddrOverride = ResolveDialAddrs() so the dial uses the registry's
// AddrInfo directly.
func TestCLI_FullCommerce_Stage3B_RegistryOnlyDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
		TopicAdapter:    bus,
		EscrowAdapter:   escrow,
		ContractFactory: memoryContractFactory(contract),
		SignalCh:        sellerSig,
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
			"--synth",
			"--synth-fps=20",
			"--key-hex=" + fixedKeyHex,
			"--listen=/ip4/127.0.0.1/udp/0/quic-v1",
		}, map[string]string{}, sellerStdout, sellerStderr, sellerDeps)
	}()

	time.Sleep(500 * time.Millisecond)

	// Buyer discovers via registry + dials using registry multiaddrs ONLY.
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	sellerEVM, err := keylib.EVMAddressFromHex(sellerEVMHex)
	require.NoError(t, err)

	discovery, err := remoteid.DiscoverSeller(ctx, sellerEVM, registryAddr, 296, contract)
	require.NoError(t, err)

	// Stage 3B load-bearing assertion: AgentURI carries usable multiaddrs.
	dialInfo, err := discovery.ResolveDialAddrs()
	require.NoError(t, err, "Stage 3B: AgentURI must carry multiaddrs")
	require.NotEmpty(t, dialInfo.Addrs)
	assert.Equal(t, discovery.PeerID, dialInfo.ID.String())

	sellerStdInLocator := discovery.TopicConfigFor("stdIn")
	require.NotEmpty(t, sellerStdInLocator)
	sellerStdIn, err := topic.NewTopicRef(bus.SupportedTransport(), sellerStdInLocator)
	require.NoError(t, err)
	buyerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "stage3b-buyer-stdin"})
	require.NoError(t, err)

	buyerHost, err := delivery.NewLibp2pHost(buyerEcdsa, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = buyerHost.Close() })

	go func() {
		defer wg.Done()
		final, err := remoteid.RunBuyerCLISession(ctx, remoteid.BuyerCLIOptions{
			Key:                  &buyerNeuronKey,
			EcdsaPriv:            buyerEcdsa,
			Adapter:              bus,
			SellerStdIn:          sellerStdIn,
			BuyerStdIn:           buyerStdIn,
			RequestID:            "cli-stage3b-1",
			ExpectedSellerPeerID: discovery.PeerID,
			Mode:                 remoteid.CommerceModeFull,
			Escrow:               escrow,
			EscrowBinding:        "memory",
			SellerEVM:            sellerEVMHex,
			BuyerHost:            buyerHost,
			FrameLimit:           3,
			DialAddrOverride:     dialInfo, // Stage 3B: dial via registry, NOT ECIES.
			OnFrameReceived:      func(_ remoteid.RemoteIdFrame) error { return nil },
		})
		require.NoError(t, err)
		assert.Equal(t, "approved", final.FinalAction)
	}()

	wg.Wait()
	assert.Equal(t, 0, sellerExit, "seller CLI returned non-zero: stderr=%s", sellerStderr.String())
	logs := sellerStderr.String()
	assert.Contains(t, logs, "AgentURI carries", "Stage 3B: seller logs multiaddrs embedded in AgentURI")
	assert.Contains(t, logs, "ACTIVE→COMPLETED")
}

// TestRun_RegistryMode_HeartbeatLoopAdvertisesOperational asserts the
// Stage 3C seller CLI wiring: after RegisterSeller succeeds and the
// libp2p host is built, the heartbeat publisher spawns and emits at
// least one heartbeat carrying the full Operational disclosure block
// (sellerEVM, sellerPeerID, serviceName=remote-id, topicBackend,
// escrowBackend, agentURISha256). FR-R21 anchor.
func TestRun_RegistryMode_HeartbeatLoopAdvertisesOperational(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(64)
	escrow := payment.NewMemoryEscrow()
	contract := registry.NewMemoryRegistryContract()

	sellerNeuronKey, err := keylib.NeuronPrivateKeyFromHex(fixedKeyHex)
	require.NoError(t, err)
	contract.SetPendingOwner(common.BytesToAddress(sellerNeuronKey.PublicKey().EVMAddress().Bytes()))
	sellerEVMHex := sellerNeuronKey.PublicKey().EVMAddress().Hex()
	sellerPeerID, err := sellerNeuronKey.PublicKey().PeerID()
	require.NoError(t, err)

	sellerSig := make(chan os.Signal, 1)
	sellerStdout := &bytes.Buffer{}
	sellerStderr := &bytes.Buffer{}
	sellerDeps := Deps{
		TopicAdapter:      bus,
		EscrowAdapter:     escrow,
		ContractFactory:   memoryContractFactory(contract),
		SignalCh:          sellerSig,
		HeartbeatInterval: 30 * time.Millisecond,
	}

	var sellerExit int
	sellerDone := make(chan struct{})
	go func() {
		defer close(sellerDone)
		sellerExit = run([]string{
			"--mode=eip8004-registry",
			"--commerce-mode=full",
			"--topic-backend=memory",
			"--escrow-backend=memory",
			"--registry-address=0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
			"--rpc-url=memory://unused",
			"--chain-id=296",
			"--synth", "--synth-fps=10",
			"--key-hex=" + fixedKeyHex,
			"--listen=/ip4/127.0.0.1/udp/0/quic-v1",
		}, map[string]string{}, sellerStdout, sellerStderr, sellerDeps)
	}()

	// Wait for the seller to come up + register. We discover via the
	// memory contract to find the stdOut topic ref.
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	sellerEVM, err := keylib.EVMAddressFromHex(sellerEVMHex)
	require.NoError(t, err)

	var discovery remoteid.DiscoverResult
	require.Eventually(t, func() bool {
		d, derr := remoteid.DiscoverSeller(ctx, sellerEVM, registryAddr, 296, contract)
		if derr != nil {
			return false
		}
		discovery = d
		return true
	}, 5*time.Second, 50*time.Millisecond, "seller never registered")

	stdOutLoc := discovery.TopicConfigFor("stdOut")
	require.NotEmpty(t, stdOutLoc)
	stdOutRef, err := topic.NewTopicRef(bus.SupportedTransport(), stdOutLoc)
	require.NoError(t, err)

	subCh, err := bus.Subscribe(ctx, stdOutRef, topic.SubscribeOpts{})
	require.NoError(t, err)

	// Receive a heartbeat (the publisher cycles every 30ms; ample within 2s).
	var caps *health.Capabilities
	timeout := time.After(2 * time.Second)
collect:
	for {
		select {
		case delivery, ok := <-subCh:
			if !ok {
				t.Fatal("stdOut subscription closed before any heartbeat arrived")
			}
			var payload health.HeartbeatPayload
			require.NoError(t, json.Unmarshal(delivery.Message.Payload(), &payload))
			if payload.Capabilities != nil && payload.Capabilities.Operational != nil {
				caps = payload.Capabilities
				break collect
			}
		case <-timeout:
			t.Fatal("no heartbeat with Operational arrived within 2s")
		}
	}

	op := caps.Operational
	assert.Equal(t, "remote-id", op.ServiceName)
	assert.Equal(t, sellerEVMHex, op.SellerEVM)
	assert.Equal(t, "memory", op.TopicBackend)
	assert.Equal(t, "memory", op.EscrowBackend)
	assert.NotEmpty(t, op.AgentURISha256, "agentURISha256 from RegisterSeller must surface on the heartbeat")
	assert.Equal(t, discovery.AgentURISha256, op.AgentURISha256,
		"heartbeat agentURISha256 must equal the registered AgentURI SHA-256 (FR-R21 cross-check anchor)")
	assert.False(t, op.Degraded, "Stage 3C ships DegradedFunc as a constant-false stub")

	// PeerID is derived from libp2p host (not the secp256k1 key directly),
	// but for in-process tests both produce the same multihash. Compare
	// via canonical string form.
	assert.Equal(t, sellerPeerID.String(), op.SellerPeerID)

	// Shut down cleanly so the deferred cancels in run() drain.
	sellerSig <- os.Interrupt
	<-sellerDone
	assert.Equal(t, 0, sellerExit, "seller CLI exit; stderr=%s", sellerStderr.String())
	assert.Contains(t, sellerStderr.String(), "[heartbeat] loop started",
		"seller logs the Stage 3C startup line")
}
