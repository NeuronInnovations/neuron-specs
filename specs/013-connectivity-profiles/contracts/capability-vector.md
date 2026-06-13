# Capability Vector — Machine-Readable Enumeration (pending)

This file is a stub. The normative capability vocabulary is defined in [`../spec.md`](../spec.md) section "Capability Vocabulary".

The full machine-readable enumeration (per-key typed value constraints, closed-enum lists, scalar ranges, cross-capability implication rules) is produced by `/speckit.plan` for spec 013.

**Capability keys** (normative in v2 of this spec; eleven keys total):

- `control-plane`
- `audit-trail`
- `identity-lifetime`
- `listen-capability`
- `nat-traversal`
- `settlement`
- `max-payload`
- `confidentiality`
- `ordering`
- `reconnect-semantics`
- `stream-init-direction` *(added 2026-05-08, v2 amendment; allowed values: `seller`, `buyer`, `either`. Default for new profiles is `either`. Profile E pins `seller` for back-compat. Per-stream override via 008 FR-P33a `streams[].direction` takes precedence over the profile-level default.)*

Each key's allowed values are defined in `../spec.md`. Closure and extensibility rules are governed by FR-CP-003. Adding new keys is a major-version amendment of this spec; the v1 → v2 transition was the addition of `stream-init-direction`.

**v2.x minor-version vocabulary additions (2026-05-12, demo/lab/TEVV amendment for Profile F)** — three new enum values introduced by `profiles/F-fixture-direct.md`. Per FR-CP-003, additive enum values within existing keys are minor-version amendments:

- `control-plane = out-of-band` — control information (seller multiaddr, identities) is exchanged outside the protocol (operator CLI, hand-edited file, paste). Allowed only under Profile F.
- `nat-traversal = explicit-multiaddr` — the dialled multiaddr is supplied directly; no traversal protocol engaged. Allowed under Profile F.
- `settlement = n/a` — no escrow, no settlement state machine. Allowed only under Profile F.

Profile F additionally uses `audit-trail = none` (already enumerated; documented as Profile A optional in spec.md) and authorises `identity-lifetime = ephemeral` (already enumerated). The full Profile F capability vector lives in `profiles/F-fixture-direct.md` §"Capability vector — machine-readable form".
