# Data Model: Key Library

**Branch**: `002-key-library` | **Date**: 2026-02-25 | **Source**: spec.md FR-001..FR-024, Key Entities

---

## Entities

### NeuronPrivateKey

The primary type in the Key Library. All key operations originate from or relate to this type.

| Field / Method | Type | Description | Source FR |
|----------------|------|-------------|----------|
| (internal raw bytes) | Bytes32 | secp256k1 private key material. NOT exposed. | FR-001, FR-022 |
| `PublicKey()` | NeuronPublicKey | Derives the corresponding public key | FR-001 |
| `EVMAddress()` | EVMAddress | Derives EVM-compatible address | FR-005 |
| `PeerID()` | PeerID | Derives libp2p PeerID | FR-006 |
| `DIDKey()` | string | Derives `did:key:zQ3s...` representation | FR-006a |
| `Sign(message)` | Signature | ECDSA sign (Keccak256 + R\|\|S\|\|V) | FR-014, FR-017 |
| `Matches(pubkey)` | Boolean | Constant-time key relationship check | FR-016 |
| `Matches(address)` | Boolean | Checks if derived EVMAddress matches | FR-016 |
| `Zeroize()` | (none) | Zeroes private key material from memory | FR-021 |

**Construction** (FR-002, FR-010):
- `NewNeuronPrivateKey()` — generate with secure randomness (FR-012)
- `NeuronPrivateKeyFromHex(hex)` — from hex string (with/without 0x prefix)
- `NeuronPrivateKeyFromBytes(bytes)` — from raw 32 bytes
- `NeuronPrivateKeyFromMnemonic(mnemonic, path?)` — BIP-39/44 restoration (FR-013)
- `NeuronPrivateKeyFromBlockchainKey(key)` — elevation from blockchain SDK key (FR-011)

**Invariants** (FR-018, FR-022): Immutable after construction. Valid by construction — no invalid instances exist. Thread-safe for concurrent reads.

---

### NeuronPublicKey

| Field / Method | Type | Description | Source FR |
|----------------|------|-------------|----------|
| (internal compressed bytes) | Bytes33 | Compressed secp256k1 public key (0x02/0x03 prefix) | FR-001 |
| `Compressed()` | ByteArray | 33-byte compressed format | FR-001 |
| `Uncompressed()` | ByteArray | 65-byte uncompressed format (0x04 prefix) | FR-001 |
| `EVMAddress()` | EVMAddress | Keccak256(uncompressed[1:]) → last 20 bytes → EIP-55 | FR-005 |
| `PeerID()` | PeerID | libp2p standard conversion | FR-006 |
| `DIDKey()` | string | `did:key:zQ3s...` from compressed key + multicodec | FR-006a |
| `Matches(peerID)` | Boolean | Checks derived PeerID matches | FR-016 |
| `Matches(address)` | Boolean | Checks derived EVMAddress matches | FR-016 |

**Construction** (FR-002, FR-010):
- `NeuronPublicKeyFromHex(hex)` — from compressed or uncompressed hex
- `NeuronPublicKeyFromBytes(bytes)` — from raw bytes (33 or 65)
- `NeuronPublicKeyFromBlockchainKey(key)` — elevation from blockchain SDK key (FR-011)

**Serialization** (FR-010, W-003): NOT directly JSON-serializable. When embedded in higher-level structures, represented as compressed hex string.

**Invariants**: Immutable, valid by construction, thread-safe for concurrent reads.

---

### EVMAddress

| Field / Method | Type | Description | Source FR |
|----------------|------|-------------|----------|
| (internal bytes) | Bytes20 | 20-byte Ethereum address | FR-005 |
| `Hex()` | string | EIP-55 checksummed hex (e.g. `0x5aAe...`) | FR-005 |
| `LowercaseHex()` | string | Lowercase hex (e.g. `0x5aae...`) | FR-005 |
| `Bytes()` | ByteArray | Raw 20 bytes | FR-005 |

**Construction**:
- `EVMAddressFromHex(hex)` — accepts lowercase or EIP-55 checksummed (FR-005)
- `EVMAddressFromBytes(bytes)` — from raw 20 bytes

**Canonical form**: EIP-55 mixed-case checksum encoding is the default output.

---

### PeerID

| Field / Method | Type | Description | Source FR |
|----------------|------|-------------|----------|
| (internal multihash) | PeerID | libp2p PeerID (multihash encoded) | FR-006 |
| `String()` | string | Base58btc-encoded multihash (e.g. `12D3KooW...`) | FR-006 |

**Derivation process** (from Key Entities): compressed secp256k1 public key → libp2p crypto key → multihash → PeerID. Used in `neuron-p2p-exchange` services (003 FR-R03) and topic system (004 FR-T17).

---

### Signature

| Field / Method | Type | Description | Source FR |
|----------------|------|-------------|----------|
| R | Bytes32 | ECDSA R component | FR-014 |
| S | Bytes32 | ECDSA S component | FR-014 |
| V | Byte | Recovery ID (0/1 standard, 27/28 Ethereum legacy) | FR-014 |
| `Bytes()` | ByteArray | 65-byte R\|\|S\|\|V concatenation | FR-014 |
| `EthereumV()` | Byte | V + 27 (Ethereum legacy format) | FR-014 |
| `StandardV()` | Byte | V as 0 or 1 | FR-014 |
| `Verify(message, pubkey)` | Boolean | Direct verification | FR-017 |
| `RecoverPublicKey(message)` | NeuronPublicKey | Recover signer's public key | FR-017 |

**Signing**: Keccak256(message) → ECDSA sign with RFC 6979 deterministic nonce → R\|\|S\|\|V.

---

### EncryptedPrivateKey

| Field | Type | JSON Key | Description | Source FR |
|-------|------|----------|-------------|----------|
| Version | UnsignedInt8 | `"version"` | Encryption scheme version (1 or 2) | FR-015 |
| Salt | Bytes16 | `"salt"` | Random salt for Argon2id | FR-015 |
| Nonce | Bytes12 | `"nonce"` | Random nonce for AES-GCM | FR-015 |
| Ciphertext | Bytes48 | `"ciphertext"` | 32-byte key + 16-byte GCM tag | FR-015 |
| Time | UnsignedInt32 | `"time"` | Argon2 time parameter (v2 only) | FR-015 |
| Memory | UnsignedInt32 | `"memory"` | Argon2 memory parameter (v2 only) | FR-015 |
| Threads | UnsignedInt8 | `"threads"` | Argon2 threads parameter (v2 only) | FR-015 |

**Serialization**: JSON-serializable (the ONLY Neuron key type that supports JSON). Version 1 uses hardcoded Argon2 defaults; version 2 stores custom parameters.

---

### MultisigKey

| Field / Method | Type | Description | Source FR |
|----------------|------|-------------|----------|
| Protocol | string | Protocol identifier (e.g. `"secp256k1-aggregated"`, `"hedera-threshold"`) | FR-023 |
| Threshold | UnsignedInteger | m in m-of-n | FR-024 |
| TotalKeys | UnsignedInteger | n in m-of-n | FR-024 |
| `EVMAddress()` | EVMAddress or Error | Only for `"secp256k1-aggregated"` mode | FR-023 |
| `PeerID()` | PeerID or Error | Only for `"secp256k1-aggregated"` mode | FR-023 |

**Construction** (FR-024):
- `NewMultisigKey(keys Array<NeuronPrivateKey>, threshold)` — secp256k1-aggregated mode
- `MultisigKeyFromBlockchainKey(key, protocol)` — blockchain-specific threshold key

**Limitation**: `"secp256k1-aggregated"` aggregation algorithm is deferred (GAP-005). Blockchain-specific protocols delegate to underlying SDK.

---

### Error Types (FR-008)

| Error Kind | Description | Source FR |
|------------|-------------|----------|
| `InvalidFormat` | Input format not recognized | FR-008 |
| `InvalidLength` | Input has wrong byte length | FR-008 |
| `InvalidHex` | Non-hex characters in input | FR-008 |
| `InvalidKey` | Key fails curve validation | FR-008 |
| `ZeroValue` | All-zero key material | FR-008 |
| `KeyMismatch` | Key relationship verification failed | FR-008 |
| `Encryption` | Encrypt/decrypt failure | FR-008 |
| `Mnemonic` | Invalid mnemonic (bad checksum, etc.) | FR-008 |
| `Derivation` | BIP-44 derivation failure | FR-008 |
| `UnsupportedKeyType` | Operation not supported for key type (e.g. ED25519, non-secp256k1 MultisigKey) | FR-008, FR-004 |
| `SDKError` | Wrapped blockchain SDK error | FR-008a |

---

## Relationships

```
NeuronPrivateKey ──[derives]──► NeuronPublicKey (FR-001)
NeuronPrivateKey ──[derives]──► EVMAddress (via NeuronPublicKey, FR-005)
NeuronPrivateKey ──[derives]──► PeerID (via NeuronPublicKey, FR-006)
NeuronPrivateKey ──[derives]──► DID:key (via NeuronPublicKey, FR-006a)
NeuronPrivateKey ──[signs]──► Signature (FR-014, FR-017)
NeuronPrivateKey ──[encrypts to]──► EncryptedPrivateKey (FR-015)
NeuronPrivateKey ──[aggregates into]──► MultisigKey (FR-024)
NeuronPrivateKey ──[↔ blockchain key]──► BlockchainSDKKey (FR-011)

NeuronPublicKey ──[derives]──► EVMAddress (FR-005)
NeuronPublicKey ──[derives]──► PeerID (FR-006)
NeuronPublicKey ──[derives]──► DID:key (FR-006a)
NeuronPublicKey ──[verifies]──► Signature (FR-017)

Signature ──[recovers]──► NeuronPublicKey (FR-017)
EncryptedPrivateKey ──[decrypts to]──► NeuronPrivateKey (FR-015)
```
