package remoteid

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DroneScout MQTT JSON parser + normalizer — Stage B.
//
// **Scope (Stage B)**:
//   - ParseSensorPayload handles plain-UTF-8 JSON, one-or-more
//     top-level objects concatenated in a single MQTT message,
//     trailing 0x00 / 0x0a cleanup. compression == "none" only.
//   - NormalizeToDecodedFrame maps a data-kind SensorMessage into a
//     DecodedFrame. The OpenDroneID UASdata binary is **preserved as
//     base64** on DecodedFrame.DroneID with
//     DroneIDType = "uasdata-base64"; byte-level decoding of
//     UASdata (position, velocity, operator) is Stage C work.
//
// **Out of Stage B**: LZMA decompression; UASdata byte-level decode;
// aircraft/mobile-network sub-objects (they are recognised but not
// surfaced as decoded sub-structs).
//
// **Feed-source classification**: a seller reading these JSON bytes
// from disk advertises `feedSource = "replay"` per spec 017 FR-R15.
// `live` only applies to seller runs that subscribe to a real MQTT
// broker (Stage C).
//
// The JSON envelope reference is the DroneScout MQTT research notes
// (§5 + §6); staged rollout follows the live-feed plan.

// SensorMessageKind discriminates the top-level sub-object inside a
// DroneScout MQTT payload. Each payload carries `protocol: 1.0` plus
// exactly one of these blocks.
type SensorMessageKind string

const (
	// SensorMessageData — Remote ID broadcast detection.
	SensorMessageData SensorMessageKind = "data"
	// SensorMessageStatus — sensor health / firmware version.
	SensorMessageStatus SensorMessageKind = "status"
	// SensorMessageLocation — sensor self-location (LTE add-on only).
	SensorMessageLocation SensorMessageKind = "location"
	// SensorMessageAircraft — ADS-B/UAT (ADS-B add-on only). Out of scope
	// for this DApp; 016 ADS-B owns aircraft messages.
	SensorMessageAircraft SensorMessageKind = "aircraft"
	// SensorMessageMobileNetwork — LTE network stats. Out of scope.
	SensorMessageMobileNetwork SensorMessageKind = "mobile-network"
)

// SensorMessage is one decoded DroneScout MQTT JSON object. At most one
// of Data / Status / Location is non-nil; Kind matches. Aircraft and
// MobileNetwork kinds are recognised but produce no sub-struct in
// Stage B.
type SensorMessage struct {
	Kind     SensorMessageKind
	Protocol float64
	Data     *SensorDataMessage
	Status   *SensorStatusMessage
	Location *SensorLocationMessage
}

// SensorDataMessage carries one detected Remote ID broadcast envelope.
// Drone position / velocity / operator sit inside UASdataB64; this
// struct only surfaces the JSON envelope. NormalizeToDecodedFrame
// preserves UASdataB64 verbatim until Stage C decodes it.
type SensorDataMessage struct {
	// SensorID identifies the receiver; arbitrary operator-set string.
	SensorID string
	// RSSI in dBm.
	RSSI int
	// Channel is the RF channel index.
	Channel int
	// Timestamp is milliseconds since Unix epoch (UTC).
	Timestamp int64
	// MACAddress is the RID transmitter MAC, e.g. "AA:BB:CC:DD:EE:FF".
	MACAddress string
	// Type is the RID transmission type, e.g. "bt4", "bt5",
	// "wlan-beacon", "wlan-nan".
	Type string
	// UASdataB64 is the base64-encoded ODID_UAS_Data C-struct dump.
	// Stage B preserves the string verbatim into
	// DecodedFrame.DroneID; Stage C decodes the byte layout per
	// research-report §6.
	UASdataB64 string
	// RawB64 is the optional base64 raw RF bytes; present only when
	// the sensor has `raw_data = 1` set (firmware ≥ 20240717-1353).
	// Stage B ignores it.
	RawB64 string
}

// SensorStatusMessage is the sensor's periodic health beat.
type SensorStatusMessage struct {
	SensorID        string
	Timestamp       int64 // ms since Unix epoch
	FirmwareVersion string
	Model           string
	Status          string
}

// SensorLocationMessage is the sensor's own self-location, surfaced
// when the LTE add-on is fitted (firmware ≥ 20240927-1205). Not the
// drone's position — that sits inside SensorDataMessage.UASdataB64.
type SensorLocationMessage struct {
	SensorID    string
	Timestamp   int64 // ms since Unix epoch
	Latitude    float64
	Longitude   float64
	AltitudeMSL float64
	Satellites  int
	HDOP        float64
}

// DroneIDTypeUASdataBase64 is the DroneIDType value the normalizer
// stamps onto DecodedFrame when UASdata is preserved as base64 rather
// than decoded. Stage B always emits this value; Stage C will emit
// the proper "serial" / "caa" / "utm" / "specific-session" tokens
// extracted from the decoded BasicID block.
const DroneIDTypeUASdataBase64 = "uasdata-base64"

// Errors surfaced by the parser + normalizer. Callers branch via
// errors.Is.

// ErrDroneScoutLZMANotYetSupported is returned by ParseSensorPayload
// when invoked with compression == "lzma". Lifts in Stage C.
var ErrDroneScoutLZMANotYetSupported = errors.New(
	"feeds/remoteid: DroneScout LZMA decompression not yet supported (Stage C feature)",
)

// ErrDroneScoutInvalidCompression is returned when the compression
// argument is not "none", "" (treated as "none"), or "lzma".
var ErrDroneScoutInvalidCompression = errors.New(
	"feeds/remoteid: DroneScout compression must be \"none\" or \"lzma\"",
)

// ErrDroneScoutMalformedJSON is returned when one or more concatenated
// objects in the MQTT payload fail to parse.
var ErrDroneScoutMalformedJSON = errors.New(
	"feeds/remoteid: DroneScout payload contains malformed JSON",
)

// ErrDroneScoutNotDataKind is returned by NormalizeToDecodedFrame when
// invoked with a non-data SensorMessage (status / location / aircraft
// / mobile-network). Only data-kind messages produce a DecodedFrame.
var ErrDroneScoutNotDataKind = errors.New(
	"feeds/remoteid: DroneScout NormalizeToDecodedFrame requires Kind=data",
)

// ErrDroneScoutMissingUASdata is returned by NormalizeToDecodedFrame
// when the data-kind SensorMessage has an empty UASdataB64. Without
// it there is no DroneID to populate.
var ErrDroneScoutMissingUASdata = errors.New(
	"feeds/remoteid: DroneScout data message is missing UASdata",
)

// ParseSensorPayload parses one MQTT payload (one full message body,
// not a stream) into one-or-more SensorMessages. The payload may
// contain a single JSON object or several top-level objects
// concatenated (DroneScout `transmit_mode = 2` aggregate publishing,
// where the separator is `}{` between sibling objects).
//
// compression accepts "none" (default for empty) or "lzma". LZMA
// returns ErrDroneScoutLZMANotYetSupported in Stage B.
//
// Edge cases handled:
//   - Trailing 0x00 / 0x0a / whitespace cleanup.
//   - Empty / whitespace-only payload → empty result, no error.
//   - JSON object whose `protocol` field is not 1.0 is accepted (we
//     log the value via SensorMessage.Protocol but do not gate).
//   - Object with neither data / status / location / aircraft /
//     mobile-network → skipped silently (forward-compat with vendor
//     adding new message kinds).
//   - Any JSON parse failure → ErrDroneScoutMalformedJSON wrapped with
//     the offending fragment index.
func ParseSensorPayload(payload []byte, compression string) ([]SensorMessage, error) {
	switch compression {
	case "", "none":
		// ok
	case "lzma":
		// TODO(stage-c) lzma-decode: add LZMA decompression branch.
		return nil, ErrDroneScoutLZMANotYetSupported
	default:
		return nil, fmt.Errorf("%w: got %q", ErrDroneScoutInvalidCompression, compression)
	}

	// Trim trailing 0x00 / 0x0a then surrounding whitespace.
	s := strings.TrimRight(string(payload), "\x00\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	fragments := splitTopLevelJSONObjects(s)
	if len(fragments) == 0 {
		return nil, nil
	}

	out := make([]SensorMessage, 0, len(fragments))
	for i, frag := range fragments {
		frag = strings.TrimSpace(frag)
		if frag == "" {
			continue
		}
		msg, ok, err := parseOneFragment(frag)
		if err != nil {
			return nil, fmt.Errorf("%w: fragment %d: %v", ErrDroneScoutMalformedJSON, i, err)
		}
		if !ok {
			continue // unknown kind; skip
		}
		out = append(out, msg)
	}
	return out, nil
}

// splitTopLevelJSONObjects walks a JSON-encoded string and returns
// each balanced top-level object as a separate slice element. Brace
// counting is string-literal-aware so a `}{` that appears inside a
// quoted value does not cause a false split. This is the safer
// reimplementation of the BlueMark reference's naive `}{` substring
// split.
func splitTopLevelJSONObjects(s string) []string {
	var out []string
	depth := 0
	inString := false
	escape := false
	start := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			switch c {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				out = append(out, s[start:i+1])
				start = -1
			}
		}
	}
	return out
}

// parseOneFragment unmarshals a single JSON object and discriminates
// by which sub-block is populated. Returns (msg, true, nil) on a
// recognised kind, (zero, false, nil) on unknown kind (forward-compat
// skip), or (zero, false, err) on JSON parse error.
func parseOneFragment(frag string) (SensorMessage, bool, error) {
	var raw rawEnvelope
	if err := json.Unmarshal([]byte(frag), &raw); err != nil {
		return SensorMessage{}, false, err
	}

	msg := SensorMessage{Protocol: raw.Protocol}
	switch {
	case raw.Data != nil:
		msg.Kind = SensorMessageData
		msg.Data = &SensorDataMessage{
			SensorID:   raw.Data.SensorID,
			RSSI:       raw.Data.RSSI,
			Channel:    raw.Data.Channel,
			Timestamp:  raw.Data.Timestamp,
			MACAddress: raw.Data.MACAddress,
			Type:       raw.Data.Type,
			UASdataB64: raw.Data.UASdata,
			RawB64:     raw.Data.Raw,
		}
	case raw.Status != nil:
		msg.Kind = SensorMessageStatus
		msg.Status = &SensorStatusMessage{
			SensorID:        raw.Status.SensorID,
			Timestamp:       raw.Status.Timestamp,
			FirmwareVersion: raw.Status.FirmwareVersion,
			Model:           raw.Status.Model,
			Status:          raw.Status.Status,
		}
	case raw.Location != nil:
		msg.Kind = SensorMessageLocation
		msg.Location = &SensorLocationMessage{
			SensorID:    raw.Location.SensorID,
			Timestamp:   raw.Location.Timestamp,
			Latitude:    raw.Location.Latitude,
			Longitude:   raw.Location.Longitude,
			AltitudeMSL: raw.Location.AltitudeMSL,
			Satellites:  raw.Location.Satellites,
			HDOP:        raw.Location.HDOP,
		}
	case len(raw.Aircraft) > 0:
		msg.Kind = SensorMessageAircraft
		// Out of scope: 016 ADS-B owns aircraft messages. Surface
		// the kind so the caller can route, but do not decode here.
	case len(raw.MobileNetwork) > 0:
		msg.Kind = SensorMessageMobileNetwork
		// Out of scope: operator telemetry, not data-plane traffic.
	default:
		return SensorMessage{}, false, nil
	}
	return msg, true, nil
}

// rawEnvelope mirrors the on-disk JSON shape. Field tags use the exact
// keys the BlueMark sensor publishes (some contain spaces). Each
// sub-object is a pointer so its absence is distinguishable from its
// zero-value presence.
type rawEnvelope struct {
	Protocol      float64             `json:"protocol"`
	Data          *rawDataMessage     `json:"data,omitempty"`
	Status        *rawStatusMessage   `json:"status,omitempty"`
	Location      *rawLocationMessage `json:"location,omitempty"`
	Aircraft      json.RawMessage     `json:"aircraft,omitempty"`
	MobileNetwork json.RawMessage     `json:"mobile network,omitempty"`
}

type rawDataMessage struct {
	SensorID   string `json:"sensor ID"`
	RSSI       int    `json:"RSSI"`
	Channel    int    `json:"channel"`
	Timestamp  int64  `json:"timestamp"`
	MACAddress string `json:"MAC address"`
	Type       string `json:"type"`
	UASdata    string `json:"UASdata"`
	Raw        string `json:"raw,omitempty"`
}

type rawStatusMessage struct {
	SensorID        string `json:"sensor ID"`
	Timestamp       int64  `json:"timestamp"`
	FirmwareVersion string `json:"firmware version"`
	Model           string `json:"model"`
	Status          string `json:"status"`
}

type rawLocationMessage struct {
	SensorID    string  `json:"sensor ID"`
	Timestamp   int64   `json:"timestamp"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	AltitudeMSL float64 `json:"altitude MSL"`
	Satellites  int     `json:"satellites"`
	HDOP        float64 `json:"HDOP"`
}

// NormalizeToDecodedFrame converts one data-kind SensorMessage into a
// DecodedFrame ready for canonical-JSON RemoteIdFrame emission via
// internal/dapp/remoteid.FromDecoded + RemoteIdFrame.MarshalJSON.
//
// Stage B behaviour:
//   - Only data-kind messages produce a DecodedFrame. Other kinds
//     return ErrDroneScoutNotDataKind so the caller can skip them.
//   - DecodedFrame.ObservedAt is the SensorDataMessage.Timestamp (ms)
//     converted to time.Time.
//   - DecodedFrame.Source is sourceTag (or "dronescout-ds240" if
//     sourceTag is empty — 2026-05-13 likely-model default).
//   - DecodedFrame.DroneID is set to the raw base64 UASdata string;
//     DecodedFrame.DroneIDType is "uasdata-base64". Per the spec-017
//     FR-R14 "placeholder" feed-source semantics, this is explicit
//     not-yet-decoded framing rather than silently-bad data. Stage C
//     decodes the binary and emits proper "serial" / "caa" / "utm" /
//     "specific-session" droneIdType values.
//   - Position / Velocity / Operator / RegulatorVariant are left nil
//     / empty. We do not invent position from absent data.
//
// sourceTag is typically DroneScoutMQTTConfig.SourceTag() —
// "dronescout-ds240" by default per the 2026-05-13 likely-model
// guidance.
func NormalizeToDecodedFrame(m SensorMessage, sourceTag string) (DecodedFrame, error) {
	if m.Kind != SensorMessageData {
		return DecodedFrame{}, fmt.Errorf("%w: got kind=%q", ErrDroneScoutNotDataKind, m.Kind)
	}
	if m.Data == nil {
		return DecodedFrame{}, fmt.Errorf("%w: data kind has nil Data sub-struct", ErrDroneScoutNotDataKind)
	}
	if m.Data.UASdataB64 == "" {
		return DecodedFrame{}, ErrDroneScoutMissingUASdata
	}

	tag := sourceTag
	if tag == "" {
		tag = "dronescout-ds240"
	}

	// data.timestamp is milliseconds since Unix epoch (UTC); convert
	// to a UTC time.Time via nanosecond multiplication.
	observed := time.Unix(0, m.Data.Timestamp*int64(time.Millisecond)).UTC()

	// TODO(stage-c) uasdata-decode: replace the base64 preservation
	// below with byte-level ODID decoding (research notes §6.2).
	// Until then, surface the raw base64
	// with an explicit "uasdata-base64" type so downstream consumers
	// see "not-yet-decoded" framing instead of silently-bad data.
	return DecodedFrame{
		Type:        "remote-id-frame",
		Version:     "1.0.0",
		ObservedAt:  observed,
		Source:      tag,
		DroneID:     m.Data.UASdataB64,
		DroneIDType: DroneIDTypeUASdataBase64,
		// Position / Velocity / Operator deliberately nil — Stage C
		// decodes them out of UASdata.
		// RegulatorVariant deliberately empty — see research report
		// §6.2 + open question §10 Q11.
	}, nil
}
