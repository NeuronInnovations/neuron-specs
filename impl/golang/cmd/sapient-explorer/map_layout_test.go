package main

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// cssZIndex extracts the z-index declared inside the first CSS block whose
// selector line contains sel. Brittle-by-design: these are our own embedded
// assets, and the test exists precisely to pin their stacking order.
func cssZIndex(t *testing.T, css, sel string) int {
	t.Helper()
	idx := strings.Index(css, sel)
	require.GreaterOrEqual(t, idx, 0, "selector %q not found", sel)
	block := css[idx:]
	end := strings.Index(block, "}")
	require.Greater(t, end, 0, "unterminated block for %q", sel)
	m := regexp.MustCompile(`z-index:\s*(\d+)`).FindStringSubmatch(block[:end])
	require.NotNil(t, m, "no z-index in block for %q", sel)
	n, err := strconv.Atoi(m[1])
	require.NoError(t, err)
	return n
}

// TestMapLayout_DrawerAboveFiltersAndShifts pins the two fixes for the
// drawer/filter-panel overlap: the drawer stacks ABOVE the filter panel
// (selected-object details are never hidden behind filters), and opening the
// drawer shifts the filters aside (#view-map.drawer-open) so both stay fully
// visible and clickable.
func TestMapLayout_DrawerAboveFiltersAndShifts(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir()})

	_, body := getBody(t, ts.URL+"/app.css")
	css := string(body)

	drawer := cssZIndex(t, css, ".drawer {")
	filters := cssZIndex(t, css, "#filters {")
	legend := cssZIndex(t, css, "#legend {")
	require.Greater(t, drawer, filters, "drawer must stack above the filter panel")
	require.Greater(t, drawer, legend, "drawer must stack above the legend")

	require.Contains(t, css, "#view-map.drawer-open #filters",
		"filters must shift aside while the drawer is open")

	_, js := getBody(t, ts.URL+"/app.js")
	for _, want := range []string{"drawer-open", "closeDrawer", "restyleMarker"} {
		require.Contains(t, string(js), want)
	}
}

// TestMapLayout_SidebarCountsWrap pins the horizontal-scroll fix: the LIVE
// counter row wraps instead of forcing the 300px sidebar (and with it the
// page) to scroll horizontally.
func TestMapLayout_SidebarCountsWrap(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir()})

	_, body := getBody(t, ts.URL+"/app.css")
	css := string(body)
	idx := strings.Index(css, ".counts {")
	require.GreaterOrEqual(t, idx, 0)
	block := css[idx : idx+strings.Index(css[idx:], "}")]
	require.Contains(t, block, "flex-wrap: wrap", "counter row must wrap inside the 300px sidebar")

	// The fix must be the wrap, not a blunt overflow-x:hidden hiding content.
	require.NotContains(t, css, "overflow-x: hidden")
}

// TestMapLayout_PerModalityRecenterControls pins the split recenter controls:
// one button per modality with correct English labels, and the honest
// disabled-at-zero behaviour wired through the shared logic module.
func TestMapLayout_PerModalityRecenterControls(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir()})

	resp, body := getBody(t, ts.URL+"/app.js")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	js := string(body)
	for _, want := range []string{"recenterStates", "focusForKind", "updateRecenterButtons", "recenterOnKind"} {
		require.Contains(t, js, want)
	}
	require.NotContains(t, js, "aircrafts", "correct English: 'live aircraft', never 'aircrafts'")

	// Labels live in the shared logic module the page loads.
	_, logic := getBody(t, ts.URL+"/logic.js")
	require.Contains(t, string(logic), "'live aircraft'")
	require.Contains(t, string(logic), "'live drones'")

	_, html := getBody(t, ts.URL+"/")
	require.Contains(t, string(html), `src="/logic.js"`, "index.html must load the shared logic module")

	_, css := getBody(t, ts.URL+"/app.css")
	require.Contains(t, string(css), ".recenter-ctl a.disabled", "disabled state must be styled")
}

// TestMapLayout_SharedLogicByteIdentical pins the explorer's logic.js to the
// display's: both UIs interpret payloads through the SAME pure functions
// (markerKind, classLine, focusForKind, …), so the same frame can never render
// different class/confidence text or a different marker kind in the two UIs.
func TestMapLayout_SharedLogicByteIdentical(t *testing.T) {
	ours, err := staticFS.ReadFile("static/logic.js")
	require.NoError(t, err)
	theirs, err := os.ReadFile(filepath.Join("..", "sapient-fid-display", "static", "logic.js"))
	require.NoError(t, err)
	require.Equal(t, string(theirs), string(ours),
		"cmd/sapient-explorer/static/logic.js must be a byte-identical copy of cmd/sapient-fid-display/static/logic.js — copy it over rather than editing one side")
}

// TestMapLayout_MarkerVisibility pins the marker contrast + clickability
// treatment: dark stroke on the aircraft glyph, oversized hitbox container,
// hover affordance, and stale/selected states.
func TestMapLayout_MarkerVisibility(t *testing.T) {
	ts := testServer(t, config{evidenceDir: t.TempDir()})

	_, body := getBody(t, ts.URL+"/app.css")
	css := string(body)
	for _, want := range []string{
		"stroke: #0f172a",         // aircraft outline for light tiles
		".marker:hover",           // hover affordance
		".marker.stale",           // stale fade
		".marker.selected::after", // selected ring
		".mini.aircraft",          // legend swatch for aircraft
	} {
		require.Contains(t, css, want)
	}

	_, js := getBody(t, ts.URL+"/app.js")
	require.Contains(t, string(js), "iconSize: [30, 30]", "track hitbox container is 30px")
}
