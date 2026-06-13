package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func readStatic(t *testing.T, name string) string {
	t.Helper()
	b, err := staticFS.ReadFile("static/" + name)
	require.NoError(t, err)
	return string(b)
}

func displayCSSZIndex(t *testing.T, css, sel string) int {
	t.Helper()
	idx := strings.Index(css, sel)
	require.GreaterOrEqual(t, idx, 0, "selector %q not found", sel)
	block := css[idx:]
	end := strings.Index(block, "}")
	require.Greater(t, end, 0)
	m := regexp.MustCompile(`z-index:\s*(\d+)`).FindStringSubmatch(block[:end])
	require.NotNil(t, m, "no z-index in block for %q", sel)
	n, err := strconv.Atoi(m[1])
	require.NoError(t, err)
	return n
}

// TestUILayout_DrawerAboveLegend pins the stacking order: the detail drawer
// must render above every other floating map layer so a selected entity's
// details are never hidden, and its close control stays clickable.
func TestUILayout_DrawerAboveLegend(t *testing.T) {
	css := readStatic(t, "app.css")
	require.Greater(t, displayCSSZIndex(t, css, "#drawer {"), displayCSSZIndex(t, css, "#legend {"))
}

// TestUILayout_PerModalityRecenterControls pins the split recenter controls
// and their honest disabled-at-zero state, all wired through logic.js.
func TestUILayout_PerModalityRecenterControls(t *testing.T) {
	js := readStatic(t, "app.js")
	for _, want := range []string{"recenterStates", "focusForKind", "updateRecenterButtons", "recenterOnKind"} {
		require.Contains(t, js, want)
	}
	require.NotContains(t, js, "aircrafts", "correct English: 'live aircraft', never 'aircrafts'")

	logic := readStatic(t, "logic.js")
	require.Contains(t, logic, "'live aircraft'")
	require.Contains(t, logic, "'live drones'")

	css := readStatic(t, "app.css")
	require.Contains(t, css, ".recenter-ctl a.disabled")
}

// TestUILayout_MarkerVisibility pins the marker contrast + clickability
// treatment: dark stroke on the aircraft glyph (readable on light tiles),
// oversized hitbox container, hover affordance, stale fade + selected ring.
func TestUILayout_MarkerVisibility(t *testing.T) {
	css := readStatic(t, "app.css")
	for _, want := range []string{
		"stroke: #0f172a",
		".marker:hover",
		".marker.stale",
		".marker.selected::after",
	} {
		require.Contains(t, css, want)
	}

	js := readStatic(t, "app.js")
	require.Contains(t, js, "iconSize: [30, 30]", "track hitbox container is 30px")
	require.Contains(t, js, "setZIndexOffset", "selected marker rises above overlapping neighbors")
}

// TestUILayout_SharedClassConfidenceFormatter pins that the drawer renders
// class/confidence through the shared logic.js classLine — the same function
// the explorer uses — so the two UIs can never disagree on the text.
func TestUILayout_SharedClassConfidenceFormatter(t *testing.T) {
	js := readStatic(t, "app.js")
	require.Contains(t, js, "FL.classLine(")
	require.NotContains(t, js, "snap.classification.confidence ?",
		"no truthiness-hide of confidence — missing confidence must render an explicit honest state")

	logic := readStatic(t, "logic.js")
	require.Contains(t, logic, "conf not provided")
}

// TestUILayout_OverflowHardening pins the layout-overflow fixes: the header
// wraps, the map pane may shrink (min-width: 0), narrow viewports get a
// narrower rail, and long monospace values wrap rather than widening cards.
func TestUILayout_OverflowHardening(t *testing.T) {
	css := readStatic(t, "app.css")
	for _, want := range []string{
		"flex-wrap: wrap",
		"@media (max-width: 900px)",
		"overflow-wrap: anywhere",
	} {
		require.Contains(t, css, want)
	}
	require.NotContains(t, css, "overflow-x: hidden",
		"fix the overflow at its source, never by hiding content")
}
