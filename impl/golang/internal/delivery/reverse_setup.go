package delivery

import (
	"crypto/ecdsa"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
)

// BuildReverseConnectionSetup is the dialee-side equivalent of
// BuildConnectionSetup for the reverse-connect topology used by NAT-shielded
// edge sellers (specs/013-connectivity-profiles, profile E).
//
// The forward path described by FR-D15 / FR-P33 has the seller publish its
// multiaddrs to the buyer; the buyer dials in. That assumes the seller is
// reachable. For NAT-shielded sellers (NeuronHeartBeat reports
// natReachability:false) the topology is inverted:
//
//   - The BUYER is the dialee (reachable).
//   - The SELLER is the dialer.
//   - The BUYER publishes a ConnectionSetup to the seller's stdIn topic.
//   - The seller receives it, decrypts, and dials the buyer.
//
// Functionally, BuildConnectionSetup is direction-agnostic — it just encrypts
// "the dialee's host addresses" with "the dialer's public key". This wrapper
// exists for callers who want the role names to be explicit at the call site.
//
// Parameters:
//
//   - requestID: ties this connection setup to a payment.AgreementStateMachine
//     instance, exactly as in the forward path.
//   - buyerHost: the buyer's libp2p host (must be listening on a reachable
//     multiaddr — IPv4 public, IPv6 public, or DNS-routable hostname).
//   - protocol: the libp2p stream protocol ID (e.g. "/neuron/edge-feed/1.0.0").
//   - sellerPubKey: the seller's secp256k1 ECDSA public key. The buyer's
//     multiaddrs are ECIES-encrypted with this key so only the seller can
//     decrypt them.
//
// The buyer publishes the returned ConnectionSetup to the seller's stdIn
// HCS topic (or whichever control-plane channel the seller observes).
func BuildReverseConnectionSetup(
	requestID string,
	buyerHost host.Host,
	protocol string,
	sellerPubKey *ecdsa.PublicKey,
	opts ...BuildSetupOption,
) (*payment.ConnectionSetup, error) {
	return BuildConnectionSetup(requestID, buyerHost, protocol, sellerPubKey, opts...)
}

// ConnectFromReverseSetup is the dialer-side equivalent of ConnectFromSetup
// for the reverse-connect topology. The seller calls this with its own
// libp2p adapter, the ConnectionSetup payload it received from the buyer's
// stdIn observation, and its own ECDSA private key. The function:
//
//  1. Decrypts the buyer's multiaddrs with sellerPrivKey.
//  2. Validates the buyer's PeerID matches the published value.
//  3. Dials the buyer via adapter.Connect, returning a DeliveryChannel ready
//     for SendStream.
//
// The seller is responsible for everything subsequent: streaming feed frames
// (via SendStream), heartbeating to its own stdOut, and calling
// adapter.Disconnect on graceful shutdown to signal EOF to the buyer.
func ConnectFromReverseSetup(
	adapter DeliveryAdapter,
	setup *payment.ConnectionSetup,
	sellerPrivKey *ecdsa.PrivateKey,
) (*DeliveryChannel, error) {
	return ConnectFromSetup(adapter, setup, sellerPrivKey)
}
