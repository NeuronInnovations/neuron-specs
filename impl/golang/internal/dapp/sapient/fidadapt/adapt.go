// Package fidadapt projects a SAPIENT SapientMessage into the Remote ID DISPLAY
// frame the FID map renders on its drone path (TaggedFrame source="remote-id" ->
// fid-display applyDrone). This is a lossy display projection — the canonical
// SAPIENT wire stays protobuf on /sapient/detection/2.0.0. It reuses the existing
// internal/dapp/remoteid canonical-JSON RemoteIdFrame format (FromDecoded), so
// SAPIENT detections display as a drone + paired operator marker with ZERO
// changes to cmd/fid-display.
//
// The mapping is a faithful field copy of the bridge's already-decoded
// DetectionReport — it does NOT re-decode OpenDroneID (the bridge handles the
// ASTM F3411 INVALID sentinels). Velocity is omitted when the report carries
// none, so the display never shows a fake 0.0.
package fidadapt

import (
	"fmt"
	"math"
	"strconv"

	dappremoteid "github.com/neuron-sdk/neuron-go-sdk/internal/dapp/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	feedsremoteid "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/remoteid"
)

// ToRemoteIdFrame adapts a SAPIENT SapientMessage (its DetectionReport) into a
// RemoteIdFrame display projection (canonical-JSON in the remoteid sense; not the
// canonical SAPIENT wire). Returns an error when the message carries no
// DetectionReport or no usable identity.
func ToRemoteIdFrame(msg *sapientpb.SapientMessage) (dappremoteid.RemoteIdFrame, error) {
	dr := msg.GetDetectionReport()
	if dr == nil {
		return dappremoteid.RemoteIdFrame{}, fmt.Errorf("fidadapt: message carries no DetectionReport")
	}
	info := indexObjectInfo(dr.GetObjectInfo())

	droneID := dr.GetId() // native tail-number = RID serial
	if droneID == "" {
		droneID = dr.GetObjectId() // fall back to the ULID track id
	}
	if droneID == "" {
		return dappremoteid.RemoteIdFrame{}, fmt.Errorf("fidadapt: DetectionReport has no usable id")
	}

	df := feedsremoteid.DecodedFrame{
		DroneID:     droneID,
		DroneIDType: droneIDType(info["rid.idType"]),
		Source:      "sapient-rid", // inner frame.source -> fid-display feed badge
		ObservedAt:  msg.GetTimestamp().AsTime(),
	}

	if loc := dr.GetLocation(); loc != nil {
		df.Position = &feedsremoteid.Position{
			Lat: loc.GetY(), // proto Y = latitude
			Lon: loc.GetX(), // proto X = longitude
			Alt: loc.GetZ(),
			Fix: fixFromStatus(info["rid.status"]),
		}
	}

	// ENU velocity -> speed + true-track heading. Omit entirely when absent so
	// the display shows nothing rather than a fabricated 0.0 (presence-aware).
	if v := dr.GetEnuVelocity(); v != nil {
		e, n := v.GetEastRate(), v.GetNorthRate()
		df.Velocity = &feedsremoteid.Velocity{
			SpeedHorizontal: math.Hypot(e, n),
			SpeedVertical:   v.GetUpRate(),
			Track:           trackDeg(e, n),
		}
	}

	// Operator -> stationary pilot marker. Needs at least a position.
	if lat, ok := parseFloat(info["rid.operatorLatDeg"]); ok {
		lon, _ := parseFloat(info["rid.operatorLonDeg"])
		alt, _ := parseFloat(info["rid.operatorAltM"])
		id := info["rid.operatorId"]
		if id == "" {
			id = droneID + "-operator"
		}
		df.Operator = &feedsremoteid.Operator{
			ID:       id,
			IDType:   info["rid.operatorIdType"],
			Position: &feedsremoteid.Position{Lat: lat, Lon: lon, Alt: alt, Fix: "2D"},
		}
	}

	return dappremoteid.FromDecoded(df), nil
}

func indexObjectInfo(infos []*sapientpb.DetectionReport_TrackObjectInfo) map[string]string {
	m := make(map[string]string, len(infos))
	for _, oi := range infos {
		if oi.GetType() != "" {
			m[oi.GetType()] = oi.GetValue()
		}
	}
	return m
}

// droneIDType normalizes the SAPIENT rid.idType label to the RemoteId
// droneIdType vocabulary; defaults to "serial" (RemoteIdFrame requires a
// non-empty droneIdType to marshal).
func droneIDType(s string) string {
	switch s {
	case "", "SerialNumber":
		return "serial"
	case "CAARegistration":
		return "caa"
	case "UTMAssignedUUID":
		return "utm"
	case "SpecificSessionID":
		return "specific-session"
	default:
		return s
	}
}

func fixFromStatus(status string) string {
	switch status {
	case "Airborne", "Emergency", "RemoteIDFailure":
		return "3D"
	case "Ground":
		return "2D"
	default:
		return ""
	}
}

// trackDeg converts an East/North vector to a true-track heading in degrees
// [0,360); 0°/360° = due north, 90° = due east.
func trackDeg(e, n float64) float64 {
	if e == 0 && n == 0 {
		return 0
	}
	deg := math.Atan2(e, n) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}

func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
