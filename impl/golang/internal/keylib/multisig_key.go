package keylib

import "fmt"

// MultisigKey represents a multi-signature key configuration, supporting both
// native secp256k1 key aggregation and external blockchain-specific multisig
// protocols (e.g. Hedera threshold keys, FROST, BLS).
//
// For native secp256k1 keys, MultisigKey stores the individual NeuronPrivateKeys
// and an m-of-n threshold. For external protocols, it wraps an opaque blockchain
// key with a protocol identifier.
//
// FR-014: MultisigKey MUST track the signing protocol for interoperability.
// GAP-005: secp256k1 key aggregation algorithm is not yet specified; EVMAddress
// and PeerID derivation are therefore blocked for all MultisigKey variants.
type MultisigKey struct {
	protocol      string
	threshold     uint
	totalKeys     uint
	keys          []NeuronPrivateKey // populated only for "secp256k1-aggregated"
	blockchainKey interface{}        // populated only for external protocols
}

// NewMultisigKey creates a MultisigKey from a set of NeuronPrivateKeys with an
// m-of-n threshold. The protocol is automatically set to "secp256k1-aggregated".
//
// Validation:
//   - keys must not be empty
//   - threshold must be > 0
//   - threshold must be <= len(keys)
func NewMultisigKey(keys []NeuronPrivateKey, threshold uint) (MultisigKey, error) {
	const op = "NewMultisigKey"

	if len(keys) == 0 {
		return MultisigKey{}, NewKeyError(
			ErrInvalidKey, op,
			"keys slice must not be empty",
		)
	}

	if threshold == 0 {
		return MultisigKey{}, NewKeyError(
			ErrInvalidKey, op,
			"threshold must be greater than zero",
		)
	}

	if threshold > uint(len(keys)) {
		return MultisigKey{}, NewKeyError(
			ErrInvalidKey, op,
			fmt.Sprintf("threshold %d exceeds number of keys %d", threshold, len(keys)),
		)
	}

	// Defensive copy of the keys slice to prevent external mutation.
	keysCopy := make([]NeuronPrivateKey, len(keys))
	copy(keysCopy, keys)

	return MultisigKey{
		protocol:  "secp256k1-aggregated",
		threshold: threshold,
		totalKeys: uint(len(keys)),
		keys:      keysCopy,
	}, nil
}

// MultisigKeyFromBlockchainKey wraps an external blockchain-specific key with
// a protocol identifier. This supports interoperability with non-secp256k1
// multisig schemes such as Hedera threshold keys, FROST, or BLS.
//
// The key parameter is treated as opaque and stored as-is. The protocol string
// identifies the signing scheme (e.g. "hedera-threshold", "frost", "bls").
//
// Validation:
//   - key must not be nil
//   - protocol must not be empty
func MultisigKeyFromBlockchainKey(key interface{}, protocol string) (MultisigKey, error) {
	const op = "MultisigKeyFromBlockchainKey"

	if key == nil {
		return MultisigKey{}, NewKeyError(
			ErrInvalidKey, op,
			"blockchain key must not be nil",
		)
	}

	if protocol == "" {
		return MultisigKey{}, NewKeyError(
			ErrInvalidKey, op,
			"protocol must not be empty",
		)
	}

	return MultisigKey{
		protocol:      protocol,
		threshold:     1, // default for external keys; actual threshold is protocol-dependent
		totalKeys:     1,
		blockchainKey: key,
	}, nil
}

// MultisigKeyFromMetadata creates a MultisigKey containing only metadata
// (protocol, threshold, totalKeys) without any key material. This is used
// during deserialization where actual private keys or blockchain keys are
// not available in the serialized form.
//
// The resulting MultisigKey can report its Protocol(), Threshold(), and
// TotalKeys() but cannot perform signing or address derivation operations.
func MultisigKeyFromMetadata(protocol string, threshold, totalKeys uint) MultisigKey {
	return MultisigKey{
		protocol:  protocol,
		threshold: threshold,
		totalKeys: totalKeys,
	}
}

// Protocol returns the signing protocol identifier for this MultisigKey.
// For native secp256k1 keys this is "secp256k1-aggregated"; for external keys
// it is the protocol string provided at construction (e.g. "hedera-threshold",
// "frost", "bls").
func (mk *MultisigKey) Protocol() string {
	return mk.protocol
}

// Threshold returns the m in the m-of-n threshold scheme.
// For external blockchain keys, this returns a default of 1 since the actual
// threshold is managed by the underlying protocol.
func (mk *MultisigKey) Threshold() uint {
	return mk.threshold
}

// TotalKeys returns the n in the m-of-n threshold scheme.
// For external blockchain keys, this returns 1.
func (mk *MultisigKey) TotalKeys() uint {
	return mk.totalKeys
}

// EVMAddress attempts to derive an EVM address from the multisig key.
//
// For non-secp256k1-aggregated protocols, this returns ErrUnsupportedKeyType
// because address derivation is only defined for secp256k1 keys.
//
// For secp256k1-aggregated, this also returns an error because the key
// aggregation algorithm is not yet specified (GAP-005).
func (mk *MultisigKey) EVMAddress() (EVMAddress, error) {
	const op = "MultisigKey.EVMAddress"

	if mk.protocol != "secp256k1-aggregated" {
		return EVMAddress{}, NewKeyError(
			ErrUnsupportedKeyType, op,
			fmt.Sprintf("EVMAddress not supported for protocol %q", mk.protocol),
		)
	}

	// GAP-005: aggregation algorithm not yet specified.
	return EVMAddress{}, NewKeyError(
		ErrUnsupportedKeyType, op,
		"secp256k1 key aggregation algorithm not yet specified (GAP-005)",
	)
}

// PeerID attempts to derive a libp2p PeerID from the multisig key.
//
// For non-secp256k1-aggregated protocols, this returns ErrUnsupportedKeyType
// because PeerID derivation is only defined for secp256k1 keys.
//
// For secp256k1-aggregated, this also returns an error because the key
// aggregation algorithm is not yet specified (GAP-005).
func (mk *MultisigKey) PeerID() (PeerID, error) {
	const op = "MultisigKey.PeerID"

	if mk.protocol != "secp256k1-aggregated" {
		return PeerID{}, NewKeyError(
			ErrUnsupportedKeyType, op,
			fmt.Sprintf("PeerID not supported for protocol %q", mk.protocol),
		)
	}

	// GAP-005: aggregation algorithm not yet specified.
	return PeerID{}, NewKeyError(
		ErrUnsupportedKeyType, op,
		"secp256k1 key aggregation algorithm not yet specified (GAP-005)",
	)
}

// ToBlockchainKey converts the MultisigKey back to its underlying key material.
//
// For secp256k1-aggregated keys, this returns the stored []NeuronPrivateKey slice.
// For external blockchain keys, this returns the opaque key provided at construction.
func (mk *MultisigKey) ToBlockchainKey() (interface{}, error) {
	const op = "MultisigKey.ToBlockchainKey"

	if mk.protocol == "secp256k1-aggregated" {
		if mk.keys == nil {
			return nil, NewKeyError(
				ErrInvalidKey, op,
				"no keys stored in secp256k1-aggregated MultisigKey",
			)
		}
		keysCopy := make([]NeuronPrivateKey, len(mk.keys))
		copy(keysCopy, mk.keys)
		return keysCopy, nil
	}

	if mk.blockchainKey == nil {
		return nil, NewKeyError(
			ErrInvalidKey, op,
			"no blockchain key stored",
		)
	}
	return mk.blockchainKey, nil
}
