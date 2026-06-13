package health

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock TopicAdapter ---

type mockTopicAdapter struct {
	publishFn  func(ref topic.TopicRef, msg topic.TopicMessage, opts topic.PublishOpts) (topic.PublishResult, error)
	maxMsgSize uint64
	transport  topic.BackendKind
}

func (m *mockTopicAdapter) CreateTopic(_ topic.CreateTopicOpts) (topic.TopicRef, error) {
	return topic.TopicRef{}, nil
}

func (m *mockTopicAdapter) Publish(ref topic.TopicRef, msg topic.TopicMessage, opts topic.PublishOpts) (topic.PublishResult, error) {
	if m.publishFn != nil {
		return m.publishFn(ref, msg, opts)
	}
	return topic.PublishResult{Confirmed: false}, nil
}

func (m *mockTopicAdapter) Subscribe(_ context.Context, _ topic.TopicRef, _ topic.SubscribeOpts) (<-chan topic.MessageDelivery, error) {
	return nil, nil
}

func (m *mockTopicAdapter) Resolve(_ topic.TopicRef) (topic.TopicMetadata, error) {
	return topic.TopicMetadata{}, nil
}

func (m *mockTopicAdapter) MaxMessageSize() uint64 {
	return m.maxMsgSize
}

func (m *mockTopicAdapter) EstimatePublishCost(_ uint64) (topic.CostEstimate, error) {
	return topic.CostEstimate{}, nil
}

func (m *mockTopicAdapter) SupportedTransport() topic.BackendKind {
	return m.transport
}

// --- Helper ---

func newTestRef() topic.TopicRef {
	ref, _ := topic.NewTopicRef(topic.BackendHCS, "0.0.12345")
	return ref
}

func newTestKey(t *testing.T) *keylib.NeuronPrivateKey {
	t.Helper()
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	return &key
}

// --- ValidateOutboundHeartbeat Tests ---

func TestValidateOutboundHeartbeat(t *testing.T) {
	now := uint64(1700000000)

	t.Run("V-PUB-01: reject wrong payload type", func(t *testing.T) {
		p := HeartbeatPayload{
			Type:                  "not-heartbeat",
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now + 60,
			Role:                  RoleBuyer,
		}
		err := ValidateOutboundHeartbeat(p, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrInvalidPayloadType, he.Kind)
	})

	t.Run("V-PUB-02: reject wrong version", func(t *testing.T) {
		p := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               "2.0.0",
			NextHeartbeatDeadline: now + 60,
			Role:                  RoleBuyer,
		}
		err := ValidateOutboundHeartbeat(p, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrUnsupportedVersion, he.Kind)
	})

	t.Run("V-PUB-03: shutdown sentinel bypasses all delta checks", func(t *testing.T) {
		p := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: ShutdownSentinel,
			Role:                  NodeRole("invalid-role"), // would fail V-PUB-07 if not short-circuited
		}
		err := ValidateOutboundHeartbeat(p, now)
		assert.NoError(t, err, "shutdown sentinel must bypass all delta and role checks")
	})

	t.Run("V-PUB-04: reject deadline not in future", func(t *testing.T) {
		// Deadline equal to senderClock.
		p := BuildHeartbeatPayload(now, RoleBuyer)
		err := ValidateOutboundHeartbeat(p, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeadlineNotFuture, he.Kind)

		// Deadline in the past.
		p2 := BuildHeartbeatPayload(now-1, RoleBuyer)
		err2 := ValidateOutboundHeartbeat(p2, now)
		require.Error(t, err2)
		require.True(t, errors.As(err2, &he))
		assert.Equal(t, ErrDeadlineNotFuture, he.Kind)
	})

	t.Run("V-PUB-05: reject deadline too soon (delta < MinDeadlineDelta)", func(t *testing.T) {
		// delta = 5, MinDeadlineDelta = 10
		p := BuildHeartbeatPayload(now+5, RoleBuyer)
		err := ValidateOutboundHeartbeat(p, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeadlineTooSoon, he.Kind)
	})

	t.Run("V-PUB-06: reject deadline too far (delta > MaxDeadlineDelta)", func(t *testing.T) {
		// delta = MaxDeadlineDelta + 1
		p := BuildHeartbeatPayload(now+MaxDeadlineDelta+1, RoleBuyer)
		err := ValidateOutboundHeartbeat(p, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeadlineTooFar, he.Kind)
	})

	t.Run("V-PUB-07: reject unrecognized role", func(t *testing.T) {
		p := HeartbeatPayload{
			Type:                  PayloadTypeHeartbeat,
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now + 60,
			Role:                  NodeRole("unknown-role"),
		}
		err := ValidateOutboundHeartbeat(p, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrUnrecognizedRole, he.Kind)
	})

	t.Run("accept valid heartbeat with minimum delta", func(t *testing.T) {
		p := BuildHeartbeatPayload(now+MinDeadlineDelta, RoleBuyer)
		err := ValidateOutboundHeartbeat(p, now)
		assert.NoError(t, err)
	})

	t.Run("accept valid heartbeat with maximum delta", func(t *testing.T) {
		p := BuildHeartbeatPayload(now+MaxDeadlineDelta, RoleSeller)
		err := ValidateOutboundHeartbeat(p, now)
		assert.NoError(t, err)
	})

	t.Run("accept all valid roles", func(t *testing.T) {
		for _, role := range AllNodeRoles() {
			p := BuildHeartbeatPayload(now+60, role)
			err := ValidateOutboundHeartbeat(p, now)
			assert.NoError(t, err, "role %q should be accepted", role)
		}
	})

	t.Run("table-driven: validation order", func(t *testing.T) {
		tests := []struct {
			name     string
			payload  HeartbeatPayload
			clock    uint64
			wantKind HealthErrorKind
		}{
			{
				name: "type checked first",
				payload: HeartbeatPayload{
					Type:                  "wrong",
					Version:               "wrong",
					NextHeartbeatDeadline: 0,
					Role:                  "wrong",
				},
				clock:    now,
				wantKind: ErrInvalidPayloadType,
			},
			{
				name: "version checked second",
				payload: HeartbeatPayload{
					Type:                  PayloadTypeHeartbeat,
					Version:               "wrong",
					NextHeartbeatDeadline: now,
					Role:                  "wrong",
				},
				clock:    now,
				wantKind: ErrUnsupportedVersion,
			},
			{
				name: "deadline-not-future checked fourth",
				payload: HeartbeatPayload{
					Type:                  PayloadTypeHeartbeat,
					Version:               CurrentVersion,
					NextHeartbeatDeadline: now,
					Role:                  "wrong",
				},
				clock:    now,
				wantKind: ErrDeadlineNotFuture,
			},
			{
				name: "delta-too-soon checked fifth",
				payload: HeartbeatPayload{
					Type:                  PayloadTypeHeartbeat,
					Version:               CurrentVersion,
					NextHeartbeatDeadline: now + 1,
					Role:                  "wrong",
				},
				clock:    now,
				wantKind: ErrDeadlineTooSoon,
			},
			{
				name: "role checked last",
				payload: HeartbeatPayload{
					Type:                  PayloadTypeHeartbeat,
					Version:               CurrentVersion,
					NextHeartbeatDeadline: now + 60,
					Role:                  "wrong",
				},
				clock:    now,
				wantKind: ErrUnrecognizedRole,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateOutboundHeartbeat(tt.payload, tt.clock)
				require.Error(t, err)
				var he HealthError
				require.True(t, errors.As(err, &he))
				assert.Equal(t, tt.wantKind, he.Kind)
			})
		}
	})
}

// --- TrimPayload Tests ---

func TestTrimPayload(t *testing.T) {
	t.Run("payload under limit is not trimmed", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000060, RoleBuyer,
			WithPeers([]AbbreviatedAddress{"dead"}),
		)
		err := TrimPayload(&p, 1024)
		assert.NoError(t, err)
		assert.NotNil(t, p.Peers, "peers should not be trimmed when payload fits")
	})

	t.Run("peers trimmed first when over budget", func(t *testing.T) {
		caps := &Capabilities{
			NATReachability: true,
			NATType:         NATNone,
			Protocols:       []ProtocolID{"/test/v1"},
		}
		loc := &Location{Lat: 37.7749, Lon: -122.4194, Fix: Fix2D}
		peers := []AbbreviatedAddress{"dead", "beef", "cafe", "babe"}

		p := BuildHeartbeatPayload(1700000060, RoleBuyer,
			WithCapabilities(caps),
			WithLocation(loc),
			WithPeers(peers),
		)

		// Get the size without peers to set a limit that requires trimming peers.
		pCopy := p
		pCopy.Peers = nil
		dataWithout, err := json.Marshal(pCopy)
		require.NoError(t, err)

		// Set limit to something between (without peers) and (with peers).
		dataWith, err := json.Marshal(p)
		require.NoError(t, err)
		limit := (len(dataWithout) + len(dataWith)) / 2

		err = TrimPayload(&p, limit)
		assert.NoError(t, err)
		assert.Nil(t, p.Peers, "peers should be trimmed first")
		assert.NotNil(t, p.Capabilities, "capabilities should still be present")
		assert.NotNil(t, p.Location, "location should still be present")
	})

	t.Run("capabilities trimmed second", func(t *testing.T) {
		caps := &Capabilities{
			NATReachability: true,
			NATType:         NATAddressAndPortDependent,
			Protocols:       []ProtocolID{"/test/v1", "/relay/v1", "/mesh/v1"},
		}
		loc := &Location{Lat: 37.7749, Lon: -122.4194, Fix: Fix2D}

		p := BuildHeartbeatPayload(1700000060, RoleBuyer,
			WithCapabilities(caps),
			WithLocation(loc),
		)

		// Set limit to something that requires trimming capabilities but not location.
		pCopy := p
		pCopy.Capabilities = nil
		dataWithout, err := json.Marshal(pCopy)
		require.NoError(t, err)
		limit := len(dataWithout) // exactly the size without capabilities

		err = TrimPayload(&p, limit)
		assert.NoError(t, err)
		assert.Nil(t, p.Peers, "peers should be trimmed")
		assert.Nil(t, p.Capabilities, "capabilities should be trimmed")
		assert.NotNil(t, p.Location, "location should still be present")
	})

	t.Run("location trimmed third", func(t *testing.T) {
		loc := &Location{Lat: 37.7749, Lon: -122.4194, Fix: Fix2D}
		p := BuildHeartbeatPayload(1700000060, RoleBuyer,
			WithLocation(loc),
		)

		// Set limit to mandatory-only size.
		mandatory := BuildHeartbeatPayload(1700000060, RoleBuyer)
		mandatoryData, err := json.Marshal(mandatory)
		require.NoError(t, err)
		limit := len(mandatoryData)

		err = TrimPayload(&p, limit)
		assert.NoError(t, err)
		assert.Nil(t, p.Location, "location should be trimmed")
	})

	t.Run("mandatory fields too large returns ErrPayloadTooLarge", func(t *testing.T) {
		p := BuildHeartbeatPayload(1700000060, RoleBuyer)
		err := TrimPayload(&p, 10) // impossibly small
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrPayloadTooLarge, he.Kind)
	})

	t.Run("mandatory fields size is under 256 bytes budget", func(t *testing.T) {
		// Use the longest role ("validator") and a large deadline for worst case.
		p := BuildHeartbeatPayload(9999999999, RoleValidator)
		data, err := json.Marshal(p)
		require.NoError(t, err)
		assert.Less(t, len(data), MandatoryFieldsBudget,
			"mandatory-only payload must be under %d bytes, got %d", MandatoryFieldsBudget, len(data))
	})
}

// --- PublishHeartbeat Tests ---

func TestPublishHeartbeat(t *testing.T) {
	now := uint64(1700000000)

	t.Run("happy path: valid heartbeat published", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()

		var capturedMsg topic.TopicMessage
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, msg topic.TopicMessage, opts topic.PublishOpts) (topic.PublishResult, error) {
				capturedMsg = msg
				assert.Equal(t, topic.FireAndForget, opts.ConfirmationMode)
				return topic.PublishResult{
					TransactionRef: "txn-123",
					Confirmed:      false,
				}, nil
			},
		}

		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		result, err := PublishHeartbeat(payload, key, ref, adapter, now, 1)
		require.NoError(t, err)
		assert.Equal(t, "txn-123", result.TransactionRef)

		// Verify the message was signed with the correct sender address.
		expectedAddr := key.PublicKey().EVMAddress().Hex()
		assert.Equal(t, expectedAddr, capturedMsg.SenderAddress())

		// Verify the payload is valid JSON.
		var roundTripped HeartbeatPayload
		err = json.Unmarshal(capturedMsg.Payload(), &roundTripped)
		require.NoError(t, err)
		assert.Equal(t, PayloadTypeHeartbeat, roundTripped.Type)
		assert.Equal(t, uint64(now+60), roundTripped.NextHeartbeatDeadline)
	})

	t.Run("validation failure prevents publish", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
		}

		// Wrong type should fail validation.
		payload := HeartbeatPayload{
			Type:                  "not-heartbeat",
			Version:               CurrentVersion,
			NextHeartbeatDeadline: now + 60,
			Role:                  RoleBuyer,
		}
		_, err := PublishHeartbeat(payload, key, ref, adapter, now, 1)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrInvalidPayloadType, he.Kind)
	})

	t.Run("adapter error is propagated", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, _ topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				return topic.PublishResult{}, errors.New("backend unavailable")
			},
		}

		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		_, err := PublishHeartbeat(payload, key, ref, adapter, now, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "backend unavailable")
	})

	t.Run("payload trimming occurs when exceeding max message size", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()

		var capturedPayload []byte
		adapter := &mockTopicAdapter{
			maxMsgSize: 150, // tight limit that forces trimming
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, msg topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				capturedPayload = msg.Payload()
				return topic.PublishResult{Confirmed: false}, nil
			},
		}

		payload := BuildHeartbeatPayload(now+60, RoleBuyer,
			WithPeers([]AbbreviatedAddress{"dead", "beef"}),
			WithCapabilities(&Capabilities{
				NATReachability: true,
				NATType:         NATNone,
				Protocols:       []ProtocolID{"/test/v1"},
			}),
		)
		result, err := PublishHeartbeat(payload, key, ref, adapter, now, 1)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify the published payload was trimmed.
		var published HeartbeatPayload
		err = json.Unmarshal(capturedPayload, &published)
		require.NoError(t, err)
		// At minimum, peers should have been trimmed to fit the tight limit.
		// The exact trimming depends on the size calculation, so we just verify
		// the JSON size is within the adapter limit.
		assert.LessOrEqual(t, len(capturedPayload), 150)
	})
}

// --- ScheduleNextHeartbeat Tests ---

func TestScheduleNextHeartbeat(t *testing.T) {
	t.Run("confirmed result uses consensus timestamp", func(t *testing.T) {
		consensusTS := uint64(1700000010)
		result := topic.PublishResult{
			Confirmed:          true,
			ConsensusTimestamp: &consensusTS,
		}
		next := ScheduleNextHeartbeat(result, 60, 1700000000)
		assert.Equal(t, consensusTS+60, next,
			"should use consensus timestamp when confirmed")
	})

	t.Run("unconfirmed result uses wall clock", func(t *testing.T) {
		wallClock := uint64(1700000000)
		result := topic.PublishResult{
			Confirmed: false,
		}
		next := ScheduleNextHeartbeat(result, 60, wallClock)
		assert.Equal(t, wallClock+60, next,
			"should use wall clock when not confirmed")
	})

	t.Run("confirmed but nil consensus timestamp uses wall clock", func(t *testing.T) {
		wallClock := uint64(1700000000)
		result := topic.PublishResult{
			Confirmed:          true,
			ConsensusTimestamp: nil,
		}
		next := ScheduleNextHeartbeat(result, 60, wallClock)
		assert.Equal(t, wallClock+60, next,
			"should fall back to wall clock when consensus timestamp is nil")
	})

	t.Run("delta is added correctly", func(t *testing.T) {
		consensusTS := uint64(1700000000)
		result := topic.PublishResult{
			Confirmed:          true,
			ConsensusTimestamp: &consensusTS,
		}

		assert.Equal(t, consensusTS+10, ScheduleNextHeartbeat(result, 10, 0))
		assert.Equal(t, consensusTS+86400, ScheduleNextHeartbeat(result, 86400, 0))
	})
}

// --- HeartbeatPublisher Rate Limiting Tests ---

func TestHeartbeatPublisher(t *testing.T) {
	now := uint64(1700000000)

	t.Run("first publish succeeds", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, _ topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				return topic.PublishResult{Confirmed: false, TransactionRef: "tx-1"}, nil
			},
		}

		pub := NewHeartbeatPublisher(key, ref, adapter)
		payload := BuildHeartbeatPayload(now+60, RoleBuyer)
		result, err := pub.Publish(payload, now)
		require.NoError(t, err)
		assert.Equal(t, "tx-1", result.TransactionRef)
	})

	t.Run("rate limited when publishing too fast", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, _ topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				return topic.PublishResult{Confirmed: false}, nil
			},
		}

		pub := NewHeartbeatPublisher(key, ref, adapter)

		// First publish at time T.
		payload1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		_, err := pub.Publish(payload1, now)
		require.NoError(t, err)

		// Second publish at T+5 (too soon, MinDeadlineDelta=10).
		payload2 := BuildHeartbeatPayload(now+5+60, RoleBuyer)
		_, err = pub.Publish(payload2, now+5)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrRateLimited, he.Kind)
	})

	t.Run("publish succeeds after waiting MinDeadlineDelta", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, _ topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				return topic.PublishResult{Confirmed: false}, nil
			},
		}

		pub := NewHeartbeatPublisher(key, ref, adapter)

		// First publish.
		payload1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		_, err := pub.Publish(payload1, now)
		require.NoError(t, err)

		// Second publish after MinDeadlineDelta seconds.
		nextTime := now + MinDeadlineDelta
		payload2 := BuildHeartbeatPayload(nextTime+60, RoleBuyer)
		_, err = pub.Publish(payload2, nextTime)
		assert.NoError(t, err, "publish should succeed after waiting MinDeadlineDelta")
	})

	t.Run("rate limit at exact boundary (MinDeadlineDelta-1) is rejected", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, _ topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				return topic.PublishResult{Confirmed: false}, nil
			},
		}

		pub := NewHeartbeatPublisher(key, ref, adapter)

		payload1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		_, err := pub.Publish(payload1, now)
		require.NoError(t, err)

		// Attempt at MinDeadlineDelta-1 (should still be rate limited).
		nextTime := now + MinDeadlineDelta - 1
		payload2 := BuildHeartbeatPayload(nextTime+60, RoleBuyer)
		_, err = pub.Publish(payload2, nextTime)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrRateLimited, he.Kind)
	})

	t.Run("failed publish does not update lastPublishTime", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, _ topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				return topic.PublishResult{}, errors.New("backend error")
			},
		}

		pub := NewHeartbeatPublisher(key, ref, adapter)

		// First publish fails due to adapter error.
		payload1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		_, err := pub.Publish(payload1, now)
		require.Error(t, err)

		// Second publish at same time should succeed (lastPublishTime was not updated).
		adapter.publishFn = func(_ topic.TopicRef, _ topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
			return topic.PublishResult{Confirmed: false}, nil
		}
		payload2 := BuildHeartbeatPayload(now+60, RoleBuyer)
		_, err = pub.Publish(payload2, now)
		assert.NoError(t, err, "publish should succeed because previous failure did not update lastPublishTime")
	})
}

// --- Phase 5 (T046): Shutdown Sentinel End-to-End Publisher Test ---

func TestPublishShutdownSentinel(t *testing.T) {
	now := uint64(1700000000)

	t.Run("T046: shutdown sentinel validates, bypasses delta checks, and publishes", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()

		var capturedPayload []byte
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, msg topic.TopicMessage, opts topic.PublishOpts) (topic.PublishResult, error) {
				capturedPayload = msg.Payload()
				return topic.PublishResult{Confirmed: false, TransactionRef: "shutdown-tx"}, nil
			},
		}

		// Build payload with nextHeartbeatDeadline = 0 (shutdown sentinel).
		payload := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
		payload.NextHeartbeatDeadline = ShutdownSentinel

		// Step 1: Validate via ValidateOutboundHeartbeat — V-PUB-03 bypass must accept.
		err := ValidateOutboundHeartbeat(payload, now)
		assert.NoError(t, err, "V-PUB-03: shutdown sentinel must bypass all delta checks")

		// Step 2: Publish via PublishHeartbeat.
		result, err := PublishHeartbeat(payload, key, ref, adapter, now, 1)
		require.NoError(t, err, "publish must succeed with shutdown sentinel")
		assert.Equal(t, "shutdown-tx", result.TransactionRef)

		// Step 3: Verify published payload contains deadline=0.
		var published HeartbeatPayload
		err = json.Unmarshal(capturedPayload, &published)
		require.NoError(t, err)
		assert.Equal(t, ShutdownSentinel, published.NextHeartbeatDeadline,
			"published payload must carry shutdown sentinel (deadline=0)")
		assert.Equal(t, PayloadTypeHeartbeat, published.Type)
		assert.Equal(t, CurrentVersion, published.Version)
	})
}

// --- Phase 6 (T053): Boundary Delta Tests ---

func TestValidateOutboundHeartbeatBoundaryDeltas(t *testing.T) {
	now := uint64(1700000000)

	t.Run("T053: accept exactly delta=MinDeadlineDelta (10)", func(t *testing.T) {
		p := BuildHeartbeatPayload(now+MinDeadlineDelta, RoleBuyer)
		err := ValidateOutboundHeartbeat(p, now)
		assert.NoError(t, err, "delta exactly at MinDeadlineDelta must be accepted")
	})

	t.Run("T053: accept exactly delta=MaxDeadlineDelta (86400)", func(t *testing.T) {
		p := BuildHeartbeatPayload(now+MaxDeadlineDelta, RoleBuyer)
		err := ValidateOutboundHeartbeat(p, now)
		assert.NoError(t, err, "delta exactly at MaxDeadlineDelta must be accepted")
	})

	t.Run("T053: reject delta=MinDeadlineDelta-1 (9)", func(t *testing.T) {
		p := BuildHeartbeatPayload(now+MinDeadlineDelta-1, RoleBuyer)
		err := ValidateOutboundHeartbeat(p, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeadlineTooSoon, he.Kind,
			"delta=9 must produce ErrDeadlineTooSoon")
	})

	t.Run("T053: reject delta=MaxDeadlineDelta+1 (86401)", func(t *testing.T) {
		p := BuildHeartbeatPayload(now+MaxDeadlineDelta+1, RoleBuyer)
		err := ValidateOutboundHeartbeat(p, now)
		require.Error(t, err)
		var he HealthError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, ErrDeadlineTooFar, he.Kind,
			"delta=86401 must produce ErrDeadlineTooFar")
	})
}

// --- Phase 6 (T054): Full-Range Delta Table-Driven Tests ---

func TestValidateOutboundHeartbeatFullRangeDeltas(t *testing.T) {
	now := uint64(1700000000)

	t.Run("T054: table-driven full range deltas all pass", func(t *testing.T) {
		deltas := []uint64{10, 30, 60, 300, 3600, 43200, 86400}
		for _, delta := range deltas {
			t.Run("delta="+string(rune('0'+delta/10000)), func(t *testing.T) {
				p := BuildHeartbeatPayload(now+delta, RoleBuyer)
				err := ValidateOutboundHeartbeat(p, now)
				assert.NoError(t, err, "delta=%d must pass validation", delta)
			})
		}
	})
}

// --- Phase 6 (T055): Cadence Change Tests ---

func TestPublishCadenceChanges(t *testing.T) {
	now := uint64(1700000000)

	t.Run("T055: cadence changes across publishes are accepted", func(t *testing.T) {
		key := newTestKey(t)
		ref := newTestRef()
		adapter := &mockTopicAdapter{
			maxMsgSize: 4096,
			transport:  topic.BackendHCS,
			publishFn: func(_ topic.TopicRef, _ topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
				return topic.PublishResult{Confirmed: false}, nil
			},
		}

		pub := NewHeartbeatPublisher(key, ref, adapter)

		// Publish with delta=60.
		p1 := BuildHeartbeatPayload(now+60, RoleBuyer)
		_, err := pub.Publish(p1, now)
		require.NoError(t, err, "first publish with delta=60 must succeed")

		// Publish with delta=300 (cadence change).
		clock2 := now + MinDeadlineDelta
		p2 := BuildHeartbeatPayload(clock2+300, RoleBuyer)
		_, err = pub.Publish(p2, clock2)
		require.NoError(t, err, "second publish with delta=300 must succeed (cadence change allowed)")

		// Publish with delta=15 (another cadence change to shorter interval).
		clock3 := clock2 + MinDeadlineDelta
		p3 := BuildHeartbeatPayload(clock3+15, RoleBuyer)
		_, err = pub.Publish(p3, clock3)
		assert.NoError(t, err, "third publish with delta=15 must succeed (cadence change to shorter interval)")
	})
}

// --- Phase 6 (T056): ScheduleNextHeartbeat Boundary Tests ---

func TestScheduleNextHeartbeatBoundaries(t *testing.T) {
	t.Run("T056: minimum delta with confirmed result", func(t *testing.T) {
		consensusTS := uint64(1700000010)
		result := topic.PublishResult{
			Confirmed:          true,
			ConsensusTimestamp: &consensusTS,
		}
		next := ScheduleNextHeartbeat(result, MinDeadlineDelta, 1700000000)
		assert.Equal(t, consensusTS+MinDeadlineDelta, next,
			"schedule with minimum delta must use consensus + MinDeadlineDelta")
	})

	t.Run("T056: maximum delta with confirmed result", func(t *testing.T) {
		consensusTS := uint64(1700000010)
		result := topic.PublishResult{
			Confirmed:          true,
			ConsensusTimestamp: &consensusTS,
		}
		next := ScheduleNextHeartbeat(result, MaxDeadlineDelta, 1700000000)
		assert.Equal(t, consensusTS+MaxDeadlineDelta, next,
			"schedule with maximum delta must use consensus + MaxDeadlineDelta")
	})

	t.Run("T056: minimum delta with unconfirmed result", func(t *testing.T) {
		wallClock := uint64(1700000000)
		result := topic.PublishResult{Confirmed: false}
		next := ScheduleNextHeartbeat(result, MinDeadlineDelta, wallClock)
		assert.Equal(t, wallClock+MinDeadlineDelta, next,
			"schedule with minimum delta and unconfirmed result must use wall clock")
	})

	t.Run("T056: maximum delta with unconfirmed result", func(t *testing.T) {
		wallClock := uint64(1700000000)
		result := topic.PublishResult{Confirmed: false}
		next := ScheduleNextHeartbeat(result, MaxDeadlineDelta, wallClock)
		assert.Equal(t, wallClock+MaxDeadlineDelta, next,
			"schedule with maximum delta and unconfirmed result must use wall clock")
	})
}

// --- Phase 6 (T058-T059): Boundary Comparison Operator Verification ---

func TestBoundaryComparisonOperators(t *testing.T) {
	now := uint64(1700000000)

	// T058: Verify V-PUB-05 uses < not <= (delta < MinDeadlineDelta rejects, delta == MinDeadlineDelta accepts).
	t.Run("T058: V-PUB-05 uses strict less-than (< not <=)", func(t *testing.T) {
		// delta = MinDeadlineDelta - 1 = 9 must be rejected.
		pReject := BuildHeartbeatPayload(now+MinDeadlineDelta-1, RoleBuyer)
		err := ValidateOutboundHeartbeat(pReject, now)
		require.Error(t, err, "delta=9 must be rejected (< MinDeadlineDelta)")

		// delta = MinDeadlineDelta = 10 must be accepted.
		pAccept := BuildHeartbeatPayload(now+MinDeadlineDelta, RoleBuyer)
		err = ValidateOutboundHeartbeat(pAccept, now)
		assert.NoError(t, err, "delta=10 must be accepted (== MinDeadlineDelta)")
	})

	// T059: Verify V-PUB-06 uses > not >= (delta > MaxDeadlineDelta rejects, delta == MaxDeadlineDelta accepts).
	t.Run("T059: V-PUB-06 uses strict greater-than (> not >=)", func(t *testing.T) {
		// delta = MaxDeadlineDelta + 1 = 86401 must be rejected.
		pReject := BuildHeartbeatPayload(now+MaxDeadlineDelta+1, RoleBuyer)
		err := ValidateOutboundHeartbeat(pReject, now)
		require.Error(t, err, "delta=86401 must be rejected (> MaxDeadlineDelta)")

		// delta = MaxDeadlineDelta = 86400 must be accepted.
		pAccept := BuildHeartbeatPayload(now+MaxDeadlineDelta, RoleBuyer)
		err = ValidateOutboundHeartbeat(pAccept, now)
		assert.NoError(t, err, "delta=86400 must be accepted (== MaxDeadlineDelta)")
	})
}
