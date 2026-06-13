package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTracksProxy_HappyPath(t *testing.T) {
	upstreamBody := `{"tracks":[{"uid":"D1","position":{"lat":50.1,"lon":-5.6,"alt":100}}],"count":1,"focus":{"lat":50.1,"lon":-5.6,"count":1}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/state.json", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, upstreamBody)
	}))
	defer upstream.Close()

	ts := testServer(t, config{evidenceDir: t.TempDir(), fidURL: upstream.URL})
	resp, body := getBody(t, ts.URL+"/tracks.json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.JSONEq(t, upstreamBody, string(body), "state passed through verbatim (incl. focus)")
	require.Contains(t, string(body), "focus")
}

func TestTracksProxy_DegradesWhenDown(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir(), fidURL: "http://127.0.0.1:1"})
	resp, body := getBody(t, ts.URL+"/tracks.json")
	require.Equal(t, http.StatusOK, resp.StatusCode, "degradation is a 200, not a 5xx")
	var out struct {
		Tracks   []any  `json:"tracks"`
		Count    int    `json:"count"`
		Degraded bool   `json:"degraded"`
		Reason   string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	require.True(t, out.Degraded)
	require.Equal(t, 0, out.Count)
	require.Empty(t, out.Tracks)
	require.NotEmpty(t, out.Reason)
}

func TestEventsProxy_PassthroughHappyPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/events", r.URL.Path)
		w.Header().Set("Content-Type", "text/event-stream")
		f, _ := w.(http.Flusher)
		fmt.Fprint(w, "event: snapshot\ndata: {\"uid\":\"D1\"}\n\n")
		if f != nil {
			f.Flush()
		}
		fmt.Fprint(w, "event: update\ndata: {\"uid\":\"D2\"}\n\n")
		if f != nil {
			f.Flush()
		}
		// handler returns -> upstream closes the stream, so our copy gets EOF
	}))
	defer upstream.Close()

	ts := testServer(t, config{evidenceDir: t.TempDir(), fidURL: upstream.URL})
	resp, body := getBody(t, ts.URL+"/events")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	s := string(body)
	require.Contains(t, s, "event: snapshot")
	require.Contains(t, s, `"uid":"D1"`)
	require.Contains(t, s, "event: update")
}

func TestEventsProxy_DegradesWhenDown(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir(), fidURL: "http://127.0.0.1:1"})
	resp, body := getBody(t, ts.URL+"/events")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "event: degraded")
}
