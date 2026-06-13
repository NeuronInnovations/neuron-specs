package keylib

// Key length constants for secp256k1 operations.
const (
	PrivateKeyLength          = 32 // secp256k1 private key: 32 bytes
	CompressedPublicKeyLength = 33 // Compressed public key: 0x02/0x03 prefix + 32 bytes
	UncompressedPublicKeyLen  = 65 // Uncompressed public key: 0x04 prefix + 64 bytes
	EVMAddressLength          = 20 // Ethereum address: last 20 bytes of Keccak256(pubkey)
	SignatureLength           = 65 // ECDSA signature: R(32) || S(32) || V(1)
)

// Encryption constants for Argon2id / AES-256-GCM.
const (
	EncryptionSaltLength       = 16 // Random salt for Argon2id
	EncryptionNonceLength      = 12 // Random nonce for AES-256-GCM
	EncryptionCiphertextLength = 48 // 32-byte key + 16-byte GCM authentication tag
)

// Argon2id v1 default parameters.
const (
	Argon2idTime    = 1         // Time parameter (iterations)
	Argon2idMemory  = 64 * 1024 // Memory parameter in KiB (64 MiB)
	Argon2idThreads = 4         // Parallelism threads
	Argon2idKeyLen  = 32        // Derived key length (AES-256)
)

// BIP-44 derivation constants.
const (
	DefaultDerivationPath = "m/44'/60'/0'/0/0" // Default BIP-44 path for Ethereum
)

// DID:key multicodec constants.
const (
	// DIDKeyPrefix is the prefix for all did:key identifiers.
	DIDKeyPrefix = "did:key:z"
)

// multicodecSecp256k1Pub is the varint-encoded multicodec identifier for secp256k1-pub (0xe7).
var multicodecSecp256k1Pub = []byte{0xe7, 0x01}

// MulticodecSecp256k1Pub returns a copy of the multicodec identifier for secp256k1-pub.
func MulticodecSecp256k1Pub() []byte {
	out := make([]byte, len(multicodecSecp256k1Pub))
	copy(out, multicodecSecp256k1Pub)
	return out
}
