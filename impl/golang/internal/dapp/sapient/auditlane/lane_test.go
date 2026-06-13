package auditlane

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

func taskMsg(node, dest string) *sapientpb.SapientMessage {
	return &sapientpb.SapientMessage{
		NodeId:        proto.String(node),
		DestinationId: proto.String(dest),
		Content: &sapientpb.SapientMessage_Task{Task: &sapientpb.Task{
			TaskId:  proto.String("01J9ZSN0W5K3QH7Y8B4F2C6XME"),
			Control: sapientpb.Task_CONTROL_STOP.Enum(),
		}},
	}
}

func TestMemoryLane_PublishSubscribeRoundTrip(t *testing.T) {
	ctx := t.Context()
	lane := NewMemoryLane()
	defer lane.Close()

	ch := Channel{ASMNodeID: "asm-1", Role: RoleStdIn}
	sub, err := lane.Subscribe(ctx, ch)
	require.NoError(t, err)

	require.NoError(t, lane.Publish(ctx, ch, taskMsg("buyer-A", "asm-1")))

	select {
	case got := <-sub:
		require.Equal(t, "buyer-A", got.GetNodeId())
		require.Equal(t, sapientpb.Task_CONTROL_STOP, got.GetTask().GetControl())
	case <-time.After(2 * time.Second):
		t.Fatal("no message received")
	}
}

func TestMemoryLane_ChannelIsolation(t *testing.T) {
	ctx := t.Context()
	lane := NewMemoryLane()
	defer lane.Close()

	in, err := lane.Subscribe(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdIn})
	require.NoError(t, err)
	out, err := lane.Subscribe(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdOut})
	require.NoError(t, err)

	// Publish to stdIn only.
	require.NoError(t, lane.Publish(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdIn}, taskMsg("b", "asm-1")))

	select {
	case <-in:
	case <-time.After(2 * time.Second):
		t.Fatal("stdIn subscriber missed its message")
	}
	select {
	case <-out:
		t.Fatal("stdOut subscriber wrongly received a stdIn message")
	case <-time.After(150 * time.Millisecond):
	}
}

func TestMemoryLane_Isolation_DifferentASM(t *testing.T) {
	ctx := t.Context()
	lane := NewMemoryLane()
	defer lane.Close()

	a, err := lane.Subscribe(ctx, Channel{ASMNodeID: "asm-A", Role: RoleStdIn})
	require.NoError(t, err)
	require.NoError(t, lane.Publish(ctx, Channel{ASMNodeID: "asm-B", Role: RoleStdIn}, taskMsg("b", "asm-B")))
	select {
	case <-a:
		t.Fatal("asm-A subscriber received an asm-B message")
	case <-time.After(150 * time.Millisecond):
	}
}

func TestMemoryLane_ConcurrentPublishers(t *testing.T) {
	ctx := t.Context()
	lane := NewMemoryLane()
	defer lane.Close()

	ch := Channel{ASMNodeID: "asm-1", Role: RoleStdOut}
	sub, err := lane.Subscribe(ctx, ch)
	require.NoError(t, err)

	const n = 50
	for range n {
		go func() { _ = lane.Publish(ctx, ch, taskMsg("b", "asm-1")) }()
	}
	got := 0
	deadline := time.After(3 * time.Second)
	for got < n {
		select {
		case <-sub:
			got++
		case <-deadline:
			t.Fatalf("only received %d/%d", got, n)
		}
	}
}

func TestFileLane_ReplayFromStartThenTail(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "lane.ndjson")
	lane := NewFileLane(path)
	defer lane.Close()

	ch := Channel{ASMNodeID: "asm-1", Role: RoleStdOut}
	// Publish BEFORE subscribing — FileLane replays from the start.
	require.NoError(t, lane.Publish(ctx, ch, taskMsg("seller", "asm-1")))

	sub, err := lane.Subscribe(ctx, ch)
	require.NoError(t, err)
	select {
	case got := <-sub:
		require.Equal(t, "seller", got.GetNodeId())
	case <-time.After(2 * time.Second):
		t.Fatal("replay did not deliver the pre-existing record")
	}

	// Now tail a later append.
	require.NoError(t, lane.Publish(ctx, ch, taskMsg("seller2", "asm-1")))
	select {
	case got := <-sub:
		require.Equal(t, "seller2", got.GetNodeId())
	case <-time.After(2 * time.Second):
		t.Fatal("tail did not deliver the appended record")
	}
}

func TestFileLane_CloseStopsSubscriber(t *testing.T) {
	// t.Context() stays alive during the test, so the subscriber can only stop via
	// lane.Close() — proves Close() terminates subscribers (no goroutine leak).
	path := filepath.Join(t.TempDir(), "lane.ndjson")
	lane := NewFileLane(path)
	sub, err := lane.Subscribe(t.Context(), Channel{ASMNodeID: "asm-1", Role: RoleStdOut})
	require.NoError(t, err)
	require.NoError(t, lane.Close())
	select {
	case _, ok := <-sub:
		require.False(t, ok, "lane.Close() must close the subscriber channel")
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not stop the subscriber")
	}
}

func TestFileLane_ChannelFilter(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "lane.ndjson")
	lane := NewFileLane(path)
	defer lane.Close()

	require.NoError(t, lane.Publish(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdIn}, taskMsg("x", "asm-1")))
	sub, err := lane.Subscribe(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdOut})
	require.NoError(t, err)
	select {
	case <-sub:
		t.Fatal("stdOut subscriber received a stdIn record")
	case <-time.After(300 * time.Millisecond):
	}
}
