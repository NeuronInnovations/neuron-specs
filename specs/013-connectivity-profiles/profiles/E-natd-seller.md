# Profile E — NATed Seller → Public Buyer (reverse-connect)

> **Status:** Draft. This file is the artifact for Profile E pending its integration into `../spec.md` "Profile Definitions" section. The other profiles (A, C, D) are defined normatively in `spec.md` with stubs in this directory; Profile E is currently the inverse — substantive here, pending propagation into `spec.md` via a future `/speckit.propagate` cycle. Until then, **this file** is the authoritative normative source for Profile E.

## Overview

Profile E covers the **reverse-connect topology** introduced by the JetVision Air!Squitter edge demo (`impl/golang/cmd/edge-seller`, `cmd/edge-buyer`). The shape is:

- **Initiator** (the dialer) is a NATed agent with **outbound** UDP connectivity but no public listener — typical of edge devices behind carrier-grade NAT or a home-router NAT without UPnP.
- **Responder** (the dialee) is a publicly-reachable agent that **listens** for incoming libp2p QUIC streams.
- Control-plane messages flow over **HCS topics** in **both** directions (`stdIn`/`stdOut`/`stdErr`), independent of the data-plane libp2p stream.

This inverts the canonical buyer-seller roles in spec 008/009: in Profile E the **seller dials** the **buyer**. The buyer publishes a `ReverseConnectionSetup` payload (encrypted to the seller's pubkey via ECIES per spec 009) on the seller's stdIn; the seller observes it, decrypts, and dials the buyer's advertised libp2p multiaddrs.

Profile E is a structural complement to Profiles A, C, and D:

- **vs. Profile A** (browser → public listener): A's initiator is a browser; E's initiator is a server-class agent (NATed device or VPS). Both terminate at a public listener, but E's initiator can hold persistent identity, publish heartbeats, and operate full HCS control-plane.
- **vs. Profile C** (any → NATed peer via relay): C uses a Circuit-Relay-V2 reservation to traverse responder-side NAT; E avoids the relay entirely by inverting which side is NATed and which is reachable. Profile E is the cheaper, lower-latency alternative when the deployment can guarantee a publicly-reachable buyer.
- **vs. Profile D** (peer ↔ peer direct): D requires *both* parties be reachable; E requires *exactly one* (the buyer) be reachable. E is the strict superset of D in terms of NAT tolerance on the seller side.

## Runtime contexts

- **Initiator (seller)**: a NATed agent. Typical examples: ADS-B feeder behind a residential NAT, IoT sensor on a carrier-NATed cell connection, or a server-class agent inside a corporate network without inbound port-forwarding. The seller **must** retain enough outbound state to (a) dial libp2p QUIC over UDP, (b) maintain HCS publish capability, and (c) hold a persistent secp256k1 identity for cross-restart re-registration on EIP-8004.
- **Responder (buyer)**: a publicly-reachable listener process. Typical examples: a $5/mo VPS with a static IPv4, an enterprise gateway with port-forwarded UDP, or a cloud function with a fixed inbound endpoint. The buyer announces its multiaddrs to one or more sellers through their stdIn topics.

## Required capabilities

- `identity-lifetime = persistent` (both parties) — the seller's HCS topics, agentURI, and EIP-8004 registration MUST survive restarts; the buyer's persistent identity allows it to act as a stable rendezvous point for many sellers.
- `control-plane = topic` — both sides publish negotiation and heartbeat envelopes through HCS; in-stream control is not used because Profile E's data plane carries arbitrary payloads (BEAST frames, etc.) and must not be multiplexed with control.
- `listen-capability = outbound-dial-only` (initiator), `listen` (responder).
- `nat-traversal = outbound-dial-only` — no relay reservation, no DCUtR upgrade, no STUN/TURN; the seller's NAT must permit unsolicited outbound UDP, which is the universal case across consumer-grade NAT.
- `audit-trail = client-publish` — both parties publish heartbeats and negotiation messages to their own stdOut topics; an external validator (spec 010) can subscribe to both and attest delivery.
- `ordering = fifo-per-stream` — the libp2p QUIC stream guarantees per-stream FIFO; `consensus-ordered` would be too expensive for high-frequency data.
- `confidentiality = transport+payload-ecies` — buyer's multiaddrs in `ReverseConnectionSetup` are ECIES-encrypted to the seller's pubkey (spec 009). Within the established libp2p stream, Noise/QUIC TLS handles transport encryption.
- `reconnect-semantics = seller-driven` — seller is responsible for re-dialing on stream closure; buyer's responsibility is limited to keeping its `ReverseConnectionSetup` fresh on the seller's stdIn.
- `stream-init-direction = seller` *(added 2026-05-08; pinned for back-compat with the existing JV-box edge-seller flow which dials AND opens the data stream from the seller side. DApps composing Profile E MAY override per-stream via 008 FR-P33a `streams[].direction` — for example, a buyer-initiated control stream `/jetvision/status/1.0.0` with `direction = "buyer-initiates"` is valid even though the profile-level default is `seller`.)*

## Optional capabilities

- `settlement ∈ {mock, evm-escrow}` — Profile E does not constrain settlement; mock for development, EVM-based escrow for production sessions per spec 008.
- `max-payload` is unconstrained at the profile level; bindings MAY declare per-stream limits (e.g., 65 KiB per delivery frame).
- `audit-trail = consensus-ordered-validator` (variant) — a validator agent (spec 010) subscribes to both parties' topic flows; this is encouraged for production deployments but not required for v1.

## Valid transport bindings

- **`T-QUIC`** — direct libp2p QUIC dial from seller to buyer over UDP; the reference binding for v1. Implementation: `internal/delivery/libp2p_adapter.go` + `internal/delivery/reverse_setup.go`.
- **`T-QUIC+IPv6`** — same as above, IPv6 path; treated as a separate binding only because IPv6-only buyer multiaddrs MAY work where IPv4 fails (and vice versa).
- (Not in v1) `T-WebTransport` — WebTransport from a browser-class buyer is technically expressible but not exercised by current implementations. Profile E does not exclude it; a future binding addition would need only a `capabilitiesProvided` declaration that satisfies the required vector above.

## Unsupported assumptions

- Inbound libp2p connectivity on the seller. Profile E exists precisely because that's not available.
- Direct consensus-anchored payment metering on the data plane. Settlement happens on the control plane, not inline with the BEAST/Mode-S frames.
- Subscription-shaped commerce (per-frame/per-second metering). Profile E v1 supports only single-agreement-per-session settlement; subscription mode is deferred to a future profile or capability extension.

## Expected failure modes

- Buyer becomes unreachable mid-session (DNS / outage / port change). Seller observes stream death; reconnect loop tries again with `cfg.ReconnectBackoff` between attempts.
- Buyer republishes `ReverseConnectionSetup` with new multiaddrs while the seller is dialing the old set. Seller falls back to its reconnect loop; the next iteration uses the latest setup observed on stdIn.
- Seller's NAT mapping expires (some carrier-grade NATs aggressively close UDP holes). Heartbeat traffic to HCS plus libp2p keepalives within the QUIC stream typically refresh the mapping; if not, the failure surfaces as stream death and the seller's reconnect loop kicks in.
- Buyer's heartbeat observer (per spec 005, optional via `BuyerConfig.EnforceDeadlines`) detects seller's `nextHeartbeatDeadline` lapse; closes the data stream pre-emptively even if libp2p keepalives are still passing.

## Conformance statement

An agent claims Profile E support by publishing a descriptor with `neuronProfileId = "e-natd-seller/1"` containing at least one `T-QUIC` (or `T-QUIC+IPv6`) binding whose `capabilitiesProvided` satisfies the required vector above. Both parties' descriptors must declare Profile E for a session to proceed; an initiator advertising Profile E and a responder advertising only Profile A is a mismatch (the responder cannot drive seller-initiated dial-out).

## Capability vector — machine-readable form

This vector is the descriptor's `capabilitiesProvided` for the seller side of a Profile E binding (e.g., `T-QUIC`). The buyer side mirrors all values except `listen-capability = listen`.

```json
{
  "neuronProfileId": "e-natd-seller/1",
  "control-plane": "topic",
  "audit-trail": "client-publish",
  "identity-lifetime": "persistent",
  "listen-capability": "outbound-dial-only",
  "nat-traversal": "outbound-dial-only",
  "settlement": "mock",
  "max-payload": 65536,
  "confidentiality": "transport+payload-ecies",
  "ordering": "fifo-per-stream",
  "reconnect-semantics": "seller-driven",
  "stream-init-direction": "seller"
}
```

The capability key `outbound-dial-only` is **not yet in the closed enumeration of `nat-traversal` or `listen-capability` values defined in `../spec.md`**. Profile E's ratification therefore requires either:

1. Extending the `../spec.md` enumeration to include `outbound-dial-only`, or
2. Mapping Profile E's seller side onto an existing value (e.g., `nat-traversal = none` interpreted as "the responder must be public — symmetric with Profile A's wording from the responder's POV").

Resolution is deferred to the `/speckit.clarify` cycle that integrates this profile into `spec.md`.

## Relationship to existing implementations

The current `cmd/edge-seller/` and `cmd/edge-buyer/` (under `impl/golang/`) implement Profile E in spirit. They predate this profile artifact and do not yet emit a profile descriptor at `/.well-known/neuron-profile.json` or as a stdOut TopicMessage; that wiring is part of demo phase D2 (see plan §15.8). Until that wiring lands, peers learn the seller's identity via the legacy `seller-bootstrap.json` file fetched out-of-band — a backward-compatibility path explicitly preserved by FR-CP-009 for ≥ N+2 release windows.

## Open questions to resolve in `/speckit.clarify`

1. Is the new value `outbound-dial-only` accepted into the `../spec.md` capability enumeration, or do we reuse `none` and document the asymmetry per profile entry?
2. Does Profile E need a normative section in `../spec.md`, or can it remain a peripheral profile defined entirely under `profiles/E-natd-seller.md`? The pattern set by A/C/D suggests integration is preferred.
3. Should the buyer's `EnforceDeadlines` behavior (spec 005 deadline observation per seller) be a profile-level required capability for Profile E, or remain optional?
4. Should `settlement = mock` be the default in v1, mirroring Profile A's stance, or default to `evm-escrow` reflecting the more-server-class nature of Profile E participants?

## Test references

Reference implementations + tests live in:

- `impl/golang/internal/edgeapp/` — `state.go`, `liveness.go`, `seller.go`, `buyer.go` (the persistent-identity + deadline-enforcement pieces wired in the spec-full-flow integration plan).
- `impl/golang/internal/delivery/reverse_setup.go` — buyer-encrypts-multiaddrs, seller-decrypts-and-dials primitives.
- `impl/golang/cmd/edge-{seller,buyer}/` — entry points implementing Profile E v1.

The Phase C.2 24-hour soak (currently active; see plan §14) is the production-shape Profile E run.
