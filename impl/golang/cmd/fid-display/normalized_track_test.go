package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleNormalizedTrackJSONL mirrors the cmd/multistream-buyer on-the-wire
// envelope: TaggedFrame{source:"adsb", type:"normalized-track",
// frame=NormalizedTrack canonical-JSON per docs/normalized-track-contract.md}.
// All optional NormalizedTrack fields are populated so the snapshot path
// round-trips a non-trivial case.
const sampleNormalizedTrackJSONL = `{"source":"adsb","type":"normalized-track","sellerPeerID":"12D3KooWNormalized","receivedAt":"2026-05-14T15:30:00.123Z","frame":{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:49:00.123Z","source":"adsb","entityType":"aircraft","entityID":"A1B2C3","position":{"lat":51.4700,"lon":-0.4543,"altitudeM":11277.6},"velocity":{"groundSpeedMps":249.5,"headingDeg":290.0,"verticalRateMps":6.5},"callsign":"BAW178","squawk":"7000","quality":{"receivers":3,"horizErrM":12.5,"fakePosition":false}}}`

// TestApply_AcceptsNormalizedTrackTaggedFrame validates the full
// happy-path dispatch for source="adsb" && type="normalized-track".
// Asserts the SSEEvent kind, the snapshot's source/entityType/entityID,
// real (non-placeholder) lat/lon, all optional fields populated, and
// frame-count semantics on repeated entityID.
func TestApply_AcceptsNormalizedTrackTaggedFrame(t *testing.T) {
	t.Parallel()
	state := NewState()
	var tf TaggedFrame
	require.NoError(t, json.Unmarshal([]byte(sampleNormalizedTrackJSONL), &tf))

	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.Equal(t, "normalized-track", event.Kind,
		"source=adsb + type=normalized-track must dispatch to applyNormalizedTrack")
	require.NotNil(t, event.NormalizedTrack)
	assert.Nil(t, event.Drone, "normalized-track dispatch must not touch drone slot")
	assert.Nil(t, event.Aircraft, "normalized-track dispatch must not touch aircraft slot")

	got := event.NormalizedTrack
	assert.Equal(t, "A1B2C3", got.EntityID, "EntityID stored UPPERCASE per contract R3")
	assert.Equal(t, "aircraft", got.EntityType)
	assert.Equal(t, "adsb", got.Source)
	assert.Equal(t, "12D3KooWNormalized", got.SellerPeerID, "SellerPeerID from outer envelope")
	assert.InDelta(t, 51.4700, got.Lat, 1e-9, "REAL lat from inner frame.position")
	assert.InDelta(t, -0.4543, got.Lon, 1e-9, "REAL lon from inner frame.position")
	assert.InDelta(t, 11277.6, got.AltitudeM, 1e-9)
	assert.InDelta(t, 249.5, got.GroundSpeedMps, 1e-9)
	assert.InDelta(t, 290.0, got.HeadingDeg, 1e-9)
	assert.InDelta(t, 6.5, got.VerticalRateMps, 1e-9)
	assert.Equal(t, "BAW178", got.Callsign)
	assert.Equal(t, "7000", got.Squawk)
	assert.False(t, got.FakePosition, "quality.fakePosition=false on the wire")
	assert.Equal(t, uint64(1), got.FrameCount)

	// Independence: normalized-track ingest must NOT populate drones or
	// the placeholder-aircraft keyspace.
	assert.Empty(t, state.Snapshot(), "normalized-track must not populate drones")
	assert.Empty(t, state.AircraftSnapshot(), "normalized-track must not populate aircraft (placeholder)")
	require.Len(t, state.NormalizedTracksSnapshot(), 1)
	assert.Equal(t, "A1B2C3", state.NormalizedTracksSnapshot()[0].EntityID)

	// Frame-count semantics: same entityID increments, no duplication.
	tf2 := tf
	tf2.ReceivedAt = tf.ReceivedAt.Add(1 * time.Second)
	event2, err := state.Apply(tf2)
	require.NoError(t, err)
	require.NotNil(t, event2.NormalizedTrack)
	assert.Equal(t, uint64(2), event2.NormalizedTrack.FrameCount)
	require.Len(t, state.NormalizedTracksSnapshot(), 1,
		"same entityID must not duplicate the normalized-track entry")
}

// TestApply_NormalizedTrackPositionIsReal pins that the dispatch path
// reads lat/lon STRAIGHT from the inner frame's position object (no
// ICAO-hash placeholder math runs on this branch). Contrasts with
// TestApply_AdsBPositionIsPlaceholder which pins the opposite for the
// type="aircraft" path.
func TestApply_NormalizedTrackPositionIsReal(t *testing.T) {
	t.Parallel()
	// Pick a position that is FAR from the map center anchor so any
	// accidental placeholder-math contamination is glaringly visible.
	state := NewStateWithCenter(51.4775, -0.4614)                 // Heathrow
	const realLat, realLon, realAltM = -33.8688, 151.2093, 9144.0 // Sydney @ FL300
	tf := TaggedFrame{
		Source: "adsb", Type: "normalized-track",
		SellerPeerID: "PID-SYDNEY",
		ReceivedAt:   time.Now().UTC(),
		Frame: []byte(fmt.Sprintf(
			`{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:49:00Z","source":"adsb","entityType":"aircraft","entityID":"QFA001","position":{"lat":%f,"lon":%f,"altitudeM":%f}}`,
			realLat, realLon, realAltM,
		)),
	}
	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.NotNil(t, event.NormalizedTrack)
	got := event.NormalizedTrack

	assert.InDelta(t, realLat, got.Lat, 1e-9, "lat MUST be the real upstream value, not a placeholder")
	assert.InDelta(t, realLon, got.Lon, 1e-9, "lon MUST be the real upstream value, not a placeholder")
	assert.InDelta(t, realAltM, got.AltitudeM, 1e-9)
	assert.False(t, got.FakePosition, "default quality.fakePosition is false; no placeholder cue")

	// Negative check: the placeholder math would have anchored around
	// (51.4775, -0.4614) within ±0.1°. Sydney's lat -33.87 is nowhere
	// near that — any contamination is loud.
	assert.Greater(t, abs64(got.Lat-51.4775), 1.0,
		"lat must NOT have been pulled toward the map center anchor")
	assert.Greater(t, abs64(got.Lon-(-0.4614)), 1.0,
		"lon must NOT have been pulled toward the map center anchor")
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// discardLogger returns a logger whose io.Writer is io.Discard. Used by
// the TCP-listener tests in this file to avoid the latent race where
// handleConn's late "buyer disconnected" Printf fires after the test's
// *testing.T has returned (testWriter.Write → t.Log → race on
// common.destination). The State-side assertions don't depend on log
// output, so suppressing it is lossless for these tests.
func discardLogger() *log.Logger {
	return log.New(discardWriter{}, "", 0)
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// TestApply_NormalizedTrackIndependence asserts the dual-source
// independence rule with three keyspaces simultaneously populated: a
// Remote ID drone, a placeholder ADS-B aircraft, and a NormalizedTrack
// aircraft. Killing one source (by Evict-aging only its records) must
// leave the other two intact.
func TestApply_NormalizedTrackIndependence(t *testing.T) {
	t.Parallel()
	state := NewState()
	fresh := time.Now()
	old := fresh.Add(-10 * time.Minute)

	// Feed a fresh drone, a fresh placeholder-aircraft, and a STALE
	// normalized-track. Eviction must drop only the normalized-track.
	_, err := state.Apply(TaggedFrame{
		Source: "remote-id", Type: "drone", ReceivedAt: fresh,
		Frame: []byte(`{"droneId":"D1","droneIdType":"serial","position":{"lat":51,"lon":-0.5,"alt":100,"fix":"3D"}}`),
	})
	require.NoError(t, err)
	_, err = state.Apply(TaggedFrame{
		Source: "adsb", Type: "aircraft", SellerPeerID: "PID", ReceivedAt: fresh,
		Frame: []byte(`{"sellerEVM":"0xaaa","sellerName":"jv","sellerPeerID":"PID","meta":{"DF":17,"ICAO":"ABC123"}}`),
	})
	require.NoError(t, err)
	_, err = state.Apply(TaggedFrame{
		Source: "adsb", Type: "normalized-track", SellerPeerID: "PID-N", ReceivedAt: old,
		Frame: []byte(`{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:00:00Z","source":"adsb","entityType":"aircraft","entityID":"STALE1","position":{"lat":52,"lon":-1,"altitudeM":10000}}`),
	})
	require.NoError(t, err)

	require.Len(t, state.Snapshot(), 1)
	require.Len(t, state.AircraftSnapshot(), 1)
	require.Len(t, state.NormalizedTracksSnapshot(), 1)

	n := state.Evict(time.Now(), 5*time.Minute)
	assert.Equal(t, 1, n, "Evict drops only the stale normalized-track")
	assert.Len(t, state.Snapshot(), 1, "drone survives normalized-track eviction")
	assert.Len(t, state.AircraftSnapshot(), 1, "placeholder aircraft survives normalized-track eviction")
	assert.Empty(t, state.NormalizedTracksSnapshot(), "stale normalized-track evicted")
}

// TestApply_NormalizedTrackEviction confirms TTL eviction operates
// independently on the normalizedTracks map.
func TestApply_NormalizedTrackEviction(t *testing.T) {
	t.Parallel()
	state := NewState()
	old := time.Now().Add(-10 * time.Minute)
	fresh := time.Now()

	for _, p := range []struct {
		id   string
		seen time.Time
	}{
		{"OLD-1", old},
		{"OLD-2", old},
		{"FRESH", fresh},
	} {
		tf := TaggedFrame{
			Source: "adsb", Type: "normalized-track", SellerPeerID: "PID", ReceivedAt: p.seen,
			Frame: []byte(fmt.Sprintf(
				`{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:00:00Z","source":"adsb","entityType":"aircraft","entityID":"%s","position":{"lat":51,"lon":-0.5,"altitudeM":10000}}`,
				p.id,
			)),
		}
		_, err := state.Apply(tf)
		require.NoError(t, err)
	}
	require.Len(t, state.NormalizedTracksSnapshot(), 3)

	n := state.Evict(time.Now(), 5*time.Minute)
	assert.Equal(t, 2, n, "two OLD entries evicted, FRESH survives")
	require.Len(t, state.NormalizedTracksSnapshot(), 1)
	assert.Equal(t, "FRESH", state.NormalizedTracksSnapshot()[0].EntityID)
}

// TestApply_RejectsNormalizedTrackMissingEntityID exercises the
// required-field gate per contract §3.1: entityID MUST be present.
func TestApply_RejectsNormalizedTrackMissingEntityID(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "adsb", Type: "normalized-track",
		Frame: []byte(`{"type":"normalized-track","version":"1.0.0","source":"adsb","entityType":"aircraft","position":{"lat":51,"lon":-0.5,"altitudeM":10000}}`),
	}
	_, err := state.Apply(tf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing entityID")
}

// TestApply_NormalizedTrackEntityIDUppercase pins the contract R3
// uppercase-normalization rule.
func TestApply_NormalizedTrackEntityIDUppercase(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "adsb", Type: "normalized-track", SellerPeerID: "PID",
		ReceivedAt: time.Now().UTC(),
		Frame: []byte(
			`{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:00:00Z","source":"adsb","entityType":"aircraft","entityID":"a1b2c3","position":{"lat":51,"lon":-0.5,"altitudeM":10000}}`,
		),
	}
	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.NotNil(t, event.NormalizedTrack)
	assert.Equal(t, "A1B2C3", event.NormalizedTrack.EntityID,
		"entityID stored UPPERCASE per contract R3 regardless of inner-frame casing")
}

// TestApply_NormalizedTrackFakePositionPropagates makes sure the
// contract's quality.fakePosition signal survives the unmarshal path.
func TestApply_NormalizedTrackFakePositionPropagates(t *testing.T) {
	t.Parallel()
	state := NewState()
	tf := TaggedFrame{
		Source: "adsb", Type: "normalized-track", SellerPeerID: "PID",
		ReceivedAt: time.Now().UTC(),
		Frame: []byte(
			`{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:00:00Z","source":"adsb","entityType":"aircraft","entityID":"AAAA01","position":{"lat":51,"lon":-0.5,"altitudeM":10000},"quality":{"fakePosition":true}}`,
		),
	}
	event, err := state.Apply(tf)
	require.NoError(t, err)
	require.NotNil(t, event.NormalizedTrack)
	assert.True(t, event.NormalizedTrack.FakePosition,
		"quality.fakePosition=true must surface in the snapshot (UI cue)")
}

// TestStateHandler_IncludesNormalizedTracks asserts /state.json carries
// the new `normalizedTracks` array alongside the existing `drones` and
// `aircraft` fields.
func TestStateHandler_IncludesNormalizedTracks(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(TaggedFrame{
		Source: "adsb", Type: "normalized-track", SellerPeerID: "PID",
		ReceivedAt: time.Now().UTC(),
		Frame: []byte(
			`{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:00:00Z","source":"adsb","entityType":"aircraft","entityID":"ABCDEF","position":{"lat":51.5,"lon":-0.4,"altitudeM":10000}}`,
		),
	})
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	stateHandler(state)(rr, httptest.NewRequest(http.MethodGet, "/state.json", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Drones           []DroneSnapshot           `json:"drones"`
		Aircraft         []AircraftSnapshot        `json:"aircraft"`
		NormalizedTracks []NormalizedTrackSnapshot `json:"normalizedTracks"`
		Count            int                       `json:"count"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, 1, body.Count)
	assert.Empty(t, body.Drones)
	assert.Empty(t, body.Aircraft)
	require.Len(t, body.NormalizedTracks, 1)
	assert.Equal(t, "ABCDEF", body.NormalizedTracks[0].EntityID)
	assert.InDelta(t, 51.5, body.NormalizedTracks[0].Lat, 1e-9, "/state.json carries REAL lat")
}

// TestSSE_DeliversNormalizedTrackUpdate spins up the events handler and
// asserts a NormalizedTrack apply propagates as kind="normalized-track"
// over the SSE stream.
func TestSSE_DeliversNormalizedTrackUpdate(t *testing.T) {
	t.Parallel()
	state := NewState()
	server := httptest.NewServer(http.HandlerFunc(eventsHandler(state)))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = state.Apply(TaggedFrame{
			Source: "adsb", Type: "normalized-track", SellerPeerID: "PID",
			ReceivedAt: time.Now().UTC(),
			Frame: []byte(
				`{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:00:00Z","source":"adsb","entityType":"aircraft","entityID":"SSE-ENTITY","position":{"lat":52.3,"lon":4.9,"altitudeM":10000}}`,
			),
		})
	}()

	buf := make([]byte, 4096)
	deadline := time.Now().Add(2 * time.Second)
	var combined strings.Builder
	for time.Now().Before(deadline) && !strings.Contains(combined.String(), "SSE-ENTITY") {
		n, err := resp.Body.Read(buf)
		if err != nil {
			break
		}
		combined.Write(buf[:n])
	}
	got := combined.String()
	assert.Contains(t, got, "SSE-ENTITY", "expected SSE stream to carry the normalized-track entityID")
	assert.Contains(t, got, `"kind":"normalized-track"`, "SSE event must carry kind=normalized-track")
	assert.Contains(t, got, `"normalizedTrack"`, "envelope must include the normalizedTrack payload")
	assert.Contains(t, got, "event: update", "expected event: update header")
}

// TestSSE_NormalizedTrackInitialSnapshot asserts a NormalizedTrack
// already present in state is replayed in the initial SSE snapshot
// batch with kind="normalized-track" + event: snapshot.
func TestSSE_NormalizedTrackInitialSnapshot(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(TaggedFrame{
		Source: "adsb", Type: "normalized-track", SellerPeerID: "PID",
		ReceivedAt: time.Now().UTC(),
		Frame: []byte(
			`{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:00:00Z","source":"adsb","entityType":"aircraft","entityID":"REPLAY1","position":{"lat":51.5,"lon":-0.4,"altitudeM":10000}}`,
		),
	})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(eventsHandler(state)))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	deadline := time.Now().Add(2 * time.Second)
	var combined strings.Builder
	for time.Now().Before(deadline) && !strings.Contains(combined.String(), "REPLAY1") {
		n, err := resp.Body.Read(buf)
		if err != nil {
			break
		}
		combined.Write(buf[:n])
	}
	got := combined.String()
	assert.Contains(t, got, "event: snapshot", "initial state replay uses event: snapshot")
	assert.Contains(t, got, `"kind":"normalized-track"`, "initial snapshot batch includes normalized-track items")
	assert.Contains(t, got, "REPLAY1", "specific entityID surfaces in the initial batch")
}

// TestTCPListenerIngest_NormalizedTrack asserts the TCP listener accepts
// multistream-buyer-style JSONL with type="normalized-track" and routes it
// correctly through Apply.
func TestTCPListenerIngest_NormalizedTrack(t *testing.T) {
	t.Parallel()

	state := NewState()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	// Discard logger: handleConn writes a "buyer disconnected" line after
	// scanner.Scan returns false (the conn closed); routing that through
	// t.Log races with t's lifecycle when goroutine exit is not strictly
	// synchronised. io.Discard sidesteps the issue without losing
	// coverage — the assertions below are on State, not on log output.
	discardLogger := discardLogger()
	done := make(chan struct{})
	go func() {
		defer close(done)
		acceptLoop(ctx, listener, state, discardLogger)
	}()
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		<-done
	})

	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	for _, eid := range []string{"AAAA01", "BBBB02", "AAAA01"} {
		rec := fmt.Sprintf(
			`{"source":"adsb","type":"normalized-track","sellerPeerID":"PID","receivedAt":"2026-05-14T14:00:00Z","frame":{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:00:00Z","source":"adsb","entityType":"aircraft","entityID":"%s","position":{"lat":51,"lon":-0.5,"altitudeM":10000}}}`,
			eid,
		)
		_, err := conn.Write([]byte(rec + "\n"))
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		return len(state.NormalizedTracksSnapshot()) == 2
	}, 2*time.Second, 20*time.Millisecond,
		"expected 2 unique normalizedTracks (AAAA01 dedupes, BBBB02 distinct)")

	// Existing keyspaces untouched.
	assert.Empty(t, state.Snapshot())
	assert.Empty(t, state.AircraftSnapshot())
}

// TestApply_NormalizedTrackVelocityPresenceFlags pins the truthfulness fix for
// the "speed 0.0 / heading 0.0" bug: the snapshot carries additive
// hasGroundSpeed/hasHeading/hasVerticalRate flags so the UI can distinguish
// "velocity not decoded" (flag false → render "—") from a genuine zero
// (flag true, value 0 → render "0.0"). Per-sub-field presence is honored.
func TestApply_NormalizedTrackVelocityPresenceFlags(t *testing.T) {
	t.Parallel()

	apply := func(velJSON string) *NormalizedTrackSnapshot {
		state := NewState()
		frame := fmt.Sprintf(
			`{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:00:00Z","source":"adsb","entityType":"aircraft","entityID":"A1B2C3","position":{"lat":51,"lon":-0.5,"altitudeM":10000}%s}`,
			velJSON,
		)
		event, err := state.Apply(TaggedFrame{
			Source: "adsb", Type: "normalized-track", SellerPeerID: "PID",
			ReceivedAt: time.Now().UTC(), Frame: []byte(frame),
		})
		require.NoError(t, err)
		require.NotNil(t, event.NormalizedTrack)
		return event.NormalizedTrack
	}

	t.Run("velocity present sets flags and values", func(t *testing.T) {
		got := apply(`,"velocity":{"groundSpeedMps":249.5,"headingDeg":290.0,"verticalRateMps":6.5}`)
		assert.True(t, got.HasGroundSpeed)
		assert.InDelta(t, 249.5, got.GroundSpeedMps, 1e-9)
		assert.True(t, got.HasHeading)
		assert.InDelta(t, 290.0, got.HeadingDeg, 1e-9)
		assert.True(t, got.HasVerticalRate)
		assert.InDelta(t, 6.5, got.VerticalRateMps, 1e-9)
	})

	t.Run("velocity absent clears flags", func(t *testing.T) {
		got := apply(``) // no velocity block on the wire
		assert.False(t, got.HasGroundSpeed, "absent velocity ⇒ hasGroundSpeed=false (UI shows —)")
		assert.Zero(t, got.GroundSpeedMps)
		assert.False(t, got.HasHeading)
		assert.Zero(t, got.HeadingDeg)
		assert.False(t, got.HasVerticalRate)
	})

	t.Run("partial velocity sets only present sub-fields", func(t *testing.T) {
		got := apply(`,"velocity":{"groundSpeedMps":100.0}`)
		assert.True(t, got.HasGroundSpeed)
		assert.InDelta(t, 100.0, got.GroundSpeedMps, 1e-9)
		assert.False(t, got.HasHeading, "headingDeg absent ⇒ hasHeading=false even when speed present")
	})

	t.Run("explicit zero is a known value, not unknown", func(t *testing.T) {
		got := apply(`,"velocity":{"groundSpeedMps":0.0,"headingDeg":0.0}`)
		assert.True(t, got.HasGroundSpeed, "explicit 0 on the wire is a KNOWN value")
		assert.Zero(t, got.GroundSpeedMps)
		assert.True(t, got.HasHeading)
		assert.Zero(t, got.HeadingDeg)
	})
}
