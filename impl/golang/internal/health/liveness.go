package health

// LivenessState represents the five-state liveness classification of a peer.
// FR-H18, FR-H19, FR-H20.
type LivenessState string

const (
	// StateUnknown indicates no heartbeat has been received from this peer.
	StateUnknown LivenessState = "UNKNOWN"
	// StateAlive indicates the peer is within its declared heartbeat window.
	StateAlive LivenessState = "ALIVE"
	// StateSuspect indicates the peer has missed its deadline but is within the grace+suspect window.
	StateSuspect LivenessState = "SUSPECT"
	// StateDead indicates the peer has exceeded the grace+suspect window without a new heartbeat.
	StateDead LivenessState = "DEAD"
	// StateOffline indicates the peer has sent a graceful shutdown sentinel (deadline=0).
	StateOffline LivenessState = "OFFLINE"
)

// String returns the string representation of a LivenessState.
func (s LivenessState) String() string {
	return string(s)
}

// LivenessRecord holds the observer-local liveness state for a single peer.
// This is an in-memory record, not persisted on-chain.
// FR-H18.
type LivenessRecord struct {
	// senderAddress is the EVM address (hex) of the peer whose liveness is tracked.
	senderAddress string
	// currentState is the current liveness classification of this peer.
	currentState LivenessState
	// lastDeadline is the nextHeartbeatDeadline from the most recent valid heartbeat.
	lastDeadline uint64
	// lastSequence is the sequence number of the most recent valid heartbeat.
	lastSequence uint64
	// lastConsensusTimestamp is the consensus timestamp of the most recent valid heartbeat.
	lastConsensusTimestamp uint64
}

// SenderAddress returns the EVM address of the peer.
func (r *LivenessRecord) SenderAddress() string { return r.senderAddress }

// CurrentState returns the current liveness state.
func (r *LivenessRecord) CurrentState() LivenessState { return r.currentState }

// LastDeadline returns the last heartbeat deadline.
func (r *LivenessRecord) LastDeadline() uint64 { return r.lastDeadline }

// LastSequence returns the last sequence number.
func (r *LivenessRecord) LastSequence() uint64 { return r.lastSequence }

// LastConsensusTimestamp returns the last consensus timestamp.
func (r *LivenessRecord) LastConsensusTimestamp() uint64 { return r.lastConsensusTimestamp }

// EvaluateLiveness is a pure function that determines liveness state from a record and current time.
// It does not mutate the record. All time arithmetic uses abstract uint64 timestamps
// (consensus-sourced, not local clock).
//
// Decision tree:
//   - nil record          -> UNKNOWN  (no data)
//   - deadline == 0       -> OFFLINE  (graceful shutdown sentinel)
//   - now <= deadline+GP  -> ALIVE    (within grace window)
//   - now <= deadline+GP+S2D -> SUSPECT (grace expired, suspect window)
//   - otherwise           -> DEAD     (all windows expired)
//
// Overflow safety: The additions (deadline + GracePeriod + SuspectToDead) cannot overflow
// in practice because LastDeadline is bounded by ValidateInboundHeartbeat which enforces
// deadline <= consensusTimestamp + MaxDeadlineDelta (86400). A consensusTimestamp near
// math.MaxUint64 would require ~585 billion years of Unix time, which is physically
// unreachable. Should this assumption ever change, add explicit overflow guards here.
//
// FR-H20, SC-H04.
func EvaluateLiveness(record *LivenessRecord, currentTime uint64) LivenessState {
	if record == nil {
		return StateUnknown
	}
	if record.lastDeadline == ShutdownSentinel {
		return StateOffline
	}
	if currentTime <= record.lastDeadline+GracePeriod {
		return StateAlive
	}
	if currentTime <= record.lastDeadline+GracePeriod+SuspectToDead {
		return StateSuspect
	}
	return StateDead
}
