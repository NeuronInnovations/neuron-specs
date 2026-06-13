package keylib

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

// GenerateMnemonic produces a new BIP-39 mnemonic phrase (12 words, 128 bits of entropy).
//
// The returned mnemonic can be used with NeuronPrivateKeyFromMnemonic to derive a
// deterministic secp256k1 private key.
func GenerateMnemonic() (string, error) {
	const op = "GenerateMnemonic"

	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return "", NewSDKError(op, fmt.Errorf("entropy generation: %w", err))
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", NewSDKError(op, fmt.Errorf("mnemonic generation: %w", err))
	}

	return mnemonic, nil
}

// NeuronPrivateKeyFromMnemonic derives a NeuronPrivateKey from a BIP-39 mnemonic phrase
// using BIP-32/BIP-44 hierarchical deterministic derivation.
//
// If path is empty, DefaultDerivationPath ("m/44'/60'/0'/0/0") is used.
// The mnemonic is validated via BIP-39 checksum verification.
//
// Derivation:
//  1. Validate mnemonic (BIP-39 checksum).
//  2. Generate 64-byte seed from mnemonic (no passphrase).
//  3. Derive master key via BIP-32.
//  4. Parse and follow derivation path (hardened components use ' suffix).
//  5. Extract 32-byte private key scalar from final derived key.
//  6. Validate on secp256k1 curve.
//
// The same mnemonic + path always produces the same private key (deterministic).
func NeuronPrivateKeyFromMnemonic(mnemonic string, path string) (NeuronPrivateKey, error) {
	const op = "NeuronPrivateKeyFromMnemonic"

	// Default to standard Ethereum BIP-44 path.
	if path == "" {
		path = DefaultDerivationPath
	}

	// Validate mnemonic via BIP-39 checksum.
	if !bip39.IsMnemonicValid(mnemonic) {
		return NeuronPrivateKey{}, NewKeyError(
			ErrMnemonic, op,
			"invalid BIP-39 mnemonic (bad checksum or word list)",
		)
	}

	// Generate 64-byte seed from mnemonic with empty passphrase.
	seed := bip39.NewSeed(mnemonic, "")

	// Derive master key via BIP-32.
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return NeuronPrivateKey{}, NewKeyError(
			ErrDerivation, op,
			fmt.Sprintf("master key derivation failed: %s", err.Error()),
		)
	}

	// Parse the derivation path and follow each component.
	indices, err := parseDerivationPath(path)
	if err != nil {
		return NeuronPrivateKey{}, NewKeyError(
			ErrDerivation, op,
			err.Error(),
		)
	}

	key := masterKey
	for _, idx := range indices {
		key, err = key.NewChildKey(idx)
		if err != nil {
			return NeuronPrivateKey{}, NewKeyError(
				ErrDerivation, op,
				fmt.Sprintf("child key derivation failed at index %d: %s", idx, err.Error()),
			)
		}
	}

	// Extract the 32-byte private key scalar.
	// BIP-32 Key.Key for private keys is the raw 32-byte scalar.
	raw := key.Key
	if len(raw) != PrivateKeyLength {
		return NeuronPrivateKey{}, NewKeyError(
			ErrDerivation, op,
			fmt.Sprintf("derived key has unexpected length %d, expected %d", len(raw), PrivateKeyLength),
		)
	}

	return neuronPrivateKeyFromValidatedBytes(raw, op)
}

// parseDerivationPath parses a BIP-32/BIP-44 derivation path string (e.g. "m/44'/60'/0'/0/0")
// into a slice of uint32 child indices. Hardened components (ending with "'" or "h") have
// bip32.FirstHardenedChild (0x80000000) added to their index.
func parseDerivationPath(path string) ([]uint32, error) {
	// Trim whitespace.
	path = strings.TrimSpace(path)

	// Split by "/".
	components := strings.Split(path, "/")
	if len(components) == 0 {
		return nil, fmt.Errorf("empty derivation path")
	}

	// First component must be "m" (master).
	start := 0
	if components[0] == "m" || components[0] == "M" {
		start = 1
	}

	if start >= len(components) {
		return nil, fmt.Errorf("derivation path has no child components: %q", path)
	}

	indices := make([]uint32, 0, len(components)-start)
	for _, component := range components[start:] {
		if component == "" {
			return nil, fmt.Errorf("empty component in derivation path: %q", path)
		}

		// Check for hardened indicator.
		hardened := false
		cleaned := component
		if strings.HasSuffix(cleaned, "'") || strings.HasSuffix(cleaned, "h") || strings.HasSuffix(cleaned, "H") {
			hardened = true
			cleaned = cleaned[:len(cleaned)-1]
		}

		// Parse the numeric index.
		idx, err := strconv.ParseUint(cleaned, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid index %q in derivation path: %s", component, err.Error())
		}

		childIdx := uint32(idx)
		if hardened {
			childIdx += bip32.FirstHardenedChild
		}

		indices = append(indices, childIdx)
	}

	return indices, nil
}
