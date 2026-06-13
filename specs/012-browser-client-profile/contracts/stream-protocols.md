# Contract: libp2p Stream Protocols

**Spec**: 012-browser-client-profile
**Applies to**: both browser client and Node.js / Go seller.
**Source FRs**: FR-B10, FR-B11, FR-B12, FR-B19, FR-B20.

This contract is normative for Phase 1 (JS↔JS) and Phase 2 (JS↔Go). Any implementation of 012 MUST comply bit-for-bit.

---

## Protocol identifiers

| ID | Purpose | Direction |
|----|---------|-----------|
| `/neuron/browser-profile/control/1.0.0` | In-stream TopicAdapter; signed TopicMessage envelopes | Bidirectional |
| `/neuron/browser-profile/data/1.0.0` | File delivery (frame 0 metadata + chunks) | Server → browser |

Both IDs MUST be advertised by the seller via libp2p `identify`. The browser dials them explicitly. Unknown or mismatched protocol IDs MUST cause connection refusal (FR-B09 analogue for stream protocols).

---

## Handshake order

From the moment a libp2p connection is established:

1. **Noise XX** — mutual authentication; server PeerID verified against `bootstrap.sellerPeerID` (FR-B06). Failure aborts with `NEURON-BROWSER-041`.
2. **yamux multiplexer** — stream fan-out.
3. **libp2p identify** — protocol-list exchange.
4. **Control stream opened** — `browser_dial(remote, "/neuron/browser-profile/control/1.0.0")`.
5. **`serviceRequest` sent** — first application-level bytes.
6. **`paymentDetails` received + verified** — see In-Stream Adapter contract.
7. **`connectionSetup` received + verified + ECIES-decrypted** — FR-B16.
8. **Data stream opened** — `browser_dial(remote, "/neuron/browser-profile/data/1.0.0")`.
9. **Frame 0 (metadata) received** — content-type, size, declared SHA-256.
10. **Frames 1..N (chunks) received** — up to `MAX_FRAME_BYTES` each; total ≤ `MAX_TOTAL_BYTES`.
11. **`invoiceAck` sent on control stream** — after successful SHA-256 verification.
12. **Both streams closed gracefully**.

Any step failing aborts the transaction at that point. No partial data is rendered.

---

## Framing (both streams)

Every logical message on either stream is a **length-prefixed frame**:

```
┌────────────────────────┬──────────────────────────┐
│  4 bytes big-endian    │  N bytes payload         │
│  uint32 length = N     │                          │
└────────────────────────┴──────────────────────────┘
```

- `N = 0` is a **keep-alive**: receiver silently consumes and continues.
- `N > MAX_FRAME_BYTES` (= `4 * 1024 * 1024`) is a **protocol fault**: abort with `NEURON-BROWSER-101`.
- Payload interpretation depends on stream:
  - **Control stream**: payload is canonical-JSON-encoded Spec 004 `TopicMessage` envelope.
  - **Data stream**, frame 0: payload is canonical-JSON-encoded metadata (see below).
  - **Data stream**, frames 1..N: raw opaque bytes.

This framing is identical to `impl/golang/internal/delivery/framing.go`'s frame protocol (Spec 009 FR-D22). A byte-for-byte fixture test gates both sides.

---

## Data stream frame 0 (metadata)

The **first non-keep-alive frame** on the data stream. Canonical-JSON object:

```json
{
  "filename": "demo.jpg",
  "sizeBytes": 102400,
  "contentType": "image/jpeg",
  "sha256Hex": "3f786850e387550fdab836ed7e6dc881de23001b"
}
```

Required fields (all MUST be present):

| Field | Type | Constraints |
|-------|------|-------------|
| `filename` | string | ≤ 255 bytes, printable UTF-8, no path separators. |
| `sizeBytes` | integer | ≥ 1, ≤ `MAX_TOTAL_BYTES` (= 1 MiB in v1). |
| `contentType` | string | MIME type; browser validates allowlist for the spike (`image/jpeg`, `image/png` only). |
| `sha256Hex` | string | 64-char lowercase hex; SHA-256 of the concatenated raw chunk payloads. |

Unknown top-level keys MUST cause abort with `NEURON-BROWSER-103` — v1 is strict to prevent accidental protocol drift.

Metadata `sizeBytes > MAX_TOTAL_BYTES` → abort with `NEURON-BROWSER-101` **before any chunk frame is read** (FR-B21).

---

## Chunk frames (data stream, frames 1..N)

- Each chunk is raw bytes; no framing inside the chunk (the outer frame IS the framing).
- Receiver reassembles by concatenating in receipt order.
- Total concatenated length MUST equal `metadata.sizeBytes`; mismatch → abort with `NEURON-BROWSER-102`.
- Running `sha256` over the concatenation MUST equal `metadata.sha256Hex`; mismatch → abort with `NEURON-BROWSER-082`.
- No activity for > `READ_IDLE_MS` between chunks → abort with `NEURON-BROWSER-121`.

---

## Graceful close

Seller closes the **data stream** after the last chunk is written. Browser closes the **control stream** after sending `invoiceAck`. Either side closing before `complete` is an abnormal close:

- Browser-side abnormal close: abort with `NEURON-BROWSER-021` (transport: stream closed unexpectedly).
- Seller-side abnormal close: seller logs, transitions escrow to `refunded`, cleans up.

---

## Protocol versioning

The `/1.0.0` suffix is semver-like:

- **Patch** (`/1.0.1`): bug-fix wire compatibility; receivers MUST accept.
- **Minor** (`/1.1.0`): additive fields; senders MUST default; old receivers MUST ignore unknown fields.
- **Major** (`/2.0.0`): breaking. Bumping major means new protocol IDs; parallel-ship is allowed.

Phase 1 ships only `/1.0.0`. Any protocol-level change before main merge rebases the suffix via Phase 2 H5 (error-taxonomy alignment) if it breaks wire compatibility.
