# API Contract: ECIES Encryption Profile

**Source**: spec.md FR-D11–D14, satisfying 008 FR-P34

---

## Profile

| Component | Algorithm | Parameters |
|-----------|-----------|------------|
| Key Agreement | secp256k1 ECDH | Sender generates ephemeral keypair; shared secret = ECDH(ephemeral_priv, recipient_pub) |
| KDF | HKDF-SHA256 | salt = empty (zero-length), info = `"neuron-multiaddr-v1"`, output = 32 bytes |
| AEAD | AES-256-GCM | key = KDF output (32 bytes), nonce = 12 bytes (crypto/rand) |

## Encrypt

**Input**: JSON array of multiaddr strings, recipient's NeuronPublicKey
**Output**: base64-encoded ciphertext (RFC 4648 §4)

```
1. Generate ephemeral secp256k1 keypair (ephemeral_priv, ephemeral_pub)
2. Compute shared_secret = ECDH(ephemeral_priv, recipient_pub) → 32 bytes
3. Derive AES key = HKDF-SHA256(shared_secret, salt="", info="neuron-multiaddr-v1") → 32 bytes
4. Generate nonce = crypto/rand(12 bytes)
5. Serialize multiaddrs as JSON array → UTF-8 bytes (plaintext)
6. Encrypt: ciphertext || tag = AES-256-GCM(key, nonce, plaintext)
7. Output = ephemeral_pub_compressed(33) || nonce(12) || ciphertext(N) || tag(16)
8. Base64-encode output per 006 FR-W03
```

## Decrypt

**Input**: base64-encoded ciphertext, recipient's NeuronPrivateKey
**Output**: JSON array of multiaddr strings

```
1. Base64-decode input
2. Parse: ephemeral_pub(33) || nonce(12) || ciphertext(N) || tag(16)
3. Decompress ephemeral_pub → secp256k1 point
4. Compute shared_secret = ECDH(recipient_priv, ephemeral_pub) → 32 bytes
5. Derive AES key = HKDF-SHA256(shared_secret, salt="", info="neuron-multiaddr-v1") → 32 bytes
6. Decrypt: plaintext = AES-256-GCM-Open(key, nonce, ciphertext || tag)
7. If tag verification fails → ConnectionSetupEncryptionFailed error
8. Parse plaintext as JSON array of multiaddr strings
```

## Security Properties

- **Randomized**: Ephemeral key freshly generated per encryption (FR-D13). Same input → different ciphertext.
- **Authenticated**: AES-256-GCM tag prevents tampering (FR-D14).
- **Recipient-only**: Only holder of recipient's NeuronPrivateKey can derive shared secret.
- **Forward secrecy**: Ephemeral key is discarded after encryption; compromise of long-term key doesn't decrypt past ciphertexts.

## Ciphertext Format

```
| Ephemeral PubKey | Nonce    | Ciphertext | Auth Tag |
| 33 bytes         | 12 bytes | N bytes    | 16 bytes |
```

Total: 61 + N bytes (before base64 encoding).
