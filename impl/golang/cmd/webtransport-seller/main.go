// Command webtransport-seller runs a go-libp2p host on a WebTransport
// listener and serves the browser-profile protocols for the 012
// WebTransport spike.
//
// Tier 1 (spec 012 v1) is the Node.js WSS seller at
// impl/typescript/src/server-demo/. This binary does NOT replace it.
// It exists to prove direct browser -> VPS connectivity without the
// SSH -L tunnel that Tier 1 relies on.
//
// Protocols registered:
//
//	/neuron/webtransport-spike/echo/1.0.0     (Tier A — always)
//	/neuron/browser-profile/control/1.0.0     (Tier B — only when --jpeg is set)
//
// When --jpeg is set, the seller also initiates /neuron/browser-profile/data/1.0.0
// streams back to the buyer to deliver the asset.
//
// Usage:
//
//	# Local loopback, Tier A only
//	webtransport-seller --listen 4443 --public-ip 127.0.0.1 \
//	    --bootstrap-out ./bootstrap-wt.json
//
//	# Local loopback, Tier B (full buy flow)
//	webtransport-seller --listen 4443 --public-ip 127.0.0.1 \
//	    --bootstrap-out ./bootstrap-wt.json \
//	    --jpeg ../golang/cmd/buyer-seller-demo/testdata/photo.jpg
//
//	# VPS
//	webtransport-seller --listen 4443 --public-ip 203.0.113.10 \
//	    --bootstrap-out /opt/bootstrap-wt.json \
//	    --jpeg /opt/demo.jpg
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
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/neuron-sdk/neuron-go-sdk/internal/browserprofile"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

func main() {
	listenPort := flag.Int("listen", 4443, "UDP port for WebTransport listener")
	publicIP := flag.String("public-ip", "127.0.0.1", "public IP advertised in bootstrap multiaddr")
	bootstrapOut := flag.String("bootstrap-out", "./bootstrap-wt.json", "path to write bootstrap JSON")
	identityFile := flag.String("identity", "", "optional path to persistent secp256k1 identity; ephemeral if empty")
	jpegPath := flag.String("jpeg", "", "optional path to a JPEG asset to serve over the browser-profile control/data flow; Tier A echo is registered unconditionally")
	flag.Parse()

	libp2pPriv, neuronKey, err := loadOrCreateIdentity(*identityFile)
	if err != nil {
		log.Fatalf("[wt-seller] identity: %v", err)
	}

	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1/webtransport", *listenPort)

	h, err := libp2p.New(
		libp2p.Identity(libp2pPriv),
		libp2p.ListenAddrStrings(listenAddr),
		libp2p.ForceReachabilityPublic(),
	)
	if err != nil {
		log.Fatalf("[wt-seller] create libp2p host: %v", err)
	}
	defer h.Close()

	// Tier A echo handler — always registered so the Ping button works
	// with or without Tier B.
	h.SetStreamHandler(protocol.ID(browserprofile.EchoProtocolID), browserprofile.HandleEcho)

	pid := h.ID()
	fmt.Printf("[wt-seller] peerID: %s\n", pid)
	fmt.Println("[wt-seller] advertised multiaddrs:")
	for _, a := range h.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", a, pid)
	}

	evmAddr := neuronKey.PublicKey().EVMAddress().Hex()

	bootstrapMA, err := browserprofile.PickWebTransportMultiaddr(h, *publicIP)
	if err != nil {
		log.Fatalf("[wt-seller] pick multiaddr: %v", err)
	}
	fmt.Printf("[wt-seller] bootstrap multiaddr: %s\n", bootstrapMA)

	bootstrap := browserprofile.BootstrapWT{
		Version:                 browserprofile.BootstrapVersion,
		SellerEVMAddress:        evmAddr,
		SellerPeerID:            pid.String(),
		SellerWTMultiaddr:       bootstrapMA,
		ControlStreamProtocolID: browserprofile.ControlProtocolID,
		DataStreamProtocolID:    browserprofile.DataProtocolID,
		EchoProtocolID:          browserprofile.EchoProtocolID,
	}
	if err := browserprofile.WriteBootstrap(*bootstrapOut, bootstrap); err != nil {
		log.Fatalf("[wt-seller] write bootstrap: %v", err)
	}
	fmt.Printf("[wt-seller] bootstrap written to %s\n", *bootstrapOut)
	fmt.Printf("[wt-seller] registered handler: %s\n", browserprofile.EchoProtocolID)

	// Tier B wiring — only if the operator supplied an asset.
	if *jpegPath != "" {
		asset, err := browserprofile.LoadAsset(*jpegPath, "image/jpeg")
		if err != nil {
			log.Fatalf("[wt-seller] load asset: %v", err)
		}
		escrow := browserprofile.NewMockEscrow()
		server := browserprofile.NewServer(h, &neuronKey, asset, escrow, bootstrapMA)
		server.RegisterHandlers()
		fmt.Printf("[wt-seller] registered handler: %s\n", browserprofile.ControlProtocolID)
		fmt.Printf("[wt-seller] asset loaded: %s (%d bytes, sha=%s)\n",
			asset.Metadata.Filename, asset.Metadata.SizeBytes, asset.Metadata.Sha256Hex)
	}

	fmt.Println("[wt-seller] ready (Ctrl+C to stop)")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("\n[wt-seller] shutting down")
}

// loadOrCreateIdentity returns both the libp2p-wrapped private key (for
// libp2p.Identity) and the neuron keylib wrapper (for signing TopicMessage
// envelopes on the control stream). Both are derived from the same 32-byte
// secp256k1 scalar, so the PeerID emitted by libp2p matches the EVM address
// derived by keylib.
func loadOrCreateIdentity(path string) (libp2pcrypto.PrivKey, keylib.NeuronPrivateKey, error) {
	if path == "" {
		priv, _, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.Secp256k1, 0, rand.Reader)
		if err != nil {
			return nil, keylib.NeuronPrivateKey{}, fmt.Errorf("generate ephemeral key: %w", err)
		}
		nkey, err := neuronKeyFromLibp2p(priv)
		if err != nil {
			return nil, keylib.NeuronPrivateKey{}, err
		}
		return priv, nkey, nil
	}
	if data, err := os.ReadFile(path); err == nil {
		priv, err := libp2pcrypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, keylib.NeuronPrivateKey{}, fmt.Errorf("unmarshal identity %s: %w", path, err)
		}
		nkey, err := neuronKeyFromLibp2p(priv)
		if err != nil {
			return nil, keylib.NeuronPrivateKey{}, err
		}
		return priv, nkey, nil
	} else if !os.IsNotExist(err) {
		return nil, keylib.NeuronPrivateKey{}, fmt.Errorf("read identity %s: %w", path, err)
	}

	priv, _, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.Secp256k1, 0, rand.Reader)
	if err != nil {
		return nil, keylib.NeuronPrivateKey{}, fmt.Errorf("generate new key: %w", err)
	}
	raw, err := libp2pcrypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, keylib.NeuronPrivateKey{}, fmt.Errorf("marshal identity: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, keylib.NeuronPrivateKey{}, fmt.Errorf("persist identity to %s: %w", path, err)
	}
	fmt.Printf("[wt-seller] generated new identity at %s\n", path)
	nkey, err := neuronKeyFromLibp2p(priv)
	if err != nil {
		return nil, keylib.NeuronPrivateKey{}, err
	}
	return priv, nkey, nil
}

// neuronKeyFromLibp2p extracts the raw 32-byte scalar from a libp2p secp256k1
// private key and builds a keylib.NeuronPrivateKey so the seller can sign
// TopicMessage envelopes on the browser-profile control stream.
func neuronKeyFromLibp2p(priv libp2pcrypto.PrivKey) (keylib.NeuronPrivateKey, error) {
	raw, err := priv.Raw()
	if err != nil {
		return keylib.NeuronPrivateKey{}, fmt.Errorf("extract raw key: %w", err)
	}
	return keylib.NeuronPrivateKeyFromBytes(raw)
}
