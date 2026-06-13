package cotadapt

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
)

// unknownError is the CoT sentinel for an unknown circular/linear error (metres).
const unknownError = 9999999.0

// cotTimeLayout is ISO-8601 UTC with milliseconds (e.g. 2026-06-02T09:39:25.000Z).
const cotTimeLayout = "2006-01-02T15:04:05.000Z07:00"

// Event is a CoT event. Floats are pre-formatted strings so the output is
// deterministic and CoT-conventional (decimal, not scientific).
type Event struct {
	XMLName xml.Name `xml:"event"`
	Version string   `xml:"version,attr"`
	UID     string   `xml:"uid,attr"`
	Type    string   `xml:"type,attr"`
	Time    string   `xml:"time,attr"`
	Start   string   `xml:"start,attr"`
	Stale   string   `xml:"stale,attr"`
	How     string   `xml:"how,attr"`
	Point   Point    `xml:"point"`
	Detail  *Detail  `xml:"detail,omitempty"`
}

// Point is the CoT <point>.
type Point struct {
	Lat string `xml:"lat,attr"`
	Lon string `xml:"lon,attr"`
	HAE string `xml:"hae,attr"`
	CE  string `xml:"ce,attr"`
	LE  string `xml:"le,attr"`
}

// Detail is the CoT <detail>.
type Detail struct {
	Contact *Contact `xml:"contact,omitempty"`
	Link    *Link    `xml:"link,omitempty"`
	Remarks string   `xml:"remarks,omitempty"`
}

// Contact is a CoT <contact>.
type Contact struct {
	Callsign string `xml:"callsign,attr"`
}

// Link is a CoT <link> (relationship to another event).
type Link struct {
	Relation string `xml:"relation,attr"`
	UID      string `xml:"uid,attr"`
	Type     string `xml:"type,attr,omitempty"`
}

// Options configures the projection. The zero value is NOT usable directly — use
// DefaultOptions() or rely on normalize() filling defaults (affiliation 'u',
// never hostile).
type Options struct {
	Affiliation  byte          // CoT affiliation char; default 'u' (unknown)
	Type         string        // drone type atom; default "a-<aff>-A"
	OperatorType string        // operator type atom; default "a-<aff>-G"
	How          string        // CoT how (h-/m- origin taxonomy); default "m-g" (machine/GPS)
	TTL          time.Duration // stale = time + TTL; default 5s
	OmitOperator bool          // when true, never emit the operator event

	// ProvenanceNodeID, when non-empty, appends " node_id=<id>" to the drone
	// event's <remarks> — surfacing the seller's SAPIENT node_id (the
	// identity-bound id re-stamped on the SapientMessage) as display provenance.
	// Default empty: remarks are unchanged (the library default output is
	// byte-stable, so golden vectors are unaffected).
	ProvenanceNodeID string
}

// DefaultOptions returns the unknown-affiliation defaults.
func DefaultOptions() Options {
	return normalize(Options{})
}

// Normalize returns o with every unset field filled with the library default
// (affiliation 'u', type "a-<aff>-A", operator type "a-<aff>-G", how "m-g",
// TTL 5s) — exactly what ToCoT applies internally. Exported so callers that
// need the canonical derived values (e.g. a display surfacing the CoT type
// without building events) don't re-implement the defaults. Idempotent.
func Normalize(o Options) Options {
	return normalize(o)
}

func normalize(o Options) Options {
	if o.Affiliation == 0 {
		o.Affiliation = 'u'
	}
	if o.Type == "" {
		o.Type = "a-" + string(o.Affiliation) + "-A"
	}
	if o.OperatorType == "" {
		o.OperatorType = "a-" + string(o.Affiliation) + "-G"
	}
	if o.How == "" {
		// CoT `how` uses the human/machine origin taxonomy (h-* / m-*), NOT the
		// type-atom "a-" prefix. RID positions are GPS-derived → machine/GPS.
		o.How = "m-g"
	}
	if o.TTL <= 0 {
		o.TTL = 5 * time.Second
	}
	return o
}

// ToCoT projects a SapientMessage's DetectionReport into one or two CoT events
// (drone, and optionally its operator).
func ToCoT(msg *sapientpb.SapientMessage, opts Options) ([]Event, error) {
	opts = normalize(opts)
	dr := msg.GetDetectionReport()
	if dr == nil {
		return nil, New(ErrNoDetection, "ToCoT", "message carries no DetectionReport")
	}
	uid := dr.GetId()
	if uid == "" {
		uid = dr.GetObjectId()
	}
	if uid == "" {
		return nil, New(ErrNoIdentity, "ToCoT", "DetectionReport has neither id nor object_id")
	}
	loc := dr.GetLocation()
	if loc == nil {
		return nil, New(ErrNoLocation, "ToCoT", "DetectionReport has no Location for a CoT point")
	}

	t := msg.GetTimestamp().AsTime().UTC()
	timeStr := t.Format(cotTimeLayout)
	staleStr := t.Add(opts.TTL).Format(cotTimeLayout)

	drone := Event{
		Version: "2.0",
		UID:     uid,
		Type:    opts.Type,
		Time:    timeStr,
		Start:   timeStr,
		Stale:   staleStr,
		How:     opts.How,
		Point: Point{
			Lat: fnum(loc.GetY()),
			Lon: fnum(loc.GetX()),
			HAE: fnum(loc.GetZ()),
			CE:  fnum(horizontalError(loc)),
			LE:  fnum(verticalError(loc)),
		},
		Detail: &Detail{
			Contact: &Contact{Callsign: uid},
			Remarks: classRemark(dr),
		},
	}
	// Optional display provenance: surface the seller's identity-bound node_id.
	if opts.ProvenanceNodeID != "" {
		drone.Detail.Remarks += " node_id=" + opts.ProvenanceNodeID
	}
	events := []Event{drone}

	info := indexObjectInfo(dr.GetObjectInfo())
	if !opts.OmitOperator {
		if op, ok := operatorEvent(uid, opts, timeStr, staleStr, info); ok {
			events = append(events, op)
		}
	}
	return events, nil
}

// ToXML projects a SapientMessage into CoT XML (one indented <event> per event,
// newline-separated).
func ToXML(msg *sapientpb.SapientMessage, opts Options) ([]byte, error) {
	events, err := ToCoT(msg, opts)
	if err != nil {
		return nil, err
	}
	return EventsToXML(events)
}

// EventsToXML renders events as indented CoT XML, newline-separated.
func EventsToXML(events []Event) ([]byte, error) {
	var buf bytes.Buffer
	for i, e := range events {
		if i > 0 {
			buf.WriteByte('\n')
		}
		b, err := xml.MarshalIndent(e, "", "  ")
		if err != nil {
			return nil, Wrap(ErrEncode, "EventsToXML", err)
		}
		buf.Write(b)
	}
	return buf.Bytes(), nil
}

func operatorEvent(droneUID string, opts Options, timeStr, staleStr string, info map[string]string) (Event, bool) {
	// Require BOTH lat and lon so a half-populated operator never lands the pin at
	// lon=0 (the Gulf of Guinea) — suppress the event instead.
	lat, latOK := parseFloat(info["rid.operatorLatDeg"])
	lon, lonOK := parseFloat(info["rid.operatorLonDeg"])
	if !latOK || !lonOK {
		return Event{}, false
	}
	alt, _ := parseFloat(info["rid.operatorAltM"])
	callsign := info["rid.operatorId"]
	if callsign == "" {
		callsign = droneUID + "-OP"
	}
	return Event{
		Version: "2.0",
		UID:     droneUID + "-OP",
		Type:    opts.OperatorType,
		Time:    timeStr,
		Start:   timeStr,
		Stale:   staleStr,
		How:     opts.How,
		Point: Point{
			Lat: fnum(lat),
			Lon: fnum(lon),
			HAE: fnum(alt),
			CE:  fnum(unknownError),
			LE:  fnum(unknownError),
		},
		Detail: &Detail{
			Contact: &Contact{Callsign: callsign},
			Link:    &Link{Relation: "p-p", UID: droneUID, Type: opts.Type},
			Remarks: "operator (display projection; not a SAPIENT DetectionReport)",
		},
	}, true
}

// horizontalError returns the CoT circular error from the location's x/y error, or
// the unknown sentinel. It uses the dominant axis (max of x/y). For Remote ID the
// bridge sets x_error == y_error from one ODID horizontal-accuracy bound, so the
// dominant axis IS the reported horizontal accuracy (no √2 inflation).
func horizontalError(loc *sapientpb.Location) float64 {
	xe, ye := loc.GetXError(), loc.GetYError()
	if xe <= 0 && ye <= 0 {
		return unknownError
	}
	if xe > ye {
		return xe
	}
	return ye
}

// verticalError returns the CoT linear error from the location's z error, or the
// unknown sentinel.
func verticalError(loc *sapientpb.Location) float64 {
	if ze := loc.GetZError(); ze > 0 {
		return ze
	}
	return unknownError
}

func classRemark(dr *sapientpb.DetectionReport) string {
	cls := dr.GetClassification()
	if len(cls) == 0 {
		return "class=unknown"
	}
	c := cls[0]
	r := "class=" + c.GetType()
	if c.GetConfidence() > 0 {
		// Format at float32 precision so 0.99 renders "0.99", not 0.9900000095…
		r += " confidence=" + strconv.FormatFloat(float64(c.GetConfidence()), 'f', -1, 32)
	}
	return r
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

// fnum formats a float for a CoT attribute: decimal, minimal digits (e.g.
// "9999999", "50.1027", "95").
func fnum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

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
