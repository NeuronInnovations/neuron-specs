package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOnboardingAssetsEmbedded verifies the guided-onboarding layer is embedded
// in the served static assets. The tour is vanilla HTML/CSS/JS baked into the
// binary via //go:embed, so there is no JS test harness; this asserts the
// markers ship in the built assets (mirrors TestStaticAssetsServed). Behaviour
// is verified manually. Written Red-first: these substrings are absent until the
// onboarding layer is added to index.html / app.js / app.css.
func TestOnboardingAssetsEmbedded(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir()})

	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/", []string{`id="tourBtn"`, `id="onboard"`}},
		{"/app.js", []string{"initOnboarding", "startTour", "neuron-explorer-tour"}},
		{"/app.css", []string{".onboard", "tour-btn"}},
	} {
		resp, body := getBody(t, ts.URL+tc.path)
		require.Equal(t, http.StatusOK, resp.StatusCode, tc.path)
		for _, want := range tc.want {
			require.Contains(t, string(body), want, "%s must contain %q", tc.path, want)
		}
	}
}
