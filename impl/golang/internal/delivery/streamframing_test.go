package delivery

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAdapter is a memory-only DeliveryAdapter for testing SendStream/ReceiveStream
// without standing up a real libp2p host. Send pushes onto an internal queue;
// Receive pops from it and blocks until something arrives. Disconnect closes
// the queue, causing pending Receive to return io.EOF.
type fakeAdapter struct {
	mu     sync.Mutex
	cond   *sync.Cond
	queue  [][]byte
	closed bool
	failOn []error // each non-nil entry causes the i-th Send/Receive call to return that error
	op     int
}

func newFakeAdapter() *fakeAdapter {
	a := &fakeAdapter{}
	a.cond = sync.NewCond(&a.mu)
	return a
}

func (a *fakeAdapter) injectError(at int, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for len(a.failOn) <= at {
		a.failOn = append(a.failOn, nil)
	}
	a.failOn[at] = err
}

func (a *fakeAdapter) Connect(_ string, _ []string, _ string, _ *ConnectOptions) (*DeliveryChannel, error) {
	return &DeliveryChannel{ID: "fake-1"}, nil
}

func (a *fakeAdapter) Send(_ *DeliveryChannel, data []byte) (*SendResult, error) {
	a.mu.Lock()
	idx := a.op
	a.op++
	if idx < len(a.failOn) && a.failOn[idx] != nil {
		err := a.failOn[idx]
		a.mu.Unlock()
		return nil, err
	}
	if a.closed {
		a.mu.Unlock()
		return nil, NewDeliveryError(ErrChannelClosed, "Send", "queue closed")
	}
	cp := append([]byte(nil), data...)
	a.queue = append(a.queue, cp)
	a.cond.Signal()
	a.mu.Unlock()
	return &SendResult{BytesSent: len(data)}, nil
}

func (a *fakeAdapter) Receive(_ *DeliveryChannel) (*DataFrame, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for len(a.queue) == 0 && !a.closed {
		a.cond.Wait()
	}
	if len(a.queue) > 0 {
		out := a.queue[0]
		a.queue = a.queue[1:]
		return &DataFrame{Data: out, ReceivedAt: time.Now()}, nil
	}
	// Closed and empty — surface as wrapped EOF, exactly as the libp2p adapter does.
	return nil, WrapDeliveryError(ErrStreamError, "Receive", io.EOF)
}

func (a *fakeAdapter) Disconnect(_ *DeliveryChannel) error {
	a.mu.Lock()
	a.closed = true
	a.cond.Broadcast()
	a.mu.Unlock()
	return nil
}

func (a *fakeAdapter) GetStatus(_ *DeliveryChannel) ChannelStatus {
	return ChannelStatus{State: StateConnected}
}

func TestSendStream_RoundTrip(t *testing.T) {
	a := newFakeAdapter()
	ch, _ := a.Connect("p", nil, "/x", nil)

	in := make(chan []byte, 4)
	in <- []byte("alpha")
	in <- []byte("beta")
	in <- []byte("gamma")
	close(in)

	require.NoError(t, SendStream(a, ch, context.Background(), in))

	a.mu.Lock()
	defer a.mu.Unlock()
	assert.Equal(t, [][]byte{[]byte("alpha"), []byte("beta"), []byte("gamma")}, a.queue)
}

func TestSendStream_ContextCancel(t *testing.T) {
	a := newFakeAdapter()
	ch, _ := a.Connect("p", nil, "/x", nil)

	in := make(chan []byte) // never written
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- SendStream(a, ch, ctx, in) }()

	cancel()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("SendStream did not return after cancel")
	}
}

func TestSendStream_StreamClosedReturnsNil(t *testing.T) {
	a := newFakeAdapter()
	ch, _ := a.Connect("p", nil, "/x", nil)

	a.injectError(0, NewDeliveryError(ErrChannelClosed, "Send", "stream closed by peer"))

	in := make(chan []byte, 1)
	in <- []byte("foo")
	close(in)

	err := SendStream(a, ch, context.Background(), in)
	assert.NoError(t, err, "stream-closed error should map to nil for graceful exit")
}

func TestSendStream_OtherErrorPropagates(t *testing.T) {
	a := newFakeAdapter()
	ch, _ := a.Connect("p", nil, "/x", nil)

	a.injectError(0, errors.New("connection reset by peer"))

	in := make(chan []byte, 1)
	in <- []byte("foo")
	close(in)

	err := SendStream(a, ch, context.Background(), in)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection reset")
}

func TestReceiveStream_RoundTrip(t *testing.T) {
	a := newFakeAdapter()
	ch, _ := a.Connect("p", nil, "/x", nil)

	// Pre-load the queue.
	a.mu.Lock()
	a.queue = [][]byte{[]byte("one"), []byte("two"), []byte("three")}
	a.mu.Unlock()

	out := make(chan []byte, 4)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- ReceiveStream(a, ch, ctx, out) }()

	// Drain the three frames.
	for i, want := range []string{"one", "two", "three"} {
		select {
		case got := <-out:
			assert.Equal(t, want, string(got), "frame %d", i)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for frame %d", i)
		}
	}

	// Closing the queue (graceful) should cause ReceiveStream to return nil.
	require.NoError(t, a.Disconnect(ch))

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("ReceiveStream did not return after Disconnect")
	}
	cancel()
}

func TestReceiveStream_ContextCancel(t *testing.T) {
	a := newFakeAdapter()
	ch, _ := a.Connect("p", nil, "/x", nil)

	out := make(chan []byte, 1)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- ReceiveStream(a, ch, ctx, out) }()

	cancel()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("ReceiveStream did not return after cancel")
	}
}

func TestSendThenReceive_EndToEnd(t *testing.T) {
	// Two fake adapters bridged via a small relay goroutine — proves
	// SendStream and ReceiveStream interoperate when both run concurrently.
	a := newFakeAdapter()
	ch, _ := a.Connect("p", nil, "/x", nil)

	in := make(chan []byte, 8)
	out := make(chan []byte, 8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = SendStream(a, ch, ctx, in) }()
	go func() { _ = ReceiveStream(a, ch, ctx, out) }()

	for i := 0; i < 5; i++ {
		in <- []byte{byte(i)}
	}

	for i := 0; i < 5; i++ {
		select {
		case got := <-out:
			assert.Equal(t, byte(i), got[0])
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for frame %d", i)
		}
	}
	close(in)
	require.NoError(t, a.Disconnect(ch))
}
