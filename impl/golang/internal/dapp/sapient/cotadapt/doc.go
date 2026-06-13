// Package cotadapt projects a SAPIENT DetectionReport into Cursor-on-Target (CoT)
// XML for a TAK / military display.
//
// Framing (spec 015 architecture diagram, p3): canonical CoT is the *fused* TAK
// output of an 018 Fusion Node ("Classified, fused, targets"). SAPIENT itself has
// no allegiance / affiliation / IFF field — affiliation is a fusion output assigned
// downstream at the HLDMM. This package is therefore an MVP **pre-fusion display
// projection** at the buyer: it emits TAK-ready CoT with affiliation **unknown**
// (`a-u-…`) and performs **no classification or fusion**. Real affiliation/IFF +
// fusion are deferred to 018. The default affiliation is NEVER hostile.
//
// Mapping (drone event):
//   - uid   = DetectionReport.id (RID serial) else object_id (ULID)
//   - type  = "a-<affiliation>-A" (default "a-u-A", atoms-unknown-air)
//   - point = Location.{Y→lat, X→lon, Z→hae}; ce from horizontal x/y error,
//     le from z error (CoT 9999999.0 sentinel when unknown)
//   - time/start = SapientMessage.timestamp; stale = time + TTL
//   - detail = contact callsign + a class remark
//
// Operator event (optional, when rid.operator* object_info is present): a second
// CoT event uid=<drone>-OP, type "a-<aff>-G", linked to the drone via a p-p link.
// This is a **display projection**, not a second SAPIENT DetectionReport.
//
// The package is pure and deterministic (CoT time fields come from the message
// timestamp, not the wall clock) — golden-testable. It does NOT modify fid-display
// and does not touch the /ds240/* paths.
package cotadapt
