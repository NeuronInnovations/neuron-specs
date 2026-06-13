package payment

import (
	"encoding/json"
	"strconv"
)

// Lifecycle payloads added 2026-05-08 (reference-demo direction; 008 FR-P36/P37/P38).
//
// These three payloads form the explicit lifecycle-control set on top of the
// existing six-message negotiation/delivery/settlement core. They share the
// canonical-JSON envelope discipline of FR-P12 and do NOT introduce a new
// transport mechanism — they are TopicMessage payloads on stdIn, signed by
// the sender's NeuronPrivateKey per the existing TopicMessage path (002 + 004).
//
// State-machine integration: each payload corresponds to one event in
// AgreementStateMachine (see agreement.go); ServiceStop drives ACTIVE → TERMINATED,
// ServiceCancel drives any pre-COMPLETED state → TERMINATED, and ServiceRenew
// updates the persisted active-service entry's expiresAt without changing
// the lifecycle state.

// ServiceStop is the buyer→seller signal to discontinue an ACTIVE service.
// FR-P36.
//
// Canonical order (per 006 wire-format.md §2): type → version → requestId → reason* → effectiveAt*.
//
// EffectiveAt is the Unix timestamp in nanoseconds at which the buyer wants
// the stop to take effect. When zero (the field is absent on the wire), the
// stop is effective immediately on receipt.
type ServiceStop struct {
	Type        string `json:"-"`
	Version     string `json:"-"`
	RequestID   string `json:"-"`
	Reason      string `json:"-"` // optional, informational
	EffectiveAt uint64 `json:"-"` // optional, nanoseconds since Unix epoch; 0 = immediate
}

// MarshalJSON implements canonical field ordering for ServiceStop. FR-P12 + FR-P36.
func (s ServiceStop) MarshalJSON() ([]byte, error) {
	m := []jsonKeyValue{
		{"type", s.Type},
		{"version", s.Version},
		{"requestId", s.RequestID},
	}
	if s.Reason != "" {
		m = append(m, jsonKeyValue{"reason", s.Reason})
	}
	if s.EffectiveAt != 0 {
		m = append(m, jsonKeyValue{"effectiveAt", strconv.FormatUint(s.EffectiveAt, 10)})
	}
	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a ServiceStop from JSON.
func (s *ServiceStop) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	unmarshalString(raw, "type", &s.Type)
	unmarshalString(raw, "version", &s.Version)
	unmarshalString(raw, "requestId", &s.RequestID)
	unmarshalString(raw, "reason", &s.Reason)
	if effRaw, ok := raw["effectiveAt"]; ok {
		var str string
		if err := json.Unmarshal(effRaw, &str); err == nil {
			s.EffectiveAt, _ = strconv.ParseUint(str, 10, 64)
		}
	}
	return nil
}

// ServiceCancel is the either-party signal to abort an agreement before it
// reaches COMPLETED. FR-P37.
//
// Canonical order (per 006 wire-format.md §2): type → version → requestId → reason* → refundRequested*.
//
// RefundRequested defaults to false on the wire (omitted when zero). Buyers
// SHOULD set RefundRequested = true when the agreement has reached FUNDED or
// later; the standard claimRefund flow (FR-P22) handles the actual fund
// movement after the escrow timeout elapses.
type ServiceCancel struct {
	Type             string `json:"-"`
	Version          string `json:"-"`
	RequestID        string `json:"-"`
	Reason           string `json:"-"` // optional, informational
	RefundRequested  bool   `json:"-"` // optional; true → buyer expects claimRefund to be available after timeout
}

// MarshalJSON implements canonical field ordering for ServiceCancel. FR-P12 + FR-P37.
func (c ServiceCancel) MarshalJSON() ([]byte, error) {
	m := []jsonKeyValue{
		{"type", c.Type},
		{"version", c.Version},
		{"requestId", c.RequestID},
	}
	if c.Reason != "" {
		m = append(m, jsonKeyValue{"reason", c.Reason})
	}
	if c.RefundRequested {
		m = append(m, jsonKeyValue{"refundRequested", true})
	}
	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a ServiceCancel from JSON.
func (c *ServiceCancel) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type            string `json:"type"`
		Version         string `json:"version"`
		RequestID       string `json:"requestId"`
		Reason          string `json:"reason"`
		RefundRequested bool   `json:"refundRequested"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Type = raw.Type
	c.Version = raw.Version
	c.RequestID = raw.RequestID
	c.Reason = raw.Reason
	c.RefundRequested = raw.RefundRequested
	return nil
}

// ServiceRenew is the buyer→seller signal to extend an ACTIVE agreement's
// expiresAt without state-machine transition. FR-P38.
//
// Canonical order (per 006 wire-format.md §2): type → version → requestId → extendUntil → reason*.
//
// ExtendUntil is the Unix timestamp in nanoseconds the buyer commits to.
// MUST be strictly greater than the current expiresAt; receivers SHOULD
// surface a RenewExpiryNotMonotonic error (FR-P32) when this constraint is
// violated and ignore the renewal.
type ServiceRenew struct {
	Type        string `json:"-"`
	Version     string `json:"-"`
	RequestID   string `json:"-"`
	ExtendUntil uint64 `json:"-"` // nanoseconds since Unix epoch
	Reason      string `json:"-"` // optional, informational
}

// MarshalJSON implements canonical field ordering for ServiceRenew. FR-P12 + FR-P38.
func (r ServiceRenew) MarshalJSON() ([]byte, error) {
	m := []jsonKeyValue{
		{"type", r.Type},
		{"version", r.Version},
		{"requestId", r.RequestID},
		{"extendUntil", strconv.FormatUint(r.ExtendUntil, 10)},
	}
	if r.Reason != "" {
		m = append(m, jsonKeyValue{"reason", r.Reason})
	}
	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes a ServiceRenew from JSON.
func (r *ServiceRenew) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	unmarshalString(raw, "type", &r.Type)
	unmarshalString(raw, "version", &r.Version)
	unmarshalString(raw, "requestId", &r.RequestID)
	unmarshalString(raw, "reason", &r.Reason)
	if extRaw, ok := raw["extendUntil"]; ok {
		var str string
		if err := json.Unmarshal(extRaw, &str); err == nil {
			r.ExtendUntil, _ = strconv.ParseUint(str, 10, 64)
		}
	}
	return nil
}
