# Profile D — Peer to Peer Direct (pending)

This file is a stub. The normative Profile D definition lives in [`../spec.md`](../spec.md) section "Profile Definitions → Profile D — Peer ↔ Peer Direct".

The full per-profile artifact — with expanded runtime-context descriptions, per-binding capability matrices, HCS + in-stream hybrid-control-plane diagrams, sample descriptor fixtures, and test-vector references — is produced by `/speckit.plan` for spec 013.

**Normative summary** (see `../spec.md` for the complete requirement set):

- **Profile id**: `d-peer-to-peer-direct/1`
- **Runtime contexts**: both parties = publicly-reachable processes; no NAT obstruction
- **Valid transport bindings**: `T-QUIC`, `T-TCP-Noise`; `T-WebTransport` feasible for server-to-server but not yet validated
- **Authoritative libp2p delivery binding**: spec 009
- **Authoritative topic binding**: spec 004 (HCS primary per Constitution Principle VIII)

See FR-CP-006 and the Profile D subsection of `../spec.md` for required capabilities, optional capabilities, unsupported assumptions, failure modes, and the conformance statement.
