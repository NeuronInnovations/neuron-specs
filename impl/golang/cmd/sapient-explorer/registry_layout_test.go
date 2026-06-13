package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRegistryTwoPaneLayout verifies the Agent Registry uses a two-pane (list +
// detail) layout so the agent list stays reachable while inspecting an agent,
// and that the full Agent Card JSON has a bounded internal scroll. The UI is
// vanilla HTML/CSS/JS baked in via //go:embed (no JS test harness), so this
// asserts the structural markers ship in the served assets; interactive
// behaviour is verified manually. Written Red-first: absent until the two-pane
// restructure lands in index.html / app.css / app.js.
func TestRegistryTwoPaneLayout(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir()})

	for _, tc := range []struct {
		path string
		want []string
	}{
		// List pane and detail pane both present in the DOM (list never replaced
		// by detail), plus the breadcrumb and the narrow-screen back control.
		{"/", []string{
			`id="registrySplit"`, `id="registryListPane"`, `id="agentDetail"`,
			`id="backToAgents"`, `id="registryCrumb"`,
		}},
		// Two-pane grid + bounded Agent Card JSON scroll.
		{"/app.css", []string{
			"#registrySplit", "grid-template-columns", ".card-json-pre", "max-height", "overflow",
		}},
		// Breadcrumb + back wiring + narrow list/detail toggle state.
		{"/app.js", []string{"backToAgents", "show-detail", "setCrumb"}},
	} {
		resp, body := getBody(t, ts.URL+tc.path)
		require.Equal(t, http.StatusOK, resp.StatusCode, tc.path)
		for _, want := range tc.want {
			require.Contains(t, string(body), want, "%s must contain %q", tc.path, want)
		}
	}
}
