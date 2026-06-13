# Data Model: Protocol Determinism

**Branch**: `006-protocol-determinism` | **Date**: 2026-03-03 | **Source**: spec.md FR-W01..FR-X04, Key Entities

---

## Primitive Type Encoding Table

This table defines how every primitive type used across specs 001–005 MUST be encoded in JSON and in byte representations (for hashing and signing).

These are **protocol-level type names** used throughout the spec set. They are not tied to any language's type system.

| Protocol Type | JSON Encoding | Byte Representation | Notes | Source FR |
|---------------|--------------|---------------------|-------|----------|
| `UnsignedInt64` | JSON string containing decimal digits | 8 bytes big-endian | No leading zeros except `"0"`. Example: `"1700000000000000000"` | FR-W02 |
| `ByteArray` | JSON string, RFC 4648 §4 base64 (`A-Za-z0-9+/=`) | Raw bytes | Standard alphabet with `=` padding | FR-W03 |
| `EVMAddress` | JSON string, EIP-55 mixed-case hex with `0x` prefix | 20 bytes | Example: `"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"` | FR-W06 |
| `BigInteger` | JSON string containing decimal digits | Variable-length big-endian | No leading zeros except `"0"`. Absent → omit key | FR-W07 |
| `String` | JSON string, UTF-8 | UTF-8 bytes | Escaping per RFC 8259 §7 | FR-W08 |
| `Boolean` | JSON `true` or `false` | 1 byte: `0x00` (false) / `0x01` (true) | | FR-W01 |
| `Float64` | JSON number, up to 15 significant digits | IEEE 754 double, 8 bytes big-endian | Integer-valued floats include `.0` (e.g., `37.0`) | FR-W10 |
| `Optional<T>` | Omit key if absent | — | Never serialize as `null` | FR-W04 |
| `Enum(String)` | JSON string | UTF-8 bytes of the enum value | Values defined per spec (e.g., `"buyer"`, `"seller"`, `"relay"`) | FR-W01 |
| `UnsignedInt8` | JSON number | 1 byte | Used in EncryptedPrivateKey version/threads | FR-W01 |
| `UnsignedInt32` | JSON number | 4 bytes big-endian | Used in EncryptedPrivateKey time/memory | FR-W01 |

---

## Canonical Field Ordering

JSON objects MUST emit keys in the exact order listed below. This order is defined by the canonical field order specified in this section.

### TopicMessage (Spec 004)

Source: `specs/004-topic-system/data-model.md`

```json
{
  "senderAddress": "<EIP-55 checksummed address>",
  "signature": "<base64 R||S||V 65 bytes>",
  "timestamp": "<UnsignedInt64 decimal string>",
  "sequenceNumber": "<UnsignedInt64 decimal string>",
  "payload": "<base64 raw bytes>"
}
```

Field order: `senderAddress` → `signature` → `timestamp` → `sequenceNumber` → `payload`

All fields are required. No optional fields.

### HeartbeatPayload (Spec 005)

Source: `specs/005-health/data-model.md`

```json
{
  "type": "heartbeat",
  "version": "<semver string>",
  "nextHeartbeatDeadline": "<UnsignedInt64 decimal string>",
  "role": "<buyer|seller|relay>",
  "capabilities": { ... },
  "location": { ... },
  "peers": ["<4-char hex>", ...]
}
```

Field order: `type` → `version` → `nextHeartbeatDeadline` → `role` → `capabilities` (optional, omit if absent) → `location` (optional, omit if absent) → `peers` (optional, omit if absent)

### Capabilities (nested in HeartbeatPayload)

```json
{
  "natReachability": true,
  "natType": "<nat-type-string>",
  "protocols": ["<protocol-id>", ...]
}
```

Field order: `natReachability` → `natType` (optional, omit if absent) → `protocols` (optional, omit if absent)

### Location (nested in HeartbeatPayload)

```json
{
  "lat": 37.7749,
  "lon": -122.4194,
  "alt": 10.0,
  "fix": "3D"
}
```

Field order: `lat` → `lon` → `alt` (optional, omit if absent) → `fix` (optional, omit if absent)

### EncryptedPrivateKey (Spec 002)

Source: `specs/002-key-library/data-model.md`

```json
{
  "version": 1,
  "salt": "<base64 16 bytes>",
  "nonce": "<base64 12 bytes>",
  "ciphertext": "<base64 48 bytes>",
  "time": 1,
  "memory": 65536,
  "threads": 4
}
```

Field order: `version` → `salt` → `nonce` → `ciphertext` → `time` (v2 only, omit for v1) → `memory` (v2 only, omit for v1) → `threads` (v2 only, omit for v1)

Note: For v1, `time`, `memory`, and `threads` are omitted because they use hardcoded defaults (FR-A11).

---

## Implementation Type Mapping Table (FR-X02)

Protocol-level types and their implementation equivalents across common languages.

| Protocol Type | Used In | Protocol Definition | Implementation Examples | Source |
|---------------|---------|--------------------|-----------------------|--------|
| `AsyncStream<MessageDelivery>` | 004 TopicAdapter.Subscribe | An asynchronous stream/iterator of MessageDelivery objects. The stream delivers items one at a time and blocks when no items are available. The stream is closed when the subscription ends or a permanent error occurs. | Go: `<-chan`, Python/JS: async iterator, Rust: `mpsc::Receiver`, Java: `Stream`, Swift: `AsyncSequence` | D-1 |
| `JSONObject` | 004 Config | A JSON object: a key-value map where keys are strings and values are strings, numbers, booleans, arrays, or nested objects. | Go: `map[string]interface{}`, Python: `dict`, JS: `object`, Rust/Java: `HashMap` | D-2 |
| `BigInteger (optional)` | 001 creditBalance, balanceAllocation, balance | An optional arbitrary-precision integer. When absent, the value has not been set. JSON encoding: decimal string or omit if absent. | Go: `*big.Int`, Java: `BigInteger`, Python: `int`, JS: `BigInt`, Rust: `num::BigInt` | D-3 |
| `Secp256k1Key` (accepts raw bytes or typed key) | 002 ToBlockchainKey/FromBlockchainKey | The accepted input is a raw 32-byte secp256k1 private key scalar, or a typed ECDSA private key object containing a secp256k1 scalar. Only secp256k1 keys are accepted; Ed25519 and other curves MUST be rejected. | Each language should accept its native ECDSA private key type or raw bytes. | D-4 |
| `Error` | ALL return types | A structured error object with: `code` (string, from the error taxonomy), `message` (human-readable string), and optionally `cause` (wrapped underlying error). | Go: `error` interface, Java/Python: `Exception`, JS: `Error` class, Rust: `Result::Err` | D-5 |
| `Options` (named-field object) | 004 TopicAdapter methods | An options object with named fields. Each field has a defined type and default value. The field names, types, and defaults are defined in spec 004's contracts. | Go/Rust/C: struct, Java/Python/JS: class, or builder pattern | D-6 |

---

## Test Vector Data Structure

Each golden test vector chain follows this schema:

### Key Derivation Chain

```
label:                     "<human-readable name>"
private_key_hex:           "0x<64 hex chars>"
public_key_compressed_hex: "0x<66 hex chars>"
public_key_uncompressed_hex: "0x<130 hex chars>"
evm_address:               "0x<40 hex chars, EIP-55>"
evm_address_keccak_input:  "0x<128 hex chars>"
evm_address_keccak_hash:   "0x<64 hex chars>"
peer_id_protobuf_hex:      "0x<74 hex chars>"
peer_id_multihash_hex:     "0x<78 hex chars>"
peer_id:                   "<base58btc string>"
did_key_multicodec_hex:    "0x<70 hex chars>"
did_key:                   "did:key:z<base58btc string>"
```

### Signing Chain

```
label:                     "<human-readable name>"
private_key_hex:           "0x<64 hex chars>"
sender_address:            "0x<40 hex chars, EIP-55>"
timestamp:                 <UnsignedInt64 value>
sequence_number:           <UnsignedInt64 value>
payload_hex:               "0x<hex>"
payload_base64:            "<base64 string>"
signing_preimage_hex:      "0x<hex of timestamp(8) || seqnum(8) || payload(N)>"
signing_hash_hex:          "0x<64 hex chars, Keccak256 of preimage>"
signature_r_hex:           "0x<64 hex chars>"
signature_s_hex:           "0x<64 hex chars>"
signature_v:               <0 or 1>
signature_hex:             "0x<130 hex chars, R||S||V>"
signature_base64:          "<base64 of 65 bytes>"
canonical_json:            "<exact JSON string>"
canonical_json_hex:        "0x<hex of UTF-8 bytes>"
```

---

## Entities

### WireFormatRule

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Rule identifier (e.g., `WF-UINT64`, `WF-BASE64`) |
| `scope` | string[] | List of entities and fields this rule applies to |
| `encoding` | string | The encoding specification |
| `rationale` | string | Why this encoding was chosen |
| `audit_items` | string[] | Audit items this rule resolves (e.g., `B-1`, `A-5`) |

### AlgorithmDefinition

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Algorithm identifier (e.g., `ALG-PEERID`, `ALG-EIP55`) |
| `name` | string | Human-readable name |
| `inputs` | InputSpec[] | Typed input parameters |
| `steps` | string[] | Ordered step-by-step procedure at the byte level |
| `outputs` | OutputSpec[] | Typed output values |
| `edge_cases` | string[] | Known edge cases and how to handle them |
| `test_vector_ref` | string | Reference to test vector chain that exercises this algorithm |
| `audit_items` | string[] | Audit items this algorithm resolves |

### TestVectorChain

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Chain identifier (e.g., `TV-KEYDER-01`, `TV-TOPICSIGN-01`) |
| `label` | string | Human-readable description |
| `type` | string | One of: `key-derivation`, `topic-signing`, `heartbeat-signing`, `encryption` |
| `values` | ordered dictionary (String → String) | Ordered map of intermediate value names to hex/encoded values |
| `verified_against` | string[] | Libraries used to cross-verify (e.g., `ethereumjs-util 7.x`, `eth_keys 0.5.0`, `ethers-rs 2.x`) |

### ErrorDefinition

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Unique error code (e.g., `NEURON-KEY-001`) |
| `name` | string | Error name (e.g., `InvalidFormat`) |
| `category` | string | Spec domain: `KEY`, `ACCOUNT`, `REGISTRY`, `TOPIC`, `HEALTH`, `WIRE` |
| `description` | string | Human-readable description |
| `trigger` | string | Condition that triggers this error |
| `recommended_action` | string | What the caller should do |
| `source_spec` | string | Original spec (e.g., `002-key-library`) |
| `source_fr` | string | Original functional requirement (e.g., `FR-008`) |

### AmendmentEntry

| Field | Type | Description |
|-------|------|-------------|
| `audit_id` | string | Audit item identifier (e.g., `A-1`, `B-3`, `C-5`) |
| `category` | string | Audit category (A/B/C/D/X) |
| `description` | string | Brief description of the gap |
| `resolved_by` | string | 006 artifact and section (e.g., `contracts/algorithm-reference.md §5`) |
| `status` | string | `Resolved`, `Partially Resolved`, or `Deferred` |
| `justification` | string | For Deferred: why and when it will be resolved |
