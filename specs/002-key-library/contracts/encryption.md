# API Contract: Key Encryption

**Source**: spec.md FR-015

---

## Encrypt

Encrypts a NeuronPrivateKey with a password.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `privateKey` | NeuronPrivateKey | Key to encrypt |
| `password` | string | User-provided password |
| `options` | EncryptOptions | Optional: version (1 or 2), Argon2 params |

**Output**: Returns EncryptedPrivateKey. Raises Error if encryption fails.

**Algorithm** (FR-015):
1. Generate 16-byte random salt
2. Generate 12-byte random nonce
3. Derive 32-byte encryption key: `Argon2id(password, salt, params)`
   - Version 1: hardcoded defaults (time=1, memory=64MB, threads=4)
   - Version 2: custom params from `options`
4. Encrypt: `AES-256-GCM(encKey, nonce, privateKeyBytes)` → ciphertext (32 + 16 GCM tag = 48 bytes)
5. Return EncryptedPrivateKey with version, salt, nonce, ciphertext, and params (v2)

---

## Decrypt

Decrypts an EncryptedPrivateKey with a password.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `encrypted` | EncryptedPrivateKey | Encrypted key structure |
| `password` | string | User-provided password |

**Output**: Returns NeuronPrivateKey. Raises Error if decryption fails.

**Algorithm** (FR-015):
1. Read version from encrypted structure
2. Derive encryption key: `Argon2id(password, salt, params)`
   - Version 1: hardcoded defaults
   - Version 2: read params from structure
3. Decrypt: `AES-256-GCM_Open(encKey, nonce, ciphertext)` → 32-byte private key
4. If decryption fails: return `Encryption` error (MUST NOT reveal whether password or key was wrong)
5. Elevate decrypted bytes to NeuronPrivateKey

---

## EncryptedPrivateKey JSON Format

```json
{
  "version": 1,
  "salt": "base64-encoded-16-bytes",
  "nonce": "base64-encoded-12-bytes",
  "ciphertext": "base64-encoded-48-bytes"
}
```

Version 2 adds:

```json
{
  "version": 2,
  "salt": "...",
  "nonce": "...",
  "ciphertext": "...",
  "time": 3,
  "memory": 65536,
  "threads": 4
}
```
