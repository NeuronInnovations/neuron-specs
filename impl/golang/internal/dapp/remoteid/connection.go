package remoteid

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

// PublishConnectionSetup builds a payment.ConnectionSetup containing the
// seller's ECIES-encrypted multiaddrs + the streams[] catalog, signs it,
// and publishes to the buyer's stdIn topic. Returns the built setup for
// evidence logging.
//
// FR anchors:
//   - 008 FR-P33 / FR-P33a: ConnectionSetup carries encrypted multiaddrs +
//     streams[] catalog.
//   - 009 FR-D11 / FR-D15: ECIES-secp256k1-AES-256-GCM multiaddr exchange.
//   - 017 FR-R02: streams[] catalog ON THE WIRE (Stage 2 advance over
//     Stage 1's descriptor-only field).
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
		return nil, errors.New("remoteid.PublishConnectionSetup: signingKey required")
	}
	if host == nil {
		return nil, errors.New("remoteid.PublishConnectionSetup: libp2p host required")
	}
	if buyerPubKey == nil {
		return nil, errors.New("remoteid.PublishConnectionSetup: buyerPubKey required (ECIES recipient key)")
	}
	if len(streams) == 0 {
		return nil, errors.New("remoteid.PublishConnectionSetup: at least one StreamCatalogEntry required (FR-R02)")
	}

	// The legacy `protocol` field on ConnectionSetup MUST be a single
	// libp2p protocol-id for pre-Phase-5 buyers per FR-P33. Use the
	// first stream's protocol-id (the canonical raw entry).
	setup, err := delivery.BuildConnectionSetup(
		requestID, host, streams[0].ProtocolID, buyerPubKey,
		delivery.WithStreams(streams),
	)
	if err != nil {
		return nil, fmt.Errorf("remoteid.PublishConnectionSetup: build: %w", err)
	}

	if _, err := PublishPayload(ctx, adapter, buyerStdIn, signingKey, seq, setup); err != nil {
		return nil, fmt.Errorf("remoteid.PublishConnectionSetup: publish: %w", err)
	}
	return setup, nil
}

// ReceiveConnectionSetup blocks on buyerStdIn until a ConnectionSetup with
// requestID arrives. Returns the seller's libp2p AddrInfo (PeerID + decrypted
// multiaddrs) plus the raw setup envelope for evidence logging.
//
// The buyer's load-bearing R2 check happens here: `setup.PeerID` MUST equal
// expectedPeerID (the value read out of the registered AgentURI). On
// mismatch the function returns ErrPeerIDMismatch with the two PeerIDs
// embedded; the CLI surfaces this as "PeerID mismatch — refusing to dial".
//
// Defence-in-depth: after decryption, each decrypted multiaddr that already
// carries a `/p2p/<peerID>` suffix is cross-checked against the same
// expectedPeerID. A bogus suffix surfaces as ErrPeerIDMismatch even if the
// envelope-level PeerID was correct.
func ReceiveConnectionSetup(
	ctx context.Context,
	adapter topic.TopicAdapter,
	buyerStdIn topic.TopicRef,
	requestID string,
	expectedPeerID string,
	recipientPrivKey *ecdsa.PrivateKey,
) (*payment.ConnectionSetup, peer.AddrInfo, error) {
	if recipientPrivKey == nil {
		return nil, peer.AddrInfo{}, errors.New("remoteid.ReceiveConnectionSetup: recipientPrivKey required")
	}
	if expectedPeerID == "" {
		return nil, peer.AddrInfo{}, errors.New("remoteid.ReceiveConnectionSetup: expectedPeerID required (registry-derived identity binding)")
	}
	if requestID == "" {
		return nil, peer.AddrInfo{}, errors.New("remoteid.ReceiveConnectionSetup: requestID required")
	}

	body, _, err := ReceiveTypedPayload(ctx, adapter, buyerStdIn, payment.PayloadConnectionSetup, requestID)
	if err != nil {
		return nil, peer.AddrInfo{}, err
	}
	var setup payment.ConnectionSetup
	if err := json.Unmarshal(body, &setup); err != nil {
		return nil, peer.AddrInfo{}, fmt.Errorf("remoteid.ReceiveConnectionSetup: decode: %w", err)
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
		return &setup, peer.AddrInfo{}, fmt.Errorf("remoteid.ReceiveConnectionSetup: decrypt: %w", err)
	}

	pid, err := peer.Decode(expectedPeerID)
	if err != nil {
		return &setup, peer.AddrInfo{}, fmt.Errorf("remoteid.ReceiveConnectionSetup: parse expected PeerID: %w", err)
	}

	addrs := make([]ma.Multiaddr, 0, len(result.Multiaddrs))
	for _, addr := range result.Multiaddrs {
		parsed, perr := ma.NewMultiaddr(addr)
		if perr != nil {
			return &setup, peer.AddrInfo{}, fmt.Errorf("remoteid.ReceiveConnectionSetup: parse multiaddr %q: %w", addr, perr)
		}
		// Defence-in-depth: if the multiaddr carries a `/p2p/<peerID>`
		// suffix, the embedded peerID MUST match the registry-derived
		// expectation. A bare multiaddr without /p2p/ is fine.
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

// PeerIDMismatchError signals that an observed PeerID did not match the
// registry-derived expectation. Source labels which check fired
// ("connectionSetup.peerID" vs "decryptedMultiaddr.suffix"). Surfaced to
// the buyer CLI as exit 1 + "refusing to dial".
type PeerIDMismatchError struct {
	Expected string
	Observed string
	Source   string
}

// Error implements the error interface.
func (e *PeerIDMismatchError) Error() string {
	return fmt.Sprintf("remoteid: PeerID mismatch in %s — registered=%s observed=%s; refusing to dial",
		e.Source, e.Expected, e.Observed)
}

// ExtractSenderECDSAPublicKey ecrecovers the signer's secp256k1 public
// key from a TopicMessage's signature over its canonical signing input.
// The seller uses this on the buyer's ServiceRequest to obtain the
// buyer's pubkey for ECIES encryption of the subsequent ConnectionSetup.
func ExtractSenderECDSAPublicKey(msg topic.TopicMessage) (*ecdsa.PublicKey, error) {
	sigBytes := msg.Signature()
	if len(sigBytes) == 0 {
		return nil, errors.New("remoteid.ExtractSenderECDSAPublicKey: message has no signature")
	}
	sig, err := keylib.SignatureFromBytes(sigBytes)
	if err != nil {
		return nil, fmt.Errorf("remoteid.ExtractSenderECDSAPublicKey: parse signature: %w", err)
	}
	signingInput := topic.TopicMessageSigningInput(msg.Timestamp(), msg.SequenceNumber(), msg.Payload())
	pub, err := sig.RecoverPublicKey(signingInput)
	if err != nil {
		return nil, fmt.Errorf("remoteid.ExtractSenderECDSAPublicKey: ecrecover: %w", err)
	}
	return pub.ToBlockchainKey()
}
