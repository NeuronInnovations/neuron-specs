package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLivenessState(t *testing.T) {
	t.Run("five states exist with correct values", func(t *testing.T) {
		assert.Equal(t, LivenessState("UNKNOWN"), StateUnknown)
		assert.Equal(t, LivenessState("ALIVE"), StateAlive)
		assert.Equal(t, LivenessState("SUSPECT"), StateSuspect)
		assert.Equal(t, LivenessState("DEAD"), StateDead)
		assert.Equal(t, LivenessState("OFFLINE"), StateOffline)
	})

	t.Run("String returns correct representation", func(t *testing.T) {
		assert.Equal(t, "UNKNOWN", StateUnknown.String())
		assert.Equal(t, "ALIVE", StateAlive.String())
		assert.Equal(t, "SUSPECT", StateSuspect.String())
		assert.Equal(t, "DEAD", StateDead.String())
		assert.Equal(t, "OFFLINE", StateOffline.String())
	})

	t.Run("all five states are distinct", func(t *testing.T) {
		states := []LivenessState{StateUnknown, StateAlive, StateSuspect, StateDead, StateOffline}
		seen := make(map[LivenessState]bool)
		for _, s := range states {
			assert.False(t, seen[s], "duplicate state: %s", s)
			seen[s] = true
		}
		assert.Len(t, seen, 5)
	})
}

func TestLivenessRecord(t *testing.T) {
	t.Run("LivenessRecord fields have correct types", func(t *testing.T) {
		r := LivenessRecord{
			senderAddress:         "0x1234567890abcdef1234567890abcdef12345678",
			currentState:          StateAlive,
			lastDeadline:          uint64(1700000060),
			lastSequence:          uint64(42),
			lastConsensusTimestamp: uint64(1700000000),
		}
		assert.Equal(t, "0x1234567890abcdef1234567890abcdef12345678", r.senderAddress)
		assert.Equal(t, StateAlive, r.currentState)
		assert.Equal(t, uint64(1700000060), r.lastDeadline)
		assert.Equal(t, uint64(42), r.lastSequence)
		assert.Equal(t, uint64(1700000000), r.lastConsensusTimestamp)
	})
}

func TestEvaluateLiveness(t *testing.T) {
	t.Run("nil record returns UNKNOWN", func(t *testing.T) {
		result := EvaluateLiveness(nil, 1700000000)
		assert.Equal(t, StateUnknown, result)
	})

	t.Run("deadline=0 (shutdown sentinel) returns OFFLINE", func(t *testing.T) {
		record := &LivenessRecord{
			senderAddress: "0xabc",
			currentState:  StateAlive,
			lastDeadline:  ShutdownSentinel,
		}
		result := EvaluateLiveness(record, 1700000000)
		assert.Equal(t, StateOffline, result)
	})

	t.Run("currentTime <= deadline+GracePeriod returns ALIVE", func(t *testing.T) {
		deadline := uint64(1700000060)
		record := &LivenessRecord{
			senderAddress: "0xabc",
			lastDeadline:  deadline,
		}

		// Well before deadline
		assert.Equal(t, StateAlive, EvaluateLiveness(record, deadline-10))
		// Exactly at deadline
		assert.Equal(t, StateAlive, EvaluateLiveness(record, deadline))
		// Within grace period
		assert.Equal(t, StateAlive, EvaluateLiveness(record, deadline+GracePeriod-1))
		// Exactly at deadline+GracePeriod boundary
		assert.Equal(t, StateAlive, EvaluateLiveness(record, deadline+GracePeriod))
	})

	t.Run("currentTime <= deadline+GracePeriod+SuspectToDead returns SUSPECT", func(t *testing.T) {
		deadline := uint64(1700000060)
		record := &LivenessRecord{
			senderAddress: "0xabc",
			lastDeadline:  deadline,
		}

		// Just past grace period
		assert.Equal(t, StateSuspect, EvaluateLiveness(record, deadline+GracePeriod+1))
		// Middle of suspect window
		assert.Equal(t, StateSuspect, EvaluateLiveness(record, deadline+GracePeriod+60))
		// At the end of suspect window
		assert.Equal(t, StateSuspect, EvaluateLiveness(record, deadline+GracePeriod+SuspectToDead))
	})

	t.Run("currentTime > deadline+GracePeriod+SuspectToDead returns DEAD", func(t *testing.T) {
		deadline := uint64(1700000060)
		record := &LivenessRecord{
			senderAddress: "0xabc",
			lastDeadline:  deadline,
		}

		// Just past suspect window
		assert.Equal(t, StateDead, EvaluateLiveness(record, deadline+GracePeriod+SuspectToDead+1))
		// Well past suspect window
		assert.Equal(t, StateDead, EvaluateLiveness(record, deadline+GracePeriod+SuspectToDead+3600))
	})

	t.Run("boundary: exact ALIVE->SUSPECT transition", func(t *testing.T) {
		deadline := uint64(1700000060)
		record := &LivenessRecord{
			senderAddress: "0xabc",
			lastDeadline:  deadline,
		}

		// Exactly at boundary: deadline + GracePeriod = ALIVE
		assert.Equal(t, StateAlive, EvaluateLiveness(record, deadline+GracePeriod),
			"at deadline+GracePeriod must be ALIVE")
		// One second past: deadline + GracePeriod + 1 = SUSPECT
		assert.Equal(t, StateSuspect, EvaluateLiveness(record, deadline+GracePeriod+1),
			"at deadline+GracePeriod+1 must be SUSPECT")
	})

	t.Run("boundary: exact SUSPECT->DEAD transition", func(t *testing.T) {
		deadline := uint64(1700000060)
		record := &LivenessRecord{
			senderAddress: "0xabc",
			lastDeadline:  deadline,
		}

		// Exactly at boundary: deadline + GracePeriod + SuspectToDead = SUSPECT
		assert.Equal(t, StateSuspect, EvaluateLiveness(record, deadline+GracePeriod+SuspectToDead),
			"at deadline+GracePeriod+SuspectToDead must be SUSPECT")
		// One second past: deadline + GracePeriod + SuspectToDead + 1 = DEAD
		assert.Equal(t, StateDead, EvaluateLiveness(record, deadline+GracePeriod+SuspectToDead+1),
			"at deadline+GracePeriod+SuspectToDead+1 must be DEAD")
	})

	t.Run("EvaluateLiveness is a pure function (same input -> same output)", func(t *testing.T) {
		record := &LivenessRecord{
			senderAddress: "0xabc",
			lastDeadline:  1700000060,
		}
		// Call multiple times with the same inputs
		for i := 0; i < 10; i++ {
			assert.Equal(t, StateAlive, EvaluateLiveness(record, 1700000060))
		}
	})

	t.Run("table-driven: comprehensive state transitions", func(t *testing.T) {
		deadline := uint64(1000)

		tests := []struct {
			name        string
			record      *LivenessRecord
			currentTime uint64
			want        LivenessState
		}{
			{
				name:        "nil record -> UNKNOWN",
				record:      nil,
				currentTime: 0,
				want:        StateUnknown,
			},
			{
				name:        "shutdown sentinel -> OFFLINE",
				record:      &LivenessRecord{lastDeadline: 0},
				currentTime: 5000,
				want:        StateOffline,
			},
			{
				name:        "before deadline -> ALIVE",
				record:      &LivenessRecord{lastDeadline: deadline},
				currentTime: deadline - 1,
				want:        StateAlive,
			},
			{
				name:        "at deadline -> ALIVE",
				record:      &LivenessRecord{lastDeadline: deadline},
				currentTime: deadline,
				want:        StateAlive,
			},
			{
				name:        "at deadline+GP -> ALIVE",
				record:      &LivenessRecord{lastDeadline: deadline},
				currentTime: deadline + GracePeriod,
				want:        StateAlive,
			},
			{
				name:        "at deadline+GP+1 -> SUSPECT",
				record:      &LivenessRecord{lastDeadline: deadline},
				currentTime: deadline + GracePeriod + 1,
				want:        StateSuspect,
			},
			{
				name:        "at deadline+GP+S2D -> SUSPECT",
				record:      &LivenessRecord{lastDeadline: deadline},
				currentTime: deadline + GracePeriod + SuspectToDead,
				want:        StateSuspect,
			},
			{
				name:        "at deadline+GP+S2D+1 -> DEAD",
				record:      &LivenessRecord{lastDeadline: deadline},
				currentTime: deadline + GracePeriod + SuspectToDead + 1,
				want:        StateDead,
			},
			{
				name:        "far future -> DEAD",
				record:      &LivenessRecord{lastDeadline: deadline},
				currentTime: deadline + 1000000,
				want:        StateDead,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := EvaluateLiveness(tt.record, tt.currentTime)
				require.Equal(t, tt.want, got)
			})
		}
	})
}

// TestTransitionState covers the 8 state transitions required by the spec,
// sequence ordering enforcement (FR-H17), and the recovery invariant (FR-H21).
func TestTransitionState(t *testing.T) {
	addr := "0x1234567890abcdef1234567890abcdef12345678"

	t.Run("UNKNOWN -> ALIVE: first heartbeat from unknown peer", func(t *testing.T) {
		payload := BuildHeartbeatPayload(1700000060, RoleBuyer)
		record := TransitionState(nil, addr, &payload, 1700000000, 1)
		require.NotNil(t, record)
		assert.Equal(t, addr, record.senderAddress)
		assert.Equal(t, StateAlive, record.currentState)
		assert.Equal(t, uint64(1700000060), record.lastDeadline)
		assert.Equal(t, uint64(1), record.lastSequence)
	})

	t.Run("ALIVE -> ALIVE: subsequent heartbeat keeps ALIVE", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress: addr,
			currentState:  StateAlive,
			lastDeadline:  1700000060,
			lastSequence:  1,
		}
		payload := BuildHeartbeatPayload(1700000120, RoleBuyer)
		record := TransitionState(existing, addr, &payload, 1700000060, 2)
		assert.Equal(t, StateAlive, record.currentState)
		assert.Equal(t, uint64(1700000120), record.lastDeadline)
	})

	t.Run("ALIVE -> SUSPECT: evaluated via EvaluateLiveness, not TransitionState", func(t *testing.T) {
		// TransitionState does not produce SUSPECT directly.
		// SUSPECT is only produced by EvaluateLiveness when time passes.
		record := &LivenessRecord{
			senderAddress: addr,
			currentState:  StateAlive,
			lastDeadline:  1700000060,
			lastSequence:  1,
		}
		// After deadline + grace period.
		state := EvaluateLiveness(record, 1700000060+GracePeriod+1)
		assert.Equal(t, StateSuspect, state)
	})

	t.Run("ALIVE -> OFFLINE: shutdown heartbeat", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress: addr,
			currentState:  StateAlive,
			lastDeadline:  1700000060,
			lastSequence:  1,
		}
		payload := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: ShutdownSentinel,
			Role:                  RoleBuyer,
		}
		record := TransitionState(existing, addr, &payload, 1700000060, 2)
		assert.Equal(t, StateOffline, record.currentState)
		assert.Equal(t, ShutdownSentinel, record.lastDeadline)
	})

	t.Run("SUSPECT -> ALIVE: recovery from SUSPECT via valid heartbeat (FR-H21)", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress: addr,
			currentState:  StateSuspect,
			lastDeadline:  1700000060,
			lastSequence:  5,
		}
		payload := BuildHeartbeatPayload(1700000200, RoleBuyer)
		record := TransitionState(existing, addr, &payload, 1700000100, 6)
		assert.Equal(t, StateAlive, record.currentState,
			"FR-H21: any valid heartbeat from SUSPECT must transition to ALIVE")
	})

	t.Run("SUSPECT -> DEAD: evaluated via EvaluateLiveness, not TransitionState", func(t *testing.T) {
		record := &LivenessRecord{
			senderAddress: addr,
			currentState:  StateSuspect,
			lastDeadline:  1700000060,
			lastSequence:  1,
		}
		// After deadline + grace + suspect window.
		state := EvaluateLiveness(record, 1700000060+GracePeriod+SuspectToDead+1)
		assert.Equal(t, StateDead, state)
	})

	t.Run("DEAD -> ALIVE: recovery from DEAD via valid heartbeat (FR-H21)", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress: addr,
			currentState:  StateDead,
			lastDeadline:  1700000060,
			lastSequence:  10,
		}
		payload := BuildHeartbeatPayload(1700001000, RoleBuyer)
		record := TransitionState(existing, addr, &payload, 1700000900, 11)
		assert.Equal(t, StateAlive, record.currentState,
			"FR-H21: any valid heartbeat from DEAD must transition to ALIVE")
	})

	t.Run("OFFLINE -> ALIVE: recovery from OFFLINE via valid heartbeat (FR-H21)", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress: addr,
			currentState:  StateOffline,
			lastDeadline:  ShutdownSentinel,
			lastSequence:  20,
		}
		payload := BuildHeartbeatPayload(1700001060, RoleBuyer)
		record := TransitionState(existing, addr, &payload, 1700001000, 21)
		assert.Equal(t, StateAlive, record.currentState,
			"FR-H21: any valid heartbeat from OFFLINE must transition to ALIVE")
	})

	t.Run("sequence ordering: lower sequence is rejected", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress:         addr,
			currentState:          StateAlive,
			lastDeadline:          1700000060,
			lastSequence:          10,
			lastConsensusTimestamp: 1700000000,
		}
		payload := BuildHeartbeatPayload(1700000120, RoleBuyer)
		record := TransitionState(existing, addr, &payload, 1700000060, 5)
		// Must not be updated.
		assert.Equal(t, uint64(10), record.lastSequence)
		assert.Equal(t, uint64(1700000060), record.lastDeadline)
	})

	t.Run("sequence ordering: equal sequence is rejected", func(t *testing.T) {
		existing := &LivenessRecord{
			senderAddress:         addr,
			currentState:          StateAlive,
			lastDeadline:          1700000060,
			lastSequence:          10,
			lastConsensusTimestamp: 1700000000,
		}
		payload := BuildHeartbeatPayload(1700000120, RoleBuyer)
		record := TransitionState(existing, addr, &payload, 1700000060, 10)
		// Must not be updated.
		assert.Equal(t, uint64(1700000060), record.lastDeadline)
	})

	t.Run("recovery invariant: valid heartbeat from every state produces ALIVE", func(t *testing.T) {
		states := []LivenessState{StateUnknown, StateAlive, StateSuspect, StateDead, StateOffline}
		for _, fromState := range states {
			t.Run("from_"+fromState.String(), func(t *testing.T) {
				var existing *LivenessRecord
				if fromState != StateUnknown {
					existing = &LivenessRecord{
						senderAddress: addr,
						currentState:  fromState,
						lastDeadline:  1700000060,
						lastSequence:  5,
					}
				}
				payload := BuildHeartbeatPayload(1700000200, RoleBuyer)
				seq := uint64(6)
				if existing == nil {
					seq = 1
				}
				record := TransitionState(existing, addr, &payload, 1700000100, seq)
				assert.Equal(t, StateAlive, record.currentState,
					"FR-H21: valid heartbeat from %s must produce ALIVE", fromState)
			})
		}
	})

	t.Run("first heartbeat with sequence 0 is accepted for new record", func(t *testing.T) {
		payload := BuildHeartbeatPayload(1700000060, RoleBuyer)
		record := TransitionState(nil, addr, &payload, 1700000000, 0)
		require.NotNil(t, record)
		assert.Equal(t, StateAlive, record.currentState)
		assert.Equal(t, uint64(0), record.lastSequence)
	})
}

// --- Phase 5 (T048): OFFLINE -> ALIVE Recovery Test ---

func TestOfflineToAliveRecovery(t *testing.T) {
	addr := "0x1234567890abcdef1234567890abcdef12345678"

	t.Run("T048: OFFLINE -> ALIVE recovery via valid heartbeat with non-zero deadline", func(t *testing.T) {
		// Start with a record in OFFLINE state (peer sent shutdown sentinel).
		offlineRecord := &LivenessRecord{
			senderAddress:         addr,
			currentState:          StateOffline,
			lastDeadline:          ShutdownSentinel,
			lastSequence:          10,
			lastConsensusTimestamp: 1700000000,
		}

		// EvaluateLiveness confirms OFFLINE before recovery.
		assert.Equal(t, StateOffline, EvaluateLiveness(offlineRecord, 1700000100),
			"precondition: record with shutdown sentinel must evaluate to OFFLINE")

		// Send a new valid heartbeat with non-zero deadline (recovery).
		recoveryPayload := BuildHeartbeatPayload(1700001060, RoleBuyer)
		recoveredRecord := TransitionState(offlineRecord, addr, &recoveryPayload, 1700001000, 11)

		// FR-H21: Must transition from OFFLINE to ALIVE.
		assert.Equal(t, StateAlive, recoveredRecord.currentState,
			"FR-H21: OFFLINE -> ALIVE on receipt of valid heartbeat with non-zero deadline")
		assert.Equal(t, uint64(1700001060), recoveredRecord.lastDeadline,
			"LastDeadline must be updated to the new deadline")
		assert.Equal(t, uint64(11), recoveredRecord.lastSequence,
			"LastSequence must be updated")
		assert.Equal(t, uint64(1700001000), recoveredRecord.lastConsensusTimestamp,
			"LastConsensusTimestamp must be updated")

		// EvaluateLiveness confirms ALIVE after recovery.
		assert.Equal(t, StateAlive, EvaluateLiveness(recoveredRecord, 1700001000),
			"postcondition: recovered record must evaluate to ALIVE")
	})

	t.Run("T048: OFFLINE recovery preserves correct state machine behavior", func(t *testing.T) {
		// After recovery from OFFLINE, the normal ALIVE->SUSPECT->DEAD cascade
		// should work as expected based on the new deadline.
		offlineRecord := &LivenessRecord{
			senderAddress:         addr,
			currentState:          StateOffline,
			lastDeadline:          ShutdownSentinel,
			lastSequence:          5,
			lastConsensusTimestamp: 1700000000,
		}

		recoveryPayload := BuildHeartbeatPayload(1700001060, RoleBuyer)
		record := TransitionState(offlineRecord, addr, &recoveryPayload, 1700001000, 6)
		assert.Equal(t, StateAlive, record.currentState)

		// Verify the full state cascade from the new deadline.
		newDeadline := uint64(1700001060)
		assert.Equal(t, StateAlive, EvaluateLiveness(record, newDeadline),
			"at deadline: ALIVE")
		assert.Equal(t, StateAlive, EvaluateLiveness(record, newDeadline+GracePeriod),
			"at deadline+GP: ALIVE")
		assert.Equal(t, StateSuspect, EvaluateLiveness(record, newDeadline+GracePeriod+1),
			"at deadline+GP+1: SUSPECT")
		assert.Equal(t, StateDead, EvaluateLiveness(record, newDeadline+GracePeriod+SuspectToDead+1),
			"at deadline+GP+S2D+1: DEAD")
	})
}

// --- Phase 8 (T069): Cross-Chain Determinism Test ---

func TestCrossChainDeterminism(t *testing.T) {
	// T069: Given identical inputs, EvaluateLiveness must produce the same result
	// regardless of which backend the consensus timestamp came from. EvaluateLiveness
	// is a pure function operating on abstract uint64 timestamps.
	t.Run("T069: EvaluateLiveness is deterministic across backends", func(t *testing.T) {
		deadline := uint64(1700000060)

		// Simulate records with identical data but different backend origins.
		hcsRecord := &LivenessRecord{
			senderAddress:         "0xabc",
			currentState:          StateAlive,
			lastDeadline:          deadline,
			lastSequence:          1,
			lastConsensusTimestamp: 1700000000, // from HCS consensus
		}

		evmRecord := &LivenessRecord{
			senderAddress:         "0xabc",
			currentState:          StateAlive,
			lastDeadline:          deadline,
			lastSequence:          1,
			lastConsensusTimestamp: 1700000000, // from EVM block.timestamp
		}

		kafkaRecord := &LivenessRecord{
			senderAddress:         "0xabc",
			currentState:          StateAlive,
			lastDeadline:          deadline,
			lastSequence:          1,
			lastConsensusTimestamp: 1700000000, // from Kafka broker timestamp
		}

		// Evaluate at multiple time points — all three must produce identical results.
		testTimes := []struct {
			name string
			time uint64
			want LivenessState
		}{
			{"before deadline", deadline - 10, StateAlive},
			{"at deadline", deadline, StateAlive},
			{"within grace", deadline + GracePeriod, StateAlive},
			{"past grace", deadline + GracePeriod + 1, StateSuspect},
			{"within suspect", deadline + GracePeriod + SuspectToDead, StateSuspect},
			{"past suspect", deadline + GracePeriod + SuspectToDead + 1, StateDead},
		}

		for _, tt := range testTimes {
			t.Run(tt.name, func(t *testing.T) {
				hcsResult := EvaluateLiveness(hcsRecord, tt.time)
				evmResult := EvaluateLiveness(evmRecord, tt.time)
				kafkaResult := EvaluateLiveness(kafkaRecord, tt.time)

				assert.Equal(t, tt.want, hcsResult,
					"HCS: expected %s at time %d", tt.want, tt.time)
				assert.Equal(t, hcsResult, evmResult,
					"EVM must match HCS at time %d", tt.time)
				assert.Equal(t, hcsResult, kafkaResult,
					"Kafka must match HCS at time %d", tt.time)
			})
		}
	})

	t.Run("T069: TransitionState is deterministic across backends", func(t *testing.T) {
		addr := "0x1234567890abcdef1234567890abcdef12345678"
		deadline := uint64(1700000120)
		consensusTS := uint64(1700000060)

		payload := BuildHeartbeatPayload(deadline, RoleSeller)

		// Apply identical transition from nil record with different "backend" contexts.
		hcsRecord := TransitionState(nil, addr, &payload, consensusTS, 1)
		evmRecord := TransitionState(nil, addr, &payload, consensusTS, 1)

		assert.Equal(t, hcsRecord.currentState, evmRecord.currentState,
			"state must be identical regardless of backend origin")
		assert.Equal(t, hcsRecord.lastDeadline, evmRecord.lastDeadline)
		assert.Equal(t, hcsRecord.lastSequence, evmRecord.lastSequence)
		assert.Equal(t, hcsRecord.lastConsensusTimestamp, evmRecord.lastConsensusTimestamp)
	})
}
