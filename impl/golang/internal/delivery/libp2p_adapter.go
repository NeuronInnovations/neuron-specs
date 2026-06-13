package delivery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
)

// Libp2pAdapter implements DeliveryAdapter using a libp2p host.
// DD-D02: Host is shared per-agent across multiple channels.
type Libp2pAdapter struct {
	host          host.Host
	channels      map[string]*activeChannel
	mu            sync.RWMutex
	nextID        int
	backoffConfig BackoffConfig
}

// activeChannel tracks a single delivery channel's runtime state.
type activeChannel struct {
	channel   *DeliveryChannel
	stream    network.Stream
	writer    *FrameWriter
	reader    *FrameReader
	stateMach *ConnectionStateMachine
	cancel    context.CancelFunc
}

// Libp2pAdapterOption is a functional option for NewLibp2pAdapter. FR-D09.
type Libp2pAdapterOption func(*Libp2pAdapter)

// WithBackoffConfig sets a custom backoff configuration. FR-D09.
// Default uses spec parameters (5s/2x/10min/1hr).
func WithBackoffConfig(cfg BackoffConfig) Libp2pAdapterOption {
	return func(a *Libp2pAdapter) {
		a.backoffConfig = cfg
	}
}

// NewLibp2pAdapter creates a DeliveryAdapter backed by the given libp2p host.
func NewLibp2pAdapter(h host.Host, opts ...Libp2pAdapterOption) *Libp2pAdapter {
	a := &Libp2pAdapter{
		host:          h,
		channels:      make(map[string]*activeChannel),
		backoffConfig: DefaultBackoffConfig(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// BackoffConfig returns the current backoff configuration.
func (a *Libp2pAdapter) BackoffConfig() BackoffConfig {
	return a.backoffConfig
}

// Connect establishes a delivery channel to the specified peer. FR-D02.
// Parses multiaddrs, dials the peer, opens a stream with the protocol ID,
// wraps with FrameWriter/FrameReader, verifies PeerID (FR-D28).
func (a *Libp2pAdapter) Connect(peerID string, multiaddrs []string, proto string, opts *ConnectOptions) (*DeliveryChannel, error) {
	const op = "Connect"

	// Parse peerID.
	pid, err := peer.Decode(peerID)
	if err != nil {
		return nil, NewDeliveryError(ErrDialFailed, op,
			fmt.Sprintf("invalid PeerID: %v", err))
	}

	// Parse multiaddrs.
	addrs := make([]ma.Multiaddr, 0, len(multiaddrs))
	for _, addrStr := range multiaddrs {
		maddr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			return nil, NewDeliveryError(ErrInvalidMultiaddr, op,
				fmt.Sprintf("invalid multiaddr %q: %v", addrStr, err))
		}
		addrs = append(addrs, maddr)
	}

	// Create state machine with callback.
	var stateCallback StateChangeCallback
	sm := NewConnectionStateMachine(stateCallback)

	// Transition: IDLE → CONNECTING
	sm.Transition(EventConnect, "dial initiated")

	// Add peer addresses to host's peerstore.
	a.host.Peerstore().AddAddrs(pid, addrs, time.Hour)

	// Dial and open stream. WithAllowLimitedConn permits the stream to use a
	// circuit-v2 (relay) connection that libp2p marks as transient until/unless
	// DCUtR upgrades it. Without this, relay-assisted delivery streams time out.
	ctx := network.WithAllowLimitedConn(context.Background(), "neuron-delivery")
	stream, err := a.host.NewStream(ctx, pid, protocol.ID(proto))
	if err != nil {
		sm.Transition(EventDialFailAll, err.Error())
		return nil, NewDeliveryError(ErrDialFailed, op,
			fmt.Sprintf("dial failed: %v", err))
	}

	// FR-D28: Verify remote PeerID matches expected.
	remotePeerID := stream.Conn().RemotePeer()
	if remotePeerID != pid {
		stream.Reset()
		sm.Transition(EventDialFailAll, "PeerID mismatch")
		return nil, NewDeliveryError(ErrPeerIDMismatch, op,
			fmt.Sprintf("expected %s, got %s", pid, remotePeerID))
	}

	// Transition: CONNECTING → CONNECTED
	sm.Transition(EventDialSuccess, "direct dial succeeded")

	// Protect this peer in the connection manager so the underlying
	// connection isn't garbage-collected while the stream is open. Mirrors
	// the burst-investigation findings: prevents the 0x1005 ConnGarbageCollected
	// teardown waves observed in production under fan-in.
	if cm := a.host.ConnManager(); cm != nil {
		cm.Protect(pid, ActiveStreamProtectTag)
	}

	// Detect transport from connection.
	transport := detectTransport(stream.Conn())

	// Create channel.
	a.mu.Lock()
	a.nextID++
	chanID := fmt.Sprintf("ch-%d", a.nextID)
	a.mu.Unlock()

	ch := &DeliveryChannel{
		ID:        chanID,
		PeerID:    peerID,
		Protocol:  proto,
		Transport: transport,
		Path:      describeConnection(stream.Conn()),
		state:     sm,
	}

	ac := &activeChannel{
		channel:   ch,
		stream:    stream,
		writer:    NewFrameWriter(stream),
		reader:    NewFrameReader(stream),
		stateMach: sm,
	}

	a.mu.Lock()
	a.channels[chanID] = ac
	a.mu.Unlock()

	return ch, nil
}

// Send transmits a data frame over the delivery channel. FR-D03.
func (a *Libp2pAdapter) Send(channel *DeliveryChannel, data []byte) (*SendResult, error) {
	const op = "Send"

	ac, err := a.getChannel(channel.ID)
	if err != nil {
		return nil, err
	}

	if err := ac.writer.WriteFrame(data); err != nil {
		return nil, WrapDeliveryError(ErrStreamError, op, err)
	}

	return &SendResult{BytesSent: len(data)}, nil
}

// Receive reads the next data frame from the delivery channel. FR-D04.
// Blocks until a frame is available or the channel is closed.
func (a *Libp2pAdapter) Receive(channel *DeliveryChannel) (*DataFrame, error) {
	const op = "Receive"

	ac, err := a.getChannel(channel.ID)
	if err != nil {
		return nil, err
	}

	data, err := ac.reader.ReadFrame()
	if err != nil {
		return nil, WrapDeliveryError(ErrStreamError, op, err)
	}

	return &DataFrame{
		Data:       data,
		ReceivedAt: time.Now(),
	}, nil
}

// Disconnect closes the delivery channel gracefully. FR-D05.
func (a *Libp2pAdapter) Disconnect(channel *DeliveryChannel) error {
	ac, err := a.getChannel(channel.ID)
	if err != nil {
		return err
	}

	// Release the connection manager's Protect tag so the conn becomes
	// eligible for the next prune cycle. Best-effort: the remote PeerID
	// may be unparseable (test fakes), in which case we just skip — it's
	// a hint to the connmgr, not a correctness invariant.
	if cm := a.host.ConnManager(); cm != nil {
		if pid, perr := peer.Decode(channel.PeerID); perr == nil {
			cm.Unprotect(pid, ActiveStreamProtectTag)
		}
	}

	// Close stream.
	ac.stream.Close()

	// Drive state machine to DISCONNECTED.
	ac.stateMach.Transition(EventDisconnect, "explicit disconnect")

	// Remove from channels map.
	a.mu.Lock()
	delete(a.channels, channel.ID)
	a.mu.Unlock()

	return nil
}

// GetStatus returns the current state of the delivery channel. FR-D06.
func (a *Libp2pAdapter) GetStatus(channel *DeliveryChannel) ChannelStatus {
	ac, err := a.getChannel(channel.ID)
	if err != nil {
		return ChannelStatus{State: StateDisconnected}
	}

	return ChannelStatus{
		State:     ac.stateMach.State(),
		Transport: channel.Transport,
		Path:      describeConnection(ac.stream.Conn()),
	}
}

// HandleIncoming registers a stream handler for incoming delivery connections.
// When a remote peer opens a stream with the given protocol, the handler is
// called with a new DeliveryChannel.
func (a *Libp2pAdapter) HandleIncoming(protocolID protocol.ID, handler func(*DeliveryChannel)) {
	a.host.SetStreamHandler(protocolID, func(stream network.Stream) {
		// Create state machine.
		sm := NewConnectionStateMachine(nil)
		sm.Transition(EventConnect, "incoming stream")
		sm.Transition(EventDialSuccess, "accepted incoming")

		// Protect this peer in the connection manager — mirrors the dialer
		// path in Connect. See note there for context.
		if cm := a.host.ConnManager(); cm != nil {
			cm.Protect(stream.Conn().RemotePeer(), ActiveStreamProtectTag)
		}

		transport := detectTransport(stream.Conn())

		a.mu.Lock()
		a.nextID++
		chanID := fmt.Sprintf("ch-%d", a.nextID)
		a.mu.Unlock()

		ch := &DeliveryChannel{
			ID:        chanID,
			PeerID:    stream.Conn().RemotePeer().String(),
			Protocol:  string(protocolID),
			Transport: transport,
			Path:      describeConnection(stream.Conn()),
			state:     sm,
		}

		ac := &activeChannel{
			channel:   ch,
			stream:    stream,
			writer:    NewFrameWriter(stream),
			reader:    NewFrameReader(stream),
			stateMach: sm,
		}

		a.mu.Lock()
		a.channels[chanID] = ac
		a.mu.Unlock()

		handler(ch)
	})
}

// StartReconnection begins an asynchronous reconnection attempt for a channel.
// FR-D08, FR-D09, FR-D10: On stream drop, transition to RECONNECTING, attempt
// re-dial with exponential backoff, fire state change events.
func (a *Libp2pAdapter) StartReconnection(channel *DeliveryChannel) {
	ac, err := a.getChannel(channel.ID)
	if err != nil {
		return
	}

	// Only reconnect from CONNECTED or RELAYING.
	currentState := ac.stateMach.State()
	if currentState != StateConnected && currentState != StateRelaying {
		return
	}

	// Transition to RECONNECTING.
	if currentState == StateConnected {
		ac.stateMach.Transition(EventTransportDrop, "stream error detected")
	} else {
		ac.stateMach.Transition(EventRelayDrop, "relay stream error detected")
	}

	// Close old stream.
	ac.stream.Close()

	// Launch reconnection goroutine.
	go a.reconnectionLoop(ac)
}

// reconnectionLoop attempts to re-establish the stream with exponential backoff.
func (a *Libp2pAdapter) reconnectionLoop(ac *activeChannel) {
	cfg := a.backoffConfig
	start := time.Now()

	pid, err := peer.Decode(ac.channel.PeerID)
	if err != nil {
		ac.stateMach.Transition(EventBackoffExhausted, "invalid PeerID for reconnection")
		return
	}

	for attempt := 0; ; attempt++ {
		elapsed := time.Since(start)
		if cfg.IsExhausted(elapsed) {
			ac.stateMach.Transition(EventBackoffExhausted,
				fmt.Sprintf("max duration %v exceeded after %d attempts", cfg.MaxDuration, attempt))
			return
		}

		delay := cfg.NextDelay(attempt)
		time.Sleep(delay)

		// Attempt re-dial. WithAllowLimitedConn permits the stream to use a
		// relay-assisted connection that libp2p marks as transient.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		ctx = network.WithAllowLimitedConn(ctx, "neuron-delivery-reconnect")
		stream, dialErr := a.host.NewStream(ctx, pid, protocol.ID(ac.channel.Protocol))
		cancel()

		if dialErr != nil {
			continue // Try again after next backoff delay.
		}

		// Re-wire stream.
		ac.stream = stream
		ac.writer = NewFrameWriter(stream)
		ac.reader = NewFrameReader(stream)

		// Detect new transport.
		ac.channel.Transport = detectTransport(stream.Conn())
		ac.channel.Path = describeConnection(stream.Conn())

		ac.stateMach.Transition(EventReconnectDirect, "reconnected after backoff")
		return
	}
}

// --- helpers ---

func (a *Libp2pAdapter) getChannel(id string) (*activeChannel, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ac, ok := a.channels[id]
	if !ok {
		return nil, NewDeliveryError(ErrChannelClosed, "getChannel",
			fmt.Sprintf("channel %q not found or closed", id))
	}
	return ac, nil
}

// detectTransport extracts a transport name from a libp2p connection.
func detectTransport(conn network.Conn) string {
	addr := conn.RemoteMultiaddr().String()
	switch {
	case contains(addr, "/quic-v1"):
		return "quic-v1"
	case contains(addr, "/webtransport"):
		return "webtransport"
	case contains(addr, "/webrtc"):
		return "webrtc"
	case contains(addr, "/tcp"):
		return "tcp"
	default:
		return "unknown"
	}
}

func describeConnection(conn network.Conn) ConnectionPath {
	return ConnectionPath{
		LocalMultiaddr:  conn.LocalMultiaddr().String(),
		RemoteMultiaddr: conn.RemoteMultiaddr().String(),
		Limited:         conn.Stat().Limited,
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Verify interface compliance at compile time.
var _ DeliveryAdapter = (*Libp2pAdapter)(nil)
