# Tasks: Protocol Determinism

**Input**: Design documents from `/specs/006-protocol-determinism/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US6)

## Constitution Compliance Notes

- **Principle VII**: Every task references the FR-* or SC-* it satisfies
- **Principle IX**: Test/verification tasks appear before dependent implementation tasks
- **Principle X**: Signing-related tasks include determinism verification

---

## Phase 1: Setup

**Purpose**: Directory structure and skeleton files

- [ ] T001 [P] [US1-6] Create `specs/006-protocol-determinism/` directory structure with `contracts/` subdirectory
- [ ] T002 [P] [US1-6] Create skeleton spec.md with user stories US1–US6 and all FR-* identifiers (FR-W01–W10, FR-A01–A14, FR-V01–V04, FR-X01–X04)
- [ ] T003 [US1-6] Run constitution check against plan.md — verify all 10 principles pass or have justified reconciliation (Principle VI reconciled per Research R5)

**Checkpoint**: Skeleton structure ready. All subsequent tasks produce content within these files.

---

## Phase 2: Wire Format (US1)

**Goal**: Complete wire-format.md contract — unambiguous JSON encoding rules for all serialized structures

- [ ] T004 [P] [US1] Write §1 Global JSON Rules: compact format, UTF-8, no BOM, escaping per RFC 8259 §7 (FR-W01, FR-W08)
- [ ] T005 [P] [US1] Write §2 Canonical Field Ordering for TopicMessage, HeartbeatPayload, Capabilities, Location, EncryptedPrivateKey (FR-W05)
- [ ] T006 [P] [US1] Write §3 uint64 Encoding: JSON string with decimal, no leading zeros (FR-W02) — resolve audit B-1
- [ ] T007 [P] [US1] Write §4 Binary Encoding: RFC 4648 §4 base64 with padding (FR-W03) — resolve audit B-2, A-5
- [ ] T008 [P] [US1] Write §5 Optional Field Handling: omit absent, never null (FR-W04) — resolve audit B-3
- [ ] T009 [US1] Write §6–§9: EVMAddress encoding (FR-W06, FR-W09), big.Int encoding (FR-W07), float64 encoding (FR-W10), UTF-8 mandate (C-2) — resolve audit B-4, B-5

**Checkpoint**: Wire format contract complete. Any implementation can produce deterministic JSON for all entities.

---

## Phase 3: Algorithms (US2)

**Goal**: Complete algorithm-reference.md contract — byte-level algorithms replacing all Go library references

### Key Generation & Derivation

- [ ] T010 [P] [US2] Write §1 secp256k1 Key Generation: curve params, key validation, scalar multiplication (FR-A01)
- [ ] T011 [P] [US2] Write §2 Point Compression: parity rule (0x02 even, 0x03 odd), decompression (FR-A02) — resolve audit C-7
- [ ] T012 [P] [US2] Write §3 EVM Address Derivation: uncompressed key → Keccak256 → last 20 bytes (FR-A03)
- [ ] T013 [P] [US2] Write §4 EIP-55 Checksum Encoding: 4-step algorithm with nibble comparison (FR-A04) — resolve audit A-2

### Identity Derivation

- [ ] T014 [US2] Write §5 PeerID Derivation: protobuf wrap → multihash → identity-vs-SHA256 threshold → base58btc (FR-A05) — resolve audit A-1
- [ ] T015 [US2] Write §6 DID:key Construction: multicodec varint + compressed key → base58btc (FR-A06) — resolve audit A-6

### Signing

- [ ] T016 [US2] Write §7 RFC 6979 Deterministic Signing: HMAC-SHA256 nonce generation, step-by-step (FR-A07)
- [ ] T017 [P] [US2] Write §8 Keccak256 Pre-Image for TopicMessage: timestamp || seqnum || payload (FR-A08)
- [ ] T018 [P] [US2] Write §9 Keccak256 Pre-Image for HeartbeatPayload: canonical JSON as payload (FR-A09)
- [ ] T019 [US2] Write §10 ECDSA Signature Encoding: R||S||V 65 bytes, low-S normalization, V=0/1 convention (FR-A10)

### Encryption & Detection

- [ ] T020 [P] [US2] Write §11 Argon2id Key Encryption: time=3, memory=65536, threads=4 defaults (FR-A11) — resolve audit C-6
- [ ] T021 [P] [US2] Write §12–§13 BIP-39 Mnemonic to Seed + BIP-44 HD Derivation Path m/44'/60'/0'/0/0 (FR-A12, FR-A13)
- [ ] T022 [US2] Write §14 Ed25519 Key Detection and Rejection: OID, protobuf KeyType, prefix byte, raw byte rules (FR-A14) — resolve audit A-7

**Checkpoint**: All 14 algorithms described at byte level. No Go library references remain.

---

## Phase 4: Test Vectors (US3)

**Goal**: Complete test-vectors.md with golden chains verified across at least two independent implementations in different languages

- [ ] T023 [US3] Define Chain 1 structure: key derivation (private → compressed → uncompressed → EVM → PeerID → DID:key) with test key k=1 (FR-V01)
- [ ] T024 [US3] Generate Chain 1 values: compute values using the algorithm definitions in `algorithm-reference.md` with k=1, record all intermediate hex values (FR-V01)
- [ ] T025 [US3] Cross-verify Chain 1 against Python `eth_keys`/`eth_utils` for EVM address and against `base58` for PeerID encoding (FR-V01, SC-003)
- [ ] T026 [US3] Define and generate Chain 2: TopicMessage signing chain — canonical JSON, pre-image, Keccak256, ECDSA signature (FR-V02)
- [ ] T027 [US3] Define and generate Chain 3: HeartbeatPayload signing chain — payload JSON, TopicMessage wrapping, signing (FR-V03)
- [ ] T028 [US3] Define and generate Chain 4: Key encryption round-trip — Argon2id → AES-GCM → decrypt → verify (FR-V03)
- [ ] T029 [US3] Verify signing determinism: sign same (hash, key) pair in at least two independent implementations — assert byte-identical R||S||V (SC-008, Principle X)

**Checkpoint**: All 4 golden chains complete with hex values. Cross-verified against at least one non-Go implementation.

---

## Phase 5: Cross-Spec Resolution (US5)

**Goal**: Resolve contradictions and bridge gaps between specs

- [ ] T030 [US5] Document signing responsibility resolution in spec.md FR-X01: caller signs, adapter validates. Reference the protocol-level separation-of-concerns rationale in Research R1 (FR-X01)
- [ ] T031 [P] [US5] Write Implementation Type Mapping Table in data-model.md: protocol-level type equivalents with implementation examples across multiple languages (FR-X02) — resolve audit D-1 through D-6
- [ ] T032 [US5] Define registration bridge mapping (001 ↔ 003) in spec.md FR-X04: which NeuronAccount fields map to which registry inputs (FR-X04) — resolve audit X-4

**Checkpoint**: All cross-spec contradictions documented and resolved.

---

## Phase 6: Error Taxonomy (US4)

**Goal**: Complete error-taxonomy.md with unified cross-spec error codes

- [ ] T033 [US4] Design error code namespace: NEURON-{DOMAIN}-{NUMBER} format, domain codes for KEY/ACCT/REG/TOPIC/HEALTH/WIRE
- [ ] T034 [US4] Catalog all errors per spec: extract from specs 001–005 data models and contracts, assign codes, define trigger conditions and recommended actions
- [ ] T035 [US4] Define error propagation model: cross-module wrapping convention, cause chain, severity model

**Checkpoint**: Every error condition across specs 001–005 maps to a unique error code.

---

## Phase 7: Assembly (US6)

**Goal**: Complete amendment-log.md, verify internal cross-references, ensure self-containment

- [ ] T036 [US6] Write amendment-log.md: map each of the 31 audit items (A-1..X-6) to the 006 artifact and section that resolves it (SC-001)
- [ ] T037 [US6] Add inline normative summaries for external standards: EIP-55, libp2p PeerID, DID:key, RFC 6979, BIP-39, BIP-44, secp256k1 curve params (FR-X03) — resolve audit X-6
- [ ] T038 [US6] Internal cross-reference check: verify every FR-* in spec.md has at least one corresponding contract section and one task (SC-001, Principle V)

**Checkpoint**: All artifacts internally consistent. Amendment log covers all audit items.

---

## Phase 8: Verification (SC-002 through SC-008)

**Goal**: End-to-end verification that specs 001–006 are machine-deterministic

- [ ] T039 [US6] Self-containment test: read spec 006 + specs 001–005 without Go source code or web access. Attempt to trace the key derivation chain from spec text alone. Document any step that requires external knowledge (SC-002)
- [ ] T040 [US3] Cross-language dry-run: describe step-by-step how a Python developer would implement key derivation using only the spec text. Verify no step requires Go knowledge (SC-002, SC-004)
- [ ] T041 [US1-6] Final review: verify SC-001 through SC-008 are all satisfied

**Checkpoint**: Spec 006 is complete. The full set (001–006) is a self-contained, machine-deterministic protocol definition.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — can start immediately
- **Phase 2 (Wire Format)** and **Phase 3 (Algorithms)**: Depend on Phase 1. Can run **in parallel** with each other.
- **Phase 4 (Test Vectors)**: Depends on Phase 2 and Phase 3 (needs wire format rules and algorithm definitions to generate correct vectors)
- **Phase 5 (Cross-Spec)**: Depends on Phase 1. Can run **in parallel** with Phases 2/3.
- **Phase 6 (Error Taxonomy)**: Depends on Phase 1. Can run **in parallel** with Phases 2/3/5.
- **Phase 7 (Assembly)**: Depends on Phases 2, 3, 4, 5, 6 (needs all content to cross-reference)
- **Phase 8 (Verification)**: Depends on Phase 7 (needs complete spec set)

### Parallel Opportunities

```
Phase 1 (Setup)
    ├── Phase 2 (Wire Format)  ──┐
    ├── Phase 3 (Algorithms)   ──┤
    ├── Phase 5 (Cross-Spec)   ──┼── Phase 7 (Assembly) ── Phase 8 (Verification)
    └── Phase 6 (Error Taxonomy) ┘
                                 │
         Phase 4 (Test Vectors) ─┘
              (needs 2+3)
```

### Within Each Phase

- Tasks marked [P] can run in parallel
- Signing tasks (T016, T019, T029) include determinism verification per Principle X
- Test vector tasks (T024–T029) require implementing the algorithm chain in at least one language
