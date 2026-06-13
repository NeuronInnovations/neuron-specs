package persistence

import (
	"sync"
	"time"
)

// MemoryStore is a non-persistent ActiveServiceStore useful for tests and
// for runs that intentionally opt out of disk persistence. Operations are
// serialized via an internal mutex.
type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]ActiveServiceEntry
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{entries: make(map[string]ActiveServiceEntry)}
}

func (s *MemoryStore) Save(entry ActiveServiceEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.RequestID] = entry
	return nil
}

func (s *MemoryStore) Load(requestID string) (ActiveServiceEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[requestID]
	if !ok {
		return ActiveServiceEntry{}, ErrNotFound
	}
	return e, nil
}

func (s *MemoryStore) Replay(now time.Time) ([]ActiveServiceEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nowNanos := now.UnixNano()
	out := make([]ActiveServiceEntry, 0, len(s.entries))
	for _, e := range s.entries {
		if e.ExpiresAt != 0 && e.ExpiresAt <= nowNanos {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *MemoryStore) Evict(now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nowNanos := now.UnixNano()
	var count int
	for k, e := range s.entries {
		if e.ExpiresAt != 0 && e.ExpiresAt <= nowNanos {
			delete(s.entries, k)
			count++
		}
	}
	return count, nil
}

func (s *MemoryStore) Delete(requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, requestID)
	return nil
}
