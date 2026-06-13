package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocolConstants(t *testing.T) {
	t.Run("MinDeadlineDelta is 10 seconds", func(t *testing.T) {
		assert.Equal(t, uint64(10), MinDeadlineDelta,
			"FR-H06: minimum seconds between current time and nextHeartbeatDeadline")
	})

	t.Run("MaxDeadlineDelta is 86400 seconds (24 hours)", func(t *testing.T) {
		assert.Equal(t, uint64(86400), MaxDeadlineDelta,
			"FR-H07: maximum seconds between current time and nextHeartbeatDeadline")
	})

	t.Run("GracePeriod is 30 seconds", func(t *testing.T) {
		assert.Equal(t, uint64(30), GracePeriod,
			"FR-H08: seconds after deadline before SUSPECT transition")
	})

	t.Run("SuspectToDead is 120 seconds", func(t *testing.T) {
		assert.Equal(t, uint64(120), SuspectToDead,
			"FR-H09: seconds in SUSPECT before DEAD transition")
	})

	t.Run("ShutdownSentinel is 0", func(t *testing.T) {
		assert.Equal(t, uint64(0), ShutdownSentinel,
			"FR-H12: nextHeartbeatDeadline value signaling graceful shutdown")
	})

	t.Run("CurrentVersion is 1.0.0", func(t *testing.T) {
		assert.Equal(t, "1.0.0", CurrentVersion,
			"protocol version for HeartbeatPayload")
	})

	t.Run("PayloadTypeHeartbeat is heartbeat", func(t *testing.T) {
		assert.Equal(t, "heartbeat", PayloadTypeHeartbeat,
			"payload type discriminator for heartbeat messages")
	})

	t.Run("MandatoryFieldsBudget is 256 bytes", func(t *testing.T) {
		assert.Equal(t, 256, MandatoryFieldsBudget,
			"FR-H29: max JSON bytes for mandatory fields")
	})
}
