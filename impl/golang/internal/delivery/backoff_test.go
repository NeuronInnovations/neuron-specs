package delivery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- T004: BackoffConfig Tests ---

func TestDefaultBackoffConfig(t *testing.T) {
	// FR-D09: 5s initial, 2x factor, 10min cap, 1hr max.
	cfg := DefaultBackoffConfig()
	assert.Equal(t, 5*time.Second, cfg.InitialDelay)
	assert.Equal(t, 2.0, cfg.Factor)
	assert.Equal(t, 10*time.Minute, cfg.MaxDelay)
	assert.Equal(t, 1*time.Hour, cfg.MaxDuration)
}

func TestBackoffConfig_NextDelay(t *testing.T) {
	cfg := DefaultBackoffConfig()

	// Attempt 0: 5s
	assert.Equal(t, 5*time.Second, cfg.NextDelay(0))
	// Attempt 1: 10s
	assert.Equal(t, 10*time.Second, cfg.NextDelay(1))
	// Attempt 2: 20s
	assert.Equal(t, 20*time.Second, cfg.NextDelay(2))
	// Attempt 3: 40s
	assert.Equal(t, 40*time.Second, cfg.NextDelay(3))
	// Attempt 4: 80s
	assert.Equal(t, 80*time.Second, cfg.NextDelay(4))
	// Attempt 5: 160s
	assert.Equal(t, 160*time.Second, cfg.NextDelay(5))
	// Attempt 6: 320s
	assert.Equal(t, 320*time.Second, cfg.NextDelay(6))
}

func TestBackoffConfig_MaxDelayCap(t *testing.T) {
	cfg := DefaultBackoffConfig()

	// Attempt 10: 5s * 2^10 = 5120s = 85min → capped at 10min
	assert.Equal(t, 10*time.Minute, cfg.NextDelay(10))
	// Attempt 20: still capped
	assert.Equal(t, 10*time.Minute, cfg.NextDelay(20))
}

func TestBackoffConfig_IsExhausted(t *testing.T) {
	cfg := DefaultBackoffConfig()

	assert.False(t, cfg.IsExhausted(0))
	assert.False(t, cfg.IsExhausted(30*time.Minute))
	assert.False(t, cfg.IsExhausted(59*time.Minute))
	assert.True(t, cfg.IsExhausted(1*time.Hour))
	assert.True(t, cfg.IsExhausted(2*time.Hour))
}

func TestBackoffConfig_CustomValues(t *testing.T) {
	cfg := BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		Factor:       3.0,
		MaxDelay:     1 * time.Second,
		MaxDuration:  5 * time.Second,
	}

	assert.Equal(t, 100*time.Millisecond, cfg.NextDelay(0))
	assert.Equal(t, 300*time.Millisecond, cfg.NextDelay(1))
	assert.Equal(t, 900*time.Millisecond, cfg.NextDelay(2))
	assert.Equal(t, 1*time.Second, cfg.NextDelay(3)) // capped
	assert.True(t, cfg.IsExhausted(5*time.Second))
}
