package account

import (
	"fmt"
	"strings"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// DIDDocument is a placeholder for the W3C DID Document structure.
// The full specification of the document fields will be defined in a future version.
// For now, it serves as the type anchor for NeuronDID.Document.
type DIDDocument struct{}

// NeuronDID represents a Neuron decentralized identifier in did:key format.
// The identifier field holds the full DID string (e.g. "did:key:zQ3s...").
//
// Parent accounts MUST have a NeuronDID. Child and Shared accounts MUST NOT.
type NeuronDID struct {
	identifier string
	document   DIDDocument
}

// Identifier returns the full DID string (e.g. "did:key:zQ3s...").
func (d *NeuronDID) Identifier() string { return d.identifier }

// Document returns the DID document.
func (d *NeuronDID) Document() DIDDocument { return d.document }

// FR-012: DID:key format generation for Parent accounts
// GenerateDID derives a NeuronDID from a NeuronPublicKey using the did:key method.
// The resulting identifier uses the "did:key:zQ3s..." format for secp256k1 keys.
//
// Returns an error if the public key is the zero value (all-zero compressed bytes).
func GenerateDID(pubKey keylib.NeuronPublicKey) (NeuronDID, error) {
	// Check for zero-value public key by examining compressed bytes.
	compressed := pubKey.Compressed()
	allZero := true
	for _, b := range compressed {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return NeuronDID{}, fmt.Errorf("cannot generate DID from zero-value public key")
	}

	didKey := pubKey.DIDKey()
	if !strings.HasPrefix(didKey, "did:key:z") {
		return NeuronDID{}, fmt.Errorf("unexpected DID format: %s", didKey)
	}

	return NeuronDID{
		identifier: didKey,
		document:   DIDDocument{},
	}, nil
}
