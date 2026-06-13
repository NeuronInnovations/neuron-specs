package keylib

import "crypto/subtle"

// MatchesPublicKey checks whether the derived public key of this private key
// matches the given NeuronPublicKey, using constant-time comparison.
// FR-016, SEC-004: Type-safe matching with constant-time comparison.
func (k *NeuronPrivateKey) MatchesPublicKey(pub NeuronPublicKey) bool {
	derived := k.PublicKey()
	return subtle.ConstantTimeCompare(derived.data[:], pub.data[:]) == 1
}

// MatchesEVMAddress checks whether the derived EVMAddress of this private key
// matches the given EVMAddress, using constant-time comparison.
func (k *NeuronPrivateKey) MatchesEVMAddress(addr EVMAddress) bool {
	derived := k.PublicKey().EVMAddress()
	return subtle.ConstantTimeCompare(derived.addr[:], addr.addr[:]) == 1
}

// MatchesPeerID checks whether the derived PeerID of this public key
// matches the given PeerID.
func (k NeuronPublicKey) MatchesPeerID(pid PeerID) bool {
	derived, err := k.PeerID()
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(derived.id), []byte(pid.id)) == 1
}

// MatchesEVMAddress checks whether the derived EVMAddress of this public key
// matches the given EVMAddress, using constant-time comparison.
func (k NeuronPublicKey) MatchesEVMAddress(addr EVMAddress) bool {
	derived := k.EVMAddress()
	return subtle.ConstantTimeCompare(derived.addr[:], addr.addr[:]) == 1
}

// EVMAddressMatchesPeerID checks whether the given EVMAddress and PeerID both
// belong to the same identity by deriving both from the provided NeuronPublicKey
// and verifying that each matches the supplied value.
//
// This is the cross-ecosystem bridge: given an EVM address, a libp2p PeerID,
// and a public key that claims to link them, this function verifies the claim.
// Both comparisons use constant-time comparison (EVMAddress) or deterministic
// derivation (PeerID) to avoid timing side-channels.
//
// FR-016, SEC-004: Cross-format identity verification with constant-time comparison.
func EVMAddressMatchesPeerID(addr EVMAddress, pid PeerID, pubkey NeuronPublicKey) bool {
	// Derive the EVMAddress from the public key and compare with constant-time.
	derivedAddr := pubkey.EVMAddress()
	if subtle.ConstantTimeCompare(derivedAddr.addr[:], addr.addr[:]) != 1 {
		return false
	}

	// Derive the PeerID from the public key and compare.
	derivedPID, err := pubkey.PeerID()
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(derivedPID.id), []byte(pid.id)) == 1
}
