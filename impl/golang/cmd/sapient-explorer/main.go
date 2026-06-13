// Command sapient-explorer is a standalone, additive tactical situational-
// awareness console for the Neuron SAPIENT demo. It serves one tabbed web UI on
// 127.0.0.1:8194 with two views:
//
//   - Tactical Map: live SAPIENT drone tracks + operators, consumed read-only by
//     proxying the live sapient-fid-display (:8193) /state.json + /events. The
//     map degrades gracefully when that feed is down.
//   - Agent Registry: the registered SAPIENT agents read from local seller
//     evidence JSON — Agent Card, SIM vs ON-CHAIN provenance, sensor model, wire
//     format, and the verbatim agentURI card.
//
// It is standalone by design: a deliberately SEPARATE sibling that never imports
// or modifies cmd/fid-display (the legacy public reference demo) or cmd/sapient-fid-
// display (the live public demo) — it only consumes the latter's HTTP endpoints
// read-only. No deploy, no Caddy route, no Hedera calls, no key material.
//
//	sapient-explorer --dir <evidence-dir>
//	sapient-explorer --dir <evidence-dir> --fid-url http://127.0.0.1:8193
//	sapient-explorer --dir <evidence-dir> --http 127.0.0.1:8194 --lat 50.1 --lon -5.6 --zoom 13
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		os.Exit(2) // flag.ContinueOnError already printed usage
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, log.New(os.Stderr, "", log.LstdFlags)); err != nil {
		log.Fatalf("sapient-explorer: %v", err)
	}
}

func parseFlags(args []string) (config, error) {
	fs := flag.NewFlagSet("sapient-explorer", flag.ContinueOnError)
	httpAddr := fs.String("http", "127.0.0.1:8194", "HTTP UI bind address (loopback)")
	dir := fs.String("dir", ".", "directory of seller agent-evidence JSON files")
	fidURL := fs.String("fid-url", "http://127.0.0.1:8193", "base URL of the live sapient-fid-display (read-only proxy source)")
	sensors := fs.String("sensors", "", "optional operator-provided sensor-locations.json (enables the sensor layer; off when empty)")
	lat := fs.Float64("lat", 50.1027, "initial map center latitude")
	lon := fs.Float64("lon", -5.6705, "initial map center longitude")
	zoom := fs.Int("zoom", 13, "initial map zoom (1-19)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	return config{
		httpAddr:    *httpAddr,
		evidenceDir: *dir,
		fidURL:      *fidURL,
		sensorsPath: *sensors,
		center:      mapCenter{Lat: *lat, Lon: *lon, Zoom: *zoom},
	}, nil
}

// run binds the configured loopback address and serves until ctx is cancelled.
func run(ctx context.Context, cfg config, logger *log.Logger) error {
	ln, err := net.Listen("tcp", cfg.httpAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.httpAddr, err)
	}
	logger.Printf("Neuron Agent Explorer on http://%s (evidence=%q, fid=%s)", ln.Addr(), cfg.evidenceDir, cfg.fidURL)
	return serve(ctx, ln, newServer(cfg, logger).routes(), logger)
}

// serve runs h on ln until ctx is cancelled, then shuts down gracefully. Split
// from run so tests can drive it with an ephemeral listener.
func serve(ctx context.Context, ln net.Listener, h http.Handler, logger *log.Logger) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Printf("shutting down")
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}
