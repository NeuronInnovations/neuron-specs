package sapient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// ReadBridgeFeed dials a SAPIENT-edge feed at addr and streams decoded
// SapientMessages until ctx is cancelled or the feed closes. It is the reader for
// BOTH local SAPIENT edges: the seller dials the neuron-rid-bridge
// (--sapient-listen, --sapient-format protobuf), and the FID consumer dials the
// Buyer Proxy's edge (FeedServer).
//
// Each message is a 4-byte LITTLE-endian length-prefixed protobuf SapientMessage
// — the FR-S91 conformant SAPIENT-edge framing (ICD v7 §2.1.2; see
// edge_framing.go), NOT the big-endian Neuron p2p lane framing. The data channel
// is closed when reading stops; a non-clean error (dial fault, truncated/oversize
// frame) is delivered once on the returned error channel (a clean EOF is not an
// error).
//
// The bridge stamps a placeholder node_id; the seller re-stamps node_id with its
// Neuron identity before publishing (the runtime boundary — see NodeIDFromIdentity).
// DiscardUnknown keeps the reader forward-compatible across DSTL BSI Flex 335
// minor revisions.
func ReadBridgeFeed(ctx context.Context, addr string) (<-chan *sapientpb.SapientMessage, <-chan error) {
	out := make(chan *sapientpb.SapientMessage, 16)
	errc := make(chan error, 1)

	go func() {
		defer close(out)

		var dialer net.Dialer
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			errc <- fmt.Errorf("sapient.ReadBridgeFeed: dial %s: %w", addr, err)
			return
		}
		defer conn.Close()

		// Close the conn on cancel so a blocking read returns promptly.
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			select {
			case <-ctx.Done():
				_ = conn.Close()
			case <-stop:
			}
		}()

		um := proto.UnmarshalOptions{DiscardUnknown: true}
		for {
			data, rerr := readLEFrame(conn)
			if rerr != nil {
				// A clean EOF (feed closed) or a cancel-induced close are not errors.
				if !errors.Is(rerr, io.EOF) && ctx.Err() == nil {
					errc <- fmt.Errorf("sapient.ReadBridgeFeed: read %s: %w", addr, rerr)
				}
				return
			}
			msg := &sapientpb.SapientMessage{}
			if err := um.Unmarshal(data, msg); err != nil {
				// Tolerate a single malformed frame; keep the feed alive.
				continue
			}
			select {
			case out <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, errc
}
