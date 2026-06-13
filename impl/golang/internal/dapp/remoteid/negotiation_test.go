package remoteid

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Test_FR_P06_NegotiationEnvelopes groups every assertion that proves a
// buyer→seller serviceRequest and the seller's serviceResponse round-trip
// cleanly over the memory topic adapter.
func Test_FR_P06_NegotiationEnvelopes(t *testing.T) {
	t.Run("service-request-round-trip", TestBuildServiceRequest_RoundTrip)
	t.Run("service-response-accept", TestServiceResponse_Accept)
	t.Run("missing-buyer-stdin-rejected", TestBuildServiceRequest_RequiresBuyerStdIn)
	t.Run("missing-request-id-rejected", TestBuildServiceRequest_RequiresRequestID)
	t.Run("receive-filters-by-type", TestReceiveTypedPayload_FiltersByType)
	t.Run("receive-filters-by-request-id", TestReceiveTypedPayload_FiltersByRequestID)
	t.Run("receive-honors-ctx-cancel", TestReceiveTypedPayload_HonorsContextCancel)
}

func TestBuildServiceRequest_RoundTrip(t *testing.T) {
	t.Parallel()
	req, err := BuildServiceRequest(ServiceRequestOptions{
		RequestID:  "req-1",
		BuyerStdIn: "buyer-stdin-locator",
	})
	require.NoError(t, err)
	assert.Equal(t, payment.PayloadServiceRequest, req.Type)
	assert.Equal(t, PayloadVersion, req.Version)
	assert.Equal(t, "req-1", req.RequestID)
	assert.Equal(t, P2PServiceName, req.ServiceRef, "default ServiceRef is the remote-id p2p service name")
	assert.Equal(t, DefaultSettlementBinding, req.SettlementBinding)
	assert.Equal(t, "1", req.ProposedAmount, "Stage-2 default is the smallest positive unit (MemoryEscrow rejects 0)")
	assert.Equal(t, "USDC", req.ProposedCurrency)
	assert.Equal(t, "0", req.ProposedInterval)
	assert.Equal(t, "buyer-stdin-locator", req.BuyerStdIn)
}

func TestBuildServiceRequest_RequiresBuyerStdIn(t *testing.T) {
	t.Parallel()
	_, err := BuildServiceRequest(ServiceRequestOptions{RequestID: "req-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BuyerStdIn is required")
}

func TestBuildServiceRequest_RequiresRequestID(t *testing.T) {
	t.Parallel()
	_, err := BuildServiceRequest(ServiceRequestOptions{BuyerStdIn: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RequestID is required")
}

func TestServiceResponse_Accept(t *testing.T) {
	t.Parallel()
	req, _ := BuildServiceRequest(ServiceRequestOptions{RequestID: "req-2", BuyerStdIn: "x"})
	resp := BuildServiceResponse(req, "accept")
	assert.Equal(t, payment.PayloadServiceResponse, resp.Type)
	assert.Equal(t, PayloadVersion, resp.Version)
	assert.Equal(t, "req-2", resp.RequestID)
	assert.Equal(t, "accept", resp.Action)
}

func TestReceiveTypedPayload_FiltersByType(t *testing.T) {
	t.Parallel()
	adapter, ref, _ := setupMemoryTopic(t, "filter-by-type")
	sellerKey := newTestKey(t)

	// MemoryTopicAdapter delivers only messages published AFTER subscribe
	// time, so we must arrange the subscriber first then publish both.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	var wg sync.WaitGroup
	var got payment.ServiceRequest
	var recvErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		got, _, recvErr = ReceiveServiceRequest(ctx, adapter, ref)
	}()

	// Tiny pause to let the subscribe goroutine register.
	time.Sleep(20 * time.Millisecond)

	// First a non-matching envelope (a ServiceResponse) — receiver must skip.
	resp := BuildServiceResponse(payment.ServiceRequest{RequestID: "other"}, "accept")
	_, err := PublishPayload(context.Background(), adapter, ref, sellerKey, 1, resp)
	require.NoError(t, err)

	// Then the matching ServiceRequest.
	req, err := BuildServiceRequest(ServiceRequestOptions{RequestID: "wanted", BuyerStdIn: "x"})
	require.NoError(t, err)
	_, err = PublishPayload(context.Background(), adapter, ref, sellerKey, 2, req)
	require.NoError(t, err)

	wg.Wait()
	require.NoError(t, recvErr)
	assert.Equal(t, "wanted", got.RequestID, "subscriber must filter past the off-type envelope")
}

func TestReceiveTypedPayload_FiltersByRequestID(t *testing.T) {
	t.Parallel()
	adapter, ref, _ := setupMemoryTopic(t, "filter-by-rid")
	sellerKey := newTestKey(t)

	// Two ServiceResponses with different requestIDs; receiver wants the second.
	resp1 := BuildServiceResponse(payment.ServiceRequest{RequestID: "alpha"}, "accept")
	resp2 := BuildServiceResponse(payment.ServiceRequest{RequestID: "beta"}, "accept")

	// Subscribe BEFORE publishing so the channel sees both.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	var wg sync.WaitGroup
	var got payment.ServiceResponse
	var recvErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		got, _, recvErr = ReceiveServiceResponse(ctx, adapter, ref, "beta")
	}()

	// Tiny pause to let the subscribe goroutine register.
	time.Sleep(20 * time.Millisecond)
	_, err := PublishPayload(context.Background(), adapter, ref, sellerKey, 1, resp1)
	require.NoError(t, err)
	_, err = PublishPayload(context.Background(), adapter, ref, sellerKey, 2, resp2)
	require.NoError(t, err)

	wg.Wait()
	require.NoError(t, recvErr)
	assert.Equal(t, "beta", got.RequestID)
}

func TestReceiveTypedPayload_HonorsContextCancel(t *testing.T) {
	t.Parallel()
	adapter, ref, _ := setupMemoryTopic(t, "honors-cancel")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, _, err := ReceiveServiceRequest(ctx, adapter, ref)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// setupMemoryTopic creates an in-memory adapter + a fresh topic by name and
// returns both. Tests use this when they need to publish + subscribe without
// caring about full bus setup.
func setupMemoryTopic(t *testing.T, topicName string) (*topic.MemoryTopicAdapter, topic.TopicRef, func()) {
	t.Helper()
	adapter := topic.NewMemoryTopicAdapter()
	ref, err := adapter.CreateTopic(topic.CreateTopicOpts{Memo: topicName})
	require.NoError(t, err)
	return adapter, ref, func() {}
}
