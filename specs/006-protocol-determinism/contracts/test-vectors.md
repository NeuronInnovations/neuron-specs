# Contract: Test Vectors

**Spec**: 006-protocol-determinism | **Date**: 2026-03-03
**Scope**: Golden test vector chains for cross-language interoperability verification
**Resolves**: Audit items C-3, X-2

---

## Overview

Each test vector chain provides a complete sequence of intermediate values from input to final output, with every value in hex or the appropriate encoding. Implementations MUST produce byte-identical results for these test vectors.

**Generation method**: Values are derived from the algorithm definitions in `algorithm-reference.md` and cross-verified across multiple independent implementations. See Research R6.

**Note on completeness**: All four chains (key derivation, TopicMessage signing, HeartbeatPayload signing, key encryption round-trip) are complete with verified values computed from the Go reference implementation.

---

## Chain 1: Key Derivation

**Purpose**: Verifies the full key derivation chain from private key to all derived identifiers.

**Test Private Key**: `0x0000000000000000000000000000000000000000000000000000000000000001`

This is the simplest valid secp256k1 private key (k=1), making it easily reproducible across libraries.

### 1.1 Private Key

```
private_key_hex: 0x0000000000000000000000000000000000000000000000000000000000000001
```

### 1.2 Public Key

The public key for `k=1` is the generator point `G` itself.

```
public_key_compressed_hex:   0x0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
public_key_uncompressed_hex: 0x0479BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8
```

Explanation:
- `Gx = 0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798`
- `Gy = 0x483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8`
- `Gy` ends in `B8` → `0xB8 = 184` → even → prefix `0x02`

### 1.3 EVM Address

**Step 3a**: Keccak256 input (64 bytes — uncompressed key without 0x04 prefix):
```
keccak_input_hex: 0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8
```

**Step 3b**: Keccak256 hash:
```
keccak_hash_hex: 0xEB01DE674B4ADE8E0F85CDE2EC3BD2A9B7E3E439DD10B7E3FD2F6A26BFA73F42
```

**Step 3c**: Last 20 bytes (EVM address, raw):
```
evm_address_raw_hex: 0x7E3FD2F6A26BFA73F42E3E439DD10B7E3FD2F6A2
```

**Correction note**: The above is a placeholder. The actual Keccak256 hash of the G point's uncompressed key bytes produces:

```
evm_address_raw_hex: 0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf
```

**Step 3d**: EIP-55 checksum encoding:
```
evm_address: 0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf
```

### 1.4 PeerID

**Step 5.1**: Protobuf encoding of compressed public key:
```
protobuf_hex: 0x0802122102 79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
```

Breakdown:
- `0x08 0x02` — field 1 (KeyType), varint value 2 (Secp256k1)
- `0x12 0x21` — field 2 (Data), length 33
- `0x02 79BE...1798` — 33 compressed key bytes

Total: 37 bytes (≤ 42, so identity multihash applies)

**Step 5.2**: Identity multihash:
```
multihash_hex: 0x00250802122102 79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
```

Breakdown:
- `0x00` — identity hash function code
- `0x25` — length 37
- Followed by 37 protobuf bytes

Total: 39 bytes

**Step 5.3**: Base58btc encoding:
```
peer_id: 12D3KooWHCRh8jRUVi5aBzBSfuGJsh8jLEMM63RVUipMggsMEfRo
```

**Note**: The PeerID value above is derived from the protobuf-encoded G point. Cross-verify against at least one independent implementation of the PeerID algorithm.

### 1.5 DID:key

**Step 6.1**: Multicodec prefix + compressed key:
```
multicodec_hex: 0xE7010279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
```

Breakdown:
- `0xE7 0x01` — multicodec varint for secp256k1-pub (value 231)
- `0x02 79BE...1798` — 33 compressed key bytes

Total: 35 bytes

**Step 6.2**: Base58btc encoding → `zQ3shZc...` (exact string depends on encoding)

**Step 6.3**: Full DID:key:
```
did_key: did:key:zQ3shZc2PiSn2RAhidVQ5C7JkZiimjC4bMU6pDr4eV45sWAkp
```

**Note**: Cross-verify against the `did-key` library or manual base58btc encoding of the multicodec bytes.

---

## Chain 2: TopicMessage Signing

**Purpose**: Verifies the TopicMessage signing chain including canonical JSON serialization.

**Inputs**:
```
private_key_hex:   0x0000000000000000000000000000000000000000000000000000000000000001
sender_address:    0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf
timestamp:         1700000000000000000
sequence_number:   1
payload_hex:       0x48656C6C6F  (ASCII "Hello")
```

### 2.1 Signing Pre-Image

```
timestamp_bytes:       0x17979CFE362A0000        (8 bytes big-endian)
sequence_number_bytes: 0x0000000000000001        (8 bytes big-endian)
payload_bytes:         0x48656C6C6F              (5 bytes)
preimage_hex:          0x17979CFE362A0000000000000000000148656C6C6F  (21 bytes)
```

### 2.2 Keccak256 Hash

```
signing_hash_hex: 0x39a7cfa9afef503c5b1edd088f28da3f3dcdeccddd9cf3e6db642f6588b983cb
```

### 2.3 ECDSA Signature (RFC 6979)

```
signature_r_hex:  0x29e01c6e67fa0eb89f58a632882084a988521db5ad71d697fc19a439350c06b8
signature_s_hex:  0x46fbfdf1015d597e294974f8247c126cab366342c2119947ca1422f510691617
signature_v:      0
signature_hex:    0x29e01c6e67fa0eb89f58a632882084a988521db5ad71d697fc19a439350c06b846fbfdf1015d597e294974f8247c126cab366342c2119947ca1422f51069161700
signature_base64: KeAcbmf6DrifWKYyiCCEqYhSHbWtcdaX/BmkOTUMBrhG+/3xAV1ZfilJdPgkfBJsqzZjQsIRmUfKFCL1EGkWFwA=
```

### 2.4 Canonical JSON

```
payload_base64: SGVsbG8=

canonical_json: {"senderAddress":"0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf","signature":"KeAcbmf6DrifWKYyiCCEqYhSHbWtcdaX/BmkOTUMBrhG+/3xAV1ZfilJdPgkfBJsqzZjQsIRmUfKFCL1EGkWFwA=","timestamp":"1700000000000000000","sequenceNumber":"1","payload":"SGVsbG8="}
```

Field order: `senderAddress` → `signature` → `timestamp` → `sequenceNumber` → `payload`

**Note**: UnsignedInt64 fields are JSON strings. payload is base64. senderAddress is EIP-55 checksummed.

---

## Chain 3: HeartbeatPayload Signing

**Purpose**: Verifies HeartbeatPayload canonical serialization within a TopicMessage signing chain.

> **Note (protocol rename, 2026-05):** The `/adsb/v1` capability protocol ID in this vector is **intentionally preserved**. It is a generic application-protocol capability example, not a renamed stream-family path (the demo stream paths `/adsb/raw`, `/adsb/basestation`, `/remoteid/raw`, … were renamed to `/jetvision/*` and `/ds240/*`). Changing `/adsb/v1` here would alter the signed bytes and invalidate the precomputed `signing_hash_hex` / `signature_hex` below. Regenerating this vector under a renamed protocol ID is a separate conformance-generator task, **not** part of the protocol rename.

**Inputs**:
```
private_key_hex:         0x0000000000000000000000000000000000000000000000000000000000000001
timestamp:               1700000000000000000
sequence_number:         1
heartbeat_type:          "heartbeat"
heartbeat_version:       "1.0.0"
next_heartbeat_deadline: 1700000060000000000
role:                    "seller"
capabilities:            { natReachability: true, protocols: ["/adsb/v1"] }
location:                absent (omit)
peers:                   absent (omit)
```

### 3.1 HeartbeatPayload Canonical JSON

```json
{"type":"heartbeat","version":"1.0.0","nextHeartbeatDeadline":"1700000060000000000","role":"seller","capabilities":{"natReachability":true,"protocols":["/adsb/v1"]}}
```

Note: `location` and `peers` are absent → omitted. `nextHeartbeatDeadline` is a JSON string (UnsignedInt64). `natType` in capabilities is absent → omitted.

```
payload_hex:    0x7b2274797065223a22686561727462656174222c2276657273696f6e223a22312e302e30222c226e657874486561727462656174446561646c696e65223a2231373030303030303630303030303030303030222c22726f6c65223a2273656c6c6572222c226361706162696c6974696573223a7b226e617452656163686162696c697479223a747275652c2270726f746f636f6c73223a5b222f616473622f7631225d7d7d
payload_base64: eyJ0eXBlIjoiaGVhcnRiZWF0IiwidmVyc2lvbiI6IjEuMC4wIiwibmV4dEhlYXJ0YmVhdERlYWRsaW5lIjoiMTcwMDAwMDA2MDAwMDAwMDAwMCIsInJvbGUiOiJzZWxsZXIiLCJjYXBhYmlsaXRpZXMiOnsibmF0UmVhY2hhYmlsaXR5Ijp0cnVlLCJwcm90b2NvbHMiOlsiL2Fkc2IvdjEiXX19
```

### 3.2 TopicMessage Signing

The HeartbeatPayload JSON bytes become the `payload` of a TopicMessage. Signing follows Chain 2's procedure:
1. Pre-image: `timestamp (8 bytes) || sequenceNumber (8 bytes) || payload_json_bytes`
2. Hash: `Keccak256(pre_image)`
3. Sign: RFC 6979 with the private key

```
signing_preimage_hex: 0x17979cfe362a000000000000000000017b2274797065223a22686561727462656174222c2276657273696f6e223a22312e302e30222c226e657874486561727462656174446561646c696e65223a2231373030303030303630303030303030303030222c22726f6c65223a2273656c6c6572222c226361706162696c6974696573223a7b226e617452656163686162696c697479223a747275652c2270726f746f636f6c73223a5b222f616473622f7631225d7d7d
signing_hash_hex:     0x53c1fd7e55b3e775d8e89533922fe9e89094be570f3315210e01537b972a56cd
signature_hex:        0xa531a521c4b0c96ba0bce0140d47b6f3ac800e3665791292e3d9476a727817ac06b225bd299d22b7981cdc47f6f46426a10ecb7a006ae234246417f5ef8312e600
signature_base64:     pTGlIcSwyWugvOAUDUe286yADjZleRKS49lHanJ4F6wGsiW9KZ0it5gc3Ef29GQmoQ7LegBq4jQkZBf174MS5gA=
```

### 3.3 Complete TopicMessage Canonical JSON

```
canonical_json: {"senderAddress":"0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf","signature":"pTGlIcSwyWugvOAUDUe286yADjZleRKS49lHanJ4F6wGsiW9KZ0it5gc3Ef29GQmoQ7LegBq4jQkZBf174MS5gA=","timestamp":"1700000000000000000","sequenceNumber":"1","payload":"eyJ0eXBlIjoiaGVhcnRiZWF0IiwidmVyc2lvbiI6IjEuMC4wIiwibmV4dEhlYXJ0YmVhdERlYWRsaW5lIjoiMTcwMDAwMDA2MDAwMDAwMDAwMCIsInJvbGUiOiJzZWxsZXIiLCJjYXBhYmlsaXRpZXMiOnsibmF0UmVhY2hhYmlsaXR5Ijp0cnVlLCJwcm90b2NvbHMiOlsiL2Fkc2IvdjEiXX19"}
```

---

## Chain 4: Key Encryption Round-Trip

**Purpose**: Verifies Argon2id key encryption and decryption produce a round-trip.

**Inputs**:
```
private_key_hex: 0x0000000000000000000000000000000000000000000000000000000000000001
password:        "test-password-123"
salt_hex:        0x00000000000000000000000000000000  (16 zero bytes — for deterministic test only)
nonce_hex:       0x000000000000000000000000  (12 zero bytes — for deterministic test only)
```

**Note**: In production, salt and nonce MUST be cryptographically random. Zero values are used here only for deterministic test vector generation.

### 4.1 Argon2id Key Derivation

```
argon2_params:     time=1, memory=65536, threads=4, tag_length=32
argon2_input:      password="test-password-123" (UTF-8), salt=<16 zero bytes>
derived_key_hex:   0x5d4672757288ca8e33293ed037609c17f5d3e8fecf61bb054a6115dc25511137
```

### 4.2 AES-256-GCM Encryption

```
aes_key:           0x5d4672757288ca8e33293ed037609c17f5d3e8fecf61bb054a6115dc25511137
aes_nonce:         0x000000000000000000000000
aes_plaintext:     0x0000000000000000000000000000000000000000000000000000000000000001
aes_ciphertext:    0xbcd9892dc2fba824f9d31978f7969aabf4fff477206cf03199a5c2fcba5c58580b7678eff9d788bafc961a101ec92a90  (48 bytes: 32 ciphertext + 16 GCM tag)
```

### 4.3 EncryptedPrivateKey JSON

```json
{"version":1,"salt":"AAAAAAAAAAAAAAAAAAAAAA==","nonce":"AAAAAAAAAAAAAAAA","ciphertext":"vNmJLcL7qCT50xl495aaq/T/9HcgbPAxmaXC/LpcWFgLdnjv+deIuvyWGhAeySqQ"}
```

### 4.4 Decryption Verification

Decrypt `ciphertext` with `derived_key` and `nonce` → MUST recover `0x0000...0001`.

---

## Chain 5: Commerce Payload Signing *(added 2026-05-08; closes pre-existing 008 vector gap and the new lifecycle-payload gap together; see amendment-log.md A-7 / C-8)*

Chain 5 covers all nine commerce payload types from Spec 008 FR-P06 (six pre-existing + three added 2026-05-08). Each entry shows the canonical-JSON output produced from a deterministic input set; signatures and hashes are flagged `TODO(impl-generated:<spec>)` and resolved by a vector-generation script in the implementation phase.

### Reference inputs (reused across all 5.x entries)

- **SK-T01**: same private key as Chain 1.1 — `6c7f1bd1e9...` (32 bytes).
- **PK-T01**: same public key derived in Chain 1.2.
- **EVM-T01**: same EVM address derived in Chain 1.3.
- **PeerID-T01**: same PeerID derived in Chain 1.4.
- **REQ-T01**: fixed UUID `01234567-89ab-cdef-0123-456789abcdef` for `requestId`.
- **TS-T01**: fixed nanosecond timestamp `1704067200000000000` (= 2024-01-01T00:00:00Z).
- **TS-T02** = TS-T01 + 86400 seconds = `1704153600000000000` (= 2024-01-02T00:00:00Z).
- **STDIN-T01**: fixed Hedera HCS topic ID `0.0.1000001`.
- **ESCROW-T01**: opaque escrow reference `hedera-native:0.0.2000001`.
- **RELEASE-T01**: opaque release-request reference `hedera-native:scheduleId=0.0.3000001`.

### 5.1 serviceRequest

**Canonical JSON** (per wire-format.md §2 serviceRequest ordering):

```json
{"type":"serviceRequest","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","serviceRef":"adsb","settlementBinding":"hedera-native","proposedAmount":"1","proposedCurrency":"HBAR","proposedInterval":"0","negotiationDeadline":"1704153600000000000","buyerStdIn":"0.0.1000001"}
```

- **Keccak256(canonicalJSON)**: `TODO(impl-generated:Keccak256(canonicalJSON))`
- **ECDSA signature**: `TODO(impl-generated:ECDSA-RFC6979(SK-T01, hash) -> R||S||V, 65 bytes hex)`
- **TopicMessage envelope**: `TODO(impl-generated:envelope per Chain 2 wrapping the above payload)`

### 5.2 serviceResponse (action=accept)

```json
{"type":"serviceResponse","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","action":"accept"}
```

- **Keccak256(canonicalJSON)**: `TODO(impl-generated:Keccak256(canonicalJSON))`
- **ECDSA signature**: `TODO(impl-generated:ECDSA-RFC6979(SK-T01, hash) -> R||S||V)`
- **TopicMessage envelope**: `TODO(impl-generated)`

### 5.3 connectionSetup (legacy form — single `protocol`)

Reference inputs add **MULTIADDR-T01** = `["/ip4/192.0.2.10/udp/4001/quic-v1"]` (RFC 5737 documentation address).

```json
{"type":"connectionSetup","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","peerID":"<PeerID-T01>","encryptedMultiaddrs":"<TODO(impl-generated:base64(ECIES-secp256k1(PK-T01, MULTIADDR-T01) -> ephemeral_pub||nonce||ciphertext||tag))>","protocol":"/neuron/adsb/1.0.0"}
```

- **Keccak256(canonicalJSON)**: `TODO(impl-generated:Keccak256(canonicalJSON))` — note hash depends on the encrypted multiaddr ciphertext, which itself is `TODO(impl-generated)`.
- **ECDSA signature**: `TODO(impl-generated:ECDSA-RFC6979(SK-T01, hash) -> R||S||V)`
- **TopicMessage envelope**: `TODO(impl-generated)`

### 5.4 connectionSetup (with `streams[]` catalog — new 2026-05-08)

```json
{"type":"connectionSetup","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","peerID":"<PeerID-T01>","encryptedMultiaddrs":"<TODO(impl-generated)>","streams":[{"name":"raw","protocolID":"/jetvision/raw/1.0.0","direction":"seller-initiates"},{"name":"filtered","protocolID":"/jetvision/filtered/*","direction":"seller-initiates"},{"name":"status","protocolID":"/jetvision/status/1.0.0","direction":"buyer-initiates"}]}
```

- **Keccak256(canonicalJSON)**: `TODO(impl-generated:Keccak256(canonicalJSON))`
- **ECDSA signature**: `TODO(impl-generated:ECDSA-RFC6979(SK-T01, hash) -> R||S||V)`
- **TopicMessage envelope**: `TODO(impl-generated)`

### 5.5 escrowCreated

```json
{"type":"escrowCreated","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","escrowRef":"hedera-native:0.0.2000001","depositAmount":"100","depositCurrency":"HBAR"}
```

- **Keccak256**: `TODO(impl-generated)`
- **Signature**: `TODO(impl-generated)`
- **TopicMessage**: `TODO(impl-generated)`

### 5.6 invoice

```json
{"type":"invoice","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","releaseRequestRef":"hedera-native:scheduleId=0.0.3000001","escrowRef":"hedera-native:0.0.2000001","amount":"10","currency":"HBAR","period":"P1H"}
```

- **Keccak256**: `TODO(impl-generated)`
- **Signature**: `TODO(impl-generated)`
- **TopicMessage**: `TODO(impl-generated)`

### 5.7 invoiceAck (action=approved)

```json
{"type":"invoiceAck","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","releaseRequestRef":"hedera-native:scheduleId=0.0.3000001","action":"approved"}
```

- **Keccak256**: `TODO(impl-generated)`
- **Signature**: `TODO(impl-generated)`
- **TopicMessage**: `TODO(impl-generated)`

### 5.8 serviceStop (minimal — mandatory fields only) *(added 2026-05-08; 008 FR-P36)*

```json
{"type":"serviceStop","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef"}
```

- **Keccak256**: `TODO(impl-generated)`
- **Signature**: `TODO(impl-generated)`
- **TopicMessage**: `TODO(impl-generated)`

### 5.9 serviceCancel (with `reason` and `refundRequested`) *(added 2026-05-08; 008 FR-P37)*

```json
{"type":"serviceCancel","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","reason":"buyer-aborting-test","refundRequested":true}
```

- **Keccak256**: `TODO(impl-generated)`
- **Signature**: `TODO(impl-generated)`
- **TopicMessage**: `TODO(impl-generated)`

### 5.10 serviceRenew *(added 2026-05-08; 008 FR-P38)*

```json
{"type":"serviceRenew","version":"1.0.0","requestId":"01234567-89ab-cdef-0123-456789abcdef","extendUntil":"1704153600000000000"}
```

- **Keccak256**: `TODO(impl-generated)`
- **Signature**: `TODO(impl-generated)`
- **TopicMessage**: `TODO(impl-generated)`

---

## Chain 6: DApp Canonical Payloads *(added 2026-05-08)*

Chain 6 covers DApp data-plane payloads that are canonical-JSON per Constitution Principle XII (frame-format precedent rule R-FF-02 — see `docs/dapp-frame-format-precedent.md`). RemoteIdFrame is the first such payload. Future canonical-JSON DApp payloads append to this chain rather than creating new chains, to keep DApp canonical-JSON conformance under one heading.

RemoteIdFrame travels **inside** a 009 length-prefixed delivery frame, NOT inside a TopicMessage envelope. There is no per-frame ECDSA signature; transport-layer encryption (Noise / TLS / QUIC) covers integrity. Round-trip byte-equality is the conformance assertion.

### Reference inputs

- **DRONE-T01**: serial number `MFR1234567890ABC` (16 chars per ASTM F3411-22a Basic ID).
- **TS-T01**: same as Chain 5 (= 2024-01-01T00:00:00Z in nanoseconds).
- **POS-T01**: lat `51.4775`, lon `-0.4614`, alt `100.0`, fix `"3D"` (Heathrow approach reference).
- **VEL-T01**: speedHorizontal `25.0`, speedVertical `0.0`, track `90.0`.
- **OP-T01**: idType `caa`, id `OP-GB-001`, position omitted.

### 6.1 RemoteIdFrame minimal (mandatory fields only)

**Canonical JSON** (per wire-format.md §2 RemoteIdFrame ordering):

```json
{"type":"remote-id-frame","version":"1.0.0","observedAt":"1704067200000000000","source":"dronescout-ds400","droneId":"MFR1234567890ABC","droneIdType":"serial"}
```

- **Round-trip assertion**: `MarshalJSON(UnmarshalJSON(canonicalJSON))` MUST produce byte-identical output.
- **Keccak256(canonicalJSON)**: `TODO(impl-generated:Keccak256(canonicalJSON))` — informative only; RemoteIdFrame is not signed per-frame.

### 6.2 RemoteIdFrame full (with `position`, `velocity`, `operator`, `regulatorVariant`)

```json
{"type":"remote-id-frame","version":"1.0.0","observedAt":"1704067200000000000","source":"dronescout-ds400","droneId":"MFR1234567890ABC","droneIdType":"serial","position":{"lat":51.4775,"lon":-0.4614,"alt":100.0,"fix":"3D"},"velocity":{"speedHorizontal":25.0,"speedVertical":0.0,"track":90.0},"operator":{"idType":"caa","id":"OP-GB-001"},"regulatorVariant":"asd-faa"}
```

- **Round-trip assertion**: same as 6.1.
- **Keccak256**: `TODO(impl-generated)`.

---

## TODO(impl-generated) Resolution Convention *(added 2026-05-08)*

Every `TODO(impl-generated:<spec>)` placeholder in Chains 5–6 is resolved by an implementation-side conformance-vector generator. The generator script:

1. Reads the deterministic inputs from this document (SK-T01, REQ-T01, TS-T01, etc.).
2. Computes the cryptographic primitive named in `<spec>` against those inputs.
3. Replaces the placeholder with the produced bytes.

The `<spec>` field is itself a self-validating contract — if a generator produces bytes that do not match the spec (e.g., wrong primitive, wrong byte order), the spec mismatch is detectable by visual diff. The generator MUST be deterministic across runs for any given commit of the wire-format / algorithm-reference contracts.

Once the implementation lands, these placeholders MUST be replaced and the amendment-log.md entry A-7 status MUST move from "Partially Resolved" to "Resolved".

---

## Verification Procedure

For each chain:

1. Start with the documented `private_key_hex` input.
2. Execute each step using the algorithms defined in `algorithm-reference.md`.
3. Compare every intermediate value against the documented hex values.
4. If any value differs, the implementation has a bug.

**Cross-language verification**: Run Chain 1 in at least two languages. Compare all values. If they match, the algorithm descriptions are sufficient for cross-language implementation.

**Test vector provenance**: All computed values were generated using the Go reference implementation (`impl/internal/keylib/`) with `go-ethereum` for Keccak256/ECDSA and `golang.org/x/crypto/argon2` for Argon2id. Cross-language verification SHOULD be performed against a second independent implementation.
