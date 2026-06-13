package remoteid

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FR-R12-equivalent (the DApp-side replay/source contract): RunDS400
// MUST satisfy the FeedSource shape — i.e., the same function-typed
// signature RunReplay and RunSynth already use, so the DApp seller can
// plug DS-400 in without orchestration changes.

func TestRunDS400_ReturnsUnavailableWhenNoDecoder(t *testing.T) {
	// Not t.Parallel(): this test reads the package-global
	// frameDecoders map; the two tests that register decoders
	// (TestRegisterFrameDecoder_ReturnsPrevious +
	// TestRunDS400_ReturnsUnavailableEvenWithDecoder) also run in
	// the same package and would race on the map even under the
	// 2026-05-13 RWMutex protection (the lock prevents memory
	// races but not the logical "decoder visible at lookup time"
	// race). All three tests now run serially.

	// Sanity-check: no decoder is registered for these transports at
	// rest. (If a future test forgets to unregister, this test will
	// fail loudly.)
	for _, tr := range []DS400Transport{DS400TransportUDP, DS400TransportTCP, DS400TransportHTTP} {
		if got := LookupFrameDecoder(tr); got != nil {
			t.Fatalf("expected no decoder for %s at rest; got %T", tr, got)
		}
	}

	out := make(chan DecodedFrame, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := RunDS400(ctx, DS400Config{Transport: DS400TransportUDP, Address: "0.0.0.0:14550"}, out)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDS400Unavailable), "error must wrap ErrDS400Unavailable; got %v", err)
	assert.Contains(t, err.Error(), "transport=udp")
	assert.Contains(t, err.Error(), "address=0.0.0.0:14550")
	assert.Contains(t, err.Error(), "RegisterFrameDecoder", "error MUST point operators at the unblock path")

	// Stub MUST NOT have produced any frames.
	select {
	case f := <-out:
		t.Fatalf("expected no frames from stub; got %+v", f)
	default:
	}
}

func TestRunDS400_RejectsMissingTransport(t *testing.T) {
	t.Parallel()
	out := make(chan DecodedFrame, 1)
	err := RunDS400(context.Background(), DS400Config{Address: "x"}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Transport is required")
}

func TestRunDS400_RejectsMissingAddress(t *testing.T) {
	t.Parallel()
	out := make(chan DecodedFrame, 1)
	err := RunDS400(context.Background(), DS400Config{Transport: DS400TransportUDP}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Address is required")
}

func TestRunDS400_RejectsUnknownTransport(t *testing.T) {
	t.Parallel()
	out := make(chan DecodedFrame, 1)
	err := RunDS400(context.Background(), DS400Config{Transport: "ftp", Address: "x"}, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown DS-400 transport "ftp"`)
}

func TestRegisterFrameDecoder_ReturnsPrevious(t *testing.T) {
	// Not t.Parallel(): writes to the package-global frameDecoders
	// map; see TestRunDS400_ReturnsUnavailableWhenNoDecoder for
	// rationale.

	// Register a stand-in decoder and verify the previous (nil) value
	// is returned. Clean up by unregistering with nil.
	stub := func(payload []byte) ([]DecodedFrame, error) {
		return nil, nil
	}
	prev := RegisterFrameDecoder(DS400TransportUDP, stub)
	defer RegisterFrameDecoder(DS400TransportUDP, nil)
	assert.Nil(t, prev, "first registration MUST return nil")

	// Verify the lookup path returns the newly-registered decoder.
	got := LookupFrameDecoder(DS400TransportUDP)
	require.NotNil(t, got)

	// Replacing returns the prior decoder pointer.
	stub2 := func(payload []byte) ([]DecodedFrame, error) {
		return nil, nil
	}
	prev2 := RegisterFrameDecoder(DS400TransportUDP, stub2)
	require.NotNil(t, prev2, "replacement MUST return previous decoder")
}

// RunDS400_ReturnsUnavailableEvenWithDecoder asserts the documented
// Phase-2 stub semantics: even when a decoder is registered, the
// per-transport network read loop is NOT implemented yet and returns
// ErrDS400Unavailable. This is intentional — the swap-in for real
// device support requires both (a) decoder registration AND (b)
// implementing one of runDS400UDP/TCP/HTTP. When (b) lands, this test
// will need updating to assert the real read behavior.
func TestRunDS400_ReturnsUnavailableEvenWithDecoder(t *testing.T) {
	// Not t.Parallel(): writes to the package-global frameDecoders
	// map; see TestRunDS400_ReturnsUnavailableWhenNoDecoder for
	// rationale.

	stub := func(payload []byte) ([]DecodedFrame, error) {
		return nil, nil
	}
	defer RegisterFrameDecoder(DS400TransportTCP, nil)
	RegisterFrameDecoder(DS400TransportTCP, stub)

	out := make(chan DecodedFrame, 1)
	err := RunDS400(context.Background(), DS400Config{Transport: DS400TransportTCP, Address: "ds400.local:9100"}, out)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDS400Unavailable))
	assert.Contains(t, err.Error(), "network read loop not yet implemented")
}

func TestDS400Config_SourceTagDefaultsToVendor(t *testing.T) {
	t.Parallel()

	cfg := DS400Config{Transport: DS400TransportUDP, Address: "x"}
	assert.Equal(t, "dronescout-ds400", cfg.SourceTag())

	cfg.Source = "ds400-london-1"
	assert.Equal(t, "ds400-london-1", cfg.SourceTag())
}

// TestRunDS400_FeedSourceShape verifies the function signature
// matches the FeedSource contract used by RunReplay/RunSynth. If the
// signature drifts this test stops compiling — that's the point.
func TestRunDS400_FeedSourceShape(t *testing.T) {
	t.Parallel()
	// The line below would fail to compile if RunDS400's signature
	// stopped matching the (ctx, cfg, chan<-DecodedFrame) error shape.
	var _ func(context.Context, DS400Config, chan<- DecodedFrame) error = RunDS400
}

// TestDS400ErrorPointsToCheckList verifies that operators encountering
// the stub error learn what is required to unblock the stub.
func TestDS400ErrorPointsToCheckList(t *testing.T) {
	t.Parallel()
	assert.True(t, strings.Contains(ErrDS400Unavailable.Error(), "capture fixtures required"),
		"the sentinel error MUST state the unblock requirement")
}
