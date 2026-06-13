// Command sapient-feed-replay serves a captured SAPIENT NDJSON fixture as a
// live LE-framed protobuf feed — a stand-in for a real bridge
// (neuron-rid-bridge / neuron-jv-bridge --sapient-listen) in local
// multi-source demos and smoke tests. It reads protojson SapientMessages from
// --fixture, then serves every connected TCP client the FR-S91 SAPIENT edge
// (4-byte little-endian length-prefixed BSI Flex 335 v2.0 protobuf), one
// message per --interval, optionally looping.
//
//	sapient-feed-replay --listen 127.0.0.1:29999 --fixture testdata/bridge-sample.ndjson --interval 200ms --loop
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		log.Fatalf("sapient-feed-replay: %v", err)
	}
}

func run(ctx context.Context, args []string, stdout *os.File) error {
	fs := flag.NewFlagSet("sapient-feed-replay", flag.ContinueOnError)
	var (
		listen   = fs.String("listen", "127.0.0.1:0", "TCP address to serve the LE-framed SAPIENT feed (a bridge --sapient-listen stand-in)")
		fixture  = fs.String("fixture", "", "protojson NDJSON fixture of SapientMessages (one per line) [required]")
		interval = fs.Duration("interval", 200*time.Millisecond, "delay between replayed messages")
		loop     = fs.Bool("loop", false, "replay the fixture forever (until Ctrl-C)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *fixture == "" {
		return fmt.Errorf("--fixture is required")
	}
	logger := log.New(os.Stderr, "[sapient-feed-replay] ", log.LstdFlags)

	msgs, err := loadFixture(*fixture)
	if err != nil {
		return err
	}

	srv, err := sapient.ServeFeed(*listen)
	if err != nil {
		return err
	}
	defer srv.Close()
	// The bound address on stdout so scripts can read it (pass --listen :0).
	fmt.Fprintln(stdout, srv.Addr())
	logger.Printf("serving %d fixture messages on %s (interval=%s loop=%v)", len(msgs), srv.Addr(), *interval, *loop)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	i, published := 0, 0
	for {
		select {
		case <-ctx.Done():
			logger.Printf("done; published %d messages", published)
			return nil
		case <-ticker.C:
			if i >= len(msgs) {
				if !*loop {
					logger.Printf("fixture exhausted; published %d messages (serving idle until Ctrl-C)", published)
					<-ctx.Done()
					return nil
				}
				i = 0
			}
			if perr := srv.Publish(msgs[i]); perr != nil {
				logger.Printf("publish error: %v", perr)
			}
			i++
			published++
		}
	}
}

// loadFixture parses a protojson NDJSON file of SapientMessages.
func loadFixture(path string) ([]*sapientpb.SapientMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixture: %w", err)
	}
	defer f.Close()
	var msgs []*sapientpb.SapientMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), sapient.MaxEdgeFrameSize)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		m := &sapientpb.SapientMessage{}
		if uerr := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(sc.Bytes(), m); uerr != nil {
			return nil, fmt.Errorf("parse fixture line %d: %w", len(msgs)+1, uerr)
		}
		msgs = append(msgs, m)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("fixture %s carries no messages", path)
	}
	return msgs, nil
}
