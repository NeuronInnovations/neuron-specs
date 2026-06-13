# Profile C — Any to NATed Peer via Relay (pending)

This file is a stub. The normative Profile C definition lives in [`../spec.md`](../spec.md) section "Profile Definitions → Profile C — Any → NATed Peer via Relay".

The full per-profile artifact — with expanded runtime-context descriptions, per-binding capability matrices, relay-lifecycle diagrams, sample descriptor fixtures, and test-vector references — is produced by `/speckit.plan` for spec 013.

**Normative summary** (see `../spec.md` for the complete requirement set):

- **Profile id**: `c-relay-assisted/1`
- **Runtime contexts**: initiator = any runtime with outbound connectivity; responder = NATed process holding a relay reservation
- **Valid transport bindings**: `T-QUIC+Relay` (implemented); `T-WebTransport+Relay` (deferred)
- **Authoritative relay behavior**: spec 011 (to be created by a separate `/speckit.specify` run)

See FR-CP-005 and the Profile C subsection of `../spec.md` for required capabilities, optional capabilities, unsupported assumptions, failure modes, and the conformance statement. Relay semantics, consent model, resource accounting, and evidence tier belong in spec 011.
