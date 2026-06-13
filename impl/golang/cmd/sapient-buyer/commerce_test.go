package main

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
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

const testRegistryHex = "0x5d9B1fE5eB02173205AEe8DC4f72db15bFB5f73C"

func TestBuyerCommerceFlags_Validation(t *testing.T) {
	err := run([]string{"--commerce-mode", "banana"}, io.Discard, Deps{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown --commerce-mode")

	// full without --seller-evm (host + edge get created first; use
	// ephemeral ports so parallel tests never collide).
	err = run([]string{
		"--commerce-mode", "full",
		"--listen", "/ip4/127.0.0.1/udp/0/quic-v1",
		"--sapient-edge", "127.0.0.1:0",
	}, io.Discard, Deps{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "--seller-evm")

	// full without a registry address.
	t.Setenv("NEURON_REGISTRY_CONTRACT", "")
	err = run([]string{
		"--commerce-mode", "full",
		"--listen", "/ip4/127.0.0.1/udp/0/quic-v1",
		"--sapient-edge", "127.0.0.1:0",
		"--seller-evm", testRegistryHex,
	}, io.Discard, Deps{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "--registry-address")
}

// TestAdmissionGate: streams are HELD until the funded gate opens, then only
// the expected seller is admitted; everything else is reset.
func TestAdmissionGate(t *testing.T) {
	g := newAdmissionGate()
	resCh := make(chan bool, 2)
	// Two early streams arrive BEFORE funding: they must wait, not pass.
	go func() { ok, _ := g.admit(context.Background(), "seller-A"); resCh <- ok }()
	go func() { ok, _ := g.admit(context.Background(), "intruder-B"); resCh <- ok }()
	select {
	case <-resCh:
		t.Fatal("admit returned before the gate opened — data could flow pre-FUNDED")
	case <-time.After(150 * time.Millisecond):
	}

	g.open("seller-A")
	got := map[bool]int{}
	for range 2 {
		got[<-resCh]++
	}
	require.Equal(t, 1, got[true], "exactly the funded seller admitted")
	require.Equal(t, 1, got[false], "the unexpected peer rejected")

	// Post-open admissions resolve immediately.
	ok, expected := g.admit(context.Background(), "seller-A")
	require.True(t, ok)
	require.Equal(t, "seller-A", expected)
	ok, _ = g.admit(context.Background(), "intruder-C")
	require.False(t, ok)
}

// TestBuyerCommerce_EndToEnd_MemoryBackends drives the buyer CLI's
// --commerce-mode=full end-to-end in-process against a package-level seller:
// discovery off the shared registry → escrow funding → reverse setup →
// admission-gated stream → frame-limit → settlement → evidence.
func TestBuyerCommerce_EndToEnd_MemoryBackends(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	bus := topic.NewMemoryTopicAdapter()
	escrow := payment.NewMemoryEscrow()
	contract := registry.NewMemoryRegistryContract()
	regAddr, err := keylib.EVMAddressFromHex(testRegistryHex)
	require.NoError(t, err)

	// In-process seller: topics + commerce card registered on the shared
	// contract BEFORE the buyer starts (the buyer's lookup must succeed).
	sellerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	stdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stdin-e2e"})
	require.NoError(t, err)
	stdOut, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stdout-e2e"})
	require.NoError(t, err)
	stdErr, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stderr-e2e"})
	require.NoError(t, err)
	card, err := sapient.BuildSellerCard(sapient.SellerCardOptions{
		ChildKey: &sellerKey,
		Commerce: &sapient.CommerceCardOptions{
			SettlementBinding: sapient.SettlementBindingMemory,
			PricingAmount:     "7",
			TopicConfig: map[string]map[string]any{
				"stdIn":  {"topicId": stdIn.Locator()},
				"stdOut": {"topicId": stdOut.Locator()},
				"stdErr": {"topicId": stdErr.Locator()},
			},
			TopicTransport: "memory",
		},
	})
	require.NoError(t, err)
	contract.SetPendingOwner(common.BytesToAddress(sellerKey.PublicKey().EVMAddress().Bytes()))
	_, err = sapient.RegisterSeller(ctx, &sellerKey, card, regAddr, 0, contract)
	require.NoError(t, err)

	sellerECDSA, err := sellerKey.ToBlockchainKey()
	require.NoError(t, err)
	sellerHost, err := delivery.NewLibp2pHost(sellerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	type sellerOut struct {
		res sapient.SellerCommerceResult
		err error
	}
	sellerCh := make(chan sellerOut, 1)
	go func() {
		session, serr := sapient.StartSellerCommerce(ctx, sapient.SellerCommerceOptions{
			Key:           &sellerKey,
			Adapter:       bus,
			SellerStdIn:   stdIn,
			Host:          sellerHost,
			Escrow:        escrow,
			EscrowBinding: sapient.SettlementBindingMemory,
		})
		if serr != nil {
			sellerCh <- sellerOut{err: serr}
			return
		}
		if cerr := sellerHost.Connect(ctx, session.BuyerAddr); cerr != nil {
			sellerCh <- sellerOut{err: cerr}
			return
		}
		stream, serr2 := sellerHost.NewStream(ctx, session.BuyerAddr.ID, protocol.ID(sapient.ProtocolDetection))
		if serr2 != nil {
			sellerCh <- sellerOut{err: serr2}
			return
		}
		w := delivery.NewFrameWriter(stream)
		var sent uint64
		first := uint64(time.Now().UnixNano())
		for { // pump until the buyer resets at its frame-limit
			frame, merr := proto.Marshal(&sapientpb.SapientMessage{NodeId: proto.String("seller-e2e")})
			if merr != nil {
				sellerCh <- sellerOut{err: merr}
				return
			}
			if werr := w.WriteFrame(frame); werr != nil {
				break
			}
			sent++
			time.Sleep(5 * time.Millisecond)
		}
		last := uint64(time.Now().UnixNano())
		res, ferr := session.Finalise(ctx, func() (uint64, uint64, uint64) { return sent, first, last })
		sellerCh <- sellerOut{res: res, err: ferr}
	}()

	// The buyer CLI under test.
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	evidenceOut := filepath.Join(t.TempDir(), "buyer-commerce.json")
	deps := Deps{
		TopicAdapter:  bus,
		EscrowAdapter: escrow,
		ContractFactory: func(_ context.Context, _ string, _ keylib.EVMAddress) (registry.RegistryContract, error) {
			return contract, nil
		},
	}
	err = run([]string{
		"--commerce-mode", "full",
		"--key-hex", buyerKey.Hex(),
		"--listen", "/ip4/127.0.0.1/udp/0/quic-v1",
		"--sapient-edge", "127.0.0.1:0",
		"--seller-evm", sellerKey.PublicKey().EVMAddress().Hex(),
		"--registry-address", testRegistryHex,
		"--topic-backend", "memory", "--escrow-backend", "memory",
		"--frame-limit", "5",
		"--commerce-evidence-out", evidenceOut,
	}, io.Discard, deps)
	require.NoError(t, err, "buyer run() completes the session and exits")

	sout := <-sellerCh
	require.NoError(t, sout.err)
	require.Equal(t, payment.StateCompleted, sout.res.FinalState)
	require.Equal(t, "approved", sout.res.InvoiceAckAction)

	ev, rerr := sapient.ReadCommerceEvidence(evidenceOut)
	require.NoError(t, rerr)
	require.Equal(t, "buyer", ev.Role)
	require.Equal(t, "approved", ev.FinalAction)
	require.Equal(t, "7", ev.ReleasedAmount, "card-advertised pricing adopted and released in full")
	require.Equal(t, sellerKey.PublicKey().EVMAddress().Hex(), ev.ReleaseRecipient)
	require.Equal(t, uint64(5), ev.FrameCount, "frame-limit honoured")
	require.Equal(t, "memory", ev.TopicBackend)
	require.Equal(t, "memory", ev.EscrowBackend)
	require.Equal(t, "injected", ev.RegistryBackend)
	require.NotEmpty(t, ev.EscrowRef)
	require.NotEmpty(t, ev.DepositTx)
	require.NotEmpty(t, ev.Topics["sellerStdIn"])
	require.NotEmpty(t, ev.Topics["buyerStdIn"])
}
