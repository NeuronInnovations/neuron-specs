package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/auditlane"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/tasking"
)

type noopForwarder struct{}

func (noopForwarder) Send(*sapientpb.SapientMessage) error { return nil }

// TestRun_IssuesAndReceivesAck wires the CLI against an in-memory lane with a live
// tasking.Manager (the ASM) and asserts STOP is accepted end-to-end.
func TestRun_IssuesAndReceivesAck(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()

	mgr := tasking.NewManager(tasking.Options{ASMNodeID: "asm-1", Lane: lane})
	require.NoError(t, mgr.Start(t.Context()))
	mgr.RegisterSession("hldmm", noopForwarder{})

	var out bytes.Buffer
	err := run(t.Context(), lane, "asm-1", "hldmm", "stop", 3*time.Second, &out)
	require.NoError(t, err)
	require.Contains(t, out.String(), "TaskAck")
	require.Contains(t, out.String(), "ACCEPTED")
}

func TestRun_UnknownControl(t *testing.T) {
	lane := auditlane.NewMemoryLane()
	defer lane.Close()
	err := run(t.Context(), lane, "asm-1", "hldmm", "wiggle", time.Second, &bytes.Buffer{})
	require.Error(t, err)
}
