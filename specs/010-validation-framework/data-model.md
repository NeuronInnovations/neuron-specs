# Data Model: Validation Framework

## Entity Relationship

```
ValidatorService ──declares──> domains[]
       │
       └── registered in ──> IdentityRegistry (007)
                                    │
EvidenceEnvelope ──references──> subjectAgentId ──> IdentityRegistry
       │
       ├── validatorAgentId ──> IdentityRegistry
       ├── evidenceHash ──> off-chain document (opaque)
       ├── verdict ──> OracleVerdict
       ├── compositeRefs[] ──> other EvidenceEnvelope (by payload hash)
       │
       └── published to ──> validator's stdOut (TopicMessage)
                                    │
                                    └── responseHash ──> ValidationRegistry (007)
```

## EvidenceEnvelope

The primary data structure of the validation framework. A TopicMessage payload containing a validator's verdict about an agent's spec compliance.

| Field | Type | Req | Description |
|-------|------|-----|-------------|
| `type` | string | MUST | Always `"validationEvidence"` — discriminator |
| `version` | SemVer | MUST | Envelope version, e.g. `"1.0.0"` |
| `validatorAgentId` | UnsignedInt256 | MUST | Validator's on-chain agentId from Identity Registry |
| `subjectAgentId` | UnsignedInt256 | MUST | Agent being validated |
| `specRef` | string | MUST | Spec reference, e.g. `"005-health"` or custom domain |
| `verdict` | Verdict | MUST | One of `"compliant"`, `"non-compliant"`, `"inconclusive"` |
| `evidenceHash` | HexBytes | MUST | `keccak256(off-chain evidence document)` |
| `evidenceURI` | string | MUST | Pointer to full evidence (IPFS CID, HTTPS URL) |
| `compositeRefs` | HexBytes[] | MAY | `keccak256(canonicalJSON(envelope))` of related envelopes |
| `observationWindow` | ObservationWindow | MAY | Time range of observation |

**Serialization order** (FR-V03): `type` → `version` → `validatorAgentId` → `subjectAgentId` → `specRef` → `verdict` → `evidenceHash` → `evidenceURI` → optional fields in alphabetical order (`compositeRefs` → `observationWindow`).

**Validation rules**:
- `verdict` must be one of the three recognized values (FR-V04)
- `evidenceHash` must be 32 bytes (Keccak256 output)
- `validatorAgentId` and `subjectAgentId` must be non-zero
- Version compatibility per FR-V07 (accept major 1, reject major 2+)

**Hash computation**:
- **Envelope hash** (for `responseHash` and `compositeRefs`): `keccak256(canonicalJSON(envelope))` — hash of the envelope payload itself
- **Evidence hash** (for `evidenceHash` field): `keccak256(off-chain document at evidenceURI)` — hash of the external evidence

## ObservationWindow

| Field | Type | Req | Description |
|-------|------|-----|-------------|
| `end` | UnixTimestamp | MUST | End of observation period |
| `start` | UnixTimestamp | MUST | Start of observation period |

**Serialization order**: `end` → `start` (alphabetical per 006 rules).

## OracleVerdict

Enumeration mapping between envelope string verdicts and Validation Registry integer codes.

| Verdict String | Registry Code | Semantics |
|----------------|---------------|-----------|
| `"compliant"` | `1` (PASS) | Evidence supports spec adherence |
| `"non-compliant"` | `2` (FAIL) | Evidence indicates spec violation |
| `"inconclusive"` | `3` (INCONCLUSIVE) | Insufficient evidence to determine |
| — | `0` (PENDING) | Initial state (no verdict yet) |

## ValidatorService

A `neuron-validator` service object in a validator agent's agentURI. Follows the same service pattern as `neuron-topic`, `neuron-p2p-exchange`, and `neuron-commerce`.

| Field | Type | Req | Description |
|-------|------|-----|-------------|
| `type` | string | MUST | Always `"neuron-validator"` |
| `name` | string | MUST | Service name, e.g. `"validation"` |
| `version` | SemVer | MUST | Service version, e.g. `"1.0.0"` |
| `domains` | string[] | MUST | Spec refs or domain tags covered, e.g. `["005-health"]` |
| `verdictDelivery` | string | MUST | How verdicts are published. Value: `"topic"` |

## CompositeValidation

A cross-spec validation that aggregates individual evidence envelopes.

**Structure**: The composite envelope is itself an EvidenceEnvelope with the `compositeRefs` field populated. Each entry in `compositeRefs` is `keccak256(canonicalJSON(individualEnvelope))`.

**Lifecycle**:
1. Validator produces individual envelopes (one per spec-level check)
2. Each individual envelope is published to stdOut independently
3. Validator computes `keccak256(canonicalJSON(envelope))` for each
4. Composite envelope's `compositeRefs` array contains these hashes
5. Composite envelope is published to stdOut

## Evidence Lifecycle State Flow

```
Observation
    │
    ▼
EvidenceEnvelope (constructed)
    │
    ├── Publish to validator's stdOut (autonomous, FR-V16)
    │       │
    │       └── consensus timestamp + signature (TopicMessage)
    │
    └── [Optional] On-chain anchoring (request-initiated)
            │
            ├── Requires prior validationRequest() by agent owner
            │
            └── validationResponse(requestHash, code, responseURI, responseHash, tag)
                    │
                    └── responseHash = keccak256(canonicalJSON(envelope))
```
