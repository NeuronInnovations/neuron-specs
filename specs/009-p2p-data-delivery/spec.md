# Feature Specification: P2P Data Delivery

**Feature Branch**: `009-p2p-data-delivery`
**Created**: 2026-03-26
**Status**: Draft

## Related Specs

**In-repo:**

- **002 Key Library**: NeuronPublicKey, NeuronPrivateKey, PeerID derivation (FR-006), secp256k1 keys (FR-003), signature (FR-014/017)
- **004 Topic System**: TopicMessage envelope (FR-T02/T03), stdIn channel (FR-T07), payload extensibility (FR-T20), `neuron-p2p-exchange` service (FR-T17), `topicRef` cross-reference (FR-T18)
- **006 Protocol Determinism**: Canonical JSON (FR-W01–W10), binary encoding (FR-W03), error taxonomy (NEURON-{DOMAIN}-{NNN})
- **008 Payment**: `delivery` descriptor (FR-P01a/P01b), `connectionSetup` message (FR-P33), encryption requirement (FR-P34), delivery-mode conditionality (FR-P35), lifecycle clarification (FR-P14a)

**External:**

- **libp2p** (https://libp2p.io/docs/): QUIC, WebRTC, WebTransport transports; AutoNAT, Circuit Relay v2, DCUtR for NAT traversal; multistream-select for protocol negotiation; PeerID identity model
- **ECIES** (IEEE 1363a / SECG SEC-1): Elliptic Curve Integrated Encryption Scheme for multiaddr confidentiality
- **RFC 4648**: Base64 encoding for binary fields in canonical JSON

## Clarifications

### Session 2026-03-26

- Q: Should backoff timing (FR-D09: 5s/10min/1hr) be more conservative like old SDK (30s/12hr)? → A: Keep aggressive defaults for real-time services. 5s initial, 2x factor, 10min cap, 1hr max confirmed.
- Q: Should max frame size (FR-D22) be smaller (1 MiB) or larger (16 MiB)? → A: 4 MiB confirmed. Good general-purpose default for ADSB, sensor, small video.
- Q: Should ECIES HKDF info string include requestId for cross-agreement replay prevention? → A: Static info `"neuron-multiaddr-v1"` only. Ephemeral keys (FR-D13) already prevent replay. Keep simple.
- Q: Should VR-DEL-04 diagnostic endpoint be MUST-level? → A: Keep optional. Delivery channel is private by design. Validators rely on evidenceHash (008 FR-P20).

### Session 2026-05-08

- Q: The current spec's User Story 1 says "the seller establishes a direct P2P delivery channel to the buyer", and FR-D15 has the receiver of `connectionSetup` initiate `connect(...)`. This conflates connection-establishment direction with stream-initiation direction. Either party should be able to open streams once the libp2p connection exists. → A: Confirmed conflation. Splitting into two distinct concepts: **connection direction** (who dials whom; governed by FR-D15 and per-profile descriptor in 013) and **stream-init direction** (who calls `OpenStream` for a given declared stream; governed by new FR-D-stream-direction and the `streams[].direction` field of `connectionSetup` per 008 FR-P33a). The two concepts are independent. Profile E reverse-connect (seller dials buyer, seller opens primary stream) remains a valid concrete shape but is now expressed as `direction = seller-initiates` on the relevant stream catalog entry, not as an implicit assumption.
- Q: `BuildConnectionSetup` advertises `host.Addrs()`, but if the host listens only on a public interface, only one address is sent. Should we mandate listening on all interfaces, and should we filter out loopback / Docker / virtual-bridge addresses? → A: Listen broadly, advertise selectively. The agent SHOULD listen on all interfaces (`/ip4/0.0.0.0/...`, `/ip6/::/...`) so `host.Addrs()` enumerates the full reachable set. The advertised multiaddr array MUST include LAN (RFC1918), public (globally routable), and active relay-circuit addresses. The advertised array MUST exclude: loopback addresses (`127.0.0.0/8`, `::1`), Docker bridge default ranges (`172.16.0.0/12` when sourced from a `docker0`-style virtual interface, or any address sourced from a `veth*` / `br-*` interface name), point-to-point virtual interfaces (`utun*`, `tun*`, `tap*`), and link-local addresses (`169.254.0.0/16`, `fe80::/10`). Codified as FR-D11 update + new FR-D11a.
- Q: Should multiple stream variants per service (raw, filtered/100, filtered/200, status) be supported? → A: Yes, and via path-based protocol IDs as separate libp2p streams: unlike HTTP query parameters (`?filter-altitude=100`), `/filter/altitude/100` and `/filter/altitude/200` are distinct streams with separate stream handlers. Adding new FR-D-multi-protocol (a peer MAY register N protocol IDs concurrently) and FR-D-wildcard-handler (registration of wildcard patterns such as `/jetvision/filtered/*` with runtime parameter parsing). The `streams[]` catalog in 008 FR-P33a is the on-the-wire advertisement; this spec defines the libp2p binding semantics.
- Q: Should the SDK expose libp2p pubsub / floodsub / gossipsub? → A: Yes, as **primitives** only. The Core SDK exposes them as substrate (per Constitution Principle XII). No fan-out topology, mesh parameters, copy/drop semantics, or distribution-relay roles are prescribed at this layer. DApp specs (016 ADS-B, 017 Remote ID, future) compose these primitives into application-specific fan-out behavior. Codified as new FR-D-pubsub-primitives.
- Q: When the topic backend (HCS) is down, should the data plane keep flowing? → A: Yes. The data plane MUST be independent of control-plane availability. Mirrors 008 Section K (Degraded-Mode Operation). Codified as new FR-D-degraded.

## Out of Scope

- **Payment and negotiation**: Service discovery, negotiation sub-protocol, agreement lifecycle, escrow operations, invoice cycle. Defined in Spec 008.
- **Topic-based delivery internals**: Publishing and subscribing to topic channels for `delivery.mode: "topic"` is handled by Spec 004 TopicAdapter. This spec covers only `delivery.mode: "p2p"`.
- **Service-specific data schemas**: What data a service produces (ADSB, radiation, video) is a dApp/termsRef concern. This spec defines the delivery channel, not the content.
- **On-chain contracts**: Identity, Reputation, and Validation registries. Defined in Spec 007.
- **Stream content encryption**: End-to-end encryption of service data payloads. libp2p transports provide transport-layer encryption (TLS 1.3 / Noise). Application-layer content encryption is dApp-level.
- **Relay node operation**: How to deploy and operate public relay infrastructure. This spec defines how agents use relays, not how relays are administered.

## Assumptions

- Agents have completed identity registration (003) and have a `neuron-p2p-exchange` service in their agentURI with a valid PeerID (002 FR-006).
- The agreement lifecycle (008) has reached AGREED state and a `connectionSetup` message (008 FR-P33) has been exchanged before delivery channel establishment.
- The underlying network supports UDP (for QUIC) or TCP (for WebSocket fallback). Agents behind restrictive firewalls that block all UDP and TCP outbound cannot participate in P2P delivery without relay assistance.
- libp2p is the implementation substrate for P2P delivery. The abstract DeliveryAdapter interface permits future non-libp2p bindings, but the normative first binding is libp2p.

---

## User Scenarios & Testing _(mandatory)_

### User Story 1 — Establish a P2P Delivery Channel After Agreement (Priority: P1)

After a buyer and seller reach agreement on a service (008 AGREED state) and exchange `connectionSetup` messages, the seller establishes a direct P2P delivery channel to the buyer and begins streaming service data.

**Why this priority**: Without a delivery channel, the agreed service cannot be fulfilled. This is the core data plane operation that completes the commerce loop started by Spec 008.

**Independent Test**: Can be fully tested by running two agents on the same network. Agent A (seller) and Agent B (buyer) exchange `connectionSetup` messages with multiaddrs. Seller dials buyer via libp2p, opens a stream tagged with the agreed protocol ID, and sends test data. Buyer receives the data on the stream. No payment or escrow is required — delivery channel operates independently.

**Acceptance Scenarios**:

1. **Given** a buyer has sent a `connectionSetup` message with their PeerID and encrypted multiaddrs, **When** the seller decrypts the multiaddrs and dials the buyer via the DeliveryAdapter, **Then** a bidirectional delivery channel is established and the seller can send service data.
2. **Given** an established delivery channel, **When** the seller sends a sequence of data frames, **Then** the buyer receives them in order with no data loss or corruption.
3. **Given** an established delivery channel, **When** either party calls `disconnect`, **Then** the channel is closed cleanly and both sides are notified.

---

### User Story 2 — Encrypt and Decrypt connectionSetup Multiaddrs (Priority: P1)

A peer encrypts their multiaddress list using the counterparty's NeuronPublicKey so that only the intended recipient can learn the sender's network addresses, even though the `connectionSetup` message is published on a public topic.

**Why this priority**: This satisfies the security-critical requirement 008 FR-P34. Without this, peer network addresses are publicly exposed.

**Independent Test**: Can be tested by encrypting a known multiaddr list with a test NeuronPublicKey, then decrypting with the corresponding NeuronPrivateKey. Verify: ciphertext is not the same as plaintext, decryption recovers the original multiaddrs, decryption with a wrong key fails with an authenticated-encryption error.

**Acceptance Scenarios**:

1. **Given** a list of multiaddrs and the counterparty's NeuronPublicKey, **When** the sender encrypts them using the ECIES profile defined by this spec, **Then** the resulting `encryptedMultiaddrs` base64 string can only be decrypted by the holder of the corresponding NeuronPrivateKey.
2. **Given** an `encryptedMultiaddrs` field, **When** a third party with a different NeuronPrivateKey attempts decryption, **Then** the operation fails with a `ConnectionSetupEncryptionFailed` error (008 FR-P32).
3. **Given** the same multiaddrs and the same NeuronPublicKey, **When** encrypted twice, **Then** the ciphertexts differ (randomized encryption — no deterministic oracle).

---

### User Story 3 — NAT Traversal with Automatic Fallback (Priority: P2)

A seller attempts to dial a buyer who is behind a NAT. The system automatically detects the NAT situation, falls back to a relay, and then attempts to upgrade to a direct connection.

**Why this priority**: Many real-world agents operate behind NATs. Without NAT traversal, P2P delivery only works on public networks — severely limiting deployment.

**Independent Test**: Can be tested by running two agents where at least one has a non-routable address (e.g., behind a simulated NAT). Verify the system detects unreachability, establishes a relay path, and attempts DCUtR upgrade.

**Acceptance Scenarios**:

1. **Given** a buyer behind a NAT whose multiaddrs are not directly reachable, **When** the seller's direct dial fails, **Then** the DeliveryAdapter automatically attempts connection via a Circuit Relay v2 node.
2. **Given** a relay-connected delivery channel, **When** both peers support DCUtR, **Then** the system attempts to upgrade the relayed connection to a direct connection via coordinated hole punching.
3. **Given** a relay-connected delivery channel where DCUtR fails, **When** the seller sends data, **Then** data flows through the relay path and the buyer receives it (degraded but functional).

---

### User Story 4 — Reconnection After Connection Loss (Priority: P2)

An established delivery channel drops due to a network interruption. The system automatically attempts to re-establish the channel using exponential backoff.

**Why this priority**: Network interruptions are common in real deployments. Without reconnection, every transient failure terminates the service agreement.

**Independent Test**: Can be tested by establishing a delivery channel, simulating a network drop (close the underlying transport), and verifying that the DeliveryAdapter attempts reconnection with backoff and eventually re-establishes the channel when the network recovers.

**Acceptance Scenarios**:

1. **Given** an active delivery channel, **When** the underlying transport connection drops, **Then** the DeliveryAdapter transitions to RECONNECTING state and begins reconnection attempts with exponential backoff.
2. **Given** a RECONNECTING delivery channel, **When** the network recovers and a new connection succeeds, **Then** the channel transitions to CONNECTED and data flow resumes.
3. **Given** a RECONNECTING delivery channel, **When** reconnection attempts exceed the maximum backoff duration, **Then** the channel transitions to DISCONNECTED and the application layer is notified.

---

### User Story 5 — Multi-Transport Delivery (Priority: P3)

A seller offers a service to buyers on different platforms — a server-based IoT agent uses QUIC, while a browser-based dashboard uses WebRTC. The DeliveryAdapter selects the appropriate transport based on the buyer's multiaddrs.

**Why this priority**: Multi-transport support enables browser agents and cross-platform delivery, but the core P2P flow works with QUIC alone.

**Independent Test**: Can be tested by running a seller agent that advertises both QUIC and WebRTC multiaddrs, then connecting from two buyers — one via QUIC (server) and one via WebRTC (browser). Verify both channels deliver data correctly.

**Acceptance Scenarios**:

1. **Given** a buyer advertising QUIC multiaddrs (`/ip4/.../udp/.../quic-v1`), **When** the seller dials, **Then** a QUIC-based delivery channel is established.
2. **Given** a buyer advertising WebRTC multiaddrs (`/webrtc/...`), **When** the seller dials, **Then** a WebRTC-based delivery channel is established via relay signaling.
3. **Given** a buyer advertising both QUIC and WebTransport multiaddrs, **When** the seller dials, **Then** the DeliveryAdapter selects the most efficient transport (QUIC preferred over WebTransport).

---

### Edge Cases

- What happens when the seller receives a `connectionSetup` with multiaddrs for a transport they do not support? The DeliveryAdapter MUST skip unsupported multiaddrs and attempt only those matching configured transports. If no compatible multiaddr exists, produce a `NoCompatibleTransport` error.
- What happens when decryption of `encryptedMultiaddrs` succeeds but the contained data is not valid multiaddr format? The DeliveryAdapter MUST validate multiaddr structure after decryption. Invalid multiaddrs produce an `InvalidMultiaddr` error.
- What happens when both parties send `connectionSetup` simultaneously? Both sides attempt to dial. libp2p handles simultaneous connect gracefully — one connection is kept, the duplicate is closed by the multiplexer.
- What happens when a delivery channel is CONNECTED but no data is sent for an extended period? The DeliveryAdapter MUST NOT assume the channel is dead based on data inactivity alone. Liveness is managed by Spec 005 heartbeats on the control plane. The delivery channel remains open until explicitly closed or the transport detects a failure.
- What happens when a relay node becomes unavailable mid-session? If the delivery channel was relay-only (DCUtR failed), the channel transitions to RECONNECTING. The agent discovers a new relay and re-establishes the path.

---

## Requirements _(mandatory)_

### Functional Requirements

**A. DeliveryAdapter Interface**

- **FR-D01**: System MUST define an abstract `DeliveryAdapter` interface with five operations that all delivery bindings MUST implement: `connect`, `send`, `receive`, `disconnect`, `getStatus`.
- **FR-D02**: `connect(peerID, multiaddrs[], protocol, options?) → DeliveryChannel | Error` MUST establish a delivery channel to the specified peer. The `peerID` is the counterparty's libp2p PeerID (002 FR-006). The `multiaddrs` are the decrypted addresses from `connectionSetup`. The `protocol` is the stream protocol ID string (e.g., `"/neuron/adsb/1.0.0"`). The returned `DeliveryChannel` is an opaque handle used by subsequent operations.
- **FR-D03**: `send(channel, data) → SendResult | Error` MUST transmit a data frame over the delivery channel. The `data` is opaque bytes — the DeliveryAdapter does not inspect or validate content. The `SendResult` includes a `bytesSent` count.
- **FR-D04**: `receive(channel) → AsyncStream<DataFrame> | Error` MUST return an asynchronous stream of data frames received on the delivery channel. Each `DataFrame` includes `data` (bytes) and `receivedAt` (timestamp).
- **FR-D05**: `disconnect(channel) → Result | Error` MUST close the delivery channel gracefully. Both sides MUST be notified of the closure. After disconnect, subsequent `send` or `receive` calls on the same channel MUST return a `ChannelClosed` error.
- **FR-D06**: `getStatus(channel) → ChannelStatus` MUST return the current state of the delivery channel without side effects. `ChannelStatus` includes `state` (ConnectionState enum) and `transport` (transport identifier string).

**B. Connection Lifecycle**

- **FR-D07**: System MUST implement a connection lifecycle state machine with these states: IDLE, CONNECTING, CONNECTED, RECONNECTING, RELAYING, DISCONNECTED.
- **FR-D08**: State transitions MUST follow these rules:
  - IDLE → CONNECTING: `connect` called
  - CONNECTING → CONNECTED: direct dial succeeds
  - CONNECTING → RELAYING: direct dial fails, relay connection succeeds
  - CONNECTING → DISCONNECTED: all connection attempts exhausted
  - CONNECTED → RECONNECTING: transport connection drops
  - RELAYING → CONNECTED: DCUtR upgrade succeeds
  - RELAYING → RECONNECTING: relay connection drops
  - RECONNECTING → CONNECTED: reconnection succeeds (direct)
  - RECONNECTING → RELAYING: reconnection succeeds (via relay)
  - RECONNECTING → DISCONNECTED: max backoff exceeded
  - CONNECTED → DISCONNECTED: `disconnect` called or fatal error
  - RELAYING → DISCONNECTED: `disconnect` called or fatal error
- **FR-D09**: Reconnection MUST use exponential backoff: initial delay of 5 seconds, factor of 2, maximum delay cap of 10 minutes, maximum total reconnection duration of 1 hour. After the maximum duration, the channel transitions to DISCONNECTED.
- **FR-D10**: Connection state changes MUST be observable by the application layer via a callback or event mechanism. The exact API shape is implementation-defined, but the events MUST include the new state and the reason for the transition.

**C. Multiaddr Encryption (ECIES Profile)**

- **FR-D11**: System MUST define an ECIES (Elliptic Curve Integrated Encryption Scheme) profile for encrypting `connectionSetup.encryptedMultiaddrs` (satisfying 008 FR-P34). The profile MUST use: secp256k1 ECDH for key agreement, HKDF-SHA256 for key derivation (salt = empty, info = `"neuron-multiaddr-v1"`), AES-256-GCM for authenticated encryption.
- **FR-D11a** (2026-05-08 amendment; multiaddr advertisement rules): The plaintext multiaddr array carried inside `encryptedMultiaddrs` MUST include every reachable binding the publisher is willing to be reached on for this agreement, drawn from the host's listen set. The publisher SHOULD listen on all interfaces (`/ip4/0.0.0.0/...` for IPv4, `/ip6/::/...` for IPv6) so that `host.Addrs()` (or the equivalent listen-set enumeration in non-libp2p bindings) yields the full reachable set, then apply the filter rules below before encryption. The publisher MUST include in the advertised array:
  - RFC1918 private LAN addresses when the publisher is reachable on a LAN segment (e.g., `/ip4/10.x.x.x/...`, `/ip4/172.16-31.x.x/...`, `/ip4/192.168.x.x/...`).
  - Globally-routable public addresses (IPv4 and IPv6).
  - Any active Circuit Relay v2 reservation circuits the publisher holds (per 011), expressed as `/ip4/<relay-public-ip>/.../p2p/<relay-peer-id>/p2p-circuit/p2p/<self-peer-id>` or the IPv6 equivalent.
  The publisher MUST exclude from the advertised array:
  - Loopback addresses (`127.0.0.0/8`, `::1`).
  - IPv4 link-local (`169.254.0.0/16`) and IPv6 link-local (`fe80::/10`).
  - Docker default bridge ranges sourced from `docker0`-style virtual interfaces (`172.17.0.0/16` on a `docker0` interface), Kubernetes pod CIDR sourced from a `cni*`/`flannel*`/`calico*` interface, and any address whose source interface name matches `veth*`, `br-*`, `cni*`, `flannel*`, `calico*`, `vxlan*`, `cilium*`.
  - Point-to-point virtual interfaces with names matching `utun*`, `tun*`, `tap*`, `wg*` (WireGuard) unless the operator explicitly opts in by configuration.
  Filtering rules are evaluated against the source interface (where available from the OS network stack) and the address itself; addresses that satisfy ANY exclusion criterion MUST be dropped before encryption. The order of remaining addresses in the array is implementation-defined, but the same address SHOULD NOT appear twice. Rationale: advertising loopback or docker addresses leaks internal infrastructure information into the encrypted-but-decryptable-by-counterparty payload, and the counterparty cannot route to those addresses anyway.
- **FR-D12**: Encryption input: a JSON array of multiaddr strings (e.g., `["/ip4/1.2.3.4/udp/4001/quic-v1"]`), serialized as UTF-8 bytes. Encryption output: a single opaque byte string (ephemeral public key ‖ nonce ‖ ciphertext ‖ authentication tag), encoded as base64 per 006 FR-W03.
- **FR-D13**: The ephemeral key pair MUST be freshly generated for each encryption operation. This ensures ciphertexts are randomized — encrypting the same multiaddrs twice MUST produce different ciphertexts.
- **FR-D14**: Decryption MUST verify the authentication tag before returning plaintext. If verification fails, the operation MUST return a `ConnectionSetupEncryptionFailed` error (008 FR-P32). Decryption with a non-matching NeuronPrivateKey MUST also fail with the same error.

**D. connectionSetup Processing**

- **FR-D15**: **Connection direction (who dials)**. Upon receiving a `connectionSetup` message (008 FR-P33), the receiver MUST: (1) decrypt `encryptedMultiaddrs` using their own NeuronPrivateKey per FR-D11–D14, (2) validate the decrypted multiaddr format and apply the filter rules of FR-D11a (the receiver MAY also drop entries whose route is unreachable from its own network position), (3) initiate `connect(peerID, multiaddrs)` via the DeliveryAdapter to establish the underlying libp2p connection. The "receiver" here is the **dialing** party; whichever party receives the `connectionSetup` is the dialer for that exchange. Profiles MAY (and Profile E does) reverse the canonical buyer-dials-seller direction; per 013 FR-CP-008 the direction is a profile-level decision, not an 009-level one. **Connection direction is independent of stream-init direction (FR-D-stream-direction).**
- **FR-D-stream-direction** (2026-05-08 amendment; **separate concept from FR-D15**): For each entry in the `streams[]` catalog of the agreed `connectionSetup` (008 FR-P33a), the value of `direction` determines which party MAY call `OpenStream` (or its libp2p `NewStream` equivalent) for that protocol ID:
  - `"seller-initiates"`: only the seller MAY open the stream after the underlying libp2p connection is established. The buyer MUST register an incoming-stream handler for the protocol ID.
  - `"buyer-initiates"`: only the buyer MAY open the stream. The seller MUST register an incoming-stream handler.
  - `"either"`: both parties MAY open the stream; the first OpenStream wins; the other side accepts.
  When `connectionSetup` carries only the legacy single-string `protocol` field (FR-P33a back-compat), `direction` defaults to `"seller-initiates"` to preserve the behavior of pre-2026-05-08 deployments (notably the JV-box edge-seller flow on Profile E). Implementations MUST NOT override the declared `direction` for an entry; opening a `"buyer-initiates"` stream from the seller is a `StreamDirectionViolation` error (FR-D29).
- **FR-D-multi-protocol** (2026-05-08 amendment): A peer MAY register multiple libp2p stream protocol IDs concurrently — one per entry in the agreed `streams[]` catalog — under the same underlying libp2p connection. Each entry's protocol ID MUST be addressable independently via libp2p's multistream-select. Stream protocol IDs registered for one agreement MUST NOT collide with protocol IDs registered for an unrelated agreement on the same host; collisions MAY be resolved by namespacing (e.g., per-DApp protocol ID prefix) — the resolution scheme is DApp-defined per Constitution Principle XII, and the SDK's role is to provide the `RegisterProtocol(protocolID, handler)` primitive without prescribing namespacing.
- **FR-D-wildcard-handler** (2026-05-08 amendment): A protocol-ID pattern MAY contain a single trailing `*` segment (e.g., `/jetvision/filtered/*`, `/sensor/<modelID>/*`). When such a wildcard pattern is registered, the SDK MUST:
  - Match incoming-stream requests whose protocol ID shares the literal prefix and has at least one additional path segment in place of `*`.
  - Surface the matched parameter (the substring replacing `*`) to the handler as a structured argument (e.g., a `params map[string]string` with a single `"*"` key, or a positional argument; the surface API shape is implementation-defined but the parameter MUST be available).
  - Reject (with `UnknownProtocol`) any incoming protocol ID that matches no registered literal entry and no registered wildcard pattern.
  The SDK MUST NOT validate the *value* of the wildcard parameter; semantic validation (e.g., "altitude must be a positive integer ≤ 60000") is the consumer DApp's responsibility per Constitution Principle XII. Only one wildcard `*` per pattern is permitted in v1; multi-segment wildcards (`/jetvision/filtered/**`) are reserved for a future amendment.
- **FR-D-pubsub-primitives** (2026-05-08 amendment; per Constitution Principle XII): The SDK MUST expose libp2p's pubsub primitives (`pubsub.PubSub`, `floodsub.FloodSub`, `gossipsub.GossipSub`, or their equivalents in non-Go bindings) as **substrate primitives** available to consumer code. The SDK MUST NOT prescribe:
  - Topic naming conventions for fan-out (these belong to the DApp).
  - Mesh parameters (D, Dlo, Dhi, Dout, gossip factor) — the SDK exposes the underlying libp2p configuration knobs verbatim and the DApp picks values appropriate to its traffic shape.
  - Copy or drop semantics under backpressure.
  - Distribution-relay roles (a "fan-out relay" that subscribes to a seller's pubsub topic and re-publishes to N indirect buyers is a DApp-defined composition, not an SDK behavior).
  Conformance requirement: an SDK MUST allow a DApp to (a) create or join a pubsub topic with DApp-chosen name, (b) publish to that topic, (c) subscribe to that topic, (d) configure the underlying libp2p pubsub router type and parameters. Anything beyond this — including any policy about WHEN to use pubsub vs direct streams — is out of scope for 009 and belongs in the consuming DApp spec.
- **FR-D-degraded** (2026-05-08 amendment; mirrors 008 FR-P50–FR-P52): An established delivery channel MUST NOT be closed by the SDK on the basis of control-plane (topic backend / chain) unavailability. Specifically:
  - When the topic backend (HCS or other 004 adapter) is unreachable, an active delivery channel MUST continue to send and receive frames per FR-D03/FR-D04. Heartbeat publishing on the control plane (005) MAY pause; the data plane is independent.
  - When the settlement binding (escrow contract / on-chain RPC) is unreachable, an active delivery channel MUST continue. Settlement catches up when the binding recovers per 008 FR-P52.
  - When the libp2p reachability probe (AutoNAT, FR-D19) fails, an already-CONNECTED channel MUST NOT transition to DISCONNECTED on that signal alone; AutoNAT failure is informational for *new* dials, not a kill signal for *existing* connections.
- **FR-D16**: When `connectionSetup` carries the legacy single-string `protocol` field (008 FR-P33a back-compat), that string MUST match the stream protocol ID used by the DeliveryAdapter when opening the libp2p stream. When `connectionSetup` carries the `streams[]` catalog (008 FR-P33a), each entry's `protocolID` (literal or wildcard pattern) MUST be registered per FR-D-multi-protocol / FR-D-wildcard-handler. Protocol ID format follows libp2p convention: path-like string, optionally with version (e.g., `"/neuron/adsb/1.0.0"`, or wildcard `"/jetvision/filtered/*"`).
- **FR-D17**: If the receiver's `connect` attempt fails (DISCONNECTED state), the receiver MAY send a new `connectionSetup` message with updated multiaddrs (e.g., including relay addresses). The number of retry `connectionSetup` exchanges is application-defined but SHOULD NOT exceed 3.

**E. NAT Traversal Policy**

- **FR-D18**: The DeliveryAdapter MUST support the libp2p NAT traversal stack: AutoNAT for reachability detection, Circuit Relay v2 for indirect connectivity, and DCUtR for upgrading relay connections to direct.
- **FR-D19**: AutoNAT: The agent MUST periodically probe its own reachability by requesting other peers to dial its addresses. The `natStatus` field in `connectionSetup` (008 FR-P33) MUST reflect the most recent AutoNAT result: `"public"` if reachable, `"private"` if behind NAT, `"unknown"` if not yet determined.
- **FR-D20**: Circuit Relay v2: When direct dial fails and the counterparty's `natStatus` indicates `"private"` or `"unknown"`, the agent MUST attempt relay-based connectivity. Relay nodes MAY be discovered via the libp2p DHT or configured statically. Relay connections are resource-limited by the relay node (per libp2p Circuit Relay v2 specification).
- **FR-D21**: DCUtR: When a relay connection is established, the agent MUST attempt to upgrade to a direct connection via the DCUtR protocol (synchronized hole punching). If DCUtR succeeds, the delivery channel transitions from RELAYING to CONNECTED (FR-D08). If DCUtR fails, the relay path remains active.

**F. Stream Data Framing**

- **FR-D22**: Service data on P2P delivery channels MUST be framed using length-prefixed framing: each frame consists of a 4-byte unsigned big-endian length prefix followed by the payload bytes. Maximum frame size is 4 MiB (4,194,304 bytes).
- **FR-D23**: The frame payload is opaque bytes — the delivery layer does not parse, validate, or transform the content. The data format is determined by the service's `termsRef` document and `serviceParams`.
- **FR-D24**: A zero-length frame (4 bytes of zeros followed by no payload) is reserved as a keep-alive signal. The DeliveryAdapter MUST silently consume keep-alive frames and not deliver them to the application layer.

**G. Transport Configuration**

- **FR-D25**: The libp2p binding MUST support at minimum two transports: QUIC (`/quic-v1`) for server-to-server delivery and WebRTC (`/webrtc`) for browser-capable delivery. WebTransport (`/webtransport`) is RECOMMENDED as a third transport.
- **FR-D26**: Transport selection MUST be automatic based on the multiaddr format provided in `connectionSetup`. The agent MUST NOT require manual transport configuration for standard libp2p multiaddrs.
- **FR-D27**: All transports MUST provide transport-layer encryption (TLS 1.3 for QUIC/WebTransport, DTLS for WebRTC, Noise for TCP). The DeliveryAdapter MUST NOT accept unencrypted transport connections.
- **FR-D28**: PeerID verification: During the transport handshake, the DeliveryAdapter MUST verify that the remote peer's PeerID matches the `peerID` from the `connectionSetup` message. A mismatch MUST produce a `PeerIDMismatch` error and the connection MUST be rejected.

**H. Error Handling**

- **FR-D29**: System MUST define structured error types in the `NEURON-DELIVERY-*` domain following the 006 error taxonomy format. Error kinds MUST include at minimum: `DialFailed` (all dial attempts exhausted), `StreamError` (stream I/O failure), `RelayError` (relay connection failed or relay unavailable), `PeerIDMismatch` (remote PeerID does not match connectionSetup), `NoCompatibleTransport` (no multiaddr matches configured transports), `InvalidMultiaddr` (decrypted multiaddrs are malformed), `ChannelClosed` (operation on closed channel), `FrameTooLarge` (frame exceeds 4 MiB limit), `BackoffExhausted` (max reconnection duration exceeded), `ConnectionSetupEncryptionFailed` (ECIES decryption failed — shared with 008 FR-P32), `StreamDirectionViolation` (FR-D-stream-direction: a party opened a stream whose declared `direction` excludes that party), `UnknownProtocol` (FR-D-wildcard-handler: an incoming stream's protocol ID matched no registered literal or wildcard pattern), `MultiaddrFilterRejected` (FR-D11a: an advertisement attempt included a forbidden address — loopback / Docker / virtual; emitted locally as a configuration warning before encryption rather than over the wire).

### Key Entities

- **DeliveryAdapter**: Abstract interface defining five operations (connect, send, receive, disconnect, getStatus) that all delivery bindings implement. Analogous to Spec 004's TopicAdapter for the data plane.
- **DeliveryChannel**: Opaque handle representing an active or historical delivery channel between two peers. Contains the negotiated protocol, transport identifier, and current connection state.
- **ConnectionState**: Lifecycle state for a delivery channel. One of: IDLE, CONNECTING, CONNECTED, RECONNECTING, RELAYING, DISCONNECTED.
- **DataFrame**: A length-prefixed data unit transmitted over a delivery channel. Contains `data` (opaque bytes) and metadata (`receivedAt` timestamp).
- **ECIESCiphertext**: The encrypted multiaddr payload structure: ephemeral public key (33 bytes, compressed secp256k1) ‖ nonce (12 bytes) ‖ ciphertext (variable) ‖ authentication tag (16 bytes). Encoded as base64 for JSON inclusion.

---

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-D01**: Two agents can establish a P2P delivery channel, exchange data frames, and cleanly disconnect, completing the full connect → send/receive → disconnect cycle on at least two transports (QUIC and WebRTC).
- **SC-D02**: Multiaddr encryption via the ECIES profile produces ciphertexts that only the intended NeuronPrivateKey holder can decrypt. Decryption with any other key MUST fail with authenticated-encryption error.
- **SC-D03**: The connection lifecycle state machine is deterministic — given the same sequence of transport events (dial success, dial failure, drop, reconnect), any two SDK implementations compute the same current state.
- **SC-D04**: NAT traversal successfully establishes a delivery channel between two agents where at least one is behind a NAT, using the AutoNAT → Relay → DCUtR escalation path.
- **SC-D05**: After a transport connection drop, the DeliveryAdapter reconnects within the exponential backoff parameters (initial 5s, max 10min) and resumes data delivery without application-layer intervention.
- **SC-D06**: Stream data framing is interoperable — a frame written by the Go SDK can be read by the TypeScript SDK and vice versa, using the 4-byte length-prefix format.
- **SC-D07**: PeerID verification rejects connections from peers whose PeerID does not match the `connectionSetup.peerID` — preventing man-in-the-middle impersonation.
- **SC-D08** (2026-05-08 amendment): Connection direction (who dials) and stream-init direction (who opens streams) are **independently** configurable. An integration test demonstrates four combinations on the same SDK build: (seller-dials-buyer + seller-initiates-stream, seller-dials-buyer + buyer-initiates-stream, buyer-dials-seller + seller-initiates-stream, buyer-dials-seller + buyer-initiates-stream). All four cycles succeed with no SDK code change; only the profile descriptor and `streams[].direction` differ.
- **SC-D09**: Multi-protocol stream catalog: a single `connectionSetup` carrying a `streams[]` catalog with three entries — one literal (`/raw/1.0.0`), one wildcard (`/filtered/*`), one buyer-initiated (`/control/1.0.0`, direction = `buyer-initiates`) — round-trips through canonical JSON with byte equality; the receiver registers handlers for all three; literal and pattern-matched streams open and accept correctly under multistream-select; the buyer-initiates entry rejects an attempted seller-side `OpenStream` with `StreamDirectionViolation`.
- **SC-D10**: Multiaddr filtering: with the host configured to listen on all interfaces (loopback + LAN + public + an active relay-circuit reservation), the advertised array carried in `encryptedMultiaddrs` after FR-D11a filtering excludes loopback (`127.0.0.1`, `::1`), excludes any address sourced from a `docker0` or `veth*` interface, includes the LAN address, includes the public address, and includes the relay-circuit address. Verifiable by a unit test that exercises the filter against a synthetic interface enumeration.
- **SC-D11**: Pubsub primitive exposure: a DApp test creates a `gossipsub.GossipSub` instance, joins a DApp-chosen topic, publishes a message, subscribes from a second peer, and receives the message — all without any SDK function call that prescribes mesh parameters or copy semantics. The SDK provides only the substrate; the DApp configures the behavior.
- **SC-D12**: Degraded-mode data plane: with an established delivery channel and the topic backend made unreachable for 60 seconds (simulated outage), data frames continue to flow end-to-end with no channel state transition to DISCONNECTED. Verifiable by an integration test that toggles the topic adapter into a fault state while a stream is active.

---

## Third-Party Validation _(mandatory)_

### Verification Tier

**`topic-observable`** for connectionSetup exchange. **`proof-required`** for delivery channel operation.

A third-party validator can verify that `connectionSetup` messages were exchanged (by subscribing to stdIn topics) and that the metadata fields conform to 008 FR-P33. However, the delivery channel itself is private (P2P stream, not on a public topic). To verify actual delivery, the validator relies on delivery proof evidence published by the seller (via `evidenceHash` in 008 FR-P20) and on-chain escrow state. The validator cannot observe the P2P stream directly — this is by design (private data plane).

### Validator Checklist

- **VR-DEL-01**: Observe `connectionSetup` on seller's stdIn. Verify `peerID` field is a valid libp2p PeerID format (base58btc multihash). Verify `protocol` field follows path-like format with version. Pass if fields are well-formed. Fail if malformed.
- **VR-DEL-02**: Observe both parties' `connectionSetup` messages (bidirectional exchange). Verify `requestId` matches the agreement's `requestId` from the `serviceRequest`/`serviceResponse` exchange (008 FR-P07/P08). Pass if consistent. Fail if `requestId` mismatch.
- **VR-DEL-03**: After delivery, observe `invoice` on buyer's stdIn (008 FR-P10) with `evidenceHash`. Verify `evidenceHash` matches a delivery proof TopicMessage on seller's stdOut. Pass if evidence chain is intact. Fail if `evidenceHash` does not match any published proof.
- **VR-DEL-04**: Query connection state via SDK diagnostic endpoint (if exposed). Verify reported `ConnectionState` is one of the six defined states (FR-D07). Pass if state is valid. Fail if state is undefined or unreported.
- **VR-DEL-05** (2026-05-08 amendment): Observe `connectionSetup` carrying a `streams[]` catalog (008 FR-P33a). For each entry, verify `protocolID` is a well-formed libp2p path-style identifier (literal or single-trailing-`*` wildcard) and `direction` is one of `"seller-initiates" | "buyer-initiates" | "either"`. Pass if all entries validate; fail otherwise.
- **VR-DEL-06** (2026-05-08 amendment): Filter sanity for advertised multiaddrs (FR-D11a). Where the validator has any side-channel ability to learn the seller's listen set (e.g., by being on the same LAN or by separately running its own AutoNAT probes), verify that the seller's advertised array includes globally-routable addresses but does NOT include loopback or virtual-bridge addresses. This rule is **inconclusive** by default because the encrypted-multiaddr payload is opaque to validators by design; the verdict is informative only when the validator and seller share a network position.

### Observable State Commitments

The `connectionSetup` message (008 FR-P33) is published to stdIn — this provides the primary observable state for the delivery setup phase. No additional topic messages are required by this spec.

For delivery proof observability, this spec relies on 008's `evidenceHash` mechanism (FR-P20): the seller publishes delivery proof to their stdOut and references it in the `invoice`. The proof format is application-defined (via `termsRef`), but the hash linkage is protocol-enforced.
