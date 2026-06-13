package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

func writeSensors(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sensor-locations.json")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

type sensorsResp struct {
	Sensors  []SensorView `json:"sensors"`
	Count    int          `json:"count"`
	Source   string       `json:"source"`
	Warnings []string     `json:"warnings"`
	Error    string       `json:"error"`
}

func getSensors(t *testing.T, cfg config) sensorsResp {
	t.Helper()
	ts := testServer(t, cfg)
	resp, body := getBody(t, ts.URL+"/sensors.json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out sensorsResp
	require.NoError(t, json.Unmarshal(body, &out))
	return out
}

func TestSensorsHandler_Valid(t *testing.T) {
	p := writeSensors(t, `[
	  {"sensorId":"ds220500000100","agentId":"1","peerID":"16UiuPeer","nodeId":"node-1",
	   "label":"Demo Receiver","lat":50.1027,"lon":-5.6705,"altM":30,
	   "source":"configured","confidence":"exact","lastUpdated":"2026-06-08T00:00:00Z"}
	]`)
	out := getSensors(t, config{evidenceDir: t.TempDir(), sensorsPath: p})
	require.Empty(t, out.Error)
	require.Empty(t, out.Warnings)
	require.Equal(t, 1, out.Count)
	require.Equal(t, "config-file", out.Source)
	s := out.Sensors[0]
	require.Equal(t, "ds220500000100", s.SensorID)
	require.Equal(t, "1", s.AgentID)
	require.Equal(t, "Demo Receiver", s.Label)
	require.Equal(t, 50.1027, s.Lat)
	require.Equal(t, -5.6705, s.Lon)
	require.Equal(t, "configured", s.Source)
	require.Equal(t, "exact", s.Confidence)
}

func TestSensorsHandler_UnsetIsEmpty(t *testing.T) {
	out := getSensors(t, config{evidenceDir: t.TempDir()}) // no sensorsPath
	require.Equal(t, 0, out.Count)
	require.Empty(t, out.Sensors)
	require.Empty(t, out.Error)
}

func TestSensorsHandler_MissingFileIsEmpty(t *testing.T) {
	out := getSensors(t, config{evidenceDir: t.TempDir(), sensorsPath: filepath.Join(t.TempDir(), "nope.json")})
	require.Equal(t, 0, out.Count)
	require.NotEmpty(t, out.Error, "missing file is surfaced, not silent")
}

func TestSensorsHandler_InvalidLatLonRejected(t *testing.T) {
	p := writeSensors(t, `[
	  {"sensorId":"ok","lat":50.1,"lon":-5.6,"source":"configured"},
	  {"sensorId":"bad-lat","lat":999,"lon":-5.6,"source":"configured"},
	  {"sensorId":"bad-lon","lat":50.1,"lon":-200,"source":"configured"}
	]`)
	out := getSensors(t, config{evidenceDir: t.TempDir(), sensorsPath: p})
	require.Equal(t, 1, out.Count, "only the in-range entry survives")
	require.Equal(t, "ok", out.Sensors[0].SensorID)
	require.Len(t, out.Warnings, 2, "both out-of-range entries are reported")
}

func TestSensorsHandler_NullIslandRejected(t *testing.T) {
	p := writeSensors(t, `[{"sensorId":"zero","lat":0,"lon":0,"source":"configured"}]`)
	out := getSensors(t, config{evidenceDir: t.TempDir(), sensorsPath: p})
	require.Equal(t, 0, out.Count)
	require.NotEmpty(t, out.Warnings)
}

func TestSensorsHandler_SourceNormalizeAndReject(t *testing.T) {
	p := writeSensors(t, `[
	  {"sensorId":"empty-src","lat":50.1,"lon":-5.6},
	  {"sensorId":"bad-src","lat":50.2,"lon":-5.7,"source":"made-up"},
	  {"sensorId":"estimated-ok","lat":50.3,"lon":-5.8,"source":"estimated"}
	]`)
	out := getSensors(t, config{evidenceDir: t.TempDir(), sensorsPath: p})
	require.Equal(t, 2, out.Count, "empty-src normalized + estimated kept; made-up rejected")
	byID := map[string]SensorView{}
	for _, s := range out.Sensors {
		byID[s.SensorID] = s
	}
	require.Equal(t, "configured", byID["empty-src"].Source, "empty source defaults to configured")
	require.Equal(t, "estimated", byID["estimated-ok"].Source)
	require.NotContains(t, byID, "bad-src")
	require.Len(t, out.Warnings, 1)
}

func TestSensorsHandler_NoIdentityRejected(t *testing.T) {
	p := writeSensors(t, `[{"label":"anon","lat":50.1,"lon":-5.6,"source":"configured"}]`)
	out := getSensors(t, config{evidenceDir: t.TempDir(), sensorsPath: p})
	require.Equal(t, 0, out.Count, "an entry with no identity field cannot anchor/link")
	require.NotEmpty(t, out.Warnings)
}

func TestSensorsHandler_MalformedFileGraceful(t *testing.T) {
	p := writeSensors(t, `{not an array`)
	out := getSensors(t, config{evidenceDir: t.TempDir(), sensorsPath: p})
	require.Equal(t, 0, out.Count)
	require.NotEmpty(t, out.Error, "parse error surfaced, page still serves")
}

func TestSensorsHandler_NoSecretLeak(t *testing.T) {
	p := writeSensors(t, `[{"sensorId":"s1","agentId":"1","lat":50.1,"lon":-5.6,"source":"configured"}]`)
	ts := testServer(t, config{evidenceDir: t.TempDir(), sensorsPath: p})
	_, body := getBody(t, ts.URL+"/sensors.json")
	suspicious := regexp.MustCompile(`(?i)"(priv(ate)?_?key|secret|mnemonic|seed_?phrase|passphrase)"\s*:`)
	require.False(t, suspicious.Match(body))
	require.NotContains(t, strings.ToLower(string(body)), "privatekey")
}

func TestSensorsHandler_LinksToAgent(t *testing.T) {
	dir := t.TempDir()
	contract := registry.NewMemoryRegistryContract()
	ev, _ := seedAgent(t, dir, contract)
	p := writeSensors(t, `[{"sensorId":"s1","agentId":"`+ev.AgentID+`","lat":50.1,"lon":-5.6,"source":"configured"}]`)
	out := getSensors(t, config{evidenceDir: dir, sensorsPath: p})
	require.Equal(t, 1, out.Count)
	require.Equal(t, ev.AgentID, out.Sensors[0].AgentID, "sensor carries the agentId the client links on")
}
