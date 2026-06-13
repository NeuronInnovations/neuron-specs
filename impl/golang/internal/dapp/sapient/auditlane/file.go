package auditlane

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// filePollInterval is how often a FileLane subscriber tails for appended lines.
const filePollInterval = 100 * time.Millisecond

// fileRecord is one NDJSON line in the audit file: the channel coordinates plus
// the whole SapientMessage as protojson (full-message anchoring, CL-3/FR-S32, and
// human-auditable — you can `cat` the file).
type fileRecord struct {
	TS        string          `json:"ts"`
	ASMNodeID string          `json:"asmNodeId"`
	Role      string          `json:"role"`
	Msg       json.RawMessage `json:"msg"`
}

// FileLane is an append-only NDJSON Lane backing the cross-process demo: a seller
// process and a tasking CLI share one audit file. Subscribers replay the whole
// file from the start (full audit) and then tail it for new lines.
type FileLane struct {
	path     string
	now      func() time.Time
	done     chan struct{}
	closeOne sync.Once

	mu sync.Mutex
	w  *os.File
}

// NewFileLane returns a FileLane backed by path (created on first Publish).
func NewFileLane(path string) *FileLane {
	return &FileLane{path: path, now: time.Now, done: make(chan struct{})}
}

// Publish appends one NDJSON record for msg on ch. Appends are O_APPEND so
// multiple processes may share the file.
func (l *FileLane) Publish(_ context.Context, ch Channel, msg *sapientpb.SapientMessage) error {
	raw, err := protojson.Marshal(msg)
	if err != nil {
		return Wrap(ErrEncode, "Publish", err)
	}
	rec := fileRecord{
		TS:        l.now().UTC().Format(time.RFC3339Nano),
		ASMNodeID: ch.ASMNodeID,
		Role:      string(ch.Role),
		Msg:       json.RawMessage(raw),
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return Wrap(ErrEncode, "Publish", err)
	}
	line = append(line, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.w == nil {
		f, oerr := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if oerr != nil {
			return Wrap(ErrIO, "Publish", oerr)
		}
		l.w = f
	}
	if _, werr := l.w.Write(line); werr != nil {
		return Wrap(ErrIO, "Publish", werr)
	}
	return nil
}

// Subscribe replays every matching record from the start of the file, then tails
// for new lines until ctx is cancelled.
func (l *FileLane) Subscribe(ctx context.Context, ch Channel) (<-chan *sapientpb.SapientMessage, error) {
	out := make(chan *sapientpb.SapientMessage, 256)
	go func() {
		defer close(out)
		um := protojson.UnmarshalOptions{DiscardUnknown: true}
		var offset int64
		ticker := time.NewTicker(filePollInterval)
		defer ticker.Stop()
		for {
			offset = l.drain(ctx, ch, offset, um, out)
			select {
			case <-ctx.Done():
				return
			case <-l.done: // lane Close() stops subscribers even without ctx cancel
				return
			case <-ticker.C:
			}
		}
	}()
	return out, nil
}

// drain reads complete newline-terminated lines from offset, emits the ones
// matching ch, and returns the new offset (only advanced past full lines).
func (l *FileLane) drain(ctx context.Context, ch Channel, offset int64, um protojson.UnmarshalOptions, out chan<- *sapientpb.SapientMessage) int64 {
	f, err := os.Open(l.path)
	if err != nil {
		return offset
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset
	}
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			// Partial trailing line (no '\n' yet) — do not advance past it.
			return offset
		}
		offset += int64(len(line))
		var rec fileRecord
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		if rec.ASMNodeID != ch.ASMNodeID || rec.Role != string(ch.Role) {
			continue
		}
		var msg sapientpb.SapientMessage
		if um.Unmarshal(rec.Msg, &msg) != nil {
			continue
		}
		select {
		case out <- &msg:
		case <-ctx.Done():
			return offset
		case <-l.done:
			return offset
		}
	}
}

// Close stops all subscriber goroutines (via the done channel) and closes the
// append writer. Idempotent.
func (l *FileLane) Close() error {
	l.closeOne.Do(func() { close(l.done) })
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.w == nil {
		return nil
	}
	err := l.w.Close()
	l.w = nil
	return err
}
