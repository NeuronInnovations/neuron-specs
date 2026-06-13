# Feature Specification: Payment

**Feature Branch**: `008-payment`
**Created**: 2026-03-19
**Status**: Draft

## Related Specs

**In-repo:**
- **001 NeuronAccount**: SharedAccount (FR-021), paymentAddress (FR-023/024), LedgerAttachment (FR-018/019), balance tracking (FR-016)
- **002 Key Library**: MultisigKey `"hedera-threshold"` (FR-023/024), signing (FR-014/017), EVMAddress (FR-005)
- **003 Peer Registry**: agentURI `services[]` (FR-R02/R03), service type extensibility
- **004 Topic System**: TopicMessage envelope (FR-T02/T03), payload extensibility (FR-T20), stdIn/stdOut channels (FR-T07), canonical JSON serialization (FR-T21)
- **005 Health**: LivenessState machine (FR-H18–H20) — pre-engagement liveness check
- **006 Protocol Determinism**: Canonical JSON (FR-W01–W10), error taxonomy (NEURON-{DOMAIN}-{NNN})
- **007 Identity Contract**: Identity Registry lookup (FR-C-10), Reputation Registry (FR-C-20–C-26), Validation Registry (FR-C-27–C-33)
- **009 P2P Data Delivery** (planned): DeliveryAdapter interface, libp2p binding, connection lifecycle, stream framing, multiaddr encryption algorithm — consumes `delivery` (FR-P01a) and `connectionSetup` (FR-P33) defined here

**External:**
- **EIP-20** (ERC-20): Token standard for EVM escrow deposits
- **EIP-2612** (ERC-2612): Gasless token approval via permit (EOA only)
- **Permit2** (Uniswap): Universal signature-based token transfers (EOA + smart accounts)
- **EIP-712**: Typed structured data signing — domain separation and replay protection
- **EIP-8004**: Agent identity / URI standard — `services[]` array structure

## Clarifications

### Session 2026-03-19

- Q: How should receivers handle version mismatches in negotiation payloads? → A: Same rule as Health FR-H28 — accept `1.x.y` (ignore unknown fields), reject `2.x.y` until explicitly upgraded. Added as FR-P12a.
- Q: Should negotiation messages (pricing, amounts, terms) be encrypted or public on stdIn/stdOut? → A: Public. Consistent with existing architecture (heartbeats, registration are public). Enables third-party validator observability (VR-PAY-01–05). Added as FR-P12b.
- Q: Can a buyer hold multiple active agreements with the same seller simultaneously? → A: Yes. Concurrent agreements permitted. State tracked per `requestId`, not per buyer-seller pair. Added as FR-P13a.
- Q: What happens when negotiation stalls (seller never responds to serviceRequest)? → A: Buyer specifies a `negotiationDeadline` in the `serviceRequest`. If the seller does not respond before it, the buyer's SDK auto-transitions to REJECTED. Added as FR-P07a, FR-P14 update.

### Session 2026-03-24

- Q: Should the protocol mandate specific balance thresholds (e.g., ≥ agreedAmount, 2× invoice) as universal rules? → A: No. Balance thresholds are service-level funding compliance rules, not settlement preconditions. Different dApps require different funding models (prepaid, milestone, minimum-buffer, tolerant). The protocol defines settlement preconditions (requestRelease fails if amount > available; claimRefund fails before timeout) and provides observability primitives (getBalance, termsRef), but delegates service-level funding rules to the agreed terms. Revised Section E (FR-P24–P27a).

### Session 2026-05-08

- Q: Should the protocol have an explicit StopService message? Currently the only termination paths are timeout, invoice refusal, or local-SDK invocation of EventTerminate; none are wire-level. → A: Yes. Adding three new messages: `serviceStop` (buyer→seller), `serviceCancel` (either→other), `serviceRenew` (buyer→seller). Once a service reaches ACTIVE, the seller MUST keep streaming until it receives a `serviceStop` from the buyer or the process shuts down. Resource pressure or transient network glitches are NOT grounds for seller-side stream termination. Added as Section J (Long-Lived Service Discipline) and three new payload types in Section B.
- Q: How long does an active service entry remain valid? Should the seller remember a request from N days ago across restart? → A: Active-service entries MUST be persisted to local durable storage and replayed on startup. The eligibility window MUST be configurable, with a default of 24 hours. Entries past the cutoff MAY be evicted. Added as Section I (Active-Service Persistence).
- Q: What happens when the topic backend (HCS) is down but a service is already ACTIVE? → A: Existing P2P delivery streams MUST continue to flow. The agent MUST NOT consult the chain or topic backend to validate continuation of an already-AGREED, already-FUNDED, already-ACTIVE service. Heartbeat publishing MAY pause; control-plane operations MAY be queued or dropped; data plane MUST NOT be gated on control-plane availability. Added as Section K (Degraded-Mode Operation).
- Q: How should multiple stream variants of the same service (e.g., raw, filtered/100, filtered/200, status) be advertised within a single service agreement? → A: Extend `connectionSetup` with an OPTIONAL `streams[]` catalog. Each entry declares `name`, `protocolID` (libp2p stream protocol identifier; MAY be a wildcard pattern such as `/jetvision/filtered/*`), `direction` (`seller-initiates`, `buyer-initiates`, or `either`), and OPTIONAL `schema` reference. Wildcard semantics are defined by 009 FR-D-wildcard-handler. The legacy single-string `protocol` field remains valid for back-compat with existing Profile E deployments. Added as updates to FR-P33.
- Q: Where does buyer admission policy (priority list, allowlist, denylist) live in the spec corpus? → A: It does NOT belong in core. Per Constitution Principle XII, admission policy semantics are DApp-defined. The Core SDK exposes a small declarative `AdmissionPolicy` interface stub that DApp specs (e.g., 016 ADS-B, 017 Remote ID) implement with their own concrete policies. Added as Section L (AdmissionPolicy Interface Stub).
- Q: Are buyer→seller commands (e.g., "filter planes below 10000 ft") part of this spec? → A: No; deferred. Filtering is realized as **path-based protocol IDs** (e.g., `/jetvision/filtered/100` and `/jetvision/filtered/200` are distinct streams with distinct handlers), exposed via the `streams[]` catalog above. A dedicated buyer→seller command channel is deferred. The `streams[]` catalog with `direction = buyer-initiates` is the forward-compatible hook.

## Out of Scope

- **SLA content and format**: Each dApp defines its own service terms. This spec provides a `termsRef` URI pointer, not the SLA schema.
- **Pricing algorithms**: The `pricing` field is declarative. How an agent computes its price is application logic.
- **Marketplace UI / search / discovery**: Frontend concern. This spec provides machine-readable service offerings.
- **Revenue split ratios**: The specific split (e.g., 60/40 Parent/Child) is a product configuration. The spec defines that the transfer destination is configurable with `paymentAddress` as the default.
- **Arbiter selection criteria**: Domain-specific. The spec defines the arbiter role and threshold semantics; who fills the role is application logic.
- **Dispute resolution beyond refund timeout**: Buyer can refuse to counter-sign; arbiter can intervene in 2-of-3 threshold. Further arbitration is application-layer.
- **x402 / streaming / ERC-8183 payment bindings**: Different payment models. Deferred to future specs. The `settlement.binding` field provides the extension point.
- **Cross-chain escrow (bridging)**: Requires bridge infrastructure not yet in place.
- **On-chain agentURI parsing**: Per 007 DD-01, agentURI is opaque on-chain. Content validation is SDK-level.
- **Service Description Framework**: Full dApp-level service schemas (ADSB, radiation, etc.) are future specs. This spec defines the pointer (`termsRef`), not the content.
- **Funding compliance formulas**: Specific balance thresholds (minimum deposit, top-up triggers, milestone schedules, pre-payment percentages) are agreement-level policy. This spec provides observability primitives (`getBalance`, negotiation payloads) and the `termsRef` hook; it does not prescribe when a seller should start or stop delivery based on balance.
- **P2P Data Delivery**: DeliveryAdapter interface, connection lifecycle state machine, stream data framing, delivery error taxonomy, NAT traversal / relay policy, and the multiaddr encryption algorithm profile (key agreement, KDF, AEAD cipher for FR-P34). Deferred to Spec 009. This spec defines the discovery hooks (`delivery` in `neuron-commerce`), the address exchange message (`connectionSetup`), and the encryption requirement (FR-P34). The mechanics of establishing and maintaining delivery channels are Spec 009's domain.

## Assumptions

- Agents have completed identity registration (003) and are discoverable via the Identity Registry (007) before offering or purchasing services.
- The topic system (004) is operational and agents have stdIn/stdOut channels available for negotiation messages.
- At least one settlement binding (hedera-native or evm-escrow) is available to both buyer and seller.
- Agents resolve `paymentAddress` per 001 FR-023/024: Child's payment address = Parent's EVMAddress.
- Trust gating (liveness, reputation, validation checks) is SHOULD-level — agents that skip these checks accept the risk.
- A delivery binding specification (Spec 009) defines how `connectionSetup.encryptedMultiaddrs` is decrypted and how delivery channels are established based on `delivery.mode`. Spec 009 must be completed before implementations can satisfy FR-P34 and FR-P35.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Discover and Purchase a Service (Priority: P1)

A buyer agent discovers a seller's commercial offering via the seller's agentURI, negotiates terms, funds an escrow, receives service delivery, and settles payment through the invoice/counter-sign cycle.

**Why this priority**: This is the core end-to-end payment flow — the escrow settlement mechanism formalized. Without this, no commerce happens.

**Independent Test**: Can be fully tested by running a buyer and seller agent on the same settlement binding (e.g., hedera-native). Buyer discovers the seller's `neuron-commerce` service, sends a `serviceRequest`, receives `serviceResponse` (accept), creates escrow, seller delivers, seller sends `invoice`, buyer sends `invoiceAck` (approved). Funds transfer to seller's paymentAddress.

**Acceptance Scenarios**:

1. **Given** a seller has a `neuron-commerce` entry in their agentURI with delivery mode, pricing, and settlement binding, **When** a buyer queries the seller's agentURI via the Identity Registry, **Then** the buyer can discover the service name, delivery mode, pricing, settlement binding, and termsRef.
2. **Given** buyer and seller agree on terms via `serviceRequest`/`serviceResponse` exchange, **When** the buyer creates an escrow and deposits funds, **Then** the seller can query the escrow balance and evaluate whether funding meets the requirements established in the agreed terms before starting work.
3. **Given** an active service delivery, **When** the seller sends an `invoice` with `evidenceHash` linking to a delivery proof TopicMessage, **Then** the buyer can verify the evidence, counter-sign the release, and funds transfer to the seller's paymentAddress.
4. **Given** a completed invoice cycle, **When** the buyer deposits more funds and the seller issues a new invoice, **Then** the ACTIVE ⇄ INVOICED streaming cycle continues until either party terminates.

---

### User Story 2 — Publish a Commercial Service Offering (Priority: P1)

A seller agent registers a `neuron-commerce` service entry in its agentURI, declaring what it offers, at what price, on which settlement binding, and under what terms.

**Why this priority**: Without discoverable service offerings, buyers cannot find sellers. This is the supply side of the marketplace.

**Independent Test**: Can be tested by creating a `neuron-commerce` entry, including it in the agentURI, and verifying it round-trips through JSON serialization with canonical field ordering.

**Acceptance Scenarios**:

1. **Given** a seller agent with a NeuronAccount (Child), **When** the seller adds a `neuron-commerce` service to its agentURI, **Then** the entry includes all MUST fields: `type`, `name`, `version`, `delivery` (with `mode`), `settlement` (with `binding`), and `pricing` (with `amount`, `currency`, `unit`, `interval`).
2. **Given** a seller offering the same service on multiple chains, **When** the seller adds multiple `neuron-commerce` entries with different `settlement` objects, **Then** buyers can discover and select by binding.
3. **Given** a `neuron-commerce` entry, **When** serialized to JSON and deserialized, **Then** the result is byte-identical (canonical field ordering per 006 FR-W05).

---

### User Story 3 — Negotiate Terms Before Commitment (Priority: P2)

A buyer proposes terms to a seller. The seller may accept, reject, or counter-offer. Negotiation happens via TopicMessage payloads on stdIn/stdOut until both parties agree or one rejects.

**Why this priority**: Negotiation enables dynamic pricing and term adjustment. Without it, agents can only accept or reject fixed offers.

**Independent Test**: Can be tested by exchanging `serviceRequest` and `serviceResponse` messages between two agents. Verify state transitions: IDLE → REQUESTED → NEGOTIATING → AGREED (or REJECTED).

**Acceptance Scenarios**:

1. **Given** a buyer sends a `serviceRequest` to the seller's stdIn, **When** the seller responds with `serviceResponse` (action: `counter`), **Then** the agreement lifecycle transitions to NEGOTIATING and the buyer receives the counter-offer.
2. **Given** a negotiation in progress, **When** the buyer sends a revised `serviceRequest` and the seller responds with `serviceResponse` (action: `accept`), **Then** the lifecycle transitions to AGREED.
3. **Given** a `serviceRequest`, **When** the seller responds with `serviceResponse` (action: `reject`), **Then** the lifecycle transitions to REJECTED and no escrow is created.

---

### User Story 4 — Refund on Timeout (Priority: P2)

A buyer funds an escrow but the seller fails to deliver or invoice within the agreed timeout. After the timeout elapses, the buyer claims a refund.

**Why this priority**: Without a refund mechanism, buyers risk permanent fund loss if sellers disappear.

**Independent Test**: Can be tested by creating an escrow with a short timeout, waiting for expiry, and calling `claimRefund`. Verify funds return to buyer.

**Acceptance Scenarios**:

1. **Given** an escrow with `timeout` set to a future timestamp, **When** the timeout elapses without the seller requesting a release, **Then** the buyer can call `claimRefund` and recover the deposited funds.
2. **Given** an escrow where the seller has already requested a release (INVOICED state), **When** the timeout elapses without the buyer approving, **Then** the buyer can call `claimRefund` and the pending release is cancelled.
3. **Given** an escrow that has not yet timed out, **When** the buyer attempts `claimRefund`, **Then** the operation fails with an appropriate error.

---

### User Story 5 — Multi-Binding Settlement Discovery (Priority: P3)

A buyer discovers that a seller supports multiple settlement bindings (e.g., both `hedera-native` and `evm-escrow`). During negotiation, the buyer selects the preferred binding.

**Why this priority**: Multi-binding support enables cross-chain commerce and cost optimization. Deferred priority because single-binding flows work without this.

**Independent Test**: Can be tested by creating a seller with two `neuron-commerce` entries (same service, different bindings) and verifying the buyer's `serviceRequest` includes `settlementBinding` selection.

**Acceptance Scenarios**:

1. **Given** a seller with `neuron-commerce` entries for both `hedera-native` and `evm-escrow`, **When** a buyer queries the agentURI, **Then** both entries are visible with distinct `settlement` objects.
2. **Given** the buyer sends a `serviceRequest` with `settlementBinding: "evm-escrow"`, **When** the seller accepts, **Then** the escrow is created on the specified EVM chain using the `NeuronEscrow.sol` contract.

---

### User Story 6 — Trust-Gated Engagement (Priority: P3)

Before funding an escrow, a buyer checks the seller's liveness, identity registration, reputation score, and validation status. The buyer only proceeds if all checks pass.

**Why this priority**: Trust gating improves safety but is SHOULD-level — the protocol works without it.

**Independent Test**: Can be tested by querying the seller's stdOut for a heartbeat (liveness), calling `lookup()` on the Identity Registry, `getSummary()` on Reputation and Validation Registries, and verifying thresholds.

**Acceptance Scenarios**:

1. **Given** a seller with `LivenessState == ALIVE` and positive reputation, **When** the buyer runs pre-engagement checks, **Then** all checks pass and the buyer proceeds to negotiation.
2. **Given** a seller with `LivenessState == DEAD`, **When** the buyer runs the liveness check, **Then** the SDK warns the buyer and the buyer can choose to abort.

---

### Edge Cases

- What happens when the seller's `neuron-commerce` entry specifies a settlement binding the buyer does not support? The buyer's SDK MUST detect the mismatch during discovery and skip incompatible offerings.
- What happens when the buyer deposits less than the agreed amount? The seller evaluates whether the deposit meets the funding requirements established in the agreed terms (referenced via `termsRef` and negotiation parameters). If the seller determines funding is insufficient per those terms, the seller does not begin delivery. The protocol does not mandate a specific threshold — this is agreement-defined.
- What happens when the seller sends an `invoice` for a different amount than agreed? The buyer's SDK SHOULD verify invoice amount matches the agreed terms before counter-signing. Mismatched invoices SHOULD be refused.
- What happens when both buyer and seller call escrow operations simultaneously (race condition)? On EVM: transaction ordering determines winner; second call reverts. On Hedera: schedule operations are serialized by consensus.
- What happens when the escrow's token is devalued between deposit and release? The protocol operates on nominal amounts. Currency risk is borne by the parties, not the protocol.
- What happens when a seller's paymentAddress (Parent's EVMAddress) is unreachable or invalid? The release operation fails. The seller must ensure their paymentAddress is valid before requesting release.
- What happens when `delivery.mode` is `"p2p"` but no `connectionSetup` is exchanged? The seller cannot establish a delivery channel. The agreement MAY remain in FUNDED indefinitely or be TERMINATED by either party. The protocol does not auto-terminate — explicit action is required (FR-P14). The buyer's SDK SHOULD produce a `ConnectionSetupRequired` warning after a configurable timeout.
- What happens when `connectionSetup.encryptedMultiaddrs` cannot be decrypted? The receiver produces a `ConnectionSetupEncryptionFailed` error. The sender MAY retry with a corrected message. The agreement state is unaffected — `connectionSetup` does not trigger lifecycle transitions (FR-P14a).
- What happens when the seller's `neuron-commerce` entry references a `delivery.serviceRef` that does not exist in the agentURI? The buyer's SDK MUST detect the broken cross-reference during discovery and produce an `InvalidDeliveryRef` error. The offering is treated as invalid and MUST be skipped.

---

## Requirements *(mandatory)*

### Functional Requirements

**A. Service Offering Schema**

- **FR-P01**: System MUST define a `neuron-commerce` service type for inclusion in the agentURI `services[]` array. The service type MUST include the following MUST fields: `type` (always `"neuron-commerce"`), `name` (human-readable service name), `version` (semver), `delivery` (delivery descriptor object), `settlement` (binding descriptor object), and `pricing` (declarative pricing object). Canonical field order for `neuron-commerce` JSON: `type` → `name` → `version` → `delivery` → `settlement` → `pricing` → `termsRef*` (*optional, omit if absent per 006 FR-W04).
- **FR-P01a**: The `delivery` object MUST include a `mode` field identifying the delivery mechanism. Defined values: `"p2p"` (direct peer-to-peer stream), `"topic"` (topic channel subscription), or `"custom:<type>"` (application-defined delivery, where `<type>` follows the `custom:` namespace convention). Mode-specific fields:
  - When `mode` is `"p2p"`: `serviceRef` (string, MUST) — cross-references a `neuron-p2p-exchange` service `name` in the same agentURI.
  - When `mode` is `"topic"`: `channelRef` (string, MUST) — cross-references a `neuron-topic` service `name` in the same agentURI.
  - When `mode` is `"custom:<type>"`: no additional MUST fields; application-defined.
  Canonical field order for `delivery` JSON: `mode` → `serviceRef*` → `channelRef*` (*conditional on mode; omit when not applicable).
- **FR-P01b**: Cross-reference validation for `delivery`: when `mode` is `"p2p"`, `delivery.serviceRef` MUST match the `name` of an existing `neuron-p2p-exchange` service in the same agentURI. When `mode` is `"topic"`, `delivery.channelRef` MUST match the `name` of an existing `neuron-topic` service in the same agentURI. An invalid reference produces an `InvalidDeliveryRef` error (FR-P32). This follows the same cross-reference validation pattern as 004 FR-T18 (`topicRef`). How the delivery channel is established and maintained is defined by the delivery binding specification (Spec 009).
- **FR-P02**: The `settlement` object MUST include a `binding` field identifying the settlement binding (`"hedera-native"`, `"evm-escrow"`, or future values). Binding-specific fields (e.g., `chainId`, `contract`, `token`, `network`) are defined per binding.
- **FR-P03**: The `pricing` object MUST include `amount` (string decimal), `currency` (symbol), `unit` (denomination), and `interval` (billing interval in seconds; `"0"` = one-time payment).
- **FR-P04**: The `termsRef` field (SHOULD) provides a URI pointing to the full service terms document. The spec defines the pointer, not the content format. When applicable, the referenced terms document SHOULD include the service-level funding requirements (e.g., minimum deposit thresholds, top-up policies, milestone schedules) that the parties use to evaluate funding compliance (see Section E).
- **FR-P05**: An agentURI MAY contain 0–N `neuron-commerce` entries. An agent offering the same service on multiple settlement bindings lists separate entries with different `settlement` objects. Different `neuron-commerce` entries for the same service MAY declare different `delivery` objects (e.g., one offering P2P delivery and another offering topic-based delivery).

**B. Negotiation Sub-Protocol**

- **FR-P06**: System MUST define nine TopicMessage payload types for commerce coordination, carried in `TopicMessage.payload` (004 FR-T20) and discriminated by the `type` field: `serviceRequest`, `serviceResponse`, `connectionSetup`, `escrowCreated`, `invoice`, `invoiceAck`, `serviceStop`, `serviceCancel`, `serviceRenew`. The first six form the **negotiation–delivery–settlement** core; the latter three (added 2026-05-08) form the **explicit lifecycle control** set.
- **FR-P07**: The `serviceRequest` payload (buyer → seller's stdIn) MUST include: `type`, `version`, `requestId` (UUID), `serviceRef` (references `neuron-commerce` name), `settlementBinding`, `proposedAmount`, `proposedCurrency`, `proposedInterval`, `buyerStdIn` (topic ID), `negotiationDeadline` (Unix timestamp — deadline by which the seller must respond). The `arbiter` field is OPTIONAL. The `serviceParams` field is OPTIONAL; when present, it contains service-specific configuration parameters as an opaque JSON object — the protocol performs no validation on its contents. The schema of `serviceParams` is defined by the service's `termsRef` document. When `serviceParams` is present, its internal keys MUST be sorted in lexicographic (alphabetical, byte-order) order, recursively for nested objects, to ensure canonical JSON compliance per 006 FR-W05.
- **FR-P07a**: If the seller does not send a `serviceResponse` before `negotiationDeadline`, the buyer's SDK MUST auto-transition the agreement to REJECTED. The `negotiationDeadline` also applies during NEGOTIATING state — if no acceptance or counter-offer is received before the deadline, the negotiation is abandoned.
- **FR-P08**: The `serviceResponse` payload (seller → buyer's stdIn) MUST include: `type`, `version`, `requestId`, `action` (`accept`, `reject`, or `counter`). When `action` is `counter`, `counterAmount` and `counterInterval` MUST be present.
- **FR-P09**: The `escrowCreated` payload (buyer → seller's stdIn) MUST include: `type`, `version`, `requestId`, `escrowRef` (opaque binding-specific string), `depositAmount`, `depositCurrency`.
- **FR-P10**: The `invoice` payload (seller → buyer's stdIn) MUST include: `type`, `version`, `requestId`, `releaseRequestRef` (opaque binding-specific string), `escrowRef`, `amount`, `currency`, `period` (ISO 8601 interval).
- **FR-P11**: The `invoiceAck` payload (buyer → seller's stdIn) MUST include: `type`, `version`, `requestId`, `releaseRequestRef`, `action` (`approved` or `refused`). Optional fields: `depositedMore` (boolean), `newBalance` (string decimal).
- **FR-P12**: All negotiation payloads MUST use canonical JSON field ordering per 006 FR-W05. Numeric fields MUST use JSON string decimal per 006 FR-W02. Optional fields, when absent, MUST be omitted (not null) per 006 FR-W04. Canonical field orders:
  - `serviceRequest`: `type` → `version` → `requestId` → `serviceRef` → `settlementBinding` → `proposedAmount` → `proposedCurrency` → `proposedInterval` → `serviceParams*` → `negotiationDeadline` → `arbiter*` → `buyerStdIn` (*optional, omit if absent)
  - `serviceResponse`: `type` → `version` → `requestId` → `action` → `counterAmount*` → `counterInterval*` (*present only when action is `counter`)
  - `escrowCreated`: `type` → `version` → `requestId` → `escrowRef` → `depositAmount` → `depositCurrency`
  - `invoice`: `type` → `version` → `requestId` → `releaseRequestRef` → `escrowRef` → `amount` → `currency` → `period`
  - `invoiceAck`: `type` → `version` → `requestId` → `releaseRequestRef` → `action` → `depositedMore*` → `newBalance*` (*optional, omit if absent)
  - `connectionSetup`: `type` → `version` → `requestId` → `peerID` → `encryptedMultiaddrs` → `protocol*` → `streams*` → `natStatus*` (*optional, omit if absent; either `protocol` or `streams` MUST be present, both MAY be present per FR-P33a)
  - `serviceStop`: `type` → `version` → `requestId` → `reason*` → `effectiveAt*` (*optional, omit if absent)
  - `serviceCancel`: `type` → `version` → `requestId` → `reason*` → `refundRequested*` (*optional, omit if absent)
  - `serviceRenew`: `type` → `version` → `requestId` → `extendUntil` → `reason*` (*optional, omit if absent)
- **FR-P12a**: Version compatibility MUST follow the same rule as 005 FR-H28: payloads with `version` major `1` (e.g., `"1.0.0"`, `"1.1.0"`, `"1.99.0"`) MUST be accepted; unknown fields in minor/patch versions MUST be ignored; no field removal in minor/patch versions. Payloads with `version` major `2` or higher MUST be rejected until the receiving agent explicitly upgrades.
- **FR-P12b**: Negotiation payloads (serviceRequest, serviceResponse, escrowCreated, invoice, invoiceAck) are NOT encrypted. stdIn and stdOut are public channels (004 FR-T07). All pricing, amounts, and terms are publicly observable. This is by design — third-party validators (VR-PAY-01 through VR-PAY-05) depend on reading these messages for compliance verification. Exception: the `connectionSetup` payload contains an encrypted field (`encryptedMultiaddrs`) — see FR-P34 for rationale.
- **FR-P12c**: Commerce payloads that produce verifiable state commitments (`escrowCreated`, `invoice`, `invoiceAck`) SHOULD be published with `WaitForConsensus` confirmation mode (004 FR-T23) to ensure `agreementHash` and `evidenceHash` are provably anchored before downstream operations reference them. `serviceRequest` and `serviceResponse` MAY use `FireAndForget` as they do not create on-chain commitments. `connectionSetup` MAY use `FireAndForget` as it does not produce verifiable hashes.
- **FR-P33**: The `connectionSetup` payload (buyer → seller's stdIn, or seller → buyer's stdIn) MUST include: `type` (always `"connectionSetup"`), `version`, `requestId` (same UUID as the agreement), `peerID` (sender's libp2p PeerID per 002 FR-006), `encryptedMultiaddrs` (encrypted multiaddress data — see FR-P34). At least one of `protocol` or `streams` MUST be present (see FR-P33a). OPTIONAL: `natStatus` (`"public"`, `"private"`, or `"unknown"`).
  - `protocol` (CONDITIONAL): single stream protocol ID string, e.g. `"/neuron/adsb/1.0.0"`. Back-compat form for single-stream services. MUST be present if `streams` is absent.
  - `streams` (CONDITIONAL): array of stream catalog entries. MUST be present if `protocol` is absent. MAY accompany `protocol` (in which case `protocol` is interpreted as the default stream and `streams` enumerates the full catalog including any wildcard variants).
- **FR-P33a** (2026-05-08 amendment): The `streams[]` catalog declares one or more concurrent libp2p stream protocol IDs the parties intend to open within this agreement. Each entry MUST contain:
  - `name` (string, MUST): a short logical name (e.g., `"raw"`, `"filtered"`, `"status"`).
  - `protocolID` (string, MUST): the libp2p stream protocol identifier. MAY be a literal protocol ID (e.g., `"/jetvision/raw/1.0.0"`) or a wildcard pattern (e.g., `"/jetvision/filtered/*"`). Wildcard registration and parameter parsing semantics are normatively defined by 009 FR-D-wildcard-handler. The protocol layer of 008 treats the string as opaque.
  - `direction` (string, MUST): one of `"seller-initiates"`, `"buyer-initiates"`, or `"either"`. Identifies which party MAY open the stream after the underlying libp2p connection is established. Connection direction (who dials) is unrelated and is governed by 009 FR-D15 / FR-D-stream-direction.
  - `schema` (string, OPTIONAL): a URI pointing to the per-stream payload schema document. Same role as `termsRef` (FR-P04) but scoped to this stream entry.
  Within a single `streams[]` array, `name` values MUST be unique. Multi-stream catalogs allow a service to expose, for example, raw ADS-B frames on `/jetvision/raw/1.0.0`, an altitude-filtered variant family on `/jetvision/filtered/*`, and a buyer-initiated status query on `/jetvision/status/1.0.0`, all under one agreement. Per Constitution Principle XII, the *contents* of any specific stream catalog (frame format, control commands, filter parameters) belong to the DApp spec consuming this primitive — Spec 008 defines only the catalog envelope.
- **FR-P34**: The `encryptedMultiaddrs` field MUST be encrypted such that only the holder of the counterparty's NeuronPrivateKey can decrypt it. The encryption scheme MUST provide authenticated encryption (confidentiality + integrity). The specific algorithm (key agreement, KDF, AEAD cipher) is normatively defined by the delivery binding specification (Spec 009). The field MUST be encoded as a JSON string using RFC 4648 Section 4 base64 per 006 FR-W03. Rationale: `connectionSetup` is published to stdIn, which is a public topic (004 FR-T07). Without encryption, the sender's network addresses are visible to any topic subscriber. This is the sole exception to FR-P12b's "negotiation payloads are not encrypted" rule — `connectionSetup` contains network-sensitive data (peer addresses), not pricing data.
- **FR-P35**: `connectionSetup` is REQUIRED when the agreed `neuron-commerce` entry's `delivery.mode` is `"p2p"`. It is NOT required when `delivery.mode` is `"topic"` (buyer subscribes directly to the declared channel). Both parties MAY send `connectionSetup` (bidirectional exchange). The message MUST be sent after the agreement reaches AGREED state. It MAY be sent before, concurrent with, or after `escrowCreated`. How the receiver processes `connectionSetup` (dialing, NAT traversal, stream creation) is defined by the delivery binding specification (Spec 009).
- **FR-P36** (2026-05-08 amendment): The `serviceStop` payload (buyer → seller's stdIn) MUST include: `type` (always `"serviceStop"`), `version`, `requestId` (same UUID as the agreement). OPTIONAL: `reason` (string, free-form rationale; informational), `effectiveAt` (Unix timestamp in nanoseconds; if absent, the stop is effective immediately upon receipt). `serviceStop` is the **sole authoritative wire-level signal** for the seller to discontinue an ACTIVE service. Receipt MUST trigger the ACTIVE → TERMINATED transition (FR-P14) at the seller. The seller MAY issue a final `invoice` covering work delivered up to `effectiveAt` (or receipt time if `effectiveAt` is absent); the buyer settles per the existing INVOICED → ACTIVE → COMPLETED path. `serviceStop` MUST NOT be sent before AGREED — earlier termination uses `serviceCancel` (FR-P37).
- **FR-P37** (2026-05-08 amendment): The `serviceCancel` payload (buyer → seller's stdIn, or seller → buyer's stdIn) MUST include: `type` (always `"serviceCancel"`), `version`, `requestId`. OPTIONAL: `reason` (string), `refundRequested` (boolean; defaults to `true` when sender is the buyer and the agreement is FUNDED or beyond). `serviceCancel` is the explicit pre-COMPLETED termination signal usable from any state in `{REQUESTED, NEGOTIATING, AGREED, FUNDED, ACTIVE, INVOICED}`. Receipt MUST trigger transition to TERMINATED (FR-P14). When `refundRequested` is `true` and the escrow is still funded, the standard `claimRefund` flow (FR-P22) applies after `timeout`. Cancellation does NOT bypass settlement preconditions (FR-P25); a cancel before the escrow timeout has elapsed leaves funds locked until timeout, exactly as today.
- **FR-P38** (2026-05-08 amendment): The `serviceRenew` payload (buyer → seller's stdIn) MUST include: `type` (always `"serviceRenew"`), `version`, `requestId`, `extendUntil` (Unix timestamp in nanoseconds; the new expiry the buyer commits to). OPTIONAL: `reason` (string). `serviceRenew` does NOT trigger an agreement state transition. It updates the buyer's commitment to keep the service active and refreshes the **active-service entry's** `expiresAt` (Section I) at both parties. `extendUntil` MUST be greater than the current `expiresAt`; if not, the message MUST be ignored and a `RenewExpiryNotMonotonic` error (FR-P32) MAY be surfaced to local observers. `serviceRenew` MAY be sent at any time during ACTIVE or INVOICED. It does not by itself fund additional escrow; if the renewal extends beyond the current escrow's `timeout`, the buyer SHOULD top up the escrow before or alongside the renew, exactly as the existing ACTIVE ⇄ INVOICED top-up pattern (FR-P15).

**C. Agreement Lifecycle**

- **FR-P13**: System MUST implement an agreement lifecycle state machine with these states: IDLE, REQUESTED, NEGOTIATING, AGREED, FUNDED, ACTIVE, INVOICED, COMPLETED, TERMINATED, REJECTED.
- **FR-P14**: State transitions MUST follow these rules:
  - IDLE → REQUESTED: buyer sends `serviceRequest`
  - REQUESTED → NEGOTIATING: seller sends `serviceResponse` (action: counter)
  - REQUESTED → AGREED: seller sends `serviceResponse` (action: accept)
  - REQUESTED → REJECTED: seller sends `serviceResponse` (action: reject) or `negotiationDeadline` elapses
  - NEGOTIATING → AGREED: either party sends acceptance
  - NEGOTIATING → REJECTED: either party withdraws or `negotiationDeadline` elapses
  - AGREED → FUNDED: buyer creates escrow and deposits via `escrowCreated`
  - FUNDED → ACTIVE: seller evaluates funding compliance per agreed terms and begins delivery
  - ACTIVE → INVOICED: seller sends `invoice`
  - INVOICED → ACTIVE: buyer sends `invoiceAck` (action: approved) and optionally deposits more
  - INVOICED → TERMINATED: buyer sends `invoiceAck` (action: refused) or timeout
  - ACTIVE → TERMINATED: buyer sends `serviceStop` (FR-P36) or either party sends `serviceCancel` (FR-P37). Pre-2026-05-08 implementations MAY also reach TERMINATED via local SDK invocation; the new wire-level signals are now the preferred path.
  - REQUESTED|NEGOTIATING|AGREED|FUNDED|INVOICED → TERMINATED: either party sends `serviceCancel` (FR-P37). REQUESTED|NEGOTIATING via cancel is equivalent to REJECTED for observers; AGREED|FUNDED|INVOICED via cancel is the new explicit pre-COMPLETED termination path.
  - ACTIVE → COMPLETED: mutual graceful shutdown
  - ACTIVE → ACTIVE (no transition): buyer sends `serviceRenew` (FR-P38). The agreement remains ACTIVE; only the persisted `expiresAt` (Section I) is updated.
- **FR-P13a**: Agreement state MUST be tracked per `requestId`, not per buyer-seller pair. The same buyer and seller MAY have multiple concurrent agreements (different `requestId` values) in independent lifecycle states. Each agreement has its own escrow, negotiation history, and state machine instance.
- **FR-P14a**: The `connectionSetup` exchange (FR-P33–P35) occurs within the AGREED state and does not trigger a state transition. Connection establishment mechanics (dialing, NAT traversal, stream creation) are delivery-layer behavior defined by the delivery binding specification (Spec 009) and do not affect agreement lifecycle transitions.
- **FR-P15**: The ACTIVE ⇄ INVOICED cycle MUST support continuous streaming: after each approved invoice, the buyer MAY deposit additional funds and the seller MAY issue another invoice, repeating until COMPLETED or TERMINATED.

**D. Escrow Operations Interface**

- **FR-P16**: System MUST define an abstract `EscrowAdapter` interface with six operations that ALL settlement bindings MUST implement: `createEscrow`, `deposit`, `getBalance`, `requestRelease`, `approveRelease`, `claimRefund`.
- **FR-P17**: `createEscrow(buyer, seller, arbiter?, currency, threshold, agreementHash, timeout)` MUST return an `EscrowRef` (opaque reference with `binding` and `locator` fields). The `agreementHash` (bytes32) links the escrow to off-chain negotiation terms. The `timeout` (uint64) sets the refund eligibility timestamp.
- **FR-P18**: `deposit(escrowRef, amount)` MUST accept funds into the escrow and return a `DepositResult` with `transactionRef` and `newBalance`. How deposit authorization works (Permit2, permit, approve+transfer, CryptoTransfer) is binding-specific.
- **FR-P19**: `getBalance(escrowRef)` MUST return a `Balance` with `available` (string decimal), `currency`, and `lastSynced` (timestamp).
- **FR-P20**: `requestRelease(escrowRef, amount, recipient, evidenceHash)` MUST record a pending release request and return a `ReleaseRequestRef`. The `evidenceHash` (bytes32) links the release to a delivery proof TopicMessage.
- **FR-P21**: `approveRelease(escrowRef, releaseRequestRef)` MUST authorize fund release and return a `ReleaseResult`. How funds physically transfer (push on Hedera, pull via `withdraw()` on EVM) is binding-internal. The abstract interface does NOT include a `withdraw` operation.
- **FR-P22**: `claimRefund(escrowRef)` MUST return funds to the buyer after the `timeout` has elapsed. The operation MUST fail if the timeout has not yet passed.
- **FR-P23**: The `EscrowRef` and `ReleaseRequestRef` locator formats are binding-specific. The protocol layer treats them as opaque strings.

**E. Settlement Preconditions and Funding Compliance**

- **FR-P24**: The protocol distinguishes two categories of balance-related rules:
  (a) **Settlement preconditions** — escrow-mechanics invariants enforced by the `EscrowAdapter`, independent of any agreement's service terms. These are MUST-level and always apply.
  (b) **Service-level funding compliance** — rules governing when a seller should start, continue, or stop delivery based on escrow balance. These are determined by the agreed terms between buyer and seller, not mandated by this specification.

- **FR-P25**: Settlement preconditions (MUST):
  (a) `requestRelease(escrowRef, amount, ...)` MUST fail if `amount` exceeds `getBalance(escrowRef).available`. This is an escrow arithmetic invariant — a binding MUST NOT release more funds than are held.
  (b) `claimRefund(escrowRef)` MUST fail if the `timeout` has not elapsed (per FR-P22).

- **FR-P26**: Service-level funding rules — including but not limited to minimum deposit thresholds, top-up triggers, milestone schedules, pre-payment percentages, and zero-balance delivery policies — are NOT defined by this specification. They are determined by the agreed terms between buyer and seller, referenced via `termsRef` (FR-P04) and negotiation parameters (`proposedAmount`, `proposedCurrency`, `proposedInterval`).

- **FR-P27**: Non-compliance with service-level funding requirements:
  (a) Is **observable** — any party or third-party validator can detect it by querying `getBalance(escrowRef)` and comparing against the criteria in the agreed terms.
  (b) Constitutes **evidence for dispute** — a validator or arbiter can evaluate whether the agreed funding requirements were met.
  (c) Is **grounds for refusal or continuation** — the affected party decides how to respond: refuse delivery, continue under tolerated deviation, request corrective action (e.g., top-up deposit), or terminate the agreement. The protocol does not prescribe the response.
  (d) Does NOT cause an **automatic protocol failure** — the agreement lifecycle state machine does not transition based on balance thresholds. All transitions require explicit party actions (FR-P14).

- **FR-P27a**: For the FUNDED → ACTIVE transition, the seller decides whether to begin delivery based on the funding requirements established in the agreed terms. The protocol does not mandate a specific balance threshold for this transition. The seller MAY query `getBalance(escrowRef)` and evaluate the result against the agreed terms before proceeding.

**F. Payment Address Resolution**

- **FR-P28**: The `recipient` in `requestRelease` SHOULD default to the seller's `paymentAddress`, which resolves to the Parent's EVMAddress per 001 FR-023/024.
- **FR-P29**: Revenue splitting (e.g., multiple recipients per release) is product-configurable. The spec defines single-recipient release as the default. Multi-recipient patterns (splitter contracts, multiple release calls) are permitted but not mandated.

**G. Trust Gating**

- **FR-P30**: Before funding an escrow, the buyer SHOULD verify the seller's liveness (`LivenessState == ALIVE` per 005 FR-H19), identity registration (007 FR-C-10), reputation (007 FR-C-24), and validation status (007 FR-C-31). These checks are SDK-level recommendations, not MUST requirements.
- **FR-P31**: After settlement (COMPLETED), the buyer SHOULD submit feedback to the Reputation Registry via `giveFeedback()` (007 FR-C-20), called directly by the buyer to preserve correct `msg.sender` attribution.

**H. Error Handling**

- **FR-P32**: System MUST define structured error types in the `NEURON-PAYMENT-*` domain following the 006 error taxonomy format (`NEURON-{DOMAIN}-{NNN}`). Error kinds MUST include at minimum: `InvalidServiceOffering`, `NegotiationFailed`, `NegotiationExpired` (FR-P07a: negotiationDeadline elapsed without response), `VersionMismatch` (FR-P12a: received payload with unsupported major version), `EscrowCreationFailed`, `InsufficientBalance`, `InvoiceValidationFailed`, `ReleaseNotAuthorized`, `RefundNotEligible`, `BindingUnavailable`, `TimeoutNotElapsed`, `InvalidEscrowRef`, `UnsupportedDeliveryMode` (FR-P01a: `delivery.mode` value not recognized by this SDK version), `InvalidDeliveryRef` (FR-P01b: `delivery.serviceRef` or `delivery.channelRef` does not match any service in the agentURI), `ConnectionSetupRequired` (FR-P35: agreement has `delivery.mode: "p2p"` but no `connectionSetup` was exchanged before delivery attempt), `ConnectionSetupEncryptionFailed` (FR-P34: encryption or decryption of `encryptedMultiaddrs` failed), `RenewExpiryNotMonotonic` (FR-P38: `extendUntil` not strictly greater than current `expiresAt`), `ActiveServicePersistenceFailed` (FR-P40: durable write of an active-service entry failed), `ActiveServiceReplayFailed` (FR-P41: startup replay of persisted entries failed), `LongLivedDisciplineViolated` (FR-P45/P46: seller closed an ACTIVE stream without buyer authority), `DegradedModeRefused` (FR-P50: an implementation declined the data-plane / control-plane decoupling required by Section K).

**I. Active-Service Persistence** *(2026-05-08 amendment)*

- **FR-P40**: Each agent MUST persist every active-service entry to durable local storage. An "active-service entry" is the minimum state required to resume serving (seller side) or consuming (buyer side) the service across process restart. Each entry MUST include at least: `requestId`, `counterpartEVM`, `role` (`buyer`|`seller`), the agreed `neuron-commerce` reference (service name and version), `escrowRef` (when the agreement is FUNDED or beyond), the most recent `connectionSetup` payload received from the counterparty (including `streams[]` if present), the most recent locally-known `lastInvoiceSeq`, the agreement's current lifecycle state per FR-P13, and `expiresAt` (Unix timestamp; default = `acceptedAt + active_service_cutoff` per FR-P42).
- **FR-P41**: At startup, every agent MUST replay the persisted active-service entries: for each entry whose state is `{AGREED, FUNDED, ACTIVE, INVOICED}` and whose `expiresAt` is in the future, the agent MUST resume operation as if the process had never stopped. Resumption MUST NOT require a fresh `serviceRequest`/`serviceResponse` exchange, MUST NOT require the counterparty to re-issue any prior message, and MUST NOT consult the chain or topic backend before resuming the data plane (see Section K). Replay failure for any single entry MUST NOT prevent replay of other entries; failures MUST surface as `ActiveServiceReplayFailed` (FR-P32) with per-entry diagnostics.
- **FR-P42**: Active-service entries MUST be evicted when their `expiresAt` is reached. The eligibility window (the default delta between `acceptedAt` and `expiresAt` for new entries) MUST be **configurable**, with a default value of **86400 seconds (24 hours)**. Implementations MUST expose this value as a configuration knob (e.g., `ActiveServiceCutoff` field, `NEURON_ACTIVE_SERVICE_CUTOFF` environment variable, or equivalent). Setting the value to `0` disables auto-eviction entirely (entries persist until explicit `serviceStop` / `serviceCancel`); setting a finite positive value is the default operational mode. Receipt of a valid `serviceRenew` (FR-P38) updates `expiresAt` to `extendUntil`, effectively re-anchoring the eviction clock.
- **FR-P43**: The persistence storage format is implementation-defined (JSON file, embedded KV store, SQLite, or equivalent). Conformance is verified **behaviorally**: a conforming SDK MUST pass a "restart-and-resume" integration test in which a seller serving an ACTIVE service is killed and restarted, and the data-plane stream resumes within an implementation-defined recovery window (recommended ≤ 5 seconds for in-memory adapters, ≤ 30 seconds for adapters that re-establish libp2p connectivity).
- **FR-P44**: Persisted active-service entries MUST NOT contain settlement-binding private keys, escrow signing material, or any other secret. They MAY contain public references (`escrowRef.locator`, `peerID`, multiaddr strings, signed `connectionSetup` envelopes). Recovery uses the agent's existing NeuronPrivateKey (002) and the binding-specific signing path (008 EscrowAdapter); persisted state stores only the *observed agreement state*, not the means to act on it.

**J. Long-Lived Service Discipline** *(2026-05-08 amendment)*

- **FR-P45**: Once an agreement reaches ACTIVE, the seller MUST NOT discontinue stream production except for one of the following authoritative causes:
  (a) Receipt of a valid `serviceStop` (FR-P36) from the buyer.
  (b) Receipt of a valid `serviceCancel` (FR-P37) from either party.
  (c) Process shutdown (after which Section I replay restores the entry on next start).
  (d) Eviction of the active-service entry per FR-P42 (`expiresAt` reached without renewal).
  (e) Settlement-binding hard failure (escrow contract reverts, refund timeout exhaustion); MUST be surfaced as a structured error and SHOULD be evidenced via a final `invoice` referencing the failure.
- **FR-P46**: The seller MUST NOT terminate the data-plane stream on the basis of:
  - Resource pressure (memory, CPU, fan-out backpressure) — admission decisions for new buyers are governed by Section L; existing ACTIVE buyers MUST be kept served until one of the FR-P45 conditions is met.
  - Transient transport faults (libp2p stream closure, brief network outages, NAT mapping refresh). Reconnection is governed by 009 FR-D08/FR-D09 and MUST resume the existing active-service entry without re-negotiation.
  - Control-plane unavailability (HCS or chain outage). See Section K.
- **FR-P47**: The buyer MAY at any time send `serviceStop` (FR-P36) — that is the *only* protocol-defined trigger for the seller to stop. Buyer-initiated stop is the default and intentional path. Buyers SHOULD send `serviceStop` rather than silently disappearing; silent disappearance leaves the seller streaming until FR-P42 eviction.

**K. Degraded-Mode Operation** *(2026-05-08 amendment)*

- **FR-P50**: Existing active-service entries (Section I) are sufficient authority to keep streaming. An agent in possession of a valid, non-expired active-service entry MUST NOT consult the chain, topic backend, registry, or reputation/validation registries to decide whether to *continue* an already-ACTIVE service. The entry IS the local source of truth.
- **FR-P51**: When the topic backend is unreachable:
  - Heartbeat publishing (Spec 005) MAY pause; observers handle this per 005 FR-H-degraded.
  - Negotiation messages (`serviceRequest`, `serviceResponse`, `escrowCreated`, `invoice`, `invoiceAck`, `serviceStop`, `serviceCancel`, `serviceRenew`) MAY be queued for later publication; the agent SHOULD retry per 004 FR-T26.
  - Already-ACTIVE data-plane streams MUST continue. Newly-arriving frames MUST flow without interruption. The agent MUST NOT close streams to "wait for the chain to recover".
- **FR-P52**: When the settlement binding is unreachable (e.g., EVM RPC down, Hedera mirror unavailable):
  - `getBalance(escrowRef)` MAY fail; the seller's funding-compliance check (FR-P27a) operates on stale data per the agreed terms.
  - `requestRelease` and `claimRefund` MAY be queued, retried, or fail with `BindingUnavailable` (FR-P32).
  - The data-plane stream MUST continue if the active-service entry is otherwise valid. Settlement may catch up when the binding recovers.
- **FR-P53**: Degraded-mode operation does NOT bypass settlement preconditions (FR-P25). When the binding recovers, releases and refunds remain subject to escrow arithmetic and timeout rules. Continued data-plane operation under degraded mode is an *operational* commitment by the seller; it does not constitute on-chain authorization for additional fund movement.

**L. AdmissionPolicy Interface Stub** *(2026-05-08 amendment; per Constitution Principle XII)*

- **FR-P55**: System MUST define a small, declarative `AdmissionPolicy` interface as a Core SDK primitive. The interface is intentionally minimal and policy-free; concrete admission semantics (priority lists, allowlists, denylists, per-tenant rules, fan-out routing decisions) belong in DApp specs (e.g., 016 ADS-B, 017 Remote ID).
- **FR-P56**: The `AdmissionPolicy` interface MUST expose at minimum a single synchronous decision operation:
  - `Decide(buyer EVMAddress, service NeuronCommerceService, requestContext) → Decision`
  - `Decision` is an enum: `"allow-direct"` (seller serves this buyer on its own stream), `"allow-via-fanout"` (seller MAY off-load this buyer to a DApp-defined fan-out distribution path; if no fan-out path exists, treat as `"allow-direct"`), `"deny"` (refuse the agreement with `ServiceAdmissionDenied` error). The `requestContext` is an opaque struct providing access to the incoming `serviceRequest`, the seller's current per-buyer count, and any DApp-injected metadata.
  - Implementations of `AdmissionPolicy` are provided by the DApp layer or by application code; the Core SDK provides ONE built-in default implementation, `AllowAll`, which always returns `"allow-direct"` and exists solely to keep the SDK usable in DApp-free contexts.
- **FR-P57**: The Core SDK MUST NOT define any policy beyond `AllowAll`. Specifically: the Core SDK MUST NOT define priority lists, allowlists, denylists, per-organization rules, coalition-partner rules, sensor-class hierarchies, or any other domain-specific admission semantics. DApp specs MAY define such policies and MAY register implementations through the `AdmissionPolicy` interface. This requirement enforces Constitution Principle XII boundary: admission *mechanism* is Core SDK; admission *policy* is DApp.

**M. Commerce Mode Descriptor** *(2026-05-12 amendment; optional polish; closes audit gap G5 — edge-demo env-var ladder undocumented at spec level)*

The edge demo's iteration ladder (`NEURON_EDGE_REGISTRATION_MODE`, `NEURON_EDGE_PAYMENT_MODE`) determines whether the seller runs the full commerce stack, registers without settling, or runs the data plane in isolation. A validator agent (010) observing heartbeats alone today cannot tell which sub-flow is active. The descriptor below makes the sub-flow visible at the protocol level, parallel to the connectivity-profile descriptor pattern in 013 and the feed-source descriptor pattern in 016 §H / 017 §G.

- **FR-P58**: A seller MAY advertise its commerce sub-flow posture in heartbeat capabilities (005 FR-H05) under the key `commerceMode`. The allowed values are:
  - **`full`** — registration engaged (003 + 007 `register()` + `agentURI` write), negotiation engaged (008 FR-P06 messages on stdIn/stdOut), and settlement engaged (008 Section H + I + J). The canonical operational posture.
  - **`registration-only`** — registration engaged; negotiation and settlement skipped. Used during deployment-test phases where on-chain identity is established but the commerce flow is not yet wired or is intentionally disabled (e.g., the early iterations of the edge demo before payment was added).
  - **`data-only`** — registration and settlement skipped; the data plane runs in isolation. Used by Phase 2 vertical-slice fixtures (e.g., `cmd/remoteid-{seller,buyer}` operating under Fixture Profile F per 013). Sellers in `data-only` mode SHOULD additionally advertise `profile = "f-fixture-direct/1"` per 013 FR-F-02.
    Absent advertisement defaults to `"full"` (operational posture). A validator agent (010) observing `commerceMode ≠ "full"` MUST treat the run as fixture or degraded-operational evidence (depending on context) and MUST disclose the value in any verdict envelope it publishes. False-`full` advertisement is a discipline failure, not a structural protocol failure; the discipline mechanism is the disclosure rule, not a runtime check.

`commerceMode` is **descriptive**, not prescriptive — advertising `commerceMode = "data-only"` does not relieve the seller of any other normative obligation. A seller operating under Fixture Profile F (013) which itself authorises skipping 003/007/008 (per 013 FR-F-01) advertises `commerceMode = "data-only"` to be explicit, but it is the Profile F advertisement that licences the deviation, not the commerce-mode advertisement.

### Key Entities

- **NeuronCommerceService**: EIP-8004 service object in agentURI `services[]`. Declares: service identity (name, version), delivery descriptor (mode, service/channel reference), settlement binding (binding, chain-specific config), pricing (amount, currency, unit, interval), and terms reference (URI).
- **DeliveryDescriptor**: Object within `NeuronCommerceService` describing how service data is delivered. Contains `mode` (delivery mechanism identifier: `"p2p"`, `"topic"`, or `"custom:<type>"`) and a mode-specific cross-reference (`serviceRef` for P2P → `neuron-p2p-exchange`, `channelRef` for topic → `neuron-topic`).
- **EscrowRef**: Opaque reference to a settlement escrow instance. Contains `binding` (identifies settlement binding) and `locator` (binding-specific address/ID string).
- **ReleaseRequestRef**: Opaque reference to a pending release request within an escrow. Same structure as EscrowRef.
- **AgreementState**: Lifecycle state for a buyer-seller agreement. One of: IDLE, REQUESTED, NEGOTIATING, AGREED, FUNDED, ACTIVE, INVOICED, COMPLETED, TERMINATED, REJECTED.
- **EscrowAdapter**: Abstract interface defining six operations (createEscrow, deposit, getBalance, requestRelease, approveRelease, claimRefund) that all settlement bindings implement.
- **StreamCatalogEntry**: An entry in the OPTIONAL `streams[]` field of `connectionSetup` (FR-P33a). Contains `name`, `protocolID` (literal or wildcard), `direction`, and OPTIONAL `schema`. Per Constitution Principle XII, the contents of any specific stream catalog (frame format, control commands, filter parameters) belong to the DApp consuming this primitive; 008 defines only the envelope.
- **ActiveServiceEntry**: A persisted record (Section I) capturing the minimum state required to resume serving or consuming a service across process restart. Includes `requestId`, `counterpartEVM`, `role`, agreement reference, `escrowRef`, latest `connectionSetup`, `lastInvoiceSeq`, lifecycle state, and `expiresAt`. Persistence format is implementation-defined; conformance is verified behaviorally per FR-P43.
- **AdmissionPolicy**: A Core SDK interface (FR-P55–FR-P57) for seller-side admission decisions. Exposes one synchronous `Decide` operation returning `"allow-direct"`, `"allow-via-fanout"`, or `"deny"`. The Core SDK provides only the `AllowAll` default; concrete admission semantics live in DApp specs per Constitution Principle XII.
- **CommerceMode** *(2026-05-12 amendment)*: A descriptive heartbeat-advertised enum per FR-P58. Allowed values: `"full"`, `"registration-only"`, `"data-only"`. Defaults to `"full"` when absent. Closes the discoverability gap for sellers running with portions of the commerce stack disabled. Pairs with 013 Fixture Profile F for fully-disclosed fixture runs.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-P01**: A buyer agent can discover a seller's commercial offering from the agentURI, negotiate terms, create an escrow, receive service, and settle payment through a complete IDLE → ACTIVE → INVOICED → ACTIVE → COMPLETED cycle on at least two settlement bindings (hedera-native and evm-escrow).
- **SC-P02**: The `neuron-commerce` service type serializes to canonical JSON and round-trips identically (byte equality) across any two conforming SDK implementations.
- **SC-P03**: All six commerce payload types (serviceRequest, serviceResponse, connectionSetup, escrowCreated, invoice, invoiceAck) produce deterministic signed TopicMessages — signing the same payload twice yields identical bytes.
- **SC-P04**: Agreement lifecycle state transitions are deterministic — given the same sequence of negotiation messages, any two SDK implementations compute the same current state.
- **SC-P05**: The abstract `EscrowAdapter` interface is implementable on both Hedera (via SharedAccount + ScheduleCreate/Sign) and EVM (via NeuronEscrow.sol + ERC-20) without changes to the protocol layer.
- **SC-P06**: `claimRefund` succeeds only after timeout has elapsed and fails before timeout, on both bindings.
- **SC-P07**: `agreementHash` stored at escrow creation matches `keccak256(canonicalJSON(acceptedServiceResponse))` — verifiable by a third party with access to the topic message history.
- **SC-P08**: `evidenceHash` stored in release request matches `keccak256(canonicalJSON(deliveryProofTopicMessage))` — verifiable by a third party with access to the seller's stdOut.
- **SC-P09**: Settlement preconditions are enforced at the escrow level — `requestRelease(amount)` MUST fail when `amount` exceeds `getBalance(escrowRef).available`. Service-level funding compliance (e.g., minimum balance for FUNDED → ACTIVE) is determined by the agreed terms, not protocol-mandated.
- **SC-P10**: The `neuron-commerce` service type with `delivery` field serializes to canonical JSON and round-trips identically (byte equality), including cross-reference validation of `delivery.serviceRef` (against `neuron-p2p-exchange` names) and `delivery.channelRef` (against `neuron-topic` names) per FR-P01b.
- **SC-P11**: `connectionSetup` with `encryptedMultiaddrs` produces a deterministic signed TopicMessage — the non-encrypted fields (`type`, `version`, `requestId`, `peerID`, `protocol` and/or `streams`) are publicly readable by third-party validators; the `encryptedMultiaddrs` field is opaque to any party other than the intended recipient.
- **SC-P12** (2026-05-08 amendment): `serviceStop`, `serviceCancel`, and `serviceRenew` round-trip through canonical JSON serialization with byte equality across any two conforming SDK implementations, and the receiver's lifecycle state machine reaches the same end state regardless of which conforming implementation processes the message.
- **SC-P13**: A seller serving an ACTIVE service whose process is killed and restarted resumes data-plane streaming to the buyer without any new `serviceRequest` / `serviceResponse` exchange and without requiring chain reachability during the restart window. Verifiable by integration test with simulated restart.
- **SC-P14**: A `connectionSetup` payload with a multi-stream `streams[]` catalog (e.g., one literal protocol-ID entry plus one wildcard entry) round-trips through canonical JSON with byte equality; the receiver registers handlers for each entry and can open or accept streams whose protocol IDs match the declared entries (literal match for literal entries; pattern match per 009 FR-D-wildcard-handler for wildcard entries).
- **SC-P15**: A seller in possession of a valid, non-expired active-service entry continues to serve data when the topic backend is unreachable for at least the configured `ActiveServiceCutoff` duration (default 24 hours). No data-plane stream is closed by the seller during the outage.
- **SC-P16**: The Core SDK's `AllowAll` default implementation of `AdmissionPolicy` returns `"allow-direct"` for every `Decide` call. No other policy is built into the Core SDK; all other behaviors require a DApp-provided implementation.
- **SC-P17** *(2026-05-12 amendment)*: A seller's heartbeat advertises `commerceMode ∈ {"full", "registration-only", "data-only"}` per FR-P58, OR omits the key (interpreted as `"full"`). Verified by parsing the seller's heartbeat from each of the three reference seller postures (full edge demo with registration+payment, edge demo with registration-only, Phase 2 vertical-slice with data-only) and asserting the advertised value matches the configured posture.

---

## Third-Party Validation *(mandatory)*

### Verification Tier

**`topic-observable`** for the protocol layer. **`on-chain-only`** for settlement binding compliance.

A third-party validator can verify payment protocol compliance by subscribing to both parties' stdIn/stdOut topics (protocol layer) and querying on-chain escrow state (binding layer). No access to agent internals is required.

### Validator Checklist

- **VR-PAY-01**: Observe `serviceRequest` on seller's stdIn and `serviceResponse` on buyer's stdIn. Pass if both payloads contain all MUST fields per FR-P07/FR-P08 and canonical JSON field ordering per FR-P12. Fail if MUST fields are missing or field order is non-canonical.
- **VR-PAY-02**: Observe `escrowCreated` on seller's stdIn. Query escrow balance on-chain. Pass if `depositAmount` in the message matches the on-chain balance. Fail if mismatch.
- **VR-PAY-03**: Observe `invoice` on buyer's stdIn. Verify `evidenceHash` by finding the matching TopicMessage on seller's stdOut and computing `keccak256(canonicalJSON(topicMessage))`. Pass if hashes match. Fail if no matching message or hash mismatch.
- **VR-PAY-04**: Observe `invoiceAck` (action: `approved`) on seller's stdIn. Query on-chain escrow for release confirmation. Pass if funds transferred to seller's paymentAddress. Fail if funds remain locked after approval.
- **VR-PAY-05**: Query on-chain escrow `agreementHash`. Compute `keccak256(canonicalJSON(acceptedServiceResponse))` from topic history. Pass if values match. Fail if mismatch.
- **VR-PAY-06**: After timeout has elapsed with no release, verify buyer can claim refund. Pass if `claimRefund` returns funds to buyer. Fail if funds remain locked past timeout.
- **VR-PAY-07**: Observe `connectionSetup` on seller's stdIn after `serviceResponse` (accept). Pass if non-encrypted fields (`type`, `version`, `requestId`, `peerID`, and at least one of `protocol` or `streams[]` per FR-P33a) are present and well-formed per FR-P33. The validator CANNOT verify `encryptedMultiaddrs` content (encrypted by design, FR-P34). Fail if MUST fields are missing or field order is non-canonical per FR-P12.
- **VR-PAY-08** (2026-05-08 amendment): Observe `serviceStop` on seller's stdIn after the agreement reaches ACTIVE. Pass if MUST fields (`type`, `version`, `requestId`) are present and the `requestId` matches an agreement the validator has tracked through ACTIVE. Pass if the seller subsequently issues at most one final `invoice` and ceases data-plane production within an implementation-defined cease window. Fail if data-plane production is observably continuing more than the cease window after `serviceStop` (gauged by the validator's external probes or by absence of subsequent activity on observable proxies).
- **VR-PAY-09** (2026-05-08 amendment): Observe `serviceCancel` on either party's stdIn during any pre-COMPLETED state. Pass if MUST fields are present and `requestId` matches an agreement the validator has tracked. Pass if the agreement reaches TERMINATED on observable proxies (no further `invoice` issuance, no further `connectionSetup` or `serviceRenew` for the same `requestId`).
- **VR-PAY-10** (2026-05-08 amendment): Observe `serviceRenew` on seller's stdIn during ACTIVE or INVOICED. Pass if MUST fields (`type`, `version`, `requestId`, `extendUntil`) are present, `extendUntil` is strictly greater than any prior `extendUntil` (or the agreement's original `expiresAt`), and the agreement remains observably ACTIVE / INVOICED. Fail if `extendUntil` is non-monotonic.
- **VR-PAY-11** *(2026-05-12 amendment)*: Observe a seller's heartbeat. Pass if `capabilities.commerceMode ∈ {"full", "registration-only", "data-only"}` per FR-P58 or absent (defaulted to `"full"`). The validator's verdict envelope MUST record the observed value verbatim so downstream reviewers can correlate the seller's commerce posture with on-chain activity. Fail if the advertisement is present but malformed (unknown enum value, wrong JSON type). Inconclusive if the heartbeat is unavailable.

**Funding compliance observation**: Third-party validators MAY evaluate whether parties complied with service-level funding requirements by comparing `getBalance(escrowRef)` snapshots against the agreed terms (discoverable via `agreementHash` → topic history → negotiation payloads → `termsRef`). The specific compliance criteria are agreement-defined, not protocol-defined. Validators assess compliance against the terms the parties agreed to, not against protocol-imposed thresholds.

### Observable State Commitments

All commerce payloads (FR-P06–FR-P11, FR-P33) are published to stdIn/stdOut topics — these are the primary observable state for the protocol layer. No additional topic messages beyond those defined in FR-P06 are required.

For on-chain observability:
- **hedera-native**: `agreementHash` and `evidenceHash` are stored as transaction memos (FR-P17, FR-P20), visible via Hedera mirror node queries.
- **evm-escrow**: `EscrowCreated`, `ReleaseRequested`, `ReleaseApproved`, `Withdrawn`, `RefundClaimed` events provide on-chain observability per Constitution Principle XI.
