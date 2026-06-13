package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
)

const fixtureNDJSON = `{"timestamp":"2026-01-26T13:00:00.131304119Z","nodeId":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","detectionReport":{"reportId":"01TESTREPORT","objectId":"01TESTOBJECT","location":{"x":-6.69565,"y":50.03406,"z":9563.1,"coordinateSystem":"LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M","datum":"LOCATION_DATUM_WGS84_E"},"objectInfo":[{"type":"adsb.icao24","value":"3949E8"}]}}
{"timestamp":"2026-01-26T13:00:01.000000000Z","nodeId":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","detectionReport":{"reportId":"01TESTREPORT2","objectId":"01TESTOBJECT2","location":{"x":-6.3,"y":49.9,"z":11000,"coordinateSystem":"LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M","datum":"LOCATION_DATUM_WGS84_E"}}}
`

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.ndjson")
	require.NoError(t, os.WriteFile(path, []byte(fixtureNDJSON), 0o644))
	return path
}

func TestLoadFixture(t *testing.T) {
	t.Parallel()
	msgs, err := loadFixture(writeFixture(t))
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "01TESTREPORT", msgs[0].GetDetectionReport().GetReportId())
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", msgs[0].GetNodeId())
}

func TestLoadFixture_Errors(t *testing.T) {
	t.Parallel()
	_, err := loadFixture(filepath.Join(t.TempDir(), "missing.ndjson"))
	require.Error(t, err)

	empty := filepath.Join(t.TempDir(), "empty.ndjson")
	require.NoError(t, os.WriteFile(empty, nil, 0o644))
	_, err = loadFixture(empty)
	require.Error(t, err, "empty fixture is an error, not a silent no-op")
}

// TestFeedReplay_ServesLEFrames: a client dialing the replay feed receives the
// fixture messages as LE-framed protobuf — exactly what a seller's
// ReadBridgeFeed consumes from a real bridge.
func TestFeedReplay_ServesLEFrames(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer pr.Close()

	fixture := writeFixture(t)
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{
			"--listen", "127.0.0.1:0",
			"--fixture", fixture,
			"--interval", "20ms",
			"--loop",
		}, pw)
	}()

	// First stdout line is the bound address.
	buf := make([]byte, 256)
	n, err := pr.Read(buf)
	require.NoError(t, err)
	addr := string(buf[:n])
	addr = addr[:len(addr)-1] // trailing newline

	msgs, _ := sapient.ReadBridgeFeed(ctx, addr)
	seen := map[string]bool{}
	require.Eventually(t, func() bool {
		select {
		case m := <-msgs:
			if m != nil {
				seen[m.GetDetectionReport().GetReportId()] = true
			}
		case <-time.After(50 * time.Millisecond):
		}
		return seen["01TESTREPORT"] && seen["01TESTREPORT2"]
	}, 10*time.Second, time.Millisecond, "client must receive both fixture messages (seen=%v)", seen)

	cancel()
	select {
	case rerr := <-done:
		require.NoError(t, rerr)
	case <-time.After(5 * time.Second):
		t.Fatal("replay run() did not exit on cancel")
	}
}

func TestRun_RequiresFixture(t *testing.T) {
	t.Parallel()
	err := run(context.Background(), []string{"--listen", "127.0.0.1:0"}, os.Stdout)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--fixture")
}
