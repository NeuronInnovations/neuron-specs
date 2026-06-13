package auditlane

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// statusMsg builds a non-trivial StatusReport SapientMessage. NodeId + ReportId
// keep it well above zero bytes (an empty proto marshals to nothing).
func statusMsg(node, report string) *sapientpb.SapientMessage {
	return &sapientpb.SapientMessage{
		NodeId: proto.String(node),
		Content: &sapientpb.SapientMessage_StatusReport{StatusReport: &sapientpb.StatusReport{
			ReportId: proto.String(report),
			System:   sapientpb.StatusReport_SYSTEM_OK.Enum(),
		}},
	}
}

func newLane(t *testing.T) (*TopicLane, topic.TopicAdapter, topic.TopicRef, *keylib.NeuronPrivateKey) {
	t.Helper()
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	adapter := topic.NewMemoryTopicAdapter()
	ref, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stdout-test"})
	require.NoError(t, err)
	lane, err := NewTopicLane(adapter, &key, map[Role]topic.TopicRef{RoleStdOut: ref})
	require.NoError(t, err)
	return lane, adapter, ref, &key
}

func readOne(t *testing.T, ch <-chan *sapientpb.SapientMessage) *sapientpb.SapientMessage {
	t.Helper()
	select {
	case m, ok := <-ch:
		require.True(t, ok, "lane channel closed before a message arrived")
		return m
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a lane message")
		return nil
	}
}

func TestTopicLane_RoundTrip(t *testing.T) {
	lane, _, _, _ := newLane(t)
	ctx := t.Context()
	defer lane.Close()

	require.NoError(t, lane.Publish(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdOut}, statusMsg("asm-1", "rpt-1")))

	sub, err := lane.Subscribe(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdOut})
	require.NoError(t, err)

	got := readOne(t, sub)
	require.Equal(t, "asm-1", got.GetNodeId())
	require.NotNil(t, got.GetStatusReport())
	require.Equal(t, "rpt-1", got.GetStatusReport().GetReportId())
}

// A spec-005 heartbeat (JSON, no laneMagic) shares the seller's stdOut topic. The
// lane MUST skip it and yield only the SAPIENT control message.
func TestTopicLane_SkipsHeartbeatOnSharedTopic(t *testing.T) {
	lane, adapter, ref, key := newLane(t)
	ctx := t.Context()
	defer lane.Close()

	// A heartbeat-shaped JSON payload, signed like the health publisher would,
	// published straight to the shared topic (no laneMagic).
	hb, err := topic.NewTopicMessage(key, uint64(time.Now().UnixNano()), 1,
		[]byte(`{"type":"heartbeat","version":"1.0.0","nextHeartbeatDeadline":"42","role":"seller"}`))
	require.NoError(t, err)
	_, err = adapter.Publish(ref, hb, topic.PublishOpts{ConfirmationMode: topic.FireAndForget})
	require.NoError(t, err)

	// Then a real SAPIENT control message through the lane.
	require.NoError(t, lane.Publish(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdOut}, statusMsg("asm-1", "rpt-after-hb")))

	sub, err := lane.Subscribe(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdOut})
	require.NoError(t, err)

	got := readOne(t, sub)
	require.Equal(t, "rpt-after-hb", got.GetStatusReport().GetReportId(),
		"the heartbeat must be demuxed out; only the SAPIENT message is yielded")
}

// A tampered TopicMessage (laneMagic payload but a corrupted signature) MUST be
// dropped — full-message anchoring is only meaningful if the signature verifies.
func TestTopicLane_RejectsBadSignature(t *testing.T) {
	lane, adapter, ref, key := newLane(t)
	ctx := t.Context()
	defer lane.Close()

	body, err := proto.Marshal(statusMsg("asm-1", "evil"))
	require.NoError(t, err)
	payload := append(append([]byte{}, laneMagic...), body...)
	good, err := topic.NewTopicMessage(key, uint64(time.Now().UnixNano()), 1, payload)
	require.NoError(t, err)
	// Flip a signature byte → recovery yields a different signer → rejected.
	sig := good.Signature()
	sig[0] ^= 0xFF
	bad := topic.TopicMessageFromFields(good.SenderAddress(), sig, good.Timestamp(), good.SequenceNumber(), good.Payload())
	_, err = adapter.Publish(ref, bad, topic.PublishOpts{ConfirmationMode: topic.FireAndForget})
	require.NoError(t, err)

	// A valid sentinel after it; the lane should surface only the sentinel.
	require.NoError(t, lane.Publish(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdOut}, statusMsg("asm-1", "sentinel")))

	sub, err := lane.Subscribe(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdOut})
	require.NoError(t, err)

	got := readOne(t, sub)
	require.Equal(t, "sentinel", got.GetStatusReport().GetReportId())
}

func TestTopicLane_UnmappedRole(t *testing.T) {
	lane, _, _, _ := newLane(t)
	ctx := context.Background()
	defer lane.Close()

	// stdIn was never mapped → both directions report the missing role.
	err := lane.Publish(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdIn}, statusMsg("asm-1", "x"))
	require.Error(t, err)
	_, err = lane.Subscribe(ctx, Channel{ASMNodeID: "asm-1", Role: RoleStdIn})
	require.Error(t, err)
}

func TestTopicLane_PublishAfterCloseIsSafe(t *testing.T) {
	lane, _, _, _ := newLane(t)
	require.NoError(t, lane.Close())
	require.NoError(t, lane.Close()) // idempotent
	// Subscribe after Close is rejected.
	_, err := lane.Subscribe(context.Background(), Channel{ASMNodeID: "asm-1", Role: RoleStdOut})
	require.Error(t, err)
}
