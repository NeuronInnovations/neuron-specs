package topic

// ConfirmationMode specifies how the publish operation reports completion.
type ConfirmationMode string

const (
	// FireAndForget returns immediately after submitting to the backend
	// without waiting for consensus or acknowledgement.
	FireAndForget ConfirmationMode = "FIRE_AND_FORGET"
	// WaitForConsensus blocks until the backend confirms the message has
	// reached consensus or has been durably acknowledged.
	WaitForConsensus ConfirmationMode = "WAIT_FOR_CONSENSUS"
)

// FR-T22: publish() returns PublishResult with confirmation data
// PublishResult captures the outcome of a publish operation.
// In FireAndForget mode, Confirmed is false and optional fields are nil.
// In WaitForConsensus mode, Confirmed is true and timestamps are populated.
type PublishResult struct {
	// TransactionRef is the backend-specific transaction identifier
	// (e.g., HCS transaction ID, Kafka offset).
	TransactionRef string `json:"transactionRef"`
	// ConsensusTimestamp is the backend-assigned consensus timestamp in Unix nanoseconds.
	// Nil for FireAndForget mode or backends that do not provide consensus timestamps.
	ConsensusTimestamp *uint64 `json:"consensusTimestamp,omitempty"`
	// SequenceNumber is the backend-assigned sequence number for the message.
	// Nil for FireAndForget mode or backends that do not provide sequence numbers.
	SequenceNumber *uint64 `json:"sequenceNumber,omitempty"`
	// Confirmed indicates whether the backend has confirmed the message.
	// True when WaitForConsensus mode receives confirmation; false for FireAndForget.
	Confirmed bool `json:"confirmed"`
}
