# Proposed Constitution Amendment — Principle XII (Layered Architecture)

**Status**: PROPOSED — not applied. The live constitution remains at **1.7.0**; this file is the proposal and rationale record.
**Governance**: Per the amendment procedure in the constitution's Governance section, this change must be reviewed and ratified by the constitution maintainer before it is applied to `.specify/memory/constitution.md`. Until then, dependent specs (015, 016, 017, 018) reference it as pending.
**Bump (when applied)**: MINOR — expands an existing principle (adds a tier); no existing principle is contradicted and no 001–013 spec becomes non-compliant.

## Exact edits to apply (once ratified)

Apply via `/speckit-constitution` or directly to `.specify/memory/constitution.md`:

1. **Principle XII heading** → `### XII. Layered Architecture (Core SDK · Shared Application Profile · DApp)`; intro → "three strictly separated layers."
2. **Core SDK layer** → add: "The **application payload is opaque to the Core SDK layer** — Core specs treat what rides the lanes as bytes and MUST NOT prescribe an application message schema."
3. **Insert a new `#### Shared Application Profile layer (specs 014–015, reserved for this tier)`** between the Core SDK layer and the DApp layer — OPTIONAL cross-DApp conventions (shared envelope/standard adoption, registry-entry content, lane binding, tasking, fan-out, extension mechanism); MUST compose Core and MUST NOT redefine it; defines a *mechanism* shared by a family of DApps, not a vendor taxonomy / fusion policy / decoder / single product's semantics; MUST carry a Layering Compliance Check. State **015 = SAPIENT Sensor Interop Profile**; fan-out (old 014) is Shared-Profile/DApp, never Core; reusing 014/015 for anything else still needs a further amendment.
4. **DApp layer intro** → DApps compose Core "and any relevant Shared Application Profiles"; MUST cite the profiles they consume; MUST NOT redefine Shared-Profile behavior (changes go via a Shared-Profile amendment).
5. **Boundary test** → add the third branch: "mechanism shared by a family of related DApps, composing Core without redefining it → Shared Application Profile."
6. **Cross-references** → DApps MAY cite Shared-Profile FR ids (e.g. FR-S11); Core MUST NOT cite DApp/Shared-Profile ids; a Shared Profile MAY cite Core ids but not DApp ids; adding a DApp MAY require a Shared Profile (existing or new), never a Core amendment.
7. **Version** → 1.7.0 → **1.8.0**, Last Amended → ratification date; prepend a Sync Impact Report.

**Carried TODO (with the apply):** CLAUDE.md / AGENTS.md "Constitution Principles" tables and `.specify/templates/plan-template.md` Constitution Check row XII → three-layer model.

## Why

Principle XII currently defines a **binary** architecture: **Core SDK (001–013)** vs **DApp (016+)**, with the payload opaque to Core and each DApp owning its own payload schema. It also retired the old proposed core specs **014 (Fan-Out)** and **015 (admission policy)** into the DApp tier, and recorded the rule:

> *"Future authors MUST NOT resurrect 014/015 as core specs without amending Principle XII first."*

The SAPIENT Sensor Interop Profile (015) is **neither** Core **nor** a single DApp: it standardises an application-payload convention (the SAPIENT envelope + registry/lane binding + tasking + fan-out + extension mechanism) that is **shared across multiple sensor DApps** (016 JetVision, 017 DroneScout, 018 CoT Display Consumer). The binary model has no slot for it. This amendment adds that slot and reclaims **015** for it (honouring the "amend XII first" rule).

## Amendment (proposed replacement text for Principle XII)

### XII. Layered Architecture (Core SDK · Shared Application Profile · DApp)

The system is organised in three tiers:

1. **Core SDK (specs 001–013)** — transport, identity, discovery, topics, health, payment, validation, relay, connectivity primitives. **The application payload is opaque to Core.** Core specs MUST NOT assume any specific DApp or payload schema.

2. **Shared Application Profile (specs 014–015, reserved for this tier)** — OPTIONAL cross-DApp conventions that define how a *family* of DApps uses the Core lanes for a shared application concern: e.g. a common message envelope/standard, registry-entry content, lane binding, tasking, fan-out, and extension mechanism. A Shared Application Profile MUST compose Core primitives and MUST NOT redefine them. It defines a *mechanism*, not a vendor taxonomy or a fusion/decision policy. A DApp MAY depend on at most the profiles relevant to it. **015 = SAPIENT Sensor Interop Profile.** (Fan-out, formerly proposed core 014, is a Shared-Profile/DApp concern, not Core.)

3. **DApp (specs 016+)** — application semantics for a specific product/vendor: the concrete services offered, classification taxonomy, extension namespace, admission policy, and (for consumers) fusion/affiliation/output. A DApp MUST NOT redefine Core or Shared-Profile primitives; it composes them.

**Layering check (enforced at plan and tasks generation):** every spec MUST declare its tier and MUST NOT restate normative requirements owned by a lower tier; it references them. A Shared Application Profile spec MUST carry a Layering Compliance Check showing it composes Core and defines only shared mechanism. Reusing the reserved numbers 014/015 for anything other than a Shared Application Profile still requires a further amendment.

## Downstream doc updates required by this amendment

- `.specify/memory/constitution.md` — replace Principle XII text; add a Sync Impact Report entry; bump version.
- `CLAUDE.md` "Constitution Principles" table — update the XII row to the three-tier wording.
- `AGENTS.md` — update the layering description (note it was already stale).
- `.specify/templates/plan-template.md` / `tasks-template.md` — the "Layering check" must accept the Shared Application Profile tier.

## Impact on existing specs

- **001–013**: none (still Core; none violate the amended principle).
- **016/017/018**: defined on 015 as DApps that compose the Shared Application Profile (separate change on this branch / follow-on branches).
- **014**: remains reserved; available for a future Shared Application Profile if needed.
