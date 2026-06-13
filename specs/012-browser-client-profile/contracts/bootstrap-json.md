# Contract: Bootstrap JSON

**Spec**: 012-browser-client-profile
**Applies to**: any serving mechanism (Phase 1 Vite dev server; Phase 2 Caddy static) and to every browser-client implementation.
**Source FRs**: FR-B23, FR-B24.

The browser consumes exactly one JSON resource from the same origin as the page bundle. This contract defines its shape, validation rules, and failure responses.

---

## Minimum schema

```json
{
  "version": 1,
  "sellerEVMAddress": "0xA0B1c2D3e4F5a6B7c8D9e0F1a2B3c4D5e6F7a8B9",
  "sellerPeerID": "12D3KooWExampleBase58btcEncodedPeerIDString",
  "sellerWSSMultiaddr": "/ip4/127.0.0.1/tcp/8080/ws/p2p/12D3KooWExampleBase58btcEncodedPeerIDString",
  "controlStreamProtocolID": "/neuron/browser-profile/control/1.0.0",
  "dataStreamProtocolID": "/neuron/browser-profile/data/1.0.0"
}
```

---

## Fields

| Field | Type | Required | Validation |
|-------|------|----------|-----------|
| `version` | integer | ✓ | MUST equal `1` (Phase 1). Future majors require a new browser bundle. |
| `sellerEVMAddress` | string | ✓ | 42-char hex with `0x` prefix, EIP-55 checksum valid. |
| `sellerPeerID` | string | ✓ | Valid base58btc multihash of seller's compressed secp256k1 public key (identity-hash or sha256 per libp2p spec). |
| `sellerWSSMultiaddr` | string | ✓ | Valid libp2p multiaddr string. Scheme in `{/ws, /wss}`. Last protocol MUST be `/p2p/<sellerPeerID>`. |
| `controlStreamProtocolID` | string | ✓ | Phase 1: MUST equal `"/neuron/browser-profile/control/1.0.0"`. Future minors allowed; unknown prefixes rejected. |
| `dataStreamProtocolID` | string | ✓ | Phase 1: MUST equal `"/neuron/browser-profile/data/1.0.0"`. |

---

## Strictness

**Unknown top-level keys MUST cause a load failure with `NEURON-BROWSER-003`.** This prevents accidental protocol drift and is enforced even for forward-compatible-looking fields.

**Missing required fields** MUST cause `NEURON-BROWSER-004`. No defaulting.

**Type mismatches** (e.g., `version: "1"` as a string) MUST cause `NEURON-BROWSER-009`.

---

## Scheme-specific validation

When `sellerWSSMultiaddr` begins with `/ws` (plain WebSocket):

- The **page origin** MUST be `http://localhost[:PORT]` or `http://127.0.0.1[:PORT]`.
- Any other origin → reject with `NEURON-BROWSER-008`. This is a defence-in-depth check; browser mixed-content rules would already block a cross-origin `ws://` from an `https://` page, but the browser client enforces the same rule explicitly so a misconfigured deployment fails loud and early.

When `sellerWSSMultiaddr` begins with `/wss`:

- Any origin is acceptable.
- Browser expects a valid TLS cert chain from the browser's trust store. Self-signed or internal-CA certs cause a raw WebSocket dial failure; this surfaces as `NEURON-BROWSER-020` (transport: TLS/WS connect refused).

---

## Fetch rules

- Path: `/bootstrap.json` (relative). Fetched via `fetch("/bootstrap.json", { credentials: "omit", cache: "no-store", mode: "same-origin" })`.
- **`mode: "same-origin"`** enforces FR-B24. A cross-origin-injected bootstrap would be rejected by the browser's fetch implementation; explicit `mode` ensures we don't accidentally opt in to CORS.
- **`cache: "no-store"`** prevents stale bootstrap during development. Live deployments MAY adjust caching in Phase 2, but v1 defaults to fresh-every-load.
- **`credentials: "omit"`** — we don't send cookies or HTTP auth. The page has no auth state anyway (FR-B03).
- Non-200 HTTP status → `NEURON-BROWSER-001`.
- `Content-Type` MUST start with `application/json`; any other → `NEURON-BROWSER-010`.
- Body MUST parse as JSON; parse failure → `NEURON-BROWSER-011`.

---

## Example: malformed bootstrap → expected browser behaviour

```json
{
  "version": 1,
  "sellerEVMAddress": "0x0000000000000000000000000000000000000000",
  "sellerPeerID": "12D3KooW…",
  "sellerWSSMultiaddr": "/ip4/127.0.0.1/tcp/8080/ws/p2p/12D3KooW…",
  "controlStreamProtocolID": "/neuron/browser-profile/control/1.0.0",
  "dataStreamProtocolID": "/neuron/browser-profile/data/1.0.0",
  "extraField": "hello"     // <-- unknown top-level key
}
```

→ Browser rejects load with `NEURON-BROWSER-003`. No dial attempted. Ledger shows no entries. Page displays: *"Bootstrap rejected: unknown field 'extraField'."*

---

## Versioning policy

- **v1**: the only schema supported by Phase 1.
- **v2**: Phase 2 may introduce, e.g., a `bootstrapSignature` field for signed bootstrap delivery. Browsers understanding v2 MUST also understand v1; browsers understanding only v1 MUST reject a v2 document.

This is asymmetric on purpose: the v1 browser's strictness is the anti-drift mechanism.
