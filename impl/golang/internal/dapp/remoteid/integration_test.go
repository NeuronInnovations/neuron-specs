package remoteid

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/edgeapp"
	remoteidsrc "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// testWriter pipes log output into t.Logf for easy debugging.
type testWriter struct {
	t      *testing.T
	prefix string
}

func (w *testWriter) Write(p []byte) (int, error) {
	w.t.Logf("[%s] %s", w.prefix, strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// taggedFrame mirrors the on-the-wire envelope that cmd/multistream-buyer
// emits to the JSONL sink. Replicated here (rather than imported from
// cmd/) because cmd packages should not be imported by tests; the
// envelope is intentionally simple JSON.
type taggedFrame struct {
	Source       string        `json:"source"`
	Type         string        `json:"type"`
	SellerPeerID string        `json:"sellerPeerID"`
	ReceivedAt   time.Time     `json:"receivedAt"`
	Frame        RemoteIdFrame `json:"frame"`
}

// TestPhase2_FixtureVerticalSlice is the Phase-2 end-to-end visible-proof
// test: a Remote ID seller backed by a JSON fixture replay source, a
// buyer that dials the seller and opens /ds240/raw/1.0.0, and a
// tagged JSONL file sink that captures the dual-stream output the React/
// FID display will consume in Phase 5. Asserts that the file contains
// well-formed tagged records with source="remote-id" + type="drone" +
// inner canonical RemoteIdFrame.
func TestPhase2_FixtureVerticalSlice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// Seller host + DApp seller with replay source (Phase 0 P0.4 fixture).
	sellerKey := makeECDSAKey(t)
	sellerHost, err := delivery.NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	running, err := Start(ctx, SellerConfig{
		Host: sellerHost,
		Source: ReplaySource(
			"../../feeds/remoteid/testdata/two-drones.json",
			remoteidsrc.ReplayOptions{FrameInterval: 5 * time.Millisecond, Loop: true},
		),
	})
	require.NoError(t, err)
	defer running.Cancel()

	// Buyer host opens the protocol and reads frames.
	buyerKey := makeECDSAKey(t)
	buyerHost, err := delivery.NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	require.NoError(t, buyerHost.Connect(ctx, peer.AddrInfo{
		ID:    sellerHost.ID(),
		Addrs: sellerHost.Addrs(),
	}))

	stream, err := buyerHost.NewStream(ctx, sellerHost.ID(), protocol.ID(ProtocolRaw))
	require.NoError(t, err)
	defer stream.Close()

	// Tagged JSONL sink — file under the test's TempDir.
	tmpFile := filepath.Join(t.TempDir(), "drones.jsonl")
	sink, err := edgeapp.NewTaggedJSONLSink("file:" + tmpFile)
	require.NoError(t, err)

	// Pump frames buyer-side, wrap each in the tagged envelope, emit.
	captured := pumpFramesToSink(t, stream, sink, sellerHost.ID().String(), 8)
	require.NoError(t, sink.Close())

	// Verify file contains tagged records with the expected shape.
	body := mustReadFile(t, tmpFile)
	lines := splitNonEmpty(body)
	require.GreaterOrEqual(t, len(lines), captured, "file line count must match captured count")

	for i, line := range lines {
		var tf taggedFrame
		require.NoError(t, json.Unmarshal([]byte(line), &tf), "line %d malformed", i)
		assert.Equal(t, "remote-id", tf.Source)
		assert.Equal(t, "drone", tf.Type)
		assert.NotEmpty(t, tf.SellerPeerID)
		assert.Equal(t, FrameType, tf.Frame.Type)
		assert.Equal(t, FrameVersion, tf.Frame.Version)
		assert.NotEmpty(t, tf.Frame.DroneID)
		assert.NotEmpty(t, tf.Frame.DroneIDType)
	}
}

// pumpFramesToSink reads up to maxFrames RemoteIdFrame payloads off the
// stream, wraps each in a taggedFrame, emits to sink, and returns the
// count captured.
func pumpFramesToSink(t *testing.T, stream libp2pnetwork.Stream, sink edgeapp.TaggedSink, sellerPID string, maxFrames int) int {
	t.Helper()
	reader := delivery.NewFrameReader(stream)
	deadline := time.Now().Add(3 * time.Second)
	var captured int
	for captured < maxFrames && time.Now().Before(deadline) {
		stream.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		data, err := reader.ReadFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			// likely a read deadline; loop until outer deadline.
			continue
		}
		var frame RemoteIdFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			t.Logf("malformed frame skipped: %v", err)
			continue
		}
		tf := taggedFrame{
			Source:       "remote-id",
			Type:         "drone",
			SellerPeerID: sellerPID,
			ReceivedAt:   time.Now().UTC(),
			Frame:        frame,
		}
		require.NoError(t, sink.Emit(context.Background(), tf))
		captured++
	}
	require.GreaterOrEqual(t, captured, 4, "expected at least 4 frames in 3s; got %d", captured)
	return captured
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := readFile(path)
	require.NoError(t, err)
	return string(data)
}

func splitNonEmpty(s string) []string {
	out := strings.Split(strings.TrimRight(s, "\n"), "\n")
	res := out[:0]
	for _, l := range out {
		if l != "" {
			res = append(res, l)
		}
	}
	return res
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// TestFullCommerceLifecycle drives the full Stage-2 / Items-1+2+3 flow
// over a memory topic adapter + memory escrow, end-to-end:
//
//	ServiceRequest → ServiceResponse(accept) → EscrowCreated → ConnectionSetup →
//	dial → frames → ServiceStop → Invoice → InvoiceAck → COMPLETED
//
// Wrapper: Test_FR_P13_FullCommerceLifecycle.
func Test_FR_P13_FullCommerceLifecycle(t *testing.T) {
	t.Run("end-to-end", TestFullCommerceLifecycle)
}

func TestFullCommerceLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// --- Topic bus + escrow ---
	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(64)
	escrow := payment.NewMemoryEscrow()

	SetReceiveTraceLogger(log.New(&testWriter{t: t, prefix: "recv"}, "", 0))
	t.Cleanup(func() { SetReceiveTraceLogger(nil) })

	sellerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "seller-stdin"})
	require.NoError(t, err)
	sellerStdOut, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "seller-stdout"})
	require.NoError(t, err)
	buyerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "buyer-stdin"})
	require.NoError(t, err)

	// --- Keys ---
	sellerKey := newTestKey(t)
	buyerKey := newTestKey(t)
	sellerEcdsaPriv, _ := mustECDSAKeys(t, sellerKey)
	buyerEcdsaPriv, _ := mustECDSAKeys(t, buyerKey)

	// --- Seller libp2p host ---
	sellerHost, err := delivery.NewLibp2pHost(sellerEcdsaPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	// Build the descriptor BEFORE starting the seller session because we
	// need a Streams catalog on the wire.
	descriptor, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:   sellerKey,
		ChainID:    296,
		FeedSource: FeedSourceSynthetic,
	})
	require.NoError(t, err)

	// --- Seller's libp2p frame handler ---
	running, err := Start(ctx, SellerConfig{
		Host:   sellerHost,
		Source: SynthSource(remoteidsrc.SynthOptions{FPS: 50, DroneCount: 1}),
	})
	require.NoError(t, err)
	defer running.Cancel()

	// --- Seller commerce session goroutine ---
	type sellerResult struct {
		res SellerSessionResult
		err error
	}
	sellerResultCh := make(chan sellerResult, 1)
	sellerLogger := log.New(&testWriter{t: t, prefix: "seller"}, "", 0)
	go func() {
		res, err := RunSellerSession(ctx, SellerSessionOptions{
			Key:         sellerKey,
			Adapter:     bus,
			SellerStdIn: sellerStdIn,
			Descriptor:  descriptor,
			Host:        sellerHost,
			Escrow:      escrow,
			Mode:        CommerceModeFull,
			Logger:      sellerLogger,
			FrameSummary: func() (uint64, uint64, uint64) {
				return 8, 1, 2
			},
		})
		sellerResultCh <- sellerResult{res, err}
	}()

	// --- Buyer ---
	buyerLogger := log.New(&testWriter{t: t, prefix: "buyer"}, "", 0)
	buyerOpts := BuyerSessionOptions{
		Key:                  buyerKey,
		EcdsaPriv:            buyerEcdsaPriv,
		Adapter:              bus,
		SellerStdIn:          sellerStdIn,
		BuyerStdIn:           buyerStdIn,
		RequestID:            "req-int-1",
		ExpectedSellerPeerID: sellerHost.ID().String(),
		Mode:                 CommerceModeFull,
		Escrow:               escrow,
		SellerEVM:            sellerKey.PublicKey().EVMAddress().Hex(),
		Logger:               buyerLogger,
	}

	// Tiny pause to let the seller's ReceiveServiceRequest subscribe.
	time.Sleep(50 * time.Millisecond)

	partial, err := RunBuyerSession(ctx, buyerOpts)
	require.NoError(t, err)
	require.NotNil(t, partial.Discovery, "buyer must receive ConnectionSetup")
	require.NotEmpty(t, partial.Discovery.Streams, "FR-R02 streams[] catalog on wire")
	assert.Equal(t, ProtocolRaw, partial.Discovery.Streams[0].ProtocolID)

	// Dial seller host using the descriptor's PeerID + the seller's
	// known multiaddrs. (The buyer already received decrypted multiaddrs
	// inside RunBuyerSession; for the integration test we dial directly
	// for simplicity since both sides are in-process.)
	_ = sellerStdOut
	buyerHost, err := delivery.NewLibp2pHost(buyerEcdsaPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()
	require.NoError(t, buyerHost.Connect(ctx, peer.AddrInfo{
		ID:    sellerHost.ID(),
		Addrs: sellerHost.Addrs(),
	}))
	stream, err := buyerHost.NewStream(ctx, sellerHost.ID(), protocol.ID(partial.Discovery.Streams[0].ProtocolID))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	// Read a handful of frames; the synth source publishes continuously
	// so 4 frames is comfortable within the 10s timeout.
	reader := delivery.NewFrameReader(stream)
	for i := 0; i < 4; i++ {
		_ = stream.SetReadDeadline(time.Now().Add(2 * time.Second))
		data, err := reader.ReadFrame()
		require.NoError(t, err, "buyer should be able to read frame %d", i)
		require.NotEmpty(t, data)
	}
	_ = stream.Close()

	// --- Buyer settles ---
	// Sequence numbers continue from where RunBuyerSession left off; we
	// used seq up to ~3 (ServiceRequest=1, EscrowCreated=2). Start from 3.
	final, err := FinaliseBuyerSession(ctx, buyerOpts, partial, 3)
	require.NoError(t, err)
	assert.Equal(t, "approved", final.FinalAction)

	// --- Seller session result ---
	select {
	case sr := <-sellerResultCh:
		require.NoError(t, sr.err, "seller session must succeed")
		assert.Equal(t, payment.StateCompleted, sr.res.FinalState,
			"full-commerce lifecycle ends in COMPLETED")
		assert.NotEmpty(t, sr.res.EvidenceHash)
	case <-time.After(5 * time.Second):
		t.Fatal("seller session did not complete within 5s")
	}
}

func TestRunFullCommerceFlow_InvokesOutputCallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(64)
	escrow := payment.NewMemoryEscrow()
	contract := registry.NewMemoryRegistryContract()

	sellerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "remoteid-seller-stdin"})
	require.NoError(t, err)
	sellerStdOut, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "remoteid-seller-stdout"})
	require.NoError(t, err)
	sellerStdErr, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "remoteid-seller-stderr"})
	require.NoError(t, err)

	sellerKey := newTestKey(t)
	buyerKey := newTestKey(t)
	sellerEcdsaPriv, _ := mustECDSAKeys(t, sellerKey)
	buyerEcdsaPriv, _ := mustECDSAKeys(t, buyerKey)

	sellerHost, err := delivery.NewLibp2pHost(sellerEcdsaPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()
	require.NotEmpty(t, sellerHost.Addrs())

	descriptor, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:     sellerKey,
		ChainID:      296,
		FeedSource:   FeedSourceSynthetic,
		CommerceMode: CommerceModeFull,
		TopicConfig: map[string]map[string]any{
			"stdIn":  {"topicId": sellerStdIn.Locator()},
			"stdOut": {"topicId": sellerStdOut.Locator()},
			"stdErr": {"topicId": sellerStdErr.Locator()},
		},
		Catalog: CatalogOptions{IncludeBasestation: true},
	})
	require.NoError(t, err)

	contract.SetPendingOwner(common.BytesToAddress(sellerKey.PublicKey().EVMAddress().Bytes()))
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	_, err = RegisterSeller(ctx, sellerKey, descriptor, registryAddr, 296, contract)
	require.NoError(t, err)

	running, err := Start(ctx, SellerConfig{
		Host:        sellerHost,
		Source:      SynthSource(remoteidsrc.SynthOptions{FPS: 50, DroneCount: 1}),
		ProtocolIDs: []string{ProtocolRaw, ProtocolBasestation},
	})
	require.NoError(t, err)
	defer running.Cancel()

	type sellerResult struct {
		res SellerSessionResult
		err error
	}
	sellerResultCh := make(chan sellerResult, 1)
	go func() {
		res, err := RunSellerSession(ctx, SellerSessionOptions{
			Key:         sellerKey,
			Adapter:     bus,
			SellerStdIn: sellerStdIn,
			Descriptor:  descriptor,
			Host:        sellerHost,
			Escrow:      escrow,
			Mode:        CommerceModeFull,
			Logger:      log.New(&testWriter{t: t, prefix: "seller"}, "", 0),
			FrameSummary: func() (uint64, uint64, uint64) {
				return 4, 1, 1
			},
		})
		sellerResultCh <- sellerResult{res, err}
	}()

	buyerHost, err := delivery.NewLibp2pHost(buyerEcdsaPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	var callbackCount int
	var callbackPeer peer.ID
	final, err := RunFullCommerceFlow(ctx, FullCommerceFlowOpts{
		Logger:           log.New(&testWriter{t: t, prefix: "buyer"}, "", 0),
		Key:              buyerKey,
		EcdsaPriv:        buyerEcdsaPriv,
		BuyerHost:        buyerHost,
		Adapter:          bus,
		Escrow:           escrow,
		EscrowBinding:    "memory",
		Contract:         contract,
		RegistryAddress:  registryAddr,
		ChainID:          296,
		SellerEVM:        sellerKey.PublicKey().EVMAddress(),
		PricingAmount:    "1",
		FrameLimit:       3,
		SellerMaOverride: sellerHost.Addrs()[0].String() + "/p2p/" + sellerHost.ID().String(),
		AllowMaOverride:  true,
	}, func(frame RemoteIdFrame, sellerPID peer.ID) error {
		callbackCount++
		callbackPeer = sellerPID
		assert.Equal(t, FrameType, frame.Type)
		assert.Equal(t, FrameVersion, frame.Version)
		assert.NotEmpty(t, frame.DroneID)
		assert.NotEmpty(t, frame.DroneIDType)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "approved", final.FinalAction)
	assert.GreaterOrEqual(t, callbackCount, 3)
	assert.Equal(t, sellerHost.ID(), callbackPeer)

	select {
	case sr := <-sellerResultCh:
		require.NoError(t, sr.err)
		assert.Equal(t, payment.StateCompleted, sr.res.FinalState)
	case <-time.After(5 * time.Second):
		t.Fatal("seller session did not complete")
	}
}
