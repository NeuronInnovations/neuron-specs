package edgeapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConsensusToSeconds covers the buyer-side wiring's consensus-timestamp
// normalizer. Both HCSAdapter and MemoryBus stamp deliveries with Unix
// nanoseconds, while spec-005 deadline arithmetic is in seconds. The
// helper passes through values that already look like seconds, and scales
// values that look like nanos.
func TestConsensusToSeconds(t *testing.T) {
	cases := []struct {
		name string
		in   uint64
		want uint64
	}{
		{"already-seconds passes through", 1_700_000_000, 1_700_000_000},
		{"zero passes through", 0, 0},
		{"HCS nanos scaled", 1_700_000_000_000_000_000, 1_700_000_000},
		{"MemoryBus nanos scaled", 1_750_000_000_123_456_789, 1_750_000_000},
		{"just below threshold passes through",
			1_000_000_000_000_000 - 1, 1_000_000_000_000_000 - 1},
		{"at threshold scales", 1_000_000_000_000_000, 1_000_000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, consensusToSeconds(tc.in))
		})
	}
}

// Note: the buyer-side wiring (startSellerLivenessWatch) integrates
// MemoryBus.Subscribe + consensusToSeconds + LivenessTracker. Each piece
// has direct unit coverage:
//   - LivenessTracker behavior: liveness_test.go (7 tests, fakeClock-driven)
//   - consensusToSeconds: above table-driven test
//   - MemoryBus.Subscribe: internal/topic tests
//
// An end-to-end timing test of the wiring would need to wait through
// grace+suspectToDead seconds (~150s minimum), which exceeds the CI
// window. Manual soak tests cover the integrated behavior.
