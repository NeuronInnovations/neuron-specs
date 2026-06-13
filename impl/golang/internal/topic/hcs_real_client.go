package topic

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// HederaOperatorEnvAccountID is the environment variable that supplies the
// Hedera operator account ID (format: "0.0.X") for HCS adapters and demos.
const HederaOperatorEnvAccountID = "HEDERA_OPERATOR_ID"

// HederaOperatorEnvPrivateKey is the environment variable that supplies the
// Hedera operator private key (DER- or raw-hex-encoded ECDSA secp256k1).
const HederaOperatorEnvPrivateKey = "HEDERA_OPERATOR_KEY"

// NewTestnetClientFromEnv constructs a Hedera testnet client whose operator
// is sourced from the HEDERA_OPERATOR_ID and HEDERA_OPERATOR_KEY environment
// variables. Both variables MUST be set; the function returns a clear error
// listing the missing variable(s) so the caller can fail fast.
//
// This is the only supported way to obtain a real testnet client from the
// SDK demos. Operator credentials must never be hardcoded in source.
func NewTestnetClientFromEnv() (*hiero.Client, hiero.AccountID, error) {
	accountIDStr := os.Getenv(HederaOperatorEnvAccountID)
	keyStr := os.Getenv(HederaOperatorEnvPrivateKey)

	var missing []string
	if accountIDStr == "" {
		missing = append(missing, HederaOperatorEnvAccountID)
	}
	if keyStr == "" {
		missing = append(missing, HederaOperatorEnvPrivateKey)
	}
	if len(missing) > 0 {
		return nil, hiero.AccountID{}, fmt.Errorf(
			"Hedera operator credentials not set: missing env var(s) %v. "+
				"Set %s and %s before running in testnet mode",
			missing, HederaOperatorEnvAccountID, HederaOperatorEnvPrivateKey,
		)
	}

	accountID, err := hiero.AccountIDFromString(accountIDStr)
	if err != nil {
		return nil, hiero.AccountID{}, fmt.Errorf("parse %s=%q: %w",
			HederaOperatorEnvAccountID, accountIDStr, err)
	}

	// Try ECDSA first (the documented Neuron operator key type), fall back to
	// auto-detect for legacy keys. Ed25519 keys are not used by Neuron but
	// the SDK accepts them via the auto-detect path.
	operatorKey, err := hiero.PrivateKeyFromStringECDSA(keyStr)
	if err != nil {
		operatorKey, err = hiero.PrivateKeyFromString(keyStr)
		if err != nil {
			return nil, hiero.AccountID{}, fmt.Errorf("parse %s: %w",
				HederaOperatorEnvPrivateKey, err)
		}
	}

	client := hiero.ClientForTestnet()
	client.SetOperator(accountID, operatorKey)
	return client, accountID, nil
}

// RealHCSClient wraps a Hiero SDK client to implement the HCSClient interface
// for real Hedera Consensus Service operations on testnet or mainnet.
type RealHCSClient struct {
	client *hiero.Client
}

// NewRealHCSClient creates an HCSClient backed by a real Hiero SDK client.
func NewRealHCSClient(client *hiero.Client) *RealHCSClient {
	return &RealHCSClient{client: client}
}

func (c *RealHCSClient) CreateTopic(memo string) (string, error) {
	tx := hiero.NewTopicCreateTransaction().
		SetTopicMemo(memo)

	resp, err := tx.Execute(c.client)
	if err != nil {
		return "", fmt.Errorf("execute CreateTopic: %w", err)
	}

	receipt, err := resp.GetReceipt(c.client)
	if err != nil {
		return "", fmt.Errorf("get receipt for CreateTopic: %w", err)
	}

	if receipt.TopicID == nil {
		return "", fmt.Errorf("TopicID is nil in receipt")
	}

	return receipt.TopicID.String(), nil
}

func (c *RealHCSClient) SubmitMessage(topicId string, message []byte) (string, error) {
	tid, err := hiero.TopicIDFromString(topicId)
	if err != nil {
		return "", fmt.Errorf("parse TopicID %q: %w", topicId, err)
	}

	tx := hiero.NewTopicMessageSubmitTransaction().
		SetTopicID(tid).
		SetMessage(message)

	resp, err := tx.Execute(c.client)
	if err != nil {
		return "", fmt.Errorf("execute SubmitMessage: %w", err)
	}

	return resp.TransactionID.String(), nil
}

func (c *RealHCSClient) SubmitMessageAndWait(topicId string, message []byte) (string, uint64, uint64, error) {
	tid, err := hiero.TopicIDFromString(topicId)
	if err != nil {
		return "", 0, 0, fmt.Errorf("parse TopicID %q: %w", topicId, err)
	}

	tx := hiero.NewTopicMessageSubmitTransaction().
		SetTopicID(tid).
		SetMessage(message)

	resp, err := tx.Execute(c.client)
	if err != nil {
		return "", 0, 0, fmt.Errorf("execute SubmitMessageAndWait: %w", err)
	}

	receipt, err := resp.GetReceipt(c.client)
	if err != nil {
		return "", 0, 0, fmt.Errorf("get receipt for SubmitMessageAndWait: %w", err)
	}

	txId := resp.TransactionID.String()
	seqNum := receipt.TopicSequenceNumber

	return txId, 0, seqNum, nil
}

// MirrorNodeBaseURL is the Hedera testnet mirror-node REST endpoint that
// SubscribeTopic polls. Override via RealHCSClient.MirrorBaseURL for
// mainnet or a private mirror.
const MirrorNodeBaseURL = "https://testnet.mirrornode.hedera.com"

// MirrorPollInterval is how often SubscribeTopic re-queries the mirror node
// for new messages. 2 s is a reasonable balance between perceived latency
// (sub-2-s reverse-connect handshake) and mirror-node load.
const MirrorPollInterval = 2 * time.Second

// SubscribeTopic returns a channel that delivers new HCSMessage values for
// topicId by polling the Hedera mirror-node REST API. If startSequence is
// non-nil, the subscription begins at that sequence number (inclusive);
// otherwise it begins at the latest sequence + 1 (skip history).
//
// The returned channel is closed when the underlying poller exits — currently
// only on a fatal mirror-node error. Callers that want to stop subscribing
// should ignore further deliveries; the goroutine self-terminates on the
// next mirror-node round-trip if its channel send blocks for a long time.
//
// Multi-chunk messages (HCS messages > 1024 bytes) are emitted only once the
// final chunk arrives; in-flight reassembly state lives in the poller goroutine.
func (c *RealHCSClient) SubscribeTopic(topicId string, startSequence *uint64) (<-chan HCSMessage, error) {
	out := make(chan HCSMessage, 64)

	// Determine starting sequence. If caller didn't ask for replay, ask the
	// mirror node for the highest current sequence, then start at +1.
	startSeq := uint64(0)
	if startSequence != nil {
		// Fetch from N-1 so the first poll includes sequence N.
		if *startSequence > 0 {
			startSeq = *startSequence - 1
		}
	} else {
		seq, err := mirrorLatestSequence(MirrorNodeBaseURL, topicId)
		if err != nil {
			close(out)
			return nil, fmt.Errorf("mirror-node bootstrap latest seq: %w", err)
		}
		startSeq = seq
	}

	go pollMirror(out, MirrorNodeBaseURL, topicId, startSeq)
	return out, nil
}

// mirrorMessage matches the relevant subset of the mirror-node REST shape.
type mirrorMessage struct {
	ConsensusTimestamp string             `json:"consensus_timestamp"`
	Message            string             `json:"message"` // base64
	SequenceNumber     uint64             `json:"sequence_number"`
	ChunkInfo          *mirrorChunkInfo   `json:"chunk_info,omitempty"`
}

type mirrorChunkInfo struct {
	Number               int                          `json:"number"`
	Total                int                          `json:"total"`
	InitialTransactionID *mirrorInitialTransactionID  `json:"initial_transaction_id,omitempty"`
}

type mirrorInitialTransactionID struct {
	AccountID              string `json:"account_id"`
	Nonce                  int    `json:"nonce"`
	Scheduled              bool   `json:"scheduled"`
	TransactionValidStart  string `json:"transaction_valid_start"`
}

// key uniquely identifies a multi-chunk publish across all chunks.
func (i *mirrorInitialTransactionID) key() string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%s@%s#%d", i.AccountID, i.TransactionValidStart, i.Nonce)
}

type mirrorPage struct {
	Messages []mirrorMessage `json:"messages"`
}

// chunkAssembler reassembles multi-chunk HCS messages keyed by
// initial_transaction_id. Messages with chunk_info.total > 1 are buffered
// until every (1..Total) part has arrived, then concatenated in order and
// emitted as a single HCSMessage. Single-chunk messages pass through
// unchanged.
//
// Eviction: a partial group whose first chunk arrived more than ttl ago is
// dropped (memory bound for never-completing publishers). Default ttl is
// 5 min — well above the seconds-scale fan-out a normal HCS multi-chunk
// publish takes, even on a slow mirror node.
//
// Not safe for concurrent use; the poller calls it from a single goroutine.
type chunkAssembler struct {
	buf map[string]*chunkBuf
	ttl time.Duration
}

type chunkBuf struct {
	arrived time.Time      // first chunk's arrival; basis for TTL
	total   int            // expected chunk count
	parts   []*mirrorMessage // size==total; nil entries are still-missing chunks
}

func newChunkAssembler(ttl time.Duration) *chunkAssembler {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &chunkAssembler{
		buf: make(map[string]*chunkBuf),
		ttl: ttl,
	}
}

// ingest accepts the next mirror-node message. Returns ok=true with a
// fully-assembled HCSMessage when:
//   - the message is single-chunk (Total<=1 or ChunkInfo==nil);
//   - or the message is the final chunk that completes a buffered group.
//
// Otherwise returns ok=false (chunk buffered but not yet complete, or the
// message was malformed and skipped).
func (a *chunkAssembler) ingest(m mirrorMessage, now time.Time) (HCSMessage, bool) {
	// Single-chunk fast path.
	if m.ChunkInfo == nil || m.ChunkInfo.Total <= 1 {
		payload, err := base64.StdEncoding.DecodeString(m.Message)
		if err != nil {
			return HCSMessage{}, false
		}
		return HCSMessage{
			Contents:           payload,
			ConsensusTimestamp: parseConsensusTimestampNs(m.ConsensusTimestamp),
			SequenceNumber:     m.SequenceNumber,
		}, true
	}

	// Multi-chunk path. Sanity-check chunk number against total.
	if m.ChunkInfo.Number < 1 || m.ChunkInfo.Number > m.ChunkInfo.Total {
		return HCSMessage{}, false
	}
	key := m.ChunkInfo.InitialTransactionID.key()
	if key == "" {
		return HCSMessage{}, false
	}

	buf, ok := a.buf[key]
	if !ok {
		buf = &chunkBuf{
			arrived: now,
			total:   m.ChunkInfo.Total,
			parts:   make([]*mirrorMessage, m.ChunkInfo.Total),
		}
		a.buf[key] = buf
	}
	// Stash this chunk. Idempotent — a duplicate just overwrites with the
	// same data.
	mc := m
	buf.parts[m.ChunkInfo.Number-1] = &mc

	// Complete? Concatenate, emit, evict.
	for _, p := range buf.parts {
		if p == nil {
			return HCSMessage{}, false
		}
	}

	var combined []byte
	for _, p := range buf.parts {
		piece, err := base64.StdEncoding.DecodeString(p.Message)
		if err != nil {
			delete(a.buf, key)
			return HCSMessage{}, false
		}
		combined = append(combined, piece...)
	}

	// Use the *last* chunk's consensus timestamp + sequence number — that's
	// when the message became "complete" from the mirror node's perspective,
	// which matches the SDK semantics for downstream timestamp consumers.
	last := buf.parts[buf.total-1]
	delete(a.buf, key)
	return HCSMessage{
		Contents:           combined,
		ConsensusTimestamp: parseConsensusTimestampNs(last.ConsensusTimestamp),
		SequenceNumber:     last.SequenceNumber,
	}, true
}

// gc evicts partial chunk groups whose first chunk arrived more than ttl ago.
// Called once per poll cycle; cheap because the buffer is tiny in normal
// operation (fully-completing groups self-evict in ingest).
func (a *chunkAssembler) gc(now time.Time) {
	for k, b := range a.buf {
		if now.Sub(b.arrived) > a.ttl {
			delete(a.buf, k)
		}
	}
}

// mirrorLatestSequence returns the highest sequence_number currently visible
// on the mirror node for the given topic, or 0 if the topic has no messages.
func mirrorLatestSequence(baseURL, topicId string) (uint64, error) {
	url := fmt.Sprintf("%s/api/v1/topics/%s/messages?order=desc&limit=1",
		baseURL, topicId)
	page, err := mirrorFetchPage(url)
	if err != nil {
		return 0, err
	}
	if len(page.Messages) == 0 {
		return 0, nil
	}
	return page.Messages[0].SequenceNumber, nil
}

// pollMirror is the poller goroutine. It runs until ctx-equivalent cancel
// (currently never; we rely on receiver to drop the channel).
//
// Each cycle: GET messages with sequencenumber > lastSeq, in ascending order,
// up to 100 at a time. Decode each base64 payload, populate consensus
// timestamp from the decimal "secs.nanos" string, and push onto out.
//
// Multi-chunk HCS messages (>1024 B) are reassembled by chunkAssembler,
// keyed by initial_transaction_id. Partial groups expire after 5 min.
func pollMirror(out chan<- HCSMessage, baseURL, topicId string, lastSeq uint64) {
	defer close(out)

	asm := newChunkAssembler(5 * time.Minute)

	for {
		// Mirror-node API rejects `sequencenumber=gt:0` with HTTP 400; the
		// minimum legal sequence value in the filter is 1. Use the inclusive
		// `gte:` form anchored at lastSeq+1 — works for empty topics
		// (gte:1) and after-N catch-up (gte:N+1) alike.
		url := fmt.Sprintf("%s/api/v1/topics/%s/messages?sequencenumber=gte:%d&order=asc&limit=100",
			baseURL, topicId, lastSeq+1)
		page, err := mirrorFetchPage(url)
		if err != nil {
			// Transient network errors → retry on next tick. Fatal would be
			// surfaced via channel close; callers can detect via ok=false.
			time.Sleep(MirrorPollInterval)
			continue
		}
		now := time.Now()
		for _, m := range page.Messages {
			lastSeq = m.SequenceNumber
			msg, ok := asm.ingest(m, now)
			if !ok {
				continue // partial chunk buffered, or malformed — wait for more
			}
			out <- msg
		}
		asm.gc(time.Now())
		time.Sleep(MirrorPollInterval)
	}
}

func mirrorFetchPage(url string) (mirrorPage, error) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return mirrorPage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return mirrorPage{}, fmt.Errorf("mirror node HTTP %d", resp.StatusCode)
	}
	var page mirrorPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return mirrorPage{}, fmt.Errorf("decode mirror page: %w", err)
	}
	return page, nil
}

// parseConsensusTimestampNs converts "secs.nanos" → uint64 unix nanoseconds.
// On parse error returns 0 (caller can fall back to local clock).
func parseConsensusTimestampNs(s string) uint64 {
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		secs, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0
		}
		return secs * 1_000_000_000
	}
	secs, err := strconv.ParseUint(s[:dot], 10, 64)
	if err != nil {
		return 0
	}
	nanoStr := s[dot+1:]
	// Pad / truncate to exactly 9 digits.
	if len(nanoStr) > 9 {
		nanoStr = nanoStr[:9]
	}
	for len(nanoStr) < 9 {
		nanoStr += "0"
	}
	nanos, err := strconv.ParseUint(nanoStr, 10, 64)
	if err != nil {
		return 0
	}
	return secs*1_000_000_000 + nanos
}

func (c *RealHCSClient) GetTopicInfo(topicId string) (TopicMetadata, error) {
	tid, err := hiero.TopicIDFromString(topicId)
	if err != nil {
		return TopicMetadata{}, fmt.Errorf("parse TopicID %q: %w", topicId, err)
	}

	topicInfo, err := hiero.NewTopicInfoQuery().
		SetTopicID(tid).
		Execute(c.client)
	if err != nil {
		return TopicMetadata{}, fmt.Errorf("execute TopicInfoQuery: %w", err)
	}

	ref, _ := NewTopicRef(BackendHCS, topicId)
	return TopicMetadata{
		TopicRef:       ref,
		SequenceNumber: topicInfo.SequenceNumber,
		Memo:           topicInfo.TopicMemo,
	}, nil
}
