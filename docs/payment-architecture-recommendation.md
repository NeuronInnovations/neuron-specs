# Payment Architecture Recommendation

| Field           | Value                                                                            |
| --------------- | -------------------------------------------------------------------------------- |
| **Date**        | 2026-03-19 (v4 — chain-agnostic + EVM best-practice corrections)                |
| **Status**      | Architecture Recommendation (pre-spec)                                           |
| **Inputs**      | Specs 001–007, Constitution v1.4.0, legacy SDK, architecture notes, EVM best practices |
| **Supersedes**  | v1 (ERC-8183-centered), v2 (Hedera-centered), v3 (pre-EVM-review)               |
| **Target Spec** | 008 Payment                                                                      |

> **This is NOT a specification.** It is an architectural recommendation informing the creation of Spec 008 — Payment. The design follows the same chain-agnostic pattern established by Spec 004 (TopicAdapter) and Spec 007 (FR-C-17: any EVM chain). The original shared-account mechanism is preserved as one settlement binding among equals.

---

## 1. Executive Summary

**Recommendation: A single Spec 008 — Payment — with a chain-agnostic protocol layer and pluggable settlement bindings, mirroring how Spec 004 abstracts topic transports.**

```
┌───────────────────────────────────────────────────────────┐
│  Protocol Layer (normative, chain-agnostic)               │
│  A. Service Offering    — neuron-commerce in services[]   │
│  B. Negotiation         — TopicMessage payloads           │
│  C. Agreement Lifecycle — state machine                   │
│  D. Escrow Interface    — abstract operations             │
│  E. Payment Address     — resolution rules                │
├──────────────────┬────────────────────┬───────────────────┤
│  Binding:        │  Binding:          │  Future:          │
│  hedera-native   │  evm-escrow        │  x402, streaming  │
│  SharedAccount   │  NeuronEscrow.sol  │  ERC-8183, etc.   │
│  + scheduled TX  │  + ERC-20 + permit │                   │
└──────────────────┴────────────────────┴───────────────────┘
```

This follows the **TopicAdapter precedent**: Spec 004 defines abstract pub/sub operations (publish, subscribe, resolve) and then binds to HCS, ERC-log, and Kafka. Spec 008 defines abstract escrow operations (create, deposit, requestRelease, approveRelease, refund) and then binds to Hedera native and EVM contract.

Key decisions:

| Decision | Choice |
|----------|--------|
| Architecture model | Abstract escrow interface + chain-specific settlement bindings |
| Protocol layer | Chain-agnostic — no Hedera or EVM primitives in Sections A–C, E |
| First bindings | `hedera-native` AND `evm-escrow` ship together as co-equal |
| Shared-account mechanism | Preserved in full as the `hedera-native` binding |
| EVM escrow | `NeuronEscrow.sol` — minimal Solidity contract; ERC-20, permit, timelock |
| Service offering | `neuron-commerce` in agentURI `services[]` with `settlement` object |
| Negotiation | TopicMessage payload types on stdIn/stdOut |
| Spec count | Single spec: 008 Payment |

---

## 2. Current Neuron Architecture Baseline

### 2.1 Design Pattern

Spec 008 follows the same chain-agnostic pattern used by Spec 004 (TopicAdapter: abstract interface + chain-specific bindings), Spec 007 (FR-C-17: deployable on any EVM chain), and Spec 001 (FR-018: ledger-agnostic attachment). Per the Constitution and Principle VIII, the protocol layer is blockchain-agnostic, with Hedera as the primary binding and EVM as a co-equal binding.

### 2.2 Architectural Seams Relevant to Payment

**Chain-agnostic seams (usable by all bindings):**

| Seam | Spec | What It Provides |
|------|------|-----------------|
| `TopicMessage.payload` extensibility | 004 FR-T20 | Opaque payload with `type` discriminator — new payment message types |
| `services[]` in agentURI | 003 FR-R02/R03 | Typed service descriptions — `neuron-commerce` for service offerings |
| `paymentAddress` resolution | 001 FR-023/024 | Child → Parent address mapping — who receives payment |
| Reputation Registry | 007 FR-C-20–C-26 | Pre-engagement trust gating and post-settlement feedback |
| Validation Registry | 007 FR-C-27–C-33 | Capability verification before engagement |
| Canonical JSON | 006 FR-W01–W10 | Deterministic serialization for payment messages |

**Hedera-specific seams (only for hedera-native binding):**

| Seam | Spec | What It Provides |
|------|------|-----------------|
| SharedAccount | 001 FR-021, FR-007a | Multisig threshold account — escrow container on Hedera |
| MultisigKey `"hedera-threshold"` | 002 FR-023/024 | Hedera native threshold keys (not blocked by GAP-005) |
| LedgerAttachment | 001 FR-018/019 | Link SDK-level SharedAccount to on-chain Hedera account |

**EVM-specific seams (only for evm-escrow binding):**

| Seam | Standard | What It Provides |
|------|----------|-----------------|
| ERC-20 | EIP-20 | Token standard for escrow deposits |
| ERC-2612 permit | EIP-2612 | Gasless token approvals via signature |
| Solidity events | EVM | On-chain observability for validators (Principle XI) |
| Block timestamps | EVM | Timeout enforcement for refund eligibility |

---

## 3. Architectural Fit Analysis

### 3.1 Escrow Operation Mapping Across Chains

Escrow implementations share identical semantics across chains — different on-chain primitives, same lifecycle:

| Abstract Operation | Hedera Native | EVM Contract |
|-------------------|---------------|-------------|
| `createEscrow` | Create threshold-key account | Deploy contract instance |
| `deposit` | CryptoTransfer to shared account | ERC-20 transfer to contract |
| `getBalance` | AccountInfoQuery | Contract `balanceOf()` |
| `requestRelease` | ScheduleCreate (1st sig) | Contract `requestRelease()` |
| `approveRelease` | ScheduleSign (2nd sig) | Contract `approveRelease()` |
| `claimRefund` | Reverse schedule / timeout | Contract `claimRefund()` after expiry |

### 3.2 Dependency Surface

| Spec | What 008 Needs | Coupling |
|------|---------------|----------|
| 001 | paymentAddress (FR-023/024), balance model | **Hard** |
| 003 | agentURI services[] for `neuron-commerce` | **Hard** |
| 004 | TopicMessage for negotiation/invoice payloads | **Hard** |
| 006 | Canonical JSON for payload determinism | **Hard** (reference) |
| 002 | Signing (transitive via 004); MultisigKey (hedera binding) | **Hard** |
| 005 | LivenessState check | **Soft** |
| 007 | Reputation/Validation queries | **Soft** |

Each binding adds its own chain-specific dependencies (ERC-20/Permit2 for EVM; SharedAccount/MultisigKey for Hedera). No binding's dependencies leak into the protocol layer.

---

## 4. Boundary Decisions

### 4.1 What Belongs in the Protocol Layer (Chain-Agnostic)

| Concept | Rationale |
|---------|-----------|
| `neuron-commerce` service type schema | Service discovery — chain-agnostic; includes `settlement` object for binding selection |
| Negotiation payloads (serviceRequest, serviceResponse, invoice, invoiceAck) | Topic-based coordination — same messages regardless of settlement chain |
| Agreement lifecycle state machine | Universal states (IDLE → REQUESTED → AGREED → FUNDED → ACTIVE ⇄ INVOICED → COMPLETED) |
| Abstract escrow operations interface | 6 operations that all bindings implement |
| Balance validation rules | Universal business rules (min balance, stop-service threshold) |
| Payment address resolution | Identity-layer concern (001 FR-023/024) — chain-agnostic |
| Error codes (`NEURON-PAYMENT-*`) | Unified taxonomy per 006 |

### 4.2 What Belongs in Settlement Bindings (Chain-Specific)

| Concept | Binding | Rationale |
|---------|---------|-----------|
| SharedAccount + MultisigKey threshold | hedera-native | Hedera-specific escrow primitive |
| ScheduleCreate/ScheduleSign | hedera-native | Hedera-specific invoice/counter-sign |
| HBAR transfers | hedera-native | Hedera native currency |
| NeuronEscrow.sol contract | evm-escrow | EVM-specific escrow contract |
| ERC-20 token handling | evm-escrow | EVM token standard |
| ERC-2612 permit | evm-escrow | EVM gasless approval |
| Block-timestamp timelock | evm-escrow | EVM-specific timeout |
| Solidity events | evm-escrow | EVM-specific observability |

### 4.3 What Belongs in App-Layer

| Concept | Why App-Layer |
|---------|---------------|
| SLA content and format | dApp-defined; 008 provides `termsRef` URI pointer |
| Pricing algorithms | `pricing` field is declarative; computation is agent-specific |
| Marketplace UI/search | Frontend concern |
| Revenue split ratios (60/40, etc.) | Product configuration per deployment |
| Arbiter selection criteria | Domain-specific; protocol defines the role, not who fills it |
| Dispute resolution beyond refund | Beyond protocol scope; arbiter logic is app-layer |

### 4.4 Outside the Scope of Spec 008

| Concept | Why Out of Scope |
|---------|-------------|
| x402 HTTP payment binding | Different payment model (stateless); needs its own binding spec |
| Streaming payment binding (Sablier/Superfluid-style) | Different payment model (continuous); needs its own binding spec |
| ERC-8183 integration | Can be added as another binding when market adoption matures |
| Cross-chain escrow (bridge-based) | Requires bridge infrastructure |
| Service Description Framework | Full dApp-level schemas |

---

## 5. Architecture

### 5.1 Section A: Service Offering Schema

A `neuron-commerce` service type in agentURI `services[]`:

```json
{
  "type": "neuron-commerce",
  "name": "adsb-data-feed",
  "version": "1.0.0",
  "settlement": {
    "binding": "evm-escrow",
    "chainId": 1,
    "contract": "0x...",
    "token": "0x..."
  },
  "pricing": {
    "amount": "100",
    "currency": "USDC",
    "unit": "wei",
    "interval": "3600"
  },
  "termsRef": "ipfs://Qm.../adsb-terms-v1.json"
}
```

Or for Hedera:

```json
{
  "type": "neuron-commerce",
  "name": "adsb-data-feed",
  "version": "1.0.0",
  "settlement": {
    "binding": "hedera-native",
    "network": "hedera-mainnet"
  },
  "pricing": {
    "amount": "100",
    "currency": "HBAR",
    "unit": "millibar",
    "interval": "3600"
  },
  "termsRef": "ipfs://Qm.../adsb-terms-v1.json"
}
```

Fields:

- `type`: Always `"neuron-commerce"` (MUST)
- `name`: Human-readable service name (MUST)
- `version`: Service version, semver (MUST)
- `settlement`: Settlement binding descriptor (MUST)
- `settlement.binding`: Binding identifier — `"hedera-native"` | `"evm-escrow"` (MUST)
- `settlement.*`: Binding-specific fields (chain ID, contract address, network, token) (binding-dependent)
- `pricing`: Declarative pricing object (MUST)
- `pricing.amount`: Price per interval as string decimal (MUST)
- `pricing.currency`: Currency symbol (MUST)
- `pricing.unit`: Denomination unit (MUST)
- `pricing.interval`: Billing interval in seconds; `"0"` = one-time (MUST)
- `termsRef`: URI to full service terms document (SHOULD)

Cardinality: 0–N `neuron-commerce` entries per agentURI. An agent offering the same service on multiple chains lists multiple entries with different `settlement` objects.

### 5.2 Section B: Negotiation Sub-Protocol

TopicMessage payload types — identical regardless of settlement binding:

**`serviceRequest`** — buyer → seller's stdIn:
```json
{
  "type": "serviceRequest",
  "version": "1.0.0",
  "requestId": "<uuid>",
  "serviceRef": "adsb-data-feed",
  "settlementBinding": "evm-escrow",
  "proposedAmount": "100",
  "proposedCurrency": "USDC",
  "proposedInterval": "3600",
  "arbiter": "0x...",
  "buyerStdIn": 12345
}
```

**`serviceResponse`** — seller → buyer's stdIn:
```json
{
  "type": "serviceResponse",
  "version": "1.0.0",
  "requestId": "<uuid>",
  "action": "accept|reject|counter",
  "counterAmount": "150",
  "counterInterval": "1800"
}
```

**`escrowCreated`** — buyer → seller's stdIn (after escrow setup):
```json
{
  "type": "escrowCreated",
  "version": "1.0.0",
  "requestId": "<uuid>",
  "escrowRef": "<binding-specific reference>",
  "depositAmount": "100",
  "depositCurrency": "USDC"
}
```

**`invoice`** — seller → buyer's stdIn:
```json
{
  "type": "invoice",
  "version": "1.0.0",
  "requestId": "<uuid>",
  "releaseRequestRef": "<binding-specific reference>",
  "escrowRef": "<binding-specific reference>",
  "amount": "100",
  "currency": "USDC",
  "period": "2026-03-18T10:00:00Z/2026-03-18T11:00:00Z"
}
```

**`invoiceAck`** — buyer → seller's stdIn:
```json
{
  "type": "invoiceAck",
  "version": "1.0.0",
  "requestId": "<uuid>",
  "releaseRequestRef": "<binding-specific reference>",
  "action": "approved|refused",
  "depositedMore": true,
  "newBalance": "200"
}
```

Note: `escrowRef` and `releaseRequestRef` are opaque strings whose format is binding-specific. The protocol layer treats them as identifiers — only the settlement binding interprets their content.

### 5.3 Section C: Agreement Lifecycle

State machine — chain-agnostic:

```
IDLE → REQUESTED → NEGOTIATING → AGREED → FUNDED → ACTIVE ⇄ INVOICED → COMPLETED
                       ↓                                          ↓
                    REJECTED                                  TERMINATED
```

States and transitions:

| From | To | Trigger | Message |
|------|----|---------|---------|
| IDLE | REQUESTED | Buyer sends service request | `serviceRequest` |
| REQUESTED | NEGOTIATING | Seller counter-offers | `serviceResponse` (action: counter) |
| REQUESTED | AGREED | Seller accepts | `serviceResponse` (action: accept) |
| REQUESTED | REJECTED | Seller rejects | `serviceResponse` (action: reject) |
| NEGOTIATING | AGREED | Both accept | `serviceResponse` (action: accept) |
| NEGOTIATING | REJECTED | Either withdraws | `serviceResponse` (action: reject) |
| AGREED | FUNDED | Buyer creates escrow + deposits | `escrowCreated` |
| FUNDED | ACTIVE | Seller verifies balance, begins delivery | (implicit — delivery starts) |
| ACTIVE | INVOICED | Seller requests release | `invoice` |
| INVOICED | ACTIVE | Buyer approves + deposits more | `invoiceAck` (action: approved) |
| INVOICED | TERMINATED | Buyer refuses or timeout | `invoiceAck` (action: refused) |
| ACTIVE | TERMINATED | Balance depleted or party stops | (explicit terminate message) |
| ACTIVE | COMPLETED | Mutual graceful shutdown | (explicit complete message) |

**Streaming cycle**: ACTIVE → INVOICED → ACTIVE → INVOICED → ... repeats until COMPLETED or TERMINATED.

### 5.4 Section D: Escrow Operations Interface (Abstract)

Six operations that ALL settlement bindings MUST implement:

```
EscrowAdapter
├── createEscrow(buyer, seller, arbiter?, currency, threshold,
│                agreementHash, timeout) → EscrowRef
├── deposit(escrowRef, amount) → DepositResult
├── getBalance(escrowRef) → Balance
├── requestRelease(escrowRef, amount, recipient,
│                  evidenceHash) → ReleaseRequestRef
├── approveRelease(escrowRef, releaseRequestRef) → ReleaseResult
└── claimRefund(escrowRef) → RefundResult
```

**Types (chain-agnostic):**

| Type | Fields | Description |
|------|--------|-------------|
| `EscrowRef` | `binding` (string), `locator` (string) | Opaque reference — binding interprets locator |
| `DepositResult` | `transactionRef` (string), `newBalance` (string) | Confirmation of deposit |
| `Balance` | `available` (string), `currency` (string), `lastSynced` (timestamp) | Current escrow balance |
| `ReleaseRequestRef` | `binding` (string), `locator` (string) | Opaque reference to pending release |
| `ReleaseResult` | `transactionRef` (string), `amountReleased` (string) | Confirmation of release |
| `RefundResult` | `transactionRef` (string), `amountRefunded` (string) | Confirmation of refund |

**Parameters added per the 2026 EVM best-practice review:**

| Parameter | Type | Operation | Purpose |
|-----------|------|-----------|---------|
| `agreementHash` | bytes32 | `createEscrow` | Links escrow to off-chain negotiation terms. Computed as `keccak256(canonicalJSON(acceptedServiceResponse))`. Stored on-chain for dispute evidence. |
| `timeout` | uint64 | `createEscrow` | Refund eligibility timestamp. After this, buyer can call `claimRefund()`. Binding-specific enforcement (Hedera: schedule expiry; EVM: `block.timestamp` check). |
| `evidenceHash` | bytes32 | `requestRelease` | Links release request to delivery proof. Computed as `keccak256(canonicalJSON(deliveryProofTopicMessage))`. Stored on-chain for verification. |

**Design note — `withdraw` is NOT an abstract operation.** How funds physically move after `approveRelease` is binding-internal. On Hedera, the scheduled transaction executes immediately upon threshold signature (push). On EVM, best practice is pull payment: `approveRelease` marks funds as claimable, and the recipient calls a binding-specific `withdraw()` function. The abstract interface returns `ReleaseResult` in both cases — the SDK caller does not need to know whether push or pull was used internally.

**Balance validation rules** (protocol-level, binding-agnostic):

- Seller MUST verify `getBalance(escrowRef).available ≥ agreedAmount` before starting work
- Seller SHOULD stop delivering if balance < 1× invoice amount
- Buyer SHOULD be notified when balance < 2× invoice amount
- Seller MUST NOT serve if balance = 0

#### D.1 Settlement Binding: `hedera-native`

Maps abstract operations to Hedera native primitives:

| Abstract Operation | Hedera Implementation |
|-------------------|----------------------|
| `createEscrow` | Create Hedera account with MultisigKey `"hedera-threshold"` (001 FR-021, 002 FR-023). Threshold: 2-of-2 (bilateral) or 2-of-3 (with arbiter). `agreementHash` stored as account creation transaction memo. `timeout` maps to scheduled transaction expiry window. `EscrowRef.locator` = Hedera AccountID string. |
| `deposit` | `CryptoTransfer` from buyer to shared account. Returns Hedera TransactionID. |
| `getBalance` | `AccountInfoQuery` on shared account. Balance in HBAR/tinybar. |
| `requestRelease` | `ScheduleCreate` with transfer from shared account to seller's paymentAddress. `evidenceHash` stored as schedule transaction memo. Seller provides first signature. `ReleaseRequestRef.locator` = Hedera ScheduleID string. |
| `approveRelease` | `ScheduleSign` on the schedule. Buyer provides second signature. Threshold met → transfer executes immediately (push payment — safe on Hedera). |
| `claimRefund` | `ScheduleCreate` with reverse transfer (shared account → buyer). Requires arbiter signature in 2-of-3, or mutual agreement in 2-of-2. Only permitted after `timeout` has elapsed. |

**Hedera adherence notes** (per Constitution):
- Scheduled transactions have a configurable expiry (default 30 minutes on Hedera)
- If buyer doesn't counter-sign within expiry, seller must re-issue invoice
- HBAR is the native currency; HTS tokens follow the same `CryptoTransfer` pattern
- GAP-005 does NOT block this binding — `"hedera-threshold"` MultisigKey is available now
- Push payment is the correct model for Hedera — `CryptoTransfer` is reliable and does not suffer from the recipient-DoS risk that affects EVM `transfer()` calls
- `agreementHash` and `evidenceHash` are stored as transaction memos (max 100 bytes on Hedera; hex-encoded bytes32 = 66 chars, fits)

#### D.2 Settlement Binding: `evm-escrow`

Maps abstract operations to an EVM smart contract. Design informed by the 2026 EVM best-practice review.

| Abstract Operation | EVM Implementation |
|-------------------|-------------------|
| `createEscrow` | Call `NeuronEscrow.create(seller, arbiter, token, timeout, agreementHash)`. Factory pattern — single deployed contract manages multiple escrow instances. `agreementHash` stored in Escrow struct. Returns `escrowId`. `EscrowRef.locator` = `chainId:contractAddress:escrowId`. |
| `deposit` | Three paths (ordered by preference): (1) Permit2 `depositWithPermit2()` — all tokens, all wallets including smart accounts; (2) ERC-2612 `depositWithPermit()` — compliant tokens + EOA wallets only; (3) Standard `deposit()` — requires prior `approve()`. All use `SafeERC20.safeTransferFrom`. |
| `getBalance` | Contract `getBalance(escrowId)` — view function, no gas. |
| `requestRelease` | Seller calls `requestRelease(escrowId, amount, recipient, evidenceHash)`. `evidenceHash` stored in ReleaseRequest struct. Contract records pending release. `ReleaseRequestRef.locator` = `chainId:contractAddress:releaseId`. |
| `approveRelease` | Buyer calls `approveRelease(escrowId, releaseId)`. If threshold met (buyer + seller, or 2-of-3 with arbiter), marks funds as **claimable** (does NOT transfer — pull payment pattern). |
| `withdraw` (binding-specific) | Recipient calls `withdraw(escrowId, releaseId)`. Transfers tokens via `SafeERC20.safeTransfer`. Protected by `ReentrancyGuard`. This operation is internal to the EVM binding and not exposed in the abstract interface. |
| `claimRefund` | Buyer calls `claimRefund(escrowId)` after `timeout` block timestamp has passed. Remaining balance becomes claimable by buyer via `withdraw`. |

**NeuronEscrow.sol design principles:**

- **Minimal surface**: No governance, no upgradability in v1. Solidity interfaces are INFORMATIVE, not normative (following 007 FR-C-37).
- **ERC-20 only**: Native ETH/MATIC wrapping is out of scope (use WETH).
- **SafeERC20**: ALL token interactions use OpenZeppelin `SafeERC20` (`safeTransfer`, `safeTransferFrom`, `forceApprove`). Required for non-standard tokens like USDT that do not return `bool`.
- **Three-tier deposit authorization**:

| Priority | Method | Tokens | Wallets | Gasless? | Requires |
|----------|--------|--------|---------|----------|----------|
| 1 | Permit2 `permitTransferFrom` | All ERC-20 | EOA + Smart Accounts | Yes (via ERC-1271) | One-time `approve` to Permit2 contract |
| 2 | ERC-2612 `permit` + deposit | Compliant only | EOA only | Yes | Token implements `permit()` |
| 3 | Standard `approve` + `transferFrom` | All ERC-20 | All | No (1 extra tx) | None |

  SDK SHOULD attempt in order 1 → 2 → 3. First successful path wins.

- **Pull payment pattern**: `approveRelease` marks funds as claimable (state change only, no external call). Recipient calls `withdraw()` to pull funds. This prevents recipient-DoS (a reverting recipient cannot block the release) and eliminates reentrancy risk during state transitions.
- **On-chain evidence linkage**:
  - `Escrow` struct includes `bytes32 agreementHash` — links to off-chain negotiation terms
  - `ReleaseRequest` struct includes `bytes32 evidenceHash` — links to delivery proof TopicMessage
- **EIP-712 domain separator**: `{name: "NeuronEscrow", version: "1", chainId: block.chainid, verifyingContract: address(this)}`. Provides replay protection across chains and contract instances. Used for Permit2 witness data and future off-chain signing features.
- **Replay protection**: `chainId` + `verifyingContract` in domain separator; unique auto-incrementing `escrowId` per instance; unique `releaseId` per release request; `block.timestamp` for timeout enforcement.
- **Configurable timeout**: Per-escrow `expiredAt` timestamp set at creation. After expiry, buyer can unilaterally claim refund. `block.timestamp` is the clock (reliable post-Merge, reliable on L2s).
- **2-of-2 or 2-of-3**: `arbiter == address(0)` means bilateral. Non-zero arbiter means 2-of-3 (any two of buyer/seller/arbiter).
- **Smart account compatibility**: All role checks use `msg.sender` (works for both EOAs and ERC-4337 smart accounts). No `tx.origin` checks (broken by EIP-7702). ERC-1271 dual verification designed in for future off-chain signing features.
- **Events** (on-chain observability per Constitution Principle XI):

```
EscrowCreated(escrowId, buyer, seller, arbiter, token, timeout, agreementHash)
Deposited(escrowId, depositor, amount, method)
ReleaseRequested(escrowId, releaseId, amount, recipient, evidenceHash)
ReleaseApproved(escrowId, releaseId, approver)
Withdrawn(escrowId, releaseId, amount, recipient)
RefundClaimed(escrowId, amount, buyer)
```

  The `method` field in `Deposited` records which authorization path was used (`"permit2"` | `"permit"` | `"standard"`).

- **Any EVM chain**: No chain-specific opcodes. Deployable on Ethereum, Base, Arbitrum, Hedera EVM, Polygon, etc. (following 007 FR-C-17).

### 5.5 Section E: Payment Address Resolution

Chain-agnostic rule per 001 FR-023/024:

- Seller (Child agent) has an EVMAddress derived from its NeuronPublicKey (002 FR-005)
- Seller's `paymentAddress` resolves to the Parent's EVMAddress
- The `recipient` in `requestRelease` SHOULD default to `paymentAddress`
- Revenue splitting (e.g., 60% Parent / 40% Child) is implemented as multiple `requestRelease` calls or a splitter contract — this is a product configuration, not a protocol requirement

### 5.6 Trust Gating (SDK-Level)

Pre-engagement checks — applicable to ALL settlement bindings:

1. **Liveness**: Subscribe to seller's stdOut; require `LivenessState == ALIVE` (005 FR-H19)
2. **Identity**: `IIdentityRegistry.lookup(sellerAddress)` → verify registered (007 FR-C-10)
3. **Reputation**: `IReputationRegistry.getSummary()` → verify score threshold (007 FR-C-24)
4. **Validation**: `IValidationRegistry.getSummary()` → verify capability (007 FR-C-31)

Post-settlement: Buyer calls `IReputationRegistry.giveFeedback()` directly (007 FR-C-20). No automatic hooks — correct `msg.sender` attribution preserved.

All trust checks are SHOULD-level recommendations. Buyers that skip them accept the risk.

---

## 6. Normative Core vs Extension Surface

### 6.1 MUST Be Normative (Protocol Layer)

| Item | Rationale |
|------|-----------|
| `neuron-commerce` service type schema (including `settlement` object) | Interoperability: buyers discover sellers' offerings and supported bindings |
| Negotiation payload schemas (serviceRequest, serviceResponse, escrowCreated, invoice, invoiceAck) | Interoperability: agents negotiate using the same format regardless of binding |
| Agreement lifecycle states and transitions | Determinism: observers agree on agreement state |
| Abstract escrow operations (6 operations + return types, including `agreementHash`, `timeout`, `evidenceHash` parameters) | SDK portability: all bindings implement the same interface; evidence linkage is universal |
| Balance validation rules (min balance, stop-service, zero-balance refusal) | Correctness: prevents legacy "serve anyway" anti-pattern |
| Canonical field order for all new payload types | Determinism per 006 FR-W05 |
| Error codes: `NEURON-PAYMENT-*` domain | Consistency per 006 error taxonomy |

### 6.2 MUST Be Normative (Per Binding)

| Item | Binding | Rationale |
|------|---------|-----------|
| SharedAccount + MultisigKey mapping; memo format for `agreementHash`/`evidenceHash` | hedera-native | Implementors must agree on how abstract ops map to Hedera |
| NeuronEscrow.sol interface (function signatures, events, reverts); pull payment semantics; EIP-712 domain | evm-escrow | Implementors must agree on contract ABI and payment flow |
| Three-tier deposit authorization (Permit2 → ERC-2612 → approve) and SDK fallback order | evm-escrow | Interoperability: all EVM SDKs attempt the same paths in the same order |
| SafeERC20 usage for all token interactions | evm-escrow | Correctness: non-standard tokens (USDT) must work |
| EscrowRef locator format | Both | SDK must parse locator strings consistently |

### 6.3 SHOULD Remain Extensible

| Item | How |
|------|-----|
| Settlement bindings | New `binding` values in `settlement` object; new binding spec section |
| Pricing models | `pricing.interval`: "0" (one-time), "3600" (hourly), "86400" (daily) |
| Token support | `settlement.token` accepts any ERC-20 address; Hedera supports HBAR + HTS |
| Arbiter selection | Per-agreement; address, multisig, or omitted |
| Revenue split | Configurable per deployment |
| Service terms format | `termsRef` URI: any document format |

### 6.4 SHOULD Remain Out of Scope

| Item | Why |
|------|-----|
| x402 / streaming / ERC-8183 | Different payment models; future binding specs |
| SLA content semantics | dApp-defined |
| Marketplace / price discovery | Frontend concern |
| Cross-chain escrow (bridging) | Requires bridge infra |
| On-chain agentURI parsing | 007 DD-01: agentURI is opaque on-chain |

---

## 7. Determinism and Conformance

### 7.1 Canonical Field Orders

**`neuron-commerce` service**: `type` → `name` → `version` → `settlement` → `pricing` → `termsRef`

**`settlement` object (evm-escrow)**: `binding` → `chainId` → `contract` → `token`

**`settlement` object (hedera-native)**: `binding` → `network`

**`pricing` object**: `amount` → `currency` → `unit` → `interval`

**`serviceRequest`**: `type` → `version` → `requestId` → `serviceRef` → `settlementBinding` → `proposedAmount` → `proposedCurrency` → `proposedInterval` → `arbiter` → `buyerStdIn`

**`serviceResponse`**: `type` → `version` → `requestId` → `action` → `counterAmount` → `counterInterval`

**`escrowCreated`**: `type` → `version` → `requestId` → `escrowRef` → `depositAmount` → `depositCurrency`

**`invoice`**: `type` → `version` → `requestId` → `releaseRequestRef` → `escrowRef` → `amount` → `currency` → `period`

**`invoiceAck`**: `type` → `version` → `requestId` → `releaseRequestRef` → `action` → `depositedMore` → `newBalance`

All numeric fields: JSON string decimal per 006 FR-W02. Optional fields: omit key per 006 FR-W04.

### 7.2 Test Vectors

**Chain 5: Service Commerce Signing** — sign a `serviceRequest` payload inside a TopicMessage. Verify deterministic output across implementations.

**Chain 6: Invoice Signing** — sign an `invoice` payload. Verify counterparty can reconstruct and verify.

**Chain 7: EscrowRef Round-Trip** — serialize/deserialize EscrowRef for both `hedera-native` and `evm-escrow` locator formats.

### 7.3 Multi-Language SDK Impact

Spec 008 adds:
- 5 new TopicMessage payload types
- 1 new agentURI service type with 2 settlement binding schemas
- 1 abstract escrow interface (6 operations)
- 2 concrete binding implementations
- 1 state machine (~10 states, ~12 transitions)
- ~10–15 new error codes

Each SDK implements: protocol layer (mandatory) + at least one binding (hedera-native for Hedera SDKs, evm-escrow for EVM SDKs, both for full-stack SDKs).

---

## 8. Risk Register

### R1: Two Bindings Doubles Implementation Work — MEDIUM

**Risk**: Shipping hedera-native AND evm-escrow in v1 doubles the settlement code.

**Mitigation**: The protocol layer (Sections A–C, E) is shared — ~70% of the spec. Each binding (Section D.1, D.2) is a thin mapping layer. The escrow interface abstraction means SDK tests are written once against the interface and run against both bindings. Follow the TopicAdapter precedent: Spec 004 shipped HCS + ERC-log adapters in v1.

### R2: EscrowRef Locator Format Fragility — LOW

**Risk**: Opaque locator strings (`"hedera-mainnet:0.0.12345"`, `"1:0xABC...:42"`) may be parsed incorrectly or collide across bindings.

**Mitigation**: Locator format is defined per binding with validation rules. The `binding` field disambiguates which parser to use. Format: `binding:chain-specific-parts`. SDK validates at construction time (valid-by-construction, following 002 FR-022).

### R3: EVM Contract Security — MEDIUM

**Risk**: `NeuronEscrow.sol` holds user funds. Bugs or exploits can cause loss.

**Mitigation**: v1 contract is deliberately minimal. Uses SafeERC20 for all token interactions (handles non-standard tokens like USDT). Pull payment pattern eliminates reentrancy risk during release authorization. `ReentrancyGuard` on `withdraw()`. Accepts only ERC-20 (no native ETH handling). EIP-712 domain separator prevents cross-chain/cross-contract replay. Formal audit recommended before mainnet deployment. The contract is INFORMATIVE in the spec (following 007 FR-C-37: "Solidity interfaces INFORMATIVE, NOT NORMATIVE").

### R4: Streaming Service Model + EVM Gas — MEDIUM

**Risk**: The deposit-invoice-deposit cycle on EVM costs ~$0.10–0.30 per cycle (vs ~$0.03–0.05 on Hedera). For hourly invoicing, this is $2.40–7.20/day per buyer-seller pair.

**Mitigation**: EVM deployments SHOULD use longer intervals for cost efficiency. The `pricing.interval` field is configurable. For high-frequency low-value services, the hedera-native binding is recommended. This is a deployment choice, not a protocol limitation.

### R5: Deposit Authorization Fragility — LOW

**Risk**: ERC-2612 `permit()` does not work with smart account wallets (requires `ecrecover`) and is not supported by all tokens (USDT, WBTC). Relying solely on permit would exclude a significant portion of the 2026 wallet ecosystem (200M+ smart accounts).

**Mitigation**: Three-tier deposit authorization: (1) Permit2 — works with all tokens and all wallets including smart accounts via ERC-1271; (2) ERC-2612 — optimization for compliant tokens + EOA; (3) standard `approve` + `transferFrom` — universal fallback. SDK attempts in priority order, first success wins.

### R6: Arbiter Trust — MEDIUM

Same as v2. Three options: 2-of-2 bilateral, 2-of-3 with known arbiter, or arbiter discovery via Validation Registry. All supported by both bindings.

### R7: No On-Chain Agreement State — LOW

Same as v2. Topic messages are the audit trail. Escrow balance is on-chain. Agreement state is reconstructable from topic message history.

### R8: Hedera Scheduled Transaction Expiry — LOW

Specific to hedera-native binding. Scheduled transactions expire after configurable timeout (default 30 min). If buyer doesn't counter-sign, seller re-issues invoice. Documented in binding section.

### R9: Contract Upgrade Path — LOW

**Risk**: `NeuronEscrow.sol` v1 is not upgradeable. If bugs are found, a new contract must be deployed and agents must update their `settlement.contract` in agentURI.

**Mitigation**: v1 intentionally avoids proxy patterns to reduce attack surface. Contract is minimal. For v2, EIP-1967 proxy pattern can be added (following 007 DD-05 precedent).

### R10: Permit2 External Dependency — LOW

**Risk**: Permit2 is a Uniswap Labs contract deployed at a canonical address across chains. The EVM binding depends on it for the primary gasless deposit path. If Permit2 is compromised or unavailable on a chain, the primary deposit path fails.

**Mitigation**: Permit2 is a fallback chain — if `depositWithPermit2` fails, the SDK falls back to ERC-2612 permit, then standard `approve` + `transferFrom`. The contract never depends solely on Permit2. Additionally, Permit2 is deployed at the same deterministic address on all major chains and has been in production since 2022 with billions in volume — the risk of compromise is low.

---

## 9. Next Moves

### 9.1 Next Step

**Begin `/speckit.specify` for Spec 008 — Payment.**

This recommendation provides:
- Protocol layer boundary (chain-agnostic)
- Two settlement binding specifications
- Service offering schema
- Negotiation message types
- State machine
- Abstract escrow interface

### 9.2 Clarification Questions

**Q1: Should both bindings ship in v1, or should one be primary?**
Constitution Principle VIII says HCS is "first." By analogy, should hedera-native be first with evm-escrow following? Or should both ship simultaneously given the blockchain-agnostic mandate?

**Q2: Should NeuronEscrow.sol be part of Spec 008 or a separate spec (like 007)?**
Spec 003 defines SDK registration; Spec 007 defines on-chain contracts. Should the EVM escrow contract follow the same split? Or is it simple enough to include in 008?

**Q3: Should the arbiter be protocol-level or app-level?**
Same question from v2. If protocol-level, both bindings must implement arbiter support. If app-level, the spec only defines 2-of-N threshold semantics.

**Q4: What is the minimum invoice interval?**
Same question from v2. Should there be a protocol-level minimum (like Health's MIN_DEADLINE_DELTA of 10 seconds)?

### 9.3 Workflow

```
1. Resolve Q1–Q4                          ← Decision meeting
2. /speckit.specify for 008 Payment      ← Spec creation
3. /speckit.clarify for 008              ← Resolve ambiguities
4. /speckit.plan for 008                 ← Implementation plan
5. /speckit.tasks for 008               ← Task list
6. /speckit.implement for 008           ← SDK + contract implementation
```

---

## 10. Dependency Graph

```
002 Key Library                          zero dependencies
 │
 ├──► 001 NeuronAccount                  depends on: 002
 │     │
 │     ├──► 004 Topic System             depends on: 001, 002
 │     │     │
 │     │     ├──► 003 Peer Registry      depends on: 001, 002, 004
 │     │     │
 │     │     └──► 005 Health             depends on: 001, 002, 004, 003
 │     │
 │     └──────────────────────────────────────────────────────┐
 │                                                            │
 ├──► 006 Protocol Determinism           cross-cutting        │
 │                                                            │
 └──► 007 Identity Contract              depends on: 001, 002, 003
       │                                                      │
       │  ┌───────────────────────────────────────────────────┘
       │  │
       ▼  ▼
      008 Payment
       ├── Protocol Layer          hard: 001, 003, 004, 006
       │                           soft: 005, 007
       ├── hedera-native binding   hard: 001 (SharedAccount), 002 (MultisigKey)
       └── evm-escrow binding      hard: ERC-20, ERC-2612 (external EIPs)
```

### Extended Dependency Matrix

```
         002  001  004  003  005  006  007  008
002       .    .    .    .    .    .    .    .
001       Y    .    .    .    .    .    .    .
004       Y    Y    .    .    .    .    .    .
003       Y    Y    Y    .    .    .    .    .
005       Y    Y    Y    Y    .    .    .    .
006       Y    Y    Y    .    Y    .    .    .
007       Y    Y    .    Y    .    .    .    .
008       Y    Y    Y    Y    s    Y    s    .

Y = hard dependency (protocol layer or at least one binding)
s = soft dependency (SHOULD, not MUST)
. = no dependency
```

---

*This document was produced after scanning the full Neuron spec corpus (001–007), constitution v1.4.0, legacy SDK, architecture notes, and a 2026 EVM best-practice review. It follows the chain-agnostic architectural pattern established by Spec 004 (TopicAdapter) and Spec 007 (FR-C-17). The original shared-account mechanism is preserved as the hedera-native binding. The EVM binding incorporates post-Pectra best practices: Permit2, pull payments, SafeERC20, EIP-712 domain separation, and smart-account compatibility. Future payment models (x402, streaming, ERC-8183) can be added as additional bindings without protocol-layer changes.*
