package sapient

import (
	"bytes"
	"fmt"
	"net"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// feedClientBacklog is the per-client frame queue depth. A client whose queue
// overflows (it stopped reading and its kernel buffers are full) is dropped —
// live telemetry never buffers unboundedly, and one stuck consumer must never
// stall the fan-out for the others (the multi-source buyer publishes every
// seller's frames through one FeedServer). Mirrors the bridge feed's backlog.
const feedClientBacklog = 256

// feedClient is one connected consumer: its conn, a buffered frame queue
// drained by a dedicated writer goroutine, and a close latch.
type feedClient struct {
	conn net.Conn
	send chan []byte
	done chan struct{}
	once sync.Once
}

// shut closes the client exactly once: the done latch stops the writer, and
// closing the conn unblocks any in-flight Write/Read.
func (c *feedClient) shut() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

// FeedServer is the SAPIENT-edge SERVER counterpart to ReadBridgeFeed (the
// client). It listens on a TCP address and serves every connected client the
// FR-S91 conformant SAPIENT edge: BSI Flex 335 v2.0 SapientMessage protobufs,
// each framed with a 4-byte little-endian length prefix (ICD v7 §2.1.2; see
// edge_framing.go) — exactly the shape ReadBridgeFeed consumes. The generic
// Buyer Proxy (cmd/sapient-buyer) Publishes each received SapientMessage here;
// the 018/FID display consumer dials in with ReadBridgeFeed. This is the local
// realisation of the FR-S91 Buyer-Proxy↔HLDMM SAPIENT edge — a conformant
// third-party HLDMM connects unmodified.
//
// The framing is little-endian per the SAPIENT connection model, distinct from
// the big-endian Neuron p2p inter-proxy lane (internal/delivery): little-endian
// at the SAPIENT edge, big-endian between the proxies (015 §B).
//
// Semantics are live telemetry: a Publish with no client connected is dropped
// (no buffering). Clients may connect and disconnect freely; the server fans out
// to whoever is currently connected. Publish never blocks on a client: each
// client has a buffered queue drained by its own writer goroutine, and a client
// whose queue overflows is dropped — a slow or dead consumer cannot stall the
// other consumers or the publishing seller sessions.
type FeedServer struct {
	ln     net.Listener
	mu     sync.Mutex
	conns  map[*feedClient]struct{}
	closed bool
	wg     sync.WaitGroup
}

// ServeFeed binds a TCP listener on addr and starts accepting clients. Pass a
// :0 port and read Addr to discover the bound address.
func ServeFeed(addr string) (*FeedServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("sapient.ServeFeed: listen %s: %w", addr, err)
	}
	s := &FeedServer{ln: ln, conns: make(map[*feedClient]struct{})}
	s.wg.Add(1)
	go s.acceptLoop()
	return s, nil
}

// Addr is the server's bound TCP address (host:port).
func (s *FeedServer) Addr() string { return s.ln.Addr().String() }

// ClientCount reports how many consumers are currently connected (observability
// for tests and session summaries).
func (s *FeedServer) ClientCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}

func (s *FeedServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // listener closed
		}
		c := &feedClient{
			conn: conn,
			send: make(chan []byte, feedClientBacklog),
			done: make(chan struct{}),
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		s.conns[c] = struct{}{}
		// Goroutines are registered under the same lock that serialises Close,
		// so wg.Add never races wg.Wait (Close flips `closed` first).
		s.wg.Add(2)
		s.mu.Unlock()
		go s.writeLoop(c)
		go s.reapOnClose(c)
	}
}

// writeLoop drains the client's queue onto its conn. It exits when the client
// is shut (dropped or server Close) or when a write fails; queue order is the
// publish order (single writer, FIFO channel).
func (s *FeedServer) writeLoop(c *feedClient) {
	defer s.wg.Done()
	for {
		select {
		case <-c.done:
			return
		case b := <-c.send:
			if _, err := c.conn.Write(b); err != nil {
				s.removeClient(c)
				return
			}
		}
	}
}

// reapOnClose blocks reading the one-way (server→client) connection and prunes
// the client the moment it disconnects (EOF/err), even when no Publish is in
// flight. The consumer never writes, so Read blocks until close.
func (s *FeedServer) reapOnClose(c *feedClient) {
	defer s.wg.Done()
	buf := make([]byte, 1)
	for {
		if _, err := c.conn.Read(buf); err != nil {
			s.removeClient(c)
			return
		}
	}
}

// removeClient drops c from the set and shuts it. Idempotent: a double removal
// (overflow-drop then reap, write-error then reap, or vice versa) is a no-op.
func (s *FeedServer) removeClient(c *feedClient) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
	c.shut()
}

// Publish marshals msg once (protobuf) and enqueues it, as a single 4-byte
// little-endian length-prefixed frame, to every currently-connected client. The
// enqueue never blocks: a client whose queue is full is dropped. A marshal
// error is returned; an absent audience is not an error.
func (s *FeedServer) Publish(msg *sapientpb.SapientMessage) error {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("sapient.FeedServer.Publish: marshal: %w", err)
	}
	// Pre-frame once so every client receives byte-identical output and each
	// client takes a single enqueue under the lock.
	var frame bytes.Buffer
	if err := writeLEFrame(&frame, payload); err != nil {
		return fmt.Errorf("sapient.FeedServer.Publish: frame: %w", err)
	}
	b := frame.Bytes()

	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.conns {
		select {
		case c.send <- b:
		default:
			// Queue full: the consumer stopped reading long ago (kernel
			// buffers + backlog exhausted). Shed it; delete-during-range is
			// safe in Go, and shut() unblocks its writer goroutine.
			delete(s.conns, c)
			c.shut()
		}
	}
	return nil
}

// Close stops accepting, drops every client, closes the listener, and waits
// for every per-client goroutine to exit (no goroutine outlives the server).
func (s *FeedServer) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	for c := range s.conns {
		delete(s.conns, c)
		c.shut()
	}
	s.mu.Unlock()
	err := s.ln.Close()
	s.wg.Wait()
	return err
}
