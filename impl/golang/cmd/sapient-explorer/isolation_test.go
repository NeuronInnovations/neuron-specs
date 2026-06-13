package main

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsolation_NoSharedDisplayDeps enforces the architectural boundary: the
// explorer must never link the legacy public reference demo (cmd/fid-display) or the
// live public demo (cmd/sapient-fid-display). Reuse is library-level only
// (internal/dapp/sapient, internal/registry) — never the sibling binaries.
func TestIsolation_NoSharedDisplayDeps(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not in PATH")
	}
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	require.NoError(t, err, "go list -deps .: %s", out)
	deps := string(out)
	for _, forbidden := range []string{
		"github.com/neuron-sdk/neuron-go-sdk/cmd/fid-display",
		"github.com/neuron-sdk/neuron-go-sdk/cmd/sapient-fid-display",
	} {
		require.NotContains(t, deps, forbidden, "explorer must not link %s", forbidden)
	}
}
