package receivedtap

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
)

const defaultLatestLimit = 50

// Handler returns the read-only HTTP surface for the received-SAPIENT tap:
//
//	GET /sapient/received/latest   — recent received messages (JSON array, newest-first)
//	GET /sapient/received/stream   — live NDJSON stream of received messages
//	GET /sapient/received/schema   — what this projection is (and is not)
//	GET /sapient/received/health   — size + freshness (no secrets)
//
// All routes are GET-only and expose no secrets, env, keys, or host paths.
func Handler(store *ReceivedSapientStore, logger *log.Logger) http.Handler {
	if logger == nil {
		logger = log.New(log.Writer(), "[sapient-received] ", log.LstdFlags)
	}
	mux := http.NewServeMux()
	_ = logger // reserved for future per-request diagnostics; kept for a stable signature
	mux.HandleFunc("/sapient/received/latest", latestHandler(store))
	mux.HandleFunc("/sapient/received/stream", streamHandler(store))
	mux.HandleFunc("/sapient/received/schema", schemaHandler())
	mux.HandleFunc("/sapient/received/health", healthHandler(store))
	return mux
}

// latestHandler serves the bounded ring as a JSON array, newest-first.
//
//	?limit=N    cap the number returned (default 50, bounded by capacity)
//	?protobuf=1 include the deterministic protobuf base64 (only present if the
//	            process retained it via --sapient-received-protobuf)
func latestHandler(store *ReceivedSapientStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		limit := defaultLatestLimit
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		out := store.Latest(limit)
		if !truthy(r.URL.Query().Get("protobuf")) {
			for i := range out {
				out[i].ProtobufBase64 = "" // strip unless explicitly requested
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// streamHandler serves a live NDJSON stream (one JSON object per line) of
// received messages. Read-only, bounded clients (503 when the cap is reached),
// bounded per-client buffer (lossy under a slow reader), and a clean exit on
// client disconnect via the request context.
func streamHandler(store *ReceivedSapientStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		ch, ok := store.Subscribe()
		if !ok {
			http.Error(w, "too many stream clients", http.StatusServiceUnavailable)
			return
		}
		defer store.Unsubscribe(ch)

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		enc := json.NewEncoder(w) // Encode writes a trailing newline → NDJSON framing
		for {
			select {
			case <-r.Context().Done():
				return
			case p := <-ch:
				p.ProtobufBase64 = "" // keep stream lines compact; use /latest?protobuf=1 for bytes
				if err := enc.Encode(p); err != nil {
					return // client gone or write failed
				}
				flusher.Flush()
			}
		}
	}
}

func healthHandler(store *ReceivedSapientStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, store.Health())
	}
}

func schemaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, schemaDoc)
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func truthy(s string) bool { return s == "1" || s == "true" }

// schema describes what the received-SAPIENT JSON projection is, and is not. It
// is a static document so partners can self-serve the honesty caveats.
type schema struct {
	CanonicalWire string            `json:"canonicalWire"`
	Projection    string            `json:"projection"`
	Endpoints     map[string]string `json:"endpoints"`
	Notes         []string          `json:"notes"`
}

var schemaDoc = schema{
	CanonicalWire: sapient.SapientWire,
	Projection:    "This JSON is a read-only TESTING projection of the SAPIENT messages received by the recipient/consumer, captured before any map/FID conversion. It is lossy and partner-friendly; it is not the canonical wire.",
	Endpoints: map[string]string{
		"GET /sapient/received/latest": "recent received messages as a JSON array, newest-first (?limit=N, ?protobuf=1)",
		"GET /sapient/received/stream": "live NDJSON stream of received messages (read-only, bounded, lossy under slow readers)",
		"GET /sapient/received/schema": "this document",
		"GET /sapient/received/health": "retained count, object count, latest timestamp, stream freshness (no secrets)",
	},
	Notes: []string{
		"Canonical wire is SAPIENT protobuf (" + sapient.SapientWire + "); this JSON is a testing projection only.",
		"The UI/map event stream (sapient-track on the FID display) is a SEPARATE projection; this endpoint is not that stream.",
		"Missing position or velocity may be intentionally omitted (the report carried none); absent blocks are never zero-filled.",
		"FRIENDLY is a demo CoT display profile, not tactical truth; this payload tap does not assert affiliation.",
		"The HCS heartbeat / audit lane is separate from these data-plane SAPIENT payloads.",
		"feedSource and source require the consumer to be started with seller agent-evidence; their absence is not an error.",
		"messageHash and protobufBase64 are a DETERMINISTIC RE-ENCODING of the decoded message (unknown fields were discarded on receipt). They are NOT the original wire bytes and must not be used for cryptographic non-repudiation.",
	},
}
