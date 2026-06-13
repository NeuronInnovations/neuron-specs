# API Contract: Key Operations

**Source**: spec.md FR-001..FR-012, FR-016..FR-022

---

## NewNeuronPrivateKey

Generates a new private key with cryptographically secure randomness.

**Input**: None

**Output**: Returns NeuronPrivateKey. Raises Error if key generation fails.

**Behavior** (FR-012):
- Generate 32 random bytes using cryptographically secure random source
- Validate the key is on the secp256k1 curve
- Return sealed, immutable NeuronPrivateKey

---

## NeuronPrivateKeyFromHex

Elevates a hex string to a NeuronPrivateKey.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `hex` | string | Hex-encoded private key (with or without `0x` prefix) |

**Output**: Returns NeuronPrivateKey. Raises Error if validation fails.

**Validation** (FR-002, FR-008):
- Strip `0x` prefix if present
- Validate hex characters (reject with `InvalidHex` + position)
- Validate length = 64 hex chars / 32 bytes (reject with `InvalidLength`)
- Validate non-zero (reject with `ZeroValue`)
- Validate on secp256k1 curve (reject with `InvalidKey`)

---

## NeuronPrivateKeyFromBytes

Elevates raw bytes to a NeuronPrivateKey.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `bytes` | ByteArray | 32-byte raw private key |

**Output**: Returns NeuronPrivateKey. Raises Error if validation fails.

**Validation**: Same as `FromHex` minus hex parsing.

---

## NeuronPrivateKeyFromMnemonic

Restores a NeuronPrivateKey from a BIP-39 mnemonic phrase.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `mnemonic` | string | BIP-39 mnemonic phrase |
| `path` | string | BIP-44 derivation path. Default: `m/44'/60'/0'/0/0` |

**Output**: Returns NeuronPrivateKey. Raises Error if mnemonic is invalid or derivation fails.

**Behavior** (FR-013):
1. Validate mnemonic checksum (reject with `Mnemonic` error)
2. Derive seed from mnemonic (BIP-39)
3. Derive key at path (BIP-44)
4. Return NeuronPrivateKey

---

## NeuronPrivateKeyFromBlockchainKey

Elevates a blockchain SDK private key to a NeuronPrivateKey.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `key` | Any blockchain SDK private key (implementation-specific; e.g., an ECDSA private key object) | Blockchain SDK private key |

**Output**: Returns NeuronPrivateKey. Raises Error if key type is unsupported or extraction fails.

**Behavior** (FR-011, FR-004):
1. Detect key type
2. Reject Ed25519 keys with `UnsupportedKeyType` error (FR-004)
3. Extract secp256k1 raw bytes
4. Validate and return NeuronPrivateKey

---

## NeuronPublicKeyFromHex / FromBytes / FromBlockchainKey

Same pattern as private key factories, but for public keys.

**Input**: hex string, raw bytes (33 compressed or 65 uncompressed), or blockchain SDK public key.

**Output**: Returns NeuronPublicKey. Raises Error if format or curve validation fails.

**Behavior** (FR-001, FR-002): Accepts both compressed (33 bytes) and uncompressed (65 bytes) formats. Stores internally as compressed.

---

## DeriveEVMAddress

Derives an EVM-compatible address from a NeuronPublicKey.

**Input**: `NeuronPublicKey` (method receiver)

**Output**: `EVMAddress`

**Algorithm** (FR-005):
1. Get uncompressed public key (65 bytes, 0x04 prefix)
2. Strip 0x04 prefix → 64 bytes
3. Keccak256(64 bytes) → 32 bytes
4. Take last 20 bytes
5. Apply EIP-55 checksum encoding

---

## DerivePeerID

Derives a libp2p PeerID from a NeuronPublicKey.

**Input**: `NeuronPublicKey` (method receiver)

**Output**: `PeerID`

**Algorithm** (FR-006): Compressed secp256k1 pubkey → libp2p secp256k1 public key → PeerID derivation via libp2p standard conversion.

---

## DeriveDIDKey

Derives a `did:key` identifier from a NeuronPublicKey.

**Input**: `NeuronPublicKey` (method receiver)

**Output**: `string` (e.g. `did:key:zQ3s...`)

**Algorithm** (FR-006a):
1. Get compressed public key (33 bytes)
2. Prepend multicodec varint `0xe7 0x01` (secp256k1-pub)
3. Base58btc encode
4. Return `"did:key:z" + encoded`

---

## Sign

Signs a message using the NeuronPrivateKey.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `message` | ByteArray | Message to sign |

**Output**: Signature

**Algorithm** (FR-014, FR-017):
1. `hash = Keccak256(message)`
2. `signature = ECDSA_Sign(hash, privateKey)` with RFC 6979 deterministic nonce
3. Return Signature{R, S, V} (65 bytes)

---

## Verify

Verifies a signature against a known public key.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `message` | ByteArray | Original message |
| `signature` | Signature | R\|\|S\|\|V signature |
| `publicKey` | NeuronPublicKey | Expected signer |

**Output**: Boolean

**Algorithm** (FR-017):
1. `hash = Keccak256(message)`
2. `recovered = ecrecover(hash, signature)` → uncompressed public key
3. Compare recovered key with provided public key (constant-time, SEC-004)

---

## RecoverPublicKey

Recovers the signer's public key from a signature.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `message` | ByteArray | Original message |
| `signature` | Signature | R\|\|S\|\|V signature |

**Output**: Returns NeuronPublicKey. Raises Error if recovery fails.

**Algorithm** (FR-017):
1. `hash = Keccak256(message)`
2. `uncompressed = ecrecover(hash, signature.Bytes())` → 65-byte uncompressed key
3. Elevate to NeuronPublicKey

---

## Matches (matching functions)

Type-safe relationship verification.

**Variants** (FR-016):
- `NeuronPrivateKey.Matches(NeuronPublicKey) → Boolean` — constant-time comparison
- `NeuronPrivateKey.Matches(EVMAddress) → Boolean` — derive and compare
- `NeuronPublicKey.Matches(PeerID) → Boolean` — derive and compare
- `NeuronPublicKey.Matches(EVMAddress) → Boolean` — derive and compare

All comparisons MUST use constant-time comparison to prevent timing side-channels (SEC-004).

---

## Zeroize

Zeroes private key material from memory.

**Input**: `NeuronPrivateKey` (method receiver)

**Output**: None

**Behavior** (FR-021):
- Overwrites internal byte array with zeros
- Key is unusable after zeroize
- Requires external synchronization if accessed concurrently (FR-022)
