package persistence

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each store backend MUST satisfy the same behavioral contract. We run a
// shared suite against MemoryStore and JSONFileStore.
//
// Test names use the backend tag as a t.Run prefix so failures are easy to
// trace to the affected backend.

func eachStore(t *testing.T, fn func(t *testing.T, s ActiveServiceStore)) {
	t.Helper()
	t.Run("MemoryStore", func(t *testing.T) {
		t.Parallel()
		fn(t, NewMemoryStore())
	})
	t.Run("JSONFileStore", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "entries.json")
		s, err := NewJSONFileStore(path)
		require.NoError(t, err)
		fn(t, s)
	})
}

func sampleEntry(reqID string, expiresAt int64) ActiveServiceEntry {
	return ActiveServiceEntry{
		RequestID:      reqID,
		CounterpartEVM: "0x1111111111111111111111111111111111111111",
		Role:           RoleSeller,
		ServiceName:    "adsb",
		ServiceVersion: "1.0.0",
		EscrowRef:      "hedera-native:0.0.42",
		LastInvoiceSeq: 7,
		State:          "ACTIVE",
		AcceptedAt:     1704067200000000000,
		ExpiresAt:      expiresAt,
	}
}

func TestStore_SaveLoad(t *testing.T) {
	eachStore(t, func(t *testing.T, s ActiveServiceStore) {
		e := sampleEntry("req-1", 1704153600000000000)
		require.NoError(t, s.Save(e))

		got, err := s.Load("req-1")
		require.NoError(t, err)
		assert.Equal(t, e, got)
	})
}

func TestStore_LoadMissingReturnsErrNotFound(t *testing.T) {
	eachStore(t, func(t *testing.T, s ActiveServiceStore) {
		_, err := s.Load("does-not-exist")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestStore_SaveUpdates(t *testing.T) {
	eachStore(t, func(t *testing.T, s ActiveServiceStore) {
		e := sampleEntry("req-1", 0)
		require.NoError(t, s.Save(e))

		e.LastInvoiceSeq = 99
		e.State = "INVOICED"
		require.NoError(t, s.Save(e))

		got, err := s.Load("req-1")
		require.NoError(t, err)
		assert.Equal(t, uint64(99), got.LastInvoiceSeq)
		assert.Equal(t, "INVOICED", got.State)
	})
}

func TestStore_Replay_SkipsExpired(t *testing.T) {
	eachStore(t, func(t *testing.T, s ActiveServiceStore) {
		past := time.Now().Add(-1 * time.Hour).UnixNano()
		future := time.Now().Add(1 * time.Hour).UnixNano()

		require.NoError(t, s.Save(sampleEntry("expired", past)))
		require.NoError(t, s.Save(sampleEntry("active", future)))
		require.NoError(t, s.Save(sampleEntry("no-expiry", 0)))

		entries, err := s.Replay(time.Now())
		require.NoError(t, err)
		require.Len(t, entries, 2, "expected 2 non-expired entries; got %d: %+v", len(entries), entries)

		ids := map[string]bool{}
		for _, e := range entries {
			ids[e.RequestID] = true
		}
		assert.True(t, ids["active"])
		assert.True(t, ids["no-expiry"])
		assert.False(t, ids["expired"])
	})
}

func TestStore_Evict_RemovesExpired(t *testing.T) {
	eachStore(t, func(t *testing.T, s ActiveServiceStore) {
		past := time.Now().Add(-1 * time.Hour).UnixNano()
		future := time.Now().Add(1 * time.Hour).UnixNano()

		require.NoError(t, s.Save(sampleEntry("a", past)))
		require.NoError(t, s.Save(sampleEntry("b", past)))
		require.NoError(t, s.Save(sampleEntry("c", future)))

		count, err := s.Evict(time.Now())
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		_, err = s.Load("a")
		assert.ErrorIs(t, err, ErrNotFound)
		_, err = s.Load("c")
		assert.NoError(t, err)
	})
}

func TestStore_Delete(t *testing.T) {
	eachStore(t, func(t *testing.T, s ActiveServiceStore) {
		require.NoError(t, s.Save(sampleEntry("req-1", 0)))

		require.NoError(t, s.Delete("req-1"))

		_, err := s.Load("req-1")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestStore_DeleteMissingIsIdempotent(t *testing.T) {
	eachStore(t, func(t *testing.T, s ActiveServiceStore) {
		require.NoError(t, s.Delete("never-existed"))
	})
}

// JSON-file-specific behaviors.

func TestJSONFileStore_PersistsAcrossInstances(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "entries.json")

	s1, err := NewJSONFileStore(path)
	require.NoError(t, err)
	require.NoError(t, s1.Save(sampleEntry("req-1", 0)))

	// New store instance against the same path — simulates a process restart.
	s2, err := NewJSONFileStore(path)
	require.NoError(t, err)

	got, err := s2.Load("req-1")
	require.NoError(t, err)
	assert.Equal(t, "req-1", got.RequestID)
	assert.Equal(t, "ACTIVE", got.State)
}

func TestJSONFileStore_RejectsEmptyPath(t *testing.T) {
	t.Parallel()
	_, err := NewJSONFileStore("")
	assert.Error(t, err)
}

func TestNewExpiresAt(t *testing.T) {
	t.Parallel()
	accepted := time.Unix(1704067200, 0)

	assert.Equal(t, accepted.Add(24*time.Hour).UnixNano(), NewExpiresAt(accepted, 24*time.Hour))
	assert.Zero(t, NewExpiresAt(accepted, 0))
	assert.Zero(t, NewExpiresAt(accepted, -1*time.Second))
}
