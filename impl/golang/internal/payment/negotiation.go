package payment

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PayloadType discriminates negotiation payload types. FR-P06.
//
// Six original taxonomy entries (FR-P06 pre-2026-05-08) plus three lifecycle
// entries added by the 2026-05-08 reference-demo amendment (FR-P36/P37/P38).
const (
	PayloadServiceRequest  = "serviceRequest"
	PayloadServiceResponse = "serviceResponse"
	PayloadConnectionSetup = "connectionSetup"
	// Lifecycle messages added 2026-05-08 per 008 FR-P36/P37/P38.
	PayloadServiceStop   = "serviceStop"
	PayloadServiceCancel = "serviceCancel"
	PayloadServiceRenew  = "serviceRenew"
)

// StreamDirection identifies which party MAY open a libp2p stream for a
// stream-catalog entry. FR-P33a + 009 FR-D-stream-direction.
const (
	StreamDirectionSeller = "seller-initiates"
	StreamDirectionBuyer  = "buyer-initiates"
	StreamDirectionEither = "either"
)

// StreamCatalogEntry is one entry in the optional Streams field of
// ConnectionSetup. FR-P33a.
//
// Canonical order (per 006 wire-format.md §2): name → protocolID → direction → schema*.
type StreamCatalogEntry struct {
	Name       string `json:"-"`
	ProtocolID string `json:"-"` // literal libp2p protocol ID OR wildcard pattern with single trailing "*"
	Direction  string `json:"-"` // one of StreamDirectionSeller | StreamDirectionBuyer | StreamDirectionEither
	Schema     string `json:"-"` // optional — URI pointer to per-stream payload schema
}

// MarshalJSON implements canonical field ordering for StreamCatalogEntry. FR-P12 + FR-P33a.
func (e StreamCatalogEntry) MarshalJSON() ([]byte, error) {
	m := []jsonKeyValue{
		{"name", e.Name},
		{"protocolID", e.ProtocolID},
		{"direction", e.Direction},
	}
	if e.Schema != "" {
		m = append(m, jsonKeyValue{"schema", e.Schema})
	}
	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a StreamCatalogEntry from JSON.
func (e *StreamCatalogEntry) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name       string `json:"name"`
		ProtocolID string `json:"protocolID"`
		Direction  string `json:"direction"`
		Schema     string `json:"schema"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.Name = raw.Name
	e.ProtocolID = raw.ProtocolID
	e.Direction = raw.Direction
	e.Schema = raw.Schema
	return nil
}

// ServiceRequest is the buyer→seller negotiation payload. FR-P07.
// Canonical order: type→version→requestId→serviceRef→settlementBinding→
// proposedAmount→proposedCurrency→proposedInterval→serviceParams*→
// negotiationDeadline→arbiter*→buyerStdIn
type ServiceRequest struct {
	Type                string         `json:"-"`
	Version             string         `json:"-"`
	RequestID           string         `json:"-"`
	ServiceRef          string         `json:"-"`
	SettlementBinding   string         `json:"-"`
	ProposedAmount      string         `json:"-"`
	ProposedCurrency    string         `json:"-"`
	ProposedInterval    string         `json:"-"`
	ServiceParams       map[string]any `json:"-"` // FR-P07: opaque, keys lexicographic
	NegotiationDeadline uint64         `json:"-"` // Unix epoch seconds (FR-W02a)
	Arbiter             string         `json:"-"` // optional
	BuyerStdIn          string         `json:"-"`
}

// MarshalJSON implements canonical field ordering for ServiceRequest. FR-P12.
func (r ServiceRequest) MarshalJSON() ([]byte, error) {
	m := []jsonKeyValue{
		{"type", r.Type},
		{"version", r.Version},
		{"requestId", r.RequestID},
		{"serviceRef", r.ServiceRef},
		{"settlementBinding", r.SettlementBinding},
		{"proposedAmount", r.ProposedAmount},
		{"proposedCurrency", r.ProposedCurrency},
		{"proposedInterval", r.ProposedInterval},
	}

	if r.ServiceParams != nil {
		sorted, err := marshalSortedMap(r.ServiceParams)
		if err != nil {
			return nil, err
		}
		m = append(m, jsonKeyValue{"serviceParams", json.RawMessage(sorted)})
	}

	m = append(m, jsonKeyValue{"negotiationDeadline", strconv.FormatUint(r.NegotiationDeadline, 10)})

	if r.Arbiter != "" {
		m = append(m, jsonKeyValue{"arbiter", r.Arbiter})
	}

	m = append(m, jsonKeyValue{"buyerStdIn", r.BuyerStdIn})

	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a ServiceRequest from JSON.
func (r *ServiceRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	unmarshalString(raw, "type", &r.Type)
	unmarshalString(raw, "version", &r.Version)
	unmarshalString(raw, "requestId", &r.RequestID)
	unmarshalString(raw, "serviceRef", &r.ServiceRef)
	unmarshalString(raw, "settlementBinding", &r.SettlementBinding)
	unmarshalString(raw, "proposedAmount", &r.ProposedAmount)
	unmarshalString(raw, "proposedCurrency", &r.ProposedCurrency)
	unmarshalString(raw, "proposedInterval", &r.ProposedInterval)
	unmarshalString(raw, "arbiter", &r.Arbiter)
	unmarshalString(raw, "buyerStdIn", &r.BuyerStdIn)

	if deadlineRaw, ok := raw["negotiationDeadline"]; ok {
		var s string
		if err := json.Unmarshal(deadlineRaw, &s); err == nil {
			r.NegotiationDeadline, _ = strconv.ParseUint(s, 10, 64)
		}
	}

	if paramsRaw, ok := raw["serviceParams"]; ok {
		var params map[string]any
		if err := json.Unmarshal(paramsRaw, &params); err == nil {
			r.ServiceParams = params
		}
	}

	return nil
}

// ServiceResponse is the seller→buyer negotiation response. FR-P08.
// Canonical order: type→version→requestId→action→counterAmount*→counterInterval*
type ServiceResponse struct {
	Type            string `json:"-"`
	Version         string `json:"-"`
	RequestID       string `json:"-"`
	Action          string `json:"-"` // "accept", "reject", "counter"
	CounterAmount   string `json:"-"` // present when action="counter"
	CounterInterval string `json:"-"` // present when action="counter"
}

// MarshalJSON implements canonical field ordering for ServiceResponse. FR-P12.
func (r ServiceResponse) MarshalJSON() ([]byte, error) {
	m := []jsonKeyValue{
		{"type", r.Type},
		{"version", r.Version},
		{"requestId", r.RequestID},
		{"action", r.Action},
	}

	if r.Action == "counter" {
		m = append(m, jsonKeyValue{"counterAmount", r.CounterAmount})
		m = append(m, jsonKeyValue{"counterInterval", r.CounterInterval})
	}

	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a ServiceResponse from JSON.
func (r *ServiceResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type            string `json:"type"`
		Version         string `json:"version"`
		RequestID       string `json:"requestId"`
		Action          string `json:"action"`
		CounterAmount   string `json:"counterAmount"`
		CounterInterval string `json:"counterInterval"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Type = raw.Type
	r.Version = raw.Version
	r.RequestID = raw.RequestID
	r.Action = raw.Action
	r.CounterAmount = raw.CounterAmount
	r.CounterInterval = raw.CounterInterval
	return nil
}

// ConnectionSetup is the bidirectional peer address exchange. FR-P33 + FR-P33a.
//
// Canonical order (per 006 wire-format.md §2):
//   type → version → requestId → peerID → encryptedMultiaddrs → protocol* → streams* → natStatus*
//
// Back-compat (FR-P33a): the legacy single-string Protocol field is preserved.
// Pre-2026-05-08 senders emitted only Protocol; post-2026-05-08 senders MAY
// emit a Streams catalog instead of, or in addition to, Protocol. Receivers
// MUST accept either form. At least one of Protocol or Streams MUST be
// populated per FR-P33; both MAY be present per FR-P33a, in which case
// Streams is the authoritative stream catalog and Protocol is interpreted as
// the legacy single-stream advertisement.
type ConnectionSetup struct {
	Type                string               `json:"-"`
	Version             string               `json:"-"`
	RequestID           string               `json:"-"`
	PeerID              string               `json:"-"` // libp2p PeerID (002 FR-006)
	EncryptedMultiaddrs string               `json:"-"` // base64 encoded (006 FR-W03)
	Protocol            string               `json:"-"` // legacy single stream protocol ID (FR-P33; pre-2026-05-08)
	Streams             []StreamCatalogEntry `json:"-"` // stream catalog (FR-P33a; added 2026-05-08)
	NATStatus           string               `json:"-"` // optional: "public", "private", "unknown"
}

// MarshalJSON implements canonical field ordering for ConnectionSetup. FR-P12 + FR-P33a.
func (c ConnectionSetup) MarshalJSON() ([]byte, error) {
	m := []jsonKeyValue{
		{"type", c.Type},
		{"version", c.Version},
		{"requestId", c.RequestID},
		{"peerID", c.PeerID},
		{"encryptedMultiaddrs", c.EncryptedMultiaddrs},
	}

	if c.Protocol != "" {
		m = append(m, jsonKeyValue{"protocol", c.Protocol})
	}

	if len(c.Streams) > 0 {
		m = append(m, jsonKeyValue{"streams", c.Streams})
	}

	if c.NATStatus != "" {
		m = append(m, jsonKeyValue{"natStatus", c.NATStatus})
	}

	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a ConnectionSetup from JSON.
func (c *ConnectionSetup) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type                string               `json:"type"`
		Version             string               `json:"version"`
		RequestID           string               `json:"requestId"`
		PeerID              string               `json:"peerID"`
		EncryptedMultiaddrs string               `json:"encryptedMultiaddrs"`
		Protocol            string               `json:"protocol"`
		Streams             []StreamCatalogEntry `json:"streams"`
		NATStatus           string               `json:"natStatus"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Type = raw.Type
	c.Version = raw.Version
	c.RequestID = raw.RequestID
	c.PeerID = raw.PeerID
	c.EncryptedMultiaddrs = raw.EncryptedMultiaddrs
	c.Protocol = raw.Protocol
	c.Streams = raw.Streams
	c.NATStatus = raw.NATStatus
	return nil
}

// ValidateVersion checks version compatibility per FR-P12a.
// Accept major=1, reject major>=2.
func ValidateVersion(version string) error {
	const op = "ValidateVersion"

	parts := strings.SplitN(version, ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return NewPaymentError(ErrVersionMismatch, op, "empty version string")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return NewPaymentError(ErrVersionMismatch, op,
			fmt.Sprintf("invalid major version: %s", parts[0]))
	}

	if major >= 2 {
		return NewPaymentError(ErrVersionMismatch, op,
			fmt.Sprintf("unsupported major version %d (only 1.x.y accepted)", major))
	}

	if major < 1 {
		return NewPaymentError(ErrVersionMismatch, op,
			fmt.Sprintf("invalid major version %d", major))
	}

	return nil
}

// --- helpers ---

// marshalSortedMap serializes a map with keys in lexicographic order. FR-P07.
func marshalSortedMap(m map[string]any) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]jsonKeyValue, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, jsonKeyValue{k, m[k]})
	}

	return marshalOrderedJSON(pairs)
}

// unmarshalString is a helper to extract a string field from raw JSON map.
func unmarshalString(raw map[string]json.RawMessage, key string, target *string) {
	if v, ok := raw[key]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			*target = s
		}
	}
}
