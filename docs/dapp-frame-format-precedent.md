# DApp Frame-Format Precedent

> **Authority**: Constitution Principle XII (v1.7.0). This document is informative; it elaborates the boundary test in XII for one specific DApp design decision (data-plane payload format) without amending the principle itself.
> **Created**: 2026-05-08
> **Status**: Normative-informative. Future DApp specs MUST consult this document and cite the rule that justifies their frame-format choice in their "Layering Compliance Check" section.

## Purpose

When a new DApp is specified, its data plane carries domain-specific bytes over the libp2p stream catalog defined by Spec 008 (`streams[]` per FR-P33a) and Spec 009 (multi-protocol + wildcard registration). The Core SDK does not prescribe a frame format for the bytes inside a length-prefixed 009 frame (FR-D22) — that is a DApp choice. This document tells DApp authors how to make that choice consistently.

Two valid choices exist today, exemplified by the first two DApps:

- **Opaque pass-through** (Spec 016 ADS-B with BEAST 0x1A frames).
- **Normalized canonical JSON** (Spec 017 Remote ID with `RemoteIdFrame` payloads).

Both satisfy Constitution Principle XII. Neither is universally better — the choice depends on domain characteristics. The rules below cover both.

## Rule R-FF-01 — Opaque pass-through

A DApp SHOULD use opaque pass-through (transmit the upstream sensor's bytes verbatim inside the 009 length-prefixed frame, with no SDK-level re-serialization) when ALL of the following hold:

1. The domain already has a single de facto binary wire format with broad consumer support — existing decoder libraries, vendor tooling, established analysis pipelines.
2. Re-serializing the format into JSON would be lossy (e.g., bit-packed timestamp precision) or wasteful (e.g., 14-byte frames inflated to 200-byte JSON envelopes).
3. Per-frame size is small (~ tens of bytes) AND frame rate is high (≥ 10 Hz aggregate).
4. The buyer-side ecosystem already includes decoders for the format; expecting the buyer to decode is reasonable.

**Reference DApp**: 016 ADS-B uses BEAST 0x1A frames per FR-A05. The format has decades of established consumer support (Mode-S decoders ship with most aviation-tracking software), per-frame size is 14 bytes for short DF formats and 21 for long, and frame rate at a busy receiver can exceed 100 Hz aggregate.

**Trade-offs**:

- ✅ Minimal SDK overhead; minimum bandwidth.
- ✅ No re-serialization bugs; bit-perfect fidelity to the upstream source.
- ❌ Buyer must implement or import a decoder.
- ❌ Cross-DApp fusion (e.g., a fused buyer for ADS-B + Remote ID + AIS) requires per-DApp decoder modules in the buyer.

## Rule R-FF-02 — Normalized canonical JSON

A DApp SHOULD use normalized canonical JSON (the seller decodes the upstream format and emits a Spec-006-compliant canonical-JSON envelope per 009 length-prefixed frame) when ANY of the following hold:

1. The domain has multiple regulatory variants or fragmented decoder ecosystems (e.g., FAA / EASA / ASD-STAN variants of the same standard) that benefit from seller-side normalization.
2. Per-frame size is > 1 KB, where the relative overhead of JSON envelopes is small.
3. Frame rate is low (single-digit Hz typical), where JSON encoding cost is amortized across rare frames.
4. Cross-DApp fusion is a primary use case and consumers benefit from a uniform shape across DApps.
5. The upstream binary format is poorly standardized or vendor-specific, and exposing it directly to buyers would couple them to upstream-vendor changes.

**Reference DApp**: 017 Remote ID uses `RemoteIdFrame` per FR-R05. ASTM F3411-22a has multiple regulatory deltas (FAA Part 89, EASA 2019/945, ASD-STAN extensions) and frame rate is typically 1 Hz per drone. Normalization stabilizes consumer expectations regardless of the upstream regulatory variant.

**Trade-offs**:

- ✅ Buyer needs no decoder beyond canonical-JSON parsing.
- ✅ Cross-DApp fusion is straightforward; one parser handles all canonical-JSON DApps.
- ✅ Seller-side normalization isolates buyers from upstream format drift.
- ❌ Higher per-frame size (typically 200–500 bytes).
- ❌ Seller must implement the decode + normalize pipeline.

## Rule R-FF-03 — Decision checklist

When designing a new DApp, walk this checklist in order. The first rule that produces a definite answer wins.

1. **Does an opaque format exist with established consumer support?**
   - If **no** → choose R-FF-02 (canonical JSON).
   - If **yes** → continue to step 2.
2. **Is per-frame size > 1 KB?**
   - If **yes** → choose R-FF-02 (overhead ratio favors JSON).
   - If **no** → continue to step 3.
3. **Is frame rate > 10 Hz aggregate?**
   - If **yes** → choose R-FF-01 (bandwidth and CPU favor opaque).
   - If **no** → continue to step 4.
4. **Is cross-DApp fusion a primary use case?**
   - If **yes** → choose R-FF-02 (uniform shape simplifies fusion).
   - If **no** → choose R-FF-01 by default (simpler is better when no factor pushes back).

## Constitution Principle XII reference

Frame format choice IS a DApp decision, NOT a Core SDK decision. The Core SDK defines envelopes (TopicMessage per 004, ConnectionSetup per 008 FR-P33, lifecycle messages per 008 FR-P36/P37/P38), signing rules (002, 006), the stream catalog model (008 FR-P33a, 009 FR-D-multi-protocol / FR-D-wildcard-handler), and observability metadata (heartbeat `capabilities` per 005); DApps choose their own data-plane payload format guided by this precedent.

A DApp spec MUST cite the rule it followed (R-FF-01 or R-FF-02) in its "Layering Compliance Check" section, and MUST justify the choice against the R-FF-03 checklist if the choice is non-obvious.

## Evolution rules

This document MAY be updated to add new rules as new DApp domains surface novel constraints (e.g., a video-streaming DApp may need a third rule covering chunked binary formats with side-band metadata). Updates to this document do NOT amend Constitution Principle XII; the principle authorizes DApps to make frame-format choices, and this document operationalizes the choice without expanding the principle's scope.
