package receivedtap

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func proj(id string, at time.Time) Projection {
	return Projection{ReceivedAt: at, MessageType: "DetectionReport", ObjectID: id, ID: id}
}

func TestStore_LatestNewestFirst(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 0)
	base := time.Unix(1000, 0)
	for i := range 3 {
		s.Record(proj(fmt.Sprintf("obj-%d", i), base.Add(time.Duration(i)*time.Second)))
	}
	got := s.Latest(10)
	require.Len(t, got, 3)
	assert.Equal(t, "obj-2", got[0].ObjectID, "newest first")
	assert.Equal(t, "obj-1", got[1].ObjectID)
	assert.Equal(t, "obj-0", got[2].ObjectID)
}

func TestStore_BoundedRetention(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(4, 0)
	base := time.Unix(1000, 0)
	for i := range 10 {
		s.Record(proj(fmt.Sprintf("obj-%d", i), base.Add(time.Duration(i)*time.Second)))
	}
	got := s.Latest(100)
	require.Len(t, got, 4, "ring bounded to capacity")
	assert.Equal(t, "obj-9", got[0].ObjectID)
	assert.Equal(t, "obj-6", got[3].ObjectID, "oldest retained is the 4th-newest")

	h := s.Health()
	assert.Equal(t, 4, h.RetainedCount)
	assert.Equal(t, 4, h.Capacity)
	assert.EqualValues(t, 10, h.TotalReceived)
}

func TestStore_LatestPerObject(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 0)
	base := time.Unix(1000, 0)
	for i := range 5 {
		s.Record(proj("same-obj", base.Add(time.Duration(i)*time.Second)))
	}
	s.Record(proj("other-obj", base.Add(time.Hour)))

	h := s.Health()
	assert.Equal(t, 2, h.ObjectCount, "distinct objects, not records")
	assert.Equal(t, 6, h.RetainedCount)
	require.NotNil(t, h.LatestReceivedAt)
}

func TestStore_MissingObjectIdFallsBackToId(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 0)
	s.Record(Projection{ReceivedAt: time.Unix(1, 0), ID: "serial-only"}) // no ObjectID
	s.Record(Projection{ReceivedAt: time.Unix(2, 0)})                    // neither → no key
	assert.Equal(t, 1, s.Health().ObjectCount, "keyed by id when objectId absent; empty key skipped")
}

func TestStore_ConcurrentReadersWriters(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(64, 8)
	stop := make(chan struct{})

	require.NotPanics(t, func() {
		var writers, readers sync.WaitGroup
		for w := range 4 {
			writers.Go(func() {
				for i := range 500 {
					s.Record(proj(fmt.Sprintf("w%d-%d", w, i), time.Unix(int64(i), 0)))
				}
			})
		}
		for range 4 {
			readers.Go(func() {
				for {
					select {
					case <-stop:
						return
					default:
						_ = s.Latest(10)
						_ = s.Health()
					}
				}
			})
		}
		writers.Wait() // all writes done
		close(stop)    // then drain readers
		readers.Wait()
	})
}

func TestStore_SubscriberCap(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 2)
	c1, ok1 := s.Subscribe()
	require.True(t, ok1)
	c2, ok2 := s.Subscribe()
	require.True(t, ok2)
	_, ok3 := s.Subscribe()
	assert.False(t, ok3, "third subscriber rejected at cap")

	s.Unsubscribe(c1)
	_, ok4 := s.Subscribe()
	assert.True(t, ok4, "slot frees after unsubscribe")
	_ = c2
}

func TestStore_RecordDoesNotBlockOnFullSubscriber(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 1)
	_, ok := s.Subscribe() // subscriber that never drains its buffer
	require.True(t, ok)

	done := make(chan struct{})
	go func() {
		for i := range subscriberBuffer + 50 { // overflow the buffer
			s.Record(proj("x", time.Unix(int64(i), 0)))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Record blocked on a full subscriber buffer (drop-on-full violated)")
	}
}

func TestStore_HealthEmpty(t *testing.T) {
	t.Parallel()
	h := NewReceivedSapientStore(0, 0).Health()
	assert.Equal(t, 0, h.RetainedCount)
	assert.Equal(t, 0, h.ObjectCount)
	assert.Nil(t, h.LatestReceivedAt)
	assert.Nil(t, h.StreamFreshnessSeconds)
}

// TestStore_LatestScopedByNodeID: the same object_id arriving from two
// different source nodes must be two distinct objects (multi-source demo) —
// never collapsed into one.
func TestStore_LatestScopedByNodeID(t *testing.T) {
	s := NewReceivedSapientStore(8, 2)
	s.Record(Projection{NodeID: "node-rid", ObjectID: "01SAMEULID"})
	s.Record(Projection{NodeID: "node-jv", ObjectID: "01SAMEULID"})
	require.Equal(t, 2, s.Health().ObjectCount, "same object_id from two nodes = two objects")

	// Legacy frames without a node_id keep the bare key.
	s.Record(Projection{ObjectID: "01LEGACY"})
	s.Record(Projection{ObjectID: "01LEGACY"})
	require.Equal(t, 3, s.Health().ObjectCount)
}
