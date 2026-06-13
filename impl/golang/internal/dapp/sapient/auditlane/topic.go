package auditlane

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// laneMagic prefixes every SAPIENT audit-lane payload so a stdOut topic that is
// SHARED with the spec-005 liveness heartbeat is unambiguously demultiplexed: a
// 005 heartbeat payload is canonical JSON (begins '{'); a SAPIENT control message
// is laneMagic followed by the binary protobuf SapientMessage. TopicLane.Subscribe
// skips anything lacking the prefix, and the health observer's V-OBS-02
// (type=="heartbeat") already rejects these — so the two coexist on the one stdOut
// TopicRef the registry advertises (015 FR-S12 + 005), with neither parsing the
// other's bytes.
var laneMagic = []byte("SPNT")

// TopicLane is a 004/topic-backed auditlane.Lane: it carries whole SapientMessages
// as SIGNED TopicMessages (015 FR-S12 / FR-S32 — full-message anchoring on the
// ledger, not hash-only) over any topic.TopicAdapter (memory in tests, HCS in
// production). It is the DLT-backed sibling of FileLane/MemoryLane; the
// tasking.Manager is backend-agnostic and drives any of the three unchanged.
//
// Concurrency: Publish is safe from multiple goroutines (the seller publishes the
// heartbeat and the StatusReport/TaskAck/Registration stream on the same stdOut).
// Subscribe spawns one reader goroutine per call; Close cancels them all.
type TopicLane struct {
	adapter topic.TopicAdapter
	key     *keylib.NeuronPrivateKey
	refs    map[Role]topic.TopicRef
	now     func() time.Time
	seq     atomic.Uint64

	mu      sync.Mutex
	cancels []context.CancelFunc
	closed  bool
}

// NewTopicLane builds a TopicLane that signs with key and routes each Role to the
// matching seller topic in refs (stdIn/stdOut/stdErr). refs MUST contain every
// role the caller publishes or subscribes on; an unmapped role is reported at use.
// The adapter is owned by the caller — Close does NOT close it.
func NewTopicLane(adapter topic.TopicAdapter, key *keylib.NeuronPrivateKey, refs map[Role]topic.TopicRef) (*TopicLane, error) {
	if adapter == nil {
		return nil, errors.New("auditlane.NewTopicLane: adapter required")
	}
	if key == nil {
		return nil, errors.New("auditlane.NewTopicLane: key required")
	}
	cp := make(map[Role]topic.TopicRef, len(refs))
	maps.Copy(cp, refs)
	return &TopicLane{adapter: adapter, key: key, refs: cp, now: time.Now}, nil
}

func (l *TopicLane) ref(role Role) (topic.TopicRef, error) {
	ref, ok := l.refs[role]
	if !ok {
		return topic.TopicRef{}, fmt.Errorf("auditlane.TopicLane: no topic ref for role %q", role)
	}
	return ref, nil
}

// Publish signs msg into a TopicMessage (laneMagic + protobuf payload) and submits
// it to the role's topic fire-and-forget; the ledger assigns consensus ordering.
func (l *TopicLane) Publish(_ context.Context, ch Channel, msg *sapientpb.SapientMessage) error {
	ref, err := l.ref(ch.Role)
	if err != nil {
		return err
	}
	body, err := proto.Marshal(msg)
	if err != nil {
		return Wrap(ErrEncode, "Publish", err)
	}
	payload := make([]byte, 0, len(laneMagic)+len(body))
	payload = append(payload, laneMagic...)
	payload = append(payload, body...)

	ts := uint64(l.now().UnixNano())
	seq := l.seq.Add(1)
	tm, err := topic.NewTopicMessage(l.key, ts, seq, payload)
	if err != nil {
		return Wrap(ErrEncode, "Publish", err)
	}
	if _, err := l.adapter.Publish(ref, tm, topic.PublishOpts{ConfirmationMode: topic.FireAndForget}); err != nil {
		return Wrap(ErrIO, "Publish", err)
	}
	return nil
}

// Subscribe streams SapientMessages published to the role's topic. It replays the
// topic from the start (full audit, matching FileLane), verifies each TopicMessage
// signature, demultiplexes on laneMagic (skipping heartbeats and any foreign
// payload), and emits the decoded SapientMessage. The stream closes when ctx is
// cancelled or the lane is closed.
func (l *TopicLane) Subscribe(ctx context.Context, ch Channel) (<-chan *sapientpb.SapientMessage, error) {
	ref, err := l.ref(ch.Role)
	if err != nil {
		return nil, err
	}
	subCtx, cancel := context.WithCancel(ctx)
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		cancel()
		return nil, New(ErrLaneClosed, "Subscribe", "lane is closed")
	}
	l.cancels = append(l.cancels, cancel)
	l.mu.Unlock()

	// Replay from the first message so a late subscriber still sees the full
	// audit history (the memory adapter and HCS both backfill from sequence 0).
	from := uint64(0)
	deliveries, err := l.adapter.Subscribe(subCtx, ref, topic.SubscribeOpts{FromSequence: &from})
	if err != nil {
		cancel()
		return nil, Wrap(ErrIO, "Subscribe", err)
	}

	out := make(chan *sapientpb.SapientMessage, 256)
	go func() {
		defer close(out)
		for {
			select {
			case <-subCtx.Done():
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				msg, keep := decodeLaneDelivery(d)
				if !keep {
					continue
				}
				select {
				case out <- msg:
				case <-subCtx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

// decodeLaneDelivery validates the TopicMessage signature, requires the laneMagic
// prefix, and decodes the SapientMessage. Returns (nil,false) for anything that is
// not a well-formed signed SAPIENT lane payload (heartbeats, foreign bytes, bad
// signatures) so the caller silently skips it.
func decodeLaneDelivery(d topic.MessageDelivery) (*sapientpb.SapientMessage, bool) {
	if err := topic.ValidateTopicMessage(d.Message); err != nil {
		return nil, false
	}
	payload := d.Message.Payload()
	if !bytes.HasPrefix(payload, laneMagic) {
		return nil, false // a heartbeat (JSON) or foreign payload sharing the topic
	}
	var msg sapientpb.SapientMessage
	if err := proto.Unmarshal(payload[len(laneMagic):], &msg); err != nil {
		return nil, false
	}
	return &msg, true
}

// Close cancels every live subscription. Idempotent. The underlying adapter is
// owned by the caller and is NOT closed here.
func (l *TopicLane) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	for _, c := range l.cancels {
		c()
	}
	l.cancels = nil
	return nil
}
