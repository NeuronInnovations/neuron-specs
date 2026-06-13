package delivery

import "time"

// BackoffConfig defines exponential backoff parameters for reconnection.
// FR-D09: initial 5s, factor 2, max delay 10min, max duration 1hr.
type BackoffConfig struct {
	InitialDelay time.Duration
	Factor       float64
	MaxDelay     time.Duration
	MaxDuration  time.Duration
}

// DefaultBackoffConfig returns the spec-defined default backoff parameters.
// FR-D09.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		InitialDelay: 5 * time.Second,
		Factor:       2.0,
		MaxDelay:     10 * time.Minute,
		MaxDuration:  1 * time.Hour,
	}
}

// NextDelay computes the delay for the given attempt number (0-indexed).
// delay = InitialDelay * Factor^attempt, capped at MaxDelay.
func (c BackoffConfig) NextDelay(attempt int) time.Duration {
	delay := float64(c.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= c.Factor
	}
	d := time.Duration(delay)
	if d > c.MaxDelay {
		return c.MaxDelay
	}
	return d
}

// IsExhausted returns true if the total elapsed time exceeds MaxDuration.
func (c BackoffConfig) IsExhausted(elapsed time.Duration) bool {
	return elapsed >= c.MaxDuration
}
