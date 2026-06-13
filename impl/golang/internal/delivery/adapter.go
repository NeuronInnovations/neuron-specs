package delivery

import "time"

// DeliveryAdapter is the abstract interface for data plane delivery bindings.
// FR-D01: All delivery bindings MUST implement these five operations.
// Analogous to Spec 004's TopicAdapter for the control plane.
type DeliveryAdapter interface {
	// Connect establishes a delivery channel to the specified peer. FR-D02.
	Connect(peerID string, multiaddrs []string, protocol string, opts *ConnectOptions) (*DeliveryChannel, error)

	// Send transmits a data frame over the delivery channel. FR-D03.
	Send(channel *DeliveryChannel, data []byte) (*SendResult, error)

	// Receive returns the next data frame from the delivery channel. FR-D04.
	// Blocks until a frame is available or the channel is closed.
	Receive(channel *DeliveryChannel) (*DataFrame, error)

	// Disconnect closes the delivery channel gracefully. FR-D05.
	Disconnect(channel *DeliveryChannel) error

	// GetStatus returns the current state of the delivery channel. FR-D06.
	GetStatus(channel *DeliveryChannel) ChannelStatus
}

// ConnectOptions provides optional parameters for Connect.
type ConnectOptions struct {
	NATStatus     string // "public", "private", "unknown"
	BackoffConfig *BackoffConfig
	RelayAddrs    []string // static relay multiaddrs
}

// ConnectionPath captures the libp2p connection details backing a stream.
// Limited=true indicates a relay-limited connection (in practice, circuit-v2).
type ConnectionPath struct {
	LocalMultiaddr  string
	RemoteMultiaddr string
	Limited         bool
}

// DeliveryChannel is an opaque handle for an active delivery channel.
// FR-D02: returned by Connect, used by subsequent operations.
type DeliveryChannel struct {
	ID        string         // unique channel identifier
	PeerID    string         // remote peer identity
	Protocol  string         // stream protocol ID
	Transport string         // active transport (e.g., "quic-v1", "webrtc")
	Path      ConnectionPath // concrete libp2p connection path for this stream
	state     *ConnectionStateMachine
}

// State returns the current connection state of this channel.
func (ch *DeliveryChannel) State() ConnectionState {
	if ch.state == nil {
		return StateIdle
	}
	return ch.state.State()
}

// ChannelStatus is a read-only snapshot of a delivery channel's state. FR-D06.
type ChannelStatus struct {
	State     ConnectionState
	Transport string
	Path      ConnectionPath
}

// DataFrame is a length-prefixed data unit received on a delivery channel.
// FR-D04: includes data bytes and receivedAt timestamp.
type DataFrame struct {
	Data       []byte
	ReceivedAt time.Time
}

// SendResult is the result of a Send operation. FR-D03.
type SendResult struct {
	BytesSent int
}
