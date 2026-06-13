package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/auditlane"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

func TestSetupAuditLane_UnknownBackend(t *testing.T) {
	nk, _ := sellerTestKey(t)
	_, err := setupAuditLane(context.Background(), "potato", "", "deadbeef", nk, Deps{}, log.New(io.Discard, "", 0))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown --auditlane-backend")
}

// file backend + empty --control-lane = data-only (no control plane, no topic) —
// the byte-identical original behaviour.
func TestSetupAuditLane_FileEmptyIsDataOnly(t *testing.T) {
	nk, _ := sellerTestKey(t)
	s, err := setupAuditLane(context.Background(), auditBackendFile, "", "deadbeef", nk, Deps{}, log.New(io.Discard, "", 0))
	require.NoError(t, err)
	require.Nil(t, s.lane)
	require.False(t, s.topicBacked)
	require.Empty(t, s.transport, "file mode must not advertise a topic transport")
}

// memory backend yields a topic-backed lane with the transport disclosure + stdOut
// ref, and the lane round-trips a SapientMessage.
func TestSetupAuditLane_MemoryTopicBacked(t *testing.T) {
	nk, _ := sellerTestKey(t)
	s, err := setupAuditLane(context.Background(), auditBackendMemory, "", "deadbeef", nk, Deps{}, log.New(io.Discard, "", 0))
	require.NoError(t, err)
	require.True(t, s.topicBacked)
	require.Equal(t, "memory", s.transport)
	require.NotEmpty(t, s.topicConfig["stdOut"]["topicId"])
	require.NotNil(t, s.adapter)
	defer s.lane.Close()
}

// --auditlane-backend=hcs with no operator env fails fast (no silent fallback).
func TestAuditlaneHCS_RequiresEnv(t *testing.T) {
	t.Setenv("HEDERA_OPERATOR_ID", "")
	t.Setenv("HEDERA_OPERATOR_KEY", "")
	nk, _ := sellerTestKey(t)
	err := run([]string{"--auditlane-backend", "hcs", "--register-only", "--key-hex", nk.Hex()}, Deps{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "HEDERA_OPERATOR")
}

// TestAuditlaneMemory_EndToEnd runs the seller in memory audit mode against a fake
// bridge + a buyer that drains the detection stream, and proves the new control/
// evidence plane: the Agent Card advertises the memory transport + topic IDs, and
// the seller publishes a 005 heartbeat, a StatusReport, and a Registration on its
// stdOut topic.
func TestAuditlaneMemory_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bus := topic.NewMemoryTopicAdapter()
	sellerNK, _ := sellerTestKey(t)
	sellerHex := sellerNK.Hex()
	nodeID := sapient.NodeIDFromIdentity(sellerNK.PublicKey().EVMAddress().Hex())
	cardPath := filepath.Join(t.TempDir(), "card.json")

	bridge, err := sapient.ServeFeed("127.0.0.1:0")
	require.NoError(t, err)
	defer bridge.Close()
	go func() {
		for ctx.Err() == nil {
			_ = bridge.Publish(&sapientpb.SapientMessage{NodeId: proto.String("bridge")})
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Buyer host that accepts the seller's dial + drains frames.
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerECDSA, err := buyerKey.ToBlockchainKey()
	require.NoError(t, err)
	buyerHost, err := delivery.NewLibp2pHost(buyerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()
	// Drain a handful of frames then reset — that breaks the seller's pump so its
	// run() returns (the seller owns its own signal context; the test cannot cancel
	// it directly). Registration + the first heartbeat publish before the pump even
	// starts, and the 60ms StatusReport fires well within these frames.
	buyerHost.SetStreamHandler(protocol.ID(sapient.ProtocolDetection), func(stream libp2pnetwork.Stream) {
		r := delivery.NewFrameReader(stream)
		n := 0
		for {
			if _, rerr := r.ReadFrame(); rerr != nil {
				return
			}
			if n++; n >= 12 {
				_ = stream.Reset()
				return
			}
		}
	})
	buyerMA := buyerHost.Addrs()[0].String() + "/p2p/" + buyerHost.ID().String()

	sellerErr := make(chan error, 1)
	go func() {
		sellerErr <- run([]string{
			"--auditlane-backend", "memory",
			"--key-hex", sellerHex,
			"--bridge-addr", bridge.Addr(),
			"--buyer", buyerMA,
			"--agent-card-out", cardPath,
			"--feed-source", "synthetic",
			"--heartbeat-interval", "60ms",
			"--status-interval", "60ms",
		}, Deps{TopicAdapter: bus})
	}()

	// The card lands with the memory transport + a stdOut topic id.
	var stdOutID string
	require.Eventually(t, func() bool {
		b, rerr := os.ReadFile(cardPath)
		if rerr != nil {
			return false
		}
		uri, perr := registry.AgentURIFromJSON(string(b))
		if perr != nil {
			return false
		}
		for _, ts := range uri.TopicServices() {
			if ts.Channel == "stdOut" {
				require.Equal(t, "memory", ts.Transport)
				if id, ok := ts.Config["topicId"].(string); ok {
					stdOutID = id
				}
			}
		}
		return stdOutID != ""
	}, 15*time.Second, 50*time.Millisecond, "card with stdOut topic id never appeared")

	stdOutRef, err := topic.NewTopicRef(bus.SupportedTransport(), stdOutID)
	require.NoError(t, err)

	// SAPIENT control messages (StatusReport + Registration) via a TopicLane.
	subLane, err := auditlane.NewTopicLane(bus, &buyerKey, map[auditlane.Role]topic.TopicRef{auditlane.RoleStdOut: stdOutRef})
	require.NoError(t, err)
	defer subLane.Close()
	sapSub, err := subLane.Subscribe(ctx, auditlane.Channel{ASMNodeID: nodeID, Role: auditlane.RoleStdOut})
	require.NoError(t, err)

	// The 005 heartbeat (JSON, demuxed out of the lane) via a raw subscription.
	from := uint64(0)
	rawSub, err := bus.Subscribe(ctx, stdOutRef, topic.SubscribeOpts{FromSequence: &from})
	require.NoError(t, err)

	var sawStatus, sawRegistration, sawHeartbeat bool
	deadline := time.After(10 * time.Second)
	for !sawStatus || !sawRegistration || !sawHeartbeat {
		select {
		case m := <-sapSub:
			if m.GetStatusReport() != nil {
				sawStatus = true
			}
			if m.GetRegistration() != nil {
				sawRegistration = true
			}
		case d := <-rawSub:
			var hb health.HeartbeatPayload
			if json.Unmarshal(d.Message.Payload(), &hb) == nil && hb.Type == "heartbeat" &&
				hb.Capabilities != nil && hb.Capabilities.Operational != nil &&
				hb.Capabilities.Operational.ASMNodeID == nodeID {
				sawHeartbeat = true
			}
		case <-deadline:
			t.Fatalf("missing signals: status=%v registration=%v heartbeat=%v", sawStatus, sawRegistration, sawHeartbeat)
		}
	}

	// The buyer's stream reset makes the seller's pump break and run() return clean.
	select {
	case serr := <-sellerErr:
		require.NoError(t, serr, "seller run() exits clean after the buyer closes the stream")
	case <-time.After(10 * time.Second):
		t.Fatal("seller did not exit after the buyer closed the stream")
	}
}
