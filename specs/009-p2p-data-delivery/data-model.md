# Data Model: P2P Data Delivery (Spec 009)

**Source**: spec.md FR-D01–D29, Key Entities section

---

## DeliveryAdapter

Abstract interface for data plane delivery bindings. Analogous to Spec 004's TopicAdapter.

| Operation | Signature | Returns | Source FR |
|-----------|-----------|---------|----------|
| `connect` | (peerID, multiaddrs[], protocol, options?) | DeliveryChannel \| Error | FR-D02 |
| `send` | (channel, data) | SendResult \| Error | FR-D03 |
| `receive` | (channel) | AsyncStream\<DataFrame\> \| Error | FR-D04 |
| `disconnect` | (channel) | Result \| Error | FR-D05 |
| `getStatus` | (channel) | ChannelStatus | FR-D06 |

---

## DeliveryChannel

Opaque handle for an active delivery channel.

| Field | Type | Description | Source FR |
|-------|------|-------------|----------|
| `id` | string | Unique channel identifier | FR-D02 |
| `peerID` | PeerID | Remote peer identity | FR-D02 |
| `protocol` | string | Stream protocol ID (e.g., `"/neuron/adsb/1.0.0"`) | FR-D02, FR-D16 |
| `transport` | string | Active transport identifier (e.g., `"quic-v1"`, `"webrtc"`) | FR-D06 |
| `state` | ConnectionState | Current lifecycle state | FR-D07 |

---

## ConnectionState

Lifecycle state machine for delivery channels.

**States** (FR-D07): IDLE, CONNECTING, CONNECTED, RECONNECTING, RELAYING, DISCONNECTED

**Transitions** (FR-D08):

| From | To | Trigger |
|------|----|---------|
| IDLE | CONNECTING | `connect()` called |
| CONNECTING | CONNECTED | Direct dial succeeds |
| CONNECTING | RELAYING | Direct dial fails, relay succeeds |
| CONNECTING | DISCONNECTED | All attempts exhausted |
| CONNECTED | RECONNECTING | Transport connection drops |
| RELAYING | CONNECTED | DCUtR upgrade succeeds |
| RELAYING | RECONNECTING | Relay connection drops |
| RECONNECTING | CONNECTED | Reconnection succeeds (direct) |
| RECONNECTING | RELAYING | Reconnection succeeds (relay) |
| RECONNECTING | DISCONNECTED | Max backoff exceeded (FR-D09) |
| CONNECTED | DISCONNECTED | `disconnect()` or fatal error |
| RELAYING | DISCONNECTED | `disconnect()` or fatal error |

---

## DataFrame

Length-prefixed data unit on delivery channels.

| Field | Type | Description | Source FR |
|-------|------|-------------|----------|
| `data` | bytes | Opaque payload (max 4 MiB) | FR-D22, FR-D23 |
| `receivedAt` | uint64 | Unix epoch nanoseconds (per 006 FR-W02a) | FR-D04 |

**Wire format** (FR-D22): 4-byte unsigned big-endian length prefix + payload bytes.
**Keep-alive** (FR-D24): Zero-length frame (4 bytes of zeros) consumed silently.

---

## ECIESCiphertext

Encrypted multiaddr payload structure.

| Component | Size | Description | Source FR |
|-----------|------|-------------|----------|
| Ephemeral public key | 33 bytes | Compressed secp256k1 | FR-D12 |
| Nonce | 12 bytes | AES-256-GCM nonce | FR-D12 |
| Ciphertext | Variable | AES-256-GCM encrypted data | FR-D12 |
| Authentication tag | 16 bytes | AES-256-GCM tag | FR-D12 |

**Encoding**: Base64 (RFC 4648 §4) per 006 FR-W03.

**ECIES Profile** (FR-D11):
- Key agreement: secp256k1 ECDH
- KDF: HKDF-SHA256 (salt = empty, info = `"neuron-multiaddr-v1"`)
- AEAD: AES-256-GCM
- Ephemeral key: freshly generated per encryption (FR-D13)

---

## BackoffConfig

Exponential backoff parameters for reconnection.

| Parameter | Default | Description | Source FR |
|-----------|---------|-------------|----------|
| `initialDelay` | 5 seconds | First retry delay | FR-D09 |
| `factor` | 2 | Multiplication factor per attempt | FR-D09 |
| `maxDelay` | 10 minutes | Cap on individual delay | FR-D09 |
| `maxDuration` | 1 hour | Total reconnection budget | FR-D09 |

---

## ChannelStatus

Read-only status snapshot of a delivery channel.

| Field | Type | Description | Source FR |
|-------|------|-------------|----------|
| `state` | ConnectionState | Current lifecycle state | FR-D06 |
| `transport` | string | Active transport identifier | FR-D06 |

---

## Error Taxonomy

Domain: `NEURON-DELIVERY-*` (FR-D29)

| Error Kind | Trigger | Source FR |
|------------|---------|----------|
| DialFailed | All dial attempts exhausted | FR-D02 |
| StreamError | Stream I/O failure | FR-D03 |
| RelayError | Relay connection failed or unavailable | FR-D20 |
| PeerIDMismatch | Remote PeerID != connectionSetup.peerID | FR-D28 |
| NoCompatibleTransport | No multiaddr matches configured transports | FR-D25 |
| InvalidMultiaddr | Decrypted multiaddrs malformed | FR-D15 |
| ChannelClosed | Operation on closed channel | FR-D05 |
| FrameTooLarge | Frame exceeds 4 MiB | FR-D22 |
| BackoffExhausted | Max reconnection duration exceeded | FR-D09 |
| ConnectionSetupEncryptionFailed | ECIES decryption failed (shared with 008) | FR-D14 |
