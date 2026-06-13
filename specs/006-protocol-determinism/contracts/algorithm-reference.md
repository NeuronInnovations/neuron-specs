# Contract: Algorithm Reference

**Spec**: 006-protocol-determinism | **Date**: 2026-03-03
**Scope**: Byte-level algorithm descriptions for all cryptographic and encoding operations across the Neuron SDK protocol (specs 001–010)
**Resolves**: Audit items A-1 through A-7, C-6, C-7, X-6

---

## §1. secp256k1 Key Generation (FR-A01)

**Inputs**: Cryptographically secure random bytes (32 bytes)
**Outputs**: Private key scalar `k` (32 bytes), Public key point `Q`

**Curve parameters** (secp256k1, SEC2 §2.4.1):
- `p` = `0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F` (field prime)
- `n` = `0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141` (group order)
- `G` = generator point with:
  - `Gx` = `0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798`
  - `Gy` = `0x483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8`
- `a` = `0`, `b` = `7` (curve equation: `y² = x³ + 7 mod p`)

**Steps**:
1. Generate 32 cryptographically random bytes.
2. Interpret as a big-endian unsigned integer `k`.
3. Validate: `1 ≤ k < n`. If `k = 0` or `k ≥ n`, regenerate.
4. Compute public key point: `Q = k * G` (scalar multiplication on the secp256k1 curve).

**Edge cases**:
- `k = 0` is invalid (point at infinity). Reject and regenerate.
- `k ≥ n` is invalid. Reject and regenerate.
- The probability of rejection is negligible (~2^-128).

---

## §2. secp256k1 Point Compression (FR-A02)

**Inputs**: Public key point `Q = (X, Y)` on secp256k1
**Outputs**: Compressed public key (33 bytes), Uncompressed public key (65 bytes)

**Compressed format** (33 bytes):
1. Determine the prefix byte: `0x02` if `Y mod 2 = 0` (Y is even), `0x03` if `Y mod 2 = 1` (Y is odd).
2. Concatenate: `prefix (1 byte) || X (32 bytes big-endian)`.

**Uncompressed format** (65 bytes):
1. Concatenate: `0x04 || X (32 bytes big-endian) || Y (32 bytes big-endian)`.

**Decompression** (33 bytes → point):
1. Extract prefix byte and X coordinate (32 bytes big-endian).
2. Compute `Y² = X³ + 7 mod p`.
3. Compute `Y = sqrt(Y²) mod p` (using Tonelli-Shanks or `Y = (Y²)^((p+1)/4) mod p` since `p ≡ 3 mod 4`).
4. If `(Y mod 2)` does not match the prefix parity, set `Y = p - Y`.

---

## §3. EVM Address Derivation (FR-A03)

**Inputs**: Uncompressed public key (65 bytes starting with `0x04`)
**Outputs**: 20-byte EVM address

**Steps**:
1. Strip the `0x04` prefix byte → 64 bytes (`X || Y`).
2. Compute `hash = Keccak256(64 bytes)` → 32 bytes.
3. Take the last 20 bytes of `hash` (bytes 12–31, 0-indexed).

**Note**: Keccak256 is the Ethereum variant (also called "Keccak-256"), NOT SHA-3 (NIST FIPS 202). The difference is in the domain separation padding: Keccak256 uses `0x01` padding, SHA-3 uses `0x06` padding. Use the Ethereum/Keccak variant.

---

## §4. EIP-55 Checksum Encoding (FR-A04)

**Inputs**: 20-byte EVM address
**Outputs**: Checksummed hex string (42 characters including `0x` prefix)

**Steps**:
1. Convert the 20-byte address to a **lowercase hex string** without the `0x` prefix. Result: 40 ASCII characters.
   - Example: `5aaeb6053f3e94c9b9a09f33669435e7ef1beaed`
2. Compute `hash = Keccak256(lowercase_hex_as_ascii_bytes)`. This hashes the 40 ASCII bytes of the hex string, **not** the original 20 address bytes.
   - Input to Keccak256: the bytes `[0x35, 0x61, 0x61, 0x65, ...]` (ASCII for "5aae...")
3. For each character at position `i` (0-indexed, 0 ≤ i < 40) in the hex string:
   - Compute `nibble = hash_byte[i / 2]`. If `i` is even, take the high nibble (`byte >> 4`); if `i` is odd, take the low nibble (`byte & 0x0F`).
   - If `nibble >= 8` and the character is a letter (`a-f`): uppercase it.
   - Otherwise: keep the character as-is.
4. Prepend `0x`.

**Example**:
- Input address: `0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed` (20 bytes)
- Lowercase hex: `5aaeb6053f3e94c9b9a09f33669435e7ef1beaed`
- Keccak256 of ASCII: `c0a3f2...` (hash determines which characters to capitalize)
- Output: `0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed`

**Verification**: To verify a checksummed address, apply the algorithm and compare the result. A mismatch indicates a typo or tampering.

---

## §5. PeerID Derivation (FR-A05)

**Inputs**: 33-byte compressed secp256k1 public key
**Outputs**: PeerID string (base58btc-encoded multihash)

**Steps**:

### Step 5.1: Protobuf PublicKey Message

Construct a protobuf message conforming to the libp2p `crypto.pb.PublicKey` schema:
```protobuf
message PublicKey {
  KeyType Type = 1;  // varint
  bytes Data = 2;    // length-delimited
}
```

Where `KeyType` is an enum: `RSA = 0`, `Ed25519 = 1`, `Secp256k1 = 2`, `ECDSA = 3`.

For a secp256k1 key, the wire bytes are:
```
0x08        // field 1, wire type 0 (varint)
0x02        // value = 2 (Secp256k1)
0x12        // field 2, wire type 2 (length-delimited)
0x21        // length = 33
<33 bytes>  // compressed public key
```

Total: **37 bytes**.

### Step 5.2: Multihash Construction

The libp2p PeerID uses the "identity" multihash for keys whose protobuf encoding is ≤ 42 bytes:

- If serialized length ≤ 42 bytes (true for secp256k1 — 37 bytes): use **identity multihash**.
  - Prefix: `0x00` (identity hash function code) followed by the length as an unsigned varint.
  - For 37 bytes: `0x00 0x25` (0x25 = 37 in hex).
  - Full multihash: `0x00 0x25 || <37 protobuf bytes>` → **39 bytes total**.

- If serialized length > 42 bytes (e.g., RSA keys): use **SHA2-256 multihash**.
  - Compute `hash = SHA256(serialized_bytes)`.
  - Prefix: `0x12` (SHA2-256 hash function code) `0x20` (length 32).
  - Full multihash: `0x12 0x20 || <32 hash bytes>` → **34 bytes total**.

For secp256k1 keys, the identity path (≤ 42 bytes) is always used.

### Step 5.3: Base58btc Encoding

Encode the multihash bytes using base58btc (Bitcoin's base58 encoding):
- Alphabet: `123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz`
- Leading zero bytes in the input encode as leading `1` characters.
- The encoding is the standard base58 algorithm: interpret bytes as a big-endian integer, repeatedly divide by 58, map remainders to alphabet characters, reverse.

The resulting string starts with `12D3KooW` for all secp256k1 keys (this prefix is a consequence of the `0x00 0x25 0x08 0x02 0x12 0x21` byte sequence).

---

## §6. DID:key Construction (FR-A06)

**Inputs**: 33-byte compressed secp256k1 public key
**Outputs**: DID:key string (e.g., `did:key:zQ3s...`)

**Steps**:
1. Prepend the secp256k1-pub multicodec identifier bytes: `0xe7 0x01`.
   - `0xe7 0x01` is the unsigned-LEB128 (varint) encoding of the value 231 (`0xe7`), which is the multicodec code for `secp256k1-pub`.
   - Result: `0xe7 0x01 || <33 compressed key bytes>` → **35 bytes total**.
2. Base58btc-encode the 35 bytes (same algorithm as §5 Step 5.3).
3. Prepend the string `did:key:z`.
   - The `z` prefix indicates base58btc encoding per the Multibase specification.

**Example**:
- Compressed key: `0x02<32 bytes X coordinate>`
- With multicodec: `0xe7 0x01 0x02 <32 bytes>` (35 bytes)
- Base58btc: `Q3s...` (variable length)
- DID:key: `did:key:zQ3s...`

---

## §7. RFC 6979 Deterministic Signing (FR-A07)

**Inputs**: Private key `k` (32 bytes), Message hash `h` (32 bytes from Keccak256)
**Outputs**: ECDSA signature `(r, s)` with recovery ID `v`

**Purpose**: RFC 6979 generates a deterministic nonce `k_nonce` from `(k, h)` using HMAC, eliminating the need for random number generation during signing. This ensures that signing the same `(key, hash)` pair always produces the same signature.

**Algorithm** (RFC 6979 Section 3.2, using HMAC-SHA256):

1. Let `x` = private key bytes (32 bytes, big-endian).
2. Let `h1` = message hash bytes (32 bytes).
3. Initialize:
   - `V = 0x01 0x01 ... 0x01` (32 bytes of `0x01`)
   - `K = 0x00 0x00 ... 0x00` (32 bytes of `0x00`)
4. `K = HMAC-SHA256(K, V || 0x00 || x || h1)`
5. `V = HMAC-SHA256(K, V)`
6. `K = HMAC-SHA256(K, V || 0x01 || x || h1)`
7. `V = HMAC-SHA256(K, V)`
8. Loop:
   a. `V = HMAC-SHA256(K, V)`
   b. Interpret `V` as big-endian integer `k_candidate`.
   c. If `1 ≤ k_candidate < n` and the resulting signature `(r, s)` is valid (both `r ≠ 0` and `s ≠ 0`): accept and break.
   d. Otherwise: `K = HMAC-SHA256(K, V || 0x00)`, `V = HMAC-SHA256(K, V)`, repeat.

**Verification**: Signing the same `(key, hash)` pair twice MUST produce byte-identical `(r, s)` values.

---

## §8. Keccak256 Pre-Image for TopicMessage (FR-A08)

**Inputs**: `timestamp` (UnsignedInt64), `sequenceNumber` (UnsignedInt64), `payload` (byte array)
**Outputs**: 32-byte Keccak256 hash

**Steps**:
1. Encode `timestamp` as 8 bytes, big-endian, unsigned.
2. Encode `sequenceNumber` as 8 bytes, big-endian, unsigned.
3. Concatenate: `timestamp_bytes (8) || sequenceNumber_bytes (8) || payload_bytes (N)`.
   - Total pre-image length: `16 + len(payload)` bytes.
4. Compute `hash = Keccak256(concatenated_bytes)`.

**Example**:
- timestamp = 1700000000000000000 → `0x179A0B63FEDD4000` (8 bytes)
- sequenceNumber = 1 → `0x0000000000000001` (8 bytes)
- payload = `0xDEADBEEF` (4 bytes)
- Pre-image: `0x179A0B63FEDD40000000000000000001DEADBEEF` (20 bytes)
- Hash: `Keccak256(pre_image)` → 32 bytes

---

## §9. Keccak256 Pre-Image for HeartbeatPayload (FR-A09)

**Inputs**: HeartbeatPayload fields, `timestamp` (UnsignedInt64), `sequenceNumber` (UnsignedInt64)
**Outputs**: 32-byte Keccak256 hash (same as TopicMessage signing)

**Steps**:
1. Serialize HeartbeatPayload to canonical JSON bytes per the wire format contract (compact, UTF-8, canonical field order, UnsignedInt64 as string, optional fields omitted if absent).
2. The resulting JSON bytes become the `payload` of a TopicMessage.
3. Apply §8 (TopicMessage pre-image construction): `timestamp_bytes (8) || sequenceNumber_bytes (8) || json_bytes (N)`.
4. Compute `hash = Keccak256(concatenated_bytes)`.

**Critical**: The signature covers the TopicMessage envelope (timestamp + sequence + payload), not the HeartbeatPayload alone. The HeartbeatPayload is just the payload bytes.

---

## §10. ECDSA Signature Encoding (FR-A10)

**Inputs**: ECDSA signature components `(r, s)` and recovery ID `v`
**Outputs**: 65-byte signature `R || S || V`

**Encoding**:
1. `R`: 32 bytes, big-endian encoding of the `r` component. Left-pad with zeros if `r` is less than 32 bytes.
2. `S`: 32 bytes, big-endian encoding of the `s` component. Left-pad with zeros if `s` is less than 32 bytes.
3. `V`: 1 byte, the recovery identifier. Values: `0x00` or `0x01`.

**Total**: 65 bytes. In JSON (base64): 88 characters.

**Low-S normalization** (mandatory):
- After computing `(r, s)`, check if `s > n/2` (where `n` is the secp256k1 curve order).
- If `s > n/2`: replace `s` with `n - s` and flip `v` (0 → 1 or 1 → 0).
- This ensures a unique canonical signature for each `(hash, key)` pair.

**V convention**: This spec uses `V = 0` or `V = 1` (the recovery ID). This is NOT the Ethereum legacy convention of `V = 27` or `V = 28`. To convert to Ethereum convention: `V_eth = V + 27`. Neuron protocol always uses `0`/`1`.

**Recovery**: Given `(R, S, V, hash)`, the signer's public key can be recovered using the ECDSA recovery algorithm. This is used for signature verification without knowing the public key in advance.

---

## §11. Argon2id Key Encryption (FR-A11)

**Inputs**: Private key bytes (32 bytes), password (UTF-8 string)
**Outputs**: EncryptedPrivateKey structure

**Version 1 Parameters** (hardcoded defaults):
| Parameter | Value |
|-----------|-------|
| Time iterations | 1 |
| Memory | 65536 KiB (64 MiB) |
| Parallelism | 4 threads |
| Salt length | 16 bytes (cryptographically random) |
| Tag length | 32 bytes |
| Argon2 variant | Argon2id (hybrid) |

**Steps**:
1. Generate 16 random bytes for the salt.
2. Generate 12 random bytes for the AES-GCM nonce.
3. Derive encryption key: `key = Argon2id(password_utf8_bytes, salt, time=1, memory=65536, threads=4, tag_length=32)` → 32 bytes.
4. Encrypt: `ciphertext = AES-256-GCM-Encrypt(key, nonce, private_key_bytes)` → 48 bytes (32-byte plaintext + 16-byte GCM authentication tag).
5. Construct EncryptedPrivateKey: `{version: 1, salt: <base64>, nonce: <base64>, ciphertext: <base64>}`.

**Version 2**: Same as Version 1, but `time`, `memory`, and `threads` are stored in the JSON and may differ from defaults.

**Decryption**:
1. Read salt, nonce, ciphertext from the EncryptedPrivateKey structure.
2. Read Argon2 parameters: version 1 uses hardcoded defaults; version 2 reads from JSON.
3. Derive key: `key = Argon2id(password_utf8_bytes, salt, time, memory, threads, 32)`.
4. Decrypt: `private_key_bytes = AES-256-GCM-Decrypt(key, nonce, ciphertext)`.
5. Verify: the decrypted bytes form a valid secp256k1 private key (1 ≤ k < n).

---

## §12. BIP-39 Mnemonic to Seed (FR-A12)

**Inputs**: Mnemonic words (12 or 24 English words), optional passphrase (UTF-8 string)
**Outputs**: 64-byte seed

**Steps**:
1. Join mnemonic words with single ASCII space characters (` `, 0x20). Result: a UTF-8 string.
2. Compute the salt: the literal ASCII string `"mnemonic"` concatenated with the passphrase. If no passphrase, the salt is just `"mnemonic"` (8 bytes).
3. Derive seed: `seed = PBKDF2(PRF=HMAC-SHA512, Password=mnemonic_string, Salt=salt, Iterations=2048, dkLen=64)`.

**Word list**: The English BIP-39 word list contains 2048 words. Implementations MUST use the canonical English word list from the BIP-39 specification. Each word encodes 11 bits of entropy. 12 words = 128 bits entropy + 4 bits checksum. 24 words = 256 bits entropy + 8 bits checksum.

**Checksum validation**: Before deriving the seed, implementations SHOULD validate the mnemonic checksum: compute SHA256 of the entropy bytes, take the first `entropy_bits / 32` bits, and verify they match the last bits of the mnemonic encoding.

---

## §13. BIP-44 HD Derivation Path (FR-A13)

**Inputs**: 64-byte BIP-39 seed
**Outputs**: 32-byte secp256k1 private key

**Derivation path**: `m / 44' / 60' / 0' / 0 / 0`

| Level | Index | Hardened? | Meaning |
|-------|-------|-----------|---------|
| m | — | — | Master key (derived from seed) |
| 44' | 44 + 0x80000000 = 0x8000002C | Yes | BIP-44 purpose |
| 60' | 60 + 0x80000000 = 0x8000003C | Yes | Coin type (Ethereum) |
| 0' | 0 + 0x80000000 = 0x80000000 | Yes | Account index |
| 0 | 0 | No | Change (0 = external) |
| 0 | 0 | No | Address index |

**Master key derivation** (BIP-32):
1. `I = HMAC-SHA512(Key="Bitcoin seed", Data=seed)` → 64 bytes.
2. Master private key: `I_L` (first 32 bytes). Must be valid (`1 ≤ I_L < n`).
3. Master chain code: `I_R` (last 32 bytes).

**Child key derivation** (BIP-32):
- **Hardened** (index ≥ 0x80000000):
  1. `I = HMAC-SHA512(Key=chain_code, Data=0x00 || parent_key || index_4bytes_big_endian)`.
  2. Child key: `(I_L + parent_key) mod n`. Must be valid.
  3. Child chain code: `I_R`.

- **Normal** (index < 0x80000000):
  1. `I = HMAC-SHA512(Key=chain_code, Data=parent_pubkey_compressed || index_4bytes_big_endian)`.
  2. Child key: `(I_L + parent_key) mod n`. Must be valid.
  3. Child chain code: `I_R`.

---

## §14. Ed25519 Key Detection and Rejection (FR-A14)

**Inputs**: Key material (bytes or typed object)
**Outputs**: Accept (secp256k1) or reject (Ed25519/other)

**Detection criteria**, in order of precedence:

1. **Typed key object**: If the key is provided via a language-specific type or wrapper that indicates the curve, check the curve identifier. Accept only secp256k1 (OID `1.3.132.0.10`). Reject Ed25519 (OID `1.3.101.112`), P-256 (OID `1.2.840.10045.3.1.7`), and all others. *Note: In typed languages, the curve may be indicated by the key object's type (e.g., an ECDSA key type parameterized by curve).*

2. **DER-encoded key**: If the key is DER/ASN.1-encoded (common in Java/C#), parse the `AlgorithmIdentifier.algorithm` OID. Accept `1.3.132.0.10` (secp256k1). Reject `1.3.101.112` (Ed25519).

3. **Protobuf `PublicKey` message** (libp2p format): Check the `KeyType` field. Accept `2` (Secp256k1). Reject `1` (Ed25519), `0` (RSA), `3` (ECDSA).

4. **Raw bytes without type metadata**: Ambiguous raw bytes MUST NOT be silently accepted. The implementation MUST require an explicit type indicator. If 33 bytes are provided, they could be a compressed secp256k1 key (0x02/0x03 prefix) — validate the prefix byte:
   - `0x02` or `0x03`: likely secp256k1 compressed. Accept and validate on curve.
   - `0x04`: likely uncompressed — accept only if 65 bytes total.
   - Any other prefix: reject.
   - If 32 raw bytes: could be Ed25519 public key OR secp256k1 private key — ambiguous. Require explicit type.

**Error**: When an Ed25519 or unsupported key is detected, raise `NEURON-KEY-002` (UnsupportedKeyType) with a message indicating the detected key type and that only secp256k1 is supported.
