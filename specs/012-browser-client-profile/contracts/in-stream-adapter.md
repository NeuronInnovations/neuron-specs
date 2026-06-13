# Contract: In-Stream TopicAdapter

**Spec**: 012-browser-client-profile
**Applies to**: browser client and seller (Phase 1 Node.js, Phase 2 Go).
**Source FRs**: FR-B10, FR-B11, FR-B12, FR-B13, FR-B14.

The in-stream TopicAdapter is a Spec 004 `TopicAdapter` whose transport is a single libp2p stream. It is the control-plane in v1: the only place signed TopicMessage envelopes flow between browser and seller. No HCS, no Kafka, no ERC-log fallback in v1.

---

## Semantics

From the envelope's perspective, the in-stream adapter is indistinguishable from any other Spec 004 `TopicAdapter`. The envelope is signed, canonical-JSON encoded, published; subscribers receive parsed envelopes. The wire is the only thing that differs.

---

## Stream lifecycle

Exactly **one control stream per browser session**, opened by the browser to the seller on protocol `/neuron/browser-profile/control/1.0.0` (see `stream-protocols.md`).

```
 browser                         seller
   │                                │
   │ dial(peerID, control-proto) ──▶│
   │                                │
   │◀── stream handle ──            │   // both sides now own one StreamHandle
   │                                │
   │── publish(serviceRequest) ───▶ │
   │                                │── (seller-flow.ts advances)
   │                                │
   │ ◀──── paymentDetails ──────────│
   │── verify() ───────────         │
   │                                │
   │ ◀──── connectionSetup ─────────│
   │── verify() + ECIES-decrypt ─── │
   │                                │
   │── publish(invoiceAck) ────────▶│
   │                                │── (seller-flow.ts completes)
   │                                │
   │── close() ────────────────────▶│
```

There is no round-trip message routing, no multiplexed topic IDs, no subscription filtering. One stream = one logical conversation = one browser session.

---

## Wire format

Each envelope is transmitted as one length-prefixed frame:

```
┌────────────────────┬────────────────────────────────────────────┐
│ 4 bytes BE uint32  │ N bytes canonical-JSON TopicMessage        │
│ payload length = N │                                             │
└────────────────────┴────────────────────────────────────────────┘
```

- `N = 0` → keep-alive; silently consumed. Senders MAY emit keep-alives at application discretion; receivers MUST accept them.
- `N > MAX_FRAME_BYTES` (4 MiB) → abort with `NEURON-BROWSER-101`.
- Payload MUST parse as a well-formed Spec 004 `TopicMessage` envelope; failure → `NEURON-BROWSER-060`.

---

## TopicMessage envelope fields (consumed & produced)

(Spec 004 defines the full envelope; this section calls out what this contract relies on.)

| Field | Required | Notes |
|-------|----------|-------|
| `senderAddress` | ✓ | EIP-55 hex. Must equal recovered ECDSA signer. |
| `timestamp` | ✓ | Unix nanoseconds. Browser MAY enforce skew tolerance (v1: ±2 minutes — see spec.md Assumptions). |
| `sequenceNumber` | ✓ | uint64 monotonic per sender; browser tracks seller's seqNum and MUST abort on decrement. |
| `payload` | ✓ | Application-defined; v1 payloads are `serviceRequest` / `paymentDetails` / `connectionSetup` / `invoiceAck` per Spec 008. |
| `signature` | ✓ | 65 bytes R‖S‖V (V ∈ {0, 1}); Keccak-256 of canonical signing input per Spec 006 FR-W07. |

---

## Signature verification rules

Every inbound envelope, before it reaches the state machine:

1. **Signature recovery**: `ecrecover(keccak256(signing_input), signature)` MUST return `senderAddress`. Mismatch → `NEURON-BROWSER-061`.
2. **Sender pinning**: `senderAddress` MUST equal `bootstrap.sellerEVMAddress`. Any other address → `NEURON-BROWSER-062` (FR-B14).
3. **Sequence check**: `seqNum > lastSellerSeqNum`. Violation → `NEURON-BROWSER-063`.
4. **Timestamp sanity**: `|now − envelope.timestamp| ≤ 2 minutes`. Violation → `NEURON-BROWSER-064`.
5. **Payload type check**: expected type matches current `BuyerState` (`serviceRequest`-sent → expect `paymentDetails`; etc.). Violation → `NEURON-BROWSER-140`.

All five checks MUST pass before the envelope is dispatched or logged as `"verified"` in the ledger. Any failure aborts the session.

---

## Outbound publishing rules

Browser-published envelopes (`serviceRequest`, `invoiceAck`):

1. Build payload according to Spec 008.
2. Compute signing input (canonical JSON of envelope minus signature, per Spec 006).
3. Sign with session `privateKey` via `impl/typescript/src/keylib/signature.ts` (RFC 6979 deterministic).
4. Canonical-JSON encode the full envelope.
5. Length-prefix and write to the control stream.
6. Append a ledger entry with `direction = "outbound"`, `signatureStatus = "self-signed"`.

---

## Backpressure and flow control

- libp2p's yamux provides per-stream flow control. Browser MUST respect its writable state and await `drain` before further writes.
- Phase 1 envelopes are small (< 4 KiB each); backpressure is unlikely to matter. The contract still requires correct behaviour so Phase 2 size growth doesn't silently stall.

---

## Close semantics

- **Graceful close by browser**: after sending `invoiceAck` and receiving the last data chunk + verifying SHA-256, browser calls `stream.close()`. Seller's read loop exits cleanly with EOF.
- **Graceful close by seller**: seller does NOT proactively close in v1; browser owns close timing.
- **Abnormal close** (either side): the other side MUST abort with `NEURON-BROWSER-021` and transition session to `aborted`.

---

## Keep-alive policy

- Sender MAY emit zero-length frames at any cadence (typical: once every 10 s of application idle).
- Receiver MUST silently accept and continue.
- v1 buyer and seller do NOT rely on keep-alives for liveness; `READ_IDLE_MS` (data stream only) is the sole idle detector.

---

## Validation anchors (for the eventual Go seller / H1 interop)

When Phase 2 H1 brings up the Go seller, these contract points are what Go must satisfy:

- Same control stream protocol ID (`/neuron/browser-profile/control/1.0.0`).
- Same framing (byte-for-byte with `impl/golang/internal/delivery/framing.go`).
- Same Spec 004 envelope shape (already normative across the codebase).
- Same signature rules (Spec 006 — already implemented in `internal/keylib/`).

The fixture test (`impl/typescript/tests/interop/go-envelope.fixture.bin`) gates this: a canonical envelope signed by a known Go key can be parsed + verified by the TS adapter, and vice versa.
