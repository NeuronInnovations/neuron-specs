package delivery

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/hkdf"
	"crypto/sha256"
	"io"
)

// eciesInfo is the HKDF info string for domain separation.
// FR-D11: info = "neuron-multiaddr-v1"
const eciesInfo = "neuron-multiaddr-v1"

// EncryptMultiaddrs encrypts a list of multiaddr strings using the ECIES profile.
// FR-D11: secp256k1 ECDH + HKDF-SHA256 + AES-256-GCM.
// FR-D12: Input = JSON array of multiaddrs, Output = base64(ephPub || nonce || ciphertext || tag).
// FR-D13: Ephemeral key freshly generated per call (randomized encryption).
func EncryptMultiaddrs(multiaddrs []string, recipientPubKey *ecdsa.PublicKey) (string, error) {
	const op = "EncryptMultiaddrs"

	if recipientPubKey == nil {
		return "", NewDeliveryError(ErrConnectionSetupEncryptionFailed, op, "recipient public key is nil")
	}

	// Serialize multiaddrs as JSON array → UTF-8 bytes.
	plaintext, err := json.Marshal(multiaddrs)
	if err != nil {
		return "", WrapDeliveryError(ErrConnectionSetupEncryptionFailed, op, err)
	}

	// FR-D13: Generate ephemeral secp256k1 keypair.
	ephPrivKey, err := crypto.GenerateKey()
	if err != nil {
		return "", WrapDeliveryError(ErrConnectionSetupEncryptionFailed, op, err)
	}

	// ECDH: shared secret = ephemeral_priv * recipient_pub.
	sharedX, _ := recipientPubKey.Curve.ScalarMult(recipientPubKey.X, recipientPubKey.Y, ephPrivKey.D.Bytes())
	sharedSecret := sharedX.Bytes()

	// Pad to 32 bytes if needed.
	if len(sharedSecret) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(sharedSecret):], sharedSecret)
		sharedSecret = padded
	}

	// HKDF-SHA256: derive AES key (32 bytes).
	// FR-D11: salt = empty, info = "neuron-multiaddr-v1"
	hkdfReader := hkdf.New(sha256.New, sharedSecret, nil, []byte(eciesInfo))
	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, aesKey); err != nil {
		return "", WrapDeliveryError(ErrConnectionSetupEncryptionFailed, op, err)
	}

	// AES-256-GCM encrypt.
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", WrapDeliveryError(ErrConnectionSetupEncryptionFailed, op, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", WrapDeliveryError(ErrConnectionSetupEncryptionFailed, op, err)
	}

	// Generate random nonce (12 bytes).
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", WrapDeliveryError(ErrConnectionSetupEncryptionFailed, op, err)
	}

	// Encrypt: ciphertext includes auth tag (appended by GCM).
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// FR-D12: Output = ephemeral_pub(33) || nonce(12) || ciphertext+tag(N+16)
	ephPubCompressed := crypto.CompressPubkey(&ephPrivKey.PublicKey)

	output := make([]byte, 0, len(ephPubCompressed)+len(nonce)+len(ciphertext))
	output = append(output, ephPubCompressed...)
	output = append(output, nonce...)
	output = append(output, ciphertext...)

	// Base64 encode per 006 FR-W03.
	return base64.StdEncoding.EncodeToString(output), nil
}

// DecryptMultiaddrs decrypts an ECIES-encrypted multiaddr payload.
// FR-D14: Verifies authentication tag; returns error on failure.
func DecryptMultiaddrs(encryptedBase64 string, recipientPrivKey *ecdsa.PrivateKey) ([]string, error) {
	const op = "DecryptMultiaddrs"

	if recipientPrivKey == nil {
		return nil, NewDeliveryError(ErrConnectionSetupEncryptionFailed, op, "recipient private key is nil")
	}

	// Base64 decode.
	raw, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return nil, NewDeliveryError(ErrConnectionSetupEncryptionFailed, op,
			fmt.Sprintf("invalid base64: %v", err))
	}

	// Minimum size: 33 (ephPub) + 12 (nonce) + 16 (tag) = 61 bytes.
	if len(raw) < 61 {
		return nil, NewDeliveryError(ErrConnectionSetupEncryptionFailed, op,
			"ciphertext too short")
	}

	// Parse: ephemeral_pub(33) || nonce(12) || ciphertext+tag(remainder)
	ephPubBytes := raw[:33]
	nonce := raw[33:45]
	ciphertextWithTag := raw[45:]

	// Decompress ephemeral public key.
	ephPubKey, err := crypto.DecompressPubkey(ephPubBytes)
	if err != nil {
		return nil, NewDeliveryError(ErrConnectionSetupEncryptionFailed, op,
			fmt.Sprintf("invalid ephemeral public key: %v", err))
	}

	// Verify curve match.
	if ephPubKey.Curve != crypto.S256() {
		return nil, NewDeliveryError(ErrConnectionSetupEncryptionFailed, op,
			"ephemeral key is not on secp256k1 curve")
	}

	// ECDH: shared secret = recipient_priv * ephemeral_pub (secp256k1).
	sharedX, _ := crypto.S256().ScalarMult(ephPubKey.X, ephPubKey.Y, recipientPrivKey.D.Bytes())
	sharedSecret := sharedX.Bytes()

	// Pad to 32 bytes.
	if len(sharedSecret) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(sharedSecret):], sharedSecret)
		sharedSecret = padded
	}

	// HKDF-SHA256: derive AES key.
	hkdfReader := hkdf.New(sha256.New, sharedSecret, nil, []byte(eciesInfo))
	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, aesKey); err != nil {
		return nil, WrapDeliveryError(ErrConnectionSetupEncryptionFailed, op, err)
	}

	// AES-256-GCM decrypt.
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, WrapDeliveryError(ErrConnectionSetupEncryptionFailed, op, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, WrapDeliveryError(ErrConnectionSetupEncryptionFailed, op, err)
	}

	// FR-D14: Verify authentication tag during decryption.
	plaintext, err := gcm.Open(nil, nonce, ciphertextWithTag, nil)
	if err != nil {
		return nil, NewDeliveryError(ErrConnectionSetupEncryptionFailed, op,
			"decryption failed: authentication tag mismatch or wrong key")
	}

	// Parse JSON array of multiaddr strings.
	var multiaddrs []string
	if err := json.Unmarshal(plaintext, &multiaddrs); err != nil {
		return nil, NewDeliveryError(ErrConnectionSetupEncryptionFailed, op,
			fmt.Sprintf("invalid multiaddr JSON: %v", err))
	}

	return multiaddrs, nil
}

