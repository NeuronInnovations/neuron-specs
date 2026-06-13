# API Contract: EscrowAdapter Interface

**Source**: spec.md FR-P16–P23, FR-P24–P27a

---

## Abstract Interface

All settlement bindings MUST implement these six operations (FR-P16).

### createEscrow

```
createEscrow(
  buyer:         EVMAddress,
  seller:        EVMAddress,
  arbiter:       EVMAddress | nil,    // OPTIONAL
  currency:      string,
  threshold:     UnsignedInt64,
  agreementHash: Bytes32,             // keccak256(canonicalJSON(acceptedServiceResponse))
  timeout:       UnsignedInt64         // Unix nanoseconds — refund eligibility timestamp (per 006 FR-W02a)
) → (EscrowRef, error)
```

**FR-P17**: Returns opaque EscrowRef with `binding` + `locator` fields.

### deposit

```
deposit(
  escrowRef: EscrowRef,
  amount:    string                    // Decimal string
) → (DepositResult, error)
```

**FR-P18**: Returns `transactionRef` and `newBalance`. Authorization mechanism is binding-specific.

### getBalance

```
getBalance(
  escrowRef: EscrowRef
) → (Balance, error)
```

**FR-P19**: Returns `available` (string decimal), `currency`, `lastSynced` (timestamp).

### requestRelease

```
requestRelease(
  escrowRef:    EscrowRef,
  amount:       string,               // Decimal string
  recipient:    EVMAddress,           // Default: seller's paymentAddress (FR-P28)
  evidenceHash: Bytes32               // keccak256(canonicalJSON(deliveryProofTopicMessage))
) → (ReleaseRequestRef, error)
```

**FR-P20**: Records pending release. **FR-P25a**: MUST fail if amount > available balance.

### approveRelease

```
approveRelease(
  escrowRef:        EscrowRef,
  releaseRequestRef: ReleaseRequestRef
) → (ReleaseResult, error)
```

**FR-P21**: Authorizes fund transfer. Push (Hedera) or pull (EVM withdraw) is binding-internal.

### claimRefund

```
claimRefund(
  escrowRef: EscrowRef
) → (Result, error)
```

**FR-P22**: Returns funds to buyer. **FR-P25b**: MUST fail if timeout has not elapsed.

---

## Value Types

### EscrowRef

```
EscrowRef {
  binding: string    // e.g., "hedera-native", "evm-escrow"
  locator: string    // binding-specific (e.g., account ID, contract address)
}
```

### ReleaseRequestRef

Same structure as EscrowRef (FR-P23).

### Balance

```
Balance {
  available:  string    // Decimal string
  currency:   string
  lastSynced: UnsignedInt64  // Unix nanoseconds (per 006 FR-W02a)
}
```

### DepositResult

```
DepositResult {
  transactionRef: string
  newBalance:     string    // Decimal string
}
```

### ReleaseResult

```
ReleaseResult {
  transactionRef: string
  released:       string    // Decimal string
  recipient:      EVMAddress
}
```

---

## Settlement Preconditions (FR-P24–P25)

These invariants are enforced by the EscrowAdapter, independent of service terms:

1. `requestRelease(amount)` MUST fail if `amount > getBalance().available`
2. `claimRefund()` MUST fail if `timeout` has not elapsed

Service-level funding compliance (minimum deposits, top-up triggers) is NOT enforced by the adapter — it's determined by the agreed terms (FR-P26).
