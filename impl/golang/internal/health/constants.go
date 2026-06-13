package health

// Protocol constants for the Neuron health/liveness system.
// These values are defined by the spec and MUST NOT be changed without a protocol version bump.

const (
	// MinDeadlineDelta is the minimum seconds between current time and nextHeartbeatDeadline.
	// Publisher validation V-PUB-05 rejects deltas below this floor.
	// Observer validation V-OBS-06 also enforces this minimum.
	// FR-H06.
	MinDeadlineDelta uint64 = 10

	// MaxDeadlineDelta is the maximum seconds between current time and nextHeartbeatDeadline.
	// Publisher validation V-PUB-06 rejects deltas above this ceiling (24 hours).
	// Observer validation V-OBS-06 also enforces this maximum.
	// FR-H07.
	MaxDeadlineDelta uint64 = 86400

	// GracePeriod is the number of seconds after the deadline before the observer
	// transitions a peer from ALIVE to SUSPECT. FR-H08.
	GracePeriod uint64 = 30

	// SuspectToDead is the number of seconds a peer remains in SUSPECT state
	// before the observer transitions it to DEAD. FR-H09.
	SuspectToDead uint64 = 120

	// ShutdownSentinel is the nextHeartbeatDeadline value that signals graceful shutdown.
	// When a publisher sets deadline to this value, the observer transitions the peer to OFFLINE.
	// FR-H12.
	ShutdownSentinel uint64 = 0

	// CurrentVersion is the protocol version string for HeartbeatPayload.
	// Version compatibility: major must equal 1 (any minor/patch accepted). FR-H28.
	CurrentVersion string = "1.0.0"

	// PayloadTypeHeartbeat is the payload type discriminator for heartbeat messages.
	// Publisher validation V-PUB-01 requires this exact value.
	PayloadTypeHeartbeat string = "heartbeat"

	// MandatoryFieldsBudget is the maximum JSON byte count for a HeartbeatPayload
	// with mandatory fields only. FR-H29.
	MandatoryFieldsBudget int = 256
)
