package keylib

import (
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerID is a type-safe wrapper around a libp2p peer.ID, derived from a
// secp256k1 compressed public key. The PeerID is a base58btc-encoded multihash
// of the public key, typically starting with "12D3KooW" for secp256k1 keys.
//
// T010: PeerID derivation from NeuronPublicKey.
type PeerID struct {
	id peer.ID
}

// String returns the base58btc-encoded multihash representation of the PeerID.
func (p PeerID) String() string {
	return p.id.String()
}

// PeerID derives a libp2p PeerID from the compressed secp256k1 public key.
//
// Derivation:
//  1. Get compressed bytes from NeuronPublicKey (33 bytes)
//  2. Unmarshal as libp2p secp256k1 public key
//  3. Derive PeerID via peer.IDFromPublicKey
//
// T010: PeerID derivation MUST be deterministic for the same public key.
func (k NeuronPublicKey) PeerID() (PeerID, error) {
	const op = "NeuronPublicKey.PeerID"

	compressed := k.Bytes()

	libp2pPub, err := libp2pcrypto.UnmarshalSecp256k1PublicKey(compressed)
	if err != nil {
		return PeerID{}, NewSDKError(op, err)
	}

	peerID, err := peer.IDFromPublicKey(libp2pPub)
	if err != nil {
		return PeerID{}, NewSDKError(op, err)
	}

	return PeerID{id: peerID}, nil
}
