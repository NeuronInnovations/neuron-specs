package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adsbTrackJSON is a v1.1.0 ADS-B SapientTrackSnapshot as emitted by
// cmd/sapient-fid-consumer for a JetVision bridge report (kind=adsb; no rid,
// no cot).
const adsbTrackJSON = `{
  "type":"sapient-track","version":"1.1.0",
  "uid":"F-GSPI",
  "objectId":"01KTTAWFKR2422ACV6CTTT684M",
  "reportId":"01KTTAWFKR2422ACV6CTTT684M@1769432400131304119",
  "nodeId":"jv-node-1111-2222-3333-444444444444",
  "position":{"lat":50.03406,"lon":-6.69565,"alt":9563.1},
  "velocity":{"speedMps":197.5,"trackDeg":284},
  "classification":{"type":"Air Vehicle","confidence":1},
  "feedSource":"live",
  "wire":"BSI Flex 335 v2.0 protobuf",
  "rf":{"rssiDbm":-62,"frequencyHz":1090000000},
  "agent":{"agentId":"2","sellerEVM":"0xJV","peerID":"16Uiu2JV",
           "nodeId":"jv-node-1111-2222-3333-444444444444",
           "service":"jetvision-adsb-sapient","protocol":"/sapient/detection/2.0.0","simulated":true},
  "kind":"adsb",
  "adsb":{"icao24":"3949E8","callsign":"AFR650","registration":"F-GSPI","typeCode":"A343",
          "emitterCategory":"A5","squawk":"1000","airGround":"A","source":"A","provenance":"local",
          "baroAltFt":32000,"geoAltFt":31375}
}`

// TestApply_CompositeKey_TwoNodesSameUID: the same uid arriving from two
// different source nodes must be two distinct tracks (multi-source rule:
// never key by object identity alone).
func TestApply_CompositeKey_TwoNodesSameUID(t *testing.T) {
	t.Parallel()
	state := NewState()

	_, err := state.Apply(taggedTrack(t, `{"uid":"SAME","nodeId":"node-rid","position":{"lat":1,"lon":2,"alt":3}}`))
	require.NoError(t, err)
	_, err = state.Apply(taggedTrack(t, `{"uid":"SAME","nodeId":"node-jv","position":{"lat":4,"lon":5,"alt":6}}`))
	require.NoError(t, err)

	snaps := state.Snapshot()
	require.Len(t, snaps, 2, "same uid from two nodes = two tracks")

	// Re-applying for the same node increments ITS frame count only.
	_, err = state.Apply(taggedTrack(t, `{"uid":"SAME","nodeId":"node-rid","position":{"lat":1.1,"lon":2,"alt":3}}`))
	require.NoError(t, err)
	counts := map[string]uint64{}
	for _, s := range state.Snapshot() {
		counts[s.NodeID] = s.FrameCount
	}
	assert.Equal(t, uint64(2), counts["node-rid"])
	assert.Equal(t, uint64(1), counts["node-jv"])
}

// TestApply_LegacyEmptyNodeID_BackCompat: frames without a nodeId keep today's
// bare-uid keying — replace-in-place, exactly one track.
func TestApply_LegacyEmptyNodeID_BackCompat(t *testing.T) {
	t.Parallel()
	state := NewState()

	_, err := state.Apply(taggedTrack(t, `{"uid":"U1","position":{"lat":1,"lon":2,"alt":3}}`))
	require.NoError(t, err)
	_, err = state.Apply(taggedTrack(t, `{"uid":"U1","position":{"lat":9,"lon":8,"alt":7}}`))
	require.NoError(t, err)

	snaps := state.Snapshot()
	require.Len(t, snaps, 1)
	assert.Equal(t, uint64(2), snaps[0].FrameCount)
}

// TestApply_ADSBTrackFields: the v1.1.0 adsb contract lands in state intact.
func TestApply_ADSBTrackFields(t *testing.T) {
	t.Parallel()
	state := NewState()

	_, err := state.Apply(taggedTrack(t, adsbTrackJSON))
	require.NoError(t, err)

	snaps := state.Snapshot()
	require.Len(t, snaps, 1)
	s := snaps[0]
	assert.Equal(t, "adsb", s.Kind)
	assert.Equal(t, "1.1.0", s.ContractVersion)
	require.NotNil(t, s.ADSB)
	assert.Equal(t, "3949E8", s.ADSB.ICAO24)
	assert.Equal(t, "AFR650", s.ADSB.Callsign)
	assert.Equal(t, "F-GSPI", s.ADSB.Registration)
	assert.Equal(t, "A5", s.ADSB.EmitterCategory)
	assert.Equal(t, "local", s.ADSB.Provenance)
	require.NotNil(t, s.ADSB.BaroAltFt)
	assert.InDelta(t, 32000, *s.ADSB.BaroAltFt, 1e-9)
	assert.Nil(t, s.RID, "no rid block on an adsb track")
	assert.Nil(t, s.CoT, "no cot block on an adsb track")
	require.NotNil(t, s.Agent)
	assert.Equal(t, "jetvision-adsb-sapient", s.Agent.Service)
}

// TestStateJSON_ADSBOmittedForRID: a rid track's serialized state carries no
// kind/adsb keys (raw-key contract check).
func TestStateJSON_ADSBOmittedForRID(t *testing.T) {
	t.Parallel()
	state := NewState()
	_, err := state.Apply(taggedTrack(t, fullTrackJSON))
	require.NoError(t, err)

	b, err := json.Marshal(state.Snapshot()[0])
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &raw))
	_, hasKind := raw["kind"]
	_, hasADSB := raw["adsb"]
	assert.False(t, hasKind, "rid snapshot must not grow a kind key")
	assert.False(t, hasADSB, "rid snapshot must not grow an adsb key")
}

// TestFocusFor_ADSBDoesNotHijackDroneFocus: auto-focus is a drone-demo
// behavior — a cluster of airliners (kind=adsb) must not steal the viewport
// from a single live drone.
func TestFocusFor_ADSBDoesNotHijackDroneFocus(t *testing.T) {
	t.Parallel()
	state := NewState()

	// One drone near Falmouth.
	_, err := state.Apply(taggedTrack(t, `{"uid":"DRONE","nodeId":"node-rid","position":{"lat":50.1027,"lon":-5.6635,"alt":95}}`))
	require.NoError(t, err)
	// Three airliners clustered over the Channel, far away.
	for i := range 3 {
		inner := fmt.Sprintf(`{"uid":"AC%d","nodeId":"node-jv","kind":"adsb","position":{"lat":49.9%d,"lon":-6.2%d,"alt":11000}}`, i, i, i)
		_, err = state.Apply(taggedTrack(t, inner))
		require.NoError(t, err)
	}

	f := FocusFor(state.Snapshot())
	require.NotNil(t, f)
	assert.Equal(t, 1, f.Count, "focus stays on the drone cluster")
	assert.InDelta(t, 50.1027, f.Lat, 1e-3)
	assert.InDelta(t, -5.6635, f.Lon, 1e-3)
}

// TestFocusFor_ADSBOnlyFallback: with no rid track on the map (JV standalone),
// the aircraft cluster still provides a usable initial viewport.
func TestFocusFor_ADSBOnlyFallback(t *testing.T) {
	t.Parallel()
	state := NewState()
	for i := range 2 {
		inner := fmt.Sprintf(`{"uid":"AC%d","nodeId":"node-jv","kind":"adsb","position":{"lat":49.9%d,"lon":-6.2%d,"alt":11000}}`, i, i, i)
		_, err := state.Apply(taggedTrack(t, inner))
		require.NoError(t, err)
	}

	f := FocusFor(state.Snapshot())
	require.NotNil(t, f, "adsb-only state still yields a focus")
	assert.Equal(t, 2, f.Count)
}

// TestEvict_CompositeKeysIndependent: staleness is per composite key — a stale
// jv track evicts while the fresh rid track with the SAME uid survives.
func TestEvict_CompositeKeysIndependent(t *testing.T) {
	t.Parallel()
	state := NewState()

	stale := taggedTrack(t, `{"uid":"SAME","nodeId":"node-jv","kind":"adsb","position":{"lat":1,"lon":2,"alt":3}}`)
	stale.ReceivedAt = time.Now().Add(-time.Hour)
	_, err := state.Apply(stale)
	require.NoError(t, err)

	fresh := taggedTrack(t, `{"uid":"SAME","nodeId":"node-rid","position":{"lat":1,"lon":2,"alt":3}}`)
	fresh.ReceivedAt = time.Now()
	_, err = state.Apply(fresh)
	require.NoError(t, err)

	n := state.Evict(time.Now(), 10*time.Minute)
	assert.Equal(t, 1, n)
	snaps := state.Snapshot()
	require.Len(t, snaps, 1)
	assert.Equal(t, "node-rid", snaps[0].NodeID)
}
