package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JSONFileStoreSchemaVersion identifies the on-disk format. Bump when
// adding breaking changes; the loader rejects files with a different
// version and treats them as missing (fresh-start semantics).
const JSONFileStoreSchemaVersion = 1

// JSONFileStore is a single-file JSON-on-disk ActiveServiceStore. The file
// format is a JSON object with a schemaVersion + an array of entries:
//
//	{
//	  "schemaVersion": 1,
//	  "updatedAt": "2026-05-08T12:34:56Z",
//	  "entries": [ ... ]
//	}
//
// Writes are atomic via temp+rename, mirroring the existing
// internal/edgeapp state.go pattern. Reads of a missing file return an
// empty store (not an error), so first-time startup and post-truncation
// recovery are indistinguishable.
//
// Suitable for the edge demo and the reference MVP at entry counts in the low
// hundreds. For higher counts or per-entry update efficiency, a different
// backend (SQLite, KV store) would be appropriate; this store rewrites
// the entire file on each Save.
type JSONFileStore struct {
	path string
	mu   sync.Mutex
}

// NewJSONFileStore returns a store backed by the given file path. The
// file is created on first Save; the parent directory MUST exist.
func NewJSONFileStore(path string) (*JSONFileStore, error) {
	if path == "" {
		return nil, errors.New("payment/persistence: empty path")
	}
	return &JSONFileStore{path: path}, nil
}

// fileShape mirrors the on-disk JSON layout.
type fileShape struct {
	SchemaVersion int                  `json:"schemaVersion"`
	UpdatedAt     string               `json:"updatedAt,omitempty"`
	Entries       []ActiveServiceEntry `json:"entries"`
}

// load reads the file and returns the parsed entry list. Missing file ⇒
// empty list. Schema mismatch ⇒ empty list (treat as "no state").
func (s *JSONFileStore) load() ([]ActiveServiceEntry, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("payment/persistence: read: %w", err)
	}
	var f fileShape
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("payment/persistence: parse: %w", err)
	}
	if f.SchemaVersion != JSONFileStoreSchemaVersion {
		return nil, nil
	}
	return f.Entries, nil
}

// writeAtomic writes the entry list atomically.
func (s *JSONFileStore) writeAtomic(entries []ActiveServiceEntry) error {
	shape := fileShape{
		SchemaVersion: JSONFileStoreSchemaVersion,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Entries:       entries,
	}
	data, err := json.MarshalIndent(shape, "", "  ")
	if err != nil {
		return fmt.Errorf("payment/persistence: marshal: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".active-service-*.tmp")
	if err != nil {
		return fmt.Errorf("payment/persistence: create tmp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("payment/persistence: write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("payment/persistence: fsync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("payment/persistence: close tmp: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		cleanup()
		return fmt.Errorf("payment/persistence: rename: %w", err)
	}
	return nil
}

// upsert returns a new slice with the given entry inserted or replaced.
func upsert(entries []ActiveServiceEntry, entry ActiveServiceEntry) []ActiveServiceEntry {
	for i, e := range entries {
		if e.RequestID == entry.RequestID {
			entries[i] = entry
			return entries
		}
	}
	return append(entries, entry)
}

func (s *JSONFileStore) Save(entry ActiveServiceEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.load()
	if err != nil {
		return err
	}
	entries = upsert(entries, entry)
	return s.writeAtomic(entries)
}

func (s *JSONFileStore) Load(requestID string) (ActiveServiceEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.load()
	if err != nil {
		return ActiveServiceEntry{}, err
	}
	for _, e := range entries {
		if e.RequestID == requestID {
			return e, nil
		}
	}
	return ActiveServiceEntry{}, ErrNotFound
}

func (s *JSONFileStore) Replay(now time.Time) ([]ActiveServiceEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.load()
	if err != nil {
		return nil, err
	}
	nowNanos := now.UnixNano()
	out := make([]ActiveServiceEntry, 0, len(entries))
	for _, e := range entries {
		if e.ExpiresAt != 0 && e.ExpiresAt <= nowNanos {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *JSONFileStore) Evict(now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.load()
	if err != nil {
		return 0, err
	}
	nowNanos := now.UnixNano()
	kept := make([]ActiveServiceEntry, 0, len(entries))
	var evicted int
	for _, e := range entries {
		if e.ExpiresAt != 0 && e.ExpiresAt <= nowNanos {
			evicted++
			continue
		}
		kept = append(kept, e)
	}
	if evicted == 0 {
		return 0, nil
	}
	if err := s.writeAtomic(kept); err != nil {
		return 0, err
	}
	return evicted, nil
}

func (s *JSONFileStore) Delete(requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.load()
	if err != nil {
		return err
	}
	kept := make([]ActiveServiceEntry, 0, len(entries))
	var found bool
	for _, e := range entries {
		if e.RequestID == requestID {
			found = true
			continue
		}
		kept = append(kept, e)
	}
	if !found {
		return nil
	}
	return s.writeAtomic(kept)
}
