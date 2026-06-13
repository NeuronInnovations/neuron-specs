# Data Model: NeuronAccount Module

**Branch**: `001-neuron-account-module` | **Date**: 2026-02-25 | **Source**: spec.md FR-001..FR-026, Key Entities

---

## Entities

### NeuronAccount

The primary entity — represents an agent identity with type-specific attributes.

| Field | Type | Parent | Child | Shared | Source FR |
|-------|------|--------|-------|--------|----------|
| `accountType` | AccountType (enum: 1=Parent, 2=Child, 3=Shared) | MUST | MUST | MUST | FR-013 |
| `publicKey` | NeuronPublicKey (from 002) | MUST (single) | MUST (single) | MUST NOT | FR-001, FR-002, FR-007a |
| `evmAddress` | EVMAddress (derived from publicKey) | derived | derived | N/A | FR-008 |
| `peerID` | PeerID (derived from publicKey) | derived | derived | N/A | FR-008 |
| `did` | NeuronDID | MUST | MUST NOT | MUST NOT | FR-001, FR-012 |
| `parentPubKey` | NeuronPublicKey | MUST NOT | MUST | MUST NOT | FR-002, FR-007 |
| `multisigKey` | MultisigKey (from 002) | MUST NOT | MUST NOT | MUST | FR-007a, FR-021 |
| `currencySymbol` | string | MUST | MUST | MUST | FR-020 |
| `creditBalance` | BigInteger (optional) | MUST (capability) | N/A | N/A | FR-016 |
| `balanceAllocation` | BigInteger (optional) | N/A | MUST (capability) | N/A | FR-016 |
| `balance` | BigInteger (optional) | N/A | N/A | MUST (capability) | FR-021 |
| `ledgerAttachment` | LedgerAttachment (nilable) | MAY | MAY | MAY | FR-018 |
| `registryBinding` | RegistryBinding (nilable) | N/A | MUST | N/A | FR-022 |
| `feePayer` | LedgerAccountId (nilable) | N/A | MAY | N/A | FR-026 |
| `p2pHost` | PeerID (= peerID) | N/A | derived | N/A | FR-002 |

**Completeness** (FR-011a): An account is complete when all MUST fields for its type are present, plus ledger attachment.

### NeuronDID

| Field | Type | Description | Source FR |
|-------|------|-------------|----------|
| `identifier` | string | `did:key:zQ3s...` format | FR-012 |
| `document` | DIDDocument | DID document structure | FR-001 |

Generated from Parent's NeuronPublicKey via 002 FR-006a.

### LedgerAttachment

| Field | Type | Description | Source FR |
|-------|------|-------------|----------|
| `ledgerIdentifier` | string | e.g. `"ethereum-mainnet"`, `"hedera-mainnet"` | FR-018 |
| `attachedAddress` | EVMAddress | Address linking Neuron ↔ ledger | FR-018 |
| `state` | AttachmentState | `attached` or `detached` | FR-018 |
| `verificationStatus` | VerificationStatus | `verified`, `unverified`, `failed` | FR-019 |
| `lastSyncedAt` | Timestamp (optional) | When balance/state last synced | FR-016 |

### RegistryBinding

| Field | Type | Description | Source FR |
|-------|------|-------------|----------|
| `registryIdentifier` | string | Registry URI or identifier | FR-022 |
| `externalID` | string | Opaque ID from the registry | FR-022 |

### AccountType (Enum)

| Value | Name | Source FR |
|-------|------|----------|
| 1 | Parent | FR-013 |
| 2 | Child | FR-013 |
| 3 | Shared | FR-013 |

---

## Validation Rules

| Rule | Account Type | Check | Source FR |
|------|-------------|-------|----------|
| V-PARENT-01 | Parent | Has DID | FR-006 |
| V-PARENT-02 | Parent | Has single NeuronPublicKey | FR-006 |
| V-PARENT-03 | Parent | Has credit balance capability | FR-006 |
| V-PARENT-04 | Parent | No parent reference | FR-006 |
| V-PARENT-05 | Parent | No multisig key | FR-006 |
| V-CHILD-01 | Child | Has parent NeuronPublicKey ref | FR-007 |
| V-CHILD-02 | Child | Has single NeuronPublicKey | FR-007 |
| V-CHILD-03 | Child | Has registry binding | FR-007 |
| V-CHILD-04 | Child | No DID required | FR-007 |
| V-CHILD-05 | Child | No multisig key | FR-007 |
| V-SHARED-01 | Shared | Has MultisigKey with threshold | FR-007a |
| V-SHARED-02 | Shared | No DID | FR-007a |
| V-SHARED-03 | Shared | No parent reference | FR-007a |
| V-SHARED-04 | Shared | No single NeuronPublicKey | FR-007a |

---

## Relationships

```
NeuronAccount ──[has]──► NeuronPublicKey (from 002; Parent/Child only)
NeuronAccount ──[has]──► MultisigKey (from 002; Shared only)
NeuronAccount ──[derives]──► EVMAddress (from publicKey, via 002 FR-005)
NeuronAccount ──[derives]──► PeerID (from publicKey, via 002 FR-006)
NeuronAccount ──[has]──► NeuronDID (Parent only, via 002 FR-006a)
NeuronAccount ──[attached to]──► LedgerAttachment
NeuronAccount ──[bound to]──► RegistryBinding (Child only)
Child.parentPubKey ──[references]──► Parent.publicKey
Child.paymentAddress ──[resolves to]──► Parent.evmAddress (FR-024)
```
