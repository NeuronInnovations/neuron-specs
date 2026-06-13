---
description: Analyze the impact of a spec change across the dependency graph and identify all affected artifacts and implementations.
handoffs:
  - label: Regenerate Tasks
    agent: speckit.tasks
    prompt: Regenerate the task list to reflect the propagated changes
    send: true
  - label: Verify Conformance
    agent: speckit.conform
    prompt: Run conformance verification after propagation
    send: true
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Goal

Given a spec change (described in `$ARGUMENTS` or detected from git diff), classify the change severity, trace its downstream impact through the dependency graph, and produce a structured impact report listing every affected artifact and implementation file.

This command is **STRICTLY READ-ONLY**. It does not modify any files. It produces an impact analysis report and offers handoffs for remediation.

## Operating Constraints

**Dependency Graph is Authoritative**: The build order defined in `CLAUDE.md` is the canonical dependency graph. All impact tracing follows this graph strictly.

**006 is Cross-Cutting**: Changes to `specs/006-protocol-determinism/` affect ALL specs (001–005, 007). These are always classified as high-impact.

## Execution Steps

### 1. Initialize

Run `.specify/scripts/bash/check-prerequisites.sh --json --paths-only` from repo root and parse `REPO_ROOT`.
For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

### 2. Determine the Change

**If `$ARGUMENTS` is not empty**: Parse the change description. The user may provide:
- A spec number and description: `"002: renamed EVMAddress to EthereumAddress"`
- A file path: `"specs/004-topic-system/data-model.md"`
- A general description: `"added new error code NEURON-KEY-015"`

Extract the source spec number and the nature of the change.

**If `$ARGUMENTS` is empty or says "auto-detect"**:
- Run `git diff HEAD~1 --name-only -- specs/` to detect changed spec files
- Run `git diff HEAD~1 -- specs/` to see the actual changes
- If no spec changes detected, abort: "No spec changes detected. Provide a change description via arguments or ensure recent commits modify specs/."

### 3. Classify the Change

Evaluate each change against RFC 2119 severity levels:

| Classification | Criteria | Impact Level | Action Required |
|---------------|----------|--------------|-----------------|
| **BREAKING** | Modifies a MUST or MUST NOT requirement, renames a type, changes a wire format rule, alters an algorithm step | All downstream implementations must update | Mandatory |
| **SIGNIFICANT** | Modifies a SHOULD requirement, adds a new validation rule, changes error semantics | Implementations should update | Recommended |
| **ADDITIVE** | Adds new FR-\*/SC-\* without modifying existing ones, adds new entity fields (optional), adds new error codes | Implementations may update | Optional |
| **INFORMATIONAL** | Wording clarifications, documentation fixes, corrected examples | No implementation change needed | None |

Report the classification with justification.

### 4. Load the Dependency Graph

The canonical dependency chain (from `CLAUDE.md`):

```
002 Key Library       <- Foundation: zero dependencies
 |
001 NeuronAccount     <- Identity built on keys
 |
004 Topic System      <- Communication substrate
 |
003 Peer Registry     <- Registration requires topic schemas
 |
005 Health            <- Heartbeats flow through topics
```

Additional relationships:
- **006 Protocol Determinism** → cross-cutting (affects all specs)
- **007 Identity Contract** → branches from 003

Spec-to-Go-package mapping:
- 002 → `impl/internal/keylib/`
- 001 → `impl/internal/account/`
- 004 → `impl/internal/topic/`
- 003 → `impl/internal/registry/`
- 005 → `impl/internal/health/`

Determine the source spec of the change. Identify all downstream specs from the dependency graph.

**Special case — 006 changes**: If the change is in `specs/006-protocol-determinism/`, ALL specs (001–005, 007) are downstream.

### 5. Trace Downstream Impact

For each downstream spec (in dependency order):

#### 5a. Spec Artifact Impact

Check whether the change affects these artifacts in the downstream spec:

| Artifact | Check Method |
|----------|-------------|
| `data-model.md` | Grep for changed type names, field names, or upstream spec references |
| `contracts/*.md` | Grep for changed function signatures, parameter types, algorithm references |
| `spec.md` | Grep for changed FR-\* identifiers, cross-spec references |
| `plan.md` | Check if architecture assumptions still hold |
| `tasks.md` | Grep for task descriptions referencing changed types/operations |

For each match, note:
- The file and approximate location
- What specifically references the changed element
- Whether the reference is direct (uses the changed name) or indirect (uses a derived concept)

#### 5b. Implementation Impact

For each affected spec's Go package, check:

| File Pattern | Check Method |
|-------------|-------------|
| `*.go` (non-test) | Grep for changed type names, function names, constant values |
| `*_test.go` | Grep for test assertions referencing changed values |
| `doc.go` | Check if package documentation references changed concepts |
| `errors.go` | Check if error codes or names are affected |
| `constants.go` | Check if constant values changed |

Report specific file paths and line ranges where changes are needed.

#### 5c. Protocol Determinism Impact

If the change affects any of these, flag as cross-cutting:
- **Wire format** (field ordering, type encoding) → affects all JSON serialization
- **Algorithm** (crypto operations, derivation steps) → affects signing, verification
- **Test vectors** (golden values) → affects all conformance tests
- **Error taxonomy** (error codes, names) → affects error handling in all packages

### 6. Produce Impact Report

Output a structured Markdown report:

```markdown
## Change Impact Report

**Date**: [today]
**Change source**: [spec number and file]
**Classification**: [BREAKING | SIGNIFICANT | ADDITIVE | INFORMATIONAL]
**Change summary**: [1-2 sentences]

### Dependency Path

[Show the path from source spec through all affected downstream specs]

### Affected Spec Artifacts

| Spec | Artifact | Impact | Description |
|------|----------|--------|-------------|
| 004 | data-model.md | Direct | Type 'X' renamed to 'Y' — update reference |
| 005 | contracts/health-publisher.md | Indirect | Uses TopicMessage which changed |

### Affected Implementation Files

| Package | File | Impact | Description |
|---------|------|--------|-------------|
| topic | message.go | Direct | Field name change in struct |
| topic | message_test.go | Direct | Test assertions reference old name |
| health | publisher.go | Indirect | Calls topic.NewTopicMessage() |

### Protocol Determinism Impact

| Artifact | Affected? | Detail |
|----------|-----------|--------|
| test-vectors.md | [Yes/No] | [If yes: which chains] |
| wire-format.md | [Yes/No] | [If yes: which rules] |
| algorithm-reference.md | [Yes/No] | [If yes: which algorithms] |
| error-taxonomy.md | [Yes/No] | [If yes: which domains] |

### Recommended Action Plan

1. [First thing to do — usually update 006 if affected]
2. [Update downstream spec artifacts in dependency order]
3. [Update implementation files in dependency order]
4. [Run /speckit.conform to verify cross-language consistency]

### Metrics

- Source spec: [NNN]
- Downstream specs affected: [N]
- Spec artifacts affected: [N]
- Implementation files affected: [N]
- Classification: [BREAKING/SIGNIFICANT/ADDITIVE/INFORMATIONAL]
```

### 7. Offer Remediation

Based on the classification:

- **BREAKING**: "This is a breaking change. The following tasks.md files need regeneration: [list]. Would you like me to hand off to /speckit.tasks for each affected spec?"
- **SIGNIFICANT**: "This change should be propagated. Review the impact report and update affected files. Consider running /speckit.tasks to regenerate task lists."
- **ADDITIVE**: "This is an additive change. New tasks may be needed. Consider running /speckit.tasks for the affected specs."
- **INFORMATIONAL**: "No implementation changes needed. Consider updating documentation in affected specs for clarity."

**Do NOT** apply any changes automatically. Wait for user approval before offering handoffs.

## Relation to Pipeline Checkpoints

This command supports:
- **Stage 7** (Change Propagation) of the AI Spec-to-SDK pipeline
- Feeds into **CP-3** (FR Traceability) and **CP-4** (Cross-Spec Consistency) validation

Run `.specify/scripts/bash/validate-pipeline.sh` after propagation to verify all checkpoints still pass.
