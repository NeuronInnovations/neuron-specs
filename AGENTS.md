# AGENTS.md

Operational guide for AI coding agents working in this repository. Quick-reference companion to `CLAUDE.md` with the same substance — start here if you have limited context budget.

## What This Repo Is

Neuron SDK specification corpus with Go and TypeScript reference implementations.

Neuron is a **language-agnostic protocol** (Constitution Principle VI): specs under `specs/` — together with Spec 006 (Protocol Determinism) and its golden test vectors — are the sole source of truth for protocol semantics. No SDK is normative. This repo currently uses Go as the **primary reference implementation path** (first to land each spec, first HCS integration, default `/speckit.plan` target). TypeScript is a first-class subsequent implementation; other languages are equal in standing once they pass the Spec 006 conformance suite.

Specs define a hierarchical agent identity model with DLT-traceable communication: Parent accounts hold root DIDs and control Child agents, which register as EIP-8004 NFTs. Communication flows through signed, append-only topic channels (HCS primary). Liveness is proven via self-declared deadline heartbeats. Validators are autonomous agents that publish evidence-based verdicts to on-chain registries (Principle XI).

## Commands

### Go SDK (run from `impl/golang/`)

```bash
# Build
cd impl/golang && go build ./...

# Run all tests + vet
cd impl/golang && go test ./internal/... && go vet ./internal/...

# Run tests for a single module
cd impl/golang && go test ./internal/keylib/...      # 002 keys
cd impl/golang && go test ./internal/account/...     # 001 account
cd impl/golang && go test ./internal/topic/...       # 004 topics
cd impl/golang && go test ./internal/registry/...    # 003 registry
cd impl/golang && go test ./internal/health/...      # 005 health
cd impl/golang && go test ./internal/payment/...     # 008 payment
cd impl/golang && go test ./internal/delivery/...    # 009 delivery
cd impl/golang && go test ./internal/validation/...  # 010 validation

# Run a single test function
cd impl/golang && go test ./internal/keylib/... -run TestFunctionName

# Verbose output
cd impl/golang && go test -v ./internal/keylib/...
```

Go module: `github.com/neuron-sdk/neuron-go-sdk` (Go 1.25.7, see `impl/golang/go.mod`).

### Race detection (mandatory after runtime/concurrency changes)

After any runtime, concurrency, scheduling, or shared-state change — buyer/seller flow, topic adapters, payment lifecycle, libp2p/delivery, edge app, FID display, remoteid DApp — run the scoped race check before declaring the change done:

```bash
scripts/validation/race-check.sh
```

The script runs `go test -race ... -count=1 -timeout 180s` across the affected surfaces in three groups (A protocol / B runtime / C cmd) and prints a per-group PASS/FAIL summary. It is **additional to** the standard `go test`, `go vet`, and `go build` gates, not a replacement.

Do not hide race failures. If a group fails, quote the race-detector report verbatim in the change summary and fix it before shipping. This rule exists because a race condition once crashed a deployed buyer in the field (2026-05-13).

### Demo CLIs (run from `impl/golang/`)

```bash
# Buyer-seller JPEG demo — mock (instant, no network) or testnet (~90s, real HCS)
cd impl/golang && go run ./cmd/buyer-seller-demo/ --jpeg cmd/buyer-seller-demo/testdata/photo.jpg --mode=mock
cd impl/golang && go run ./cmd/buyer-seller-demo/ --jpeg cmd/buyer-seller-demo/testdata/photo.jpg --mode=testnet

# Live HCS heartbeat test (publishes to Hedera testnet, verifies via mirror node)
cd impl/golang && go run ./cmd/hcs-heartbeat-test/

# P2P delivery demo (two terminals: seller + buyer over libp2p QUIC)
cd impl/golang && go run ./cmd/delivery-demo/ --mode seller
cd impl/golang && go run ./cmd/delivery-demo/ --mode buyer --peer <seller-multiaddr>
```

### TypeScript SDK (run from `impl/typescript/`)

```bash
cd impl/typescript && npm install
cd impl/typescript && npm run build
cd impl/typescript && npm test                    # all tests
cd impl/typescript && npm run test:conformance    # Spec 006 conformance suite only
cd impl/typescript && npm run lint
cd impl/typescript && npm run typecheck
```

Package: `@neuron-sdk/typescript` (TypeScript 5.7+, Node.js >= 18, vitest).

## Project Structure

```
specs/                             # Language-neutral feature specifications
  NNN-feature-name/
    spec.md                        # Feature specification (primary artifact)
    plan.md                        # Implementation plan
    tasks.md                       # Dependency-ordered task list
    data-model.md                  # Entity/type definitions
    research.md                    # Technical research (planning only)
    quickstart.md                  # Developer quick-start guide
    contracts/                     # API contracts, wire format, test vectors
    checklists/                    # Quality checklists
impl/
  golang/                          # Go reference implementation
    internal/
      keylib/                      # 002 — keys, signatures, PeerID, DID:key
      account/                     # 001 — Parent/Child/Shared identity, DID
      topic/                       # 004 — TopicMessage, HCS/Kafka/ERC adapters
      registry/                    # 003 — EIP-8004 registration, AgentURI
      health/                      # 005 — HeartbeatPayload, liveness observer
      payment/                     # 008 — commerce protocol, negotiation, escrow
      delivery/                    # 009 — libp2p P2P data delivery, ECIES
      validation/                  # 010 — validators, evidence, verdicts
    cmd/
      buyer-seller-demo/           # End-to-end negotiation + payment + JPEG + validation
      delivery-demo/               # Standalone libp2p P2P delivery test
      hcs-heartbeat-test/          # Live Hedera testnet heartbeat publishing
  typescript/                      # TypeScript SDK (subsequent implementation)
    src/                           # keylib, account, topic, registry, health, wire, contracts
    tests/
      conformance/                 # Spec 006 golden-vector tests (cross-language)
docs/                              # Reference materials, architecture docs
.specify/
  memory/constitution.md           # Constitution Principles I-XI (v1.6.0)
  templates/                       # Spec Kit plan + tasks generation templates
```

## Execution Order and Dependencies

Build order is strictly sequential. Each module depends on all modules above it. Do not begin spec N+1 until spec N passes all tests and exported types are stable.

```
002 Key Library            <- Foundation: zero dependencies
 |
001 NeuronAccount          <- Identity built on keys
 |
004 Topic System           <- Communication substrate
 |
003 Peer Registry          <- Registration requires topic schemas
 |
005 Health                 <- Heartbeats flow through topics
 |
006 Protocol Determinism   <- Cross-cutting: wire format, signing, test vectors
 |
007 Identity Contract      <- On-chain: EVM registries (identity, reputation, validation)
 |
008 Payment                <- Commerce protocol between agents
 |
009 P2P Data Delivery      <- Data plane: libp2p streams, ECIES encryption
 |
010 Validation Framework   <- Evidence model, validator agents, oracle verdicts
```

**Why 004 before 003:** Peer Registry's `agentURI` must embed `NeuronTopicService` and `NeuronP2PExchangeService` schemas defined by Topic System.

**Spec 006 is cross-cutting and normative.** Its contracts — `specs/006-protocol-determinism/contracts/{wire-format,algorithm-reference,test-vectors}.md` — define the byte-level JSON canonicalization, pre-image construction, and signing algorithms that every language implementation MUST reproduce exactly (Constitution Principle X). Conformance is verified by the golden test vectors run from `impl/typescript/tests/conformance/` and `impl/golang/internal/keylib/conformance_test.go`.

## Spec Lifecycle (Spec Kit Toolchain)

Each spec follows a strict 6-phase lifecycle via slash commands. Phases are sequential — complete each gate before proceeding.

```
/speckit.specify  ->  /speckit.clarify  ->  /speckit.plan  ->  /speckit.tasks  ->  /speckit.implement  ->  test (manual)
```

| Phase     | Gate to Next                                                                |
| --------- | --------------------------------------------------------------------------- |
| Specify   | All mandatory sections present (scenarios, FRs, SCs, Evidence & Validation) |
| Clarify   | Zero `[NEEDS CLARIFICATION]` markers remain                                 |
| Plan      | Constitution Check passes 11/11 principles                                  |
| Tasks     | FR/SC traceability + test tasks precede implementation tasks                |
| Implement | All tasks done, code compiles, tests pass                                   |
| Verify    | All tests green, exported types stable, no vet/lint warnings                |

Implementation uses TDD Red-Green-Refactor (Constitution IX). Optional quality tools: `/speckit.analyze` (consistency check), `/speckit.checklist` (custom checklist), `/speckit.conform` (cross-language conformance against Spec 006 vectors), `/speckit.taskstoissues` (GitHub Issues), `/speckit.propagate` (spec-change impact analysis).

## AI Context Loading Strategy

When working on a specific spec, load context in this order:

1. **Always**: `AGENTS.md` (or `CLAUDE.md`), `.specify/memory/constitution.md`, `specs/<target>/{spec,plan,tasks}.md`
2. **Dependencies**: `specs/<dep>/contracts/` (API contracts only) + `impl/golang/internal/<dep-pkg>/doc.go`
3. **Implementation**: `specs/<target>/data-model.md`, `specs/<target>/contracts/`, `impl/golang/internal/<target-pkg>/` (and `impl/typescript/src/<target-pkg>/` if doing TS work)
4. **Conformance**: For any signing, canonical-JSON, or wire-format work, always cross-check `specs/006-protocol-determinism/contracts/{wire-format,algorithm-reference,test-vectors}.md` — these are normative.
5. **Skip**: Full spec.md of dependencies, downstream specs, research.md / quickstart.md during implementation

## Hedera Testnet Integration

The Go SDK integrates with Hedera Consensus Service (HCS) for real on-chain publishing. Key facts for agents working on 004/005/008/010:

- **Operator account**: via `HEDERA_OPERATOR_ID` / `HEDERA_OPERATOR_KEY` env vars (ECDSA secp256k1, funded testnet account)
- **HCS adapter**: `impl/golang/internal/topic/adapter_hcs.go`
- **Reusable client**: `impl/golang/internal/topic/hcs_real_client.go` wraps the Hiero SDK
- **SDK import**: `hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"`
- **HCS message limit**: 1024 bytes max per message (`HCSMaxMessageSize` constant)
- **Mirror node**: `https://testnet.mirrornode.hedera.com/api/v1/topics/{topicId}/messages`

The buyer-seller demo's `--mode=testnet` flag exercises real HCS topic creation and message publishing end-to-end.

## Key Dependencies

### Go SDK
- `go-ethereum` — secp256k1, Keccak256, EIP-55, ecrecover
- `go-libp2p` — PeerID derivation, P2P transport (QUIC/WebRTC) for delivery
- `hiero-sdk-go` — Hedera Consensus Service (topic creation, message publishing)
- `go-bip39` / `go-bip32` — Mnemonic / HD key derivation
- `argon2` (golang.org/x/crypto) — Argon2id KDF for key encryption
- `base58` — DID:key base58btc encoding
- `stretchr/testify` — Test assertions

### TypeScript SDK
- `@noble/secp256k1` + `@noble/hashes` — secp256k1, Keccak256
- `@scure/bip32` / `@scure/bip39` — HD key derivation, mnemonics
- `bs58` — Base58 encoding
- `hash-wasm` — Argon2id KDF

## Constitution Principles (v1.6.0)

All implementation must comply with 11 ratified principles. Full text: `.specify/memory/constitution.md`.

| #    | Principle                                            | Key Rule                                                                                     |
| ---- | ---------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| I    | Specification-First                                  | Spec exists before any code                                                                  |
| II   | Testable Stories                                     | Given/When/Then acceptance criteria; each story independently testable                       |
| III  | Clarification Before Plan                            | No unresolved `[NEEDS CLARIFICATION]` before plan                                            |
| IV   | Semantic Types                                       | Domain types, not primitives                                                                 |
| V    | Traceability                                         | Every task traces to FR-\* or SC-\*                                                          |
| VI   | Language-Neutral Protocol, Reference Implementations | Specs + Spec 006 vectors normative; Go is this repo's current reference implementation path  |
| VII  | Strict Spec Compliance                               | No silent divergence from MUST / SHOULD                                                      |
| VIII | Hedera Topic Transport Binding                       | HCS is the first-class TopicAdapter; Spec 005 validated end-to-end on HCS first              |
| IX   | Test-First Development                               | TDD: Red → Green → Refactor; integration tests across cross-spec contracts                   |
| X    | Deterministic Message Signing                        | RFC 6979 + Keccak256 + R\|\|S\|\|V; reproduce Spec 006 vectors byte-for-byte across SDKs     |
| XI   | Verifiable Execution (Evidence-Based Validation)     | Every spec defines observable signals, evidence rules, non-observable areas, evidence recipes |

### Principle XI — Evidence & Validation Section

Every spec MUST include an **"Evidence & Validation"** section with four subsections:

1. **Observable Signals** — what can be externally observed (blockchain transactions, topic messages, emitted events, public state transitions)
2. **Evidence Rules** (`VR-*`) — how signals map to **Compliant** (code `1`) / **Non-compliant** (`2`) / **Inconclusive** (`3`) verdicts. These are *suggested* interpretations, not mandated procedures.
3. **Non-Observable Areas** — what CANNOT be verified externally. Include **Behavioral Inference Recipes** for observable proxies.
4. **Suggested Evidence Recipes** — informative step-by-step guidance for common validation scenarios.

Validators are autonomous agents that register with a `neuron-validator` service type and choose their own methods. The protocol evaluates **evidence quality and defensibility of the judgment**, not the tooling used. Validator output has three outcomes (Compliant / Non-compliant / Inconclusive). If behaviour cannot be evidenced, it cannot be enforced — specs must be honest about their verification limits.

## Cross-Spec Data Flow

Types flow across specs in a directed pipeline:

```
NeuronPrivateKey (002) ──> NeuronAccount (001) ──> TopicRef (004) ──> AgentURI (003) ──> HeartbeatPayload (005)
      │                          │                      │                   │
      ├── EVMAddress ────────────┤                      │                   ├── register() on-chain (007)
      ├── PeerID ────────────────┼── NeuronP2PExchange ─┘                   │
      ├── DID:key ───────────────┘                                          │
      └── Sign/Verify ─────────────────── TopicMessage envelope ────────────┘
```

- **002 → 001**: Keys create account identities (EVM address, DID)
- **001 → 004**: Account keys sign TopicMessage envelopes
- **004 → 003**: Topic services populate `agentURI` for registration
- **003 → 007**: SDK registration calls map to on-chain `register()` / `updateAgentURI()` / `revoke()`
- **004 → 005**: Heartbeats are TopicMessages published to `stdOut`
- **003 → 005**: Observer discovers peer's `stdOut` via `LookupRegistration` + `ResolveTopics`
- **006**: Wire format, canonical JSON, and signing algorithms apply normatively across all specs
- **007 → 008**: Payment uses Identity Registry for agent discovery + settlement
- **008 → 009**: Delivery implements the data plane for negotiated service connections (connectionSetup, ECIES)
- **009**: libp2p QUIC/WebRTC streams carry service data; multiaddrs encrypted via ECIES (secp256k1 ECDH + AES-256-GCM)
- **010**: Validators publish evidence envelopes to topics; verdicts anchored on-chain via Validation Registry (007)

## Code Style

### Go Implementation (`impl/golang/`)

- **Package layout**: `internal/` packages with a `doc.go` per package providing the package-level godoc (design principles, usage examples, API overview). Read `doc.go` first when exploring a package.
- **Error handling**: No panics. Each package defines a structured error type (e.g. `KeyError` with `ErrorKind` enum, operation name, descriptive message). Errors implement `Is()` and `Unwrap()`. Format: `"pkgname.OperationName: [ErrorKind] descriptive message"`.
- **Factory functions**: `NewTypeName()` for construction, `TypeNameFromX()` for conversion (e.g. `NeuronPrivateKeyFromHex`, `NeuronPrivateKeyFromMnemonic`).
- **Immutability**: All types immutable after construction. No setters. Invalid inputs rejected at construction time. Valid by construction.
- **Semantic types**: Strong domain types, not primitives (e.g. `NeuronPrivateKey`, `EVMAddress`, `PeerID`, `Signature`). Constitution IV.
- **FR/SC tracing**: Doc comments reference spec requirements: `// FR-001: ...`, `// SEC-003: ...`.
- **Security**: Key material MUST NOT appear in error messages or logs (SEC-003, SEC-005).
- **Testing**: `stretchr/testify` assertions. Test files alongside source (`*_test.go`). Integration tests in `integration_test.go`.
- **Constants**: Named constants for magic values (e.g. `PrivateKeyLength`, `HCSMaxMessageSize`).

### TypeScript Implementation (`impl/typescript/`)

- **Module system**: ESM (`"type": "module"`); build via `tsc -p tsconfig.build.json`.
- **Testing**: vitest. Conformance tests in `tests/conformance/` cross-check Spec 006 golden vectors against Go output.
- **Linting**: eslint with `@typescript-eslint`.
- **SDK derivation rule**: TypeScript SDK derives purely from specs, never from Go code (Constitution VI — behavior observed in one SDK is descriptive, never normative).

### Specs (`specs/`)

- **Language-neutral** (Constitution VI) — no Go or TypeScript types, runtime details, or framework references in spec prose.
- Solidity `^0.8.20` for informative contract interfaces only.
- On-chain EVM state (mappings, arrays); no off-chain database for identity contracts.
- Every spec MUST have a **Related specs** section listing in-repo dependencies and external standards (EIPs, HIPs), and MUST include **Mermaid diagrams** in an Appendix where applicable.
- Every spec MUST have an **Evidence & Validation** section with the four subsections required by Principle XI.
