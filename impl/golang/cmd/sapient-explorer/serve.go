package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
)

//go:embed static/*
var staticFS embed.FS

// fidProbeTimeout bounds every read-only call to the upstream sapient-fid-display.
const fidProbeTimeout = 1500 * time.Millisecond

// mapCenter is the initial Leaflet view served to the browser via /config.json.
type mapCenter struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Zoom int     `json:"zoom"`
}

// config is the resolved runtime configuration for the explorer server.
type config struct {
	httpAddr    string    // loopback bind address (e.g. 127.0.0.1:8194)
	evidenceDir string    // directory of seller evidence *.json files
	fidURL      string    // base URL of the live sapient-fid-display (read-only)
	sensorsPath string    // optional operator-provided sensor-locations.json (off when empty)
	center      mapCenter // initial map view
}

// server holds the explorer's request-scoped dependencies.
type server struct {
	cfg    config
	client *http.Client // for read-only proxy/probe of fidURL
	logger *log.Logger
}

func newServer(cfg config, logger *log.Logger) *server {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	// No client-level timeout: each request sets its own context deadline so the
	// long-lived SSE proxy is not capped while one-shot probes still time out.
	return &server{cfg: cfg, client: &http.Client{}, logger: logger}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", s.indexHandler())
	mux.HandleFunc("/config.json", s.configHandler)
	mux.HandleFunc("/agents.json", s.agentsHandler)
	mux.HandleFunc("/agents/", s.agentDetailHandler)
	mux.HandleFunc("/tracks.json", s.tracksHandler)
	mux.HandleFunc("/events", s.eventsHandler)
	mux.HandleFunc("/sensors.json", s.sensorsHandler)
	mux.HandleFunc("/healthz", s.healthzHandler)
	return mux
}

// indexHandler serves the embedded static UI (HTML/CSS/JS).
func (s *server) indexHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(fmt.Errorf("embed static/: %w", err))
	}
	return http.FileServer(http.FS(sub))
}

func (s *server) configHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), fidProbeTimeout)
	defer cancel()
	writeJSON(w, http.StatusOK, map[string]any{
		"lat":           s.cfg.center.Lat,
		"lon":           s.cfg.center.Lon,
		"zoom":          s.cfg.center.Zoom,
		"fidDisplayURL": s.cfg.fidURL,
		"fidDisplayUp":  s.fidUp(ctx),
	})
}

// agentsHandler lists the registered agents from the evidence directory. It is
// deliberately resilient: a missing/empty directory yields an empty list (200),
// and a parse failure is surfaced in "error" while still returning 200 so the
// registry view stays usable. It never depends on the live map feed.
func (s *server) agentsHandler(w http.ResponseWriter, r *http.Request) {
	agents, loadErr := s.loadAgents()
	summaries := make([]AgentSummary, 0, len(agents))
	for _, a := range agents {
		summaries = append(summaries, summaryFromEvidence(a.ev, a.file))
	}
	out := map[string]any{
		"agents":      summaries,
		"count":       len(summaries),
		"evidenceDir": s.cfg.evidenceDir,
	}
	if loadErr != nil {
		out["error"] = loadErr.Error()
	}
	writeJSON(w, http.StatusOK, out)
}

// agentDetailHandler returns the full AgentCardDetail for /agents/{agentId}.json.
func (s *server) agentDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/agents/"), ".json")
	if id == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "missing agentId"})
		return
	}
	agents, loadErr := s.loadAgents()
	for _, a := range agents {
		if a.ev.AgentID == id {
			writeJSON(w, http.StatusOK, detailFromEvidence(a.ev, a.file))
			return
		}
	}
	msg := "agentId not found: " + id
	if loadErr != nil {
		msg = loadErr.Error()
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": msg})
}

func (s *server) healthzHandler(w http.ResponseWriter, r *http.Request) {
	agents, _ := s.loadAgents()
	ctx, cancel := context.WithTimeout(r.Context(), fidProbeTimeout)
	defer cancel()
	fid := "down"
	if s.fidUp(ctx) {
		fid = "up"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"evidenceAgents": len(agents),
		"fidDisplay":     fid,
		"time":           time.Now().UTC().Format(time.RFC3339),
	})
}

// loadedAgent pairs a parsed evidence record with its source filename.
type loadedAgent struct {
	ev   sapient.AgentEvidence
	file string
}

// loadAgents globs the evidence directory and parses each *.json. On a parse
// error it returns the records parsed so far plus the error (no silent skip),
// so the caller can surface the problem while still rendering what is valid.
func (s *server) loadAgents() ([]loadedAgent, error) {
	matches, err := filepath.Glob(filepath.Join(s.cfg.evidenceDir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	out := make([]loadedAgent, 0, len(matches))
	for _, m := range matches {
		ev, rerr := sapient.ReadEvidence(m)
		if rerr != nil {
			return out, rerr // corrupt JSON in an evidence dir — surfaced, never hidden
		}
		if !isEvidenceRecord(ev) {
			continue // valid JSON but not an AgentEvidence (e.g. a track snapshot)
		}
		out = append(out, loadedAgent{ev: ev, file: filepath.Base(m)})
	}
	return out, nil
}

// isEvidenceRecord reports whether a parsed record is actually a seller
// AgentEvidence (an agent id or a card), as opposed to some other JSON file that
// happens to share the directory (e.g. a /state.json track snapshot). json
// unmarshalling is lenient — unrelated files parse into a zero record — so this
// guards against listing them as junk agents.
func isEvidenceRecord(ev sapient.AgentEvidence) bool {
	return ev.AgentID != "" || len(ev.AgentURI) > 0
}

// fidUp reports whether the upstream sapient-fid-display answers /state.json.
// Read-only (GET); any error or non-2xx is reported as down.
func (s *server) fidUp(ctx context.Context) bool {
	if s.cfg.fidURL == "" {
		return false
	}
	url := strings.TrimRight(s.cfg.fidURL, "/") + "/state.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
