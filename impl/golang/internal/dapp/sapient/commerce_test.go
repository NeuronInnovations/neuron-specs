package sapient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math/big"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

func TestParseAmount(t *testing.T) {
	v, err := parseAmount("5")
	require.NoError(t, err)
	require.Equal(t, int64(5), v.Int64())
	_, err = parseAmount("not-a-number")
	require.Error(t, err)
	_, err = parseAmount("-1")
	require.Error(t, err)
}

func TestComputeShortfall(t *testing.T) {
	require.Equal(t, int64(0), ComputeShortfall(big.NewInt(10), big.NewInt(5)).Int64())
	require.Equal(t, int64(0), ComputeShortfall(big.NewInt(5), big.NewInt(5)).Int64())
	require.Equal(t, int64(3), ComputeShortfall(big.NewInt(2), big.NewInt(5)).Int64())
}

func TestCommerceEvidence_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commerce.json")
	in := CommerceEvidence{
		RequestID: "req-1", Role: "buyer", Service: CommerceServiceName,
		Protocol: ProtocolDetection, TopicBackend: "memory", EscrowBackend: "memory",
		EscrowRef: "mem-escrow-1", ReleasedAmount: "5", FinalAction: "approved", FrameCount: 3,
	}
	require.NoError(t, WriteCommerceEvidence(path, in))
	out, err := ReadCommerceEvidence(path)
	require.NoError(t, err)
	require.Equal(t, in, out)
}

// TestBuildSellerCard_CommerceVariant: the Commerce options switch the rid
// entry to a real settlement advertisement + resolvable topic locators while
// keeping the card registry-valid and the neuron.rid/1 extension intact.
func TestBuildSellerCard_CommerceVariant(t *testing.T) {
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	card, err := BuildSellerCard(SellerCardOptions{
		ChildKey: &k,
		ChainID:  296,
		Commerce: &CommerceCardOptions{
			SettlementBinding: SettlementBindingEVM,
			EscrowContract:    "0x22c75CA7549046b5E3F9eb25dFF230F98974d38a",
			TokenContract:     "0xFE8518401a94EBD764C7095d7E45a474CFDE6756",
			PricingAmount:     "5",
			TopicConfig: map[string]map[string]any{
				"stdIn":  {"topicId": "0.0.111"},
				"stdOut": {"topicId": "0.0.222"},
				"stdErr": {"topicId": "0.0.333"},
			},
			TopicTransport: "hcs",
		},
	})
	require.NoError(t, err)

	valid, vErrs := registry.ValidateRegistrationCompleteness(card.AgentURI, k.PublicKey())
	require.True(t, valid, "commerce card must stay registry-valid: %v", vErrs)

	require.Equal(t, "0.0.111", TopicLocatorFor(card.AgentURI, "stdIn"))
	require.Equal(t, "0.0.222", TopicLocatorFor(card.AgentURI, "stdOut"))
	require.Equal(t, card.NodeID, ExtensionNodeID(card.AgentURI), "stdOut keeps the neuron.rid/1 extension")

	commerce := card.AgentURI.CommerceServices()
	require.Len(t, commerce, 1)
	require.Equal(t, SettlementBindingEVM, commerce[0].Settlement.Binding)
	require.Equal(t, "0x22c75CA7549046b5E3F9eb25dFF230F98974d38a", commerce[0].Settlement.Config["contract"])
	require.Equal(t, "0xFE8518401a94EBD764C7095d7E45a474CFDE6756", commerce[0].Settlement.Config["token"])
	require.Equal(t, uint64(296), commerce[0].Settlement.Config["chainId"])
	require.Equal(t, "5", commerce[0].Pricing.Amount)
	require.Equal(t, "NTT", commerce[0].Pricing.Currency)
}

// TestBuildSellerCard_DefaultUnchanged: without Commerce options the card is
// the original advertisement-only shape (settlement none, price 0, no topic
// locators, auditlane-file transport).
func TestBuildSellerCard_DefaultUnchanged(t *testing.T) {
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	card, err := BuildSellerCard(SellerCardOptions{ChildKey: &k})
	require.NoError(t, err)

	commerce := card.AgentURI.CommerceServices()
	require.Len(t, commerce, 1)
	require.Equal(t, "none", commerce[0].Settlement.Binding)
	require.Equal(t, "0", commerce[0].Pricing.Amount)
	require.Equal(t, "USDC", commerce[0].Pricing.Currency)
	require.Empty(t, TopicLocatorFor(card.AgentURI, "stdIn"))
	for _, ts := range card.AgentURI.TopicServices() {
		require.Equal(t, DefaultTopicTransport, ts.Transport)
	}
}

// newCommerceWorld wires the in-process world: shared memory topic bus,
// shared memory escrow, memory registry with the seller's COMMERCE card
// registered, plus two real loopback libp2p hosts.
func newCommerceWorld(t *testing.T, pricing string) (opts SellerCommerceOptions, bopts BuyerCommerceOptions, cleanup func()) {
	t.Helper()
	bus := topic.NewMemoryTopicAdapter()
	escrow := payment.NewMemoryEscrow()
	contract := registry.NewMemoryRegistryContract()

	sellerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// Seller topics first — their locators go into the commerce card.
	stdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stdin-test"})
	require.NoError(t, err)
	stdOut, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stdout-test"})
	require.NoError(t, err)
	stdErr, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stderr-test"})
	require.NoError(t, err)

	card, err := BuildSellerCard(SellerCardOptions{
		ChildKey: &sellerKey,
		Commerce: &CommerceCardOptions{
			SettlementBinding: SettlementBindingMemory,
			PricingAmount:     pricing,
			TopicConfig: map[string]map[string]any{
				"stdIn":  {"topicId": stdIn.Locator()},
				"stdOut": {"topicId": stdOut.Locator()},
				"stdErr": {"topicId": stdErr.Locator()},
			},
			TopicTransport: "memory",
		},
	})
	require.NoError(t, err)

	regAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	contract.SetPendingOwner(common.BytesToAddress(sellerKey.PublicKey().EVMAddress().Bytes()))
	_, err = RegisterSeller(context.Background(), &sellerKey, card, regAddr, 0, contract)
	require.NoError(t, err)

	sellerECDSA, err := sellerKey.ToBlockchainKey()
	require.NoError(t, err)
	buyerECDSA, err := buyerKey.ToBlockchainKey()
	require.NoError(t, err)
	sellerHost, err := delivery.NewLibp2pHost(sellerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)

	opts = SellerCommerceOptions{
		Key:           &sellerKey,
		Adapter:       bus,
		SellerStdIn:   stdIn,
		Host:          sellerHost,
		Escrow:        escrow,
		EscrowBinding: SettlementBindingMemory,
	}
	bopts = BuyerCommerceOptions{
		Key:             &buyerKey,
		Host:            buyerHost,
		Adapter:         bus,
		Escrow:          escrow,
		EscrowBinding:   SettlementBindingMemory,
		Contract:        contract,
		RegistryAddress: regAddr,
		ChainID:         0,
		SellerEVM:       sellerKey.PublicKey().EVMAddress(),
	}
	cleanup = func() {
		_ = sellerHost.Close()
		_ = buyerHost.Close()
	}
	return opts, bopts, cleanup
}

// TestCommerce_FullLifecycle_MemoryBackends drives the complete SAPIENT rid
// payment lifecycle in-process: request → accept → escrow fund → pre-stream
// GetBalance gate → reverse ConnectionSetup → real libp2p dial + 3 opaque
// frames → stop → invoice → approve → release to the SELLER → COMPLETED.
func TestCommerce_FullLifecycle_MemoryBackends(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sopts, bopts, cleanup := newCommerceWorld(t, "5")
	defer cleanup()

	var sellerLog bytes.Buffer
	sopts.Logger = log.New(&sellerLog, "", 0)
	var buyerLog bytes.Buffer
	bopts.Logger = log.New(&buyerLog, "", 0)

	// Buyer data plane: count inbound frames opaquely (vendor-blind M1).
	var framesReceived atomic.Uint64
	frameDone := make(chan struct{}, 1)
	bopts.Host.SetStreamHandler(protocol.ID(ProtocolDetection), func(stream libp2pnetwork.Stream) {
		defer stream.Close()
		r := delivery.NewFrameReader(stream)
		for {
			frame, err := r.ReadFrame()
			if err != nil {
				break
			}
			_ = frame
			if framesReceived.Add(1) == 3 {
				frameDone <- struct{}{}
			}
		}
	})

	// Seller side runs concurrently: session start (blocks for the request),
	// then dial + stream + finalise.
	type sellerOut struct {
		res SellerCommerceResult
		err error
	}
	sellerCh := make(chan sellerOut, 1)
	go func() {
		session, err := StartSellerCommerce(ctx, sopts)
		if err != nil {
			sellerCh <- sellerOut{err: err}
			return
		}
		// Data plane: dial the buyer (reverse-connect) and push 3 frames.
		if err := sopts.Host.Connect(ctx, session.BuyerAddr); err != nil {
			sellerCh <- sellerOut{err: fmt.Errorf("dial buyer: %w", err)}
			return
		}
		stream, err := sopts.Host.NewStream(ctx, session.BuyerAddr.ID, protocol.ID(ProtocolDetection))
		if err != nil {
			sellerCh <- sellerOut{err: fmt.Errorf("open stream: %w", err)}
			return
		}
		w := delivery.NewFrameWriter(stream)
		first := uint64(time.Now().UnixNano())
		for i := range 3 {
			if err := w.WriteFrame(fmt.Appendf(nil, "detection-%d", i)); err != nil {
				sellerCh <- sellerOut{err: fmt.Errorf("write frame: %w", err)}
				return
			}
		}
		last := uint64(time.Now().UnixNano())
		_ = stream.CloseWrite()
		res, err := session.Finalise(ctx, func() (uint64, uint64, uint64) { return 3, first, last })
		sellerCh <- sellerOut{res: res, err: err}
	}()

	// Buyer flow.
	bsession, err := StartBuyerCommerce(ctx, bopts)
	require.NoError(t, err)
	require.NotEmpty(t, bsession.ExpectedSellerPeerID)

	// Wait for the data plane to deliver the 3 frames, then settle.
	select {
	case <-frameDone:
	case <-ctx.Done():
		t.Fatalf("frames never arrived: %v (seller log:\n%s)", ctx.Err(), sellerLog.String())
	}
	bres, err := bsession.Finalise(ctx)
	require.NoError(t, err, "buyer finalise (buyer log:\n%s)", buyerLog.String())

	sout := <-sellerCh
	require.NoError(t, sout.err, "seller session (seller log:\n%s)", sellerLog.String())

	// Buyer-side evidence.
	require.Equal(t, "approved", bres.FinalAction)
	require.Equal(t, "5", bres.ReleasedAmount, "full price released")
	require.Equal(t, sopts.Key.PublicKey().EVMAddress().Hex(), bres.ReleaseRecipient,
		"release lands with the SELLER (paid service, not the remoteid free demo)")
	require.NotEmpty(t, bres.EscrowRef)
	require.NotEmpty(t, bres.DepositTx)
	require.Equal(t, "5", bres.InvoiceAmount)

	// Seller-side evidence.
	require.Equal(t, payment.StateCompleted, sout.res.FinalState)
	require.Equal(t, "5", sout.res.EscrowAvailable, "pre-stream gate observed the funded balance")
	require.Equal(t, "approved", sout.res.InvoiceAckAction)
	require.NotEmpty(t, sout.res.EvidenceHash)
	require.Equal(t, bopts.Key.PublicKey().EVMAddress().Hex(), sout.res.BuyerEVM)

	// The hard gate fired BEFORE any frame left the seller (structural: the
	// session only returns after the gate; the log records it).
	require.Contains(t, sellerLog.String(), "[escrow-verify] PASS")
	require.Equal(t, uint64(3), framesReceived.Load())
}

// TestCommerce_SellerRefusesUnderfundedEscrow: a buyer that deposits less
// than the agreed amount never gets a stream — the seller's pre-stream
// GetBalance gate fails closed.
func TestCommerce_SellerRefusesUnderfundedEscrow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sopts, bopts, cleanup := newCommerceWorld(t, "5")
	defer cleanup()
	var sellerLog bytes.Buffer
	sopts.Logger = log.New(&sellerLog, "", 0)

	type sellerOut struct {
		session *SellerCommerceSession
		err     error
	}
	sellerCh := make(chan sellerOut, 1)
	go func() {
		s, err := StartSellerCommerce(ctx, sopts)
		sellerCh <- sellerOut{session: s, err: err}
	}()

	// Dishonest buyer script: propose 5, deposit only 1.
	buyerStdIn, err := bopts.Adapter.CreateTopic(topic.CreateTopicOpts{Memo: "underfunded-buyer"})
	require.NoError(t, err)
	req, err := remoteid.BuildServiceRequest(remoteid.ServiceRequestOptions{
		RequestID:         "underfunded-1",
		ServiceRef:        P2PServiceName,
		SettlementBinding: SettlementBindingMemory,
		ProposedAmount:    "5",
		ProposedCurrency:  DefaultPricingCurrency,
		BuyerStdIn:        buyerStdIn.Locator(),
	})
	require.NoError(t, err)
	_, err = remoteid.PublishPayload(ctx, bopts.Adapter, sopts.SellerStdIn, bopts.Key, 1, req)
	require.NoError(t, err)
	resp, _, err := remoteid.ReceiveServiceResponse(ctx, bopts.Adapter, buyerStdIn, req.RequestID)
	require.NoError(t, err)
	require.Equal(t, "accept", resp.Action)

	agreementHash := sha256.Sum256([]byte("underfunded"))
	escrowRef, err := bopts.Escrow.CreateEscrow(ctx,
		bopts.Key.PublicKey().EVMAddress().Hex(), sopts.Key.PublicKey().EVMAddress().Hex(),
		nil, DefaultPricingCurrency, 1, agreementHash, uint64(time.Now().Unix())+3600)
	require.NoError(t, err)
	_, err = bopts.Escrow.Deposit(ctx, escrowRef, "1") // 1 < 5: underfunded
	require.NoError(t, err)
	created := remoteid.BuildEscrowCreated(req, escrowRef.Locator, "1", DefaultPricingCurrency)
	_, err = remoteid.PublishEscrowCreated(ctx, bopts.Adapter, sopts.SellerStdIn, bopts.Key, 2, created)
	require.NoError(t, err)

	sout := <-sellerCh
	require.Error(t, sout.err, "seller must refuse the underfunded escrow")
	require.Contains(t, sout.err.Error(), "refusing to stream")
	require.Nil(t, sout.session)
}

// TestStartBuyerCommerce_RejectsAdvertisementOnlyCard: a card without topic
// locators (the SIM advertisement card) cannot drive a payment session.
func TestStartBuyerCommerce_RejectsAdvertisementOnlyCard(t *testing.T) {
	ctx := context.Background()
	bus := topic.NewMemoryTopicAdapter()
	escrow := payment.NewMemoryEscrow()
	contract := registry.NewMemoryRegistryContract()

	sellerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	card, err := BuildSellerCard(SellerCardOptions{ChildKey: &sellerKey}) // advertisement-only
	require.NoError(t, err)
	regAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	contract.SetPendingOwner(common.BytesToAddress(sellerKey.PublicKey().EVMAddress().Bytes()))
	_, err = RegisterSeller(ctx, &sellerKey, card, regAddr, 0, contract)
	require.NoError(t, err)

	buyerECDSA, err := buyerKey.ToBlockchainKey()
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	_, err = StartBuyerCommerce(ctx, BuyerCommerceOptions{
		Key: &buyerKey, Host: buyerHost, Adapter: bus, Escrow: escrow,
		Contract: contract, RegistryAddress: regAddr, SellerEVM: sellerKey.PublicKey().EVMAddress(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no stdIn topic locator")
}
