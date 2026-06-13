package browserprofile

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
)

// EchoProtocolID identifies the Tier A handshake-proof stream. A client
// writes one newline-terminated "ping:<payload>" line; the server replies
// with "pong:<payload>\n" and closes the stream.
const EchoProtocolID = "/neuron/webtransport-spike/echo/1.0.0"

// MaxEchoLineBytes bounds a single echo request.
const MaxEchoLineBytes = 4096

// EchoReadTimeout is how long HandleEcho will wait for a client line.
const EchoReadTimeout = 30 * time.Second

// EchoRequestPrefix and EchoResponsePrefix are the fixed line prefixes.
const (
	EchoRequestPrefix  = "ping:"
	EchoResponsePrefix = "pong:"
)

// HandleEcho reads one line of the form "ping:<payload>\n" and writes
// "pong:<payload>\n". Malformed or oversized requests reset the stream.
// Register with host.SetStreamHandler(EchoProtocolID, HandleEcho).
func HandleEcho(s network.Stream) {
	defer s.Close()
	remote := s.Conn().RemotePeer().String()
	_ = s.SetDeadline(time.Now().Add(EchoReadTimeout))

	reader := bufio.NewReaderSize(s, MaxEchoLineBytes)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		fmt.Printf("[wt-seller] echo read error from %s: %v\n", remote, err)
		_ = s.Reset()
		return
	}
	if len(line) > MaxEchoLineBytes {
		fmt.Printf("[wt-seller] echo line too long from %s: %d bytes\n", remote, len(line))
		_ = s.Reset()
		return
	}

	trimmed := strings.TrimRight(line, "\r\n")
	fmt.Printf("[wt-seller] echo from %s: %s\n", remote, trimmed)

	if !strings.HasPrefix(trimmed, EchoRequestPrefix) {
		fmt.Printf("[wt-seller] echo rejected from %s: missing %q prefix\n", remote, EchoRequestPrefix)
		_ = s.Reset()
		return
	}
	payload := trimmed[len(EchoRequestPrefix):]

	if _, err := fmt.Fprintf(s, "%s%s\n", EchoResponsePrefix, payload); err != nil {
		fmt.Printf("[wt-seller] echo write error to %s: %v\n", remote, err)
		_ = s.Reset()
		return
	}
}
