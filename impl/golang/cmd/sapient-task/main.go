// Command sapient-task is the HLDMM-side tasking CLI for the local SAPIENT demo.
// It publishes a Task{CONTROL_STOP|CONTROL_START} addressed to an ASM onto the
// auditable-lane stub (the local stand-in for the 004 Topic System), then waits
// for and prints the matching TaskAck.
//
//	sapient-task --lane file:control.ndjson --asm-node-id <asm> --control stop
//
// It owns no libp2p/identity; it is a thin lane client (the seller's
// tasking.Manager applies the control and emits the TaskAck).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/auditlane"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/tasking"
)

func main() {
	fs := flag.NewFlagSet("sapient-task", flag.ContinueOnError)
	var (
		lanePath = fs.String("lane", "", "auditable-lane stub file path (file:PATH or PATH) [required]")
		asmID    = fs.String("asm-node-id", "", "target ASM node_id (Task DestinationId) [required]")
		fromID   = fs.String("from-node-id", "hldmm", "issuing consumer/session id (Task NodeId = the session key)")
		control  = fs.String("control", "", "stop|start [required]")
		wait     = fs.Duration("wait", 3*time.Second, "how long to wait for the TaskAck")
	)
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *lanePath == "" || *asmID == "" || *control == "" {
		log.Fatal("sapient-task: --lane, --asm-node-id and --control are required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	lane := auditlane.NewFileLane(strings.TrimPrefix(*lanePath, "file:"))
	defer lane.Close()

	if err := run(ctx, lane, *asmID, *fromID, *control, *wait, os.Stdout); err != nil {
		log.Fatalf("sapient-task: %v", err)
	}
}

// run issues one control Task and waits for the matching TaskAck on the ASM's
// stdOut. Exposed (lane injected) for testing against an in-memory lane.
func run(ctx context.Context, lane auditlane.Lane, asmID, fromID, controlStr string, wait time.Duration, stdout io.Writer) error {
	var control sapientpb.Task_Control
	switch strings.ToLower(controlStr) {
	case "stop":
		control = sapientpb.Task_CONTROL_STOP
	case "start":
		control = sapientpb.Task_CONTROL_START
	default:
		return fmt.Errorf("unknown --control %q (want stop|start)", controlStr)
	}

	// Subscribe to stdOut BEFORE issuing so we never miss the ack.
	acks, err := lane.Subscribe(ctx, auditlane.Channel{ASMNodeID: asmID, Role: auditlane.RoleStdOut})
	if err != nil {
		return fmt.Errorf("subscribe stdOut: %w", err)
	}

	msg, err := tasking.IssueControl(ctx, lane, asmID, fromID, control, nil)
	if err != nil {
		return fmt.Errorf("issue control: %w", err)
	}
	taskID := msg.GetTask().GetTaskId()
	fmt.Fprintf(stdout, "issued Task %s control=%s asm=%s from=%s\n", taskID, control, asmID, fromID)

	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("timed out waiting for TaskAck")
		case m, ok := <-acks:
			if !ok {
				return errors.New("lane closed before TaskAck")
			}
			ack := m.GetTaskAck()
			if ack == nil || ack.GetTaskId() != taskID {
				continue
			}
			fmt.Fprintf(stdout, "TaskAck %s status=%s reason=%v\n", taskID, ack.GetTaskStatus(), ack.GetReason())
			return nil
		}
	}
}
