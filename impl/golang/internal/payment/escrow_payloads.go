package payment

import (
	"encoding/json"
)

// Payload type constants for escrow payloads. FR-P06.
const (
	PayloadEscrowCreated = "escrowCreated"
	PayloadInvoice       = "invoice"
	PayloadInvoiceAck    = "invoiceAck"
)

// EscrowCreated notifies the seller that escrow is funded. FR-P09.
// Canonical order: type→version→requestId→escrowRef→depositAmount→depositCurrency
type EscrowCreated struct {
	Type            string `json:"-"`
	Version         string `json:"-"`
	RequestID       string `json:"-"`
	EscrowRef       string `json:"-"` // opaque binding-specific
	DepositAmount   string `json:"-"` // decimal string
	DepositCurrency string `json:"-"`
}

// MarshalJSON implements canonical field ordering. FR-P12.
func (e EscrowCreated) MarshalJSON() ([]byte, error) {
	return marshalOrderedJSON([]jsonKeyValue{
		{"type", e.Type},
		{"version", e.Version},
		{"requestId", e.RequestID},
		{"escrowRef", e.EscrowRef},
		{"depositAmount", e.DepositAmount},
		{"depositCurrency", e.DepositCurrency},
	})
}

// UnmarshalJSON deserializes an EscrowCreated from JSON.
func (e *EscrowCreated) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type            string `json:"type"`
		Version         string `json:"version"`
		RequestID       string `json:"requestId"`
		EscrowRef       string `json:"escrowRef"`
		DepositAmount   string `json:"depositAmount"`
		DepositCurrency string `json:"depositCurrency"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.Type = raw.Type
	e.Version = raw.Version
	e.RequestID = raw.RequestID
	e.EscrowRef = raw.EscrowRef
	e.DepositAmount = raw.DepositAmount
	e.DepositCurrency = raw.DepositCurrency
	return nil
}

// Invoice requests payment release for delivered service. FR-P10.
// Canonical order: type→version→requestId→releaseRequestRef→escrowRef→amount→currency→period
type Invoice struct {
	Type              string `json:"-"`
	Version           string `json:"-"`
	RequestID         string `json:"-"`
	ReleaseRequestRef string `json:"-"` // opaque binding-specific
	EscrowRef         string `json:"-"`
	Amount            string `json:"-"` // decimal string
	Currency          string `json:"-"`
	Period            string `json:"-"` // ISO 8601 interval (e.g., "PT1H")
}

// MarshalJSON implements canonical field ordering. FR-P12.
func (inv Invoice) MarshalJSON() ([]byte, error) {
	return marshalOrderedJSON([]jsonKeyValue{
		{"type", inv.Type},
		{"version", inv.Version},
		{"requestId", inv.RequestID},
		{"releaseRequestRef", inv.ReleaseRequestRef},
		{"escrowRef", inv.EscrowRef},
		{"amount", inv.Amount},
		{"currency", inv.Currency},
		{"period", inv.Period},
	})
}

// UnmarshalJSON deserializes an Invoice from JSON.
func (inv *Invoice) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type              string `json:"type"`
		Version           string `json:"version"`
		RequestID         string `json:"requestId"`
		ReleaseRequestRef string `json:"releaseRequestRef"`
		EscrowRef         string `json:"escrowRef"`
		Amount            string `json:"amount"`
		Currency          string `json:"currency"`
		Period            string `json:"period"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	inv.Type = raw.Type
	inv.Version = raw.Version
	inv.RequestID = raw.RequestID
	inv.ReleaseRequestRef = raw.ReleaseRequestRef
	inv.EscrowRef = raw.EscrowRef
	inv.Amount = raw.Amount
	inv.Currency = raw.Currency
	inv.Period = raw.Period
	return nil
}

// InvoiceAck is the buyer's response to an invoice. FR-P11.
// Canonical order: type→version→requestId→releaseRequestRef→action→depositedMore*→newBalance*
type InvoiceAck struct {
	Type              string `json:"-"`
	Version           string `json:"-"`
	RequestID         string `json:"-"`
	ReleaseRequestRef string `json:"-"`
	Action            string `json:"-"` // "approved" or "refused"
	DepositedMore     *bool  `json:"-"` // optional
	NewBalance        string `json:"-"` // optional decimal string
}

// MarshalJSON implements canonical field ordering. FR-P12.
func (a InvoiceAck) MarshalJSON() ([]byte, error) {
	m := []jsonKeyValue{
		{"type", a.Type},
		{"version", a.Version},
		{"requestId", a.RequestID},
		{"releaseRequestRef", a.ReleaseRequestRef},
		{"action", a.Action},
	}

	if a.DepositedMore != nil {
		m = append(m, jsonKeyValue{"depositedMore", *a.DepositedMore})
	}
	if a.NewBalance != "" {
		m = append(m, jsonKeyValue{"newBalance", a.NewBalance})
	}

	return marshalOrderedJSON(m)
}

// UnmarshalJSON deserializes an InvoiceAck from JSON.
func (a *InvoiceAck) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type              string `json:"type"`
		Version           string `json:"version"`
		RequestID         string `json:"requestId"`
		ReleaseRequestRef string `json:"releaseRequestRef"`
		Action            string `json:"action"`
		DepositedMore     *bool  `json:"depositedMore"`
		NewBalance        string `json:"newBalance"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	a.Type = raw.Type
	a.Version = raw.Version
	a.RequestID = raw.RequestID
	a.ReleaseRequestRef = raw.ReleaseRequestRef
	a.Action = raw.Action
	a.DepositedMore = raw.DepositedMore
	a.NewBalance = raw.NewBalance
	return nil
}

