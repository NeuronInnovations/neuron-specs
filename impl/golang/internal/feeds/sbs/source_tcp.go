package sbs

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"time"
)

// RunBaseStationTCP dials hostPort, reads SBS-1 BaseStation CSV records line
// by line, and emits SBSTrack values on out until ctx is cancelled.
//
// Records that fail to parse (wrong type, malformed, etc.) are silently
// dropped — SBS-1 streams routinely include lines this v1 parser doesn't
// support (MSG types 1/4/5/6/7/8), and an upstream decoder restart can
// produce a few partial lines that are best ignored. The connection is
// reopened with 1 s backoff on any I/O error or EOF.
//
// hostPort uses standard Go "host:port" notation. For the JetVision
// Air!Squitter SBS-1 port use "127.0.0.1:30003"; for the BlueMark DroneScout
// MQTT subscriber's SBS export use the value of config.py's
// sbs_server_port (default 30003).
//
// Read-only contract: this function MAKES NO WRITES to the upstream — SBS-1
// is a one-way text feed by design. Per the implementation-slice constraint,
// dialing the JV box's port 30003 from the dev host requires no JV-side
// modification or restart.
//
// On context cancel, returns ctx.Err(). On the persistent net.OpError that
// would indicate the host:port is invalid (e.g., DNS lookup failure), the
// loop continues retrying with backoff — callers needing a hard fail-fast
// on bad config should validate hostPort before calling.
//
// The function does NOT close out — that is the caller's responsibility
// (matches internal/feeds.RunBeastTCP convention).
func RunBaseStationTCP(ctx context.Context, hostPort string, out chan<- SBSTrack) error {
	if hostPort == "" {
		return errors.New("sbs: RunBaseStationTCP requires a host:port")
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
				continue
			}
		}

		// Encourage early detection of stuck connections.
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(1 * time.Second)
		}

		err = readSBSLines(ctx, conn, out)
		_ = conn.Close()

		if err == nil {
			// Clean EOF — reconnect.
			continue
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

// readSBSLines reads newline-delimited SBS-1 records from r, parses each,
// and emits successful parses on out. Exits when r returns EOF or ctx is
// cancelled.
func readSBSLines(ctx context.Context, r io.Reader, out chan<- SBSTrack) error {
	scanner := bufio.NewScanner(r)
	// SBS-1 lines are typically < 256 bytes vanilla, < 1 KB with full
	// mlat-server rcv_users. 64 KiB is comfortably safe.
	scanner.Buffer(make([]byte, 0, 4096), 64*1024)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		t, err := ParseMSG(line)
		if err != nil {
			// Drop lines we don't parse (wrong type, malformed, etc.).
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- *t:
		}
	}
	return scanner.Err()
}
