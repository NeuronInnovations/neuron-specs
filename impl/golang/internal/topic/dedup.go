package topic

import (
	"crypto/sha256"
	"sync"
)

// FR-T25: Message deduplication tracker
// DeduplicationTracker provides opt-in replay detection for topic messages.
// It tracks seen message signatures to identify duplicates.
// Safe for concurrent use.
type DeduplicationTracker struct {
	seen map[[32]byte]struct{}
	mu   sync.Mutex
}

// NewDeduplicationTracker creates a new DeduplicationTracker.
func NewDeduplicationTracker() *DeduplicationTracker {
	return &DeduplicationTracker{
		seen: make(map[[32]byte]struct{}),
	}
}

// IsDuplicate returns true if this message has been seen before.
// It uses the SHA-256 hash of the message signature as the dedup key,
// since signatures are unique per message content (RFC 6979 deterministic nonces).
func (d *DeduplicationTracker) IsDuplicate(msg TopicMessage) bool {
	sig := msg.Signature()
	if len(sig) == 0 {
		return false
	}
	key := sha256.Sum256(sig)

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.seen[key]; exists {
		return true
	}
	d.seen[key] = struct{}{}
	return false
}

// Reset clears all tracked messages.
func (d *DeduplicationTracker) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen = make(map[[32]byte]struct{})
}

// Size returns the number of tracked messages.
func (d *DeduplicationTracker) Size() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.seen)
}
