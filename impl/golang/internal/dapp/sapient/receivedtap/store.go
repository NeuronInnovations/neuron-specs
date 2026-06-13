package receivedtap

import (
	"sync"
	"time"
)

const (
	// DefaultCapacity bounds the ring of retained projections (latest N).
	DefaultCapacity = 256
	// DefaultMaxSubscribers bounds concurrent streaming clients.
	DefaultMaxSubscribers = 16
	// subscriberBuffer is the per-client channel buffer; a slow client whose
	// buffer fills loses messages (drop-on-full) rather than backpressuring the
	// data plane.
	subscriberBuffer = 32
)

// ReceivedSapientStore is a bounded, concurrent in-memory store of received
// SAPIENT projections with a non-blocking fan-out to streaming subscribers.
//
// It uses two independent mutexes — mu guards the ring + latest-per-object index
// + counters; subMu guards the subscriber set. They are never held
// simultaneously (Record releases mu before broadcasting under subMu), so there
// is no lock-ordering hazard. Record never blocks the caller: a bounded ring
// write under mu, then a drop-on-full broadcast under subMu.
type ReceivedSapientStore struct {
	capacity int

	mu     sync.RWMutex
	ring   []Projection // circular buffer of size capacity
	head   int          // index of the next write
	count  int          // valid entries (≤ capacity)
	latest map[string]Projection
	total  uint64    // monotonic count of all Record calls
	lastAt time.Time // ReceivedAt of the most recent record

	subMu   sync.Mutex
	subs    map[chan Projection]struct{}
	maxSubs int
}

// NewReceivedSapientStore returns a store retaining the last capacity
// projections and admitting at most maxSubs concurrent stream subscribers.
// Non-positive arguments fall back to the package defaults.
func NewReceivedSapientStore(capacity, maxSubs int) *ReceivedSapientStore {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	if maxSubs <= 0 {
		maxSubs = DefaultMaxSubscribers
	}
	return &ReceivedSapientStore{
		capacity: capacity,
		ring:     make([]Projection, capacity),
		latest:   make(map[string]Projection),
		subs:     make(map[chan Projection]struct{}),
		maxSubs:  maxSubs,
	}
}

// Record stores p and fans it out to subscribers. It is non-blocking with
// respect to the caller: the ring write is O(1) under mu, and the broadcast
// drops on any full subscriber buffer rather than waiting.
func (s *ReceivedSapientStore) Record(p Projection) {
	s.mu.Lock()
	s.ring[s.head] = p
	s.head = (s.head + 1) % s.capacity
	if s.count < s.capacity {
		s.count++
	}
	if key := objectKey(p); key != "" {
		s.latest[key] = p
	}
	s.total++
	if p.ReceivedAt.After(s.lastAt) {
		s.lastAt = p.ReceivedAt
	}
	s.mu.Unlock()

	s.broadcast(p)
}

// Latest returns up to limit projections, newest-first. A non-positive limit
// returns all retained projections. The returned slice is a fresh copy.
func (s *ReceivedSapientStore) Latest(limit int) []Projection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := s.count
	if limit > 0 && limit < n {
		n = limit
	}
	out := make([]Projection, 0, n)
	for i := 0; i < n; i++ {
		idx := ((s.head-1-i)%s.capacity + s.capacity) % s.capacity
		out = append(out, s.ring[idx])
	}
	return out
}

// Health is the read-only liveness/size view of the store. It carries no
// secrets, env, keys, or host paths.
type Health struct {
	LatestReceivedAt       *time.Time `json:"latestReceivedAt,omitempty"`
	RetainedCount          int        `json:"retainedCount"`
	ObjectCount            int        `json:"objectCount"`
	TotalReceived          uint64     `json:"totalReceived"`
	Subscribers            int        `json:"subscribers"`
	StreamFreshnessSeconds *float64   `json:"streamFreshnessSeconds,omitempty"`
	Capacity               int        `json:"capacity"`
}

// Health returns a snapshot of the store's size and freshness.
func (s *ReceivedSapientStore) Health() Health {
	s.mu.RLock()
	h := Health{
		RetainedCount: s.count,
		ObjectCount:   len(s.latest),
		TotalReceived: s.total,
		Capacity:      s.capacity,
	}
	if !s.lastAt.IsZero() {
		t := s.lastAt
		h.LatestReceivedAt = &t
		fresh := time.Since(t).Seconds()
		h.StreamFreshnessSeconds = &fresh
	}
	s.mu.RUnlock()

	s.subMu.Lock()
	h.Subscribers = len(s.subs)
	s.subMu.Unlock()
	return h
}

// Subscribe registers a streaming subscriber, returning its channel and true, or
// (nil, false) when the subscriber cap is reached. The caller MUST Unsubscribe
// when done (typically via defer in the stream handler). The returned channel is
// for receiving only.
func (s *ReceivedSapientStore) Subscribe() (chan Projection, bool) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if len(s.subs) >= s.maxSubs {
		return nil, false
	}
	ch := make(chan Projection, subscriberBuffer)
	s.subs[ch] = struct{}{}
	return ch, true
}

// Unsubscribe removes a subscriber. It is safe to call once per channel returned
// by Subscribe; the channel is left unclosed (so a concurrent broadcast can
// never send on a closed channel) and is reclaimed by GC once the handler drops
// it.
func (s *ReceivedSapientStore) Unsubscribe(ch chan Projection) {
	s.subMu.Lock()
	delete(s.subs, ch)
	s.subMu.Unlock()
}

func (s *ReceivedSapientStore) broadcast(p Projection) {
	s.subMu.Lock()
	for ch := range s.subs {
		select {
		case ch <- p:
		default: // drop on slow client; never block the data plane
		}
	}
	s.subMu.Unlock()
}

// objectKey scopes the latest-per-object index by the sending node: two
// sellers (multi-source) can legitimately mint colliding object identifiers,
// so the key is node_id-qualified. Legacy projections without a node_id keep
// the bare key (pre-multi-source behavior). The key is internal — the HTTP
// API exposes only values and counts.
func objectKey(p Projection) string {
	key := p.ObjectID
	if key == "" {
		key = p.ID
	}
	if key == "" {
		return ""
	}
	if p.NodeID != "" {
		return p.NodeID + "|" + key
	}
	return key
}
