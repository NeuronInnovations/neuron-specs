package sbs

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunBaseStationTCP_EndToEnd spins up a local TCP server that streams the
// vanilla-jv.sbs fixture, then runs RunBaseStationTCP against it, asserting
// that the source emits one SBSTrack per MSG type-3 record (and silently
// drops the other types).
func TestRunBaseStationTCP_EndToEnd(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("testdata", "vanilla-jv.sbs")
	fixtureBytes, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	addr := ln.Addr().String()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write(fixtureBytes)
		// Keep connection open briefly to let the client drain — then close.
		time.Sleep(100 * time.Millisecond)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out := make(chan SBSTrack, 16)
	clientDone := make(chan error, 1)
	go func() {
		clientDone <- RunBaseStationTCP(ctx, addr, out)
	}()

	var got []SBSTrack
loop:
	for {
		select {
		case t := <-out:
			got = append(got, t)
			if len(got) >= 6 {
				break loop
			}
		case <-ctx.Done():
			break loop
		}
	}

	cancel() // stop the client
	<-clientDone
	<-serverDone

	require.Len(t, got, 6, "expected 6 type-3 records from fixture")
	// First record's ICAO is "A1B2C3" (fixture row 2 is the first MSG,3).
	assert.Equal(t, "A1B2C3", got[0].ICAO)
	// Verify ordering by upstream GeneratedAt matches fixture order.
	for i := 1; i < len(got); i++ {
		assert.False(t, got[i].Rx.Before(got[i-1].Rx),
			"Rx must be monotonically non-decreasing across emissions")
	}
}

func TestRunBaseStationTCP_RequiresHostPort(t *testing.T) {
	t.Parallel()
	out := make(chan SBSTrack, 1)
	err := RunBaseStationTCP(context.Background(), "", out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host:port")
}

func TestRunBaseStationTCP_ExitsOnContextCancel(t *testing.T) {
	t.Parallel()

	// Listen on a port that accepts then idles (no data).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Hold the connection open without sending data.
		<-time.After(2 * time.Second)
		_ = conn.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan SBSTrack, 1)
	done := make(chan error, 1)
	go func() {
		done <- RunBaseStationTCP(ctx, ln.Addr().String(), out)
	}()

	// Give the client time to dial + start reading.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("RunBaseStationTCP did not exit within 2s of context cancel")
	}
	<-serverDone
}
