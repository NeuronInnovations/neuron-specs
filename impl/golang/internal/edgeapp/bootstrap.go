package edgeapp

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// SellerBootstrap is the JSON file the seller writes after creating its
// HCS topics so the buyer can find it. The buyer reads this file to learn
// (a) which topic to publish ReverseConnectionSetup on, (b) which key to
// ECIES-encrypt the payload to, and (c) which agent it is talking to.
//
// All fields are strings so the file is hand-editable for one-off testing.
type SellerBootstrap struct {
	EVMAddress     string `json:"evmAddress"`
	PublicKeyHex   string `json:"publicKeyHex"` // 65-byte uncompressed secp256k1 point, hex
	StdInLocator   string `json:"stdInLocator"`
	StdOutLocator  string `json:"stdOutLocator"`
	StdErrLocator  string `json:"stdErrLocator"`
	BackendKind    string `json:"backendKind"` // e.g. "hcs"
	NetworkLabel   string `json:"networkLabel,omitempty"`
}

// WriteSellerBootstrap serializes b to path with mode 0644.
func WriteSellerBootstrap(path string, b SellerBootstrap) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ReadSellerBootstrap parses a SellerBootstrap from disk.
func ReadSellerBootstrap(path string) (SellerBootstrap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SellerBootstrap{}, err
	}
	var b SellerBootstrap
	if err := json.Unmarshal(data, &b); err != nil {
		return SellerBootstrap{}, fmt.Errorf("parse bootstrap: %w", err)
	}
	return b, nil
}

// SellerPubKey decodes b.PublicKeyHex into an *ecdsa.PublicKey.
func (b SellerBootstrap) SellerPubKey() (*ecdsa.PublicKey, error) {
	bs, err := hex.DecodeString(b.PublicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode publicKeyHex: %w", err)
	}
	pub, err := ethcrypto.UnmarshalPubkey(bs)
	if err != nil {
		return nil, fmt.Errorf("unmarshal secp256k1 pubkey: %w", err)
	}
	return pub, nil
}

// SellerStdIn returns the seller's stdIn TopicRef parsed from this bootstrap.
func (b SellerBootstrap) SellerStdIn() (topic.TopicRef, error) {
	if b.StdInLocator == "" {
		return topic.TopicRef{}, fmt.Errorf("missing stdInLocator")
	}
	kind := topic.BackendHCS
	if b.BackendKind != "" {
		kind = topic.BackendKind(b.BackendKind)
	}
	return topic.NewTopicRef(kind, b.StdInLocator)
}

// SellerStdOut returns the seller's stdOut TopicRef parsed from this bootstrap,
// or the zero TopicRef + nil error when the bootstrap doesn't carry one (older
// bootstraps may lack StdOutLocator). Callers that require StdOut (e.g. spec-005
// deadline observation) must check Locator() != "" before use.
func (b SellerBootstrap) SellerStdOut() (topic.TopicRef, error) {
	if b.StdOutLocator == "" {
		return topic.TopicRef{}, nil
	}
	kind := topic.BackendHCS
	if b.BackendKind != "" {
		kind = topic.BackendKind(b.BackendKind)
	}
	return topic.NewTopicRef(kind, b.StdOutLocator)
}
