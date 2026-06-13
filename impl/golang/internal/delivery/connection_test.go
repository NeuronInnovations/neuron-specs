package delivery

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T003: ConnectionState Machine Tests ---

func TestConnectionStateMachine_InitialState(t *testing.T) {
	sm := NewConnectionStateMachine(nil)
	assert.Equal(t, StateIdle, sm.State())
}

func TestConnectionStateMachine_AllValidTransitions(t *testing.T) {
	// FR-D08: All 12+ valid transitions.
	tests := []struct {
		name  string
		from  ConnectionState
		event ConnectionEvent
		to    ConnectionState
	}{
		{"IDLE→CONNECTING", StateIdle, EventConnect, StateConnecting},
		{"CONNECTING→CONNECTED", StateConnecting, EventDialSuccess, StateConnected},
		{"CONNECTING→RELAYING", StateConnecting, EventDialFailRelay, StateRelaying},
		{"CONNECTING→DISCONNECTED", StateConnecting, EventDialFailAll, StateDisconnected},
		{"CONNECTED→RECONNECTING", StateConnected, EventTransportDrop, StateReconnecting},
		{"RELAYING→CONNECTED", StateRelaying, EventDCUtRSuccess, StateConnected},
		{"RELAYING→RECONNECTING", StateRelaying, EventRelayDrop, StateReconnecting},
		{"RECONNECTING→CONNECTED", StateReconnecting, EventReconnectDirect, StateConnected},
		{"RECONNECTING→RELAYING", StateReconnecting, EventReconnectRelay, StateRelaying},
		{"RECONNECTING→DISCONNECTED", StateReconnecting, EventBackoffExhausted, StateDisconnected},
		{"CONNECTED→DISCONNECTED (disconnect)", StateConnected, EventDisconnect, StateDisconnected},
		{"CONNECTED→DISCONNECTED (fatal)", StateConnected, EventFatalError, StateDisconnected},
		{"RELAYING→DISCONNECTED (disconnect)", StateRelaying, EventDisconnect, StateDisconnected},
		{"RELAYING→DISCONNECTED (fatal)", StateRelaying, EventFatalError, StateDisconnected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewConnectionStateMachine(nil)
			sm.state = tt.from

			newState, err := sm.Transition(tt.event, "test")
			require.NoError(t, err)
			assert.Equal(t, tt.to, newState)
		})
	}
}

func TestConnectionStateMachine_RejectInvalidTransitions(t *testing.T) {
	tests := []struct {
		name  string
		from  ConnectionState
		event ConnectionEvent
	}{
		{"IDLE + DIAL_SUCCESS", StateIdle, EventDialSuccess},
		{"CONNECTED + CONNECT", StateConnected, EventConnect},
		{"DISCONNECTED + anything", StateDisconnected, EventConnect},
		{"RELAYING + CONNECT", StateRelaying, EventConnect},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewConnectionStateMachine(nil)
			sm.state = tt.from

			_, err := sm.Transition(tt.event, "test")
			require.Error(t, err)
			var de *DeliveryError
			require.True(t, errors.As(err, &de))
		})
	}
}

func TestConnectionStateMachine_CallbackFires(t *testing.T) {
	// FR-D10: State change callback fires with new state, event, and reason.
	var captured struct {
		state  ConnectionState
		event  ConnectionEvent
		reason string
		count  int
	}

	callback := func(state ConnectionState, event ConnectionEvent, reason string) {
		captured.state = state
		captured.event = event
		captured.reason = reason
		captured.count++
	}

	sm := NewConnectionStateMachine(callback)

	_, err := sm.Transition(EventConnect, "user initiated")
	require.NoError(t, err)

	assert.Equal(t, 1, captured.count, "callback must fire once per transition")
	assert.Equal(t, StateConnecting, captured.state)
	assert.Equal(t, EventConnect, captured.event)
	assert.Equal(t, "user initiated", captured.reason)
}

func TestConnectionStateMachine_CallbackNotFiredOnError(t *testing.T) {
	callCount := 0
	sm := NewConnectionStateMachine(func(_ ConnectionState, _ ConnectionEvent, _ string) {
		callCount++
	})

	// Invalid transition should NOT fire callback.
	_, err := sm.Transition(EventDialSuccess, "test")
	require.Error(t, err)
	assert.Equal(t, 0, callCount)
}

func TestConnectionStateMachine_FullLifecycle(t *testing.T) {
	// SC-D03: Deterministic lifecycle.
	sm := NewConnectionStateMachine(nil)

	steps := []struct {
		event ConnectionEvent
		state ConnectionState
	}{
		{EventConnect, StateConnecting},
		{EventDialSuccess, StateConnected},
		{EventTransportDrop, StateReconnecting},
		{EventReconnectDirect, StateConnected},
		{EventDisconnect, StateDisconnected},
	}

	for _, step := range steps {
		s, err := sm.Transition(step.event, "test")
		require.NoError(t, err)
		assert.Equal(t, step.state, s)
	}
}

// --- T028: Reconnection State Machine Transitions ---

func TestConnectionStateMachine_ReconnectionTransitions(t *testing.T) {
	// FR-D08: Explicit reconnection transition tests.

	t.Run("CONNECTED→RECONNECTING on transport drop", func(t *testing.T) {
		sm := NewConnectionStateMachine(nil)
		sm.state = StateConnected
		s, err := sm.Transition(EventTransportDrop, "network failure")
		require.NoError(t, err)
		assert.Equal(t, StateReconnecting, s)
	})

	t.Run("RECONNECTING→CONNECTED on direct reconnect", func(t *testing.T) {
		sm := NewConnectionStateMachine(nil)
		sm.state = StateReconnecting
		s, err := sm.Transition(EventReconnectDirect, "reconnected directly")
		require.NoError(t, err)
		assert.Equal(t, StateConnected, s)
	})

	t.Run("RECONNECTING→RELAYING on relay reconnect", func(t *testing.T) {
		sm := NewConnectionStateMachine(nil)
		sm.state = StateReconnecting
		s, err := sm.Transition(EventReconnectRelay, "reconnected via relay")
		require.NoError(t, err)
		assert.Equal(t, StateRelaying, s)
	})

	t.Run("RECONNECTING→DISCONNECTED on backoff exhausted", func(t *testing.T) {
		sm := NewConnectionStateMachine(nil)
		sm.state = StateReconnecting
		s, err := sm.Transition(EventBackoffExhausted, "max duration exceeded")
		require.NoError(t, err)
		assert.Equal(t, StateDisconnected, s)
	})

	t.Run("RELAYING→RECONNECTING on relay drop", func(t *testing.T) {
		sm := NewConnectionStateMachine(nil)
		sm.state = StateRelaying
		s, err := sm.Transition(EventRelayDrop, "relay unavailable")
		require.NoError(t, err)
		assert.Equal(t, StateReconnecting, s)
	})

	t.Run("full reconnection lifecycle with callback", func(t *testing.T) {
		var states []ConnectionState
		sm := NewConnectionStateMachine(func(s ConnectionState, _ ConnectionEvent, _ string) {
			states = append(states, s)
		})
		sm.Transition(EventConnect, "")
		sm.Transition(EventDialSuccess, "")
		sm.Transition(EventTransportDrop, "")
		sm.Transition(EventReconnectDirect, "")
		sm.Transition(EventTransportDrop, "")
		sm.Transition(EventBackoffExhausted, "")

		expected := []ConnectionState{
			StateConnecting, StateConnected, StateReconnecting,
			StateConnected, StateReconnecting, StateDisconnected,
		}
		assert.Equal(t, expected, states)
	})
}
