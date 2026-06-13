package tasking

import (
	"context"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/auditlane"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// feedSourcePrefix is the StatusReport.status_value prefix carrying feedSource (017
// FR-R-E02 / CL-R9). SAPIENT v2.0 has no native field, so it rides as a native
// status[] entry under the namespaced "neuron." prefix.
const feedSourcePrefix = "neuron.feedSource="

// validFeedSources is the FR-R-E02 value domain.
var validFeedSources = map[string]bool{"live": true, "replay": true, "synthetic": true, "placeholder": true}

// ValidFeedSource reports whether s is in the FR-R-E02 enum
// {live, replay, synthetic, placeholder}.
func ValidFeedSource(s string) bool { return validFeedSources[s] }

// FeedSourceStatusValue renders a feedSource value for a StatusReport.status[]
// entry (e.g. "neuron.feedSource=synthetic"), per 017 CL-R9.
func FeedSourceStatusValue(feedSource string) string { return feedSourcePrefix + feedSource }

// ParseFeedSource extracts the feedSource from a status_value, reporting whether it
// matched the convention.
func ParseFeedSource(statusValue string) (string, bool) {
	if !strings.HasPrefix(statusValue, feedSourcePrefix) {
		return "", false
	}
	return strings.TrimPrefix(statusValue, feedSourcePrefix), true
}

// neuron.* status_value prefixes for the operational telemetry the SAPIENT
// StatusReport carries alongside feedSource. Like feedSource they ride as native
// status[] entries under the "neuron." namespace (SAPIENT v2.0 has no dedicated
// fields). Kept DISTINCT from the spec-005 heartbeat (015 FR-S31): the heartbeat
// is Neuron liveness; these are sensor/runtime telemetry.
const (
	framesForwardedPrefix = "neuron.framesForwarded="
	bridgePrefix          = "neuron.bridge="
)

// FramesForwardedStatusValue renders the count of DetectionReports forwarded so
// far (e.g. "neuron.framesForwarded=128").
func FramesForwardedStatusValue(n uint64) string {
	return framesForwardedPrefix + strconv.FormatUint(n, 10)
}

// ParseFramesForwarded extracts the forwarded-frame count from a status_value.
func ParseFramesForwarded(statusValue string) (uint64, bool) {
	if !strings.HasPrefix(statusValue, framesForwardedPrefix) {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(statusValue, framesForwardedPrefix), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// BridgeStatusValue renders the upstream bridge connection state
// ("neuron.bridge=connected" | "neuron.bridge=disconnected").
func BridgeStatusValue(connected bool) string {
	if connected {
		return bridgePrefix + "connected"
	}
	return bridgePrefix + "disconnected"
}

// ParseBridgeConnected extracts the bridge state from a status_value.
func ParseBridgeConnected(statusValue string) (connected, ok bool) {
	if !strings.HasPrefix(statusValue, bridgePrefix) {
		return false, false
	}
	return strings.TrimPrefix(statusValue, bridgePrefix) == "connected", true
}

// StatusSnapshot is the runtime telemetry the seller feeds into each StatusReport
// via Options.StatusInputs. BridgeConnected/Degraded reflect the upstream feed
// health the seller tracks; FramesForwarded is owned by the Manager itself.
type StatusSnapshot struct {
	BridgeConnected bool
	Degraded        bool
}

// commandName returns a stable label for a node-global Task_Command oneof arm, or
// "" if the task carries no command. Used to refuse node-global tasks (FR-S60).
func commandName(cmd *sapientpb.Task_Command) string {
	if cmd == nil {
		return ""
	}
	switch cmd.GetCommand().(type) {
	case *sapientpb.Task_Command_Request:
		return "request"
	case *sapientpb.Task_Command_DetectionThreshold:
		return "detection_threshold"
	case *sapientpb.Task_Command_DetectionReportRate:
		return "detection_report_rate"
	case *sapientpb.Task_Command_ClassificationThreshold:
		return "classification_threshold"
	case *sapientpb.Task_Command_ModeChange:
		return "mode_change"
	case *sapientpb.Task_Command_LookAt:
		return "look_at"
	case *sapientpb.Task_Command_MoveTo:
		return "move_to"
	case *sapientpb.Task_Command_Patrol:
		return "patrol"
	case *sapientpb.Task_Command_Follow:
		return "follow"
	default:
		return "unknown_command"
	}
}

// IssueControl is the HLDMM-side helper: it publishes a Task{control} addressed to
// asmNodeID (DestinationId) from fromNodeID (NodeId = the session key) onto the
// ASM's stdIn, and returns the published message. Reused by cmd/sapient-task and
// tests. idFunc may be nil (defaults to DefaultIDFunc).
func IssueControl(ctx context.Context, lane auditlane.Lane, asmNodeID, fromNodeID string, control sapientpb.Task_Control, idFunc func() string) (*sapientpb.SapientMessage, error) {
	if idFunc == nil {
		idFunc = DefaultIDFunc
	}
	msg := &sapientpb.SapientMessage{
		NodeId:        proto.String(fromNodeID),
		DestinationId: proto.String(asmNodeID),
		Content: &sapientpb.SapientMessage_Task{Task: &sapientpb.Task{
			TaskId:  proto.String(idFunc()),
			Control: control.Enum(),
		}},
	}
	if err := lane.Publish(ctx, auditlane.Channel{ASMNodeID: asmNodeID, Role: auditlane.RoleStdIn}, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
