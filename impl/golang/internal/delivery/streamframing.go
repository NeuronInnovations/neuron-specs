package delivery

import (
	"context"
	"errors"
	"io"
)

// SendStream forwards length-prefixed records from in to the delivery channel
// via adapter.Send. It returns when:
//   - in is closed (graceful end of stream — return value is nil),
//   - ctx is cancelled (return value is ctx.Err()),
//   - adapter.Send returns an error that is not stream-closed.
//
// SendStream does NOT call Disconnect on the channel. The caller is responsible
// for calling adapter.Disconnect when the producer side has finished and no
// further sends will happen — that is what signals graceful EOF to the receiver
// (its Receive call will return an error that wraps io.EOF).
//
// Backpressure: adapter.Send blocks on the underlying transport's flow control
// (libp2p QUIC buffers); SendStream therefore inherits the transport's
// backpressure semantics — fast producer cannot outrun a slow consumer.
func SendStream(adapter DeliveryAdapter, channel *DeliveryChannel, ctx context.Context, in <-chan []byte) error {
	if adapter == nil {
		return errors.New("delivery.SendStream: nil adapter")
	}
	if channel == nil {
		return errors.New("delivery.SendStream: nil channel")
	}
	if in == nil {
		return errors.New("delivery.SendStream: nil input channel")
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case data, ok := <-in:
			if !ok {
				return nil
			}
			if _, err := adapter.Send(channel, data); err != nil {
				if isStreamClosedErr(err) {
					return nil
				}
				return err
			}
		}
	}
}

// ReceiveStream pulls records from the delivery channel via adapter.Receive
// and emits them on out. It returns when:
//   - the peer closed the stream (return value is nil),
//   - ctx is cancelled (return value is ctx.Err()),
//   - adapter.Receive returns a non-stream-closed error.
//
// ReceiveStream does NOT close out — the caller closes it after this function
// returns, since closing a shared channel from inside a worker is error-prone.
//
// Note on cancellation: the underlying adapter.Receive call blocks on a
// network read with no cancellation hook. To unblock the goroutine on ctx
// cancel, the caller should also call adapter.Disconnect(channel) — that
// closes the underlying stream and unblocks the read with an EOF/error.
func ReceiveStream(adapter DeliveryAdapter, channel *DeliveryChannel, ctx context.Context, out chan<- []byte) error {
	if adapter == nil {
		return errors.New("delivery.ReceiveStream: nil adapter")
	}
	if channel == nil {
		return errors.New("delivery.ReceiveStream: nil channel")
	}
	if out == nil {
		return errors.New("delivery.ReceiveStream: nil output channel")
	}

	type rcv struct {
		data []byte
		err  error
	}

	rcvCh := make(chan rcv, 1)
	go func() {
		for {
			f, err := adapter.Receive(channel)
			if err != nil {
				rcvCh <- rcv{err: err}
				return
			}
			rcvCh <- rcv{data: f.Data}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case r := <-rcvCh:
			if r.err != nil {
				if isStreamClosedErr(r.err) {
					return nil
				}
				return r.err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- r.data:
			}
		}
	}
}

// isStreamClosedErr returns true if err signals graceful or peer-initiated
// end of stream — io.EOF, ErrUnexpectedEOF, or a DeliveryError of kind
// ErrChannelClosed. All such errors should terminate the stream loop without
// being treated as failures.
func isStreamClosedErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var de *DeliveryError
	if errors.As(err, &de) && de.Kind() == ErrChannelClosed {
		return true
	}
	return false
}
