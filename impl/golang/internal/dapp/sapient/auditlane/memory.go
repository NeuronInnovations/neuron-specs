package auditlane

import (
	"context"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// MemoryLane is an in-process Lane for tests and the single-process demo. It fans
// each published message out to every live subscriber on the matching channel.
// Safe for concurrent Publish/Subscribe from multiple goroutines.
type MemoryLane struct {
	mu     sync.Mutex
	subs   map[string][]chan *sapientpb.SapientMessage
	closed bool
	buf    int
}

// NewMemoryLane returns an empty in-memory lane with a generous per-subscriber
// buffer (so deterministic tests never drop).
func NewMemoryLane() *MemoryLane {
	return &MemoryLane{subs: make(map[string][]chan *sapientpb.SapientMessage), buf: 1024}
}

// Publish clones msg and delivers it to every subscriber on ch. Delivery is
// non-blocking under the lock (a saturated subscriber drops rather than stalling
// the publisher); the 1024 buffer keeps tests lossless.
func (m *MemoryLane) Publish(_ context.Context, ch Channel, msg *sapientpb.SapientMessage) error {
	cp, ok := proto.Clone(msg).(*sapientpb.SapientMessage)
	if !ok {
		return New(ErrEncode, "Publish", "clone produced wrong message type")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return New(ErrLaneClosed, "Publish", "lane is closed")
	}
	for _, sub := range m.subs[ch.key()] {
		select {
		case sub <- cp:
		default:
		}
	}
	return nil
}

// Subscribe registers a subscriber on ch; the returned channel closes when ctx is
// cancelled or the lane is closed.
func (m *MemoryLane) Subscribe(ctx context.Context, ch Channel) (<-chan *sapientpb.SapientMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, New(ErrLaneClosed, "Subscribe", "lane is closed")
	}
	out := make(chan *sapientpb.SapientMessage, m.buf)
	key := ch.key()
	m.subs[key] = append(m.subs[key], out)

	go func() {
		<-ctx.Done()
		m.mu.Lock()
		defer m.mu.Unlock()
		subs := m.subs[key]
		for i, s := range subs {
			if s == out {
				// INVARIANT: remove from m.subs AND close under the same mutex
				// acquisition. Publish only sends to channels still in m.subs
				// (also under the lock), so this pairing makes send-on-closed and
				// double-close impossible. Do NOT split these two operations.
				m.subs[key] = append(subs[:i:i], subs[i+1:]...)
				close(out)
				return
			}
		}
	}()
	return out, nil
}

// Close marks the lane closed and closes every live subscriber channel. Publish
// after Close returns ErrLaneClosed.
func (m *MemoryLane) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	for key, subs := range m.subs {
		for _, s := range subs {
			close(s)
		}
		delete(m.subs, key)
	}
	return nil
}
