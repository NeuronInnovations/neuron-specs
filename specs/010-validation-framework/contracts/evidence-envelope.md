# Contract: Evidence Envelope

## Payload Schema

The evidence envelope is a TopicMessage payload type (004 FR-T20). It follows the same canonical JSON serialization as all Neuron payloads (006 FR-W01–W10).

### Canonical JSON

```json
{
  "type": "validationEvidence",
  "version": "1.0.0",
  "validatorAgentId": "42",
  "subjectAgentId": "7",
  "specRef": "005-health",
  "verdict": "non-compliant",
  "evidenceHash": "0x1a2b3c...64hex",
  "evidenceURI": "ipfs://QmXyz..."
}
```

### With Optional Fields

```json
{
  "type": "validationEvidence",
  "version": "1.0.0",
  "validatorAgentId": "42",
  "subjectAgentId": "7",
  "specRef": "005-health",
  "verdict": "non-compliant",
  "evidenceHash": "0x1a2b3c...64hex",
  "evidenceURI": "ipfs://QmXyz...",
  "compositeRefs": [
    "0xaabb...64hex",
    "0xccdd...64hex"
  ],
  "observationWindow": {
    "end": 1708123496,
    "start": 1708123196
  }
}
```

### Field Ordering Rules

1. Mandatory fields in fixed order: `type` → `version` → `validatorAgentId` → `subjectAgentId` → `specRef` → `verdict` → `evidenceHash` → `evidenceURI`
2. Optional fields after mandatory, in alphabetical order: `compositeRefs` → `observationWindow`
3. `observationWindow` nested fields in alphabetical order: `end` → `start`
4. Absent optional fields MUST be omitted entirely (no `null` values per 006 FR-W04)

### Field Type Mapping

| Field | Wire Type | Encoding | Reference |
|-------|-----------|----------|-----------|
| `type` | string | JSON string | — |
| `version` | string | Semver string, e.g. `"1.0.0"` | — |
| `validatorAgentId` | UnsignedInt256 | Decimal string, e.g. `"42"` | 006 FR-W02 |
| `subjectAgentId` | UnsignedInt256 | Decimal string, e.g. `"7"` | 006 FR-W02 |
| `specRef` | string | Spec directory name or custom domain | FR-V24 |
| `verdict` | string | One of `"compliant"`, `"non-compliant"`, `"inconclusive"` | FR-V04 |
| `evidenceHash` | HexBytes | `0x`-prefixed lowercase hex, 66 chars | 006 FR-W06 |
| `evidenceURI` | string | URI (IPFS CID, HTTPS URL, etc.) | — |
| `compositeRefs` | HexBytes[] | Array of `0x`-prefixed lowercase hex | 006 FR-W06 |
| `observationWindow.end` | UnixTimestamp | Unsigned integer (nanoseconds) | 006 FR-W02a |
| `observationWindow.start` | UnixTimestamp | Unsigned integer (nanoseconds) | 006 FR-W02a |

### Hash Computations

**Envelope hash** (used in `responseHash` and `compositeRefs`):
```
envelopeHash = keccak256(canonicalJSON(envelope))
```
Where `canonicalJSON(envelope)` is the byte-exact JSON serialization following the field ordering and encoding rules above.

**Evidence hash** (the `evidenceHash` field value):
```
evidenceHash = keccak256(rawBytes(documentAtEvidenceURI))
```
Where `rawBytes(...)` is the raw byte content of the off-chain document. The document format is opaque to the protocol (FR-V18).

### Verdict → Registry Code Mapping

| Envelope `verdict` | `validationResponse()` `response` code |
|---------------------|----------------------------------------|
| `"compliant"` | `1` (PASS) |
| `"non-compliant"` | `2` (FAIL) |
| `"inconclusive"` | `3` (INCONCLUSIVE) |

### FR Traceability

- FR-V01: `type` discriminator
- FR-V02: Mandatory fields
- FR-V03: Field ordering
- FR-V04: Verdict values
- FR-V05: evidenceHash computation
- FR-V06: Optional fields (compositeRefs, observationWindow)
- FR-V07: Version compatibility
- FR-V09: responseHash = keccak256(canonicalJSON(envelope))
- FR-V23: Semantic types
