# Contract: Error Taxonomy

**Spec**: 006-protocol-determinism | **Date**: 2026-03-03
**Scope**: Unified cross-spec error code namespace for the Neuron SDK
**Resolves**: Audit item X-5

---

## Error Code Format

```
NEURON-{DOMAIN}-{NUMBER}
```

- **NEURON**: Fixed prefix for all Neuron SDK errors
- **DOMAIN**: Spec domain code (see table below)
- **NUMBER**: Three-digit sequential number within the domain

| Domain Code | Spec | Description |
|-------------|------|-------------|
| `KEY` | 002 Key Library | Key generation, derivation, signing, encryption errors |
| `ACCT` | 001 NeuronAccount | Account creation, validation, type-specific errors |
| `REG` | 003 Peer Registry | Registration, lookup, update, revocation errors |
| `TOPIC` | 004 Topic System | Topic creation, publishing, subscribing, adapter errors |
| `HEALTH` | 005 Health | Heartbeat construction, liveness, deadline errors |
| `WIRE` | 006 Protocol Determinism | Wire format encoding/decoding errors |
| `PAYMENT` | 008 Payment | Commerce, negotiation, escrow, settlement errors |
| `DELIVERY` | 009 P2P Data Delivery | Connection, framing, encryption, transport errors |
| `VALIDATION` | 010 Validation Framework | Evidence envelope, verdict, validator service errors |

---

## Error Structure

Every error across the Neuron SDK MUST conform to this structure:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `code` | string | MUST | Error code (e.g., `NEURON-KEY-001`) |
| `name` | string | MUST | Error name (e.g., `InvalidFormat`) |
| `message` | string | MUST | Human-readable description |
| `cause` | error | MAY | Wrapped underlying error (for debugging) |

**Language mapping**: Implement using the language's idiomatic error/exception type with `code` and `name` fields. Optionally support error chaining for wrapped causes. Examples (alphabetical):
- **Go**: Struct satisfying the `error` interface with `Code()`, `Name()`, `Error()` methods
- **Java**: Extend `Exception` with `getCode()` and `getName()` methods
- **Python**: Extend `Exception` with `code` and `name` attributes
- **Rust**: Enum with variants per error, implement `std::error::Error`
- **TypeScript/JavaScript**: Extend `Error` class with `code` and `name` properties

---

## KEY Domain (Spec 002)

Source: `specs/002-key-library/data-model.md` Error Types

| Code | Name | Trigger Condition | Recommended Action |
|------|------|-------------------|-------------------|
| `NEURON-KEY-001` | `InvalidFormat` | Input format not recognized (not hex, not valid encoding) | Check input format. Hex strings should be 64 chars (private) or 66/130 chars (public). |
| `NEURON-KEY-002` | `UnsupportedKeyType` | Ed25519 or non-secp256k1 key detected (FR-A14) | Only secp256k1 keys are supported. Check key type/curve. |
| `NEURON-KEY-003` | `InvalidLength` | Input has wrong byte length (not 32/33/65 bytes) | Private key: 32 bytes. Compressed public: 33 bytes. Uncompressed: 65 bytes. |
| `NEURON-KEY-004` | `InvalidHex` | Non-hex characters in hex string input | Remove non-hex characters. Valid: 0-9, a-f, A-F. Optional 0x prefix. |
| `NEURON-KEY-005` | `InvalidKey` | Key bytes fail secp256k1 curve validation (k=0, k≥n, point not on curve) | Generate a new key. The provided bytes are not a valid secp256k1 key. |
| `NEURON-KEY-006` | `ZeroValue` | All-zero key material provided | Generate a new key. All-zero is not a valid private key. |
| `NEURON-KEY-007` | `KeyMismatch` | Key relationship verification failed (Matches() returned false) | Verify that the keys were derived from the same source. |
| `NEURON-KEY-008` | `EncryptionFailed` | Argon2id derivation or AES-GCM encryption failed | Check password encoding (UTF-8) and system memory availability. |
| `NEURON-KEY-009` | `DecryptionFailed` | AES-GCM decryption failed (wrong password, corrupted data) | Verify password. Check that the encrypted key data is not corrupted. |
| `NEURON-KEY-010` | `InvalidMnemonic` | Bad mnemonic: wrong word count, invalid word, checksum failure | Check spelling, word count (12 or 24), and use the BIP-39 English word list. |
| `NEURON-KEY-011` | `DerivationFailed` | BIP-44 HD key derivation failed (invalid seed, intermediate key invalid) | Verify the mnemonic and derivation path. |
| `NEURON-KEY-012` | `SDKError` | Wrapped error from underlying blockchain SDK | Inspect `cause` for the original SDK error details. |
| `NEURON-KEY-013` | `SigningFailed` | ECDSA signing operation failed | Verify private key validity and message hash is 32 bytes. |
| `NEURON-KEY-014` | `VerificationFailed` | Signature verification failed (signature does not match public key + message) | Check that the correct message hash and public key are used. |

---

## ACCT Domain (Spec 001)

| Code | Name | Trigger Condition | Recommended Action |
|------|------|-------------------|-------------------|
| `NEURON-ACCT-001` | `InvalidAccountType` | Account type not one of Parent (1), Child (2), Shared (3) | Use AccountType enum values. |
| `NEURON-ACCT-002` | `MissingRequiredField` | A MUST field for the account type is absent | Check validation rules V-PARENT-*, V-CHILD-*, V-SHARED-* in data model. |
| `NEURON-ACCT-003` | `ForbiddenField` | A field present that MUST NOT be set for this account type | Parent: no parentPubKey, no multisigKey. Child: no DID. Shared: no DID, no parentPubKey. |
| `NEURON-ACCT-004` | `InvalidDID` | DID format does not match `did:key:zQ3s...` pattern | Verify DID:key construction per algorithm-reference.md §6. |
| `NEURON-ACCT-005` | `ParentKeyMismatch` | Child's parentPubKey does not correspond to any known Parent | Verify the Parent account exists and the key matches. |
| `NEURON-ACCT-006` | `InvalidLedgerAttachment` | Ledger attachment state/status is invalid | Check ledgerIdentifier format and attachedAddress validity. |
| `NEURON-ACCT-007` | `AccountIncomplete` | Account does not satisfy completeness check (FR-011a) | All MUST fields plus ledger attachment must be present. |
| `NEURON-ACCT-008` | `InvalidCurrencySymbol` | Currency symbol is empty or not recognized | Provide a valid currency symbol string. |

---

## REG Domain (Spec 003)

| Code | Name | Trigger Condition | Recommended Action |
|------|------|-------------------|-------------------|
| `NEURON-REG-001` | `RegistrationFailed` | On-chain registration transaction failed | Check network connectivity, gas/fee balance, and contract state. |
| `NEURON-REG-002` | `LookupFailed` | Agent ID or address not found in the registry | Verify the agent is registered. Check the correct registry contract. |
| `NEURON-REG-003` | `UpdateFailed` | agentURI update transaction failed | Only the NFT owner can update. Verify caller authority. |
| `NEURON-REG-004` | `RevocationFailed` | Revocation/burn transaction failed | Only the NFT owner can revoke. Verify caller authority. |
| `NEURON-REG-005` | `InvalidAgentURI` | agentURI JSON does not conform to expected schema | Verify services[] and registrations[] arrays per EIP-8004. |
| `NEURON-REG-006` | `UnauthorizedCaller` | Caller is not authorized to perform the operation | Check allowlist, ownership, and DID authorization. |

---

## TOPIC Domain (Spec 004)

| Code | Name | Trigger Condition | Recommended Action |
|------|------|-------------------|-------------------|
| `NEURON-TOPIC-001` | `InvalidTopicRef` | TopicRef transport not registered or locator malformed | Check transport kind and locator format. |
| `NEURON-TOPIC-002` | `UnsupportedOperation` | Operation (create/publish) called on read-only adapter | Use a read-write adapter for publishing. |
| `NEURON-TOPIC-003` | `InvalidSignature` | TopicMessage signature is missing, malformed, or does not verify | Sign the message before publishing. Check key matches senderAddress. |
| `NEURON-TOPIC-004` | `SequenceViolation` | sequenceNumber is not monotonically increasing | Each message must have a higher sequenceNumber than the previous. |
| `NEURON-TOPIC-005` | `PayloadTooLarge` | Payload exceeds adapter's MaxMessageSize | Reduce payload size or split across multiple messages. |
| `NEURON-TOPIC-006` | `AdapterNotRegistered` | No adapter registered for the given BackendKind | Register an adapter for this transport via RegisterAdapter(). |
| `NEURON-TOPIC-007` | `PublishFailed` | Backend rejected the message (network error, insufficient fees) | Inspect `cause` for backend-specific error. Retry or check fees. |
| `NEURON-TOPIC-008` | `SubscribeFailed` | Subscription could not be established | Check topic existence, network connectivity, and adapter state. |
| `NEURON-TOPIC-009` | `TopicResolveFailed` | Topic metadata could not be retrieved | Check topic existence and adapter connectivity. |
| `NEURON-TOPIC-010` | `InvalidTimestamp` | TopicMessage timestamp is zero or unreasonable | Timestamp must be positive nanoseconds since Unix epoch. |

---

## HEALTH Domain (Spec 005)

| Code | Name | Trigger Condition | Recommended Action |
|------|------|-------------------|-------------------|
| `NEURON-HEALTH-001` | `InvalidPayloadType` | HeartbeatPayload.type is not `"heartbeat"` | Set type to `"heartbeat"`. |
| `NEURON-HEALTH-002` | `InvalidVersion` | Version string is not a recognized semver | Use format `"1.x.y"`. Version 2.x.y is rejected. |
| `NEURON-HEALTH-003` | `InvalidDeadline` | nextHeartbeatDeadline violates MIN_DELTA/MAX_DELTA constraints | Deadline must be between `consensusTs + 10s` and `consensusTs + 86400s`. |
| `NEURON-HEALTH-004` | `InvalidRole` | Role is not one of `buyer`, `seller`, `relay` | Use one of the defined roles. `validator` is reserved. |
| `NEURON-HEALTH-005` | `PayloadTooLarge` | Serialized payload exceeds 256-byte budget | Trim optional fields in order: peers → capabilities → location. |
| `NEURON-HEALTH-006` | `InvalidLocation` | Location present but missing required lat/lon | If location is included, both lat and lon are required. |
| `NEURON-HEALTH-007` | `InvalidCapabilities` | Capabilities field has invalid natType or protocol format | natType must be one of the defined enum values. |

---

## WIRE Domain (Spec 006)

| Code | Name | Trigger Condition | Recommended Action |
|------|------|-------------------|-------------------|
| `NEURON-WIRE-001` | `InvalidFieldOrder` | JSON object keys are not in canonical order | Re-serialize using the canonical field order defined in data-model.md. |
| `NEURON-WIRE-002` | `InvalidUint64Encoding` | UnsignedInt64 value encoded as JSON number instead of string | Encode UnsignedInt64 values as JSON strings: `"42"` not `42`. |
| `NEURON-WIRE-003` | `InvalidBase64` | Binary field uses wrong base64 variant or is malformed | Use RFC 4648 §4 standard base64 with `=` padding. |
| `NEURON-WIRE-004` | `NullOptionalField` | Optional field serialized as `null` instead of omitted | Omit absent optional fields entirely. Do not serialize as `null`. |
| `NEURON-WIRE-005` | `InvalidAddressEncoding` | EVMAddress not in EIP-55 checksum format | Apply EIP-55 checksum encoding per algorithm-reference.md §4. |
| `NEURON-WIRE-006` | `InvalidUTF8` | String contains invalid UTF-8 sequences | All strings must be valid UTF-8. |

---

## PAYMENT Domain (Spec 008)

Source: `specs/008-payment/spec.md` FR-P32

| Code | Name | Trigger Condition | Recommended Action |
|------|------|-------------------|-------------------|
| `NEURON-PAYMENT-001` | `InvalidServiceOffering` | neuron-commerce service missing MUST fields or invalid schema | Verify agentURI contains valid neuron-commerce entries per FR-P01. |
| `NEURON-PAYMENT-002` | `NegotiationFailed` | serviceResponse indicates rejection or invalid transition | Review negotiation history; retry with adjusted terms. |
| `NEURON-PAYMENT-003` | `NegotiationExpired` | negotiationDeadline elapsed without seller response (FR-P07a) | Retry negotiation or select different seller. |
| `NEURON-PAYMENT-004` | `VersionMismatch` | Payload major version >= 2 received (FR-P12a) | Upgrade SDK to support new version. |
| `NEURON-PAYMENT-005` | `EscrowCreationFailed` | createEscrow transaction failed on binding | Check network connectivity, funds, contract state. |
| `NEURON-PAYMENT-006` | `InsufficientBalance` | requestRelease amount exceeds getBalance available (FR-P25a) | Deposit additional funds before release request. |
| `NEURON-PAYMENT-007` | `InvoiceValidationFailed` | Invoice amount/period does not match agreed terms | Refuse invoice or negotiate adjustment. |
| `NEURON-PAYMENT-008` | `ReleaseNotAuthorized` | Release approval rejected (binding-specific reason) | Verify escrow state; check buyer approval signature. |
| `NEURON-PAYMENT-009` | `RefundNotEligible` | claimRefund called before timeout (FR-P22) | Wait until timeout elapses. |
| `NEURON-PAYMENT-010` | `BindingUnavailable` | Settlement binding not registered or unreachable | Select different binding or configure transport. |
| `NEURON-PAYMENT-011` | `TimeoutNotElapsed` | Refund attempted pre-timeout | Wait until escrow timeout. |
| `NEURON-PAYMENT-012` | `InvalidEscrowRef` | EscrowRef format invalid or escrow not found | Verify escrowRef and binding consistency. |
| `NEURON-PAYMENT-013` | `UnsupportedDeliveryMode` | delivery.mode value not recognized (FR-P01a) | Use "p2p", "topic", or "custom:<type>". |
| `NEURON-PAYMENT-014` | `InvalidDeliveryRef` | delivery.serviceRef or channelRef does not match agentURI (FR-P01b) | Check cross-reference against existing services. |
| `NEURON-PAYMENT-015` | `ConnectionSetupRequired` | P2P delivery mode but no connectionSetup exchanged (FR-P35) | Exchange connectionSetup before starting delivery. |
| `NEURON-PAYMENT-016` | `ConnectionSetupEncryptionFailed` | Encryption/decryption of encryptedMultiaddrs failed (FR-P34) | Verify correct key pair used. Shared with DELIVERY domain. |

---

## DELIVERY Domain (Spec 009)

Source: `specs/009-p2p-data-delivery/spec.md` FR-D29

| Code | Name | Trigger Condition | Recommended Action |
|------|------|-------------------|-------------------|
| `NEURON-DELIVERY-001` | `DialFailed` | All dial attempts exhausted (FR-D02) | Verify multiaddrs, network connectivity, and seller availability. |
| `NEURON-DELIVERY-002` | `StreamError` | Stream I/O failure during send/receive (FR-D03) | Check connection state; may need reconnection. |
| `NEURON-DELIVERY-003` | `RelayError` | Relay connection failed or relay unavailable (FR-D20) | Try different relay node or direct connection. |
| `NEURON-DELIVERY-004` | `PeerIDMismatch` | Remote PeerID does not match connectionSetup (FR-D28) | Verify PeerID in connectionSetup matches actual seller. |
| `NEURON-DELIVERY-005` | `NoCompatibleTransport` | No multiaddr matches configured transports (FR-D25) | Check seller multiaddrs and buyer transport config. |
| `NEURON-DELIVERY-006` | `InvalidMultiaddr` | Decrypted multiaddrs are malformed (FR-D15) | Verify ECIES decryption and multiaddr format. |
| `NEURON-DELIVERY-007` | `ChannelClosed` | Operation on closed channel (FR-D05) | Check channel state before send/receive. |
| `NEURON-DELIVERY-008` | `FrameTooLarge` | Frame payload exceeds 4 MiB limit (FR-D22) | Split data into smaller frames. |
| `NEURON-DELIVERY-009` | `BackoffExhausted` | Max reconnection duration exceeded (FR-D09) | Accept disconnection; re-initiate via new connectionSetup. |
| `NEURON-DELIVERY-010` | `ConnectionSetupEncryptionFailed` | ECIES decryption failed (FR-D14) | Verify correct key pair. Shared with PAYMENT domain. |

---

## Error Propagation

### Cross-Module Error Wrapping

When an error crosses module boundaries (e.g., a KEY error surfaces through a TOPIC operation), the outer module SHOULD:

1. Create its own domain error (e.g., `NEURON-TOPIC-003` InvalidSignature).
2. Set the `cause` field to the original error (e.g., `NEURON-KEY-014` VerificationFailed).
3. The caller can inspect `cause` to determine the root cause.

**Example chain**:
```
NEURON-TOPIC-003 (InvalidSignature)
  └── cause: NEURON-KEY-014 (VerificationFailed)
        └── cause: <underlying crypto library error>
```

### Error Severity

Errors in this taxonomy are all **unrecoverable for the current operation** — the caller should handle them (retry with different input, report to user, etc.) rather than retrying the same operation. Transient errors (network timeouts, temporary unavailability) are wrapped in the domain-specific error with the transient error as `cause`.
