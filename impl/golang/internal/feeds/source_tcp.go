package feeds

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

// RunBeastTCP dials hostPort, reads Beast frames, and emits FeedFrames on out
// until ctx is cancelled. Mode-S short and long frames are forwarded; Mode-AC
// and status frames are dropped.
//
// On I/O error or EOF, the connection is reopened with 1 s backoff. The loop
// exits cleanly only when ctx is cancelled.
//
// hostPort uses standard Go "host:port" notation, e.g. "127.0.0.1:10003" for
// JetVision Air!Squitter, or "localhost:30005" for dump1090's Beast output.
func RunBeastTCP(ctx context.Context, hostPort string, out chan<- FeedFrame) error {
	if hostPort == "" {
		return errors.New("feeds: RunBeastTCP requires a host:port")
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		dialer := net.Dialer{Timeout: 5 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", hostPort)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			// Surface the dial error to the caller via a brief sleep + retry.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
				continue
			}
		}

		// Encourage early detection of stuck connections on slow links.
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(1 * time.Second)
		}

		err = readFrames(ctx, conn, out)
		_ = conn.Close()

		if err == nil {
			// Reader returned io.EOF cleanly — peer closed. Reconnect.
			continue
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			continue
		}
		// Any other error: brief backoff and reconnect.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}
