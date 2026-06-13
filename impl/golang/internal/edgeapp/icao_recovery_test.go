package edgeapp

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuyer_ICAORecoveryEndToEnd seeds a stream with a DF=17 frame (plaintext
// ICAO) followed by a synthesized DF=4 frame whose AP field is parity ⊕ same
// ICAO. The buyer should:
//   - Observe the plaintext ICAO into its cache.
//   - Recover the same ICAO on the DF=4 frame and mark it Recovered=true.
func TestBuyer_ICAORecoveryEndToEnd(t *testing.T) {
	bus := NewMemoryBus()

	sellerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	mkTopic := func(memo string) topic.TopicRef {
		ref, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport(), Memo: memo})
		require.NoError(t, err)
		return ref
	}
	sIn, sOut, sErr := mkTopic("seller-stdIn"), mkTopic("seller-stdOut"), mkTopic("seller-stdErr")
	bIn, bOut, bErr := mkTopic("buyer-stdIn"), mkTopic("buyer-stdOut"), mkTopic("buyer-stdErr")

	sellerPub, err := sellerKey.PublicKey().ToBlockchainKey()
	require.NoError(t, err)

	const targetICAO = "3c656f"
	icaoBytes := []byte{0x3c, 0x65, 0x6f}

	// Build frames the seller will produce.
	plaintextDF17 := append([]byte{0x8D}, icaoBytes...) // DF=17, ICAO bytes 1-3
	plaintextDF17 = append(plaintextDF17, make([]byte, 10)...) // 14 bytes total

	// DF=4 short frame whose AP = CRC(body) ⊕ ICAO. Body is 4 arbitrary bytes.
	body := []byte{0x20, 0x00, 0x05, 0x9F}
	crc := feeds.ModeSCRC(body)
	icaoUint := uint32(icaoBytes[0])<<16 | uint32(icaoBytes[1])<<8 | uint32(icaoBytes[2])
	ap := crc ^ icaoUint
	df4 := append([]byte{}, body...)
	df4 = append(df4, byte(ap>>16), byte(ap>>8), byte(ap))
	require.Len(t, df4, 7)

	frames := [][]byte{plaintextDF17, df4}
	feedSrc := func(ctx context.Context, out chan<- feeds.FeedFrame) error {
		// Emit the two frames slowly so the cache has time to absorb the
		// plaintext one before the parity-XOR'd one arrives.
		for _, raw := range frames {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- feeds.FeedFrame{Raw: raw, Rx: time.Now().UTC()}:
			}
			time.Sleep(50 * time.Millisecond)
		}
		// Hold the source open until ctx cancel so the seller doesn't close.
		<-ctx.Done()
		return ctx.Err()
	}

	var got []AggregatedFrame
	var mu sync.Mutex
	onAgg := func(af AggregatedFrame) {
		mu.Lock()
		got = append(got, af)
		mu.Unlock()
	}

	logger := tlogger{t: t}

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()

	sellerCfg := SellerConfig{
		Bus: bus, PrivateKey: &sellerKey,
		StdIn: sIn, StdOut: sOut, StdErr: sErr,
		LibP2PListenAddr: "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:         DefaultProtocol,
		HeartbeatPeriod:  10 * time.Second,
		FeedSource:       feedSrc,
		Logger:           logger,
	}
	buyerCfg := BuyerConfig{
		Bus: bus, PrivateKey: &buyerKey,
		StdIn: bIn, StdOut: bOut, StdErr: bErr,
		Sellers:           []SellerEntry{{StdIn: sIn, PubKey: sellerPub, DisplayName: "test"}},
		LibP2PListenAddr:  "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:          DefaultProtocol,
		RequestID:         "icao-recovery-001",
		HeartbeatPeriod:   10 * time.Second,
		ReconnectBackoff:  500 * time.Millisecond,
		SellerDialTimeout: 5 * time.Second,
		OnAggregatedFrame: onAgg,
		Logger:            logger,
	}

	sellerDone := make(chan error, 1)
	buyerDone := make(chan error, 1)
	go func() { sellerDone <- RunSeller(ctx, sellerCfg) }()
	time.Sleep(200 * time.Millisecond)
	go func() { buyerDone <- RunBuyer(ctx, buyerCfg) }()

	// Wait until both frames have been received.
	deadline := time.After(6 * time.Second)
	var receivedCount atomic.Int32
	pollTick := time.NewTicker(100 * time.Millisecond)
	defer pollTick.Stop()
loop:
	for {
		mu.Lock()
		receivedCount.Store(int32(len(got)))
		mu.Unlock()
		if receivedCount.Load() >= 2 {
			break loop
		}
		select {
		case <-deadline:
			t.Fatalf("only got %d frames; want 2", receivedCount.Load())
		case <-pollTick.C:
		}
	}

	cancel()
	for _, ch := range []chan error{sellerDone, buyerDone} {
		select {
		case err := <-ch:
			if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
				t.Errorf("unexpected error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("process did not exit within 3s of cancel")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(got), 2)

	// First frame: DF=17, plaintext ICAO, Recovered=false.
	df17 := got[0]
	assert.Equal(t, byte(17), df17.Meta.DF)
	assert.Equal(t, targetICAO, df17.Meta.ICAO)
	assert.False(t, df17.Meta.Recovered, "DF=17 ICAO is plaintext, not recovered")

	// Second frame: DF=4, ICAO populated by recovery, Recovered=true.
	df4Got := got[1]
	assert.Equal(t, byte(4), df4Got.Meta.DF)
	assert.Equal(t, targetICAO, df4Got.Meta.ICAO,
		"DF=4 ICAO should be recovered from the cache seeded by the DF=17 frame")
	assert.True(t, df4Got.Meta.Recovered)
}

// TestBuyer_ICAORecoveryDisabled confirms DisableICAOCache=true takes effect:
// even though we send a plaintext-then-XOR'd pair, the second frame's ICAO
// stays empty.
func TestBuyer_ICAORecoveryDisabled(t *testing.T) {
	bus := NewMemoryBus()
	sellerKey, _ := keylib.NewNeuronPrivateKey()
	buyerKey, _ := keylib.NewNeuronPrivateKey()
	mkTopic := func(memo string) topic.TopicRef {
		ref, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport(), Memo: memo})
		require.NoError(t, err)
		return ref
	}
	sIn, sOut, sErr := mkTopic("a"), mkTopic("b"), mkTopic("c")
	bIn, bOut, bErr := mkTopic("d"), mkTopic("e"), mkTopic("f")
	sellerPub, _ := sellerKey.PublicKey().ToBlockchainKey()

	icaoBytes := []byte{0x40, 0x74, 0x12}
	icaoUint := uint32(icaoBytes[0])<<16 | uint32(icaoBytes[1])<<8 | uint32(icaoBytes[2])

	plaintextDF17 := append([]byte{0x8D}, icaoBytes...)
	plaintextDF17 = append(plaintextDF17, make([]byte, 10)...)

	body := []byte{0x20, 0x00, 0x05, 0x9F}
	crc := feeds.ModeSCRC(body)
	ap := crc ^ icaoUint
	df4 := append([]byte{}, body...)
	df4 = append(df4, byte(ap>>16), byte(ap>>8), byte(ap))

	frames := [][]byte{plaintextDF17, df4}
	feedSrc := func(ctx context.Context, out chan<- feeds.FeedFrame) error {
		for _, raw := range frames {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- feeds.FeedFrame{Raw: raw, Rx: time.Now().UTC()}:
			}
			time.Sleep(50 * time.Millisecond)
		}
		<-ctx.Done()
		return ctx.Err()
	}

	var got []AggregatedFrame
	var mu sync.Mutex
	onAgg := func(af AggregatedFrame) {
		mu.Lock()
		got = append(got, af)
		mu.Unlock()
	}
	logger := tlogger{t: t}

	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Second)
	defer cancel()

	go RunSeller(ctx, SellerConfig{
		Bus: bus, PrivateKey: &sellerKey,
		StdIn: sIn, StdOut: sOut, StdErr: sErr,
		LibP2PListenAddr: "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:         DefaultProtocol,
		HeartbeatPeriod:  10 * time.Second,
		FeedSource:       feedSrc,
		Logger:           logger,
	})
	time.Sleep(200 * time.Millisecond)
	go RunBuyer(ctx, BuyerConfig{
		Bus: bus, PrivateKey: &buyerKey,
		StdIn: bIn, StdOut: bOut, StdErr: bErr,
		Sellers:           []SellerEntry{{StdIn: sIn, PubKey: sellerPub, DisplayName: "test"}},
		LibP2PListenAddr:  "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:          DefaultProtocol,
		RequestID:         "icao-disabled-001",
		HeartbeatPeriod:   10 * time.Second,
		ReconnectBackoff:  500 * time.Millisecond,
		SellerDialTimeout: 5 * time.Second,
		OnAggregatedFrame: onAgg,
		DisableICAOCache:  true,
		Logger:            logger,
	})

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("only got %d frames", n)
		case <-time.After(100 * time.Millisecond):
		}
	}
	cancel()

	mu.Lock()
	defer mu.Unlock()
	df4Got := got[1]
	assert.Equal(t, byte(4), df4Got.Meta.DF)
	assert.Empty(t, df4Got.Meta.ICAO, "with cache disabled, DF=4 ICAO must remain empty")
	assert.False(t, df4Got.Meta.Recovered)
}
