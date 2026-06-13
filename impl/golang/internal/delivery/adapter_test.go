package delivery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- T006: DeliveryAdapter Type Tests ---

func TestDeliveryChannel_Construction(t *testing.T) {
	// FR-D02: DeliveryChannel with PeerID, protocol, transport.
	ch := &DeliveryChannel{
		ID:        "ch-001",
		PeerID:    "12D3KooWTest",
		Protocol:  "/neuron/adsb/1.0.0",
		Transport: "quic-v1",
	}

	assert.Equal(t, "ch-001", ch.ID)
	assert.Equal(t, "12D3KooWTest", ch.PeerID)
	assert.Equal(t, "/neuron/adsb/1.0.0", ch.Protocol)
	assert.Equal(t, "quic-v1", ch.Transport)
}

func TestDeliveryChannel_StateDefault(t *testing.T) {
	ch := &DeliveryChannel{}
	assert.Equal(t, StateIdle, ch.State(), "nil state machine defaults to IDLE")
}

func TestChannelStatus_Construction(t *testing.T) {
	// FR-D06: ChannelStatus snapshot.
	status := ChannelStatus{
		State:     StateConnected,
		Transport: "quic-v1",
	}
	assert.Equal(t, StateConnected, status.State)
	assert.Equal(t, "quic-v1", status.Transport)
}

func TestDataFrame_Construction(t *testing.T) {
	// FR-D04: DataFrame with data and receivedAt.
	now := time.Now()
	frame := DataFrame{
		Data:       []byte("sensor-data"),
		ReceivedAt: now,
	}
	assert.Equal(t, []byte("sensor-data"), frame.Data)
	assert.Equal(t, now, frame.ReceivedAt)
}

func TestSendResult_Construction(t *testing.T) {
	// FR-D03: SendResult with bytesSent.
	result := SendResult{BytesSent: 1024}
	assert.Equal(t, 1024, result.BytesSent)
}

func TestConnectOptions_Construction(t *testing.T) {
	cfg := DefaultBackoffConfig()
	opts := ConnectOptions{
		NATStatus:     "private",
		BackoffConfig: &cfg,
		RelayAddrs:    []string{"/ip4/relay/tcp/4001"},
	}
	assert.Equal(t, "private", opts.NATStatus)
	assert.NotNil(t, opts.BackoffConfig)
	assert.Len(t, opts.RelayAddrs, 1)
}
