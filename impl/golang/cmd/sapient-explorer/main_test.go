package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseFlags_Defaults(t *testing.T) {
	t.Parallel()
	cfg, err := parseFlags(nil)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:8194", cfg.httpAddr)
	require.Equal(t, ".", cfg.evidenceDir)
	require.Equal(t, "http://127.0.0.1:8193", cfg.fidURL)
	require.Equal(t, 13, cfg.center.Zoom)
	require.Empty(t, cfg.sensorsPath, "sensor layer off by default")
}

func TestParseFlags_Overrides(t *testing.T) {
	t.Parallel()
	cfg, err := parseFlags([]string{
		"--http", "127.0.0.1:9999", "--dir", "/tmp/ev", "--fid-url", "http://x:1",
		"--sensors", "/tmp/sensors.json", "--lat", "1.5", "--lon", "-2.5", "--zoom", "9",
	})
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:9999", cfg.httpAddr)
	require.Equal(t, "/tmp/ev", cfg.evidenceDir)
	require.Equal(t, "http://x:1", cfg.fidURL)
	require.Equal(t, "/tmp/sensors.json", cfg.sensorsPath)
	require.Equal(t, 1.5, cfg.center.Lat)
	require.Equal(t, -2.5, cfg.center.Lon)
	require.Equal(t, 9, cfg.center.Zoom)
}

func TestServe_ServesThenShutsDownOnContextCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	h := newServer(config{evidenceDir: t.TempDir()}, log.New(io.Discard, "", 0)).routes()

	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, h, log.New(io.Discard, "", 0)) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/healthz") //nolint:noctx
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err, "clean shutdown on context cancel")
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not shut down within 5s")
	}
}

func TestRun_FailFastOnBadAddr(t *testing.T) {
	t.Parallel()
	err := run(context.Background(), config{httpAddr: "127.0.0.1:99999"}, log.New(io.Discard, "", 0))
	require.Error(t, err, "invalid port must fail fast")
}
