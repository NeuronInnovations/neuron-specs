// delivery-demo is a minimal CLI for testing Spec 009 P2P data delivery
// between a buyer and seller agent in separate terminal windows.
//
// Usage:
//
//	# Terminal 1 — Seller
//	go run ./cmd/delivery-demo --mode seller
//
//	# Terminal 2 — Buyer (paste the Multiaddr from seller output)
//	go run ./cmd/delivery-demo --mode buyer --peer <seller-multiaddr>
package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	libp2phost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
)

func main() {
	mode := flag.String("mode", "", "seller or buyer (required)")
	proto := flag.String("protocol", "/neuron/demo/1.0.0", "stream protocol ID")
	listen := flag.String("listen", "/ip4/0.0.0.0/udp/0/quic-v1", "listen multiaddr (seller)")
	peerAddr := flag.String("peer", "", "seller full multiaddr with /p2p/PeerID (buyer, required)")
	frames := flag.Int("frames", 5, "number of test frames to send (buyer)")
	keyHex := flag.String("key", "", "hex secp256k1 private key; random if omitted")
	payloadSize := flag.Int("payload-size", 0, "custom payload size in bytes (buyer); 0 = default JSON frames")
	disconnectAfter := flag.Int("disconnect-after", 0, "seller disconnects after N received frames; 0 = never")
	relayFlag := flag.String("relay", "", "Comma-separated Circuit Relay v2 multiaddrs; enables autorelay + DCUtR.")
	secondStreamAfterMS := flag.Int("second-stream-after-ms", 0, "buyer waits N ms, then opens a second stream on the same host to prove post-DCUtR path selection; 0 = disabled")
	flag.Parse()

	var hostOpts []delivery.HostOption
	if *relayFlag != "" {
		var relays []string
		for _, a := range strings.Split(*relayFlag, ",") {
			a = strings.TrimSpace(a)
			if a != "" {
				relays = append(relays, a)
			}
		}
		if len(relays) > 0 {
			hostOpts = append(hostOpts,
				delivery.WithRelay(relays...),
				delivery.WithForcedReachability(network.ReachabilityPrivate),
			)
		}
	}

	if *mode != "seller" && *mode != "buyer" {
		fmt.Fprintf(os.Stderr, "Usage: delivery-demo --mode seller|buyer [flags]\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	privKey := generateOrParseKey(*keyHex)

	switch *mode {
	case "seller":
		runSeller(privKey, *listen, *proto, *disconnectAfter, hostOpts)
	case "buyer":
		if *peerAddr == "" {
			log.Fatal("--peer is required in buyer mode")
		}
		runBuyer(privKey, *peerAddr, *proto, *frames, *payloadSize, *secondStreamAfterMS, hostOpts)
	}
}

// runSeller creates a host, registers a stream handler, and waits for incoming data.
// If disconnectAfter > 0, the seller force-disconnects after receiving that many frames.
func runSeller(privKey *ecdsa.PrivateKey, listen, proto string, disconnectAfter int, hostOpts []delivery.HostOption) {
	host, err := delivery.NewLibp2pHost(privKey, listen, hostOpts...)
	if err != nil {
		log.Fatalf("Failed to create seller host: %v", err)
	}
	defer host.Close()

	if len(hostOpts) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := delivery.WaitForRelayReservation(ctx, host); err != nil {
			fmt.Printf("[warn] relay reservation not acquired: %v\n", err)
		} else {
			fmt.Println("[relay] autorelay secured a /p2p-circuit reservation")
		}
	}

	adapter := delivery.NewLibp2pAdapter(host)

	// Build full multiaddrs with /p2p/PeerID for display.
	var fullAddrs []string
	for _, addr := range host.Addrs() {
		fullAddrs = append(fullAddrs, fmt.Sprintf("%s/p2p/%s", addr.String(), host.ID().String()))
	}

	fmt.Println()
	fmt.Println("=== SELLER READY ===")
	fmt.Printf("PeerID:    %s\n", host.ID().String())
	for _, a := range fullAddrs {
		fmt.Printf("Multiaddr: %s\n", a)
	}
	fmt.Printf("Protocol:  %s\n", proto)
	if disconnectAfter > 0 {
		fmt.Printf("Will disconnect after %d frame(s)\n", disconnectAfter)
	}
	fmt.Println()
	fmt.Println("Waiting for buyer connection... (Ctrl+C to stop)")
	fmt.Println()

	// Register handler — receives frames and logs them.
	var streamCount int64
	adapter.HandleIncoming(protocol.ID(proto), func(ch *delivery.DeliveryChannel) {
		streamNum := atomic.AddInt64(&streamCount, 1)
		fmt.Printf("[CONNECTED stream=%d] Buyer %s connected via %s (%s)\n", streamNum, ch.PeerID, ch.Transport, formatPath(ch.Path))
		received := 0
		for {
			frame, err := adapter.Receive(ch)
			if err != nil {
				fmt.Printf("[DISCONNECTED stream=%d] Buyer stream closed: %v\n", streamNum, err)
				return
			}
			received++
			fmt.Printf("[RECV stream=%d] %d bytes: %s\n", streamNum, len(frame.Data), string(frame.Data))

			if disconnectAfter > 0 && received >= disconnectAfter {
				fmt.Printf("[FORCE-DISCONNECT stream=%d] Limit reached (%d frames), closing connection\n", streamNum, disconnectAfter)
				adapter.Disconnect(ch)
				return
			}
		}
	})

	// Block until Ctrl+C.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nSeller shutting down.")
}

// runBuyer creates a host, connects to seller, sends test frames, and disconnects.
// If payloadSize > 0, sends raw bytes of that size instead of JSON payloads.
func runBuyer(
	privKey *ecdsa.PrivateKey,
	peerAddrStr,
	proto string,
	frameCount,
	payloadSize,
	secondStreamAfterMS int,
	hostOpts []delivery.HostOption,
) {
	// Parse seller's multiaddr to extract PeerID and address.
	sellerMA, err := ma.NewMultiaddr(peerAddrStr)
	if err != nil {
		log.Fatalf("Invalid --peer multiaddr: %v", err)
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(sellerMA)
	if err != nil {
		log.Fatalf("Cannot extract PeerID from multiaddr: %v\nMake sure the multiaddr ends with /p2p/<PeerID>", err)
	}

	sellerPeerID := addrInfo.ID.String()

	// Reconstruct multiaddr strings for the Connect call.
	var sellerAddrs []string
	for _, a := range addrInfo.Addrs {
		sellerAddrs = append(sellerAddrs, fmt.Sprintf("%s/p2p/%s", a.String(), sellerPeerID))
	}

	// Create buyer host on a random port.
	host, err := delivery.NewLibp2pHost(privKey, "/ip4/0.0.0.0/udp/0/quic-v1", hostOpts...)
	if err != nil {
		log.Fatalf("Failed to create buyer host: %v", err)
	}
	defer host.Close()

	adapter := delivery.NewLibp2pAdapter(host)

	fmt.Println()
	fmt.Println("=== BUYER CONNECTING ===")
	fmt.Printf("Buyer PeerID:  %s\n", host.ID().String())
	fmt.Printf("Seller PeerID: %s\n", sellerPeerID)
	fmt.Printf("Seller Addrs:  %s\n", strings.Join(sellerAddrs, ", "))
	fmt.Printf("Protocol:      %s\n", proto)
	fmt.Println()

	// Connect to seller.
	channel, err := adapter.Connect(sellerPeerID, sellerAddrs, proto, nil)
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	fmt.Printf("Connected: state=%s, transport=%s, %s\n\n", channel.State(), channel.Transport, formatPath(channel.Path))
	logPeerConnections(host, sellerPeerID, "after initial stream open")

	// Send test frames.
	if payloadSize > 0 {
		fmt.Printf("Sending %d frame(s) of %d bytes each...\n", frameCount, payloadSize)
	} else {
		fmt.Printf("Sending %d test frames...\n", frameCount)
	}
	for i := 0; i < frameCount; i++ {
		var payload []byte
		if payloadSize > 0 {
			// Custom payload: fill with repeating byte pattern.
			payload = make([]byte, payloadSize)
			for j := range payload {
				payload[j] = byte(j % 256)
			}
		} else {
			payload = []byte(fmt.Sprintf(`{"seq":%d,"type":"adsb","data":"test-payload","ts":%d}`, i, time.Now().Unix()))
		}

		result, err := adapter.Send(channel, payload)
		if err != nil {
			fmt.Printf("[%d/%d] Send failed: %v\n", i+1, frameCount, err)
			fmt.Println("Buyer stopping due to send error.")
			adapter.Disconnect(channel)
			fmt.Printf("state=%s\n", channel.State())
			return
		}

		if payloadSize > 0 {
			fmt.Printf("[%d/%d] Sent %d bytes (binary payload)\n", i+1, frameCount, result.BytesSent)
		} else {
			fmt.Printf("[%d/%d] Sent %d bytes: %s\n", i+1, frameCount, result.BytesSent, string(payload))
		}
		time.Sleep(200 * time.Millisecond) // Pace for readability.
	}

	if secondStreamAfterMS > 0 {
		fmt.Println()
		fmt.Printf("Waiting %d ms before opening a second stream on the same host...\n", secondStreamAfterMS)
		time.Sleep(time.Duration(secondStreamAfterMS) * time.Millisecond)
		logPeerConnections(host, sellerPeerID, "before second stream")

		secondChannel, err := adapter.Connect(sellerPeerID, sellerAddrs, proto, nil)
		if err != nil {
			fmt.Printf("[second-stream] Connect failed: %v\n", err)
		} else {
			fmt.Printf("[second-stream] Connected: state=%s, transport=%s, %s\n", secondChannel.State(), secondChannel.Transport, formatPath(secondChannel.Path))
			logPeerConnections(host, sellerPeerID, "after second stream open")

			proofPayload := []byte(fmt.Sprintf(`{"kind":"post-dcutr-proof","ts":%d}`, time.Now().UnixNano()))
			result, sendErr := adapter.Send(secondChannel, proofPayload)
			if sendErr != nil {
				fmt.Printf("[second-stream] Send failed: %v\n", sendErr)
			} else {
				fmt.Printf("[second-stream] Sent %d bytes: %s\n", result.BytesSent, string(proofPayload))
			}

			if err := adapter.Disconnect(secondChannel); err != nil {
				fmt.Printf("[second-stream] Disconnect failed: %v\n", err)
			} else {
				fmt.Printf("[second-stream] state=%s\n", secondChannel.State())
			}
		}
	}

	fmt.Println()
	fmt.Println("All frames sent. Disconnecting...")
	if err := adapter.Disconnect(channel); err != nil {
		log.Fatalf("Disconnect failed: %v", err)
	}
	fmt.Printf("state=%s\n", channel.State())
	fmt.Println("Buyer done.")
}

// generateOrParseKey creates a secp256k1 key from hex or generates a random one.
func generateOrParseKey(hexKey string) *ecdsa.PrivateKey {
	if hexKey != "" {
		hexKey = strings.TrimPrefix(hexKey, "0x")
		keyBytes, err := hex.DecodeString(hexKey)
		if err != nil {
			log.Fatalf("Invalid --key hex: %v", err)
		}
		privKey, err := ethcrypto.ToECDSA(keyBytes)
		if err != nil {
			log.Fatalf("Invalid secp256k1 key: %v", err)
		}
		return privKey
	}

	privKey, err := ethcrypto.GenerateKey()
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}
	return privKey
}

func formatPath(path delivery.ConnectionPath) string {
	return fmt.Sprintf("remote=%s local=%s limited=%t", path.RemoteMultiaddr, path.LocalMultiaddr, path.Limited)
}

func logPeerConnections(h libp2phost.Host, peerID string, label string) {
	pid, err := peer.Decode(peerID)
	if err != nil {
		fmt.Printf("[peer-conns] %s: decode peer %q failed: %v\n", label, peerID, err)
		return
	}

	conns := h.Network().ConnsToPeer(pid)
	fmt.Printf("[peer-conns] %s: %d connection(s)\n", label, len(conns))
	if len(conns) == 0 {
		return
	}

	type connLine struct {
		id   string
		line string
	}
	lines := make([]connLine, 0, len(conns))
	for _, conn := range conns {
		stats := conn.Stat()
		lines = append(lines, connLine{
			id: conn.ID(),
			line: fmt.Sprintf("id=%s remote=%s local=%s limited=%t streams=%d",
				conn.ID(),
				conn.RemoteMultiaddr().String(),
				conn.LocalMultiaddr().String(),
				stats.Limited,
				stats.NumStreams,
			),
		})
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].id < lines[j].id
	})
	for i, line := range lines {
		fmt.Printf("[peer-conns]   #%d %s\n", i+1, line.line)
	}
}
