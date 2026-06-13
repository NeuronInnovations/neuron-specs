package edgeapp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// MemoryBus is a Subscribe-capable in-memory topic.TopicAdapter.
//
// It is functionally equivalent to the buyer-seller-demo's memoryTopicBus,
// but extends Subscribe so RunSeller and RunBuyer (which observe their stdIn
// via Subscribe) can use the same code path under both mock and HCS modes.
//
// MemoryBus is safe for concurrent use.
type MemoryBus struct {
	mu      sync.Mutex
	topics  map[string][]topic.TopicMessage
	nextSeq map[string]uint64
	subs    map[string][]chan topic.MessageDelivery
}

// NewMemoryBus creates an empty MemoryBus ready for CreateTopic / Publish /
// Subscribe.
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{
		topics:  make(map[string][]topic.TopicMessage),
		nextSeq: make(map[string]uint64),
		subs:    make(map[string][]chan topic.MessageDelivery),
	}
}

// CreateTopic returns a new TopicRef whose locator is unique within this bus.
func (b *MemoryBus) CreateTopic(_ topic.CreateTopicOpts) (topic.TopicRef, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	locator := fmt.Sprintf("mem-topic-%d", len(b.topics)+1)
	b.topics[locator] = nil
	b.nextSeq[locator] = 0
	return topic.NewTopicRef(topic.BackendHCS, locator)
}

// Publish appends the message to the topic's log and fans it out to active
// subscribers (non-blocking; a slow subscriber's delivery is dropped).
func (b *MemoryBus) Publish(ref topic.TopicRef, msg topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
	b.mu.Lock()
	loc := ref.Locator()
	b.topics[loc] = append(b.topics[loc], msg)
	b.nextSeq[loc]++
	seq := b.nextSeq[loc]
	ts := uint64(time.Now().UnixNano())

	// Snapshot the subscriber list while holding the lock, fan out unlocked.
	subs := append([]chan topic.MessageDelivery(nil), b.subs[loc]...)
	b.mu.Unlock()

	delivery := topic.MessageDelivery{
		Message:            msg,
		ConsensusTimestamp: ts,
		BackendSequence:    seq,
	}
	for _, ch := range subs {
		select {
		case ch <- delivery:
		default:
			// Subscriber buffer full; drop. A real bus has flow control;
			// for tests this is sufficient.
		}
	}

	return topic.PublishResult{
		TransactionRef:     fmt.Sprintf("mem-tx-%s-%d", loc, seq),
		ConsensusTimestamp: &ts,
		SequenceNumber:     &seq,
		Confirmed:          true,
	}, nil
}

// Subscribe returns a buffered channel of MessageDelivery values for the
// given topic. The channel is closed when ctx is cancelled.
//
// On Subscribe, any messages already published with sequence number >=
// opts.FromSequence (or all messages if FromSequence is nil) are replayed
// before live messages are delivered.
func (b *MemoryBus) Subscribe(ctx context.Context, ref topic.TopicRef, opts topic.SubscribeOpts) (<-chan topic.MessageDelivery, error) {
	out := make(chan topic.MessageDelivery, 64)

	b.mu.Lock()
	loc := ref.Locator()

	fromSeq := uint64(0)
	if opts.FromSequence != nil {
		fromSeq = *opts.FromSequence
	}
	var replays []topic.MessageDelivery
	for i, msg := range b.topics[loc] {
		seq := uint64(i + 1)
		if seq < fromSeq {
			continue
		}
		replays = append(replays, topic.MessageDelivery{
			Message:         msg,
			BackendSequence: seq,
		})
	}
	b.subs[loc] = append(b.subs[loc], out)
	b.mu.Unlock()

	go func() {
		defer func() {
			b.mu.Lock()
			for i, ch := range b.subs[loc] {
				if ch == out {
					b.subs[loc] = append(b.subs[loc][:i], b.subs[loc][i+1:]...)
					break
				}
			}
			close(out)
			b.mu.Unlock()
		}()

		for _, d := range replays {
			select {
			case <-ctx.Done():
				return
			case out <- d:
			}
		}
		<-ctx.Done()
	}()

	return out, nil
}

// Resolve returns minimal metadata for the given topic.
func (b *MemoryBus) Resolve(ref topic.TopicRef) (topic.TopicMetadata, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return topic.TopicMetadata{
		TopicRef:       ref,
		SequenceNumber: b.nextSeq[ref.Locator()],
	}, nil
}

// MaxMessageSize is generous in mock mode — Demo 1 payloads are well under HCS
// limits, and tests should not be artificially squeezed.
func (b *MemoryBus) MaxMessageSize() uint64                { return 128 * 1024 }
func (b *MemoryBus) SupportedTransport() topic.BackendKind { return topic.BackendHCS }
func (b *MemoryBus) EstimatePublishCost(_ uint64) (topic.CostEstimate, error) {
	return topic.CostEstimate{Amount: 1, Unit: "tinybar"}, nil
}

// GetMessages returns a snapshot of all messages published to the given topic.
// Useful for assertions in tests.
func (b *MemoryBus) GetMessages(ref topic.TopicRef) []topic.TopicMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]topic.TopicMessage(nil), b.topics[ref.Locator()]...)
}

// Compile-time check.
var _ topic.TopicAdapter = (*MemoryBus)(nil)
