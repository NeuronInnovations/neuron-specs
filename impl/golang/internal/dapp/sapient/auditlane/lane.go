package auditlane

import (
	"context"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// Role is a 004 channel role for an ASM, per spec 015 §B.
type Role string

const (
	// RoleStdIn — inbound control to the ASM: Task, AlertAck (the consumer writes,
	// the ASM reads).
	RoleStdIn Role = "stdIn"
	// RoleStdOut — routine ASM output: TaskAck, StatusReport (the ASM writes, the
	// consumer reads).
	RoleStdOut Role = "stdOut"
	// RoleStdErr — exceptional ASM output: Alert, Error (reserved; not used by the
	// Tasking/StatusReport prototype).
	RoleStdErr Role = "stdErr"
)

// Channel identifies one ASM's 004 channel. In the real Topic System this maps to
// a TopicRef; here it is the (ASM node_id, role) pair the lane fans out on.
type Channel struct {
	ASMNodeID string
	Role      Role
}

func (c Channel) key() string { return c.ASMNodeID + "\x00" + string(c.Role) }

// Lane is the auditable-lane abstraction. Implementations carry whole
// SapientMessages (full-message anchoring, CL-3/FR-S32) between an ASM and its
// consumers over a LOCAL transport (never HCS/testnet).
type Lane interface {
	// Publish appends msg to the channel. The message is not mutated; callers may
	// reuse it after the call returns.
	Publish(ctx context.Context, ch Channel, msg *sapientpb.SapientMessage) error
	// Subscribe returns a stream of messages published to the channel. The stream
	// closes when ctx is cancelled or the lane is closed.
	Subscribe(ctx context.Context, ch Channel) (<-chan *sapientpb.SapientMessage, error)
	// Close releases any underlying resources (open files, subscriber channels).
	Close() error
}
