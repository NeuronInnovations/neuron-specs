package topic

// FR-T24: subscribe() returns MessageDelivery
// MessageDelivery wraps a TopicMessage with backend-provided delivery metadata.
// It is the type received by subscribers when a message arrives from the transport.
//
// Fields follow FR-T24 (delivery envelope) and FR-T29 (backend sequence mapping):
//   - Message: The original TopicMessage as published
//   - ConsensusTimestamp: Backend-assigned consensus timestamp in Unix nanoseconds
//   - BackendSequence: Backend-assigned monotonic sequence number for ordering
type MessageDelivery struct {
	// Message is the delivered TopicMessage envelope.
	Message TopicMessage `json:"message"`
	// FR-T29: ConsensusTimestamp from backend authoritative clock
	// ConsensusTimestamp is the backend-assigned consensus timestamp in Unix nanoseconds.
	// For HCS this is the Hedera consensus timestamp. For Kafka this is the broker timestamp.
	ConsensusTimestamp uint64 `json:"consensusTimestamp"`
	// BackendSequence is the backend-assigned sequence number for total ordering.
	// For HCS this is the topic sequence number. For Kafka this is the partition offset.
	BackendSequence uint64 `json:"backendSequence"`
}
