# Feature Specification: Protocol Determinism (Wire Format, Algorithms & Test Vectors)

**Feature Branch**: `006-protocol-determinism`
**Created**: 2026-03-03
**Status**: Draft

## Related Specs

- **[001 NeuronAccount Module](../001-neuron-account-module/spec.md)** — DID document structure, arbitrary-precision integer serialization, account identity model
- **[002 Key Library](../002-key-library/spec.md)** — Key derivation algorithms, EIP-55, PeerID, DID:key, Argon2id, RFC 6979
- **[003 Peer Registry (EIP-8004)](../003-peer-registry/spec.md)** — Registration interface, agentURI, identity-to-registration bridge
- **[004 Topic System](../004-topic-system/spec.md)** — TopicMessage wire format, JSON serialization, signing, transport
- **[005 Health](../005-health/spec.md)** — HeartbeatPayload wire format, liveness model, health status
- **[007 Identity Registry Smart Contract](../007-identity-contract/spec.md)** — On-chain contract layer; registration bridge target for 001 ↔ 003 ↔ 007 mapping (FR-X04)

## Purpose

This spec is a **normative companion** to specs 001–005. It does not replace or rewrite them. It provides the missing layer that makes the existing specs machine-deterministic for any programming language:

1. **Wire format rules** — Exact JSON encoding for every serialized structure (field ordering, null handling, numeric encoding, binary encoding, string encoding)
2. **Byte-level algorithm descriptions** — Every cryptographic and encoding operation described step-by-step in bytes, replacing Go library references
3. **Golden test vectors** — Concrete hex values through the full derivation and signing chains, verifiable by any implementation
4. **Unified error taxonomy** — Cross-spec error code namespace with structured error definitions
5. **Cross-spec resolution** — Explicit answers to contradictions and ambiguities between specs

After this spec is complete, the full set (001–006) becomes a self-contained protocol definition that any developer — or a basic LLM — can use to build a complete, interoperable Neuron SDK in any programming language, without Go knowledge, Go source code, or external references.

## Clarifications

### Session 2026-05-08

- Q: 008 added 9 commerce payload types (6 original + 3 new lifecycle: serviceStop, serviceCancel, serviceRenew) and 008 FR-P33a introduces a `streams[]` catalog inside `connectionSetup`. 017 introduces `RemoteIdFrame` as a normalized canonical-JSON DApp payload. None of these have canonical-JSON orderings or test vectors in 006. How is the gap closed? → A: In one amendment cycle (audit IDs **A-7** new + **C-8** subsumed pre-existing). `wire-format.md` §2 receives 11 new canonical orderings (9 commerce payloads + StreamCatalogEntry + RemoteIdFrame). `test-vectors.md` receives Chain 5 (Commerce Payload Signing — 10 entries) and Chain 6 (DApp Canonical Payloads — 2 entries). Each entry uses the deterministic inputs already established in Chain 1 (SK-T01, etc.) plus new fixed identifiers (REQ-T01, TS-T01, etc.). Signatures and hashes are flagged `TODO(impl-generated:<spec>)` and resolved by an impl-side conformance-vector generator. The `<spec>` in each placeholder is itself a self-validating contract — the generator either matches the spec or fails with a visible diff.
- Q: Does 006 take responsibility for DApp canonical payloads (like RemoteIdFrame) even though they belong to DApp specs (017) per Constitution Principle XII? → A: Yes. 006 governs canonical-JSON discipline for ANY payload that needs cross-language interoperability — including DApp payloads. Principle XII separates *who chooses the format* (DApp choice — see `docs/dapp-frame-format-precedent.md` rules R-FF-01 / R-FF-02) from *how the format is canonicalized once chosen* (006 rules). DApps following R-FF-02 (normalized canonical JSON) consume 006's canonical-JSON rules unmodified. DApps following R-FF-01 (opaque pass-through) bypass 006 — their payload is opaque bytes inside a 009 length-prefixed frame, no JSON envelope.
- Q: When does Chain 5 / Chain 6 status move from "Partially Resolved" to "Resolved"? → A: When the impl-side conformance-vector generator produces the actual byte values and a maintainer commits them in place of the `TODO(impl-generated)` placeholders. The generator is implementation work; this amendment ratifies the contract for the generator. Per the resolution convention documented in `test-vectors.md`, the generator MUST be deterministic across runs for any given commit of the wire-format / algorithm-reference contracts.

## Out of Scope

- **Rewriting specs 001–005**: This spec augments, not replaces. Existing FRs remain authoritative for their domains.
- **Implementation code**: This spec defines protocol, not code. No source files are produced.
- **Transport-layer encoding**: How adapters encode messages for specific backends (HCS protobuf, ERC event encoding) is defined in spec 004 adapter contracts, not here.
- **Application-layer semantics**: What heartbeat payloads mean, how liveness is computed, account lifecycle — these remain in their respective specs.

---

## User Scenarios & Testing

### User Story 1 — Wire Format Implementation (Priority: P1)

A developer implementing JSON serialization for TopicMessage and HeartbeatPayload needs unambiguous encoding rules. The developer reads the wire format contract and produces byte-identical JSON output to any other conforming implementation without consulting any specific SDK's source code.

**Why this priority**: Wire format is the foundation — if two implementations serialize differently, signatures won't verify across languages. This blocks all interop.

**Independent Test**: Serialize a TopicMessage with known field values using only the wire format rules in this spec. Compare the output bytes to the golden test vector. They MUST match exactly.

**Acceptance Scenarios**:

1. **Given** the wire format contract and a TopicMessage with fields `{senderAddress: "0x5aAe...", signature: <65 bytes>, timestamp: 1700000000000000000, sequenceNumber: 42, payload: <bytes>}`, **When** a TypeScript developer serializes to JSON following only the rules in this spec, **Then** the output is byte-identical to the golden test vector JSON string.
2. **Given** a HeartbeatPayload with optional fields `capabilities` present and `location` absent, **When** serialized to JSON, **Then** the `location` key is omitted (not set to `null`) and the remaining fields appear in the canonical order defined in this spec.
3. **Given** an UnsignedInt64 field `timestamp` with value `1700000000000000000` (above 2^53), **When** encoded to JSON, **Then** the output is a JSON string `"1700000000000000000"` (not a JSON number).

---

### User Story 2 — Cryptographic Algorithm Implementation (Priority: P1)

A developer implementing key derivation in Rust needs byte-level algorithms for PeerID derivation, EIP-55 checksumming, DID:key construction, and ECDSA signing. The developer reads the algorithm reference contract and implements each algorithm step-by-step without consulting any Go library source code or external standard documents.

**Why this priority**: Algorithms are the second foundation — if PeerID derivation produces different bytes, peer discovery fails silently.

**Independent Test**: Start with a known private key hex string. Derive public key, EVM address, PeerID, and DID:key using only the algorithm steps in this spec. Compare each intermediate and final value to the golden test vector. They MUST match exactly.

**Acceptance Scenarios**:

1. **Given** private key `0x...` (from test vector), **When** a Rust developer follows the PeerID derivation algorithm in this spec (protobuf wrap → multihash → identity-vs-SHA256 → base58btc), **Then** the result matches the PeerID in the test vector.
2. **Given** an EVM address in lowercase hex, **When** the EIP-55 algorithm in this spec is applied, **Then** the mixed-case output matches the test vector.
3. **Given** a message hash and private key, **When** RFC 6979 deterministic signing is applied per this spec, **Then** signing the same hash twice produces byte-identical 65-byte R||S||V signatures.

---

### User Story 3 — Cross-Language Interoperability Verification (Priority: P1)

A QA engineer or automated test suite needs to verify that an SDK implementation in language X produces outputs interoperable with any other conforming implementation. The engineer runs the golden test vectors from this spec against the new implementation, verifying correctness by the test vectors alone.

**Why this priority**: Test vectors are the verification mechanism — without them, interop is unprovable.

**Independent Test**: Feed the test vector private key into the SDK, produce all derived values and signed messages, compare byte-for-byte against the expected values in this spec.

**Acceptance Scenarios**:

1. **Given** golden test vector Chain 1 (key derivation), **When** a Python SDK derives all values from the test private key, **Then** every intermediate value (compressed pubkey, uncompressed pubkey, EVM address, PeerID, DID:key) matches the test vector hex exactly.
2. **Given** golden test vector Chain 2 (TopicMessage signing), **When** a Python SDK constructs and signs a TopicMessage with the test vector inputs, **Then** the canonical JSON bytes and signature match the test vector exactly.
3. **Given** golden test vector Chain 3 (HeartbeatPayload signing), **When** a Python SDK constructs and signs a HeartbeatPayload with the test vector inputs, **Then** the canonical JSON bytes and signature match the test vector exactly.

---

### User Story 4 — Error Handling Implementation (Priority: P2)

A developer implementing error handling across the SDK needs a unified error taxonomy that works consistently across all modules (key library, account, registry, topic, health). The developer reads the error taxonomy contract and implements structured error types that are compatible across SDK boundaries.

**Why this priority**: Without unified errors, each module invents its own error scheme — cross-module error propagation becomes unpredictable.

**Independent Test**: Trigger each error condition listed in the taxonomy (e.g., invalid key type, unsigned message, deadline violation) and verify the error code, name, and category match the spec.

**Acceptance Scenarios**:

1. **Given** the error taxonomy, **When** a developer passes an Ed25519 key to a function expecting secp256k1, **Then** the error code is `NEURON-KEY-002` (InvalidKeyType) with the description from the taxonomy.
2. **Given** a TopicMessage with an invalid signature, **When** the adapter validates it, **Then** the error code is `NEURON-TOPIC-003` (InvalidSignature).

---

### User Story 5 — Cross-Spec Integration (Priority: P2)

A developer implementing the full SDK end-to-end needs clear answers to contradictions and ambiguities between specs 001–005. The developer reads the cross-spec resolution section and implements the resolved behavior without guessing.

**Why this priority**: Unresolved contradictions (e.g., who signs the TopicMessage?) block full-stack implementation.

**Independent Test**: Implement the health publisher and topic adapter following the resolved signing responsibility. Verify that a heartbeat published by the health module is accepted by the topic adapter without error.

**Acceptance Scenarios**:

1. **Given** the signing responsibility resolution (caller signs, adapter validates), **When** the health publisher constructs a heartbeat, **Then** it signs the TopicMessage before calling `Publish()`, and the adapter validates the existing signature without re-signing.
2. **Given** the registration bridge resolution (001 ↔ 003), **When** a developer creates a NeuronAccount and wants to register it, **Then** the spec clearly defines which account fields map to which registry inputs.

---

### User Story 6 — Self-Contained Spec Reading (Priority: P3)

A developer or LLM with no prior knowledge of EIP-55, libp2p, BIP-39, or RFC 6979 needs to implement the SDK using only the spec text. Every referenced external standard is summarized inline with enough detail to implement.

**Why this priority**: Self-containment is the ultimate test of machine-determinism — if an LLM needs to web-search for EIP-55, the spec has failed.

**Independent Test**: Give the spec set (001–006) to an LLM with no internet access. Ask it to implement the key derivation chain. If it asks "what is EIP-55?" or "how does PeerID derivation work?", the spec has failed.

**Acceptance Scenarios**:

1. **Given** the algorithm reference contract, **When** a reader encounters "EIP-55 checksum encoding", **Then** the full 4-step algorithm is described inline — no external document is needed.
2. **Given** the algorithm reference contract, **When** a reader encounters "PeerID derivation", **Then** the full byte-level algorithm (protobuf wrapping, multihash construction, identity threshold, base58btc encoding) is described inline.

---

### Edge Cases

- What happens when a JSON encoder emits keys in non-canonical order? → The output MUST be re-ordered to match the canonical field order. Implementations MAY use a custom serializer that guarantees order.
- What happens when an UnsignedInt64 value is exactly 0? → Encoded as JSON string `"0"`, not omitted.
- What happens when `payload` is empty (zero bytes)? → Encoded as base64 empty string `""`, not omitted.
- What happens when an optional struct field (e.g., `capabilities`) is present but all its sub-fields are at default values? → The field MUST still be present in JSON with its sub-fields serialized. Only omit the top-level key when the field itself is absent.
- What happens when the signature's V value is 27 or 28 (Ethereum convention) vs 0 or 1? → The spec MUST define which convention is used. (Resolved in FR-A10.)
- What happens when a Float64 location field (lat, lon, alt) has many decimal places? → The spec MUST define maximum precision or a serialization rule. (Resolved in FR-W10.)

---

## Requirements

### Functional Requirements — Wire Format (FR-W)

- **FR-W01**: All JSON serialization MUST use compact format: no whitespace between tokens, no trailing commas, no BOM, no trailing newline. Character encoding MUST be UTF-8.
- **FR-W02**: UnsignedInt64 fields (`timestamp`, `sequenceNumber`, `nextHeartbeatDeadline`, `negotiationDeadline`, `timeout`, protocol constants) MUST be encoded as JSON strings containing the decimal representation. No leading zeros except for the value `"0"`. Example: `"timestamp": "1700000000000000000"`.
- **FR-W02a**: All timestamp fields across the protocol (`TopicMessage.timestamp`, `HeartbeatPayload.nextHeartbeatDeadline`, `serviceRequest.negotiationDeadline`, escrow `timeout`, `observationWindow.start/end`, `DataFrame.receivedAt`) MUST represent Unix epoch nanoseconds (nanoseconds since 1970-01-01T00:00:00Z) as uint64. This aligns with HCS consensus timestamp precision, preserves signing pre-image determinism (FR-A08), and matches the golden test vectors (FR-V02, FR-V03). Protocol constants (e.g., 005 MIN_DEADLINE_DELTA = 10 seconds, GRACE_PERIOD = 30 seconds) are defined in seconds for readability; implementations MUST multiply by 10^9 when comparing against nanosecond timestamps.
- **FR-W03**: Binary fields (`signature`, `payload`) MUST be encoded as JSON strings using RFC 4648 Section 4 standard base64 encoding (alphabet `A-Za-z0-9+/`, padding with `=`).
- **FR-W04**: Optional fields that are absent MUST be omitted from the JSON object entirely. They MUST NOT be serialized as `null`. This applies to: `capabilities`, `location`, `peers` in HeartbeatPayload; any optional field in any entity.
- **FR-W05**: JSON objects MUST emit keys in the canonical field order defined per entity in the data model. The canonical order for each signed entity is listed in the data-model.md of this spec.
- **FR-W06**: `EVMAddress` values in JSON MUST use EIP-55 mixed-case checksum encoding (e.g., `"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"`). The algorithm is defined in FR-A04.
- **FR-W07**: Arbitrary-precision integers (`creditBalance`, `balanceAllocation`, `balance` in spec 001) MUST be serialized as JSON strings containing the decimal representation. Absent values MUST be represented by omitting the key (per FR-W04). Example: `"creditBalance": "1000000000000000000"`.
- **FR-W08**: All string values MUST be encoded as UTF-8. JSON string escaping MUST follow RFC 8259 Section 7 (mandatory escaping of `"`, `\`, and control characters U+0000–U+001F).
- **FR-W09**: The `senderAddress` field in TopicMessage JSON MUST use EIP-55 mixed-case checksum encoding (per FR-W06). This resolves audit item B-5.
- **FR-W10**: Float64 values (`lat`, `lon`, `alt` in Location) MUST be serialized as JSON numbers with up to 15 significant digits (IEEE 754 double precision). No trailing zeros after the decimal point, except that integer-valued floats MUST include one decimal digit (e.g., `37.0`, not `37`).

### Functional Requirements — Algorithms (FR-A)

- **FR-A01**: **secp256k1 Key Generation**. A private key is a 32-byte big-endian unsigned integer `k` where `1 ≤ k < n` (the secp256k1 curve order `n = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141`). The corresponding public key point is `Q = k * G` where `G` is the secp256k1 generator point.
- **FR-A02**: **secp256k1 Point Compression**. The compressed public key is 33 bytes: a 1-byte prefix followed by the 32-byte big-endian X coordinate of `Q`. The prefix byte is `0x02` if the Y coordinate is even, `0x03` if the Y coordinate is odd. The uncompressed public key is 65 bytes: `0x04 || X (32 bytes) || Y (32 bytes)`.
- **FR-A03**: **EVM Address Derivation**. Given an uncompressed public key (65 bytes starting with `0x04`), strip the `0x04` prefix to get 64 bytes. Compute `hash = Keccak256(64 bytes)`. The EVM address is the last 20 bytes of `hash`.
- **FR-A04**: **EIP-55 Checksum Encoding**. Given a 20-byte address: (1) Convert to lowercase hex string without `0x` prefix (40 characters). (2) Compute `hash = Keccak256(lowercase_hex_as_ascii_bytes)` — this hashes the ASCII bytes of the hex string, not the address bytes. (3) For each character at position `i` (0-indexed) in the hex string: if `hash_nibble[i] >= 8`, uppercase the character; otherwise keep lowercase. `hash_nibble[i]` is the `i`-th 4-bit nibble of the hash (nibble 0 = high nibble of byte 0, nibble 1 = low nibble of byte 0, etc.). (4) Prepend `0x`.
- **FR-A05**: **PeerID Derivation**. Given a 33-byte compressed secp256k1 public key: (1) Construct a protobuf `PublicKey` message: field 1 (KeyType, varint) = `2` (Secp256k1), field 2 (Data, length-delimited) = the 33 compressed key bytes. In protobuf wire format: `0x08 0x02 0x12 0x21` followed by the 33 key bytes — total 37 bytes. (2) Since 37 bytes ≤ 42 bytes, apply identity multihash: prefix `0x00 0x25` (identity hash function code `0x00`, length `0x25` = 37) followed by the 37 raw bytes — total 39 bytes. (3) Base58btc-encode the 39-byte multihash. The result is the PeerID string (starts with `12D3KooW` for secp256k1 keys).
- **FR-A06**: **DID:key Construction**. Given a 33-byte compressed secp256k1 public key: (1) Prepend the secp256k1-pub multicodec varint bytes `0xe7 0x01` — total 35 bytes. (2) Base58btc-encode the 35 bytes. (3) Prepend `did:key:z`. The `z` indicates base58btc encoding per the did:key spec.
- **FR-A07**: **RFC 6979 Deterministic Signing**. For a given message hash `h` (32 bytes from Keccak256) and private key `k`, the nonce `k_rfc6979` MUST be generated deterministically per RFC 6979 Section 3.2 using HMAC-SHA256 as the HMAC function. The implementation MUST NOT use random nonces. Signing the same `(hash, key)` pair MUST always produce the same `(r, s)` values.
- **FR-A08**: **Keccak256 Pre-Image for TopicMessage Signing**. The pre-image bytes for TopicMessage signature are: `timestamp` as 8-byte big-endian unsigned 64-bit integer, concatenated with `sequenceNumber` as 8-byte big-endian unsigned 64-bit integer, concatenated with `payload` raw bytes. Total: `16 + len(payload)` bytes. Hash: `Keccak256(pre_image)`.
- **FR-A09**: **Keccak256 Pre-Image for HeartbeatPayload Signing**. The HeartbeatPayload is serialized to canonical JSON (per FR-W01–FR-W10, field order per FR-W05) and used as the `payload` field of a TopicMessage. The signing pre-image is therefore: `timestamp (8 bytes) || sequenceNumber (8 bytes) || canonical_json_bytes_of_heartbeat_payload`. The signature covers the envelope, not the payload alone.
- **FR-A10**: **ECDSA Signature Encoding**. The signature is 65 bytes: `R (32 bytes big-endian) || S (32 bytes big-endian) || V (1 byte)`. `R` and `S` are the ECDSA signature components. `V` is the recovery identifier: `0` or `1` (not the Ethereum convention of `27`/`28`). `V` indicates which of the two possible public keys can be recovered from `(R, S, hash)`. The low-S normalization rule MUST be applied: if `S > n/2` (where `n` is the secp256k1 curve order), replace `S` with `n - S` and flip `V`.
- **FR-A10a**: **V-Value Normalization**. Implementations MUST accept signature V values `0`, `1`, `27`, or `28` on input (e.g., when deserializing signatures from external sources such as Ethereum wallets or legacy systems). V values `27` and `28` MUST be normalized to `0` and `1` respectively (subtract `27`). Canonical storage and wire format MUST use `V ∈ {0, 1}` only. Any V value outside `{0, 1, 27, 28}` MUST be rejected with an `InvalidSignature` error.
- **FR-A11**: **Argon2id Key Encryption Parameters**. Version 1 encryption uses Argon2id with these fixed parameters: time iterations = `1`, memory = `65536` KiB (64 MiB), parallelism = `4` threads, salt = `16` bytes (cryptographically random), tag length = `32` bytes. The derived 32-byte tag is used as the AES-256-GCM key to encrypt the private key bytes.
- **FR-A12**: **BIP-39 Mnemonic to Seed**. Mnemonic words are joined with single ASCII space characters. The seed is derived via `PBKDF2(HMAC-SHA512, mnemonic_string, "mnemonic" + passphrase, 2048 iterations, 64 bytes)`. If no passphrase, the salt is the literal ASCII string `"mnemonic"`.
- **FR-A13**: **BIP-44 HD Derivation Path**. The Neuron derivation path is `m/44'/60'/0'/0/0` (purpose=44, coin_type=60 for Ethereum, account=0, change=0, address_index=0). All levels with `'` use hardened derivation (offset `0x80000000`).
- **FR-A14**: **Ed25519 Key Detection and Rejection**. Implementations MUST reject Ed25519 keys. Detection criteria: (a) If the key is provided with a type tag or OID (e.g., DER-encoded), check for Ed25519 OID `1.3.101.112`. (b) If the key is raw bytes from a protobuf `PublicKey` message, check the `KeyType` field for value `1` (Ed25519). (c) If raw 32-byte key bytes are provided without metadata, the implementation MUST require an explicit type indicator — ambiguous raw bytes MUST NOT be silently accepted as secp256k1.

- **FR-A15**: **ECIES Multiaddr Encryption Profile** (Spec 009 FR-D11). For encrypting `connectionSetup.encryptedMultiaddrs`: (1) Generate ephemeral secp256k1 keypair `(eph_priv, eph_pub)`. (2) ECDH: shared_secret = `eph_priv * recipient_pub` — take the X coordinate as 32 bytes (pad with leading zeros if shorter). (3) KDF: `derived_key = HKDF-SHA256(ikm=shared_secret, salt=empty, info="neuron-multiaddr-v1", length=32)`. (4) Encrypt: `nonce = random(12 bytes)`, `ciphertext || tag = AES-256-GCM(key=derived_key, nonce=nonce, plaintext=JSON_array_of_multiaddrs_as_UTF8)`. (5) Output: `compressed_eph_pub(33) || nonce(12) || ciphertext(N) || tag(16)`, base64 encoded per FR-W03. Decryption reverses the process; tag verification failure MUST produce `ConnectionSetupEncryptionFailed` error.
- **FR-A16**: **Delivery Frame Encoding** (Spec 009 FR-D22). Service data on P2P delivery channels MUST be framed as: 4-byte unsigned big-endian length prefix followed by payload bytes. Maximum payload size is 4 MiB (4,194,304 bytes). A zero-length frame (4 bytes of zeros, no payload) is a keep-alive sentinel consumed silently by the delivery adapter — it MUST NOT be delivered to the application layer. Example: payload `"ADSB"` (4 bytes) encodes as `0x00000004 0x41445342` (8 bytes total).

### Functional Requirements — Test Vectors (FR-V)

- **FR-V01**: The spec MUST include at least one complete golden key derivation chain: starting from a known private key hex string, showing every intermediate value (compressed public key, uncompressed public key, EVM address with EIP-55 checksum, PeerID, DID:key) in hex or the appropriate encoding.
- **FR-V02**: The spec MUST include at least one complete TopicMessage signing chain: starting from a known private key and known message field values, showing the canonical JSON bytes (hex), the signing pre-image (hex), the Keccak256 hash (hex), and the 65-byte signature (hex).
- **FR-V03**: The spec MUST include at least one complete HeartbeatPayload signing chain: starting from a known private key and known heartbeat field values, showing the canonical payload JSON bytes (hex), the full TopicMessage canonical JSON bytes (hex), the signing pre-image (hex), the Keccak256 hash (hex), and the 65-byte signature (hex).
- **FR-V04**: The spec SHOULD include at least one error condition test vector: an invalid input (e.g., Ed25519 key, malformed signature) with the expected error code from the error taxonomy.

### Functional Requirements — Cross-Spec Resolution (FR-X)

- **FR-X01**: **Signing Responsibility Resolution**. The caller (e.g., health publisher) MUST construct and sign the TopicMessage before calling `TopicAdapter.Publish()`. The adapter MUST validate the signature but MUST NOT sign or re-sign. This resolves the contradiction between spec 004 (`topic-adapter.md`: "Validate msg has a valid signature") and spec 005 (`health-publisher.md`: "adapter handles signing"). The normative behavior is: adapter validates, never signs.
- **FR-X02**: **Implementation Type Mapping Table**. This spec MUST provide a language-neutral protocol-level equivalent for every implementation-specific type referenced in specs 001–005. See data-model.md for the full mapping table.
- **FR-X03**: **External Standard Summaries**. The algorithm reference contract MUST include inline normative summaries for: EIP-55 (FR-A04), libp2p PeerID (FR-A05), DID:key (FR-A06), RFC 6979 (FR-A07), BIP-39 (FR-A12), BIP-44 (FR-A13), secp256k1 curve parameters (FR-A01). A reader MUST NOT need to consult any external document to implement these algorithms.
- **FR-X04**: **Registration Bridge (001 ↔ 003 ↔ 007)**. This spec MUST define how NeuronAccount identity data (from spec 001) maps to Peer Registry registration inputs (spec 003) and on-chain contract parameters (spec 007): which account fields become which registration parameters. The bridge includes: (a) 001 `RegistryBinding.externalId` corresponds to 007's `tokenId` (EIP-8004 `agentId`), assigned by the Identity Registry on `register()`; (b) 001 `paymentAddress` (Parent's EVMAddress per FR-023/024) corresponds to EIP-8004 `agentWallet` metadata (writing mechanism deferred — see 007 Related Specs); (c) 003's SDK `RegistryContract` interface methods map to 007's contract functions: `Register()` → `register()`, `UpdateAgentURI()` → `updateAgentURI()`, `Revoke()` → `revoke()`, `Lookup()` → `lookup()`. This resolves audit item X-4.

### Key Entities

- **WireFormatRule**: A named encoding rule with scope (which fields/entities it applies to), encoding specification, and rationale.
- **AlgorithmDefinition**: A named algorithm with inputs (typed), step-by-step procedure (byte-level), outputs (typed), edge cases, and test vector reference.
- **TestVectorChain**: A named sequence of intermediate values from input to final output, with every value in hex or appropriate encoding.
- **ErrorDefinition**: A structured error with code, name, category, description, trigger condition, and recommended action.
- **AmendmentEntry**: A mapping from an audit item ID to the spec artifact and section that resolves it, with status.

---

## Success Criteria

### Measurable Outcomes

- **SC-001**: Every audit item (A-1 through X-6) in `machine-determinism-audit.md` has a corresponding resolution in this spec's amendment log, with status "Resolved" or "Deferred" with justification.
- **SC-002**: A developer reading specs 001–006 can implement the full key derivation chain (private key → public key → EVM address → PeerID → DID:key) without consulting any Go source code, external standard document, or web search.
- **SC-003**: The golden test vectors produce byte-identical results when executed by any two independent implementations in different programming languages.
- **SC-004**: No functional requirement in this spec uses Go-specific types, Go library function names, or Go runtime behavior as its definition. All definitions are language-neutral.
- **SC-005**: A TopicMessage serialized by any conforming implementation produces identical JSON bytes for the same field values, verified by the golden test vectors.
- **SC-006**: A HeartbeatPayload serialized by any conforming implementation produces identical JSON bytes for the same field values, verified by the golden test vectors.
- **SC-007**: Every error condition across specs 001–005 maps to a unique error code in the error taxonomy.
- **SC-008**: Signing the same `(hash, key)` pair in any conforming implementation produces byte-identical 65-byte signatures, verified by the golden test vectors.
