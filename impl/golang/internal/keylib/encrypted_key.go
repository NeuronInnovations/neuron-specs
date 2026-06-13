package keylib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"runtime"

	"golang.org/x/crypto/argon2"
)

// EncryptedPrivateKey holds an AES-256-GCM encrypted secp256k1 private key with
// Argon2id key derivation parameters.
//
// Version 1 uses hardcoded default Argon2id parameters (stored in constants.go).
// Version 2 stores custom Argon2id parameters alongside the ciphertext.
//
// JSON serialization encodes salt, nonce, and ciphertext as hex strings.
// The time, memory, and threads fields are omitted for version 1.
type EncryptedPrivateKey struct {
	version    uint8
	salt       []byte // 16 bytes
	nonce      []byte // 12 bytes
	ciphertext []byte // 48 bytes (32 key + 16 GCM tag)
	time       uint32
	memory     uint32
	threads    uint8
}

// Version returns the encryption format version.
func (e *EncryptedPrivateKey) Version() uint8 { return e.version }

// Salt returns a copy of the 16-byte Argon2id salt.
func (e *EncryptedPrivateKey) Salt() []byte {
	out := make([]byte, len(e.salt))
	copy(out, e.salt)
	return out
}

// Nonce returns a copy of the 12-byte AES-GCM nonce.
func (e *EncryptedPrivateKey) Nonce() []byte {
	out := make([]byte, len(e.nonce))
	copy(out, e.nonce)
	return out
}

// Ciphertext returns a copy of the 48-byte ciphertext.
func (e *EncryptedPrivateKey) Ciphertext() []byte {
	out := make([]byte, len(e.ciphertext))
	copy(out, e.ciphertext)
	return out
}

// Time returns the Argon2id time parameter (version 2 only).
func (e *EncryptedPrivateKey) Time() uint32 { return e.time }

// Memory returns the Argon2id memory parameter in KiB (version 2 only).
func (e *EncryptedPrivateKey) Memory() uint32 { return e.memory }

// Threads returns the Argon2id threads parameter (version 2 only).
func (e *EncryptedPrivateKey) Threads() uint8 { return e.threads }

// encryptConfig holds the Argon2id parameters for encryption.
type encryptConfig struct {
	time    uint32
	memory  uint32
	threads uint8
	custom  bool // true if custom params were provided
}

// EncryptOption configures the encryption behavior.
type EncryptOption func(*encryptConfig)

// WithArgon2Params sets custom Argon2id parameters for encryption.
// When used, the resulting EncryptedPrivateKey will have version 2 and include
// the parameters in the JSON output.
func WithArgon2Params(time, memory uint32, threads uint8) EncryptOption {
	return func(cfg *encryptConfig) {
		cfg.time = time
		cfg.memory = memory
		cfg.threads = threads
		cfg.custom = true
	}
}

// Encrypt encrypts a NeuronPrivateKey using AES-256-GCM with an Argon2id-derived
// encryption key.
//
// Implementation:
//  1. Generate random 16-byte salt from crypto/rand.
//  2. Generate random 12-byte nonce from crypto/rand.
//  3. Select Argon2id parameters (default v1 or custom v2).
//  4. Derive 32-byte encryption key via argon2.IDKey.
//  5. Create AES-256-GCM cipher.
//  6. Encrypt the 32-byte private key, producing 48-byte ciphertext (32 + 16 GCM tag).
//
// SEC-003: Key material MUST NOT appear in error messages.
func Encrypt(key NeuronPrivateKey, password string, opts ...EncryptOption) (EncryptedPrivateKey, error) {
	const op = "Encrypt"

	if key.IsZero() {
		return EncryptedPrivateKey{}, NewKeyError(
			ErrZeroValue, op,
			"cannot encrypt a zeroized key",
		)
	}

	// Apply options.
	cfg := encryptConfig{
		time:    Argon2idTime,
		memory:  Argon2idMemory,
		threads: Argon2idThreads,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Generate random salt.
	salt := make([]byte, EncryptionSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return EncryptedPrivateKey{}, NewSDKError(op, fmt.Errorf("salt generation: %w", err))
	}

	// Generate random nonce.
	nonce := make([]byte, EncryptionNonceLength)
	if _, err := rand.Read(nonce); err != nil {
		return EncryptedPrivateKey{}, NewSDKError(op, fmt.Errorf("nonce generation: %w", err))
	}

	// Derive encryption key via Argon2id.
	derivedKey := argon2.IDKey(
		[]byte(password),
		salt,
		cfg.time,
		cfg.memory,
		cfg.threads,
		Argon2idKeyLen,
	)
	defer func() {
		for i := range derivedKey {
			derivedKey[i] = 0
		}
		runtime.KeepAlive(&derivedKey)
	}()

	// Create AES-256-GCM cipher.
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return EncryptedPrivateKey{}, NewSDKError(op, fmt.Errorf("AES cipher: %w", err))
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedPrivateKey{}, NewSDKError(op, fmt.Errorf("GCM: %w", err))
	}

	// Encrypt the private key bytes.
	ciphertext := gcm.Seal(nil, nonce, key.Bytes(), nil)

	// Build the result.
	result := EncryptedPrivateKey{
		salt:       salt,
		nonce:      nonce,
		ciphertext: ciphertext,
	}

	if cfg.custom {
		result.version = 2
		result.time = cfg.time
		result.memory = cfg.memory
		result.threads = cfg.threads
	} else {
		result.version = 1
	}

	return result, nil
}

// Decrypt decrypts an EncryptedPrivateKey using the given password, returning the
// original NeuronPrivateKey.
//
// Implementation:
//  1. Select Argon2id parameters based on version (1: defaults, 2: stored params).
//  2. Derive encryption key via argon2.IDKey.
//  3. Create AES-256-GCM cipher.
//  4. Decrypt the ciphertext.
//  5. Validate and construct a NeuronPrivateKey from the decrypted bytes.
//
// SEC-003: On failure, returns a generic ErrEncryption with no information leak.
func Decrypt(encrypted EncryptedPrivateKey, password string) (NeuronPrivateKey, error) {
	const op = "Decrypt"

	// Validate encrypted field lengths to prevent panics/DoS (M1).
	if len(encrypted.salt) != EncryptionSaltLength {
		return NeuronPrivateKey{}, NewKeyError(
			ErrEncryption, op,
			"decryption failed",
		)
	}
	if len(encrypted.nonce) != EncryptionNonceLength {
		return NeuronPrivateKey{}, NewKeyError(
			ErrEncryption, op,
			"decryption failed",
		)
	}
	if len(encrypted.ciphertext) != EncryptionCiphertextLength {
		return NeuronPrivateKey{}, NewKeyError(
			ErrEncryption, op,
			"decryption failed",
		)
	}

	// Validate Argon2id bounds for version 2 to prevent DoS (M1).
	if encrypted.version == 2 {
		if encrypted.time < 1 || encrypted.time > 10 {
			return NeuronPrivateKey{}, NewKeyError(
				ErrEncryption, op,
				"decryption failed",
			)
		}
		if encrypted.memory < 1 || encrypted.memory > 1048576 {
			return NeuronPrivateKey{}, NewKeyError(
				ErrEncryption, op,
				"decryption failed",
			)
		}
		if encrypted.threads < 1 || encrypted.threads > 64 {
			return NeuronPrivateKey{}, NewKeyError(
				ErrEncryption, op,
				"decryption failed",
			)
		}
	}

	// Select Argon2id parameters based on version.
	var argTime uint32
	var argMemory uint32
	var argThreads uint8

	switch encrypted.version {
	case 1:
		argTime = Argon2idTime
		argMemory = Argon2idMemory
		argThreads = Argon2idThreads
	case 2:
		argTime = encrypted.time
		argMemory = encrypted.memory
		argThreads = encrypted.threads
	default:
		return NeuronPrivateKey{}, NewKeyError(
			ErrEncryption, op,
			"unsupported encryption version",
		)
	}

	// Derive encryption key via Argon2id.
	derivedKey := argon2.IDKey(
		[]byte(password),
		encrypted.salt,
		argTime,
		argMemory,
		argThreads,
		Argon2idKeyLen,
	)
	defer func() {
		for i := range derivedKey {
			derivedKey[i] = 0
		}
		runtime.KeepAlive(&derivedKey)
	}()

	// Create AES-256-GCM cipher.
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return NeuronPrivateKey{}, NewKeyError(
			ErrEncryption, op,
			"decryption failed",
		)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return NeuronPrivateKey{}, NewKeyError(
			ErrEncryption, op,
			"decryption failed",
		)
	}

	// Decrypt the ciphertext.
	plaintext, err := gcm.Open(nil, encrypted.nonce, encrypted.ciphertext, nil)
	if err != nil {
		// Generic error message to prevent information leakage (SEC-003).
		return NeuronPrivateKey{}, NewKeyError(
			ErrEncryption, op,
			"decryption failed",
		)
	}

	return NeuronPrivateKeyFromBytes(plaintext)
}

// MarshalJSON implements json.Marshaler for EncryptedPrivateKey.
// Salt, nonce, and ciphertext are encoded as hex strings for readability.
func (e EncryptedPrivateKey) MarshalJSON() ([]byte, error) {
	type jsonEncryptedKey struct {
		Version    uint8  `json:"version"`
		Salt       []byte `json:"salt"`
		Nonce      []byte `json:"nonce"`
		Ciphertext []byte `json:"ciphertext"`
		Time       uint32 `json:"time,omitempty"`
		Memory     uint32 `json:"memory,omitempty"`
		Threads    uint8  `json:"threads,omitempty"`
	}

	j := jsonEncryptedKey{
		Version:    e.version,
		Salt:       e.salt,
		Nonce:      e.nonce,
		Ciphertext: e.ciphertext,
	}

	if e.version == 2 {
		j.Time = e.time
		j.Memory = e.memory
		j.Threads = e.threads
	}

	return json.Marshal(j)
}

// UnmarshalJSON implements json.Unmarshaler for EncryptedPrivateKey.
func (e *EncryptedPrivateKey) UnmarshalJSON(data []byte) error {
	type jsonEncryptedKey struct {
		Version    uint8  `json:"version"`
		Salt       []byte `json:"salt"`
		Nonce      []byte `json:"nonce"`
		Ciphertext []byte `json:"ciphertext"`
		Time       uint32 `json:"time,omitempty"`
		Memory     uint32 `json:"memory,omitempty"`
		Threads    uint8  `json:"threads,omitempty"`
	}
	var j jsonEncryptedKey
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	e.version = j.Version
	e.salt = j.Salt
	e.nonce = j.Nonce
	e.ciphertext = j.Ciphertext
	e.time = j.Time
	e.memory = j.Memory
	e.threads = j.Threads
	return nil
}
