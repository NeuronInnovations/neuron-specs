package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sensorsResponse(t *testing.T, path string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	sensorsHandler(path)(rec, httptest.NewRequest(http.MethodGet, "/sensors.json", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

func writeSensors(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sensors.json")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func TestSensorsHandler_UnsetIsEmptyLayer(t *testing.T) {
	t.Parallel()
	out := sensorsResponse(t, "")
	assert.EqualValues(t, 0, out["count"])
	_, hasErr := out["error"]
	assert.False(t, hasErr)
}

func TestSensorsHandler_ValidEntry(t *testing.T) {
	t.Parallel()
	p := writeSensors(t, `[{"sensorId":"ds-1","label":"Land's End","lat":50.1027,"lon":-5.6705,"nodeId":"node-rid"}]`)
	out := sensorsResponse(t, p)
	assert.EqualValues(t, 1, out["count"])
	sensors := out["sensors"].([]any)
	s := sensors[0].(map[string]any)
	assert.Equal(t, "configured", s["source"], "empty source normalizes to configured")
	assert.Equal(t, "node-rid", s["nodeId"])
	_, hasWarn := out["warnings"]
	assert.False(t, hasWarn)
}

func TestSensorsHandler_MissingFileSurfacesError(t *testing.T) {
	t.Parallel()
	out := sensorsResponse(t, filepath.Join(t.TempDir(), "nope.json"))
	assert.EqualValues(t, 0, out["count"])
	assert.Contains(t, out["error"], "read sensor config")
}

func TestSensorsHandler_ValidationRejections(t *testing.T) {
	t.Parallel()
	p := writeSensors(t, `[
		{"label":"no identity","lat":50,"lon":-5},
		{"sensorId":"null-island","lat":0,"lon":0},
		{"sensorId":"out-of-range","lat":91,"lon":0},
		{"sensorId":"bad-source","lat":50,"lon":-5,"source":"guessed"},
		{"sensorId":"good","lat":50.5,"lon":-5.5,"source":"estimated"}
	]`)
	out := sensorsResponse(t, p)
	assert.EqualValues(t, 1, out["count"], "only the valid entry survives")
	warnings := out["warnings"].([]any)
	assert.Len(t, warnings, 4, "every rejection is surfaced, never silent")
	s := out["sensors"].([]any)[0].(map[string]any)
	assert.Equal(t, "estimated", s["source"])
}

func TestSensorsHandler_MalformedFileGraceful(t *testing.T) {
	t.Parallel()
	p := writeSensors(t, `{not json`)
	out := sensorsResponse(t, p)
	assert.Contains(t, out["error"], "parse sensor config")
}
