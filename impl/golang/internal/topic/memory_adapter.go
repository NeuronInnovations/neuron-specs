package topic

import (
	"context"
	"fmt"
	"sync"
)

// MemoryTopicAdapter is an in-process TopicAdapter suitable for tests,
// local demos, and offline conformance runs. It exposes the same surface
// as a real backend (CreateTopic / Publish / Subscribe / Resolve) but
// keeps every topic + message in a process-local map.
//
// Concurrency: every method is safe for concurrent use. Subscribers receive
// messages on a buffered channel (capacity 100 by default). If a subscriber
// stops draining the channel, Publish blocks; tests that need to verify
// back-pressure SHOULD pump the channel in a goroutine. Context cancellation
// on the Subscribe ctx detaches the subscriber and closes its channel —
// callers MUST NOT continue to read after the context is cancelled.
//
// The locator-to-topic mapping is keyed by `TopicRef.locator`. Two
// MemoryTopicAdapter instances do NOT share state — instantiate one per
// test (or per process) and pass it to every actor that needs to talk
// over the bus.
type MemoryTopicAdapter struct {
	mu      sync.Mutex
	topics  map[string]struct{}
	subs    map[string][]chan MessageDelivery
	seqNums map[string]uint64
	// history retains every message published per topic so Subscribe can
	// honour SubscribeOpts.FromSequence (matches HCS semantics). When
	// FromSequence is nil the subscriber starts at "next message", same
	// as before. When non-nil the subscriber receives every retained
	// message with seq >= *FromSequence, in order, then live messages.
	history map[string][]MessageDelivery
	// subBufferSize is the per-subscriber channel capacity. Defaults to
	// 100 if NewMemoryTopicAdapter is used; SetSubscriberBuffer overrides
	// for tests that need a different size.
	subBufferSize int
}

// NewMemoryTopicAdapter returns an empty in-memory adapter.
func NewMemoryTopicAdapter() *MemoryTopicAdapter {
	return &MemoryTopicAdapter{
		topics:        make(map[string]struct{}),
		subs:          make(map[string][]chan MessageDelivery),
		seqNums:       make(map[string]uint64),
		history:       make(map[string][]MessageDelivery),
		subBufferSize: 100,
	}
}

// SetSubscriberBuffer overrides the per-subscriber channel capacity.
// Affects subscribers created AFTER the call. Tests that exercise
// back-pressure use 1; the default (100) is fine for most uses.
func (a *MemoryTopicAdapter) SetSubscriberBuffer(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n < 1 {
		n = 1
	}
	a.subBufferSize = n
}

// Compile-time assertion: MemoryTopicAdapter is a TopicAdapter.
var _ TopicAdapter = (*MemoryTopicAdapter)(nil)

// CreateTopic registers a new topic on the adapter. The Memo field is
// used as the topic locator.
func (a *MemoryTopicAdapter) CreateTopic(opts CreateTopicOpts) (TopicRef, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	name := opts.Memo
	if name == "" {
		return TopicRef{}, NewTopicError(ErrInvalidConfig, "topic name (Memo) required")
	}
	a.topics[name] = struct{}{}
	return NewTopicRef(BackendKindMemory, name)
}

// Publish delivers a signed TopicMessage to every active subscriber on
// the given topic ref. Returns an error if the topic doesn't exist.
func (a *MemoryTopicAdapter) Publish(ref TopicRef, msg TopicMessage, _ PublishOpts) (PublishResult, error) {
	a.mu.Lock()

	if _, ok := a.topics[ref.locator]; !ok {
		a.mu.Unlock()
		return PublishResult{}, NewTopicError(ErrTopicNotFound, "topic does not exist: "+ref.locator)
	}

	a.seqNums[ref.locator]++
	seq := a.seqNums[ref.locator]
	subs := append([]chan MessageDelivery(nil), a.subs[ref.locator]...)
	delivery := MessageDelivery{
		Message:            msg,
		ConsensusTimestamp: msg.timestamp,
		BackendSequence:    seq,
	}
	a.history[ref.locator] = append(a.history[ref.locator], delivery)
	a.mu.Unlock()

	for _, ch := range subs {
		ch <- delivery
	}

	return PublishResult{
		TransactionRef: fmt.Sprintf("inmemory-tx-%d", seq),
		Confirmed:      true,
		SequenceNumber: &seq,
	}, nil
}

// Subscribe opens a subscription channel for the given topic. The channel
// is closed when the ctx is cancelled.
//
// When SubscribeOpts.FromSequence is non-nil the subscriber receives every
// historical message with seq >= *FromSequence (in order) before live
// messages. With FromSequence=&0 this becomes a complete replay from the
// start of the topic — matches HCS behaviour and lets late-subscribing
// orchestrators (e.g. the seller's per-request goroutine) catch up on
// messages already published by the buyer.
//
// When FromSequence is nil the subscriber sees only messages published
// AFTER subscription (the historic behaviour).
func (a *MemoryTopicAdapter) Subscribe(ctx context.Context, ref TopicRef, opts SubscribeOpts) (<-chan MessageDelivery, error) {
	a.mu.Lock()
	bufSize := a.subBufferSize
	// Pre-load history into the channel buffer when FromSequence is set.
	var backfill []MessageDelivery
	if opts.FromSequence != nil {
		from := *opts.FromSequence
		for _, d := range a.history[ref.locator] {
			if d.BackendSequence >= from {
				backfill = append(backfill, d)
			}
		}
	}
	// Channel capacity must accommodate the entire backfill so the loop
	// below can push every retained message without blocking.
	bufNeeded := bufSize
	if len(backfill) > bufNeeded {
		bufNeeded = len(backfill) + bufSize
	}
	ch := make(chan MessageDelivery, bufNeeded)
	for _, d := range backfill {
		ch <- d
	}
	a.subs[ref.locator] = append(a.subs[ref.locator], ch)
	a.mu.Unlock()

	go func() {
		<-ctx.Done()
		a.mu.Lock()
		subs := a.subs[ref.locator]
		filtered := subs[:0]
		for _, s := range subs {
			if s == ch {
				continue
			}
			filtered = append(filtered, s)
		}
		a.subs[ref.locator] = filtered
		a.mu.Unlock()
		close(ch)
	}()

	return ch, nil
}

// Resolve returns metadata for the given topic ref.
func (a *MemoryTopicAdapter) Resolve(ref TopicRef) (TopicMetadata, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.topics[ref.locator]; !ok {
		return TopicMetadata{}, NewTopicError(ErrTopicNotFound, "topic not found")
	}
	return TopicMetadata{
		TopicRef:       ref,
		SequenceNumber: a.seqNums[ref.locator],
	}, nil
}

// MaxMessageSize returns the per-message ceiling. Memory bus is generous
// (64 KiB) to keep tests free of size-trim hassle; real backends impose
// their own limits per spec 004.
func (a *MemoryTopicAdapter) MaxMessageSize() uint64 { return 65536 }

// EstimatePublishCost is a no-op for the memory adapter.
func (a *MemoryTopicAdapter) EstimatePublishCost(_ uint64) (CostEstimate, error) {
	return CostEstimate{Amount: 0, Unit: "none"}, nil
}

// SupportedTransport returns the synthetic transport kind used by the
// memory adapter. Distinct from BackendHCS / BackendKafka so the
// adapter registry can hold multiple at once.
func (a *MemoryTopicAdapter) SupportedTransport() BackendKind {
	return BackendKindMemory
}

// BackendKindMemory is the synthetic transport kind used by
// MemoryTopicAdapter. Out-of-spec compared to the canonical
// HCS/Kafka/ERC trio; reserved for tests + local demo runs.
const BackendKindMemory BackendKind = "custom:inmemory"
