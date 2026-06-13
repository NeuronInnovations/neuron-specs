package topic

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mkInitialTxID returns a stable initial_transaction_id for a chunk group.
func mkInitialTxID(account, validStart string, nonce int) *mirrorInitialTransactionID {
	return &mirrorInitialTransactionID{
		AccountID:             account,
		Nonce:                 nonce,
		Scheduled:             false,
		TransactionValidStart: validStart,
	}
}

// mkChunk builds a synthetic mirrorMessage for chunk N of total, carrying the
// given payload bytes (already base64-encoded internally).
func mkChunk(seq uint64, ts string, payload string, n, total int, txid *mirrorInitialTransactionID) mirrorMessage {
	return mirrorMessage{
		ConsensusTimestamp: ts,
		Message:            base64.StdEncoding.EncodeToString([]byte(payload)),
		SequenceNumber:     seq,
		ChunkInfo: &mirrorChunkInfo{
			Number:               n,
			Total:                total,
			InitialTransactionID: txid,
		},
	}
}

func TestChunkAssembler_SingleChunk(t *testing.T) {
	asm := newChunkAssembler(time.Minute)
	now := time.Now()

	m := mirrorMessage{
		ConsensusTimestamp: "100.000000000",
		Message:            base64.StdEncoding.EncodeToString([]byte("hello")),
		SequenceNumber:     1,
		// ChunkInfo == nil → fast path
	}
	out, ok := asm.ingest(m, now)
	require.True(t, ok)
	assert.Equal(t, []byte("hello"), out.Contents)
	assert.Equal(t, uint64(1), out.SequenceNumber)
}

func TestChunkAssembler_SingleChunkWithExplicitTotal1(t *testing.T) {
	asm := newChunkAssembler(time.Minute)
	now := time.Now()

	m := mkChunk(1, "100.000000000", "hello", 1, 1,
		mkInitialTxID("0.0.5", "100.0", 0))
	out, ok := asm.ingest(m, now)
	require.True(t, ok, "Total=1 must be treated as single-chunk fast path")
	assert.Equal(t, []byte("hello"), out.Contents)
}

func TestChunkAssembler_MultiChunkInOrder(t *testing.T) {
	asm := newChunkAssembler(time.Minute)
	now := time.Now()
	txid := mkInitialTxID("0.0.5", "100.0", 0)

	// Chunk 1/3 — buffered, ok=false
	out, ok := asm.ingest(mkChunk(1, "100.0", "AAA", 1, 3, txid), now)
	assert.False(t, ok)
	assert.Empty(t, out.Contents)

	// Chunk 2/3
	out, ok = asm.ingest(mkChunk(2, "101.0", "BBB", 2, 3, txid), now)
	assert.False(t, ok)

	// Chunk 3/3 — completes; emits concatenated payload.
	out, ok = asm.ingest(mkChunk(3, "102.0", "CCC", 3, 3, txid), now)
	require.True(t, ok)
	assert.Equal(t, []byte("AAABBBCCC"), out.Contents)
	assert.Equal(t, uint64(3), out.SequenceNumber, "completion takes the LAST chunk's seq")
	assert.Equal(t, uint64(102_000_000_000), out.ConsensusTimestamp,
		"completion takes the LAST chunk's consensus timestamp")
}

func TestChunkAssembler_MultiChunkOutOfOrder(t *testing.T) {
	asm := newChunkAssembler(time.Minute)
	now := time.Now()
	txid := mkInitialTxID("0.0.5", "100.0", 0)

	// Out-of-order arrivals — common when mirror node ingests concurrent
	// publishes from different observers.
	_, ok := asm.ingest(mkChunk(3, "102.0", "CCC", 3, 3, txid), now)
	assert.False(t, ok)
	_, ok = asm.ingest(mkChunk(1, "100.0", "AAA", 1, 3, txid), now)
	assert.False(t, ok)
	out, ok := asm.ingest(mkChunk(2, "101.0", "BBB", 2, 3, txid), now)
	require.True(t, ok)
	assert.Equal(t, []byte("AAABBBCCC"), out.Contents)
}

func TestChunkAssembler_MultiplePublishersInterleaved(t *testing.T) {
	asm := newChunkAssembler(time.Minute)
	now := time.Now()
	tA := mkInitialTxID("0.0.5", "100.0", 0)
	tB := mkInitialTxID("0.0.7", "100.5", 0)

	// Interleave two 2-chunk publishers.
	_, ok := asm.ingest(mkChunk(1, "100.0", "alpha-1", 1, 2, tA), now)
	assert.False(t, ok)
	_, ok = asm.ingest(mkChunk(2, "100.5", "beta-1", 1, 2, tB), now)
	assert.False(t, ok)

	outA, okA := asm.ingest(mkChunk(3, "101.0", "alpha-2", 2, 2, tA), now)
	require.True(t, okA)
	assert.Equal(t, []byte("alpha-1alpha-2"), outA.Contents)

	outB, okB := asm.ingest(mkChunk(4, "102.0", "beta-2", 2, 2, tB), now)
	require.True(t, okB)
	assert.Equal(t, []byte("beta-1beta-2"), outB.Contents)
}

func TestChunkAssembler_TTLEvictsPartialGroup(t *testing.T) {
	asm := newChunkAssembler(time.Minute)
	t0 := time.Now()
	txid := mkInitialTxID("0.0.5", "100.0", 0)

	_, ok := asm.ingest(mkChunk(1, "100.0", "AAA", 1, 3, txid), t0)
	assert.False(t, ok)
	assert.Len(t, asm.buf, 1)

	// Advance past TTL; gc should drop the partial group.
	asm.gc(t0.Add(2 * time.Minute))
	assert.Empty(t, asm.buf, "partial group must be evicted after TTL")
}

func TestChunkAssembler_DuplicateChunk(t *testing.T) {
	asm := newChunkAssembler(time.Minute)
	now := time.Now()
	txid := mkInitialTxID("0.0.5", "100.0", 0)

	_, ok := asm.ingest(mkChunk(1, "100.0", "AAA", 1, 2, txid), now)
	assert.False(t, ok)

	// Same chunk arriving twice — idempotent.
	_, ok = asm.ingest(mkChunk(1, "100.0", "AAA", 1, 2, txid), now)
	assert.False(t, ok)

	out, ok := asm.ingest(mkChunk(2, "101.0", "BBB", 2, 2, txid), now)
	require.True(t, ok)
	assert.Equal(t, []byte("AAABBB"), out.Contents)
}

func TestChunkAssembler_BadInputs(t *testing.T) {
	asm := newChunkAssembler(time.Minute)
	now := time.Now()
	txid := mkInitialTxID("0.0.5", "100.0", 0)

	// Chunk number out of range.
	_, ok := asm.ingest(mkChunk(1, "100.0", "x", 0, 3, txid), now)
	assert.False(t, ok, "Number=0 is invalid; chunk numbers are 1..Total")

	_, ok = asm.ingest(mkChunk(1, "100.0", "x", 5, 3, txid), now)
	assert.False(t, ok, "Number > Total is invalid")

	// Missing initial_transaction_id.
	bad := mirrorMessage{
		Message:        base64.StdEncoding.EncodeToString([]byte("y")),
		SequenceNumber: 99,
		ChunkInfo:      &mirrorChunkInfo{Number: 1, Total: 2, InitialTransactionID: nil},
	}
	_, ok = asm.ingest(bad, now)
	assert.False(t, ok)

	// Single-chunk fast path with malformed base64.
	bad2 := mirrorMessage{Message: "!!not-base64!!", SequenceNumber: 100}
	_, ok = asm.ingest(bad2, now)
	assert.False(t, ok)
}

func TestChunkAssembler_LargePayloadConcat(t *testing.T) {
	// Realistic shape: 3-chunk publish where each chunk is ~700 B → final
	// payload >2 KB. Verifies concatenation order is preserved bytewise.
	asm := newChunkAssembler(time.Minute)
	now := time.Now()
	txid := mkInitialTxID("0.0.5", "100.0", 0)

	chunks := []string{
		strings.Repeat("A", 700),
		strings.Repeat("B", 700),
		strings.Repeat("C", 700),
	}
	for i, c := range chunks[:len(chunks)-1] {
		_, ok := asm.ingest(mkChunk(uint64(i+1), "100.0", c, i+1, 3, txid), now)
		assert.False(t, ok)
	}
	out, ok := asm.ingest(mkChunk(3, "100.0", chunks[2], 3, 3, txid), now)
	require.True(t, ok)
	assert.Len(t, out.Contents, 2100)
	assert.Equal(t, byte('A'), out.Contents[0])
	assert.Equal(t, byte('B'), out.Contents[700])
	assert.Equal(t, byte('C'), out.Contents[1400])
}
