# API Contract: Account Builder

**Source**: spec.md FR-001, FR-002, FR-006..FR-026

---

## NewParentAccountBuilder

Creates a builder for Parent accounts.

**Output**: `ParentAccountBuilder`

**Chain methods**:
- `.WithPublicKey(NeuronPublicKey)` — MUST (FR-006)
- `.WithDID(NeuronDID)` — MUST (FR-006, FR-012)
- `.WithCurrency(string)` — MUST (FR-020)
- `.WithLedgerAttachment(LedgerAttachment)` — MAY (FR-018)
- `.Build()` — Returns NeuronAccount. Raises Error if validation of Parent rules fails (FR-006)
- `.BuildComplete()` — Returns NeuronAccount. Raises Error if completeness validation fails (FR-011a)

---

## NewChildAccountBuilder

Creates a builder for Child accounts.

**Output**: `ChildAccountBuilder`

**Chain methods**:
- `.WithPublicKey(NeuronPublicKey)` — MUST (FR-007)
- `.WithParentPublicKey(NeuronPublicKey)` — MUST (FR-007)
- `.WithCurrency(string)` — MUST (FR-020)
- `.WithRegistryBinding(RegistryBinding)` — MUST for completeness (FR-022)
- `.WithFeePayer(LedgerAccountId)` — MAY (FR-026)
- `.WithLedgerAttachment(LedgerAttachment)` — MAY (FR-018)
- `.Build()` — Returns NeuronAccount. Raises Error if validation of Child rules fails (FR-007)
- `.BuildComplete()` — Returns NeuronAccount. Raises Error if completeness validation fails (FR-011a)

---

## NewSharedAccountBuilder

Creates a builder for Shared accounts.

**Output**: `SharedAccountBuilder`

**Chain methods**:
- `.WithMultisigKey(MultisigKey)` — MUST (FR-007a)
- `.WithCurrency(string)` — MUST (FR-020)
- `.WithLedgerAttachment(LedgerAttachment)` — MAY (FR-018)
- `.Build()` — Returns NeuronAccount. Raises Error if validation of Shared rules fails (FR-007a)
- `.BuildComplete()` — Returns NeuronAccount. Raises Error if completeness validation fails (FR-011a)

---

## NeuronAccount Methods

### PaymentAddress

Resolves the payment address for use by peer registries.

**Output**: Returns EVMAddress. Raises Error if resolution fails.

**Behavior** (FR-023, FR-024):
- Parent: returns own `evmAddress` (attached address)
- Child: resolves Parent's `evmAddress` from `parentPubKey`
- Shared: returns error (no payment address defined)

### Validate

Validates account structure against type-specific rules.

**Output**: Array of ValidationError

**Behavior** (FR-006, FR-007, FR-007a, FR-014): Checks all rules for the account type. Returns empty slice if valid.

### VerifyParentChild

Verifies the parent-child relationship against ledger/registry data.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `child` | NeuronAccount | Child account |
| `verifier` | ParentChildVerifier | Injected ledger/registry verifier |

**Output**: Returns VerificationResult. Raises Error if verification cannot be performed. Result is verified / unverified / failed (FR-017)

### VerifyLedgerAttachment

Verifies the account's ledger attachment.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `verifier` | LedgerVerifier | Injected ledger verifier |

**Output**: Returns VerificationResult. Raises Error if verification cannot be performed. Checks existence, ownership, semantics match (FR-019)

### Serialize / Deserialize

JSON serialization/deserialization.

**Behavior** (FR-015): Canonical JSON format. Public keys as compressed hex (002 FR-010).
