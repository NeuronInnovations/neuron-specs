// Package tasking implements the SAPIENT control plane for a Neuron ASM (seller):
// per-session DetectionReport gating driven by Task{CONTROL_STOP/START} and a
// periodic StatusReport carrying feedSource — all on the auditable lane
// (internal/dapp/sapient/auditlane), the local stand-in for the 004 Topic System.
//
// Spec mapping (015):
//   - FR-S25/FR-S29: tasking is OPTIONAL except the mandatory per-session stream
//     STOP/START, which every ASM MUST support and TaskAck. CONTROL_STOP ceases
//     DetectionReport emission to ONE session, leaving other consumers untouched;
//     CONTROL_START resumes with no new Registration. STOP is idempotent.
//   - FR-S30/FR-S60: node-global commands (mode_change, look_at, move_to, patrol,
//     follow, *_threshold, report_rate, request) are REFUSED via
//     TaskAck{REJECTED, UnsupportedTask} — never silently ignored.
//   - FR-S31: the StatusReport (sensor health/mode) is distinct from the Neuron 005
//     liveness heartbeat and rides the auditable lane.
//   - 017 FR-R-E02: feedSource ∈ {live,replay,synthetic,placeholder}. SAPIENT v2.0
//     has no native feed_source field, so it is carried as a StatusReport.status[]
//     entry {STATUS_TYPE_OTHER, INFORMATION_STATUS, "neuron.feedSource=<value>"} —
//     the carrier the spec settled on in 017 CL-R9 (see FeedSourceStatusValue).
//
// The Manager fans a DetectionReport out to every OPEN session (Forward); a Task on
// the ASM's stdIn gates the session keyed by the issuing consumer's node_id and
// emits a TaskAck on stdOut; a ticker emits StatusReports on stdOut. In-flight
// reports already sent are never recalled — gating only affects subsequent fan-out.
//
// It owns NO libp2p or identity logic (the seller cmd wires a Forwarder over the
// 009 stream) and does NOT touch the /ds240/* paths. Production replaces the
// auditlane stub with the real 004 Topic System on HCS.
package tasking
