# Profile F — Fixture Direct (demo / lab / TEVV-only)

> **Status:** Draft. This file is the artifact for Profile F pending its integration into `../spec.md` "Profile Definitions" section. Like Profile E, the profile is authoritatively defined here until a future `/speckit.propagate` cycle lifts the normative summary into `spec.md`. Until then, **this file** is the authoritative normative source for Profile F.
>
> **Origin**: EIP-8004 alignment audit, Amendment A (2026-05-12). The profile retroactively ratifies fixture-direct Remote ID and ADS-B evidence runs, now exercised by `cmd/multistream-buyer` plus the DApp sellers, the FID smoke test (`cmd/fid-display` + TaggedFrame output), and the `--mode=mock` path of `cmd/buyer-seller-demo`. Closes audit gap **G1** (direct-libp2p-dial outside any profile) and is the home for gaps **G6** (`--output=tcp:` / `file:` / `file+:` sinks), **G7** (`--listen` arbitrary multiaddr), and **G8** (`--key-hex` ephemeral identity) under one disclosed envelope.

## Overview

Profile F covers the **fixture-direct topology**: a buyer dials a seller via a static libp2p multiaddr provided out-of-band (printed to stdout, hand-edited `bootstrap.json`, an operator-pasted CLI flag) and the pair exercises the data plane **without** performing EIP-8004 registration, agent discovery via the Identity Registry, control-plane HCS topics, or 008 commerce negotiation / escrow / settlement.

Profile F exists to give evidence runs — CI, recorded demo videos, partner evidence captures — a normative anchor. Without it, every fixture-driven run is "ad-hoc demo code" and a validator (010) observing the absence of registry / topic / escrow events has to fall back to side knowledge to interpret the run.

Profile F is **not an operational deployment shape**. Profiles A, C, D, and E remain the only profiles authorised for shipped agents that transact value or carry production traffic. Profile F's defining property is that every long-running operational obligation that A/C/D/E impose (registry presence, heartbeat-anchored liveness on durable topics, escrow funding, agreement lifecycle) is intentionally absent. A run under Profile F MUST advertise the profile in heartbeat capabilities so observers can interpret the absence correctly.

Profile F is a structural complement — not a competitor — to A/C/D/E:

- **vs. Profile A** (browser → public listener): A's initiator is a browser; A still performs 008 negotiation via in-stream control-plane TopicMessages. F has no negotiation at all.
- **vs. Profile C** (any → NATed peer via relay): C uses a Circuit Relay V2 reservation; F uses neither relay nor reservation — the buyer's CLI is given the seller's multiaddr directly.
- **vs. Profile D** (peer ↔ peer direct): D requires both parties be persistently registered and exchange `serviceRequest` / `serviceResponse` over HCS topics. F skips 003 + 008 entirely.
- **vs. Profile E** (NATed seller → public buyer): E inverts who dials but still runs the full 008 commerce flow over HCS; F runs the data plane in isolation.

## Runtime contexts

- **Initiator (buyer)**: any runtime with libp2p QUIC outbound dial capability — a server process, a CI runner, a developer workstation. The buyer takes the seller's multiaddr from the command line (`--seller=<multiaddr>`), an environment variable, or a hand-edited `bootstrap.json`. The buyer does NOT call `LookupRegistration` against any Identity Registry and does NOT consume a `connectionSetup` payload from the seller's stdIn topic — there is no stdIn topic.
- **Responder (seller)**: a libp2p host that has called `host.Listen(multiaddr)` and prints (or otherwise publishes out-of-band) its resulting public multiaddrs. The seller does NOT register a `neuron-commerce` service via 003 / 007 and does NOT publish any heartbeat that claims operational settlement. The seller MAY still publish a heartbeat advertising `profile = "f-fixture-direct/1"` to a local or test-only TopicAdapter for observability.

Either party MAY hold a persistent secp256k1 identity, but Profile F explicitly authorises `--key-hex=<seed>` ephemeral identities (002 FR-K05 short-lived key pattern, mirrored from Profile A browser buyers). Ephemeral identity is a defining feature of fixture runs — it documents that "this is not a registered, accountable agent."

## Required capabilities

- `control-plane = out-of-band` — there is no in-stream control-plane and no topic control-plane. The seller's multiaddr is exchanged through whatever channel the operator chose (`--seller` flag, printed stdout, `bootstrap.json`, pasted message). This value is new in Profile F (see "Capability vocabulary extension" below). Validators MUST NOT attempt to verify control-plane behaviour against the A/D `in-stream` / `topic` enumerations under Profile F.
- `audit-trail = none` — neither party is required to publish heartbeats, negotiation envelopes, or invoices to a durable log. Optional advisory heartbeats per FR-F-04 below are still permitted.
- `identity-lifetime ∈ {ephemeral, persistent}` — both are valid under Profile F. The defining feature is that ephemeral identity is *explicitly permitted*, unlike A (initiator-only ephemeral), C/D/E (persistent both sides).
- `listen-capability = any` — initiator dials; responder listens; no relay reservation required. Both parties MAY listen if a future bidirectional fixture is needed; v1 of this profile keeps the asymmetry for clarity.
- `nat-traversal = explicit-multiaddr` — the operator selects which addresses to dial. If the seller's multiaddrs include a private RFC 1918 address, the buyer is responsible for being on the same network. There is no DCUtR upgrade, no relay reservation, no STUN/TURN. New value added in Profile F (see "Capability vocabulary extension" below).
- `settlement = n/a` — no escrow, no invoice, no settlement state machine. New value added in Profile F (see "Capability vocabulary extension" below).
- `max-payload` is stream-dependent and inherited from whatever stream protocol the run exercises; Profile F imposes no profile-level ceiling.
- `confidentiality = transport-only` — libp2p Noise / QUIC TLS over the dialled stream. No ECIES multiaddr encryption because no `connectionSetup` payload is exchanged.
- `ordering = fifo-per-stream` — the underlying libp2p QUIC stream's guarantee.
- `reconnect-semantics = full-reneg` — fixture runs are short-lived; the buyer is expected to re-invoke its CLI to re-establish, not to maintain resume tokens. Mirrors Profile A v1's stance.
- `stream-init-direction = either` — default for new profiles per the 2026-05-08 v2 amendment of `../spec.md`. The CLIs `cmd/remoteid-seller` and `cmd/multistream-buyer` use seller-initiated Remote ID streams today; nothing in Profile F prevents `buyer` or `either` for future fixture streams.

## Optional capabilities

- `audit-trail = client-publish` (variant) — an operator MAY wire Profile F to a sandbox TopicAdapter (e.g., `MemoryTopicBus` or a non-production HCS topic) for development observability. When advertised, the heartbeat MUST carry both `profile = "f-fixture-direct/1"` and the topic address (so observers can join). The topic MUST NOT be the production agent's stdIn / stdOut / stdErr; topic addresses under Profile F MUST be deployment-config-distinct from production topics per FR-F-03 below.
- `feedSource ∈ {live, replay, synthetic, placeholder}` — Profile F deliberately defers feed-source semantics to the DApp specs (016 Appendix "Feed Source Variations" and 017 Appendix "Feed Source Variations", per the same 2026-05-12 amendment pass). When a DApp seller runs under Profile F it MUST honour the DApp's feed-source advertisement rule.

## Valid transport bindings

- **`T-QUIC`** — direct libp2p QUIC dial from buyer to seller (or seller to buyer, where the multiaddr exchange direction is inverted). Reference binding for v1; exercised by `cmd/multistream-buyer --seller=role=remoteid,multiaddr=<quic-multiaddr>,protocol=/ds240/basestation/1.0.0` and `cmd/buyer-seller-demo --mode=mock` (in-process loop).
- **`T-QUIC+IPv6`** — same as above, IPv6 path. Treated as a separate binding only because IPv6-only loopback addresses MAY work where IPv4 fails in some containerised dev setups.
- **`T-TCP-Noise`** — libp2p TCP+Noise; permitted but uncommon in current fixtures. The 009 `DeliveryAdapter` supports it; no profile-specific constraints.
- (Not in v1) `T-WebTransport` and `T-WSS` — feasible for browser-initiated fixtures but not exercised by the current fixture CLIs. Browser fixtures historically use Profile A with a public listener; adding them to Profile F would require a separate appendix.
- (Not in v1) `T-Relay` — Profile F is the no-relay fixture profile. A relay-assisted fixture would be Profile C with the same disclosure discipline applied to its heartbeats.

## Unsupported assumptions

- Any expectation that the buyer can `LookupRegistration(sellerEVM)` against the Identity Registry. There is no registration under Profile F.
- Any expectation that the seller's `agentURI.services[]` contains a `neuron-commerce` entry. The seller does not write an agentURI.
- Any expectation that an `evidenceHash` chain exists for the run. There are no `invoice` messages, so the validator MUST treat `evidenceHash` absence as expected per FR-F-05 below.
- Any expectation that a `serviceStop` / `serviceCancel` / `serviceRenew` lifecycle message will appear before stream closure (008 FR-P36 / FR-P37 / FR-P38). The data-plane stream simply closes when the buyer exits or the seller is terminated.
- Any cross-run reproducibility beyond what the underlying fixture (replay file, synthetic generator, placeholder decoder) provides.

## Expected failure modes

- Operator pastes a stale multiaddr (e.g., from a previous seller invocation): the buyer's libp2p dial fails. Recovery: re-run the seller, copy the new multiaddr.
- Seller's `--listen` binds to a private interface the buyer cannot reach (e.g., `--listen=/ip4/127.0.0.1/...` while the buyer is on another host): buyer dial fails. Recovery: re-bind to a reachable interface.
- Ephemeral identity collision in the (highly unlikely) case of two parallel fixture runs reusing the same `--key-hex` seed: the libp2p PeerID match is structurally harmless but operationally confusing. Recovery: omit `--key-hex` to get a fresh random key, or pick distinct seeds.
- Buyer or seller emits a heartbeat without `profile = "f-fixture-direct/1"`: an evidence reviewer reading the heartbeat cannot tell the run is fixture-mode and may incorrectly classify it as operational. This is a disclosure-discipline failure; FR-F-02 below makes the disclosure mandatory.

## Profile-specific Functional Requirements (`FR-F-*`)

These FRs are normative within Profile F. They are scoped per-profile so as not to collide with the spec-wide `FR-CP-*` numbering.

- **FR-F-01** (Profile scope): An agent operating under Profile F MUST NOT execute the 003 registration round-trip (no `register()` call, no `agentURI` write), MUST NOT mint or update an EIP-8004 NFT under 007, and MUST NOT advance an 008 agreement state machine. The data plane MAY operate end-to-end over a libp2p stream; the protocol-level commerce flow is intentionally absent.
- **FR-F-02** (Mandatory disclosure): An agent operating under Profile F MUST advertise `profile = "f-fixture-direct/1"` in any heartbeat capabilities (005 FR-H05) it publishes during the run. If the agent publishes no heartbeats (the minimum legitimate case under Profile F), every TEVV evidence artefact that cites the run MUST disclose the profile in the artefact header — verification reports, CI artefact bundles, recorded demo annotations, and partner-facing capture writeups all qualify. The four-label source-mode vocabulary (live / replay / synthetic / placeholder) interacts with this disclosure additively, not multiplicatively: a Profile F run MAY also be a replay run.
- **FR-F-03** (Topic / contract isolation): A run under Profile F MUST NOT publish to the same Hedera testnet HCS topics, MUST NOT use the same EIP-8004 Identity Registry contract address, and MUST NOT use the same EVM Escrow contract address as production traffic. Topic IDs, contract addresses, and chain IDs MUST be deployment-config-distinct. This requirement applies even when an optional advisory TopicAdapter is wired (see "Optional capabilities" above).
- **FR-F-04** (Optional advisory heartbeat): An agent MAY publish heartbeats to a non-production TopicAdapter for development observability. When published, the heartbeat MUST carry `profile = "f-fixture-direct/1"` (FR-F-02) and SHOULD additionally carry the DApp's `feedSource` advertisement (016 FR-A18 / 017 FR-R15) so the reviewer can correlate Profile F with the source mode.
- **FR-F-05** (Validator interpretation): A validator agent (010) observing a Profile F run MUST interpret the absence of `serviceRequest` / `serviceResponse` / `connectionSetup` / `escrowCreated` / `invoice` / `invoiceAck` / `serviceStop` / `serviceCancel` / `serviceRenew` envelopes (008 FR-P06 commerce taxonomy) as **expected**, not as non-compliance. The validator's verdict SHOULD record the run as `inconclusive-by-profile` rather than `non-compliant`. A validator MAY still issue a `non-compliant` verdict if FR-F-02 (disclosure) is violated.
- **FR-F-06** (CLI canonicalisation): `--seller=<multiaddr>` (buyer side) and `--listen=<multiaddr>` (seller side) are the canonical CLI surface for Profile F. `--key-hex=<seed-hex>` is the canonical ephemeral-identity surface. `--output=stdout` / `--output=file:<path>` / `--output=file+:<path>` / `--output=tcp:<host>:<port>` are the canonical fixture sinks (originally defined by the superseded 018 fid-fusion draft; this profile is now their normative home, documented in `docs/fid-display-contract.md`). New CLI surfaces in fixture binaries SHOULD reuse these flag names so that an operator can move between fixture binaries without re-learning the flag vocabulary.

## Capability vocabulary extension

Profile F introduces three new values into existing capability keys (per `../spec.md` FR-CP-003 minor-version amendment — adding values to existing keys is a minor amendment, not a major one):

- `control-plane = out-of-band` — new value for `control-plane`. Allowed only under Profile F. Semantic: "control-plane information (seller multiaddr, identities) is exchanged outside the protocol — operator CLI, hand-edited file, pasted message."
- `nat-traversal = explicit-multiaddr` — new value for `nat-traversal`. Allowed under Profile F (and trivially under Profile D where both parties are public, though Profile D continues to use `none`). Semantic: "the dialled multiaddr is supplied directly; no traversal protocol is engaged."
- `settlement = n/a` — new value for `settlement`. Allowed only under Profile F. Semantic: "no escrow, no settlement state machine."

`listen-capability = any` is not a new value; it is a shorthand for "the binding may use `listen` or `dial-only` per role" interpreted at validation time. The `audit-trail = none` value is already defined (Profile A optional variant).

These additions are minor-version amendments to spec 013 vocabulary per FR-CP-003. If `../spec.md` is amended in a later pass to record these vocabulary additions normatively, the table in `../contracts/capability-vector.md` should be updated correspondingly.

## Conformance statement

An agent claims Profile F support by either (a) publishing a descriptor at the well-known endpoint (FR-CP-013) with `neuronProfileId = "f-fixture-direct/1"` containing at least one `T-QUIC` (or `T-QUIC+IPv6` / `T-TCP-Noise`) binding whose `capabilitiesProvided` satisfies the required vector below, OR (b) operating purely from CLI flags without an endpoint and disclosing the profile per FR-F-02 in TEVV evidence artefacts. The latter path is the common case for the fixture CLIs in this repository today.

An agent MUST NOT advertise Profile F support in a descriptor that is also advertised at an EIP-8004 `agentURI.services[].profileSupport` field, because Profile F runs do not register under 007 by definition. If a deployment wishes to expose a fixture path through its production registry surface, it MUST use a separate ephemeral agent identity that registers under a non-production contract address (FR-F-03).

## Capability vector — machine-readable form

This vector is the descriptor's `capabilitiesProvided` for the buyer side of a Profile F binding (e.g., `T-QUIC`). The seller side mirrors all values except `listen-capability = listen`.

```json
{
  "neuronProfileId":      "f-fixture-direct/1",
  "control-plane":        "out-of-band",
  "audit-trail":          "none",
  "identity-lifetime":    "ephemeral",
  "listen-capability":    "dial-only",
  "nat-traversal":        "explicit-multiaddr",
  "settlement":           "n/a",
  "max-payload":          0,
  "confidentiality":      "transport-only",
  "ordering":             "fifo-per-stream",
  "reconnect-semantics":  "full-reneg",
  "stream-init-direction": "either"
}
```

The seller-side vector is identical except `listen-capability = listen`. `max-payload = 0` is the sentinel meaning "no profile-level ceiling; inherited from the stream binding."

## Relationship to existing implementations

The following binaries operate under Profile F today (de facto; Amendment A retroactively ratifies the topology):

- `impl/golang/cmd/remoteid-seller` + `impl/golang/cmd/multistream-buyer` — the current fixture-direct Remote ID path. The seller's `--listen=<multiaddr>` and the buyer's repeated `--seller=role=...,multiaddr=...,protocol=...` flags are the canonical Profile F flags. Both binaries accept `--key-hex=<seed>` for ephemeral identity. In fixture-direct mode neither binary calls 003 / 007 / 008.
- `impl/golang/cmd/buyer-seller-demo` invoked with `--mode=mock` — the in-process loop that runs seller + buyer + memory bus + memory escrow. Useful for `make demo`. The "memory bus" is the local sandbox TopicAdapter mentioned in "Optional capabilities".
- `impl/golang/cmd/fid-display` invoked together with `cmd/multistream-buyer --output=tcp:...` — the current consolidated FID smoke test. Profile F applies on the multistream buyer side; the FID display is a consumer that does not participate in the libp2p layer.

Profile F formalises the fixture-direct deployment shape that earlier ran ad hoc. The edge-demo env-var ladder is **not** covered by Profile F: the edge-* binaries can be reconfigured to run as full Profile E with `NEURON_EDGE_REGISTRATION_MODE=force-testnet` + `NEURON_EDGE_PAYMENT_MODE=testnet`. Profile F is the home for binaries whose **design** is fixture-mode, not for operational binaries running in a degraded posture.

## Anti-scope

- Profile F MUST NOT be claimed by any agent that takes real payment or carries real partner traffic.
- Profile F MUST NOT be conflated with `--mode=mock` of `cmd/buyer-seller-demo` when running with a real testnet topic adapter (i.e. `--mode=testnet`); the latter is a Profile A or D shape running against testnet infrastructure, not Profile F.
- Profile F does NOT prescribe a fixture transport sink for emitted application data; the `--output=tcp:` / `file:` / `file+:` / `stdout` sink vocabulary is canonicalised by FR-F-06 above (documented in `docs/fid-display-contract.md`).

## Open questions to resolve in `/speckit.clarify`

1. Should Profile F's optional advisory heartbeat be permitted on a public testnet TopicAdapter, or strictly confined to local `MemoryTopicBus`? FR-F-03 currently allows non-production testnet topics if isolated; a stricter reading would say "local memory only." Partner evidence runs prefer the looser reading; CI runs prefer the stricter reading.
2. Should `control-plane = out-of-band`, `nat-traversal = explicit-multiaddr`, and `settlement = n/a` be ratified into `../spec.md` Capability Vocabulary as part of the next minor-version bump (FR-CP-003 minor amendment)? This file's normative status currently carries them; promoting them to `spec.md` makes them discoverable by the descriptor validator.
3. Should fixture identities be required to derive from a documented test-seed register (e.g., DEAD-BEEF-CAFE seeds) to ensure no fixture identity is accidentally reusable as production? The current text allows any ephemeral identity, which is operationally fine but evidence-wise weaker.

## Test references

- `impl/golang/cmd/remoteid-seller/main.go:46-62` — CLI flag surface for Profile F seller.
- `impl/golang/cmd/multistream-buyer/main.go` — CLI flag surface for the Profile F buyer.
- `impl/golang/cmd/buyer-seller-demo/main.go` `--mode=mock` branch — in-process Profile F loop.
- `impl/golang/internal/dapp/remoteid/seller.go` plus `impl/golang/cmd/multistream-buyer/main.go` — Profile F-shaped seller / buyer runtime (no 008 commerce).
