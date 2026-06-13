# Data Model: Payment (Spec 008)

**Source**: spec.md FR-P01–P35, Key Entities section

---

## NeuronCommerceService

EIP-8004 service object in agentURI `services[]`. Represents a commercial service offering.

| Field | Type | Required | Canonical Order | Validation | Source FR |
|-------|------|----------|----------------|------------|----------|
| `type` | string | MUST | 1 | Exact match: `"neuron-commerce"` | FR-P01 |
| `name` | string | MUST | 2 | Non-empty; human-readable service name | FR-P01 |
| `version` | string | MUST | 3 | Semver format (e.g., `"1.0.0"`) | FR-P01 |
| `delivery` | DeliveryDescriptor | MUST | 4 | Valid mode + mode-specific refs | FR-P01a |
| `settlement` | SettlementDescriptor | MUST | 5 | Valid binding | FR-P02 |
| `pricing` | PricingDescriptor | MUST | 6 | All sub-fields non-empty | FR-P03 |
| `termsRef` | string | SHOULD | 7 | URI; omit if absent (FR-W04) | FR-P04 |

**Multiplicity** (FR-P05): 0–N entries per agentURI. Different entries MAY vary by settlement binding or delivery mode.

---

## DeliveryDescriptor

Describes how service data is delivered. Nested within NeuronCommerceService.

| Field | Type | Required | Canonical Order | Validation | Source FR |
|-------|------|----------|----------------|------------|----------|
| `mode` | DeliveryMode | MUST | 1 | One of: `"p2p"`, `"topic"`, `"custom:<type>"` | FR-P01a |
| `serviceRef` | string | Conditional | 2 | MUST when mode=`"p2p"`; cross-refs `neuron-p2p-exchange` name | FR-P01a, FR-P01b |
| `channelRef` | string | Conditional | 3 | MUST when mode=`"topic"`; cross-refs `neuron-topic` name | FR-P01a, FR-P01b |

**DeliveryMode enum**: `"p2p"` | `"topic"` | `"custom:<type>"`

**Cross-reference validation** (FR-P01b): `serviceRef` MUST match an existing `neuron-p2p-exchange` service `name` in the same agentURI. `channelRef` MUST match an existing `neuron-topic` service `name`. Invalid references produce `InvalidDeliveryRef` error. Same pattern as 004 FR-T18 (`topicRef`).

---

## SettlementDescriptor

Identifies the payment settlement binding.

| Field | Type | Required | Canonical Order | Validation | Source FR |
|-------|------|----------|----------------|------------|----------|
| `binding` | string | MUST | 1 | One of: `"hedera-native"`, `"evm-escrow"`, or future values | FR-P02 |
| (binding-specific) | map[string]any | Varies | Alphabetical | Defined per binding (e.g., `chainId`, `contract`, `token`, `network`) | FR-P02 |

---

## PricingDescriptor

Declares pricing terms for the service.

| Field | Type | Required | Canonical Order | Validation | Source FR |
|-------|------|----------|----------------|------------|----------|
| `amount` | string | MUST | 1 | Decimal string (e.g., `"10"`) per FR-W02/W07 | FR-P03 |
| `currency` | string | MUST | 2 | Currency symbol (e.g., `"USDC"`, `"HBAR"`) | FR-P03 |
| `unit` | string | MUST | 3 | Denomination (e.g., `"token"`, `"tinybar"`) | FR-P03 |
| `interval` | string | MUST | 4 | Billing interval in seconds; `"0"` = one-time | FR-P03 |

---

## Negotiation Payloads

Six TopicMessage payload types (FR-P06), discriminated by `type` field. All follow canonical JSON (006 FR-W01–W10).

### serviceRequest (buyer → seller's stdIn)

| Field | Type | Required | Canonical Order | Source FR |
|-------|------|----------|----------------|----------|
| `type` | string | MUST | 1 | FR-P07 |
| `version` | string | MUST | 2 | FR-P07 |
| `requestId` | UUID | MUST | 3 | FR-P07 |
| `serviceRef` | string | MUST | 4 | FR-P07 |
| `settlementBinding` | string | MUST | 5 | FR-P07 |
| `proposedAmount` | string | MUST | 6 | FR-P07 |
| `proposedCurrency` | string | MUST | 7 | FR-P07 |
| `proposedInterval` | string | MUST | 8 | FR-P07 |
| `serviceParams` | object | OPTIONAL | 9 | FR-P07 (keys lexicographic) |
| `negotiationDeadline` | UnsignedInt64 | MUST | 10 | FR-P07 |
| `arbiter` | string | OPTIONAL | 11 | FR-P07 |
| `buyerStdIn` | string | MUST | 12 | FR-P07 |

### serviceResponse (seller → buyer's stdIn)

| Field | Type | Required | Canonical Order | Source FR |
|-------|------|----------|----------------|----------|
| `type` | string | MUST | 1 | FR-P08 |
| `version` | string | MUST | 2 | FR-P08 |
| `requestId` | UUID | MUST | 3 | FR-P08 |
| `action` | string | MUST | 4 | FR-P08 (`accept`, `reject`, `counter`) |
| `counterAmount` | string | Conditional | 5 | FR-P08 (when action=`counter`) |
| `counterInterval` | string | Conditional | 6 | FR-P08 (when action=`counter`) |

### connectionSetup (bidirectional)

| Field | Type | Required | Canonical Order | Source FR |
|-------|------|----------|----------------|----------|
| `type` | string | MUST | 1 | FR-P33 |
| `version` | string | MUST | 2 | FR-P33 |
| `requestId` | UUID | MUST | 3 | FR-P33 |
| `peerID` | PeerID | MUST | 4 | FR-P33 (per 002 FR-006) |
| `encryptedMultiaddrs` | Base64String | MUST | 5 | FR-P33, FR-P34 |
| `protocol` | string | MUST | 6 | FR-P33 |
| `natStatus` | string | OPTIONAL | 7 | FR-P33 (`"public"`, `"private"`, `"unknown"`) |

### escrowCreated (buyer → seller's stdIn)

| Field | Type | Required | Canonical Order | Source FR |
|-------|------|----------|----------------|----------|
| `type` | string | MUST | 1 | FR-P09 |
| `version` | string | MUST | 2 | FR-P09 |
| `requestId` | UUID | MUST | 3 | FR-P09 |
| `escrowRef` | string | MUST | 4 | FR-P09 |
| `depositAmount` | string | MUST | 5 | FR-P09 |
| `depositCurrency` | string | MUST | 6 | FR-P09 |

### invoice (seller → buyer's stdIn)

| Field | Type | Required | Canonical Order | Source FR |
|-------|------|----------|----------------|----------|
| `type` | string | MUST | 1 | FR-P10 |
| `version` | string | MUST | 2 | FR-P10 |
| `requestId` | UUID | MUST | 3 | FR-P10 |
| `releaseRequestRef` | string | MUST | 4 | FR-P10 |
| `escrowRef` | string | MUST | 5 | FR-P10 |
| `amount` | string | MUST | 6 | FR-P10 |
| `currency` | string | MUST | 7 | FR-P10 |
| `period` | string | MUST | 8 | FR-P10 (ISO 8601 interval) |

### invoiceAck (buyer → seller's stdIn)

| Field | Type | Required | Canonical Order | Source FR |
|-------|------|----------|----------------|----------|
| `type` | string | MUST | 1 | FR-P11 |
| `version` | string | MUST | 2 | FR-P11 |
| `requestId` | UUID | MUST | 3 | FR-P11 |
| `releaseRequestRef` | string | MUST | 4 | FR-P11 |
| `action` | string | MUST | 5 | FR-P11 (`approved`, `refused`) |
| `depositedMore` | bool | OPTIONAL | 6 | FR-P11 |
| `newBalance` | string | OPTIONAL | 7 | FR-P11 |

---

## AgreementState

Lifecycle state for a buyer-seller agreement. Tracked per `requestId` (FR-P13a).

**States**: IDLE, REQUESTED, NEGOTIATING, AGREED, FUNDED, ACTIVE, INVOICED, COMPLETED, TERMINATED, REJECTED

**Transitions** (FR-P14):

| From | To | Trigger |
|------|----|---------|
| IDLE | REQUESTED | buyer sends serviceRequest |
| REQUESTED | NEGOTIATING | seller sends serviceResponse (counter) |
| REQUESTED | AGREED | seller sends serviceResponse (accept) |
| REQUESTED | REJECTED | seller sends serviceResponse (reject) or negotiationDeadline elapses |
| NEGOTIATING | AGREED | either party sends acceptance |
| NEGOTIATING | REJECTED | either party withdraws or negotiationDeadline elapses |
| AGREED | FUNDED | buyer sends escrowCreated |
| FUNDED | ACTIVE | seller begins delivery |
| ACTIVE | INVOICED | seller sends invoice |
| INVOICED | ACTIVE | buyer sends invoiceAck (approved) |
| INVOICED | TERMINATED | buyer sends invoiceAck (refused) or timeout |
| ACTIVE | TERMINATED | either party terminates |
| ACTIVE | COMPLETED | mutual graceful shutdown |

**Note** (FR-P14a): connectionSetup exchange occurs within AGREED state and does NOT trigger a state transition.

---

## EscrowAdapter

Abstract interface for settlement bindings (FR-P16–P23).

| Operation | Signature | Returns | Source FR |
|-----------|-----------|---------|----------|
| `createEscrow` | (buyer, seller, arbiter?, currency, threshold, agreementHash, timeout) | EscrowRef | FR-P17 |
| `deposit` | (escrowRef, amount) | DepositResult | FR-P18 |
| `getBalance` | (escrowRef) | Balance | FR-P19 |
| `requestRelease` | (escrowRef, amount, recipient, evidenceHash) | ReleaseRequestRef | FR-P20 |
| `approveRelease` | (escrowRef, releaseRequestRef) | ReleaseResult | FR-P21 |
| `claimRefund` | (escrowRef) | Result | FR-P22 |

**EscrowRef**: Opaque reference with `binding` and `locator` fields (FR-P23).
**ReleaseRequestRef**: Same structure as EscrowRef (FR-P23).

---

## Error Taxonomy

Domain: `NEURON-PAYMENT-*` (FR-P32)

| Error Kind | Trigger | Source FR |
|------------|---------|----------|
| InvalidServiceOffering | neuron-commerce missing MUST fields | FR-P01 |
| NegotiationFailed | serviceResponse: reject | FR-P08 |
| NegotiationExpired | negotiationDeadline elapsed | FR-P07a |
| VersionMismatch | Payload version major >= 2 | FR-P12a |
| EscrowCreationFailed | createEscrow failed | FR-P17 |
| InsufficientBalance | requestRelease amount > available | FR-P25 |
| InvoiceValidationFailed | Invoice doesn't match agreed terms | FR-P10 |
| ReleaseNotAuthorized | Release approval rejected | FR-P21 |
| RefundNotEligible | claimRefund before timeout | FR-P22 |
| BindingUnavailable | Settlement binding not found | FR-P02 |
| TimeoutNotElapsed | Refund attempted pre-timeout | FR-P22 |
| InvalidEscrowRef | Escrow reference invalid | FR-P23 |
| UnsupportedDeliveryMode | delivery.mode not recognized | FR-P01a |
| InvalidDeliveryRef | delivery cross-reference broken | FR-P01b |
| ConnectionSetupRequired | P2P mode but no connectionSetup | FR-P35 |
| ConnectionSetupEncryptionFailed | Encryption/decryption failed | FR-P34 |
