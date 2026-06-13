package main

import (
	"bytes"
	"context"
	"log"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// TestCommerceFlags_Validation: the new flag family fails fast with clear
// errors and the defaults stay byte-identical (off).
func TestCommerceFlags_Validation(t *testing.T) {
	err := run([]string{"--commerce-mode", "banana"}, Deps{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown --commerce-mode")

	err = run([]string{"--commerce-mode", "full", "--register-only"}, Deps{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")

	// off (default) still requires --buyer — the original contract.
	err = run([]string{}, Deps{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "--buyer")
}

// TestSellerCommerce_EndToEnd_MemoryBackends drives the seller CLI's
// --commerce-mode=full end-to-end in-process: fake LE bridge → seller run()
// (register commerce card → pre-stream escrow gate → dial from reverse
// ConnectionSetup → pump → settle) against a package-level buyer.
func TestSellerCommerce_EndToEnd_MemoryBackends(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	bus := topic.NewMemoryTopicAdapter()
	escrow := payment.NewMemoryEscrow()
	contract := registry.NewMemoryRegistryContract()
	const regHex = "0x5d9B1fE5eB02173205AEe8DC4f72db15bFB5f73C"

	sellerNK, _ := sellerTestKey(t)
	sellerHex := sellerNK.Hex()
	dir := t.TempDir()
	evidenceOut := filepath.Join(dir, "seller-commerce.json")

	// Fake bridge: the FeedServer speaks the same FR-S91 LE wire the real
	// neuron-rid-bridge serves; publish detections continuously.
	bridge, err := sapient.ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer bridge.Close()
	go func() {
		for ctx.Err() == nil {
			_ = bridge.Publish(&sapientpb.SapientMessage{NodeId: proto.String("bridge-e2e")})
			time.Sleep(20 * time.Millisecond)
		}
	}()

	deps := Deps{
		ContractFactory: memoryContractFactory(contract),
		TopicAdapter:    bus,
		EscrowAdapter:   escrow,
	}
	sellerErr := make(chan error, 1)
	go func() {
		sellerErr <- run([]string{
			"--commerce-mode", "full",
			"--key-hex", sellerHex,
			"--bridge-addr", bridge.Addr(),
			"--registry-backend", "evm", "--registry-address", regHex,
			"--topic-backend", "memory", "--escrow-backend", "memory",
			"--pricing-amount", "5",
			"--commerce-evidence-out", evidenceOut,
		}, deps)
	}()

	// Wait until the seller's commerce card lands on the shared contract.
	sellerEVM := sellerNK.PublicKey().EVMAddress()
	regAddr, err := keylib.EVMAddressFromHex(regHex)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, lerr := registry.LookupRegistration(ctx, regAddr, defaultChainID, registry.ByEVMAddress(sellerEVM), contract)
		return lerr == nil
	}, 15*time.Second, 50*time.Millisecond, "seller registration never appeared")

	// Package-level buyer: funds the escrow, publishes the reverse setup,
	// admits the seller's dial, takes 5 frames, settles.
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerECDSA, err := buyerKey.ToBlockchainKey()
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	var got atomic.Uint64
	frameDone := make(chan struct{})
	buyerHost.SetStreamHandler(protocol.ID(sapient.ProtocolDetection), func(stream libp2pnetwork.Stream) {
		r := delivery.NewFrameReader(stream)
		for {
			if _, rerr := r.ReadFrame(); rerr != nil {
				return
			}
			if got.Add(1) == 5 {
				_ = stream.Reset() // buyer is done; seller's pump sees the error
				close(frameDone)
				return
			}
		}
	})

	var buyerLog bytes.Buffer
	bsession, err := sapient.StartBuyerCommerce(ctx, sapient.BuyerCommerceOptions{
		Key:             &buyerKey,
		Host:            buyerHost,
		Adapter:         bus,
		Escrow:          escrow,
		EscrowBinding:   sapient.SettlementBindingMemory,
		Contract:        contract,
		RegistryAddress: regAddr,
		ChainID:         defaultChainID,
		SellerEVM:       sellerEVM,
		Logger:          log.New(&buyerLog, "", 0),
	})
	require.NoError(t, err, "buyer start (log:\n%s)", buyerLog.String())

	select {
	case <-frameDone:
	case <-ctx.Done():
		t.Fatalf("frames never arrived: %v", ctx.Err())
	}
	bres, err := bsession.Finalise(ctx)
	require.NoError(t, err)
	require.Equal(t, "approved", bres.FinalAction)
	require.Equal(t, "5", bres.ReleasedAmount)
	require.Equal(t, sellerEVM.Hex(), bres.ReleaseRecipient, "release lands with the seller")

	require.NoError(t, <-sellerErr, "seller run() exits clean after settlement")

	ev, err := sapient.ReadCommerceEvidence(evidenceOut)
	require.NoError(t, err)
	require.Equal(t, "seller", ev.Role)
	require.Equal(t, string(payment.StateCompleted), ev.FinalState)
	require.Equal(t, "5", ev.EscrowAvailable, "the pre-stream gate observation is evidenced")
	require.Equal(t, "approved", ev.InvoiceAckAction)
	require.Equal(t, "evm", ev.RegistryBackend)
	require.Equal(t, "memory", ev.TopicBackend)
	require.Equal(t, "memory", ev.EscrowBackend)
	require.NotEmpty(t, ev.EvidenceHash)
	require.NotZero(t, ev.FrameCount)
	require.NotEmpty(t, ev.Topics["sellerStdIn"])
}
