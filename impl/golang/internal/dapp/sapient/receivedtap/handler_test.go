package receivedtap

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forbiddenSubstrings must never appear in any response body — a guard against
// leaking secrets, identifiers, or host paths through the test projection.
var forbiddenSubstrings = []string{
	"PRIVATE", "privateKey", "PRIVATE_KEY", "BEGIN ", "mnemonic",
	"registryAddress", "transactionHash", "agentURI", "chainId",
	"/Users/", "/home/", "/root/", "ssh", "SSH", "HEDERA", "0.0.8",
}

func assertNoSecrets(t *testing.T, body string) {
	t.Helper()
	for _, bad := range forbiddenSubstrings {
		assert.NotContains(t, body, bad, "response must not leak %q", bad)
	}
}

func getBody(t *testing.T, h http.Handler, target string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, target, nil))
	return rr, rr.Body.String()
}

func TestHandler_Latest(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 0)
	base := time.Unix(1000, 0).UTC()
	s.Record(Project(fullDetection(), base, "synthetic", &Source{AgentID: "1", SellerEVM: "0xSELLER"}, true))
	s.Record(Project(fullDetection(), base.Add(time.Second), "synthetic", nil, true))
	h := Handler(s, nil)

	rr, body := getBody(t, h, "/sapient/received/latest")
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var got []Projection
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	require.Len(t, got, 2)
	assert.True(t, got[0].ReceivedAt.After(got[1].ReceivedAt), "newest first")
	assert.Empty(t, got[0].ProtobufBase64, "base64 stripped without ?protobuf=1")
	assertNoSecrets(t, body)

	// ?limit caps the result.
	_, limBody := getBody(t, h, "/sapient/received/latest?limit=1")
	var lim []Projection
	require.NoError(t, json.Unmarshal([]byte(limBody), &lim))
	assert.Len(t, lim, 1)
}

func TestHandler_LatestProtobufOptIn(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 0)
	// includeProtobuf=true at Record time → bytes retained.
	s.Record(Project(fullDetection(), time.Unix(1, 0), "", nil, true))
	h := Handler(s, nil)

	_, off := getBody(t, h, "/sapient/received/latest")
	var offP []Projection
	require.NoError(t, json.Unmarshal([]byte(off), &offP))
	assert.Empty(t, offP[0].ProtobufBase64)

	_, on := getBody(t, h, "/sapient/received/latest?protobuf=1")
	var onP []Projection
	require.NoError(t, json.Unmarshal([]byte(on), &onP))
	assert.NotEmpty(t, onP[0].ProtobufBase64, "?protobuf=1 exposes retained bytes")
}

func TestHandler_Schema(t *testing.T) {
	t.Parallel()
	h := Handler(NewReceivedSapientStore(0, 0), nil)
	rr, body := getBody(t, h, "/sapient/received/schema")
	require.Equal(t, http.StatusOK, rr.Code)

	for _, phrase := range []string{
		"testing projection",
		"SEPARATE", // UI/map stream is a separate projection
		"intentionally omitted",
		"demo CoT",
		"audit lane",
		"non-repudiation",
	} {
		assert.Contains(t, body, phrase)
	}
	assertNoSecrets(t, body)
}

func TestHandler_Health(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 0)
	s.Record(Project(fullDetection(), time.Unix(1000, 0).UTC(), "synthetic",
		&Source{AgentID: "1", SellerEVM: "0xSELLER"}, false))
	h := Handler(s, nil)

	rr, body := getBody(t, h, "/sapient/received/health")
	require.Equal(t, http.StatusOK, rr.Code)
	var got Health
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	assert.Equal(t, 1, got.RetainedCount)
	assert.Equal(t, 1, got.ObjectCount)
	require.NotNil(t, got.LatestReceivedAt)
	assertNoSecrets(t, body)
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	h := Handler(NewReceivedSapientStore(0, 0), nil)
	for _, path := range []string{
		"/sapient/received/latest", "/sapient/received/stream",
		"/sapient/received/schema", "/sapient/received/health",
	} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, nil))
		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "POST %s", path)
	}
}

func TestHandler_StreamCleanDisconnectNoLeak(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 4)
	srv := httptest.NewServer(Handler(s, nil))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/sapient/received/stream", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/x-ndjson", resp.Header.Get("Content-Type"))

	// Subscriber registered once the headers are flushed.
	require.Eventually(t, func() bool { return s.Health().Subscribers == 1 },
		2*time.Second, 10*time.Millisecond)

	s.Record(Project(fullDetection(), time.Unix(1, 0), "", nil, false))
	sc := bufio.NewScanner(resp.Body)
	require.True(t, sc.Scan(), "one NDJSON line delivered")
	var line Projection
	require.NoError(t, json.Unmarshal(sc.Bytes(), &line))
	assert.Equal(t, "DetectionReport", line.MessageType)

	cancel() // client disconnects
	require.Eventually(t, func() bool { return s.Health().Subscribers == 0 },
		2*time.Second, 10*time.Millisecond, "handler must unsubscribe on disconnect (no leak)")
}

func TestHandler_StreamClientCap(t *testing.T) {
	t.Parallel()
	s := NewReceivedSapientStore(0, 2) // cap of 2 stream clients
	srv := httptest.NewServer(Handler(s, nil))
	defer srv.Close()

	ctx := t.Context() // cancelled at test cleanup, tearing down the open streams
	open := func() *http.Response {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/sapient/received/stream", nil)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp
	}
	r1 := open()
	defer r1.Body.Close()
	r2 := open()
	defer r2.Body.Close()
	require.Eventually(t, func() bool { return s.Health().Subscribers == 2 },
		2*time.Second, 10*time.Millisecond)

	r3 := open()
	defer r3.Body.Close()
	b, _ := io.ReadAll(r3.Body)
	assert.Equal(t, http.StatusServiceUnavailable, r3.StatusCode, "third stream rejected at cap")
	assert.Contains(t, strings.ToLower(string(b)), "too many")
}
