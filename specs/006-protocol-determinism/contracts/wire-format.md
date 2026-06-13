# Contract: Wire Format Specification

**Spec**: 006-protocol-determinism | **Date**: 2026-03-03
**Scope**: Normative JSON encoding rules for all serialized structures across the Neuron SDK protocol (specs 001–010)
**Resolves**: Audit items B-1, B-2, B-3, B-4, B-5, A-4, A-5, C-2, D-2, D-3

---

## 1. Global JSON Rules (FR-W01, FR-W08)

All JSON serialization across the Neuron protocol MUST conform to these rules:

1. **Compact format**: No whitespace between tokens. No space after `:` or `,`. No trailing commas.
2. **Character encoding**: UTF-8 (no BOM, no UTF-16, no UTF-32).
3. **No trailing newline**: The JSON string ends with the closing `}` or `]` — no `\n` appended.
4. **String escaping**: Per RFC 8259 Section 7. Mandatory escapes: `"` → `\"`, `\` → `\\`, control characters U+0000–U+001F → `\uXXXX`. Characters U+0020 and above MAY be represented literally (UTF-8 bytes) or escaped. For determinism, implementations SHOULD emit non-ASCII characters as literal UTF-8 bytes (not `\uXXXX` escapes) unless they are control characters.
5. **Key quoting**: All JSON object keys MUST be double-quoted strings.

**Example** (correct):
```
{"senderAddress":"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed","timestamp":"1700000000000000000"}
```

**Example** (incorrect — has whitespace):
```
{ "senderAddress": "0x5aAe...", "timestamp": "1700000000000000000" }
```

---

## 2. Canonical Field Ordering (FR-W05)

JSON objects MUST emit keys in the canonical order defined per entity. Implementations MUST NOT rely on hash-map insertion order or alphabetical sorting. The canonical order is:

### TopicMessage

`senderAddress` → `signature` → `timestamp` → `sequenceNumber` → `payload`

### HeartbeatPayload

`type` → `version` → `nextHeartbeatDeadline` → `role` → `capabilities`* → `location`* → `peers`*

(*optional — omit key if absent)

### Capabilities

`natReachability` → `natType`* → `protocols`*

### Location

`lat` → `lon` → `alt`* → `fix`*

### EncryptedPrivateKey

`version` → `salt` → `nonce` → `ciphertext` → `time`* → `memory`* → `threads`*

(*v2 only — omit for v1)

**Implementation guidance**: Languages without ordered maps (e.g., Python dicts pre-3.7, JavaScript objects) MUST use an ordered serialization approach — either an ordered collection type or a custom serializer that emits keys in the defined order.

### Commerce Payloads (Spec 008 FR-P12) — added 2026-05-08

The following nine canonical orderings are derived verbatim from Spec 008 FR-P12 and FR-P36/P37/P38. They were originally defined in 008 but never propagated into 006; the 2026-05-08 amendment closes that gap. See amendment-log.md audit ID **A-7** (this amendment) and **C-8** (the pre-existing gap subsumed by A-7).

#### serviceRequest

`type` → `version` → `requestId` → `serviceRef` → `settlementBinding` → `proposedAmount` → `proposedCurrency` → `proposedInterval` → `serviceParams`* → `negotiationDeadline` → `arbiter`* → `buyerStdIn`

(*optional — omit key if absent. `serviceParams` keys MUST be sorted lexicographically per 008 FR-P07.)

#### serviceResponse

`type` → `version` → `requestId` → `action` → `counterAmount`* → `counterInterval`*

(*present only when `action` is `counter`.)

#### connectionSetup

`type` → `version` → `requestId` → `peerID` → `encryptedMultiaddrs` → `protocol`* → `streams`* → `natStatus`*

(*optional — at least one of `protocol` or `streams` MUST be present per 008 FR-P33; both MAY be present per FR-P33a back-compat. `natStatus` is independently optional.)

#### escrowCreated

`type` → `version` → `requestId` → `escrowRef` → `depositAmount` → `depositCurrency`

#### invoice

`type` → `version` → `requestId` → `releaseRequestRef` → `escrowRef` → `amount` → `currency` → `period`

#### invoiceAck

`type` → `version` → `requestId` → `releaseRequestRef` → `action` → `depositedMore`* → `newBalance`*

(*optional — omit key if absent.)

#### serviceStop *(added 2026-05-08; 008 FR-P36)*

`type` → `version` → `requestId` → `reason`* → `effectiveAt`*

(*optional — omit key if absent.)

#### serviceCancel *(added 2026-05-08; 008 FR-P37)*

`type` → `version` → `requestId` → `reason`* → `refundRequested`*

(*optional — omit key if absent.)

#### serviceRenew *(added 2026-05-08; 008 FR-P38)*

`type` → `version` → `requestId` → `extendUntil` → `reason`*

(*optional — omit key if absent.)

### StreamCatalogEntry *(added 2026-05-08; 008 FR-P33a)*

A nested object inside `connectionSetup.streams[]`.

`name` → `protocolID` → `direction` → `schema`*

(*optional — omit key if absent.)

### RemoteIdFrame *(added 2026-05-08; 017 FR-R05 — DApp canonical payload)*

The data-plane payload for `/ds240/raw/1.0.0` and the filtered variants. RemoteIdFrame travels INSIDE a 009 length-prefixed delivery frame, not inside a TopicMessage envelope; signing is implicit in the underlying QUIC/TLS transport rather than per-frame ECDSA.

`type` → `version` → `observedAt` → `source` → `droneId` → `droneIdType` → `position`* → `velocity`* → `operator`* → `regulatorVariant`*

(*optional — omit key if absent. Nested objects use their own canonical orderings: `position` per Location above, `velocity` is `speedHorizontal` → `speedVertical` → `track`, `operator` is `idType` → `id` → `position`*.)

**Note on canonicalization scope**: 006 governs the canonical ordering of any payload whose round-trip byte-equality matters for cross-language interoperability. Even DApp payloads (per Constitution Principle XII) follow 006 canonical-JSON discipline when they travel as JSON; this is a Core SDK rule that DApps consume rather than redefine.

---

## 3. UnsignedInt64 Encoding (FR-W02)

**Rule**: All UnsignedInt64 fields MUST be encoded as **JSON strings** containing the decimal representation.

**Applies to**:
- `TopicMessage.timestamp`
- `TopicMessage.sequenceNumber`
- `HeartbeatPayload.nextHeartbeatDeadline`
- Protocol constants when serialized (e.g., `MIN_DEADLINE_DELTA`)

**Format**:
- Decimal digits only (`0-9`)
- No leading zeros except for the value `"0"` itself
- No sign prefix (UnsignedInt64 is unsigned)
- Enclosed in JSON double quotes

**Examples**:
| Value | Correct JSON | Incorrect JSON |
|-------|-------------|---------------|
| 0 | `"0"` | `0`, `"00"` |
| 42 | `"42"` | `42`, `"042"` |
| 1700000000000000000 | `"1700000000000000000"` | `1700000000000000000` (precision loss in JS) |
| 2^64 - 1 | `"18446744073709551615"` | `18446744073709551615` |

**Rationale** (Research R2): JavaScript `JSON.parse` loses precision for integers above 2^53. Neuron timestamps (nanoseconds since epoch) are ~1.7×10^18, well above this threshold.

---

## 4. Binary Encoding (FR-W03)

**Rule**: All binary fields (ByteArray) MUST be encoded as **JSON strings** using RFC 4648 Section 4 standard base64 encoding.

**Applies to**:
- `TopicMessage.signature` (65 bytes → 88 base64 characters)
- `TopicMessage.payload` (variable length)
- `EncryptedPrivateKey.salt` (16 bytes → 24 base64 characters)
- `EncryptedPrivateKey.nonce` (12 bytes → 16 base64 characters)
- `EncryptedPrivateKey.ciphertext` (48 bytes → 64 base64 characters)

**Alphabet**: `ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/`

**Padding**: `=` padding MUST be used. Input lengths not divisible by 3 are padded to the next multiple.

| Input length mod 3 | Padding |
|--------------------:|---------|
| 0 | No padding |
| 1 | `==` |
| 2 | `=` |

**Empty input**: Zero-length byte arrays encode to empty string `""`.

**Decoding**: Decoders MUST reject non-base64 characters. Decoders SHOULD accept inputs with or without padding for robustness, but encoders MUST always produce padded output.

---

## 5. Optional Field Handling (FR-W04)

**Rule**: Optional fields that are absent MUST be **omitted** from the JSON object entirely. They MUST NOT be serialized as `null`.

**Applies to**:
- `HeartbeatPayload.capabilities` (SHOULD)
- `HeartbeatPayload.location` (MAY)
- `HeartbeatPayload.peers` (MAY)
- `Capabilities.natType` (MAY)
- `Capabilities.protocols` (SHOULD)
- `Location.alt` (MAY)
- `Location.fix` (MAY)
- `NeuronAccount.ledgerAttachment` (MAY)
- `NeuronAccount.registryBinding` (Child MUST, others N/A)
- `NeuronAccount.feePayer` (MAY)
- Any BigInteger (optional) field: `creditBalance`, `balanceAllocation`, `balance`
- `EncryptedPrivateKey.time`, `.memory`, `.threads` (v2 only)

**Examples**:

Correct (location absent):
```json
{"type":"heartbeat","version":"1.0.0","nextHeartbeatDeadline":"1700000060000000000","role":"seller","capabilities":{"natReachability":true}}
```

Incorrect (location present as null):
```json
{"type":"heartbeat","version":"1.0.0","nextHeartbeatDeadline":"1700000060000000000","role":"seller","capabilities":{"natReachability":true},"location":null}
```

**Edge case**: If an optional struct field is present but all its sub-fields are at their zero/default values, the field MUST still be serialized with those zero values. Only omit the top-level key when the field itself is absent.

---

## 6. EVM Address Encoding (FR-W06, FR-W09)

**Rule**: `EVMAddress` values in JSON MUST use EIP-55 mixed-case checksum encoding with `0x` prefix.

**Applies to**:
- `TopicMessage.senderAddress`
- `NeuronAccount.evmAddress`
- `LedgerAttachment.attachedAddress`
- Any field typed as `EVMAddress`

**Format**: `0x` followed by 40 hex characters with EIP-55 mixed-case checksumming (algorithm defined in FR-A04 / `algorithm-reference.md` §4).

**Example**: `"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"` (not `"0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"`)

**Parsing**: Implementations accepting EVM addresses MUST accept both lowercase and EIP-55 checksummed inputs. If checksummed input is provided, the checksum SHOULD be verified and a warning/error raised on mismatch.

---

## 7. Arbitrary-Precision Integer Encoding (FR-W07)

**Rule**: Arbitrary-precision integers (`big.Int` / equivalent) MUST be serialized as **JSON strings** containing the decimal representation.

**Applies to**:
- `NeuronAccount.creditBalance`
- `NeuronAccount.balanceAllocation`
- `NeuronAccount.balance`

**Format**:
- Decimal digits only
- No leading zeros except for `"0"`
- Negative values: prefix `-` (e.g., `"-100"`)
- Absent: omit the key entirely (per FR-W04)

**Examples**:
| Value | JSON |
|-------|------|
| 0 | `"0"` |
| 1000000000000000000 (1 ETH in wei) | `"1000000000000000000"` |
| absent | *(key omitted)* |

---

## 8. Floating-Point Encoding (FR-W10)

**Rule**: Float64 values MUST be serialized as **JSON numbers** with up to 15 significant digits.

**Applies to**:
- `Location.lat`
- `Location.lon`
- `Location.alt`

**Format**:
- No trailing zeros after the decimal point, except that integer-valued floats MUST include `.0` (e.g., `37.0` not `37`)
- No exponential/scientific notation for values representable in fixed-point with ≤ 15 digits
- `NaN` and `Infinity` are invalid — implementations MUST reject them

**Examples**:
| Value | Correct JSON | Incorrect JSON |
|-------|-------------|---------------|
| 37.7749 | `37.7749` | `37.77490` (trailing zero) |
| -122.4194 | `-122.4194` | |
| 10.0 | `10.0` | `10` (missing decimal), `1e1` (exponential) |
| 0.0 | `0.0` | `0` |

---

## 9. UTF-8 Mandate (C-2)

**Rule**: All string-to-bytes conversions throughout the Neuron protocol MUST use UTF-8 encoding.

This includes:
- JSON serialization output (Section 1)
- Pre-image construction for hashing (before Keccak256)
- Mnemonic word strings for BIP-39 (FR-A12)
- EIP-55 checksum: the lowercase hex string is hashed as ASCII bytes (which are a subset of UTF-8)
- DID:key string components

Implementations in languages with UTF-16 internal string representation (Java, C#, JavaScript, Dart) MUST explicitly encode to UTF-8 bytes before any hashing or byte-level operation.
