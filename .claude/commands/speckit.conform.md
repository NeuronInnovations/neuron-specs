---
description: Verify cross-language conformance against 006 protocol determinism test vectors, wire format, and error taxonomy.
handoffs:
  - label: Propagate Changes
    agent: speckit.propagate
    prompt: Propagate conformance failures as spec changes
    send: true
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Goal

Verify that one or more SDK implementations produce byte-identical results for the 006 golden test vector chains, conform to the wire format specification, and implement the error taxonomy correctly. When multiple language implementations are detected, compare outputs cross-language.

This command is **STRICTLY READ-ONLY for spec artifacts**. It may run tests but does not modify any source files. It produces a structured conformance report.

## Operating Constraints

**Constitution Authority**: The project constitution (`.specify/memory/constitution.md`) is **non-negotiable**. Constitution Principle X (Deterministic Signing) is the primary enforcement target of this command.

**006 as Universal Oracle**: All conformance checks are evaluated against the normative artifacts in `specs/006-protocol-determinism/contracts/`. These are the single source of truth for cross-language consistency.

## Execution Steps

### 1. Initialize

Run `.specify/scripts/bash/check-prerequisites.sh --json --paths-only` from repo root and parse `REPO_ROOT`.
For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

### 2. Load Protocol Determinism Contracts

Read the following files from `specs/006-protocol-determinism/contracts/`:

- **REQUIRED**: `test-vectors.md` — 4 golden test vector chains with hex intermediate values
- **REQUIRED**: `algorithm-reference.md` — 14 byte-level algorithm specifications (FR-A01..A14)
- **REQUIRED**: `wire-format.md` — 9 normative JSON encoding rules (FR-W01..W10)
- **REQUIRED**: `error-taxonomy.md` — unified error code namespace (NEURON-{DOMAIN}-{NNN})

Also load:
- **IF EXISTS**: `specs/006-protocol-determinism/data-model.md` — Primitive Type Encoding Table

Abort with error if any REQUIRED file is missing.

### 3. Detect Available Language Implementations

Scan `REPO_ROOT` for implementation directories. Check these paths in order:

| Language | Detection Paths | Indicator File |
|----------|----------------|----------------|
| Go | `impl/internal/` | `*.go` files present |
| Rust | `impl-rust/`, `sdk-rust/` | `Cargo.toml` present |
| TypeScript | `impl-ts/`, `sdk-ts/` | `package.json` present |
| Python | `impl-py/`, `sdk-py/` | `pyproject.toml` or `setup.py` present |

Build a list of detected languages with their root paths.

If zero implementations found, abort: "No SDK implementations detected. Expected at least impl/internal/ (Go)."

Report detected languages before proceeding.

### 4. Per-Language Verification

For each detected implementation, run these checks in order:

#### 4a. Test Vector Chain 1: Key Derivation

Verify the full key derivation chain from `test-vectors.md` Chain 1:
- Private key (`k=1`) → compressed public key → uncompressed public key
- Uncompressed key → Keccak256 → last 20 bytes → EIP-55 checksum → EVM address
- Compressed key → protobuf encoding → identity multihash → base58btc → PeerID
- Compressed key → multicodec `0xe701` prefix → base58btc → `did:key:z...` → DID:key

**Verification method**: Run the language's test suite targeting key derivation tests, OR manually trace through the implementation comparing against each intermediate hex value in `test-vectors.md`.

Compare **every** intermediate value, not just final outputs. Any divergence is a FAIL.

#### 4b. Test Vector Chain 2: TopicMessage Signing

Verify from `test-vectors.md` Chain 2:
- Pre-image construction: `timestamp_bytes || sequence_bytes || payload_bytes`
- Keccak256 hash of pre-image
- RFC 6979 ECDSA signature (R || S || V, 65 bytes)
- Canonical JSON serialization (field order: senderAddress → signature → timestamp → sequenceNumber → payload)

#### 4c. Test Vector Chain 3: HeartbeatPayload Signing

Verify from `test-vectors.md` Chain 3:
- HeartbeatPayload canonical JSON (mandatory fields in order, optional fields omitted if absent)
- Payload bytes → TopicMessage pre-image → sign → canonical JSON envelope

#### 4d. Test Vector Chain 4: Key Encryption Round-Trip

Verify from `test-vectors.md` Chain 4:
- Argon2id key derivation with known parameters (time=1, memory=65536, threads=4)
- AES-256-GCM encryption
- Decrypt → verify original key bytes recovered

#### 4e. Wire Format Compliance

For each JSON-serializable type in the implementation, verify against `wire-format.md`:

| Rule | Check |
|------|-------|
| FR-W01 | Compact JSON (no whitespace between tokens) |
| FR-W02 | UnsignedInt64 encoded as JSON strings (decimal, no leading zeros) |
| FR-W03 | ByteArray encoded as base64 RFC 4648 standard alphabet with `=` padding |
| FR-W04 | Optional fields omitted when absent (never `null`) |
| FR-W06 | EVMAddress as EIP-55 checksummed hex with `0x` prefix |
| FR-W08 | All string-to-bytes conversions use UTF-8 |

**For TypeScript/JavaScript**: Verify explicit UTF-8 encoding before hashing (wire-format.md Section 9).

#### 4f. Error Taxonomy Compliance

For each error domain relevant to the detected packages, verify:
- Error codes match `error-taxonomy.md` exactly (e.g., `NEURON-KEY-001` through `NEURON-KEY-014`)
- Error names match (e.g., `InvalidFormat`, `UnsupportedKeyType`)
- Error structure includes `code`, `name`, `message` fields

Report missing error codes and extra (non-spec) error codes separately.

### 5. Cross-Language Comparison

**Only if 2+ language implementations are detected.**

For each test vector chain, compare the outputs across all detected languages:
- Byte-identical signatures for the same input
- Byte-identical JSON serialization for the same logical object
- Same error codes for the same invalid inputs

Any divergence between languages is a CRITICAL finding.

### 6. Produce Conformance Report

Output a structured Markdown report (do not write to file unless user requests):

```markdown
## Conformance Report

**Date**: [today]
**Languages tested**: [list]
**Test vector source**: specs/006-protocol-determinism/contracts/test-vectors.md

### Per-Language Results

| Language | Chain 1 | Chain 2 | Chain 3 | Chain 4 | Wire Format | Error Taxonomy | Overall |
|----------|---------|---------|---------|---------|-------------|----------------|---------|
| Go       | PASS    | PASS    | PASS    | PASS    | PASS        | PASS           | PASS    |

### Cross-Language Comparison

| Check | Languages Compared | Result | Detail |
|-------|--------------------|--------|--------|
| Chain 1 key derivation | Go, Rust | PASS | Byte-identical |

### Failures (if any)

| # | Language | Chain/Check | Step | Expected | Actual |
|---|----------|-------------|------|----------|--------|
| 1 | Rust | Chain 2 | Keccak256 hash | 0xABC... | 0xDEF... |

### Metrics

- Total checks: [N]
- Passed: [N]
- Failed: [N]
- Languages: [N]
- Cross-language comparisons: [N]
```

### 7. Next Actions

- If all checks PASS: "All conformance checks passed. No action needed."
- If failures exist for a single language: "Implementation diverges from spec 006. Fix the implementation to match algorithm-reference.md."
- If cross-language divergence: "Cross-language inconsistency detected. Use /speckit.propagate to trace the root cause."
- Offer handoff to `/speckit.propagate` if changes to specs are needed.

## Relation to Pipeline Checkpoints

This command evaluates:
- **CP-10** (Cross-Language Conformance) from the pipeline checkpoint system
- Partially validates **CP-8** (Green Phase) by running test vector tests

Run `.specify/scripts/bash/validate-pipeline.sh` for the full checkpoint evaluation.
