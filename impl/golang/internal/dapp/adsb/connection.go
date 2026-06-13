package adsb

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// PublishConnectionSetup builds an ECIES-encrypted payment.ConnectionSetup
// containing the seller's multiaddrs + the streams[] catalog and publishes
// it on the buyer's stdIn topic. Mirrors remoteid.PublishConnectionSetup.
//
// FR anchors:
//   - 008 FR-P33 / FR-P33a: ConnectionSetup carries encrypted multiaddrs +
//     streams[] catalog.
//   - 009 FR-D11 / FR-D15: ECIES-secp256k1-AES-256-GCM multiaddr exchange.
//   - 016 FR-A02: streams[] catalog on the wire.
func PublishConnectionSetup(
	ctx context.Context,
	adapter topic.TopicAdapter,
	buyerStdIn topic.TopicRef,
	signingKey *keylib.NeuronPrivateKey,
	seq uint64,
	requestID string,
	host host.Host,
	buyerPubKey *ecdsa.PublicKey,
	streams []payment.StreamCatalogEntry,
) (*payment.ConnectionSetup, error) {
	if signingKey == nil {
		return nil, errors.New("adsb.PublishConnectionSetup: signingKey required")
	}
	if host == nil {
		return nil, errors.New("adsb.PublishConnectionSetup: libp2p host required")
	}
	if buyerPubKey == nil {
		return nil, errors.New("adsb.PublishConnectionSetup: buyerPubKey required (ECIES recipient key)")
	}
	if len(streams) == 0 {
		return nil, errors.New("adsb.PublishConnectionSetup: at least one StreamCatalogEntry required (FR-A02)")
	}

	setup, err := delivery.BuildConnectionSetup(
		requestID, host, streams[0].ProtocolID, buyerPubKey,
		delivery.WithStreams(streams),
	)
	if err != nil {
		return nil, fmt.Errorf("adsb.PublishConnectionSetup: build: %w", err)
	}

	if _, err := PublishPayload(ctx, adapter, buyerStdIn, signingKey, seq, setup); err != nil {
		return nil, fmt.Errorf("adsb.PublishConnectionSetup: publish: %w", err)
	}
	return setup, nil
}

// ReceiveConnectionSetup blocks on buyerStdIn until a ConnectionSetup with
// requestID arrives, decrypts the multiaddrs with the buyer's ECIES key,
// and returns the seller AddrInfo + raw setup envelope. Enforces the load-
// bearing PeerID-mismatch check per the R2 "no silent fallback" rule.
func ReceiveConnectionSetup(
	ctx context.Context,
	adapter topic.TopicAdapter,
	buyerStdIn topic.TopicRef,
	requestID string,
	expectedPeerID string,
	recipientPrivKey *ecdsa.PrivateKey,
) (*payment.ConnectionSetup, peer.AddrInfo, error) {
	if recipientPrivKey == nil {
		return nil, peer.AddrInfo{}, errors.New("adsb.ReceiveConnectionSetup: recipientPrivKey required")
	}
	if expectedPeerID == "" {
		return nil, peer.AddrInfo{}, errors.New("adsb.ReceiveConnectionSetup: expectedPeerID required (registry-derived identity binding)")
	}
	if requestID == "" {
		return nil, peer.AddrInfo{}, errors.New("adsb.ReceiveConnectionSetup: requestID required")
	}

	body, _, err := ReceiveTypedPayload(ctx, adapter, buyerStdIn, payment.PayloadConnectionSetup, requestID)
	if err != nil {
		return nil, peer.AddrInfo{}, err
	}
	var setup payment.ConnectionSetup
	if err := json.Unmarshal(body, &setup); err != nil {
		return nil, peer.AddrInfo{}, fmt.Errorf("adsb.ReceiveConnectionSetup: decode: %w", err)
	}

	if setup.PeerID != expectedPeerID {
		return &setup, peer.AddrInfo{}, &PeerIDMismatchError{
			Expected: expectedPeerID,
			Observed: setup.PeerID,
			Source:   "connectionSetup.peerID",
		}
	}

	result, err := delivery.ProcessConnectionSetup(
		setup.PeerID, setup.EncryptedMultiaddrs, setup.Protocol, setup.NATStatus,
		recipientPrivKey,
	)
	if err != nil {
		return &setup, peer.AddrInfo{}, fmt.Errorf("adsb.ReceiveConnectionSetup: decrypt: %w", err)
	}

	pid, err := peer.Decode(expectedPeerID)
	if err != nil {
		return &setup, peer.AddrInfo{}, fmt.Errorf("adsb.ReceiveConnectionSetup: parse expected PeerID: %w", err)
	}

	addrs := make([]ma.Multiaddr, 0, len(result.Multiaddrs))
	for _, addr := range result.Multiaddrs {
		parsed, perr := ma.NewMultiaddr(addr)
		if perr != nil {
			return &setup, peer.AddrInfo{}, fmt.Errorf("adsb.ReceiveConnectionSetup: parse multiaddr %q: %w", addr, perr)
		}
		info, ierr := peer.AddrInfoFromP2pAddr(parsed)
		if ierr == nil && info != nil && info.ID != "" {
			if info.ID.String() != expectedPeerID {
				return &setup, peer.AddrInfo{}, &PeerIDMismatchError{
					Expected: expectedPeerID,
					Observed: info.ID.String(),
					Source:   "decryptedMultiaddr.suffix",
				}
			}
		}
		addrs = append(addrs, parsed)
	}

	return &setup, peer.AddrInfo{ID: pid, Addrs: addrs}, nil
}

// PeerIDMismatchError signals registry-derived ↔ observed PeerID divergence.
type PeerIDMismatchError struct {
	Expected string
	Observed string
	Source   string
}

func (e *PeerIDMismatchError) Error() string {
	return fmt.Sprintf("adsb: PeerID mismatch in %s — registered=%s observed=%s; refusing to dial",
		e.Source, e.Expected, e.Observed)
}

// ExtractSenderECDSAPublicKey ecrecovers the signer's secp256k1 pubkey from
// a TopicMessage's signature over its canonical signing input.
func ExtractSenderECDSAPublicKey(msg topic.TopicMessage) (*ecdsa.PublicKey, error) {
	sigBytes := msg.Signature()
	if len(sigBytes) == 0 {
		return nil, errors.New("adsb.ExtractSenderECDSAPublicKey: message has no signature")
	}
	sig, err := keylib.SignatureFromBytes(sigBytes)
	if err != nil {
		return nil, fmt.Errorf("adsb.ExtractSenderECDSAPublicKey: parse signature: %w", err)
	}
	signingInput := topic.TopicMessageSigningInput(msg.Timestamp(), msg.SequenceNumber(), msg.Payload())
	pub, err := sig.RecoverPublicKey(signingInput)
	if err != nil {
		return nil, fmt.Errorf("adsb.ExtractSenderECDSAPublicKey: ecrecover: %w", err)
	}
	return pub.ToBlockchainKey()
}
