package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// SensorLocation is one entry in the operator-provided sensor-locations.json.
// It is the ONLY safe source of a sensor/receiver's physical position today:
// the SAPIENT proto can carry node location (StatusReport.node_location,
// Registration.location_definition) but the seller stack never populates it, and
// DroneScout's self-location message is dropped by the bridge. So the operator
// declares it here. Never inferred from drone/operator/RF positions.
type SensorLocation struct {
	SensorID    string   `json:"sensorId"`
	AgentID     string   `json:"agentId"`
	SellerEVM   string   `json:"sellerEVM"`
	PeerID      string   `json:"peerID"`
	NodeID      string   `json:"nodeId"`
	Label       string   `json:"label"`
	Lat         float64  `json:"lat"`
	Lon         float64  `json:"lon"`
	AltM        *float64 `json:"altM"`
	Source      string   `json:"source"`
	Confidence  string   `json:"confidence"`
	LastUpdated string   `json:"lastUpdated"`
}

// SensorView is the whitelisted projection served at /sensors.json.
type SensorView struct {
	SensorID    string   `json:"sensorId,omitempty"`
	AgentID     string   `json:"agentId,omitempty"`
	SellerEVM   string   `json:"sellerEVM,omitempty"`
	PeerID      string   `json:"peerID,omitempty"`
	NodeID      string   `json:"nodeId,omitempty"`
	Label       string   `json:"label,omitempty"`
	Lat         float64  `json:"lat"`
	Lon         float64  `json:"lon"`
	AltM        *float64 `json:"altM,omitempty"`
	Source      string   `json:"source"`
	Confidence  string   `json:"confidence,omitempty"`
	LastUpdated string   `json:"lastUpdated,omitempty"`
}

// validSensorSources is the closed provenance vocabulary. Empty normalizes to
// "configured"; an unknown non-empty value is rejected so we never mislabel how a
// location was obtained.
var validSensorSources = map[string]bool{
	"configured":          true,
	"dronescout-location": true,
	"status-report":       true,
	"estimated":           true,
}

// sensorsHandler serves the operator-provided sensor layer. Off (empty list) when
// --sensors is unset. A file/parse error is surfaced in "error" with a 200 so the
// rest of the explorer keeps working; per-entry rejections appear in "warnings".
func (s *server) sensorsHandler(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"sensors": []SensorView{},
		"count":   0,
		"source":  "config-file",
	}
	if s.cfg.sensorsPath == "" {
		writeJSON(w, http.StatusOK, out) // layer off by default
		return
	}
	out["configPath"] = s.cfg.sensorsPath
	sensors, warnings, err := loadSensors(s.cfg.sensorsPath)
	if err != nil {
		out["error"] = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	out["sensors"] = sensors
	out["count"] = len(sensors)
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	writeJSON(w, http.StatusOK, out)
}

// loadSensors reads + validates the operator's sensor-locations.json. File-level
// problems return err; per-entry problems exclude the entry and append a warning
// (rejected, never silently dropped). Re-read per request so live edits show.
func loadSensors(path string) ([]SensorView, []string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read sensor config: %w", err)
	}
	var raw []SensorLocation
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, nil, fmt.Errorf("parse sensor config (expect a JSON array): %w", err)
	}
	out := make([]SensorView, 0, len(raw))
	var warnings []string
	for i, sl := range raw {
		who := sl.SensorID
		if who == "" {
			who = fmt.Sprintf("entry %d", i)
		}
		if sl.SensorID == "" && sl.AgentID == "" && sl.PeerID == "" && sl.NodeID == "" {
			warnings = append(warnings, fmt.Sprintf("%s: rejected — no identity (need one of sensorId/agentId/peerID/nodeId)", who))
			continue
		}
		if sl.Lat < -90 || sl.Lat > 90 || sl.Lon < -180 || sl.Lon > 180 {
			warnings = append(warnings, fmt.Sprintf("%s: rejected — lat/lon out of range (%.6f,%.6f)", who, sl.Lat, sl.Lon))
			continue
		}
		if sl.Lat == 0 && sl.Lon == 0 {
			warnings = append(warnings, fmt.Sprintf("%s: rejected — null-island (0,0); provide a real position or omit", who))
			continue
		}
		source := sl.Source
		if source == "" {
			source = "configured"
		} else if !validSensorSources[source] {
			warnings = append(warnings, fmt.Sprintf("%s: rejected — unknown source %q (want configured|dronescout-location|status-report|estimated)", who, source))
			continue
		}
		out = append(out, SensorView{
			SensorID: sl.SensorID, AgentID: sl.AgentID, SellerEVM: sl.SellerEVM,
			PeerID: sl.PeerID, NodeID: sl.NodeID, Label: sl.Label,
			Lat: sl.Lat, Lon: sl.Lon, AltM: sl.AltM,
			Source: source, Confidence: sl.Confidence, LastUpdated: sl.LastUpdated,
		})
	}
	return out, warnings, nil
}
