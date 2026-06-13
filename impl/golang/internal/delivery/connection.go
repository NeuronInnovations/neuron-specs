package delivery

// ConnectionState represents the lifecycle state of a delivery channel.
// FR-D07: 6 states.
type ConnectionState string

const (
	StateIdle         ConnectionState = "IDLE"
	StateConnecting   ConnectionState = "CONNECTING"
	StateConnected    ConnectionState = "CONNECTED"
	StateReconnecting ConnectionState = "RECONNECTING"
	StateRelaying     ConnectionState = "RELAYING"
	StateDisconnected ConnectionState = "DISCONNECTED"
)

// ConnectionEvent triggers a state transition. FR-D08.
type ConnectionEvent string

const (
	EventConnect          ConnectionEvent = "CONNECT"
	EventDialSuccess      ConnectionEvent = "DIAL_SUCCESS"
	EventDialFailRelay    ConnectionEvent = "DIAL_FAIL_RELAY_SUCCESS"
	EventDialFailAll      ConnectionEvent = "DIAL_FAIL_ALL"
	EventTransportDrop    ConnectionEvent = "TRANSPORT_DROP"
	EventDCUtRSuccess     ConnectionEvent = "DCUTR_SUCCESS"
	EventRelayDrop        ConnectionEvent = "RELAY_DROP"
	EventReconnectDirect  ConnectionEvent = "RECONNECT_DIRECT"
	EventReconnectRelay   ConnectionEvent = "RECONNECT_RELAY"
	EventBackoffExhausted ConnectionEvent = "BACKOFF_EXHAUSTED"
	EventDisconnect       ConnectionEvent = "DISCONNECT"
	EventFatalError       ConnectionEvent = "FATAL_ERROR"
)

// transition defines a valid state transition.
type connTransition struct {
	from  ConnectionState
	event ConnectionEvent
	to    ConnectionState
}

// transitions defines all valid state transitions per FR-D08.
var connTransitions = []connTransition{
	{StateIdle, EventConnect, StateConnecting},
	{StateConnecting, EventDialSuccess, StateConnected},
	{StateConnecting, EventDialFailRelay, StateRelaying},
	{StateConnecting, EventDialFailAll, StateDisconnected},
	{StateConnected, EventTransportDrop, StateReconnecting},
	{StateRelaying, EventDCUtRSuccess, StateConnected},
	{StateRelaying, EventRelayDrop, StateReconnecting},
	{StateReconnecting, EventReconnectDirect, StateConnected},
	{StateReconnecting, EventReconnectRelay, StateRelaying},
	{StateReconnecting, EventBackoffExhausted, StateDisconnected},
	{StateConnected, EventDisconnect, StateDisconnected},
	{StateConnected, EventFatalError, StateDisconnected},
	{StateRelaying, EventDisconnect, StateDisconnected},
	{StateRelaying, EventFatalError, StateDisconnected},
}

// connTransitionMap builds a lookup for fast transition resolution.
var connTransitionMap map[ConnectionState]map[ConnectionEvent]ConnectionState

func init() {
	connTransitionMap = make(map[ConnectionState]map[ConnectionEvent]ConnectionState)
	for _, t := range connTransitions {
		if connTransitionMap[t.from] == nil {
			connTransitionMap[t.from] = make(map[ConnectionEvent]ConnectionState)
		}
		connTransitionMap[t.from][t.event] = t.to
	}
}

// StateChangeCallback is called when the connection state changes. FR-D10.
type StateChangeCallback func(newState ConnectionState, event ConnectionEvent, reason string)

// ConnectionStateMachine tracks delivery channel connection state.
// FR-D07, FR-D08, FR-D10.
type ConnectionStateMachine struct {
	state    ConnectionState
	callback StateChangeCallback
}

// NewConnectionStateMachine creates a new state machine in IDLE state.
func NewConnectionStateMachine(callback StateChangeCallback) *ConnectionStateMachine {
	return &ConnectionStateMachine{
		state:    StateIdle,
		callback: callback,
	}
}

// State returns the current connection state.
func (m *ConnectionStateMachine) State() ConnectionState {
	return m.state
}

// Transition attempts a state transition given an event.
// Returns the new state or an error if the transition is invalid.
// FR-D08: transitions follow defined rules.
// FR-D10: fires callback on successful transition.
func (m *ConnectionStateMachine) Transition(event ConnectionEvent, reason string) (ConnectionState, error) {
	const op = "Transition"

	events, ok := connTransitionMap[m.state]
	if !ok {
		return m.state, NewDeliveryError(ErrDialFailed, op,
			"no transitions available from state "+string(m.state))
	}

	next, ok := events[event]
	if !ok {
		return m.state, NewDeliveryError(ErrDialFailed, op,
			"invalid transition: "+string(m.state)+" + "+string(event))
	}

	m.state = next

	if m.callback != nil {
		m.callback(next, event, reason)
	}

	return m.state, nil
}
