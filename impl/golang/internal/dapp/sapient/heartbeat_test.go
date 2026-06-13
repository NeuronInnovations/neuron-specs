package sapient

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// firstHeartbeat starts a loop and returns the first heartbeat it publishes,
// decoded both as the raw on-wire payload and via the inbound validator.
func firstHeartbeat(t *testing.T, opts HeartbeatOptions) (health.HeartbeatPayload, topic.MessageDelivery) {
	t.Helper()
	ctx := t.Context()
	from := uint64(0)
	sub, err := opts.Adapter.Subscribe(ctx, opts.StdOutRef, topic.SubscribeOpts{FromSequence: &from})
	require.NoError(t, err)

	loop, err := StartHeartbeatLoop(ctx, opts)
	require.NoError(t, err)
	require.NotNil(t, loop)

	select {
	case d, ok := <-sub:
		require.True(t, ok)
		var hb health.HeartbeatPayload
		require.NoError(t, json.Unmarshal(d.Message.Payload(), &hb))
		return hb, d
	case <-time.After(3 * time.Second):
		t.Fatal("no heartbeat published within 3s")
		return health.HeartbeatPayload{}, topic.MessageDelivery{}
	}
}

func sellerIdentity(t *testing.T) (key keylib.NeuronPrivateKey, evm, peerID, nodeID string) {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	evm = k.PublicKey().EVMAddress().Hex()
	pid, err := k.PublicKey().PeerID()
	require.NoError(t, err)
	return k, evm, pid.String(), NodeIDFromIdentity(evm)
}

func TestHeartbeat_PayloadFields(t *testing.T) {
	key, evm, peerID, nodeID := sellerIdentity(t)
	adapter := topic.NewMemoryTopicAdapter()
	ref, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: "hb-stdout"})
	require.NoError(t, err)

	hb, d := firstHeartbeat(t, HeartbeatOptions{
		Key: &key, StdOutRef: ref, Adapter: adapter, Interval: 50 * time.Millisecond,
		SellerEVM: evm, SellerPeerID: peerID, ASMNodeID: nodeID,
		FeedSource: "synthetic", CommerceMode: "off", TopicBackend: "memory",
		AgentURISha256: "deadbeef",
	})

	require.Equal(t, "heartbeat", hb.Type)
	require.Equal(t, health.RoleSeller, hb.Role)
	require.Greater(t, hb.NextHeartbeatDeadline, uint64(time.Now().Unix()), "deadline must be in the future")
	require.NotNil(t, hb.Capabilities)
	require.Contains(t, hb.Capabilities.Protocols, health.ProtocolID(ProtocolDetection))
	require.Equal(t, "synthetic", hb.Capabilities.FeedSource)
	require.Equal(t, "off", hb.Capabilities.CommerceMode)
	require.Equal(t, ProfileSAPIENTRID, hb.Capabilities.Profile)

	op := hb.Capabilities.Operational
	require.NotNil(t, op)
	require.Equal(t, CommerceServiceName, op.ServiceName) // "rid"
	require.Equal(t, evm, op.SellerEVM)
	require.Equal(t, peerID, op.SellerPeerID)
	require.Equal(t, nodeID, op.ASMNodeID)
	require.Equal(t, "memory", op.TopicBackend)
	require.Equal(t, "deadbeef", op.AgentURISha256)
	require.False(t, op.Degraded, "degraded must be false when DegradedFunc is nil")

	// The wire form is a valid, signed inbound heartbeat (seconds-domain clock).
	_, verr := health.ValidateInboundHeartbeat(d.Message, uint64(time.Now().Unix()))
	require.NoError(t, verr)
}

func TestHeartbeat_DegradedWhenBridgeDown(t *testing.T) {
	key, evm, peerID, nodeID := sellerIdentity(t)
	adapter := topic.NewMemoryTopicAdapter()
	ref, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: "hb-degraded"})
	require.NoError(t, err)

	hb, _ := firstHeartbeat(t, HeartbeatOptions{
		Key: &key, StdOutRef: ref, Adapter: adapter, Interval: 50 * time.Millisecond,
		SellerEVM: evm, SellerPeerID: peerID, ASMNodeID: nodeID,
		FeedSource:   "live",
		DegradedFunc: func() bool { return true },
	})

	require.NotNil(t, hb.Capabilities)
	require.NotNil(t, hb.Capabilities.Operational)
	require.True(t, hb.Capabilities.Operational.Degraded, "degraded must propagate from DegradedFunc")
}

func TestHeartbeat_RequiresIdentityFields(t *testing.T) {
	key, evm, peerID, nodeID := sellerIdentity(t)
	adapter := topic.NewMemoryTopicAdapter()
	ref, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: "hb-req"})
	require.NoError(t, err)
	base := HeartbeatOptions{Key: &key, StdOutRef: ref, Adapter: adapter, SellerEVM: evm, SellerPeerID: peerID, ASMNodeID: nodeID}

	noEVM := base
	noEVM.SellerEVM = ""
	_, err = StartHeartbeatLoop(t.Context(), noEVM)
	require.Error(t, err)

	noNode := base
	noNode.ASMNodeID = ""
	_, err = StartHeartbeatLoop(t.Context(), noNode)
	require.Error(t, err)

	noKey := base
	noKey.Key = nil
	_, err = StartHeartbeatLoop(t.Context(), noKey)
	require.Error(t, err)
}
