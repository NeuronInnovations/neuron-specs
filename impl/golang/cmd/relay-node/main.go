// relay-node is the Neuron Circuit Relay v2 node. It listens on QUIC-v1 and
// TCP, serves relay reservations and AutoNAT v2 probes, and prints its
// multiaddrs on startup so operators can copy them into the --relay flag of
// buyer-seller-demo and delivery-demo.
//
// Usage:
//
//	go run ./cmd/relay-node                              # ephemeral identity, port 4001
//	go run ./cmd/relay-node --identity ./relay.key       # persistent PeerID
//	go run ./cmd/relay-node --listen-quic /ip4/0.0.0.0/udp/4001/quic-v1 \
//	                        --listen-tcp  /ip4/0.0.0.0/tcp/4001
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
)

func main() {
	listenQUIC := flag.String("listen-quic", "/ip4/0.0.0.0/udp/4001/quic-v1", "QUIC listen multiaddr")
	listenTCP := flag.String("listen-tcp", "/ip4/0.0.0.0/tcp/4001", "TCP listen multiaddr")
	identityFile := flag.String("identity", "", "path to persistent identity key file (created if missing; ephemeral if empty)")
	flag.Parse()

	priv, err := loadOrCreateIdentity(*identityFile)
	if err != nil {
		log.Fatalf("identity: %v", err)
	}

	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(*listenQUIC, *listenTCP),
		libp2p.ForceReachabilityPublic(),
		libp2p.EnableRelayService(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
	)
	if err != nil {
		log.Fatalf("create libp2p host: %v", err)
	}
	defer h.Close()

	fmt.Println("=== Neuron Relay Node ===")
	fmt.Printf("PeerID: %s\n", h.ID())
	fmt.Println("Advertised multiaddrs (paste into --relay):")
	for _, a := range h.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", a, h.ID())
	}
	fmt.Println()
	fmt.Println("Services: circuit-relay-v2, autonat-v2-server")
	fmt.Println("Default limits: 128 reservations, 1h TTL, 128 KB/direction")
	fmt.Println("Press Ctrl+C to shut down.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("\nShutting down.")
}

func loadOrCreateIdentity(path string) (libp2pcrypto.PrivKey, error) {
	if path == "" {
		priv, _, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.Secp256k1, 0, rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ephemeral key: %w", err)
		}
		return priv, nil
	}

	if data, err := os.ReadFile(path); err == nil {
		priv, err := libp2pcrypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("unmarshal identity from %s: %w", path, err)
		}
		return priv, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read identity file %s: %w", path, err)
	}

	priv, _, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.Secp256k1, 0, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate new key: %w", err)
	}
	raw, err := libp2pcrypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal identity: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, fmt.Errorf("persist identity to %s: %w", path, err)
	}
	fmt.Printf("Generated new identity key at %s\n", path)
	return priv, nil
}
