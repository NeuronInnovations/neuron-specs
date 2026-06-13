package sbs

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"time"
)

// RunRIDBaseStationTCP dials hostPort, reads SBS-1 BaseStation CSV
// records line by line, and emits RIDSBSRecord values on out until ctx
// is cancelled.
//
// This is the Remote-ID dialect sibling of RunBaseStationTCP. The two
// differ only in their per-line parser: RunRIDBaseStationTCP routes
// through ParseRIDMSG (accepts MSG types 1, 2, 3, 4 — drone + operator
// records); RunBaseStationTCP routes through ParseMSG (strict MSG,3
// only, as the JV ADS-B seller requires).
//
// Records that fail to parse (wrong type, malformed, etc.) are
// silently dropped — same convention as RunBaseStationTCP. The
// connection is reopened with 1 s backoff on any I/O error or EOF.
//
// hostPort uses standard Go "host:port" notation. For the BlueMark
// neuron-rid-bridge running on loopback use "127.0.0.1:30003".
//
// Read-only contract: this function MAKES NO WRITES to the upstream —
// the SBS feed is a one-way text stream by design.
//
// On context cancel, returns ctx.Err(). On persistent dial failure
// the loop retries with backoff — callers needing fail-fast on bad
// config should validate hostPort before calling.
//
// The function does NOT close out — that is the caller's
// responsibility (matches RunBaseStationTCP convention).
func RunRIDBaseStationTCP(ctx context.Context, hostPort string, out chan<- RIDSBSRecord) error {
	if hostPort == "" {
		return errors.New("sbs: RunRIDBaseStationTCP requires a host:port")
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

		err = readRIDSBSLines(ctx, conn, out)
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

// readRIDSBSLines reads newline-delimited SBS-1 records from r, parses
// each, and emits successful parses on out. Exits when r returns EOF or
// ctx is cancelled.
func readRIDSBSLines(ctx context.Context, r io.Reader, out chan<- RIDSBSRecord) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 4096), 64*1024)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		rec, err := ParseRIDMSG(line)
		if err != nil {
			// Drop lines we don't parse (wrong type, malformed, etc.).
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- *rec:
		}
	}
	return scanner.Err()
}
